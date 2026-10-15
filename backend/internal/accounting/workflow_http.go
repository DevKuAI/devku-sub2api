package accounting

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type workflowInput struct {
	OrganizationID string `json:"organization_id"`
	SourceKind     string `json:"source_kind"`
	SourceID       string `json:"source_id"`
	TransferID     string `json:"transfer_id"`
	InvoiceID      string `json:"invoice_id"`
	Amount         string `json:"amount"`
	SourceAmount   string `json:"source_amount"`
	ExchangeRate   string `json:"exchange_rate"`
	Currency       string `json:"currency"`
	Reference      string `json:"reference"`
	AttachmentID   string `json:"attachment_id"`
	Reason         string `json:"reason"`
	Month          string `json:"month"`
	OccurredAt     string `json:"occurred_at"`
	AcceptPartial  bool   `json:"accept_partial"`
}

func (h *Handler) registerWorkflows(g *gin.RouterGroup) {
	g.GET("/workbench", h.workbench)
	g.GET("/activity/:kind/:id", h.activity)
	g.GET("/related/:kind/:id", h.related)
	g.GET("/receivables/:id/settlements", h.settlements)
	g.POST("/receivables/:id/settlements", h.settleReceipt)
	g.POST("/settlements/:id/void", h.voidSettlement)
	g.GET("/cost-allocations", h.costAllocations)
	g.GET("/cost-summary", h.costSummary)
	g.POST("/cost-allocations", h.allocateCost)
	g.POST("/cost-allocations/:id/void", h.voidCost)
	g.GET("/external-actions", h.externalActions)
	g.POST("/external-actions/:id/confirm", h.confirmExternal)
	g.POST("/external-actions/:id/void", h.exemptExternal)
	g.POST("/fx-exchanges/:id/allocations", h.allocateFX)
	g.GET("/periods", h.periods)
	g.POST("/periods/close", h.closePeriod)
	g.POST("/periods/:id/reopen", h.reopenPeriod)
	g.GET("/statements/:id", h.statement)
	g.GET("/statements/:id/export", h.statementExport)
}
func workflowDecode(c *gin.Context) (workflowInput, bool) {
	var in workflowInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return in, false
	}
	return in, true
}
func (h *Handler) workflowOrg(c *gin.Context, public string) (int64, bool) {
	if orgScope(c) > 0 {
		return orgScope(c), true
	}
	if public == "" {
		response.ErrorFrom(c, invalid("organization_id"))
		return 0, false
	}
	id, err := h.resolveOrg(c, public)
	return id, !response.ErrorFrom(c, err)
}
func workflowTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Now().UTC(), nil
	}
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return v, invalid("occurred_at")
	}
	return v, nil
}
func requireAdmin(c *gin.Context) bool {
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != "admin" {
		response.ErrorFrom(c, ErrForbidden)
		return false
	}
	return true
}
func (h *Handler) settleReceipt(c *gin.Context) {
	r := h.record(c, "receivables")
	if r == nil {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: "receivables", Object: r.ID, Action: "settle", Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := get(c.Request.Context(), tx, "receivables", r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if current.Status != "CONFIRMED" {
			return nil, ErrState
		}
		receipt, err := get(c.Request.Context(), tx, "transfers", in.TransferID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if receipt.Status != "RECEIVED" || detail(receipt, "funding_type") == "FX" {
			return nil, ErrState
		}
		if receipt.Currency != current.Currency {
			return nil, ErrCurrency
		}
		value, err := number(in.Amount, current.Currency, true)
		if err != nil {
			return nil, err
		}
		settled, err := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_receipt_settlements WHERE receivable_id=$1 AND status='ACTIVE'", r.ID)
		if err != nil {
			return nil, err
		}
		assigned, err := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_receipt_settlements WHERE transfer_id=$1 AND status='ACTIVE'", receipt.ID)
		if err != nil {
			return nil, err
		}
		received, err := effective(c.Request.Context(), tx, "transfers", receipt)
		if err != nil {
			return nil, err
		}
		if settled.Add(value).GreaterThan(amount(current)) || assigned.Add(value).GreaterThan(received) {
			return nil, ErrAmount
		}
		occurred, err := workflowTime(in.OccurredAt)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		_, err = tx.ExecContext(c.Request.Context(), `INSERT INTO accounting_receipt_settlements(id,organization_id,receivable_id,transfer_id,amount,currency,status,occurred_at,created_by) VALUES($1,$2,$3,$4,$5,$6,'ACTIVE',$7,$8)`, id, r.Org, r.ID, receipt.ID, value.String(), receipt.Currency, occurred, actor(c))
		return gin.H{"id": id, "amount": value.String(), "status": "ACTIVE", "receivable_id": r.ID, "transfer_id": receipt.ID}, err
	})
}
func (h *Handler) settlements(c *gin.Context) {
	r := h.record(c, "receivables")
	if r == nil {
		return
	}
	h.workflowRows(c, `SELECT jsonb_build_object('id',s.id,'transfer_id',s.transfer_id,'amount',s.amount::text,'currency',s.currency,'status',s.status,'reference',t.details->>'bank_reference','occurred_at',s.occurred_at) FROM accounting_receipt_settlements s JOIN accounting_transfers t ON t.id=s.transfer_id WHERE s.organization_id=$1 AND s.receivable_id=$2 ORDER BY s.created_at DESC`, r.Org, r.ID)
}
func (h *Handler) voidSettlement(c *gin.Context) {
	h.voidWorkflowLink(c, "accounting_receipt_settlements", "settlements")
}
func (h *Handler) voidCost(c *gin.Context) {
	h.voidWorkflowLink(c, "accounting_cost_allocations", "cost-allocations")
}
func (h *Handler) voidWorkflowLink(c *gin.Context, table, kind string) {
	if !idValid(c, c.Param("id")) {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	if safeText(in.Reason) == "" {
		response.ErrorFrom(c, invalid("reason"))
		return
	}
	var org int64
	err := h.engine.db.QueryRowContext(c.Request.Context(), "SELECT organization_id FROM "+table+" WHERE id=$1", c.Param("id")).Scan(&org)
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: org, Kind: kind, Object: c.Param("id"), Action: "void", Payload: in}, func(tx *sql.Tx) (any, error) {
		result, err := tx.ExecContext(c.Request.Context(), "UPDATE "+table+" SET status='VOID',reason=$2,voided_at=NOW() WHERE id=$1 AND status='ACTIVE'", c.Param("id"), in.Reason)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, ErrState
		}
		return gin.H{"id": c.Param("id"), "status": "VOID", "reason": in.Reason}, nil
	})
}
func (h *Handler) allocateCost(c *gin.Context) {
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	org, ok := h.workflowOrg(c, in.OrganizationID)
	if !ok {
		return
	}
	h.run(c, Operation{Org: org, Kind: "cost-allocations", Action: "create", Payload: in}, func(tx *sql.Tx) (any, error) {
		invoice, err := get(c.Request.Context(), tx, "vendor-invoices", in.InvoiceID, org, true)
		if err != nil {
			return nil, err
		}
		if invoice.Status != "CONFIRMED" {
			return nil, ErrState
		}
		if in.SourceKind != "expenses" && in.SourceKind != "vcc-transactions" {
			return nil, invalid("source_kind")
		}
		source, err := get(c.Request.Context(), tx, in.SourceKind, in.SourceID, org, true)
		if err != nil {
			return nil, err
		}
		if source.Status == "DRAFT" || source.Status == "VOID" {
			return nil, ErrState
		}

		identity := in.SourceKind + ":" + source.ID
		limit, err := effective(c.Request.Context(), tx, in.SourceKind, source)
		if err != nil {
			return nil, err
		}
		if in.SourceKind == "vcc-transactions" && isCredit(source) {
			return nil, ErrState
		}
		if in.SourceKind == "expenses" && detail(source, "transaction_id") != "" {
			linked, lookupErr := get(c.Request.Context(), tx, "vcc-transactions", detail(source, "transaction_id"), org, true)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if linked.Status != "CONFIRMED" || isCredit(linked) || linked.Currency != source.Currency {
				return nil, ErrState
			}
			identity = "vcc-transactions:" + linked.ID
			limit = amount(linked)
		}
		if strings.HasPrefix(identity, "vcc-transactions:") {
			refunded, refundErr := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_vcc_transactions WHERE details->>'original_transaction_id'=$1 AND status='CONFIRMED' AND (details->>'transaction_type'='REFUND' OR (details->>'transaction_type'='ADJUSTMENT' AND details->>'direction'='CREDIT'))", identity[len("vcc-transactions:"):])
			if refundErr != nil {
				return nil, refundErr
			}
			limit = limit.Sub(refunded)
		}
		value, err := number(in.Amount, invoice.Currency, true)
		if err != nil {
			return nil, err
		}
		sourceValue := value
		rate := decimal.NewFromInt(1)
		if source.Currency != invoice.Currency {
			sourceValue, err = number(in.SourceAmount, source.Currency, true)
			if err != nil {
				return nil, invalid("source_amount")
			}
			rate, err = parseRate(in.ExchangeRate)
			if err != nil {
				return nil, invalid("exchange_rate")
			}
			if !sourceValue.Mul(rate).Round(int32(payment.CurrencyMaxFractionDigits(invoice.Currency))).Equal(value) {
				return nil, invalid("source_amount")
			}
		} else if in.SourceAmount != "" && !dec(in.SourceAmount).Equal(value) {
			return nil, invalid("source_amount")
		}

		used, err := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_cost_allocations WHERE invoice_id=$1 AND status='ACTIVE'", invoice.ID)
		if err != nil {
			return nil, err
		}
		occupied, err := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(source_amount),0)::text FROM accounting_cost_allocations WHERE cost_identity=$1 AND status='ACTIVE'", identity)
		if err != nil {
			return nil, err
		}

		own, err := sum(c.Request.Context(), tx, "SELECT COALESCE(SUM(source_amount),0)::text FROM accounting_cost_allocations WHERE source_kind=$1 AND source_id=$2 AND status='ACTIVE'", in.SourceKind, source.ID)
		if err != nil {
			return nil, err
		}
		ownLimit, err := effective(c.Request.Context(), tx, in.SourceKind, source)
		if err != nil {
			return nil, err
		}
		if own.Add(sourceValue).GreaterThan(ownLimit) {
			return nil, ErrAmount
		}
		if used.Add(value).GreaterThan(amount(invoice)) || occupied.Add(sourceValue).GreaterThan(limit) {
			return nil, ErrAmount
		}
		occurred, err := workflowTime(in.OccurredAt)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		_, err = tx.ExecContext(c.Request.Context(), `INSERT INTO accounting_cost_allocations(id,organization_id,invoice_id,source_kind,source_id,cost_identity,amount,currency,status,occurred_at,created_by,source_amount,source_currency,exchange_rate) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'ACTIVE',$9,$10,$11,$12,$13)`, id, org, invoice.ID, in.SourceKind, source.ID, identity, value.String(), invoice.Currency, occurred, actor(c), sourceValue.String(), source.Currency, rate.String())
		return gin.H{"id": id, "amount": value.String(), "cost_identity": identity, "status": "ACTIVE"}, err
	})
}
func (h *Handler) costAllocations(c *gin.Context) {
	org, ok := h.workflowOrg(c, c.Query("organization_id"))
	if !ok {
		return
	}
	h.workflowRows(c, `WITH objects AS (`+recordUnion()+`) SELECT jsonb_build_object('id',a.id,'invoice_id',a.invoice_id,'source_kind',a.source_kind,'source_id',a.source_id,'cost_identity',a.cost_identity,'amount',a.amount::text,'currency',a.currency,'source_amount',a.source_amount::text,'source_currency',a.source_currency,'exchange_rate',a.exchange_rate::text,'status',a.status,'occurred_at',a.occurred_at,'invoice_name',i.details->>'vendor','source_name',`+recordNameSQL("source")+`) FROM accounting_cost_allocations a JOIN accounting_vendor_invoices i ON i.id=a.invoice_id JOIN objects source ON source.kind=a.source_kind AND source.id=a.source_id WHERE a.organization_id=$1 ORDER BY a.created_at DESC`, org)
}
func (h *Handler) externalActions(c *gin.Context) {
	org, ok := h.workflowOrg(c, c.Query("organization_id"))
	if !ok {
		return
	}
	h.workflowRows(c, `SELECT jsonb_build_object('id',e.id,'object_kind',e.object_kind,'object_id',e.object_id,'card_id',e.card_id,'action',e.action,'amount',e.amount::text,'currency',e.currency,'status',e.status,'reference',e.reference,'occurred_at',e.occurred_at,'confirmed_at',e.confirmed_at,'reason',e.reason,'card_name',c.details->>'alias','last_four',c.details->>'last_four') FROM accounting_external_actions e JOIN accounting_vcc_cards c ON c.id=e.card_id WHERE e.organization_id=$1 ORDER BY e.created_at DESC`, org)
}
func (h *Handler) confirmExternal(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	if safeText(in.Reference) == "" {
		response.ErrorFrom(c, invalid("reference"))
		return
	}
	var org int64
	err := h.engine.db.QueryRowContext(c.Request.Context(), "SELECT organization_id FROM accounting_external_actions WHERE id=$1", c.Param("id")).Scan(&org)
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: org, Kind: "external-actions", Object: c.Param("id"), Action: "confirm", Payload: in}, func(tx *sql.Tx) (any, error) {
		proof, err := attachment(c, tx, in.AttachmentID, org)
		if err != nil || proof.Status != "COMPLETE" || proof.DeletedAt != nil {
			return nil, ErrProof
		}
		var card string
		var object string
		var kind string
		err = tx.QueryRowContext(c.Request.Context(), "SELECT card_id::text,object_id::text,object_kind FROM accounting_external_actions WHERE id=$1 AND status='PENDING' FOR UPDATE", c.Param("id")).Scan(&card, &object, &kind)
		if err != nil {
			return nil, ErrState
		}
		cardProof := proof.ObjectKind == "vcc-cards" && proof.ObjectID == card
		objectProof := proof.ObjectKind == kind && proof.ObjectID == object
		if !cardProof && !objectProof {
			return nil, ErrProof
		}
		_, err = tx.ExecContext(c.Request.Context(), `UPDATE accounting_external_actions SET status='CONFIRMED',reference=$2,attachment_id=$3,confirmed_at=NOW(),confirmed_by=$4 WHERE id=$1`, c.Param("id"), in.Reference, proof.ID, actor(c))
		return gin.H{"id": c.Param("id"), "status": "CONFIRMED", "reference": in.Reference}, err
	})
}
func (h *Handler) allocateFX(c *gin.Context) {
	r := h.record(c, "fx-exchanges")
	if r == nil {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: "fx-exchanges", Object: r.ID, Action: "allocate", Payload: in}, func(tx *sql.Tx) (any, error) {
		fx, err := get(c.Request.Context(), tx, "fx-exchanges", r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if fx.Status != "CONFIRMED" {
			return nil, ErrState
		}
		target, err := get(c.Request.Context(), tx, "transfers", detail(fx, "target_transfer_id"), r.Org, true)
		if err != nil {
			return nil, err
		}
		return h.engine.Allocate(c.Request.Context(), tx, target, in.SourceID, in.Amount, actor(c))
	})
}
func (h *Handler) workflowRows(c *gin.Context, query string, args ...any) {
	rows, err := h.engine.db.QueryContext(c.Request.Context(), query, args...)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	out := []json.RawMessage{}
	for rows.Next() {
		var body []byte
		if response.ErrorFrom(c, rows.Scan(&body)) {
			return
		}
		out = append(out, json.RawMessage(body))
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	response.Success(c, out)
}

func (h *Handler) costSummary(c *gin.Context) {
	org, ok := h.workflowOrg(c, c.Query("organization_id"))
	if !ok {
		return
	}
	h.workflowRows(c, `WITH lines AS (
 SELECT currency,CASE WHEN details->>'transaction_type'='REFUND' OR (details->>'transaction_type'='ADJUSTMENT' AND details->>'direction'='CREDIT') THEN -amount ELSE amount END AS amount FROM accounting_vcc_transactions WHERE organization_id=$1 AND status='CONFIRMED'
 UNION ALL SELECT currency,amount+COALESCE((SELECT SUM(a.amount) FROM accounting_adjustments a WHERE a.object_kind='expenses' AND a.object_id=e.id),0) FROM accounting_expenses e WHERE organization_id=$1 AND status IN ('UNPAID','PENDING','PAID') AND COALESCE(details->>'transaction_id','')=''),
 costs AS(SELECT currency,SUM(amount) AS amount FROM lines GROUP BY currency),bills AS(SELECT currency,SUM(amount) AS amount FROM accounting_vendor_invoices WHERE organization_id=$1 AND status='CONFIRMED' GROUP BY currency),allocated AS(SELECT currency,SUM(amount) AS amount FROM accounting_cost_allocations WHERE organization_id=$1 AND status='ACTIVE' GROUP BY currency),currencies AS(SELECT currency FROM costs UNION SELECT currency FROM bills UNION SELECT currency FROM allocated)
 SELECT jsonb_build_object('id',c.currency,'currency',c.currency,'book_cost',COALESCE(x.amount,0)::text,'invoice_total',COALESCE(b.amount,0)::text,'allocated_cost',COALESCE(a.amount,0)::text,'unallocated_cost',(COALESCE(b.amount,0)-COALESCE(a.amount,0))::text) FROM currencies c LEFT JOIN costs x ON x.currency=c.currency LEFT JOIN bills b ON b.currency=c.currency LEFT JOIN allocated a ON a.currency=c.currency ORDER BY c.currency`, org)
}

func (h *Handler) exemptExternal(c *gin.Context) {
	if !requireAdmin(c) || !idValid(c, c.Param("id")) {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	if safeText(in.Reason) == "" {
		response.ErrorFrom(c, invalid("reason"))
		return
	}
	var org int64
	var occurred time.Time
	err := h.engine.db.QueryRowContext(c.Request.Context(), "SELECT organization_id,occurred_at FROM accounting_external_actions WHERE id=$1", c.Param("id")).Scan(&org, &occurred)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: org, Kind: "external-actions", Object: c.Param("id"), Action: "void", Payload: in}, func(tx *sql.Tx) (any, error) {
		if err := checkOpenPeriod(c.Request.Context(), tx, org, occurred); err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(c.Request.Context(), "UPDATE accounting_external_actions SET status='VOID',reason=$2,confirmed_at=NOW(),confirmed_by=$3 WHERE id=$1 AND status='PENDING'", c.Param("id"), in.Reason, actor(c))
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, ErrState
		}
		return gin.H{"id": c.Param("id"), "status": "VOID", "reason": in.Reason}, nil
	})
}
