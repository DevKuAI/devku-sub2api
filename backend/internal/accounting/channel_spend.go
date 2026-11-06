package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Card spend has one entry point: the imported channel statement. Each card line of a statement is booked onto the
// organization card whose platform card ID it names, in the order that card belongs to; spend recorded by hand before
// imports existed is linked instead of booked twice. Lines that cannot be booked yet wait with a reason.

const (
	// channelSource marks card transactions booked from a statement line.
	channelSource = "CHANNEL"
	// channelUnbookReason voids a booked transaction when its statement line is revoked or its card rebound.
	channelUnbookReason = "Statement line withdrawn"
)

// statementFields are the card transaction details only a statement import sets.
var statementFields = []string{"source", "channel_line_id", "channel_match", "original_amount", "original_currency"}

// ErrCardSpendFromImport refuses hand-made card spend: it only enters the books from statements.
var ErrCardSpendFromImport = ErrState.WithMetadata(map[string]string{"field": "card_spend_from_import"})

type channelSpendLine struct {
	id, kind, currency, externalID, relatedID, merchant, remark, originalCurrency string
	amount                                                                        decimal.Decimal
	original                                                                      sql.NullString
	occurredAt                                                                    time.Time
	channelCard, cardRef                                                          string
	vccCard                                                                       sql.NullString
}

// providerTransactionID is the platform's number for the transaction: spend is known by its authorization,
// refunds and adjustments by their own reference.
func (l *channelSpendLine) providerTransactionID() string {
	if l.kind == "CONSUME" && l.relatedID != "" {
		return l.relatedID
	}
	return l.externalID
}

// bookChannelLines books the card lines selected by where (on accounting_channel_lines l) that are not booked yet.
func (e *Engine) bookChannelLines(ctx context.Context, tx *sql.Tx, where string, args []any, actor int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT l.id::text,l.type,l.currency,l.external_id,l.related_id,l.merchant,l.remark,l.original_currency,l.amount::text,
 l.original_amount::text,l.occurred_at,c.id::text,c.card_ref,c.vcc_card_id::text
 FROM accounting_channel_lines l JOIN accounting_channel_cards c ON c.id=l.channel_card_id
 WHERE l.status='ACTIVE' AND l.type IN ('CONSUME','REFUND','ADJUSTMENT') AND l.transaction_id IS NULL
 AND l.booking IN ('INTERNAL','PENDING_CARD','PENDING_ORDER') AND (`+where+`)
 ORDER BY l.occurred_at,l.seq FOR UPDATE OF l`, args...)
	if err != nil {
		return err
	}
	lines := []*channelSpendLine{}
	for rows.Next() {
		l := &channelSpendLine{}
		var raw string
		if err = rows.Scan(&l.id, &l.kind, &l.currency, &l.externalID, &l.relatedID, &l.merchant, &l.remark, &l.originalCurrency, &raw, &l.original, &l.occurredAt, &l.channelCard, &l.cardRef, &l.vccCard); err != nil {
			_ = rows.Close()
			return err
		}
		l.amount = dec(raw)
		lines = append(lines, l)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, l := range lines {
		if !l.vccCard.Valid {
			if l.vccCard, err = autoBindChannelCard(ctx, tx, l.channelCard, l.cardRef); err != nil {
				return err
			}
		}
		if err = e.bookChannelLine(ctx, tx, l, actor); err != nil {
			return err
		}
	}
	return nil
}

// autoBindChannelCard binds a statement card to the organization card registered with the same platform card ID.
func autoBindChannelCard(ctx context.Context, tx *sql.Tx, channelCard, cardRef string) (sql.NullString, error) {
	var card sql.NullString
	var org int64
	err := tx.QueryRowContext(ctx, "SELECT id::text,organization_id FROM accounting_vcc_cards WHERE details->>'provider_card_id'=$1 AND status<>'VOID'", cardRef).Scan(&card, &org)
	if err == sql.ErrNoRows {
		return sql.NullString{}, nil
	}
	if err != nil {
		return card, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounting_channel_cards SET organization_id=$2,vcc_card_id=$3,bind_method='AUTO_CARD',bound_at=NOW(),updated_at=NOW()
 WHERE id=$1 AND vcc_card_id IS NULL`, channelCard, org, card.String)
	return card, err
}

func jsonText(v any) string {
	body, _ := json.Marshal(v)
	return string(body)
}

func setBooking(ctx context.Context, tx *sql.Tx, line, booking, note string, transaction sql.NullString) error {
	_, err := tx.ExecContext(ctx, "UPDATE accounting_channel_lines SET booking=$2,booking_note=$3,transaction_id=$4 WHERE id=$1", line, booking, note, transaction)
	return err
}

func (e *Engine) bookChannelLine(ctx context.Context, tx *sql.Tx, l *channelSpendLine, actor int64) error {
	none := sql.NullString{}
	if !l.vccCard.Valid {
		return setBooking(ctx, tx, l.id, "PENDING_CARD", "", none)
	}
	card, err := get(ctx, tx, "vcc-cards", l.vccCard.String, 0, true)
	if err != nil {
		return err
	}
	if card.Currency != l.currency {
		return setBooking(ctx, tx, l.id, "PENDING_REVIEW", "currency", none)
	}
	if linked, linkErr := linkHandEntered(ctx, tx, card, l); linkErr != nil || linked {
		return linkErr
	}
	if checkOpenPeriod(ctx, tx, card.Org, l.occurredAt) != nil {
		return setBooking(ctx, tx, l.id, "PENDING_REVIEW", "period_closed", none)
	}
	details := map[string]any{"merchant": l.merchant, "provider_transaction_id": l.providerTransactionID(), "source": channelSource, "channel_line_id": l.id, "remark": l.remark}
	if l.merchant == "" {
		// Refunds and adjustments must describe themselves; statements sometimes only carry a remark.
		details["merchant"] = l.remark
		if l.remark == "" {
			details["merchant"] = l.externalID
		}
	}
	if l.original.Valid && l.originalCurrency != "" && l.originalCurrency != l.currency {
		details["original_amount"] = dec(l.original.String).Abs().String()
		details["original_currency"] = l.originalCurrency
	}
	order := ""
	switch l.kind {
	case "CONSUME":
		details["transaction_type"] = "CONSUME"
		if vendor, vendorErr := vendorByAlias(ctx, tx, l.merchant); vendorErr != nil {
			return vendorErr
		} else if vendor != "" {
			details["vendor_id"] = vendor
		}
	case "ADJUSTMENT":
		details["transaction_type"] = "ADJUSTMENT"
		details["direction"] = "DEBIT"
		if l.amount.IsPositive() {
			details["direction"] = "CREDIT"
		}
	case "REFUND":
		details["transaction_type"] = "REFUND"
		original, findErr := refundedPurchase(ctx, tx, card.ID, l.relatedID)
		if findErr != nil {
			return findErr
		}
		if original == "" {
			return setBooking(ctx, tx, l.id, "PENDING_REVIEW", "original_missing", none)
		}
		details["original_transaction_id"] = original
	}
	if l.kind != "REFUND" {
		if order, err = spendOrder(ctx, tx, card); err != nil {
			return err
		}
		if order == "" {
			return setBooking(ctx, tx, l.id, "PENDING_ORDER", "", none)
		}
	}
	if err = e.prepareOrganization(ctx, tx, card.Org); err != nil {
		return err
	}
	in := Input{OrganizationID: card.OrganizationID, ParentID: card.ID, Currency: card.Currency, Amount: l.amount.Abs().String(),
		OccurredAt: l.occurredAt.UTC().Format(time.RFC3339), OrderID: order, Details: details}
	if _, err = tx.ExecContext(ctx, "SAVEPOINT channel_line"); err != nil {
		return err
	}
	r, err := e.Create(ctx, tx, "vcc-transactions", in, card.Org, actor)
	if err == nil {
		err = e.Confirm(ctx, tx, "vcc-transactions", r, actor)
	}
	if err != nil {
		// A line the books refuse (a refund above what is left of its purchase, say) waits for review instead of
		// failing the whole statement.
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT channel_line"); rollbackErr != nil {
			return rollbackErr
		}
		return setBooking(ctx, tx, l.id, "PENDING_REVIEW", bookingNote(err), none)
	}
	if _, err = tx.ExecContext(ctx, "RELEASE SAVEPOINT channel_line"); err != nil {
		return err
	}
	if err = e.noteSpendChange(ctx, tx, r, false, "channel-book", actor); err != nil {
		return err
	}
	return setBooking(ctx, tx, l.id, "BOOKED", "", sql.NullString{String: r.ID, Valid: true})
}

// bookingNote names why the books refused a line, in the error vocabulary the pages already translate.
func bookingNote(err error) string {
	app := appErrors.FromError(err)
	if field := app.Metadata["field"]; field != "" {
		return field
	}
	if app.Reason == appErrors.FromError(ErrAmount).Reason {
		return "amount"
	}
	return strings.ToLower(strings.TrimPrefix(app.Reason, "ACCOUNTING_"))
}

// linkHandEntered attaches a line to spend recorded by hand before statements were imported: first by the platform
// transaction number, then, for records saved without one, by a single same-card record of the same type, amount and
// day. Links found the second way are flagged for review.
func linkHandEntered(ctx context.Context, tx *sql.Tx, card *Record, l *channelSpendLine) (bool, error) {
	const unlinked = ` AND t.parent_id=$1 AND t.status='CONFIRMED' AND t.details->>'transaction_type'=$2 AND COALESCE(t.details->>'source','')<>'CHANNEL'
 AND NOT EXISTS(SELECT 1 FROM accounting_channel_lines x WHERE x.transaction_id=t.id AND x.status='ACTIVE')`
	reference := l.providerTransactionID()
	var id string
	match := ""
	err := tx.QueryRowContext(ctx, `SELECT t.id::text FROM accounting_vcc_transactions t WHERE t.details->>'provider_transaction_id'=$3`+unlinked+` LIMIT 1`, card.ID, l.kind, reference).Scan(&id)
	if err == sql.ErrNoRows {
		rows, queryErr := tx.QueryContext(ctx, `SELECT t.id::text FROM accounting_vcc_transactions t WHERE COALESCE(t.details->>'provider_transaction_id','')='' AND t.amount=$3
 AND t.occurred_at BETWEEN $4::timestamptz-INTERVAL '1 day' AND $4::timestamptz+INTERVAL '1 day'`+unlinked+` LIMIT 2`, card.ID, l.kind, l.amount.Abs().String(), l.occurredAt)
		if queryErr != nil {
			return false, queryErr
		}
		candidates := []string{}
		for rows.Next() {
			var candidate string
			if queryErr = rows.Scan(&candidate); queryErr != nil {
				_ = rows.Close()
				return false, queryErr
			}
			candidates = append(candidates, candidate)
		}
		queryErr = rows.Err()
		_ = rows.Close()
		if queryErr != nil || len(candidates) != 1 {
			return false, queryErr
		}
		id, match, err = candidates[0], "AMOUNT_DATE", nil
	}
	if err != nil {
		return false, err
	}
	patch := map[string]any{"channel_line_id": l.id, "provider_transaction_id": reference}
	if match != "" {
		patch["channel_match"] = match
	}
	if _, err = tx.ExecContext(ctx, "UPDATE accounting_vcc_transactions SET details=details||$2::jsonb,updated_at=NOW() WHERE id=$1", id, jsonText(patch)); err != nil {
		return false, err
	}
	return true, setBooking(ctx, tx, l.id, "LINKED", strings.ToLower(match), sql.NullString{String: id, Valid: true})
}

// refundedPurchase finds the purchase a refund returns money for, by its platform number.
func refundedPurchase(ctx context.Context, tx *sql.Tx, card, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT t.id::text FROM accounting_vcc_transactions t WHERE t.parent_id=$1 AND t.status='CONFIRMED'
 AND t.details->>'transaction_type'='CONSUME' AND (t.details->>'provider_transaction_id'=$2
   OR t.id IN (SELECT x.transaction_id FROM accounting_channel_lines x WHERE x.status='ACTIVE' AND x.external_id=$2))
 ORDER BY t.occurred_at LIMIT 1`, card, reference).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// spendOrder is where new card spend goes: the open order the card was added in, otherwise the organization's most
// recent open order. No open order leaves the spend waiting.
func spendOrder(ctx context.Context, tx *sql.Tx, card *Record) (string, error) {
	var order string
	err := tx.QueryRowContext(ctx, `SELECT o.id::text FROM accounting_orders o LEFT JOIN accounting_order_items i ON i.order_id=o.id AND i.object_kind='vcc-cards' AND i.object_id=$2
 WHERE o.organization_id=$1 AND o.status='OPEN' ORDER BY i.order_id IS NULL,o.created_at DESC,o.id LIMIT 1`, card.Org, card.ID).Scan(&order)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return order, err
}

// vendorByAlias picks the AI vendor whose name or alias appears in the platform's merchant text, when exactly one does.
func vendorByAlias(ctx context.Context, tx *sql.Tx, merchant string) (string, error) {
	if strings.TrimSpace(merchant) == "" {
		return "", nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT v.id::text FROM accounting_vendors v WHERE v.kind=$1 AND v.status='ACTIVE'
 AND EXISTS(SELECT 1 FROM jsonb_array_elements_text(v.aliases) a WHERE strpos(lower($2), a)>0) LIMIT 2`, VendorAI, merchant)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	found := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return "", err
		}
		found = append(found, id)
	}
	if len(found) != 1 {
		return "", rows.Err()
	}
	return found[0], rows.Err()
}

// prepareOrganization locks an organization before statement lines change its books and makes sure its position
// history has a baseline taken before the change.
func (e *Engine) prepareOrganization(ctx context.Context, tx *sql.Tx, org int64) error {
	if _, err := tx.ExecContext(ctx, "SELECT id FROM desktop_organizations WHERE id=$1 FOR UPDATE", org); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_position_baselines WHERE organization_id=$1)", org).Scan(&exists); err != nil || exists {
		return err
	}
	before, err := currentPositions(ctx, tx, org)
	if err != nil {
		return err
	}
	return ensurePositionBaseline(ctx, tx, org, before)
}

// noteSpendChange writes the position change of a booked (or withdrawn) card transaction at its own date, remembers
// it so the surrounding operation does not count it again, and audits it for the organization.
func (e *Engine) noteSpendChange(ctx context.Context, tx *sql.Tx, r *Record, withdrawn bool, action string, actor int64) error {
	delta := amount(r)
	if isCredit(r) {
		delta = delta.Neg()
	}
	if withdrawn {
		delta = delta.Neg()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO accounting_position_events(id,organization_id,currency,cash_delta,card_delta,occurred_at,object_kind,object_id,action) VALUES($1,$2,$3,0,$4,$5,'vcc-transactions',$6,$7)`,
		uuid.NewString(), r.Org, r.Currency, delta.String(), r.OccurredAt, r.ID, action); err != nil {
		return err
	}
	stored, _ := e.recorded.LoadOrStore(tx, map[int64]map[string]decimal.Decimal{})
	recorded, ok := stored.(map[int64]map[string]decimal.Decimal)
	if !ok {
		return ErrInvalid
	}
	if recorded[r.Org] == nil {
		recorded[r.Org] = map[string]decimal.Decimal{}
	}
	recorded[r.Org][r.Currency] = recorded[r.Org][r.Currency].Add(delta)
	return audit(ctx, tx, r.Org, "vcc-transactions", r.ID, action, actor, "", map[string]any{"amount": r.Amount, "currency": r.Currency, "channel_line_id": detail(r, "channel_line_id"), "status": r.Status})
}

// withoutRecorded takes the card changes already written with their own dates out of an operation's closing positions.
func (e *Engine) withoutRecorded(tx *sql.Tx, org int64, after map[string]Position) map[string]Position {
	stored, ok := e.recorded.Load(tx)
	if !ok {
		return after
	}
	recorded, ok := stored.(map[int64]map[string]decimal.Decimal)
	if !ok {
		return after
	}
	for currency, delta := range recorded[org] {
		p := after[currency]
		p.Card = dec(p.Card).Sub(delta).String()
		after[currency] = p
	}
	return after
}

// unbookChannelLines withdraws what the selected lines put into the organization books, newest first so refunds go
// before their purchases: booked transactions are voided, links to hand-entered records are released.
func (e *Engine) unbookChannelLines(ctx context.Context, tx *sql.Tx, where string, args []any, actor int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT l.id::text,l.booking,l.transaction_id::text FROM accounting_channel_lines l
 WHERE l.transaction_id IS NOT NULL AND (`+where+`) ORDER BY l.occurred_at DESC,l.seq DESC FOR UPDATE OF l`, args...)
	if err != nil {
		return err
	}
	type booked struct{ line, booking, transaction string }
	list := []booked{}
	for rows.Next() {
		var b booked
		if err = rows.Scan(&b.line, &b.booking, &b.transaction); err != nil {
			_ = rows.Close()
			return err
		}
		list = append(list, b)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, b := range list {
		r, getErr := get(ctx, tx, "vcc-transactions", b.transaction, 0, true)
		if getErr != nil {
			return getErr
		}
		if b.booking == "LINKED" {
			if _, err = tx.ExecContext(ctx, "UPDATE accounting_vcc_transactions SET details=details-'channel_line_id'-'channel_match',updated_at=NOW() WHERE id=$1", r.ID); err != nil {
				return err
			}
		} else if r.Status != "VOID" {
			if err = e.withdrawBooked(ctx, tx, r, actor); err != nil {
				return err
			}
		}
		if err = setBooking(ctx, tx, b.line, "PENDING_CARD", "", sql.NullString{}); err != nil {
			return err
		}
	}
	return nil
}

// withdrawBooked voids a transaction booked from a statement, takes it off its order's costs and matches the
// order's AI invoices again.
func (e *Engine) withdrawBooked(ctx context.Context, tx *sql.Tx, r *Record, actor int64) error {
	if err := checkOpenPeriod(ctx, tx, r.Org, r.OccurredAt); err != nil {
		return err
	}
	if err := e.prepareOrganization(ctx, tx, r.Org); err != nil {
		return err
	}
	order, err := orderOf(ctx, tx, "vcc-transactions", r.ID)
	if err != nil {
		return err
	}
	if err = voidCosts(ctx, tx, "(source_kind='vcc-transactions' AND source_id::text=$1) OR cost_identity='vcc-transactions:'||$1", r.ID); err != nil {
		return err
	}
	if err = e.Void(ctx, tx, "vcc-transactions", r, channelUnbookReason, actor); err != nil {
		return err
	}
	if err = e.noteSpendChange(ctx, tx, r, true, "channel-unbook", actor); err != nil {
		return err
	}
	if order == "" {
		return nil
	}
	return e.allocateOrderCosts(ctx, tx, order, r.Org, actor)
}

// bookPendingForOrganization books lines that waited for an open order of the organization.
func (e *Engine) bookPendingForOrganization(ctx context.Context, tx *sql.Tx, org int64, actor int64) error {
	return e.bookChannelLines(ctx, tx, "l.booking='PENDING_ORDER' AND c.organization_id=$1", []any{org}, actor)
}

// bindCardStatements binds statement cards with the card's platform card ID and books their waiting lines.
func (e *Engine) bindCardStatements(ctx context.Context, tx *sql.Tx, card *Record, actor int64) error {
	reference := detail(card, "provider_card_id")
	if reference == "" || card.Status == "VOID" {
		return nil
	}
	return e.bookChannelLines(ctx, tx, "c.card_ref=$1 AND (c.vcc_card_id IS NULL OR c.vcc_card_id=$2)", []any{reference, card.ID}, actor)
}

// RebookChannelLine books a line that waited for review again and reports where it stands now.
func (e *Engine) RebookChannelLine(ctx context.Context, tx *sql.Tx, id string, actor int64) (map[string]string, error) {
	result, err := tx.ExecContext(ctx, "UPDATE accounting_channel_lines SET booking='PENDING_CARD',booking_note='' WHERE id=$1 AND status='ACTIVE' AND booking='PENDING_REVIEW'", id)
	if err != nil {
		return nil, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return nil, ErrState
	}
	if err = e.bookChannelLines(ctx, tx, "l.id=$1", []any{id}, actor); err != nil {
		return nil, err
	}
	var booking, note, transaction string
	if err = tx.QueryRowContext(ctx, "SELECT booking,booking_note,COALESCE(transaction_id::text,'') FROM accounting_channel_lines WHERE id=$1", id).Scan(&booking, &note, &transaction); err != nil {
		return nil, err
	}
	return map[string]string{"id": id, "booking": booking, "booking_note": note, "transaction_id": transaction}, nil
}

// BindChannelCard binds a statement card to an organization card by hand, or unbinds it with an empty card. Lines
// already in the books move with the binding: they are withdrawn from the old card and booked onto the new one.
func (e *Engine) BindChannelCard(ctx context.Context, tx *sql.Tx, id, cardID string, actor int64) (*ChannelCardSummary, error) {
	var cardRef string
	var current sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT card_ref,vcc_card_id::text FROM accounting_channel_cards WHERE id=$1 FOR UPDATE", id).Scan(&cardRef, &current)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var card *Record
	if cardID != "" {
		if card, err = get(ctx, tx, "vcc-cards", cardID, 0, true); err != nil {
			return nil, invalid("vcc_card_id")
		}
		if card.Status == "VOID" {
			return nil, invalid("vcc_card_id")
		}
		if reference := detail(card, "provider_card_id"); reference != "" && reference != cardRef {
			return nil, ErrState.WithMetadata(map[string]string{"field": "card_id_mismatch"})
		}
		var mismatch bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_channel_lines WHERE channel_card_id=$1 AND status='ACTIVE' AND type IN ('CONSUME','REFUND','ADJUSTMENT') AND currency<>$2)", id, card.Currency).Scan(&mismatch); err != nil {
			return nil, err
		}
		if mismatch {
			return nil, ErrCurrency
		}
	}
	if current.Valid && current.String == cardID {
		return channelCardSummary(ctx, tx, id)
	}
	if err = e.unbookChannelLines(ctx, tx, "l.channel_card_id=$1 AND l.status='ACTIVE'", []any{id}, actor); err != nil {
		return nil, err
	}
	if current.Valid {
		// The statement says the old card is not this platform card, so it gives the card ID back.
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_vcc_cards SET details=details-'provider_card_id',updated_at=NOW() WHERE id=$1 AND details->>'provider_card_id'=$2", current.String, cardRef); err != nil {
			return nil, err
		}
	}
	// Lines that waited for review get another chance on the new card.
	if _, err = tx.ExecContext(ctx, "UPDATE accounting_channel_lines SET booking='PENDING_CARD',booking_note='' WHERE channel_card_id=$1 AND status='ACTIVE' AND booking='PENDING_REVIEW'", id); err != nil {
		return nil, err
	}
	if card == nil {
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_channel_cards SET organization_id=NULL,vcc_card_id=NULL,bind_method='',bound_at=NULL,bound_by=NULL,updated_at=NOW() WHERE id=$1", id); err != nil {
			return nil, err
		}
		return channelCardSummary(ctx, tx, id)
	}
	if detail(card, "provider_card_id") == "" {
		// A card registered before card IDs were kept takes the platform's ID from the statement.
		card.Details["provider_card_id"] = cardRef
		if err = save(ctx, tx, "vcc-cards", card); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE accounting_channel_cards SET organization_id=$2,vcc_card_id=$3,bind_method='MANUAL',bound_at=NOW(),bound_by=$4,updated_at=NOW() WHERE id=$1", id, card.Org, card.ID, actor); err != nil {
		return nil, err
	}
	if err = e.bookChannelLines(ctx, tx, "l.channel_card_id=$1", []any{id}, actor); err != nil {
		return nil, err
	}
	return channelCardSummary(ctx, tx, id)
}
