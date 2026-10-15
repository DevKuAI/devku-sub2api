package accounting

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func availableTransfer(ctx context.Context, q dbQuery, r *Record) (decimal.Decimal, error) {
	total, err := effective(ctx, q, "transfers", r)
	if err != nil {
		return decimal.Zero, err
	}
	used, err := transferCommitted(ctx, q, r.ID)
	return total.Sub(used), err
}
func transferCommitted(ctx context.Context, q dbQuery, id string) (decimal.Decimal, error) {
	return sum(ctx, q, `SELECT COALESCE((SELECT SUM(amount-released_amount) FROM accounting_allocations WHERE transfer_id=$1),0)+COALESCE((SELECT SUM(amount+COALESCE(NULLIF(details->>'fee_amount','')::numeric,0)) FROM accounting_fx_exchanges WHERE parent_id=$1 AND status='CONFIRMED'),0)`, id)
}
func validateWorkflowRecord(kind string, r *Record) error {
	switch kind {
	case "receivables":
		if err := required(r, "name", "reference", "due_date"); err != nil {
			return err
		}
		if _, err := time.Parse("2006-01-02", detail(r, "due_date")); err != nil {
			return invalid("due_date")
		}
	case "fx-exchanges":
		if r.ParentID == "" {
			return invalid("parent_id")
		}
		if err := required(r, "target_currency", "target_amount", "exchange_rate", "reference"); err != nil {
			return err
		}
		target, normalizeErr := payment.NormalizePaymentCurrency(detail(r, "target_currency"))
		if normalizeErr != nil {
			return invalid("target_currency")
		}
		r.Details["target_currency"] = target
		if target == r.Currency {
			return invalid("target_currency")
		}
		rate, err := parseRate(detail(r, "exchange_rate"))
		if err != nil {
			return invalid("exchange_rate")
		}
		targetAmount, err := number(detail(r, "target_amount"), target, true)
		if err != nil {
			return invalid("target_amount")
		}
		fee, err := number(detail(r, "fee_amount"), r.Currency, false)
		if err != nil || fee.IsNegative() {
			return invalid("fee_amount")
		}
		calculated := amount(r).Mul(rate)
		// Currency precision is validated by number; the explicit target amount must match the agreed rate.
		if !calculated.Round(int32(payment.CurrencyMaxFractionDigits(target))).Equal(targetAmount) {
			return invalid("target_amount")
		}
	}
	return nil
}
func (e *Engine) confirmWorkflow(ctx context.Context, tx *sql.Tx, kind string, r *Record, actor int64) error {
	if kind == "receivables" {
		r.Status = "CONFIRMED"
		return save(ctx, tx, kind, r)
	}
	if err := e.requireProof(ctx, tx, kind, r); err != nil {
		return err
	}
	source, err := get(ctx, tx, "transfers", r.ParentID, r.Org, true)
	if err != nil {
		return err
	}
	if source.Status != "RECEIVED" || source.Currency != r.Currency {
		return ErrState
	}
	available, err := availableTransfer(ctx, tx, source)
	if err != nil {
		return err
	}
	if available.LessThan(amount(r).Add(dec(detail(r, "fee_amount")))) {
		return ErrAmount
	}
	target := &Record{ID: uuid.NewString(), Org: r.Org, OrganizationID: r.OrganizationID, Status: "RECEIVED", Currency: detail(r, "target_currency"), Amount: detail(r, "target_amount"), OccurredAt: r.OccurredAt, Details: map[string]any{"funding_type": "FX", "source_exchange_id": r.ID, "payer_name": "FX " + detail(r, "reference"), "bank_reference": "FX-" + r.ID, "sales_amount": "0", "received_amount": detail(r, "target_amount")}, Version: 1}
	if err = insertRecord(ctx, tx, "transfers", target, actor); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_reconciliation(id,organization_id,object_kind,object_id,status,reference,reference_amount,currency,reason,created_by) VALUES($1,$2,'transfers',$3,'MATCHED',$4,$5,$6,'FX target funding',$7)`, uuid.NewString(), r.Org, target.ID, r.ID, target.Amount, target.Currency, actor)
	if err != nil {
		return err
	}
	r.Details["target_transfer_id"] = target.ID
	r.Status = "CONFIRMED"
	return save(ctx, tx, kind, r)
}
func workflowDependencies(ctx context.Context, q dbQuery, kind string, r *Record) error {
	var count int
	switch kind {
	case "receivables":
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_receipt_settlements WHERE receivable_id=$1 AND status='ACTIVE'", r.ID).Scan(&count); err != nil {
			return err
		}
	case "transfers":
		if detail(r, "funding_type") == "FX" {
			return ErrState
		}
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_receipt_settlements WHERE transfer_id=$1 AND status='ACTIVE'", r.ID).Scan(&count); err != nil {
			return err
		}
	case "vendor-invoices":
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_cost_allocations WHERE invoice_id=$1 AND status='ACTIVE'", r.ID).Scan(&count); err != nil {
			return err
		}
	case "vcc-transactions", "expenses":
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_cost_allocations WHERE status='ACTIVE' AND ((source_kind=$1 AND source_id=$2) OR cost_identity=$3)", kind, r.ID, "vcc-transactions:"+r.ID).Scan(&count); err != nil {
			return err
		}
	case "fx-exchanges":
		target, err := get(ctx, q, "transfers", detail(r, "target_transfer_id"), r.Org, true)
		if err != nil {
			return err
		}
		used, err := transferCommitted(ctx, q, target.ID)
		if err != nil {
			return err
		}
		if !used.IsZero() {
			return ErrState
		}
		target.Status = "VOID"
		target.Details["void_reason"] = "FX reversal"
		return save(ctx, q, "transfers", target)
	}

	if kind == "vcc-transactions" {
		var linked bool
		if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_expenses WHERE details->>'transaction_id'=$1 AND status<>'VOID')", r.ID).Scan(&linked); err != nil {
			return err
		}
		if linked {
			return ErrState
		}
	}
	if count > 0 {
		return ErrState
	}
	return nil
}
func externalAction(ctx context.Context, q dbQuery, r *Record, kind, action, card string, actor int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO accounting_external_actions(id,organization_id,object_kind,object_id,card_id,action,amount,currency,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())`, uuid.NewString(), r.Org, kind, r.ID, card, action, r.Amount, r.Currency)
	return err
}

func checkCreditCost(ctx context.Context, q dbQuery, r *Record) error {
	original := detail(r, "original_transaction_id")
	if original == "" {
		return nil
	}
	source, err := get(ctx, q, "vcc-transactions", original, r.Org, true)
	if err != nil {
		return err
	}
	occupied, err := sum(ctx, q, "SELECT COALESCE(SUM(source_amount),0)::text FROM accounting_cost_allocations WHERE cost_identity=$1 AND status='ACTIVE'", "vcc-transactions:"+original)
	if err != nil {
		return err
	}
	refunded, err := sum(ctx, q, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_vcc_transactions WHERE details->>'original_transaction_id'=$1 AND status='CONFIRMED' AND (details->>'transaction_type'='REFUND' OR (details->>'transaction_type'='ADJUSTMENT' AND details->>'direction'='CREDIT'))", original)
	if err != nil {
		return err
	}
	if occupied.GreaterThan(amount(source).Sub(refunded).Sub(amount(r))) {
		return ErrAmount.WithMetadata(map[string]string{"field": "cost_allocation_in_use"})
	}
	return nil
}

func parseRate(raw string) (decimal.Decimal, error) {
	if len(raw) == 0 || len(raw) > 64 {
		return decimal.Zero, invalid("exchange_rate")
	}
	rate, err := decimal.NewFromString(raw)
	if err != nil || rate.Exponent() < -12 || rate.Exponent() > 18 || !rate.IsPositive() || rate.NumDigits() > 30 || rate.GreaterThanOrEqual(decimal.New(1, 18)) {
		return decimal.Zero, invalid("exchange_rate")
	}
	return rate, nil
}
