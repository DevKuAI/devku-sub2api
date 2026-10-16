package accounting

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
func allocations(ctx context.Context, q dbQuery, card string) ([]Allocation, error) {
	rows, err := q.QueryContext(ctx, "SELECT id::text,transfer_id::text,card_id::text,amount::text,used_amount::text,released_amount::text,closed_at IS NOT NULL FROM accounting_allocations WHERE card_id=$1 ORDER BY created_at,id", card)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Allocation{}
	for rows.Next() {
		var a Allocation
		if err = rows.Scan(&a.ID, &a.TransferID, &a.CardID, &a.Amount, &a.Used, &a.Released, &a.Closed); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func dec(raw string) decimal.Decimal { d, _ := decimal.NewFromString(raw); return d }
func (e *Engine) Allocate(ctx context.Context, tx *sql.Tx, transfer *Record, cardID, raw string, actor int64) (*Allocation, error) {
	return e.allocate(ctx, tx, transfer, cardID, raw, actor, true)
}

// allocate funds a card from a transfer. external=false is for funding that already happened at the
// provider (card top-ups, spend recorded in an order), so no provider confirmation is queued.
func (e *Engine) allocate(ctx context.Context, tx *sql.Tx, transfer *Record, cardID, raw string, actor int64, external bool) (*Allocation, error) {
	if transfer.Status != "RECEIVED" {
		return nil, ErrState
	}
	card, err := get(ctx, tx, "vcc-cards", cardID, transfer.Org, true)
	if err != nil {
		return nil, err
	}
	if card.Status != "ACTIVE" {
		return nil, ErrState
	}
	if card.Currency != transfer.Currency {
		return nil, ErrCurrency
	}
	d, err := number(raw, transfer.Currency, true)
	if err != nil {
		return nil, err
	}
	used, err := transferCommitted(ctx, tx, transfer.ID)
	if err != nil {
		return nil, err
	}
	total, err := effective(ctx, tx, "transfers", transfer)
	if err != nil {
		return nil, err
	}
	if used.Add(d).GreaterThan(total) {
		return nil, ErrAmount
	}
	a := &Allocation{ID: uuid.NewString(), TransferID: transfer.ID, CardID: card.ID, Amount: d.String(), Used: "0", Released: "0"}
	_, err = tx.ExecContext(ctx, "INSERT INTO accounting_allocations(id,organization_id,transfer_id,card_id,amount,created_by) VALUES($1,$2,$3,$4,$5,$6)", a.ID, transfer.Org, transfer.ID, card.ID, a.Amount, actor)
	if err != nil {
		return nil, err
	}
	if err = coverOverdraft(ctx, tx, a); err != nil {
		return nil, err
	}
	if !external {
		return a, nil
	}
	event := *transfer
	event.ID = a.ID
	event.Amount = a.Amount
	if err = externalAction(ctx, tx, &event, "allocations", "FUND", card.ID, actor); err != nil {
		return nil, err
	}
	return a, nil
}
func cardBalance(ctx context.Context, q dbQuery, id string) (decimal.Decimal, error) {
	return sum(ctx, q, `SELECT (COALESCE((SELECT SUM(amount-released_amount) FROM accounting_allocations WHERE card_id=$1),0)
 - COALESCE((SELECT SUM(CASE WHEN details->>'transaction_type'='REFUND' OR (details->>'transaction_type'='ADJUSTMENT' AND details->>'direction'='CREDIT') THEN -amount ELSE amount END)
 FROM accounting_vcc_transactions WHERE parent_id=$1 AND status='CONFIRMED'),0))::text`, id)
}
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
	if err := rebalanceFunding(ctx, tx, card.ID); err != nil {
		return err
	}
	balance, err := cardBalance(ctx, tx, card.ID)
	if err != nil {
		return err
	}
	if balance.IsPositive() {
		list, err := allocations(ctx, tx, card.ID)
		if err != nil {
			return err
		}
		for _, a := range list {
			release := decimal.Min(balance, dec(a.Amount).Sub(dec(a.Used)).Sub(dec(a.Released)))
			if release.IsPositive() {
				if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET released_amount=released_amount+$2 WHERE id=$1", a.ID, release.String()); err != nil {
					return err
				}
				balance = balance.Sub(release)
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET closed_at=NOW() WHERE card_id=$1 AND closed_at IS NULL", card.ID); err != nil {
		return err
	}
	if err := externalAction(ctx, tx, card, "vcc-cards", "CLOSE", card.ID, 0); err != nil {
		return err
	}
	card.Status = "CLOSED"
	return save(ctx, tx, "vcc-cards", card)
}
func isCredit(r *Record) bool {
	return detail(r, "transaction_type") == "REFUND" || (detail(r, "transaction_type") == "ADJUSTMENT" && detail(r, "direction") == "CREDIT")
}
func debit(ctx context.Context, tx *sql.Tx, r *Record) error {
	list, err := allocations(ctx, tx, r.ParentID)
	if err != nil {
		return err
	}
	remaining := amount(r)
	for _, a := range list {
		if a.Closed {
			continue
		}
		available := dec(a.Amount).Sub(dec(a.Used)).Sub(dec(a.Released))
		part := decimal.Min(remaining, available)
		if !part.IsPositive() {
			continue
		}
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET used_amount=used_amount+$2 WHERE id=$1", a.ID, part.String()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO accounting_funding(transaction_id,allocation_id,amount) VALUES($1,$2,$3)", r.ID, a.ID, part.String()); err != nil {
			return err
		}
		remaining = remaining.Sub(part)
		if remaining.IsZero() {
			break
		}
	}
	if remaining.IsPositive() {
		_, err = tx.ExecContext(ctx, "INSERT INTO accounting_funding(transaction_id,amount) VALUES($1,$2)", r.ID, remaining.String())
	}
	return err
}

type fundingPart struct {
	id         int64
	allocation sql.NullString
	available  decimal.Decimal
}

func credit(ctx context.Context, tx *sql.Tx, r *Record) error {
	original, err := get(ctx, tx, "vcc-transactions", detail(r, "original_transaction_id"), r.Org, true)
	if err != nil {
		return err
	}
	if (detail(r, "transaction_type") == "REFUND" && detail(original, "transaction_type") != "CONSUME") || original.Status != "CONFIRMED" || isCredit(original) || original.ParentID != r.ParentID || original.Currency != r.Currency {
		return ErrState
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,allocation_id::text,(amount-refunded_amount)::text FROM accounting_funding WHERE transaction_id=$1 ORDER BY allocation_id IS NOT NULL,id FOR UPDATE", original.ID)
	if err != nil {
		return err
	}
	parts := []fundingPart{}
	total := decimal.Zero
	for rows.Next() {
		var part fundingPart
		var raw string
		if err = rows.Scan(&part.id, &part.allocation, &raw); err != nil {
			_ = rows.Close()
			return err
		}
		part.available = dec(raw)
		total = total.Add(part.available)
		parts = append(parts, part)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if amount(r).GreaterThan(total) {
		return ErrAmount
	}
	remaining := amount(r)
	for _, part := range parts {
		d := decimal.Min(remaining, part.available)
		if !d.IsPositive() {
			continue
		}
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_funding SET refunded_amount=refunded_amount+$2 WHERE id=$1", part.id, d.String()); err != nil {
			return err
		}
		if part.allocation.Valid {
			if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET used_amount=used_amount-$2 WHERE id=$1", part.allocation.String, d.String()); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO accounting_funding(transaction_id,allocation_id,amount,source_funding_id) VALUES($1,$2,$3,$4)", r.ID, part.allocation, d.String(), part.id); err != nil {
			return err
		}
		remaining = remaining.Sub(d)
	}
	return nil
}
func undoTransaction(ctx context.Context, tx *sql.Tx, r *Record) error {
	var refunds bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE details->>'original_transaction_id'=$1 AND status='CONFIRMED')", r.ID).Scan(&refunds); err != nil {
		return err
	}
	if refunds {
		return ErrState
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,allocation_id::text,amount::text,source_funding_id FROM accounting_funding WHERE transaction_id=$1 ORDER BY id FOR UPDATE", r.ID)
	if err != nil {
		return err
	}
	type reversal struct {
		allocation sql.NullString
		value      decimal.Decimal
		source     sql.NullInt64
	}
	parts := []reversal{}
	for rows.Next() {
		var p reversal
		var id int64
		var raw string
		if err = rows.Scan(&id, &p.allocation, &raw, &p.source); err != nil {
			_ = rows.Close()
			return err
		}
		p.value = dec(raw)
		parts = append(parts, p)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, p := range parts {
		if !p.value.IsPositive() {
			continue
		}
		if isCredit(r) {
			if !p.source.Valid {
				return ErrState
			}
			if _, err = tx.ExecContext(ctx, "UPDATE accounting_funding SET refunded_amount=refunded_amount-$2 WHERE id=$1", p.source.Int64, p.value.String()); err != nil {
				return err
			}
			if p.allocation.Valid {
				var allocated, used, released, transfer string
				if err = tx.QueryRowContext(ctx, "SELECT amount::text,used_amount::text,released_amount::text,transfer_id::text FROM accounting_allocations WHERE id=$1 FOR UPDATE", p.allocation.String).Scan(&allocated, &used, &released, &transfer); err != nil {
					return err
				}
				reclaim := maxZero(dec(used).Add(dec(released)).Add(p.value).Sub(dec(allocated)))
				if reclaim.IsPositive() {
					receipt, err := get(ctx, tx, "transfers", transfer, r.Org, true)
					if err != nil {
						return err
					}
					total, err := effective(ctx, tx, "transfers", receipt)
					if err != nil {
						return err
					}
					occupied, err := sum(ctx, tx, "SELECT COALESCE(SUM(amount-released_amount),0)::text FROM accounting_allocations WHERE transfer_id=$1", transfer)
					if err != nil {
						return err
					}
					if occupied.Add(reclaim).GreaterThan(total) {
						return ErrAmount
					}
				}
				if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET used_amount=used_amount+$2,released_amount=released_amount-$3 WHERE id=$1", p.allocation.String, p.value.String(), reclaim.String()); err != nil {
					return err
				}
			}
		} else if p.allocation.Valid {
			if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET used_amount=used_amount-$2 WHERE id=$1", p.allocation.String, p.value.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Closed cards release only their actual positive balance, including later fees and refunds.
func settleClosedCard(ctx context.Context, tx *sql.Tx, cardID string) error {
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM accounting_vcc_cards WHERE id=$1", cardID).Scan(&status); err != nil {
		return err
	}
	if status != "CLOSED" {
		return nil
	}
	balance, err := cardBalance(ctx, tx, cardID)
	if err != nil || !balance.IsPositive() {
		return err
	}
	list, err := allocations(ctx, tx, cardID)
	if err != nil {
		return err
	}
	for _, a := range list {
		part := decimal.Min(balance, dec(a.Amount).Sub(dec(a.Used)).Sub(dec(a.Released)))
		if !part.IsPositive() {
			continue
		}
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET released_amount=released_amount+$2 WHERE id=$1", a.ID, part.String()); err != nil {
			return err
		}
		balance = balance.Sub(part)
	}
	return nil
}

// New funding first covers previously recorded real overdrafts on an active card.
func coverOverdraft(ctx context.Context, tx *sql.Tx, a *Allocation) error {
	rows, err := tx.QueryContext(ctx, `SELECT f.id,f.transaction_id::text,(f.amount-f.refunded_amount)::text FROM accounting_funding f JOIN accounting_vcc_transactions t ON t.id=f.transaction_id
 WHERE t.parent_id=$1 AND t.status='CONFIRMED' AND f.allocation_id IS NULL AND f.source_funding_id IS NULL ORDER BY f.id FOR UPDATE OF f`, a.CardID)
	if err != nil {
		return err
	}
	type uncovered struct {
		id          int64
		transaction string
		value       decimal.Decimal
	}
	items := []uncovered{}
	for rows.Next() {
		var item uncovered
		var raw string
		if err = rows.Scan(&item.id, &item.transaction, &raw); err != nil {
			_ = rows.Close()
			return err
		}
		item.value = dec(raw)
		items = append(items, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	available := maxZero(dec(a.Amount).Sub(dec(a.Used)).Sub(dec(a.Released)))
	remaining := available
	for _, item := range items {
		part := decimal.Min(remaining, item.value)
		if !part.IsPositive() {
			continue
		}
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_funding SET amount=amount-$2 WHERE id=$1", item.id, part.String()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO accounting_funding(transaction_id,allocation_id,amount) VALUES($1,$2,$3)", item.transaction, a.ID, part.String()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_allocations SET used_amount=used_amount+$2 WHERE id=$1", a.ID, part.String()); err != nil {
			return err
		}
		remaining = remaining.Sub(part)
	}
	a.Used = dec(a.Used).Add(available.Sub(remaining)).String()
	return nil
}

func rebalanceFunding(ctx context.Context, tx *sql.Tx, cardID string) error {
	list, err := allocations(ctx, tx, cardID)
	if err != nil {
		return err
	}
	for i := range list {
		a := &list[i]
		if dec(a.Amount).Sub(dec(a.Used)).Sub(dec(a.Released)).IsPositive() {
			if err = coverOverdraft(ctx, tx, a); err != nil {
				return err
			}
		}
	}
	return nil
}

func linkedCardPaid(ctx context.Context, q dbQuery, transactionID string) (decimal.Decimal, error) {
	return sum(ctx, q, `SELECT t.amount-COALESCE((SELECT SUM(x.amount) FROM accounting_vcc_transactions x WHERE x.details->>'original_transaction_id'=t.id::text AND x.status='CONFIRMED' AND (x.details->>'transaction_type'='REFUND' OR (x.details->>'transaction_type'='ADJUSTMENT' AND x.details->>'direction'='CREDIT'))),0) FROM accounting_vcc_transactions t WHERE t.id=$1 AND t.status='CONFIRMED'`, transactionID)
}
