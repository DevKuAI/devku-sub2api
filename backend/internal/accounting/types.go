// Package accounting implements the enterprise financial ledger independently of Gateway billing.
package accounting

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalid      = appErrors.BadRequest("ACCOUNTING_INVALID", "Invalid accounting fields")
	ErrNotFound     = appErrors.NotFound("ACCOUNTING_NOT_FOUND", "Accounting record not found")
	ErrState        = appErrors.Conflict("ACCOUNTING_STATE_CONFLICT", "Record state or dependencies prevent this operation")
	ErrAmount       = appErrors.Conflict("ACCOUNTING_AMOUNT_CONFLICT", "Amount exceeds available funds or refund limit")
	ErrCurrency     = appErrors.BadRequest("ACCOUNTING_CURRENCY_MISMATCH", "Related records must use the same currency")
	ErrProof        = appErrors.BadRequest("ACCOUNTING_PROOF_REQUIRED", "A completed proof or confirmed AI Invoice is required")
	ErrPeriodClosed = appErrors.Conflict("ACCOUNTING_PERIOD_CLOSED", "The accounting period is closed")
	ErrForbidden    = appErrors.Forbidden("ACCOUNTING_AUTHORIZATION_REQUIRED", "Administrator authorization required")
	ErrStorage      = appErrors.ServiceUnavailable("ACCOUNTING_STORAGE_UNAVAILABLE", "Private accounting storage is not configured or unavailable")
)

type Storage interface {
	Ready() bool
	Upload(context.Context, string, io.Reader, int64, string) error
	Presign(context.Context, string, string, string, time.Duration) (string, error)
}

type Record struct {
	ID              string         `json:"id"`
	OrganizationID  string         `json:"organization_id"`
	ParentID        string         `json:"parent_id,omitempty"`
	Status          string         `json:"status"`
	Currency        string         `json:"currency"`
	Amount          string         `json:"amount"`
	OccurredAt      time.Time      `json:"occurred_at"`
	Details         map[string]any `json:"details"`
	CustomerVisible bool           `json:"customer_visible"`
	SharedAt        *time.Time     `json:"shared_at,omitempty"`
	SharedBy        *int64         `json:"shared_by,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	Version         int64          `json:"version"`
	Org             int64          `json:"-"`
}

type Input struct {
	OrganizationID string         `json:"organization_id"`
	ParentID       string         `json:"parent_id"`
	Currency       string         `json:"currency"`
	Amount         string         `json:"amount"`
	OccurredAt     string         `json:"occurred_at"`
	Details        map[string]any `json:"details"`
	Version        int64          `json:"version"`
	// OrderID places a new record in an order; cards and card transactions require it.
	OrderID string `json:"order_id,omitempty"`
}

type spec struct {
	table          string
	fields         []string
	public         []string
	amountRequired bool
}

var specs = map[string]spec{
	"receivables":      {"accounting_receivables", strings.Fields("name reference due_date contract_id invoice_id remark"), strings.Fields("name reference due_date settled_amount outstanding_amount"), true},
	"fx-exchanges":     {"accounting_fx_exchanges", strings.Fields("target_currency target_amount exchange_rate fee_amount reference remark"), nil, true},
	"transfers":        {"accounting_transfers", strings.Fields("payer_name contact_id contact_name contact_phone contact_email contact_wechat contract_id receiving_account_id receiving_account receiving_account_name receiving_bank_name receiving_bank_branch sales_amount tax_inclusive tax_rate tax_amount net_sales_amount receivable_amount received_amount difference_reason bank_reference remark"), strings.Fields("payer_name sales_amount tax_inclusive tax_rate tax_amount net_sales_amount receivable_amount received_amount original_amount adjustment_amount"), true},
	"vcc-cards":        {"accounting_vcc_cards", strings.Fields("provider alias last_four credit_limit active_at inactive_at card_number cvv expiry provider_token remark"), strings.Fields("alias last_four allocated_amount available_balance"), false},
	"vcc-transactions": {"accounting_vcc_transactions", strings.Fields("transaction_type direction merchant provider_transaction_id original_transaction_id ai_vendor vendor_invoice_id remark"), strings.Fields("transaction_type merchant original_transaction_id"), true},
	"contracts":        {"accounting_contracts", strings.Fields("name contract_number party_name contact_id contact_name contact_phone contact_email contact_wechat tax_inclusive tax_rate signed_at effective_at expires_at remark"), strings.Fields("name contract_number party_name contact_name contact_phone contact_email tax_inclusive tax_rate signed_at effective_at expires_at"), false},
	"invoices":         {"accounting_invoices", strings.Fields("invoice_type invoice_code invoice_number issue_date issuer_name issuer_tax_id recipient_name recipient_tax_id net_amount tax_amount total_amount remark"), strings.Fields("invoice_type invoice_code invoice_number issue_date issuer_name issuer_tax_id recipient_name recipient_tax_id net_amount tax_amount total_amount"), true},
	"vendor-invoices":  {"accounting_vendor_invoices", strings.Fields("vendor period_start period_end document_type invoice_number transaction_id expense_id remark"), nil, true},
	"expenses":         {"accounting_expenses", strings.Fields("category payment_account supplier contract_id contract_number vendor_invoice_id invoice_id transaction_id ai_vendor remark"), nil, true},
	"expense-payments": {"accounting_expense_payments", strings.Fields("payment_account transaction_number proof_attachment_id override reason remark"), nil, true},
}

func detail(r *Record, key string) string      { return text(r.Details, key) }
func text(m map[string]any, key string) string { v, _ := m[key].(string); return strings.TrimSpace(v) }
func flag(m map[string]any, key string) bool   { v, _ := m[key].(bool); return v }
func invalid(field string) error               { return ErrInvalid.WithMetadata(map[string]string{"field": field}) }
func number(raw, currency string, positive bool) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(raw)
	if err != nil || len(raw) > 64 || d.Exponent() < -6 || d.Exponent() > 18 || !d.Truncate(int32(payment.CurrencyMaxFractionDigits(currency))).Equal(d) || (positive && !d.IsPositive()) || d.Abs().GreaterThanOrEqual(decimal.New(1, 18)) {
		return decimal.Zero, invalid("amount")
	}
	return d, nil
}
func amount(r *Record) decimal.Decimal { d, _ := decimal.NewFromString(r.Amount); return d }
func selected(m map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}
func (r *Record) Public(kind string) *Record {
	copy := *r
	copy.Details = selected(r.Details, specs[kind].public)
	copy.SharedBy = nil
	copy.Version = 0
	if kind == "transfers" && detail(r, "effective_amount") != "" {
		copy.Amount = detail(r, "effective_amount")
	}
	if kind == "vcc-cards" {
		copy.Amount = detail(r, "allocated_amount")
	}
	return &copy
}

type Engine struct {
	db             *sql.DB
	desktop        *service.DesktopService
	storage        Storage
	encryptor      service.SecretEncryptor
	stableKey      bool
	desktopEnabled bool
	cardKey        []byte
}

func NewEngine(db *sql.DB, desktop *service.DesktopService, storage Storage, encryptor service.SecretEncryptor, cfg *config.Config) *Engine {
	e := &Engine{db: db, desktop: desktop, storage: storage, encryptor: encryptor, stableKey: cfg != nil && cfg.Totp.EncryptionKeyConfigured, desktopEnabled: cfg != nil && cfg.Desktop.Enabled}
	if e.stableKey {
		// Card fingerprints use a key derived from the fixed encryption key, so numbers are never stored in a guessable form.
		mac := hmac.New(sha256.New, []byte(cfg.Totp.EncryptionKey))
		_, _ = mac.Write([]byte("accounting-card-fingerprint"))
		e.cardKey = mac.Sum(nil)
	}
	return e
}

type dbQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

const recordColumns = "r.id::text,o.public_id,COALESCE(r.parent_id::text,''),r.status,r.currency,r.amount::text,r.occurred_at,r.details,r.customer_visible,r.shared_at,r.shared_by,r.created_at,r.version,r.organization_id"

func scanRecord(row interface{ Scan(...any) error }) (*Record, error) {
	r := &Record{}
	var body []byte
	err := row.Scan(&r.ID, &r.OrganizationID, &r.ParentID, &r.Status, &r.Currency, &r.Amount, &r.OccurredAt, &body, &r.CustomerVisible, &r.SharedAt, &r.SharedBy, &r.CreatedAt, &r.Version, &r.Org)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &r.Details)
	return r, err
}
func get(ctx context.Context, q dbQuery, kind, id string, org int64, lock bool) (*Record, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	s, ok := specs[kind]
	if !ok {
		return nil, ErrNotFound
	}
	query := "SELECT " + recordColumns + " FROM " + s.table + " r JOIN desktop_organizations o ON o.id=r.organization_id WHERE r.id=$1"
	args := []any{id}
	if org > 0 {
		query += " AND r.organization_id=$2"
		args = append(args, org)
	}
	if lock {
		query += " FOR UPDATE OF r"
	}
	return scanRecord(q.QueryRowContext(ctx, query, args...))
}
func save(ctx context.Context, tx dbQuery, kind string, r *Record) error {
	body, err := json.Marshal(r.Details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE "+specs[kind].table+" SET status=$2,amount=$3,currency=$4,occurred_at=$5,details=$6,updated_at=NOW(),version=version+1 WHERE id=$1", r.ID, r.Status, r.Amount, r.Currency, r.OccurredAt, body)
	r.Version++
	return err
}
