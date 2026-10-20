package accounting

import (
	"context"
	"database/sql"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Order entries are the receipts, sales invoices, cards, card spend and AI invoices recorded on the order page.
// Saving an entry makes it effective at once; editing and removing keep the ledger consistent behind the scenes.

const entryRemovedReason = "Removed on the order page"

// entryFields lists what may change on an effective entry in place. Card spend amounts, dates and cards are
// corrected by replacing the spend, because its funding was already drawn from the card.
var entryFields = map[string][]string{
	"transfers":        strings.Fields("payer_name bank_reference contact_id sales_amount tax_rate tax_inclusive difference_reason remark"),
	"invoices":         strings.Fields("invoice_code invoice_number issue_date issuer_name issuer_tax_id recipient_name recipient_tax_id net_amount tax_amount remark"),
	"vendor-invoices":  strings.Fields("vendor invoice_number period_start period_end remark"),
	"vcc-transactions": strings.Fields("merchant provider_transaction_id remark"),
}

type entryInput struct {
	Kind string `json:"kind"`
	Input
}

func (h *Handler) registerEntries(g *gin.RouterGroup) {
	g.POST("/orders/:id/entries", h.createEntry)
	g.PATCH("/orders/:id/entries/:kind/:entry", h.updateEntry)
	g.DELETE("/orders/:id/entries/:kind/:entry", h.removeEntry)
}

func (h *Handler) createEntry(c *gin.Context) {
	order := h.orderRecord(c)
	if order == nil {
		return
	}
	var in entryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	if !orderKinds[in.Kind] {
		response.ErrorFrom(c, invalid("kind"))
		return
	}
	in.OrganizationID = order.OrganizationID
	in.OrderID = order.ID
	if in.Kind == "invoices" {
		if in.Details == nil {
			in.Details = map[string]any{}
		}
		in.Details["invoice_type"] = "SALES"
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: order.Org, Kind: in.Kind, Action: "create", Payload: in.Input}, func(tx *sql.Tx) (any, error) {
		r, err := h.engine.addEntry(ctx, tx, in.Kind, in.Input, order.Org, actor(c))
		if err != nil {
			return nil, err
		}
		return r, h.engine.enrich(ctx, in.Kind, r)
	})
}

// addEntry creates a record inside its order and confirms it in the same transaction. Cards are active as
// soon as they are created.
func (e *Engine) addEntry(ctx context.Context, tx *sql.Tx, kind string, in Input, org, actor int64) (*Record, error) {
	r, err := e.Create(ctx, tx, kind, in, org, actor)
	if err != nil || kind == "vcc-cards" {
		return r, err
	}
	return r, e.Confirm(ctx, tx, kind, r, actor)
}

// entry loads a record of the order for a change, rejecting records that belong elsewhere.
func (h *Handler) entry(c *gin.Context) (*Order, string, bool) {
	order := h.orderRecord(c)
	if order == nil {
		return nil, "", false
	}
	kind := c.Param("kind")
	if !orderKinds[kind] {
		response.ErrorFrom(c, ErrNotFound)
		return nil, "", false
	}
	if !idValid(c, c.Param("entry")) {
		return nil, "", false
	}
	return order, kind, true
}

func lockEntry(ctx context.Context, tx *sql.Tx, order *Order, kind, id string) (*Record, error) {
	r, err := get(ctx, tx, kind, id, order.Org, true)
	if err != nil {
		return nil, err
	}
	owner, err := orderOf(ctx, tx, kind, r.ID)
	if err != nil {
		return nil, err
	}
	// Cards may be listed on an order through its spend; they are managed from any order that uses them.
	if owner != order.ID && kind != "vcc-cards" {
		return nil, ErrNotFound
	}
	if r.Status == "VOID" {
		return nil, ErrState
	}
	return r, nil
}

func (h *Handler) updateEntry(c *gin.Context) {
	order, kind, ok := h.entry(c)
	if !ok {
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	in.OrganizationID = ""
	ctx := c.Request.Context()
	h.run(c, Operation{Org: order.Org, Kind: kind, Object: c.Param("entry"), Action: "update", Payload: in}, func(tx *sql.Tx) (any, error) {
		if err := lockOrder(ctx, tx, order.ID, order.Org, true); err != nil {
			return nil, err
		}
		r, err := lockEntry(ctx, tx, order, kind, c.Param("entry"))
		if err != nil {
			return nil, err
		}
		next, err := h.engine.amendEntry(ctx, tx, order.ID, kind, r, in, actor(c))
		if err != nil {
			return nil, err
		}
		return next, h.engine.enrich(ctx, kind, next)
	})
}

// amendEntry applies a correction. Drafts left by the old workflow and cards keep their full edit path.
func (e *Engine) amendEntry(ctx context.Context, tx *sql.Tx, order, kind string, old *Record, in Input, actor int64) (*Record, error) {
	if old.Status == "DRAFT" || kind == "vcc-cards" {
		return e.Update(ctx, tx, kind, old, in)
	}
	if in.Version != old.Version {
		return nil, ErrState
	}
	if kind == "vcc-transactions" && spendChanged(old, in) {
		return e.replaceSpend(ctx, tx, order, old, in, actor)
	}
	for key := range in.Details {
		if !computedField(key) && !contains(entryFields[kind], key) && in.Details[key] != old.Details[key] {
			return nil, ErrState.WithMetadata(map[string]string{"field": "entry_locked"})
		}
	}
	if kind == "vcc-transactions" {
		in.Amount, in.Currency, in.OccurredAt = "", "", ""
	}
	if kind == "vendor-invoices" {
		// The invoice is matched to the order's spend again after the change.
		if err := voidCosts(ctx, tx, "invoice_id::text=$1", old.ID); err != nil {
			return nil, err
		}
	}
	if kind == "transfers" {
		if err := releaseUnusedFunding(ctx, tx, old.ID); err != nil {
			return nil, err
		}
	}
	r, err := prepare(kind, in, old)
	if err != nil {
		return nil, err
	}
	if kind == "transfers" {
		if err = e.checkReceiptCorrection(ctx, tx, old, r); err != nil {
			return nil, err
		}
	}
	if err = e.checkRelations(ctx, tx, kind, r, old); err != nil {
		return nil, err
	}
	if err = save(ctx, tx, kind, r); err != nil {
		return nil, err
	}
	switch kind {
	case "transfers":
		err = e.afterOrderChange(ctx, tx, kind, r, actor)
	case "vendor-invoices":
		err = e.allocateOrderCosts(ctx, tx, order, r.Org, actor)
	}
	return r, err
}

// checkReceiptCorrection keeps a corrected receipt above what the order's cards already spent from it.
func (e *Engine) checkReceiptCorrection(ctx context.Context, tx *sql.Tx, old, r *Record) error {
	used, err := transferCommitted(ctx, tx, old.ID)
	if err != nil {
		return err
	}
	if r.Currency != old.Currency && used.IsPositive() {
		return ErrState.WithMetadata(map[string]string{"field": "receipt_in_use"})
	}
	total, err := effective(ctx, tx, "transfers", r)
	if err != nil {
		return err
	}
	if total.LessThan(used) {
		return ErrAmount.WithMetadata(map[string]string{"field": "receipt_below_spend"})
	}
	return nil
}

func spendChanged(old *Record, in Input) bool {
	if in.Amount != "" && !dec(in.Amount).Equal(amount(old)) {
		return true
	}
	if in.Currency != "" && in.Currency != old.Currency {
		return true
	}
	if in.ParentID != "" && in.ParentID != old.ParentID {
		return true
	}
	if in.OccurredAt != "" {
		occurred, err := workflowTime(in.OccurredAt)
		if err != nil || !occurred.Equal(old.OccurredAt) {
			return true
		}
	}
	for _, key := range []string{"transaction_type", "direction", "original_transaction_id"} {
		if value, ok := in.Details[key]; ok && value != old.Details[key] {
			return true
		}
	}
	return false
}

// replaceSpend corrects card spend whose amount, date or card changed: the old spend is reversed and a new
// one recorded in the same order, then the order's AI invoices are matched again.
func (e *Engine) replaceSpend(ctx context.Context, tx *sql.Tx, order string, old *Record, in Input, actor int64) (*Record, error) {
	if err := e.removeSpend(ctx, tx, old, actor); err != nil {
		return nil, err
	}
	next := in
	next.Details = selected(old.Details, specs["vcc-transactions"].fields)
	for key, value := range in.Details {
		next.Details[key] = value
	}
	next.OrganizationID = old.OrganizationID
	next.OrderID = order
	next.Version = 0
	if next.ParentID == "" {
		next.ParentID = old.ParentID
	}
	if next.Currency == "" {
		next.Currency = old.Currency
	}
	if next.Amount == "" {
		next.Amount = old.Amount
	}
	if next.OccurredAt == "" {
		next.OccurredAt = old.OccurredAt.Format("2006-01-02T15:04:05Z07:00")
	}
	r, err := e.addEntry(ctx, tx, "vcc-transactions", next, old.Org, actor)
	if err != nil {
		return nil, err
	}
	return r, e.allocateOrderCosts(ctx, tx, order, old.Org, actor)
}

func (e *Engine) removeSpend(ctx context.Context, tx *sql.Tx, r *Record, actor int64) error {
	if err := voidCosts(ctx, tx, "(source_kind='vcc-transactions' AND source_id::text=$1) OR cost_identity='vcc-transactions:'||$1", r.ID); err != nil {
		return err
	}
	return e.Void(ctx, tx, "vcc-transactions", r, entryRemovedReason, actor)
}

func (h *Handler) removeEntry(c *gin.Context) {
	order, kind, ok := h.entry(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	payload := map[string]string{"order_id": order.ID, "reason": entryRemovedReason}
	h.run(c, Operation{Org: order.Org, Kind: kind, Object: c.Param("entry"), Action: "void", Payload: payload}, func(tx *sql.Tx) (any, error) {
		if err := lockOrder(ctx, tx, order.ID, order.Org, true); err != nil {
			return nil, err
		}
		r, err := lockEntry(ctx, tx, order, kind, c.Param("entry"))
		if err != nil {
			return nil, err
		}
		if err = h.engine.removeEntry(ctx, tx, order.ID, kind, r, actor(c)); err != nil {
			return nil, err
		}
		return r, nil
	})
}

// removeEntry takes an entry off the order. Cards are closed rather than removed, so their history stays.
func (e *Engine) removeEntry(ctx context.Context, tx *sql.Tx, order, kind string, r *Record, actor int64) error {
	switch kind {
	case "vcc-cards":
		return e.CloseCard(ctx, tx, r)
	case "vcc-transactions":
		if r.Status != "DRAFT" {
			if err := e.removeSpend(ctx, tx, r, actor); err != nil {
				return err
			}
			return e.allocateOrderCosts(ctx, tx, order, r.Org, actor)
		}
	case "vendor-invoices":
		if err := voidCosts(ctx, tx, "invoice_id::text=$1", r.ID); err != nil {
			return err
		}
		if err := e.Void(ctx, tx, kind, r, entryRemovedReason, actor); err != nil {
			return err
		}
		return e.allocateOrderCosts(ctx, tx, order, r.Org, actor)
	case "transfers":
		if err := releaseUnusedFunding(ctx, tx, r.ID); err != nil {
			return err
		}
		used, err := transferCommitted(ctx, tx, r.ID)
		if err != nil {
			return err
		}
		if used.IsPositive() {
			return ErrState.WithMetadata(map[string]string{"field": "receipt_in_use"})
		}
	}
	return e.Void(ctx, tx, kind, r, entryRemovedReason, actor)
}

// voidCosts withdraws active cost allocations so the order can match its AI invoices again.
func voidCosts(ctx context.Context, tx *sql.Tx, where, id string) error {
	_, err := tx.ExecContext(ctx, "UPDATE accounting_cost_allocations SET status='VOID',reason=$2,voided_at=NOW() WHERE status='ACTIVE' AND ("+where+")", id, entryRemovedReason)
	return err
}

// releaseUnusedFunding returns money a receipt placed on cards but no spend used yet. Funding still awaiting
// confirmation at the card provider is left alone.
func releaseUnusedFunding(ctx context.Context, tx *sql.Tx, transfer string) error {
	_, err := tx.ExecContext(ctx, `UPDATE accounting_allocations a SET released_amount=a.amount-a.used_amount
 WHERE a.transfer_id=$1 AND a.amount-a.used_amount-a.released_amount>0
 AND NOT EXISTS(SELECT 1 FROM accounting_external_actions x WHERE x.object_kind='allocations' AND x.object_id=a.id AND x.status='PENDING')`, transfer)
	return err
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
