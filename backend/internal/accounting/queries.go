package accounting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Filter struct {
	RecordKind                                                   string
	Org                                                          int64
	OrganizationID, Currency, Status, Search, From, To, ParentID string
	Page, Size                                                   int
	Customer                                                     bool
}

func (f Filter) where() (string, []any, error) {
	clauses := []string{"o.deleted_at IS NULL"}
	args := []any{}
	add := func(expr string, v any) {
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf(expr, len(args)))
	}
	if f.Org > 0 {
		add("r.organization_id=$%d", f.Org)
	}
	if f.OrganizationID != "" {
		add("o.public_id=$%d", f.OrganizationID)
	}
	if f.Currency != "" {
		add("r.currency=$%d", f.Currency)
	}
	if f.Status != "" {
		if isReconciliationStatus(f.Status) && f.RecordKind != "" {
			add("COALESCE((SELECT status FROM accounting_reconciliation c WHERE c.object_kind='"+f.RecordKind+"' AND c.object_id=r.id ORDER BY c.created_at DESC,c.id DESC LIMIT 1),'UNMATCHED')=$%d", f.Status)
		} else if !isReconciliationStatus(f.Status) {
			add("r.status=$%d", f.Status)
		}
	}
	if f.ParentID != "" {
		add("r.parent_id=$%d::uuid", f.ParentID)
	}
	if f.Search != "" {
		add("(o.name || ' ' || r.details::text) ILIKE $%d", "%"+f.Search+"%")
	}
	for _, bound := range []struct{ raw, expr string }{{f.From, "r.occurred_at >= $%d"}, {f.To, "r.occurred_at < $%d"}} {
		if bound.raw != "" {
			v, err := time.Parse(time.RFC3339, bound.raw)
			if err != nil {
				return "", nil, invalid("date_range")
			}
			add(bound.expr, v)
		}
	}
	return strings.Join(clauses, " AND "), args, nil
}
func (e *Engine) List(ctx context.Context, kind string, f Filter) ([]*Record, int64, error) {
	f.RecordKind = kind
	s, ok := specs[kind]
	if !ok {
		return nil, 0, ErrNotFound
	}
	where, args, err := f.where()
	if err != nil {
		return nil, 0, err
	}
	if kind == "transfers" && !f.Customer {
		// Card top-ups are our own money; they are listed on the card, not among customer receipts.
		where += " AND COALESCE(r.details->>'funding_type','')<>'TOPUP'"
	}
	if f.Customer {
		where += customerStatusClause(kind)
		if kind == "transfers" {
			where += " AND COALESCE(r.details->>'funding_type','')=''"
		}
		if kind == "contracts" || kind == "invoices" {
			where += " AND r.customer_visible"
		}
		if kind == "invoices" {
			where += " AND r.details->>'invoice_type'='SALES'"
		}
		if kind == "vcc-transactions" {
			where += " AND r.details->>'transaction_type' IN ('CONSUME','REFUND')"
		}
	}
	base := " FROM " + s.table + " r JOIN desktop_organizations o ON o.id=r.organization_id WHERE " + where
	var total int64
	if err = e.db.QueryRowContext(ctx, "SELECT COUNT(*)"+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Size, (f.Page-1)*f.Size)
	columns := recordColumns
	if f.Customer {
		columns = customerColumns(kind)
	}
	rows, err := e.db.QueryContext(ctx, "SELECT "+columns+base+fmt.Sprintf(" ORDER BY r.occurred_at DESC,r.id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	out := []*Record{}
	for rows.Next() {
		r, scanErr := scanRecord(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, 0, scanErr
		}
		out = append(out, r)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, 0, err
	}
	for i, r := range out {
		if err := e.enrich(ctx, kind, r); err != nil {
			return nil, 0, err
		}
		if f.Customer {
			out[i] = r.Public(kind)
		}
	}
	return out, total, nil
}

type CurrencySummary struct {
	Currency        string `json:"currency"`
	Received        string `json:"received"`
	Allocated       string `json:"allocated"`
	Consumed        string `json:"consumed"`
	Refunded        string `json:"refunded"`
	Balance         string `json:"balance"`
	Unallocated     string `json:"unallocated"`
	Unmatched       int64  `json:"unmatched"`
	UnmatchedAmount string `json:"unmatched_amount,omitempty"`
	Expenses        string `json:"expenses,omitempty"`
}

func (e *Engine) Summary(ctx context.Context, f Filter) ([]CurrencySummary, error) {
	f.RecordKind = "transfers"
	where, args, err := f.where()
	if err != nil {
		return nil, err
	}
	kinds := []string{"transfers", "vcc-transactions"}
	if !f.Customer {
		kinds = append(kinds, "expenses")
	}
	queries := []string{}
	for _, kind := range kinds {
		scoped := f
		scoped.RecordKind = kind
		kindWhere, _, err := scoped.where()
		if err != nil {
			return nil, err
		}
		value := "r.amount"
		if kind != "vcc-transactions" {
			value = "r.amount+COALESCE((SELECT SUM(a.amount) FROM accounting_adjustments a WHERE a.object_kind='" + kind + "' AND a.object_id=r.id),0)"
		}
		queries = append(queries, "SELECT '"+kind+"'::text AS kind,r.id,r.status,r.currency,"+value+" AS amount,r.details->>'transaction_type' AS transaction_type,r.details->>'direction' AS direction,COALESCE(r.details->>'funding_type','') AS funding_type FROM "+specs[kind].table+" r JOIN desktop_organizations o ON o.id=r.organization_id WHERE "+kindWhere)
	}
	query := `WITH records AS (` + strings.Join(queries, " UNION ALL ") + `) SELECT r.currency,
 COALESCE(SUM(r.amount) FILTER(WHERE kind='transfers' AND r.status='RECEIVED' AND funding_type=''),0)::text,
 COALESCE(SUM(r.amount) FILTER(WHERE kind='vcc-transactions' AND r.status='CONFIRMED' AND transaction_type NOT IN ('REFUND') AND NOT(transaction_type='ADJUSTMENT' AND direction='CREDIT')),0)::text,
 COALESCE(SUM(r.amount) FILTER(WHERE kind='vcc-transactions' AND r.status='CONFIRMED' AND (transaction_type='REFUND' OR (transaction_type='ADJUSTMENT' AND direction='CREDIT'))),0)::text,
 COALESCE(SUM(r.amount) FILTER(WHERE kind='expenses' AND r.status<>'VOID'),0)::text,
 COUNT(*) FILTER(WHERE ((kind='transfers' AND r.status='RECEIVED') OR (kind='vcc-transactions' AND r.status='CONFIRMED') OR (kind='expenses' AND r.status IN ('UNPAID','PENDING','PAID'))) AND COALESCE((SELECT status FROM accounting_reconciliation c WHERE c.object_kind=r.kind AND c.object_id=r.id ORDER BY c.created_at DESC,c.id DESC LIMIT 1),'UNMATCHED') IN ('UNMATCHED','DIFFERENCE')),
 COALESCE(SUM(r.amount) FILTER(WHERE ((kind='transfers' AND r.status='RECEIVED') OR (kind='vcc-transactions' AND r.status='CONFIRMED') OR (kind='expenses' AND r.status IN ('UNPAID','PENDING','PAID'))) AND COALESCE((SELECT status FROM accounting_reconciliation c WHERE c.object_kind=r.kind AND c.object_id=r.id ORDER BY c.created_at DESC,c.id DESC LIMIT 1),'UNMATCHED') IN ('UNMATCHED','DIFFERENCE')),0)::text
 FROM records r GROUP BY r.currency`
	if f.Customer {
		query = strings.Replace(query, "transaction_type NOT IN ('REFUND') AND NOT(transaction_type='ADJUSTMENT' AND direction='CREDIT')", "transaction_type='CONSUME'", 1)
	}
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	values := map[string]*CurrencySummary{}
	ensure := func(currency string) *CurrencySummary {
		value := values[currency]
		if value == nil {
			value = &CurrencySummary{Currency: currency, Received: "0", Allocated: "0", Consumed: "0", Refunded: "0", Balance: "0", Unallocated: "0", Expenses: "0", UnmatchedAmount: "0"}
			values[currency] = value
		}
		return value
	}
	for rows.Next() {
		var value CurrencySummary
		if err = rows.Scan(&value.Currency, &value.Received, &value.Consumed, &value.Refunded, &value.Expenses, &value.Unmatched, &value.UnmatchedAmount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		value.Allocated = "0"
		value.Balance = "0"
		value.Unallocated = "0"
		values[value.Currency] = &value
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	// Allocation flows use allocation dates, independent of the card creation date.
	allocationQuery := `SELECT r.currency,SUM(r.amount)::text FROM (SELECT t.id,a.organization_id,c.currency,t.status,a.created_at AS occurred_at,c.details||t.details AS details,a.amount-a.released_amount AS amount
 FROM accounting_allocations a JOIN accounting_vcc_cards c ON c.id=a.card_id JOIN accounting_transfers t ON t.id=a.transfer_id WHERE t.status='RECEIVED') r JOIN desktop_organizations o ON o.id=r.organization_id WHERE ` + where + ` GROUP BY r.currency`
	rows, err = e.db.QueryContext(ctx, allocationQuery, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var currency, value string
		if err = rows.Scan(&currency, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ensure(currency).Allocated = value
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	// Available balances are current positions, not period flows. The UI labels this distinction.
	current := f
	current.Status = ""
	current.From = ""
	current.To = ""
	currentWhere, currentArgs, err := current.where()
	if err != nil {
		return nil, err
	}
	balanceQuery := `SELECT r.currency,SUM(COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.card_id=r.id),0)-COALESCE((SELECT SUM(CASE WHEN t.details->>'transaction_type'='REFUND' OR (t.details->>'transaction_type'='ADJUSTMENT' AND t.details->>'direction'='CREDIT') THEN -t.amount ELSE t.amount END) FROM accounting_vcc_transactions t WHERE t.parent_id=r.id AND t.status='CONFIRMED'),0))::text FROM accounting_vcc_cards r JOIN desktop_organizations o ON o.id=r.organization_id WHERE ` + currentWhere + ` GROUP BY r.currency`
	rows, err = e.db.QueryContext(ctx, balanceQuery, currentArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var currency, value string
		if err = rows.Scan(&currency, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ensure(currency).Balance = value
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	unallocatedQuery := `SELECT r.currency,SUM(r.amount+COALESCE((SELECT SUM(a.amount) FROM accounting_adjustments a WHERE a.object_kind='transfers' AND a.object_id=r.id),0)-COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.transfer_id=r.id),0)-COALESCE((SELECT SUM(x.amount+COALESCE(NULLIF(x.details->>'fee_amount','')::numeric,0)) FROM accounting_fx_exchanges x WHERE x.parent_id=r.id AND x.status='CONFIRMED'),0))::text FROM accounting_transfers r JOIN desktop_organizations o ON o.id=r.organization_id WHERE r.status='RECEIVED' AND ` + currentWhere + ` GROUP BY r.currency`
	rows, err = e.db.QueryContext(ctx, unallocatedQuery, currentArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var currency, value string
		if err = rows.Scan(&currency, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ensure(currency).Unallocated = value
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out := []CurrencySummary{}
	for _, value := range values {
		if f.Customer {
			value.Expenses = ""
			value.Unmatched = 0
			value.UnmatchedAmount = ""
		}
		out = append(out, *value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out, nil
}
func (e *Engine) Organizations(ctx context.Context, f Filter) (any, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT o.id,o.public_id,o.name FROM desktop_organizations o WHERE o.deleted_at IS NULL AND ($1='' OR o.name ILIKE '%'||$1||'%' OR o.public_id=$1 OR EXISTS(SELECT 1 FROM accounting_transfers t WHERE t.organization_id=o.id AND t.details::text ILIKE '%'||$1||'%') OR EXISTS(SELECT 1 FROM accounting_vcc_transactions t WHERE t.organization_id=o.id AND t.details::text ILIKE '%'||$1||'%')) ORDER BY o.name LIMIT 100", f.Search)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var public, name string
		if err = rows.Scan(&id, &public, &name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, map[string]any{"id": public, "name": name, "internal_id": id})
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	for _, item := range out {
		sf := f
		sf.Org, _ = item["internal_id"].(int64)
		sf.OrganizationID = ""
		summary, err := e.Summary(ctx, sf)
		if err != nil {
			return nil, err
		}
		item["summary"] = summary
		delete(item, "internal_id")
	}
	return out, nil
}
func maxZero(d decimal.Decimal) decimal.Decimal { return decimal.Max(d, decimal.Zero) }

// The customer query selects only the public metadata whitelist at the database boundary.
func customerColumns(kind string) string {
	fields := specs[kind].public
	pairs := []string{}
	for _, key := range fields {
		pairs = append(pairs, "'"+key+"'", "r.details->'"+key+"'")
	}
	details := "'{}'::jsonb"
	if len(pairs) > 0 {
		details = "jsonb_strip_nulls(jsonb_build_object(" + strings.Join(pairs, ",") + "))"
	}
	value := "r.amount::text"
	if kind == "vcc-cards" {
		value = "'0'::text"
	}
	return "r.id::text,o.public_id,COALESCE(r.parent_id::text,''),r.status,r.currency," + value + ",r.occurred_at," + details + ",r.customer_visible,r.shared_at,NULL::bigint,r.created_at,0::bigint,r.organization_id"
}

// customerStatuses lists the record states an organization account may see.
// Drafts are unconfirmed and voided records were reversed, so both stay internal.
var customerStatuses = map[string][]string{
	"transfers":        {"RECEIVED"},
	"vcc-cards":        {"ACTIVE", "CLOSED"},
	"vcc-transactions": {"CONFIRMED"},
	"contracts":        {"SIGNED", "EXPIRED"},
	"invoices":         {"VERIFIED"},
}

func customerVisibleStatus(kind, status string) bool {
	for _, allowed := range customerStatuses[kind] {
		if status == allowed {
			return true
		}
	}
	return false
}

func customerStatusClause(kind string) string {
	quoted := []string{}
	for _, status := range customerStatuses[kind] {
		quoted = append(quoted, "'"+status+"'")
	}
	if len(quoted) == 0 {
		return " AND FALSE"
	}
	return " AND r.status IN (" + strings.Join(quoted, ",") + ")"
}

func isReconciliationStatus(status string) bool {
	return status == "UNMATCHED" || status == "MATCHED" || status == "DIFFERENCE" || status == "EXCLUDED"
}

func (e *Engine) enrich(ctx context.Context, kind string, r *Record) error {
	if kind == "vcc-cards" {
		balance, err := cardBalance(ctx, e.db, r.ID)
		if err != nil {
			return err
		}
		allocated, err := sum(ctx, e.db, "SELECT COALESCE(SUM(amount-released_amount),0)::text FROM accounting_allocations WHERE card_id=$1", r.ID)
		if err != nil {
			return err
		}
		r.Details["available_balance"] = balance.String()
		r.Details["allocated_amount"] = allocated.String()
		r.Details["has_difference"] = balance.IsNegative()
	}
	if kind == "receivables" {
		settled, err := sum(ctx, e.db, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_receipt_settlements WHERE receivable_id=$1 AND status='ACTIVE'", r.ID)
		if err != nil {
			return err
		}
		r.Details["settled_amount"] = settled.String()
		total, totalErr := effective(ctx, e.db, kind, r)
		if totalErr != nil {
			return totalErr
		}
		r.Details["effective_amount"] = total.String()
		r.Details["outstanding_amount"] = total.Sub(settled).String()
	}
	if kind == "fx-exchanges" && r.Status == "CONFIRMED" {
		target, err := get(ctx, e.db, "transfers", detail(r, "target_transfer_id"), r.Org, false)
		if err != nil {
			return err
		}
		available, err := availableTransfer(ctx, e.db, target)
		if err != nil {
			return err
		}
		r.Details["target_available"] = available.String()
	}

	if kind == "vendor-invoices" {
		allocated, err := sum(ctx, e.db, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_cost_allocations WHERE invoice_id=$1 AND status='ACTIVE'", r.ID)
		if err != nil {
			return err
		}
		r.Details["allocated_cost"] = allocated.String()
		r.Details["unallocated_cost"] = amount(r).Sub(allocated).String()
	}
	if kind == "transfers" || kind == "expenses" {
		v, err := effective(ctx, e.db, kind, r)
		if err != nil {
			return err
		}
		r.Details["effective_amount"] = v.String()
		r.Details["original_amount"] = r.Amount
		r.Details["adjustment_amount"] = v.Sub(amount(r)).String()
		if kind == "transfers" {
			r.Details["received_amount"] = v.String()
		}
		if kind == "transfers" {
			available, err := availableTransfer(ctx, e.db, r)
			if err != nil {
				return err
			}
			r.Details["unallocated_amount"] = available.String()
		}

		if kind == "expenses" {
			paid, err := paidAmount(ctx, e.db, r.ID)
			if err != nil {
				return err
			}
			r.Details["paid_amount"] = paid.String()
			r.Details["unpaid_amount"] = decimal.Max(v.Sub(paid), decimal.Zero).String()
			r.Details["overpaid_amount"] = decimal.Max(paid.Sub(v), decimal.Zero).String()
		}
	}
	if orderKinds[kind] {
		var orderID, orderName string
		err := e.db.QueryRowContext(ctx, "SELECT a.id::text,a.name FROM accounting_order_items i JOIN accounting_orders a ON a.id=i.order_id WHERE i.object_kind=$1 AND i.object_id=$2", kind, r.ID).Scan(&orderID, &orderName)
		if err == nil {
			r.Details["order_id"] = orderID
			r.Details["order_name"] = orderName
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}
