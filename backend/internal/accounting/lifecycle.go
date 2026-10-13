package accounting

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func (e *Engine) requireProof(ctx context.Context, tx dbQuery, kind string, r *Record) error {
	if e.storage == nil || !e.storage.Ready() {
		return ErrStorage
	}
	var exists bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_attachments WHERE object_kind=$1 AND object_id=$2 AND organization_id=$3 AND status='COMPLETE' AND deleted_at IS NULL)", kind, r.ID, r.Org).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProof
	}
	return nil
}
func (e *Engine) requireVendorInvoice(ctx context.Context, tx dbQuery, r *Record) error {
	if !flag(r.Details, "ai_vendor") && detail(r, "category") != "AI_VENDOR" {
		return nil
	}
	invoice, err := get(ctx, tx, "vendor-invoices", detail(r, "vendor_invoice_id"), r.Org, false)
	if err != nil {
		return ErrProof
	}
	if invoice.Status != "CONFIRMED" {
		return ErrProof
	}
	return e.requireProof(ctx, tx, "vendor-invoices", invoice)
}
func (e *Engine) Confirm(ctx context.Context, tx *sql.Tx, kind string, r *Record) error {
	if r.Status != "DRAFT" {
		return ErrState
	}
	switch kind {
	case "transfers":
		if err := e.requireProof(ctx, tx, kind, r); err != nil {
			return err
		}
		r.Status = "RECEIVED"
	case "contracts":
		if err := e.requireProof(ctx, tx, kind, r); err != nil {
			return err
		}
		r.Status = "SIGNED"
	case "invoices":
		if err := e.requireProof(ctx, tx, kind, r); err != nil {
			return err
		}
		r.Status = "VERIFIED"
	case "vendor-invoices":
		if err := e.requireProof(ctx, tx, kind, r); err != nil {
			return err
		}
		r.Status = "CONFIRMED"
	case "vcc-transactions":
		if err := e.checkRelations(ctx, tx, kind, r); err != nil {
			return err
		}
		if err := e.requireVendorInvoice(ctx, tx, r); err != nil {
			return err
		}
		if err := rebalanceFunding(ctx, tx, r.ParentID); err != nil {
			return err
		}
		if isCredit(r) {
			if err := credit(ctx, tx, r); err != nil {
				return err
			}
		} else {
			if err := debit(ctx, tx, r); err != nil {
				return err
			}
		}
		r.Status = "CONFIRMED"
	default:
		return ErrState
	}
	if err := save(ctx, tx, kind, r); err != nil {
		return err
	}
	if kind == "vcc-transactions" {
		if err := rebalanceFunding(ctx, tx, r.ParentID); err != nil {
			return err
		}
		return settleClosedCard(ctx, tx, r.ParentID)
	}
	return nil
}
func (e *Engine) Void(ctx context.Context, tx *sql.Tx, kind string, r *Record, reason string, actor int64) error {
	if reason == "" {
		return invalid("reason")
	}
	if r.Status == "VOID" || kind == "vcc-cards" {
		return ErrState
	}
	if r.Status != "DRAFT" {
		switch kind {
		case "transfers":
			used, err := sum(ctx, tx, "SELECT COALESCE(SUM(amount-released_amount),0)::text FROM accounting_allocations WHERE transfer_id=$1", r.ID)
			if err != nil {
				return err
			}
			if !used.IsZero() {
				return ErrState
			}
		case "vcc-transactions":
			if err := undoTransaction(ctx, tx, r); err != nil {
				return err
			}
		case "expenses":
			paid, err := paidAmount(ctx, tx, r.ID)
			if err != nil {
				return err
			}
			if !paid.IsZero() {
				return ErrState
			}
		case "vendor-invoices", "invoices", "contracts":
			var linked bool
			field := map[string]string{"vendor-invoices": "vendor_invoice_id", "invoices": "invoice_id", "contracts": "contract_id"}[kind]
			err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_expenses WHERE details->>$2=$1 AND status IN ('PENDING','PAID')) OR EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE details->>$2=$1 AND status='CONFIRMED')`, r.ID, field).Scan(&linked)
			if err != nil {
				return err
			}
			if linked {
				return ErrState
			}
		}
	}
	if r.Status != "DRAFT" {
		value, err := effective(ctx, tx, kind, r)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO accounting_reversals(id,organization_id,object_kind,object_id,amount,currency,reason,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", uuid.NewString(), r.Org, kind, r.ID, value.Neg().String(), r.Currency, reason, actor)
		if err != nil {
			return err
		}
	}
	r.Status = "VOID"
	r.Details["void_reason"] = reason
	if err := save(ctx, tx, kind, r); err != nil {
		return err
	}
	if kind == "vcc-transactions" {
		if err := rebalanceFunding(ctx, tx, r.ParentID); err != nil {
			return err
		}
		return settleClosedCard(ctx, tx, r.ParentID)
	}
	if kind == "expense-payments" {
		expense, err := get(ctx, tx, "expenses", r.ParentID, r.Org, true)
		if err != nil {
			return err
		}
		return updateExpenseStatus(ctx, tx, expense)
	}
	return nil
}
func paidAmount(ctx context.Context, q dbQuery, id string) (decimal.Decimal, error) {
	return sum(ctx, q, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_expense_payments WHERE parent_id=$1 AND status='PAID'", id)
}
func updateExpenseStatus(ctx context.Context, tx dbQuery, r *Record) error {
	paid, err := paidAmount(ctx, tx, r.ID)
	if err != nil {
		return err
	}
	total, err := effective(ctx, tx, "expenses", r)
	if err != nil {
		return err
	}
	r.Status = "DRAFT"
	if paid.IsPositive() {
		r.Status = "PENDING"
	}
	if paid.GreaterThanOrEqual(total) {
		r.Status = "PAID"
	}
	return save(ctx, tx, "expenses", r)
}
func (e *Engine) Pay(ctx context.Context, tx *sql.Tx, expense *Record, in Input, actor int64) (*Record, error) {
	if expense.Status == "VOID" {
		return nil, ErrState
	}
	if err := e.requireVendorInvoice(ctx, tx, expense); err != nil {
		return nil, err
	}
	// Payment proofs are uploaded to the expense before recording the payment.
	if err := e.requireProof(ctx, tx, "expenses", expense); err != nil {
		return nil, err
	}
	proof, err := attachment(ctx, tx, text(in.Details, "proof_attachment_id"), expense.Org)
	if err != nil || proof.Status != "COMPLETE" || proof.ObjectID != expense.ID || proof.ObjectKind != "expenses" {
		return nil, ErrProof
	}
	in.ParentID = expense.ID
	in.OrganizationID = expense.OrganizationID
	if in.Currency == "" {
		in.Currency = expense.Currency
	}
	r, err := prepare("expense-payments", in, nil)
	if err != nil {
		return nil, err
	}
	r.Org = expense.Org
	r.Status = "PAID"
	if r.Currency != expense.Currency {
		return nil, ErrCurrency
	}
	paid, err := paidAmount(ctx, tx, expense.ID)
	if err != nil {
		return nil, err
	}
	total, err := effective(ctx, tx, "expenses", expense)
	if err != nil {
		return nil, err
	}
	if paid.Add(amount(r)).GreaterThan(total) && (!flag(r.Details, "override") || detail(r, "reason") == "") {
		return nil, ErrAmount
	}
	if err = insertRecord(ctx, tx, "expense-payments", r, actor); err != nil {
		return nil, err
	}
	if err = updateExpenseStatus(ctx, tx, expense); err != nil {
		return nil, err
	}
	return r, nil
}
func (e *Engine) Adjust(ctx context.Context, tx *sql.Tx, kind string, r *Record, raw, reason string, actor int64) (any, error) {
	if reason == "" {
		return nil, invalid("reason")
	}
	if r.Status == "DRAFT" || r.Status == "VOID" {
		return nil, ErrState
	}
	delta, err := number(raw, r.Currency, false)
	if err != nil || delta.IsZero() {
		return nil, invalid("amount")
	}
	if kind != "transfers" && kind != "expenses" {
		return nil, invalid("object_kind")
	}
	total, err := effective(ctx, tx, kind, r)
	if err != nil {
		return nil, err
	}
	next := total.Add(delta)
	if !next.IsPositive() {
		return nil, ErrAmount
	}
	var used decimal.Decimal
	if kind == "transfers" {
		used, err = sum(ctx, tx, "SELECT COALESCE(SUM(amount-released_amount),0)::text FROM accounting_allocations WHERE transfer_id=$1", r.ID)
	} else {
		used, err = paidAmount(ctx, tx, r.ID)
	}
	if err != nil {
		return nil, err
	}
	if next.LessThan(used) {
		return nil, ErrAmount
	}
	id := uuid.NewString()
	_, err = tx.ExecContext(ctx, "INSERT INTO accounting_adjustments(id,organization_id,object_kind,object_id,amount,currency,reason,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", id, r.Org, kind, r.ID, delta.String(), r.Currency, reason, actor)
	if err != nil {
		return nil, err
	}
	if kind == "expenses" {
		if err = updateExpenseStatus(ctx, tx, r); err != nil {
			return nil, err
		}
	}
	return map[string]any{"id": id, "object_id": r.ID, "amount": delta.String(), "effective_amount": next.String(), "reason": reason}, nil
}
