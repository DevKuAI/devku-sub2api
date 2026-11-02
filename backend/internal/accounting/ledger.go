package accounting

import (
	"context"
	"database/sql"

	"github.com/shopspring/decimal"
)

// Allocation is a historical receipt-to-card funding row, kept read-only since cards no longer hold balances.
type Allocation struct {
	ID           string `json:"id"`
	CardName     string `json:"card_name,omitempty"`
	TransferName string `json:"transfer_name,omitempty"`
	FundingType  string `json:"funding_type,omitempty"`
	TransferID   string `json:"transfer_id"`
	CardID       string `json:"card_id"`
	Amount       string `json:"amount"`
	Used         string `json:"used_amount"`
	Released     string `json:"released_amount"`
	Closed       bool   `json:"closed"`
}

func sum(ctx context.Context, q dbQuery, query string, args ...any) (decimal.Decimal, error) {
	var raw string
	err := q.QueryRowContext(ctx, query, args...).Scan(&raw)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(raw)
}
func effective(ctx context.Context, q dbQuery, kind string, r *Record) (decimal.Decimal, error) {
	if kind == "expenses" && detail(r, "transaction_id") != "" {
		return linkedCardPaid(ctx, q, detail(r, "transaction_id"))
	}
	delta, err := sum(ctx, q, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_adjustments WHERE object_kind=$1 AND object_id=$2", kind, r.ID)
	return amount(r).Add(delta), err
}
func dec(raw string) decimal.Decimal { d, _ := decimal.NewFromString(raw); return d }

// Cards only attribute spend to an organization; how much money the card provider holds is the channel
// ledger's business. Closing a card therefore just stops new spend and queues the provider confirmation.
func (e *Engine) CloseCard(ctx context.Context, tx *sql.Tx, card *Record) error {
	if card.Status != "ACTIVE" {
		return ErrState
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE parent_id=$1 AND status='DRAFT')", card.ID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrState
	}
	if err := externalAction(ctx, tx, card, "vcc-cards", "CLOSE", card.ID, 0); err != nil {
		return err
	}
	card.Status = "CLOSED"
	return save(ctx, tx, "vcc-cards", card)
}

// cardSpent is a card's confirmed spend net of refunds and credit adjustments.
func cardSpent(ctx context.Context, q dbQuery, id string) (decimal.Decimal, error) {
	return sum(ctx, q, `SELECT COALESCE(SUM(CASE WHEN details->>'transaction_type'='REFUND' OR (details->>'transaction_type'='ADJUSTMENT' AND details->>'direction'='CREDIT') THEN -amount ELSE amount END),0)::text
 FROM accounting_vcc_transactions WHERE parent_id=$1 AND status='CONFIRMED'`, id)
}
func isCredit(r *Record) bool {
	return detail(r, "transaction_type") == "REFUND" || (detail(r, "transaction_type") == "ADJUSTMENT" && detail(r, "direction") == "CREDIT")
}

// checkCredit keeps a refund or credit on the card of the purchase it corrects and within what is left of it.
func checkCredit(ctx context.Context, tx *sql.Tx, r *Record) error {
	original, err := get(ctx, tx, "vcc-transactions", detail(r, "original_transaction_id"), r.Org, true)
	if err != nil {
		return err
	}
	if (detail(r, "transaction_type") == "REFUND" && detail(original, "transaction_type") != "CONSUME") || original.Status != "CONFIRMED" || isCredit(original) || original.ParentID != r.ParentID || original.Currency != r.Currency {
		return ErrState
	}
	left, err := linkedCardPaid(ctx, tx, original.ID)
	if err != nil {
		return err
	}
	if amount(r).GreaterThan(left) {
		return ErrAmount
	}
	return nil
}

// checkUndo refuses to take back a purchase that confirmed refunds or credits still point to.
func checkUndo(ctx context.Context, tx *sql.Tx, r *Record) error {
	var refunds bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE details->>'original_transaction_id'=$1 AND status='CONFIRMED')", r.ID).Scan(&refunds); err != nil {
		return err
	}
	if refunds {
		return ErrState
	}
	return nil
}

func linkedCardPaid(ctx context.Context, q dbQuery, transactionID string) (decimal.Decimal, error) {
	return sum(ctx, q, `SELECT t.amount-COALESCE((SELECT SUM(x.amount) FROM accounting_vcc_transactions x WHERE x.details->>'original_transaction_id'=t.id::text AND x.status='CONFIRMED' AND (x.details->>'transaction_type'='REFUND' OR (x.details->>'transaction_type'='ADJUSTMENT' AND x.details->>'direction'='CREDIT'))),0) FROM accounting_vcc_transactions t WHERE t.id=$1 AND t.status='CONFIRMED'`, transactionID)
}
