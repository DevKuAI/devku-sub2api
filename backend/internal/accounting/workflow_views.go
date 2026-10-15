package accounting

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func recordUnion() string {
	parts := []string{}
	for kind, s := range specs {
		parts = append(parts, fmt.Sprintf("SELECT '%s'::text AS kind,r.id,r.organization_id,r.status,r.currency,r.amount,r.occurred_at,r.details,r.parent_id,r.customer_visible,r.created_at,r.version FROM %s r", kind, s.table))
	}
	return strings.Join(parts, " UNION ALL ")
}
func recordNameSQL(prefix string) string {
	return "COALESCE(NULLIF(" + prefix + ".details->>'alias',''),NULLIF(" + prefix + ".details->>'name',''),NULLIF(" + prefix + ".details->>'payer_name',''),NULLIF(" + prefix + ".details->>'merchant',''),NULLIF(" + prefix + ".details->>'vendor',''),NULLIF(" + prefix + ".details->>'supplier',''),NULLIF(" + prefix + ".details->>'invoice_number',''),NULLIF(" + prefix + ".details->>'reference','')," + prefix + ".id::text)"
}
func (h *Handler) workbench(c *gin.Context) {
	org, ok := h.workflowOrg(c, c.Query("organization_id"))
	if !ok {
		return
	}
	cte := `WITH objects AS (` + recordUnion() + `), tasks AS (
 SELECT r.kind,r.id,r.currency,r.amount,` + recordNameSQL("r") + ` AS name,r.status,'draft'::text AS reason FROM objects r WHERE r.organization_id=$1 AND r.status='DRAFT' AND COALESCE(r.details->>'funding_type','')<>'FX'
 UNION ALL SELECT r.kind,r.id,r.currency,r.amount,` + recordNameSQL("r") + `,r.status,'proof_missing' FROM objects r WHERE r.organization_id=$1 AND r.status='DRAFT' AND r.kind IN ('transfers','contracts','invoices','vendor-invoices','fx-exchanges') AND NOT EXISTS(SELECT 1 FROM accounting_attachments a WHERE a.object_kind=r.kind AND a.object_id=r.id AND a.status='COMPLETE' AND a.deleted_at IS NULL)
 UNION ALL SELECT r.kind,r.id,r.currency,((r.amount+COALESCE((SELECT SUM(z.amount) FROM accounting_adjustments z WHERE z.object_kind=r.kind AND z.object_id=r.id),0))-COALESCE((SELECT SUM(z.amount) FROM accounting_expense_payments z WHERE z.parent_id=r.id AND z.status='PAID'),0)),` + recordNameSQL("r") + `,r.status,'unpaid' FROM objects r WHERE r.organization_id=$1 AND r.kind='expenses' AND r.status IN ('UNPAID','PENDING') AND r.amount+COALESCE((SELECT SUM(a.amount) FROM accounting_adjustments a WHERE a.object_kind='expenses' AND a.object_id=r.id),0)>COALESCE((SELECT SUM(p.amount) FROM accounting_expense_payments p WHERE p.parent_id=r.id AND p.status='PAID'),0)
 UNION ALL SELECT r.kind,r.id,r.currency,r.amount,` + recordNameSQL("r") + `,r.status,'missing_invoice' FROM objects r WHERE r.organization_id=$1 AND r.status='DRAFT' AND r.kind IN ('expenses','vcc-transactions') AND (r.details->>'ai_vendor'='true' OR r.details->>'category'='AI_VENDOR') AND NOT EXISTS(SELECT 1 FROM accounting_vendor_invoices i WHERE i.id::text=r.details->>'vendor_invoice_id' AND i.status='CONFIRMED' AND i.organization_id=$1)
 UNION ALL SELECT r.kind,r.id,r.currency,((r.amount+COALESCE((SELECT SUM(z.amount) FROM accounting_adjustments z WHERE z.object_kind=r.kind AND z.object_id=r.id),0))-COALESCE((SELECT SUM(z.amount) FROM accounting_receipt_settlements z WHERE z.receivable_id=r.id AND z.status='ACTIVE'),0)),` + recordNameSQL("r") + `,r.status,'receivable_overdue' FROM objects r WHERE r.organization_id=$1 AND r.kind='receivables' AND r.status='CONFIRMED' AND (r.details->>'due_date')::date<(NOW() AT TIME ZONE 'Asia/Shanghai')::date AND (r.amount+COALESCE((SELECT SUM(z.amount) FROM accounting_adjustments z WHERE z.object_kind=r.kind AND z.object_id=r.id),0))>COALESCE((SELECT SUM(s.amount) FROM accounting_receipt_settlements s WHERE s.receivable_id=r.id AND s.status='ACTIVE'),0)
 UNION ALL SELECT r.kind,r.id,r.currency,(r.amount-COALESCE((SELECT SUM(z.amount) FROM accounting_cost_allocations z WHERE z.invoice_id=r.id AND z.status='ACTIVE'),0)),` + recordNameSQL("r") + `,r.status,'invoice_unallocated' FROM objects r WHERE r.organization_id=$1 AND r.kind='vendor-invoices' AND r.status='CONFIRMED' AND r.amount>COALESCE((SELECT SUM(a.amount) FROM accounting_cost_allocations a WHERE a.invoice_id=r.id AND a.status='ACTIVE'),0)
 UNION ALL SELECT r.kind,r.id,r.currency,((r.amount+COALESCE((SELECT SUM(z.amount) FROM accounting_adjustments z WHERE z.object_kind=r.kind AND z.object_id=r.id),0))),` + recordNameSQL("r") + `,r.status,'difference' FROM objects r WHERE r.organization_id=$1 AND r.status NOT IN ('DRAFT','VOID') AND (SELECT x.status FROM accounting_reconciliation x WHERE x.object_kind=r.kind AND x.object_id=r.id ORDER BY x.created_at DESC,x.id DESC LIMIT 1)='DIFFERENCE'
 UNION ALL SELECT r.kind,r.id,r.currency,((r.amount+COALESCE((SELECT SUM(z.amount) FROM accounting_adjustments z WHERE z.object_kind=r.kind AND z.object_id=r.id),0))-COALESCE((SELECT SUM(z.amount-z.released_amount) FROM accounting_allocations z WHERE z.transfer_id=r.id),0)-COALESCE((SELECT SUM(z.amount+COALESCE(NULLIF(z.details->>'fee_amount','')::numeric,0)) FROM accounting_fx_exchanges z WHERE z.parent_id=r.id AND z.status='CONFIRMED'),0)),` + recordNameSQL("r") + `,r.status,'unallocated' FROM objects r WHERE r.organization_id=$1 AND r.kind='transfers' AND r.status='RECEIVED' AND r.amount+COALESCE((SELECT SUM(x.amount) FROM accounting_adjustments x WHERE x.object_kind='transfers' AND x.object_id=r.id),0)-COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.transfer_id=r.id),0)-COALESCE((SELECT SUM(x.amount+COALESCE(NULLIF(x.details->>'fee_amount','')::numeric,0)) FROM accounting_fx_exchanges x WHERE x.parent_id=r.id AND x.status='CONFIRMED'),0)>0
 UNION ALL SELECT r.kind,r.id,r.currency,(COALESCE((SELECT SUM(z.amount-z.released_amount) FROM accounting_allocations z WHERE z.card_id=r.id),0)-COALESCE((SELECT SUM(CASE WHEN z.details->>'transaction_type'='REFUND' OR (z.details->>'transaction_type'='ADJUSTMENT' AND z.details->>'direction'='CREDIT') THEN -z.amount ELSE z.amount END) FROM accounting_vcc_transactions z WHERE z.parent_id=r.id AND z.status='CONFIRMED'),0)),` + recordNameSQL("r") + `,r.status,'overdraft' FROM objects r WHERE r.organization_id=$1 AND r.kind='vcc-cards' AND COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.card_id=r.id),0)<COALESCE((SELECT SUM(CASE WHEN t.details->>'transaction_type'='REFUND' OR (t.details->>'transaction_type'='ADJUSTMENT' AND t.details->>'direction'='CREDIT') THEN -t.amount ELSE t.amount END) FROM accounting_vcc_transactions t WHERE t.parent_id=r.id AND t.status='CONFIRMED'),0)
 UNION ALL SELECT 'external-actions',e.id,e.currency,e.amount,c.details->>'alias',e.status,'external_pending' FROM accounting_external_actions e JOIN accounting_vcc_cards c ON c.id=e.card_id WHERE e.organization_id=$1 AND e.status='PENDING'
 ) `
	rows, err := h.engine.db.QueryContext(c.Request.Context(), cte+`SELECT reason,COUNT(*) FROM tasks GROUP BY reason`, org)
	if response.ErrorFrom(c, err) {
		return
	}
	counts := map[string]int64{}
	var total int64
	for rows.Next() {
		var key string
		var count int64
		if err = rows.Scan(&key, &count); err != nil {
			break
		}
		counts[key] = count
		total += count
	}
	closeErr := rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err == nil {
		err = closeErr
	}
	if response.ErrorFrom(c, err) {
		return
	}
	rows, err = h.engine.db.QueryContext(c.Request.Context(), cte+`SELECT kind,id::text,currency,amount::text,name,status,reason FROM tasks WHERE ($2='' OR reason=$2) ORDER BY reason,kind,id LIMIT 200`, org, c.Query("reason"))
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var kind, id, currency, amount, name, status, reason string
		if response.ErrorFrom(c, rows.Scan(&kind, &id, &currency, &amount, &name, &status, &reason)) {
			return
		}
		items = append(items, gin.H{"kind": kind, "id": id, "currency": currency, "amount": amount, "name": name, "status": status, "reason": reason})
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	positions, err := currentPositions(c.Request.Context(), h.engine.db, org)
	if response.ErrorFrom(c, err) {
		return
	}
	pending := []gin.H{}
	rows2, err := h.engine.db.QueryContext(c.Request.Context(), `SELECT currency,SUM(amount)::text FROM accounting_vcc_transactions WHERE organization_id=$1 AND status='DRAFT' AND details->>'transaction_type' IN ('CONSUME','FEE') GROUP BY currency`, org)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows2.Close() }()
	for rows2.Next() {
		var currency, value string
		if response.ErrorFrom(c, rows2.Scan(&currency, &value)) {
			return
		}
		pending = append(pending, gin.H{"currency": currency, "amount": value})
	}
	if response.ErrorFrom(c, rows2.Err()) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "matched_total": func() int64 {
		if c.Query("reason") == "" {
			return total
		}
		return counts[c.Query("reason")]
	}(), "counts": counts, "positions": positions, "pending_consumption": pending})
}
func (h *Handler) activity(c *gin.Context) {
	kind := c.Param("kind")
	if _, ok := specs[kind]; !ok {
		response.ErrorFrom(c, ErrNotFound)
		return
	}
	r := h.record(c, kind)
	if r == nil {
		return
	}
	h.workflowRows(c, `SELECT jsonb_build_object('id',id::text,'action',action,'actor_id',actor_id,'request_id',request_id,'summary',summary,'created_at',created_at) FROM accounting_audit_logs WHERE organization_id=$1 AND ((object_kind=$2 AND object_id=$3) OR (object_kind='attachments' AND object_id IN (SELECT id::text FROM accounting_attachments WHERE organization_id=$1 AND object_kind=$2 AND object_id=$3::uuid)) OR (object_kind='external-actions' AND object_id IN (SELECT id::text FROM accounting_external_actions WHERE organization_id=$1 AND card_id=$3::uuid)) OR (action='allocate' AND (summary->>'card_id'=$3 OR summary->>'source_id'=$3))) ORDER BY created_at DESC,id DESC`, r.Org, kind, r.ID)
}
func (h *Handler) related(c *gin.Context) {
	kind := c.Param("kind")
	if _, ok := specs[kind]; !ok {
		response.ErrorFrom(c, ErrNotFound)
		return
	}
	r := h.record(c, kind)
	if r == nil {
		return
	}
	ids := []string{r.ID}
	if r.ParentID != "" {
		ids = append(ids, r.ParentID)
	}
	for _, key := range []string{"vendor_invoice_id", "invoice_id", "contract_id", "transaction_id", "expense_id", "original_transaction_id", "target_transfer_id", "source_exchange_id"} {
		if id := detail(r, key); id != "" {
			ids = append(ids, id)
		}
	}
	raw, err := json.Marshal(ids)
	if response.ErrorFrom(c, err) {
		return
	}
	h.workflowRows(c, `WITH objects AS (`+recordUnion()+`) SELECT jsonb_build_object('kind',r.kind,'id',r.id,'organization_id',$4::text,'name',`+recordNameSQL("r")+`,'amount',r.amount::text,'currency',r.currency,'status',r.status,'occurred_at',r.occurred_at) FROM objects r WHERE r.organization_id=$1 AND r.id<>$2::uuid AND (r.id::text IN (SELECT jsonb_array_elements_text($3::jsonb)) OR r.parent_id=$2::uuid OR EXISTS(SELECT 1 FROM jsonb_each_text(r.details) f WHERE f.key IN ('vendor_invoice_id','invoice_id','contract_id','transaction_id','expense_id','original_transaction_id','source_exchange_id','target_transfer_id') AND f.value=$2::text) OR EXISTS(SELECT 1 FROM accounting_allocations a WHERE (a.transfer_id=$2::uuid AND a.card_id=r.id) OR (a.card_id=$2::uuid AND a.transfer_id=r.id)) OR EXISTS(SELECT 1 FROM accounting_receipt_settlements s WHERE s.status='ACTIVE' AND ((s.receivable_id=$2::uuid AND s.transfer_id=r.id) OR (s.transfer_id=$2::uuid AND s.receivable_id=r.id))) OR EXISTS(SELECT 1 FROM accounting_cost_allocations a WHERE a.status='ACTIVE' AND ((a.invoice_id=$2::uuid AND a.source_id=r.id) OR (a.source_id=$2::uuid AND a.invoice_id=r.id)))) ORDER BY r.occurred_at DESC,r.id`, r.Org, r.ID, string(raw), r.OrganizationID)
}
