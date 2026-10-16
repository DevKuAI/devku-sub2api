package accounting

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type reconciliationInput struct {
	Kind         string `json:"object_kind"`
	ID           string `json:"object_id"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	Reference    string `json:"reference"`
	Reason       string `json:"reason"`
	AttachmentID string `json:"attachment_id"`
}

func (h *Handler) reconciliationRecord(c *gin.Context) (*Record, reconciliationInput) {
	var in reconciliationInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return nil, in
	}
	if in.Kind != "transfers" && in.Kind != "vcc-transactions" && in.Kind != "expenses" && in.Kind != "receivables" {
		response.ErrorFrom(c, ErrInvalid)
		return nil, in
	}
	if !idValid(c, in.ID) {
		return nil, in
	}
	r, err := get(c.Request.Context(), h.engine.db, in.Kind, in.ID, 0, false)
	if response.ErrorFrom(c, err) {
		return nil, in
	}
	return r, in
}
func (h *Handler) reconcile(c *gin.Context, action string) {
	r, in := h.reconciliationRecord(c)
	if r == nil {
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: in.Kind, Object: r.ID, Action: action, Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := get(c.Request.Context(), tx, in.Kind, r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if current.Status == "DRAFT" || current.Status == "VOID" {
			return nil, ErrState
		}
		if current.Currency != in.Currency {
			return nil, ErrCurrency
		}
		ref, err := number(in.Amount, in.Currency, false)
		if err != nil || ref.IsNegative() {
			return nil, invalid("amount")
		}
		actual, err := effective(c.Request.Context(), tx, in.Kind, current)
		if err != nil {
			return nil, err
		}
		state := "MATCHED"
		if action == "exclude" {
			state = "EXCLUDED"
		} else if !ref.Equal(actual) {
			state = "DIFFERENCE"
		}
		if state != "MATCHED" && safeText(in.Reason) == "" {
			return nil, invalid("reason")
		}
		if in.AttachmentID != "" {
			a, err := attachment(c.Request.Context(), tx, in.AttachmentID, r.Org)
			if err != nil {
				return nil, err
			}
			if a.Status != "COMPLETE" || a.ObjectID != r.ID || a.ObjectKind != in.Kind {
				return nil, ErrProof
			}
		}
		id := uuid.NewString()
		_, err = tx.ExecContext(c.Request.Context(), "INSERT INTO accounting_reconciliation(id,organization_id,object_kind,object_id,status,reference,reference_amount,currency,reason,attachment_id,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid,$11)", id, r.Org, in.Kind, r.ID, state, in.Reference, ref.String(), in.Currency, in.Reason, in.AttachmentID, actor(c))
		return map[string]any{"id": id, "status": state, "object_id": r.ID}, err
	})
}
func (h *Handler) adjust(c *gin.Context) {
	r, in := h.reconciliationRecord(c)
	if r == nil {
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: in.Kind, Object: r.ID, Action: "adjust", Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := get(c.Request.Context(), tx, in.Kind, r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if in.Currency != current.Currency {
			return nil, ErrCurrency
		}
		if in.Kind == "vcc-transactions" {
			return nil, invalid("use_vcc_adjustment_transaction")
		}
		return h.engine.Adjust(c.Request.Context(), tx, in.Kind, current, in.Amount, safeText(in.Reason), actor(c))
	})
}
func (h *Handler) reconciliation(c *gin.Context) {
	f := filter(c)
	state := f.Status
	f.Status = ""
	where, args, err := f.where()
	if response.ErrorFrom(c, err) {
		return
	}
	queries := []string{}
	for _, kind := range []string{"transfers", "vcc-transactions", "expenses"} {
		queries = append(queries, "SELECT "+recordColumns+",'"+kind+"'::text FROM "+specs[kind].table+" r JOIN desktop_organizations o ON o.id=r.organization_id WHERE "+where)
	}
	cte := `WITH objects(id,organization_id,parent_id,status,currency,amount,occurred_at,details,customer_visible,shared_at,shared_by,created_at,version,org,kind) AS (` + strings.Join(queries, " UNION ALL ") + `), results AS (
 SELECT objects.*,COALESCE(c.status,'UNMATCHED') AS reconciliation_status,COALESCE(c.reason,'') AS reason,COALESCE(c.reference,'') AS reference
 FROM objects LEFT JOIN LATERAL(SELECT status,reason,reference FROM accounting_reconciliation WHERE object_kind=objects.kind AND object_id=objects.id::uuid ORDER BY created_at DESC,id DESC LIMIT 1)c ON true
 WHERE (kind='transfers' AND objects.status='RECEIVED') OR (kind='vcc-transactions' AND objects.status='CONFIRMED') OR (kind='expenses' AND objects.status IN ('UNPAID','PENDING','PAID'))) `
	condition := "true"
	if state != "" {
		args = append(args, state)
		condition = fmt.Sprintf("reconciliation_status=$%d", len(args))
	}
	var total int64
	err = h.engine.db.QueryRowContext(c.Request.Context(), cte+"SELECT COUNT(*) FROM results WHERE "+condition, args...).Scan(&total)
	if response.ErrorFrom(c, err) {
		return
	}
	args = append(args, f.Size, (f.Page-1)*f.Size)
	rows, err := h.engine.db.QueryContext(c.Request.Context(), cte+"SELECT * FROM results WHERE "+condition+fmt.Sprintf(" ORDER BY occurred_at DESC,id,kind LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	for rows.Next() {
		r := &Record{}
		var body []byte
		var kind, status, reason, reference string
		err = rows.Scan(&r.ID, &r.OrganizationID, &r.ParentID, &r.Status, &r.Currency, &r.Amount, &r.OccurredAt, &body, &r.CustomerVisible, &r.SharedAt, &r.SharedBy, &r.CreatedAt, &r.Version, &r.Org, &kind, &status, &reason, &reference)
		if response.ErrorFrom(c, err) {
			return
		}
		if err = json.Unmarshal(body, &r.Details); response.ErrorFrom(c, err) {
			return
		}
		out = append(out, map[string]any{"object_kind": kind, "record": r, "status": status, "reason": reason, "reference": reference})
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	response.Paginated(c, out, total, f.Page, f.Size)
}
func (h *Handler) auditLogs(c *gin.Context) {
	f := filter(c)
	where, args, err := f.where()
	if response.ErrorFrom(c, err) {
		return
	}
	_ = where
	_ = args
	org := f.Org
	if f.OrganizationID != "" {
		org, err = h.resolveOrg(c, f.OrganizationID)
		if response.ErrorFrom(c, err) {
			return
		}
	}
	var total int64
	err = h.engine.db.QueryRowContext(c.Request.Context(), "SELECT COUNT(*) FROM accounting_audit_logs WHERE ($1::bigint=0 OR organization_id=$1)", org).Scan(&total)
	if response.ErrorFrom(c, err) {
		return
	}
	rows, err := h.engine.db.QueryContext(c.Request.Context(), "SELECT id,object_kind,object_id,action,actor_id,request_id,summary,created_at FROM accounting_audit_logs WHERE ($1::bigint=0 OR organization_id=$1) ORDER BY id DESC LIMIT $2 OFFSET $3", org, f.Size, (f.Page-1)*f.Size)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []map[string]any{}
	for rows.Next() {
		var id, actor int64
		var kind, object, action, req string
		var summary []byte
		var created any
		if err = rows.Scan(&id, &kind, &object, &action, &actor, &req, &summary, &created); response.ErrorFrom(c, err) {
			return
		}
		items = append(items, map[string]any{"id": id, "object_kind": kind, "object_id": object, "action": action, "actor_id": actor, "request_id": req, "summary": json.RawMessage(summary), "created_at": created})
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	response.Paginated(c, items, total, f.Page, f.Size)
}

func (h *Handler) trend(c *gin.Context) {
	f := filter(c)
	f.Status = "RECEIVED"
	where, args, err := f.where()
	if response.ErrorFrom(c, err) {
		return
	}
	rows, err := h.engine.db.QueryContext(c.Request.Context(), "SELECT to_char(r.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD'),r.currency,SUM(r.amount+COALESCE((SELECT SUM(a.amount) FROM accounting_adjustments a WHERE a.object_kind='transfers' AND a.object_id=r.id),0))::text FROM accounting_transfers r JOIN desktop_organizations o ON o.id=r.organization_id WHERE "+where+" AND COALESCE(r.details->>'funding_type','')='' GROUP BY 1,2 ORDER BY 1,2", args...)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []map[string]string{}
	for rows.Next() {
		var date, currency, amount string
		if err = rows.Scan(&date, &currency, &amount); response.ErrorFrom(c, err) {
			return
		}
		items = append(items, map[string]string{"date": date, "currency": currency, "amount": amount})
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	response.Success(c, items)
}
