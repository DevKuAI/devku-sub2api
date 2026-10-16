package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func prepare(kind string, in Input, old *Record) (*Record, error) {
	s, ok := specs[kind]
	if !ok {
		return nil, ErrNotFound
	}
	currency, err := payment.NormalizePaymentCurrency(in.Currency)
	if err != nil {
		return nil, invalid("currency")
	}
	r := &Record{CreatedAt: time.Now().UTC(), ID: uuid.NewString(), OrganizationID: in.OrganizationID, ParentID: in.ParentID, Currency: currency, Status: "DRAFT", Details: map[string]any{}, Version: 1}
	if old != nil {
		*r = *old
		r.Details = selected(old.Details, s.fields)
		if in.Currency != "" {
			r.Currency = currency
		}
	}
	for k, v := range in.Details {
		if computedField(k) {
			continue
		}
		found := false
		for _, f := range s.fields {
			if k == f {
				found = true
				break
			}
		}
		if !found {
			return nil, invalid(k)
		}
		if _, ok := v.(string); !ok {
			if _, ok = v.(bool); !ok {
				return nil, invalid(k)
			}
		}
		if str, ok := v.(string); ok {
			if len(str) > 4000 {
				return nil, invalid(k)
			}
			v = strings.TrimSpace(str)
		}
		r.Details[k] = v
	}
	if old != nil && ((in.OrganizationID != "" && in.OrganizationID != old.OrganizationID) || (in.ParentID != "" && in.ParentID != old.ParentID)) {
		return nil, ErrState
	}
	if in.OccurredAt != "" {
		r.OccurredAt, err = time.Parse(time.RFC3339, in.OccurredAt)
		if err != nil {
			return nil, invalid("occurred_at")
		}
	} else if old == nil {
		r.OccurredAt = time.Now().UTC()
	}
	raw := in.Amount
	if raw == "" && old != nil {
		raw = old.Amount
	}
	if raw == "" && !s.amountRequired {
		raw = "0"
	}
	d, err := number(raw, r.Currency, s.amountRequired)
	if err != nil {
		return nil, err
	}
	if d.IsNegative() {
		return nil, invalid("amount")
	}
	r.Amount = d.String()
	if err = validateFields(kind, r); err != nil {
		return nil, err
	}
	return r, nil
}
func required(r *Record, fields ...string) error {
	for _, k := range fields {
		if detail(r, k) == "" {
			return invalid(k)
		}
	}
	return nil
}
func validateFields(kind string, r *Record) error {
	for _, key := range []string{"tax_inclusive", "ai_vendor", "override"} {
		if v, ok := r.Details[key]; ok {
			if _, valid := v.(bool); !valid {
				return invalid(key)
			}
		}
	}
	for _, key := range []string{"active_at", "inactive_at", "signed_at", "effective_at", "expires_at", "issue_date", "period_start", "period_end"} {
		if value := detail(r, key); value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return invalid(key)
			}
		}
	}
	for _, pair := range [][2]string{{"active_at", "inactive_at"}, {"effective_at", "expires_at"}, {"period_start", "period_end"}} {
		start, end := detail(r, pair[0]), detail(r, pair[1])
		if start != "" && end != "" && start > end {
			return invalid(pair[1])
		}
	}

	switch kind {
	case "receivables", "fx-exchanges":
		return validateWorkflowRecord(kind, r)
	case "transfers":
		return calculateTax(r)
	case "vcc-cards":
		if detail(r, "card_number") == "" && (len(detail(r, "last_four")) != 4) {
			return invalid("last_four")
		}
		if last := detail(r, "last_four"); last != "" {
			for _, ch := range last {
				if ch < '0' || ch > '9' {
					return invalid("last_four")
				}
			}
		}
		if limit := detail(r, "credit_limit"); limit != "" {
			d, err := number(limit, r.Currency, false)
			if err != nil || d.IsNegative() {
				return invalid("credit_limit")
			}
		}
		return required(r, "alias", "provider")
	case "vcc-transactions":
		if detail(r, "transaction_type") == "ADJUSTMENT" && detail(r, "direction") != "DEBIT" && detail(r, "direction") != "CREDIT" {
			return invalid("direction")
		}
		t := detail(r, "transaction_type")
		if t != "CONSUME" && t != "REFUND" && t != "FEE" && t != "ADJUSTMENT" {
			return invalid("transaction_type")
		}
		if t == "REFUND" && detail(r, "original_transaction_id") == "" {
			return invalid("original_transaction_id")
		}
		return required(r, "merchant", "provider_transaction_id")
	case "contracts":
		return required(r, "name", "contract_number")
	case "invoices":
		if detail(r, "invoice_type") != "SALES" && detail(r, "invoice_type") != "PURCHASE" {
			return invalid("invoice_type")
		}
		if err := required(r, "invoice_code", "invoice_number", "issuer_name", "recipient_name"); err != nil {
			return err
		}
		net, err := number(detail(r, "net_amount"), r.Currency, false)
		if err != nil || net.IsNegative() {
			return invalid("net_amount")
		}
		tax, err := number(detail(r, "tax_amount"), r.Currency, false)
		if err != nil || tax.IsNegative() {
			return invalid("tax_amount")
		}
		if !net.Add(tax).Equal(amount(r)) {
			return invalid("total_amount")
		}
		r.Details["total_amount"] = r.Amount
	case "vendor-invoices":
		r.Details["document_type"] = "INVOICE"
		return required(r, "vendor", "period_start", "period_end")
	case "expenses":
		return required(r, "category", "supplier", "payment_account")
	case "expense-payments":
		return required(r, "payment_account", "transaction_number")
	}
	return nil
}
func calculateTax(r *Record) error {
	if err := required(r, "payer_name", "bank_reference"); err != nil {
		return err
	}
	if detail(r, "receiving_account_id") == "" && detail(r, "receiving_account") == "" {
		return invalid("receiving_account_id")
	}
	sales, err := number(detail(r, "sales_amount"), r.Currency, true)
	if err != nil {
		return invalid("sales_amount")
	}
	rate, err := decimal.NewFromString(detail(r, "tax_rate"))
	if err != nil || rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(1)) || rate.Exponent() < -6 {
		return invalid("tax_rate")
	}
	if _, ok := r.Details["tax_inclusive"].(bool); !ok {
		return invalid("tax_inclusive")
	}
	digits := int32(payment.CurrencyMaxFractionDigits(r.Currency))
	tax := sales.Mul(rate)
	net, receivable := sales, sales
	if flag(r.Details, "tax_inclusive") {
		tax = tax.Div(decimal.NewFromInt(1).Add(rate)).Round(digits)
		net = sales.Sub(tax)
	} else {
		tax = tax.Round(digits)
		receivable = sales.Add(tax)
	}
	r.Details["sales_amount"] = sales.String()
	r.Details["tax_rate"] = rate.String()
	r.Details["tax_amount"] = tax.String()
	r.Details["net_sales_amount"] = net.String()
	r.Details["receivable_amount"] = receivable.String()
	r.Details["received_amount"] = r.Amount
	if !amount(r).Equal(receivable) && detail(r, "difference_reason") == "" {
		return invalid("difference_reason")
	}
	return nil
}
func insertRecord(ctx context.Context, tx dbQuery, kind string, r *Record, actor int64) error {
	body, err := json.Marshal(r.Details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+specs[kind].table+"(id,organization_id,parent_id,status,currency,amount,occurred_at,details,created_by) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9)", r.ID, r.Org, r.ParentID, r.Status, r.Currency, r.Amount, r.OccurredAt, body, actor)
	return err
}
func (e *Engine) checkRelations(ctx context.Context, tx dbQuery, kind string, r, old *Record) error {
	if kind == "contracts" {
		var organizationName string
		if err := tx.QueryRowContext(ctx, "SELECT name FROM desktop_organizations WHERE id=$1 AND deleted_at IS NULL", r.Org).Scan(&organizationName); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		r.Details["party_name"] = organizationName
	}
	if kind == "vcc-transactions" || kind == "expense-payments" {
		target := "vcc-cards"
		if kind == "expense-payments" {
			target = "expenses"
		}
		parent, err := get(ctx, tx, target, r.ParentID, r.Org, true)
		if err != nil {
			return err
		}
		if parent.Currency != r.Currency {
			return ErrCurrency
		}
		if parent.Status == "VOID" {
			return ErrState
		}
		if target == "vcc-cards" && parent.Status == "CLOSED" && detail(r, "transaction_type") != "REFUND" && detail(r, "transaction_type") != "ADJUSTMENT" {
			return ErrState
		}
	}
	for field, target := range map[string]string{"vendor_invoice_id": "vendor-invoices", "invoice_id": "invoices", "contract_id": "contracts", "transaction_id": "vcc-transactions", "expense_id": "expenses"} {
		if id := detail(r, field); id != "" {
			linked, err := get(ctx, tx, target, id, r.Org, false)
			if err != nil {
				return err
			}
			if linked.Status == "VOID" {
				return ErrState
			}
			if kind == "expenses" && field == "transaction_id" && (linked.Status != "CONFIRMED" || linked.Currency != r.Currency || !amount(linked).Equal(amount(r)) || isCredit(linked)) {
				return ErrState.WithMetadata(map[string]string{"field": "expense_transaction_match"})
			}

		}
	}
	if kind == "transfers" {
		if old == nil || old.Currency != r.Currency || detail(old, "receiving_account_id") != detail(r, "receiving_account_id") {
			if err := hydrateReceivingAccountSnapshot(ctx, tx, r); err != nil {
				return err
			}
		} else {
			for _, key := range strings.Fields("receiving_account receiving_account_name receiving_bank_name receiving_bank_branch") {
				r.Details[key] = old.Details[key]
			}
		}
	}
	if kind == "transfers" || kind == "contracts" {
		if old != nil && detail(old, "contact_id") == detail(r, "contact_id") {
			for _, key := range strings.Fields("contact_name contact_phone contact_email contact_wechat") {
				r.Details[key] = old.Details[key]
			}
		} else if err := hydrateContactSnapshot(ctx, tx, r); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) storeSecrets(ctx context.Context, tx dbQuery, r *Record, in Input) error {
	secret := selected(in.Details, strings.Fields("card_number cvv expiry provider_token"))
	for k, v := range secret {
		if value, ok := v.(string); !ok || strings.TrimSpace(value) == "" {
			delete(secret, k)
		}
	}
	for _, key := range strings.Fields("card_number cvv expiry provider_token") {
		delete(r.Details, key)
	}
	if len(secret) == 0 {
		return nil
	}
	if !e.stableKey {
		return invalid("fixed_encryption_key")
	}
	if pan := text(secret, "card_number"); pan != "" {
		if len(pan) < 12 || len(pan) > 19 {
			return invalid("card_number")
		}
		for _, ch := range pan {
			if ch < '0' || ch > '9' {
				return invalid("card_number")
			}
		}
		r.Details["last_four"] = pan[len(pan)-4:]
		if err := e.claimCard(ctx, tx, r, pan); err != nil {
			return err
		}
	}
	if cvv := text(secret, "cvv"); cvv != "" {
		if len(cvv) < 3 || len(cvv) > 4 {
			return invalid("cvv")
		}
		for _, ch := range cvv {
			if ch < '0' || ch > '9' {
				return invalid("cvv")
			}
		}
	}
	var previous string
	if err := tx.QueryRowContext(ctx, "SELECT secret_ciphertext FROM accounting_vcc_cards WHERE id=$1", r.ID).Scan(&previous); err != nil {
		return err
	}
	if previous != "" {
		plain, err := e.encryptor.Decrypt(previous)
		if err != nil {
			return err
		}
		stored := map[string]any{}
		if err = json.Unmarshal([]byte(plain), &stored); err != nil {
			return err
		}
		for k, v := range secret {
			stored[k] = v
		}
		secret = stored
	}
	encoded, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	cipher, err := e.encryptor.Encrypt(string(encoded))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE accounting_vcc_cards SET secret_ciphertext=$2 WHERE id=$1", r.ID, cipher)
	return err
}
func (e *Engine) Create(ctx context.Context, tx *sql.Tx, kind string, in Input, org, actor int64) (*Record, error) {
	r, err := prepare(kind, in, nil)
	if err != nil {
		return nil, err
	}
	r.Org = org
	if kind == "expense-payments" {
		return nil, ErrState
	}
	if kind == "transfers" && detail(r, "receiving_account_id") == "" {
		return nil, invalid("receiving_account_id")
	}
	// A card is identified by its full number so it can be checked against every organization.
	if kind == "vcc-cards" && text(in.Details, "card_number") == "" {
		return nil, invalid("card_number")
	}
	if err = e.checkRelations(ctx, tx, kind, r, nil); err != nil {
		return nil, err
	}
	// Secrets are never inserted into metadata, even inside an uncommitted transaction.
	safe := *r
	safe.Details = selected(r.Details, specs[kind].fields)
	for _, k := range strings.Fields("card_number cvv expiry provider_token") {
		delete(safe.Details, k)
	}
	if err = insertRecord(ctx, tx, kind, &safe, actor); err != nil {
		return nil, err
	}
	if kind == "vcc-cards" {
		if err = e.storeSecrets(ctx, tx, r, in); err != nil {
			return nil, err
		}
		r.Status = "ACTIVE"
		if err = save(ctx, tx, kind, r); err != nil {
			return nil, err
		}
	}
	if err = attachToOrder(ctx, tx, kind, r, in.OrderID, actor); err != nil {
		return nil, err
	}
	return r, nil
}
func (e *Engine) Update(ctx context.Context, tx *sql.Tx, kind string, old *Record, in Input) (*Record, error) {
	if old.Status != "DRAFT" && (kind != "vcc-cards" || old.Status != "ACTIVE") {
		return nil, ErrState
	}
	if in.Version != old.Version {
		return nil, ErrState
	}
	r, err := prepare(kind, in, old)
	if err != nil {
		return nil, err
	}
	if err = e.checkRelations(ctx, tx, kind, r, old); err != nil {
		return nil, err
	}
	if kind == "vcc-cards" {
		identityChanged := detail(r, "provider") != detail(old, "provider") || detail(r, "last_four") != detail(old, "last_four") || text(in.Details, "card_number") != "" || text(in.Details, "provider_token") != ""
		if identityChanged {
			var bound bool
			err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_allocations WHERE card_id=$1) OR EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE parent_id=$1)", old.ID).Scan(&bound)
			if err != nil {
				return nil, err
			}
			if bound {
				return nil, ErrState.WithMetadata(map[string]string{"field": "card_identity"})
			}
		}
		if r.Currency != old.Currency {
			return nil, ErrState
		}
		if err = e.storeSecrets(ctx, tx, r, in); err != nil {
			return nil, err
		}
	}
	if err = save(ctx, tx, kind, r); err != nil {
		return nil, err
	}
	return r, nil
}

func computedField(key string) bool {
	switch key {
	case "effective_amount", "original_amount", "adjustment_amount", "paid_amount", "unpaid_amount", "overpaid_amount", "unallocated_amount", "settled_amount", "outstanding_amount", "allocated_cost", "unallocated_cost", "target_available", "available_balance", "allocated_amount", "has_difference", "reconciliation_status", "order_id", "order_name":
		return true
	}
	return false
}
