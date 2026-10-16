//go:build integration

package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type accountingTestStorage struct {
	available bool
	mu        sync.Mutex
	uploaded  int
}

func (s *accountingTestStorage) Ready() bool { return s.available }
func (s *accountingTestStorage) Upload(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	_, err := io.Copy(io.Discard, body)
	s.mu.Lock()
	s.uploaded++
	s.mu.Unlock()
	return err
}
func (s *accountingTestStorage) Presign(_ context.Context, key, _, _ string, _ time.Duration) (string, error) {
	return "https://private.example.test/" + key + "?signature=short", nil
}

type accountingFixture struct {
	desktop            *desktopRepositoryFixture
	router             *gin.Engine
	storage            *accountingTestStorage
	receivingAccountID string
	orderID            string
}

func newAccountingFixture(t *testing.T) *accountingFixture {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	f := newDesktopRepositoryFixture(t, "accounting", 10)
	cfg := &config.Config{}
	cfg.Desktop.Enabled = true
	cfg.Totp.EncryptionKey = strings.Repeat("ab", 32)
	cfg.Totp.EncryptionKeyConfigured = true
	encryptor, err := NewAESEncryptor(cfg)
	require.NoError(t, err)
	storage := &accountingTestStorage{available: true}
	desktop := service.NewDesktopService(f.repo, nil, nil, nil, nil, nil, cfg, nil, nil, nil)
	engine := accounting.NewEngine(integrationDB, desktop, storage, encryptor, cfg)
	handler := accounting.NewHandler(engine)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		id := f.user.ID
		if value := c.GetHeader("X-Test-Actor"); value != "" {
			id, _ = strconv.ParseInt(value, 10, 64)
		}
		if c.GetHeader("X-No-Web-Session") != "true" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		}
		role := c.GetHeader("X-Test-Role")
		if c.GetHeader("Authorization") == "Bearer accounting-qa-user" {
			role = "user"
		}
		if c.GetHeader("Authorization") == "Bearer accounting-qa-finance" {
			role = "finance"
		}
		if role == "" {
			role = "admin"
		}
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	handler.RegisterAdmin(router.Group("/api/v1/admin"))
	handler.RegisterOrganization(router.Group("/api/v1"))
	t.Cleanup(func() {
		ctx := context.Background()
		for _, query := range []string{
			"DELETE FROM accounting_operation_results WHERE actor_id=$1",
			"DELETE FROM accounting_statements WHERE organization_id=$1",
			"DELETE FROM accounting_periods WHERE organization_id=$1",
			"DELETE FROM accounting_external_actions WHERE organization_id=$1",
			"DELETE FROM accounting_receipt_settlements WHERE organization_id=$1",
			"DELETE FROM accounting_cost_allocations WHERE organization_id=$1",
			"DELETE FROM accounting_fx_exchanges WHERE organization_id=$1",
			"DELETE FROM accounting_receivables WHERE organization_id=$1",
			"DELETE FROM accounting_position_events WHERE organization_id=$1",
			"DELETE FROM accounting_position_baselines WHERE organization_id=$1",
			"DELETE FROM accounting_organization_contacts WHERE organization_id=$1",
			"DELETE FROM accounting_receiving_accounts WHERE created_by=$1",
			"DELETE FROM accounting_order_items WHERE organization_id=$1",
			"DELETE FROM accounting_orders WHERE organization_id=$1",
			"DELETE FROM accounting_funding WHERE transaction_id IN (SELECT id FROM accounting_vcc_transactions WHERE organization_id=$1)",
			"DELETE FROM accounting_allocations WHERE organization_id=$1",
			"DELETE FROM accounting_reconciliation WHERE organization_id=$1",
			"DELETE FROM accounting_attachments WHERE organization_id=$1",
			"DELETE FROM accounting_adjustments WHERE organization_id=$1",
			"DELETE FROM accounting_reversals WHERE organization_id=$1",
			"DELETE FROM accounting_audit_logs WHERE organization_id=$1",
			"DELETE FROM accounting_expense_payments WHERE organization_id=$1",
			"DELETE FROM accounting_vcc_transactions WHERE organization_id=$1",
			"DELETE FROM accounting_transfers WHERE organization_id=$1",
			"DELETE FROM accounting_vcc_cards WHERE organization_id=$1",
			"DELETE FROM accounting_contracts WHERE organization_id=$1",
			"DELETE FROM accounting_invoices WHERE organization_id=$1",
			"DELETE FROM accounting_vendor_invoices WHERE organization_id=$1",
			"DELETE FROM accounting_expenses WHERE organization_id=$1",
		} {
			value := f.organization.ID
			if strings.Contains(query, "operation_results") || strings.Contains(query, "accounting_receiving_accounts") {
				value = f.user.ID
			}
			_, cleanupErr := integrationDB.ExecContext(ctx, query, value)
			require.NoError(t, cleanupErr)
		}
	})
	return &accountingFixture{desktop: f, router: router, storage: storage}
}

func TestAccountingOrganizationContactsAndRecordSnapshots(t *testing.T) {
	f := newAccountingFixture(t)
	input := accounting.OrganizationContactInput{
		OrganizationID: f.desktop.organization.PublicID,
		Name:           "Finance Contact",
		Role:           "FINANCE",
		Phone:          "+8613800000000",
		Email:          "finance@example.test",
		WeChat:         "finance-contact",
		IsPrimary:      true,
	}
	var contact accounting.OrganizationContact
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "organization-contacts", uuid.NewString(), input)), &contact))
	require.Equal(t, input.Name, contact.Name)
	require.True(t, contact.IsPrimary)
	receivingAccountID := f.receivingAccount(t)

	transfer := f.create(t, "transfers", "100", map[string]any{
		"payer_name":           "Customer",
		"contact_id":           contact.ID,
		"receiving_account_id": receivingAccountID,
		"sales_amount":         "100",
		"tax_inclusive":        true,
		"tax_rate":             "0",
		"bank_reference":       uuid.NewString(),
	}, "")
	require.Equal(t, "Finance Contact", transfer.Details["contact_name"])
	require.Equal(t, "finance@example.test", transfer.Details["contact_email"])
	require.Equal(t, "6225888800001234", transfer.Details["receiving_account"])
	require.Equal(t, "Test Bank", transfer.Details["receiving_bank_name"])

	input.Name = "Updated Contact"
	input.Email = "updated@example.test"
	accountingData(t, f.request("PATCH", "organization-contacts/"+contact.ID, uuid.NewString(), input))
	accountingData(t, f.request("PATCH", "receiving-accounts/"+receivingAccountID, uuid.NewString(), accounting.ReceivingAccountInput{
		Currency:    "CNY",
		AccountName: f.desktop.organization.Name,
		BankName:    "Updated Bank",
		BankAccount: "6225888800009999",
		BankBranch:  "Updated Branch",
	}))
	var stored accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "transfers/"+transfer.ID, "", nil)), &stored))
	require.Equal(t, "Finance Contact", stored.Details["contact_name"])
	require.Equal(t, "finance@example.test", stored.Details["contact_email"])
	require.Equal(t, "6225888800001234", stored.Details["receiving_account"])
	require.Equal(t, "Test Bank", stored.Details["receiving_bank_name"])

	contract := f.create(t, "contracts", "100", map[string]any{
		"name":            "Sales Contract",
		"contract_number": uuid.NewString(),
		"contact_id":      contact.ID,
	}, "")
	require.Equal(t, f.desktop.organization.Name, contract.Details["party_name"])
	require.Equal(t, "Updated Contact", contract.Details["contact_name"])
	require.Equal(t, "updated@example.test", contract.Details["contact_email"])
}
func (f *accountingFixture) request(method, path, key string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, "/api/v1/admin/accounting/"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}
func accountingData(t *testing.T, res *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	require.Equal(t, 200, res.Code, res.Body.String())
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
	return envelope.Data
}
func (f *accountingFixture) create(t *testing.T, kind, amount string, details map[string]any, parent string) *accounting.Record {
	t.Helper()
	in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: amount, Details: details, ParentID: parent}
	if kind == "vcc-cards" || kind == "vcc-transactions" {
		in.OrderID = f.order(t)
	}
	if kind == "vcc-cards" && details["card_number"] == nil {
		details["card_number"] = testCardNumber()
	}
	res := f.request("POST", kind, uuid.NewString(), in)
	var record accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, res), &record))
	return &record
}

// order returns the fixture's working order; cards and card spend can only be created inside one.
func (f *accountingFixture) order(t *testing.T) string {
	t.Helper()
	if f.orderID != "" {
		return f.orderID
	}
	var order accounting.Order
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "orders", uuid.NewString(), map[string]string{"organization_id": f.desktop.organization.PublicID, "name": "Fixture order"})), &order))
	f.orderID = order.ID
	return order.ID
}

// testCardNumber returns a distinct 16-digit number so card fingerprints never collide between tests.
func testCardNumber() string {
	digits := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'f' {
			return '0' + (r-'a')%10
		}
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, uuid.NewString())
	return "4" + digits[:15]
}
func (f *accountingFixture) receivingAccount(t *testing.T) string {
	t.Helper()
	if f.receivingAccountID != "" {
		return f.receivingAccountID
	}
	input := accounting.ReceivingAccountInput{
		Currency:    "CNY",
		AccountName: f.desktop.organization.Name,
		BankName:    "Test Bank",
		BankAccount: "6225888800001234",
		BankBranch:  "Test Branch",
	}
	var account accounting.ReceivingAccount
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "receiving-accounts", uuid.NewString(), input)), &account))
	f.receivingAccountID = account.ID
	return account.ID
}
func (f *accountingFixture) transfer(t *testing.T, total string) *accounting.Record {
	return f.create(t, "transfers", total, map[string]any{"payer_name": "Customer", "receiving_account_id": f.receivingAccount(t), "sales_amount": total, "tax_inclusive": true, "tax_rate": "0.06", "bank_reference": uuid.NewString(), "remark": "INTERNAL-REMARK"}, "")
}
func (f *accountingFixture) card(t *testing.T) *accounting.Record {
	return f.create(t, "vcc-cards", "0", map[string]any{"alias": "AI card", "provider": "INTERNAL-PROVIDER"}, "")
}
func (f *accountingFixture) transaction(t *testing.T, card *accounting.Record, kind, total, original string) *accounting.Record {
	return f.create(t, "vcc-transactions", total, map[string]any{"transaction_type": kind, "merchant": "AI merchant", "provider_transaction_id": uuid.NewString(), "original_transaction_id": original}, card.ID)
}
func (f *accountingFixture) confirm(t *testing.T, kind string, r *accounting.Record) {
	t.Helper()
	accountingData(t, f.request("POST", kind+"/"+r.ID+"/confirm", uuid.NewString(), map[string]any{}))
}
func (f *accountingFixture) proof(t *testing.T, kind string, r *accounting.Record) string {
	t.Helper()
	body := []byte("%PDF-1.7\nInvoice proof\n%%EOF")
	sha := fmt.Sprintf("%x", sha256.Sum256(body))
	payload := map[string]any{"organization_id": r.OrganizationID, "object_kind": kind, "object_id": r.ID, "purpose": "INVOICE", "filename": "proof.pdf", "content_type": "application/pdf", "size_bytes": len(body), "sha256": sha}
	var intent struct {
		Attachment accounting.Attachment `json:"attachment"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload)), &intent))
	id := intent.Attachment.ID
	req := httptest.NewRequest("PUT", "/api/v1/admin/accounting/attachments/"+id+"/content", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.Header.Set("Content-Type", "application/pdf")
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	accountingData(t, res)
	accountingData(t, f.request("POST", "attachments/"+id+"/complete", uuid.NewString(), map[string]any{}))
	return id
}

func TestAccountingFundingRefundCloseAndIdempotency(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	path := "transfers/" + receipt.ID + "/allocations"
	key := uuid.NewString()
	payload := map[string]string{"card_id": card.ID, "amount": "100"}
	first := f.request("POST", path, key, payload)
	body := accountingData(t, first)
	replay := f.request("POST", path, key, payload)
	require.JSONEq(t, string(body), string(accountingData(t, replay)))
	require.Equal(t, "true", replay.Header().Get("X-Idempotency-Replayed"))
	payload["amount"] = "99"
	require.Equal(t, 409, f.request("POST", path, key, payload).Code)
	consume := f.transaction(t, card, "CONSUME", "70", "")
	f.confirm(t, "vcc-transactions", consume)
	fee := f.transaction(t, card, "FEE", "10", "")
	f.confirm(t, "vcc-transactions", fee)
	refund := f.transaction(t, card, "REFUND", "20", consume.ID)
	f.confirm(t, "vcc-transactions", refund)
	require.Equal(t, 409, f.request("POST", "vcc-transactions/"+consume.ID+"/void", uuid.NewString(), map[string]string{"reason": "wrong"}).Code)
	accountingData(t, f.request("POST", "vcc-cards/"+card.ID+"/close", uuid.NewString(), map[string]any{}))
	var used, released string
	require.NoError(t, integrationDB.QueryRow("SELECT used_amount::text,released_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&used, &released))
	require.Equal(t, "60.000000", used)
	require.Equal(t, "40.000000", released)
	late := f.transaction(t, card, "REFUND", "10", consume.ID)
	f.confirm(t, "vcc-transactions", late)
	require.NoError(t, integrationDB.QueryRow("SELECT released_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&released))
	require.Equal(t, "50.000000", released)
	excessive := f.transaction(t, card, "REFUND", "41", consume.ID)
	require.Equal(t, 409, f.request("POST", "vcc-transactions/"+excessive.ID+"/confirm", uuid.NewString(), map[string]any{}).Code)
	replacement := f.card(t)
	accountingData(t, f.request("POST", path, uuid.NewString(), map[string]string{"card_id": replacement.ID, "amount": "50"}))
	require.Equal(t, 409, f.request("POST", path, uuid.NewString(), map[string]string{"card_id": replacement.ID, "amount": "0.01"}).Code)
	require.Equal(t, 409, f.request("POST", "transfers/"+receipt.ID+"/void", uuid.NewString(), map[string]string{"reason": "wrong"}).Code)
	require.Equal(t, 409, f.request("PATCH", "transfers/"+receipt.ID, uuid.NewString(), accounting.Input{Amount: "100", Version: receipt.Version}).Code)
}
func TestAccountingConcurrentAllocationAndDuplicateReferences(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "60"})
			codes <- res.Code
		}()
	}
	wg.Wait()
	close(codes)
	success := 0
	for code := range codes {
		if code == 200 {
			success++
		} else {
			require.Equal(t, 409, code)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 409, f.request("POST", "transfers", uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: receipt.Amount, Details: receipt.Details}).Code)
	r := f.transaction(t, card, "CONSUME", "1", "")
	require.Equal(t, 409, f.request("POST", "vcc-transactions", uuid.NewString(), accounting.Input{OrganizationID: r.OrganizationID, ParentID: card.ID, Currency: r.Currency, Amount: r.Amount, Details: r.Details, OrderID: f.order(t)}).Code)
	require.Equal(t, 400, f.request("POST", "transfers", "", accounting.Input{OrganizationID: receipt.OrganizationID, Currency: "CNY", Amount: "1", Details: receipt.Details}).Code)
}
func TestAccountingOverdraftAndCurrencyIsolation(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	consume := f.transaction(t, card, "CONSUME", "130", "")
	f.confirm(t, "vcc-transactions", consume)
	accountingData(t, f.request("POST", "vcc-cards/"+card.ID+"/close", uuid.NewString(), map[string]any{}))
	refund := f.transaction(t, card, "REFUND", "20", consume.ID)
	f.confirm(t, "vcc-transactions", refund)
	var released string
	require.NoError(t, integrationDB.QueryRow("SELECT released_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&released))
	require.Equal(t, "0.000000", released)
	refund = f.transaction(t, card, "REFUND", "30", consume.ID)
	f.confirm(t, "vcc-transactions", refund)
	require.NoError(t, integrationDB.QueryRow("SELECT released_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&released))
	require.Equal(t, "20.000000", released)
	require.Equal(t, 400, f.request("POST", "vcc-transactions", uuid.NewString(), accounting.Input{OrganizationID: receipt.OrganizationID, ParentID: card.ID, Currency: "USD", Amount: "1", Details: map[string]any{"transaction_type": "REFUND", "original_transaction_id": consume.ID, "merchant": "Vendor", "provider_transaction_id": uuid.NewString()}, OrderID: f.order(t)}).Code)
}
func TestAccountingOrganizationReadOnlySharingAndSecrets(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	file := f.proof(t, "transfers", receipt)
	read := func(path string, actor int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/organization/accounting/"+path, nil)
		req.Header.Set("X-Test-Role", "user")
		req.Header.Set("X-Test-Actor", strconv.FormatInt(actor, 10))
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	// Drafts and their shared proofs stay internal until the receipt is confirmed.
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	draft := read("transfers", f.desktop.user.ID)
	require.Equal(t, 200, draft.Code)
	require.NotContains(t, draft.Body.String(), receipt.ID)
	require.Equal(t, 404, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	require.Equal(t, 404, read("attachments?object_kind=transfers&object_id="+receipt.ID, f.desktop.user.ID).Code)
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": false}))
	f.confirm(t, "transfers", receipt)
	own := read("transfers?organization_id=forged", f.desktop.user.ID)
	require.Equal(t, 200, own.Code)
	require.Contains(t, own.Body.String(), receipt.ID)
	require.NotContains(t, own.Body.String(), "INTERNAL-BANK")
	require.NotContains(t, own.Body.String(), "INTERNAL-REMARK")
	require.Equal(t, 404, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	accountingData(t, f.request("PATCH", "attachments/"+file+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	require.Equal(t, 200, read("attachments/"+file+"/download-url", f.desktop.user.ID).Code)
	other := newDesktopRepositoryFixture(t, "acctother", 1)
	outside := read("transfers?organization_id="+receipt.OrganizationID, other.user.ID)
	require.Equal(t, 200, outside.Code)
	require.NotContains(t, outside.Body.String(), receipt.ID)
	require.Equal(t, 404, read("attachments/"+file+"/download-url", other.user.ID).Code)
	card := f.create(t, "vcc-cards", "0", map[string]any{"alias": "Secure", "provider": "INTERNAL-PROVIDER", "card_number": "4111111111111111", "cvv": "123", "expiry": "12/30"}, "")
	body := read("vcc-card", f.desktop.user.ID).Body.String()
	require.Contains(t, body, "1111")
	for _, secret := range []string{"4111111111111111", "INTERNAL-PROVIDER", "cvv", "expiry"} {
		require.NotContains(t, body, secret)
	}
	var cipher, details string
	require.NoError(t, integrationDB.QueryRow("SELECT secret_ciphertext,details::text FROM accounting_vcc_cards WHERE id=$1", card.ID).Scan(&cipher, &details))
	require.NotEmpty(t, cipher)
	require.NotContains(t, cipher, "4111111111111111")
	require.NotContains(t, details, "4111111111111111")
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/summary", nil)
	req.Header.Set("X-No-Web-Session", "true")
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 401, res.Code)
	_, err := integrationDB.Exec("UPDATE desktop_organizations SET deleted_at=NOW() WHERE id=$1", f.desktop.organization.ID)
	require.NoError(t, err)
	require.Equal(t, 404, read("transfers", f.desktop.user.ID).Code)
}
func TestAccountingPaymentsAIInvoiceAndAdjustments(t *testing.T) {
	f := newAccountingFixture(t)
	expense := f.create(t, "expenses", "100", map[string]any{"category": "AI_VENDOR", "supplier": "Vendor", "payment_account": "Bank", "ai_vendor": true}, "")
	proofID := f.proof(t, "expenses", expense)
	payment := accounting.Input{Amount: "60", Currency: "CNY", Details: map[string]any{"payment_account": "Bank", "transaction_number": uuid.NewString(), "proof_attachment_id": proofID}}
	require.Equal(t, 400, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment).Code)
	invoice := f.create(t, "vendor-invoices", "100", map[string]any{"vendor": "Vendor", "period_start": "2026-10-01", "period_end": "2026-10-31", "expense_id": expense.ID}, "")
	f.proof(t, "vendor-invoices", invoice)
	f.confirm(t, "vendor-invoices", invoice)
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "expenses/"+expense.ID, "", nil)), &current))
	current.Details["vendor_invoice_id"] = invoice.ID
	accountingData(t, f.request("PATCH", "expenses/"+expense.ID, uuid.NewString(), accounting.Input{Amount: "100", Currency: "CNY", Version: current.Version, Details: current.Details}))
	var first accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment)), &first))
	require.Equal(t, 409, f.request("POST", "expenses/"+expense.ID+"/void", uuid.NewString(), map[string]string{"reason": "Wrong"}).Code)
	payment.Amount = "50"
	payment.Details["transaction_number"] = uuid.NewString()
	require.Equal(t, 409, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment).Code)
	payment.Details["override"] = true
	payment.Details["reason"] = "Approved overpayment"
	accountingData(t, f.request("POST", "expenses/"+expense.ID+"/payments", uuid.NewString(), payment))
	accountingData(t, f.request("POST", "expense-payments/"+first.ID+"/void", uuid.NewString(), map[string]string{"reason": "Reverse first payment"}))
	accountingData(t, f.request("POST", "reconciliation/adjustments", uuid.NewString(), map[string]string{"object_kind": "expenses", "object_id": expense.ID, "amount": "10", "currency": "CNY", "reason": "Correct amount"}))
	require.Equal(t, 409, f.request("POST", "vendor-invoices/"+invoice.ID+"/void", uuid.NewString(), map[string]string{"reason": "Wrong"}).Code)
}
func TestAccountingAuditRollbackAndSafeRetry(t *testing.T) {
	f := newAccountingFixture(t)
	ctx := context.Background()
	receivingAccountID := f.receivingAccount(t)
	name := fmt.Sprintf("accounting_audit_fail_%d", f.desktop.user.ID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf("CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.actor_id=%d THEN RAISE EXCEPTION 'test audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER %s BEFORE INSERT ON accounting_audit_logs FOR EACH ROW EXECUTE FUNCTION %s()", name, f.desktop.user.ID, name, name))
	require.NoError(t, err)
	cleanup := func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+name+" ON accounting_audit_logs; DROP FUNCTION IF EXISTS "+name+"()")
	}
	t.Cleanup(cleanup)
	key := uuid.NewString()
	in := accounting.Input{OrganizationID: f.desktop.organization.PublicID, Currency: "CNY", Amount: "100", Details: map[string]any{"payer_name": "Customer", "receiving_account_id": receivingAccountID, "sales_amount": "100", "tax_inclusive": true, "tax_rate": "0", "bank_reference": uuid.NewString()}}
	require.Equal(t, 500, f.request("POST", "transfers", key, in).Code)
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_transfers WHERE organization_id=$1", f.desktop.organization.ID).Scan(&count))
	require.Zero(t, count)
	cleanup()
	first := f.request("POST", "transfers", key, in)
	accountingData(t, first)
	replayed := f.request("POST", "transfers", key, in)
	require.JSONEq(t, first.Body.String(), replayed.Body.String())
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM accounting_transfers WHERE organization_id=$1", f.desktop.organization.ID).Scan(&count))
	require.Equal(t, 1, count)
}

// TestAccountingBrowserFixture exposes only disposable test data for manual browser acceptance.
func TestAccountingBrowserFixture(t *testing.T) {
	if os.Getenv("SUB2API_ACCOUNTING_BROWSER_QA") != "1" {
		t.Skip("browser fixture is opt-in")
	}
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "58000")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "30000"}))
	consume := f.transaction(t, card, "CONSUME", "2480", "")
	f.confirm(t, "vcc-transactions", consume)
	desktop := service.NewDesktopService(f.desktop.repo, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil)
	f.router.GET("/api/v1/desktop/organization", func(c *gin.Context) {
		org, err := desktop.GetManagedOrganization(c.Request.Context(), f.desktop.user.ID)
		if response.ErrorFrom(c, err) {
			return
		}
		response.Success(c, map[string]any{"public_id": org.PublicID, "code": org.Code, "name": org.Name, "status": org.Status, "member_count": 0, "member_limit": 10, "gateway_user": map[string]any{"id": f.desktop.user.ID, "email": "accounting-qa@example.test"}})
	})
	f.router.GET("/api/v1/settings/public", func(c *gin.Context) {
		response.Success(c, map[string]any{"site_name": "Accounting QA", "desktop_enabled": true, "subscription_enabled": false, "payment_enabled": false, "backend_mode_enabled": false, "public_dashboard_enabled": false, "custom_menu_items": []any{}})
	})
	f.router.GET("/api/v1/auth/me", func(c *gin.Context) {
		role := "admin"
		if c.GetHeader("Authorization") == "Bearer accounting-qa-user" {
			role = "user"
		}
		if c.GetHeader("Authorization") == "Bearer accounting-qa-finance" {
			role = "finance"
		}
		response.Success(c, map[string]any{"id": f.desktop.user.ID, "role": role, "email": "accounting-qa@example.test", "username": "Accounting QA", "status": "active", "balance": 0, "concurrency": 1, "run_mode": "standard"})
	})
	f.router.GET("/setup/status", func(c *gin.Context) { c.JSON(200, map[string]any{"needs_setup": false}) })
	f.router.GET("/api/v1/admin/compliance", func(c *gin.Context) { response.Success(c, map[string]any{"required": false}) })
	f.router.GET("/api/v1/admin/settings", func(c *gin.Context) {
		response.Success(c, map[string]any{"ops_monitoring_enabled": false, "custom_menu_items": []any{}})
	})
	f.router.GET("/api/v1/admin/payment/config", func(c *gin.Context) { response.Success(c, map[string]any{"enabled": false}) })
	f.router.GET("/api/v1/subscription-accounts", func(c *gin.Context) { response.Paginated(c, []any{}, 0, 1, 20) })
	f.router.GET("/api/v1/announcements", func(c *gin.Context) { response.Success(c, []any{}) })
	f.router.GET("/api/v1/version", func(c *gin.Context) { response.Success(c, map[string]any{"version": "accounting-qa"}) })
	server := httptest.NewUnstartedServer(f.router)
	if port := os.Getenv("SUB2API_ACCOUNTING_QA_PORT"); port != "" {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
		require.NoError(t, err)
		require.NoError(t, server.Listener.Close())
		server.Listener = listener
	}
	server.Start()
	defer server.Close()
	state, err := json.Marshal(map[string]any{"url": server.URL, "user_id": f.desktop.user.ID, "organization_id": f.desktop.organization.PublicID})
	require.NoError(t, err)
	qaDir := os.Getenv("SUB2API_ACCOUNTING_QA_DIR")
	if qaDir == "" {
		qaDir = "/tmp"
	}
	require.NoError(t, os.WriteFile(filepath.Join(qaDir, "sub2api-accounting-qa.json"), state, 0600))
	t.Logf("Accounting browser fixture ready: %s", server.URL)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	deadline := time.After(60 * time.Minute)
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(qaDir, "sub2api-accounting-qa-complete")); err == nil {
				return
			}
		case <-deadline:
			t.Fatal("browser fixture timed out")
		}
	}
}

func TestAccountingClosedRefundDoesNotReleaseOtherFeeDeficit(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	consume := f.transaction(t, card, "CONSUME", "100", "")
	f.confirm(t, "vcc-transactions", consume)
	fee := f.transaction(t, card, "FEE", "10", "")
	f.confirm(t, "vcc-transactions", fee)
	accountingData(t, f.request("POST", "vcc-cards/"+card.ID+"/close", uuid.NewString(), map[string]any{}))
	refund := f.transaction(t, card, "REFUND", "20", consume.ID)
	f.confirm(t, "vcc-transactions", refund)
	var released string
	require.NoError(t, integrationDB.QueryRow("SELECT released_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&released))
	require.Equal(t, "10.000000", released)
	var uncovered string
	require.NoError(t, integrationDB.QueryRow("SELECT COALESCE(SUM(amount-refunded_amount),0)::text FROM accounting_funding WHERE transaction_id=$1 AND allocation_id IS NULL", fee.ID).Scan(&uncovered))
	require.Equal(t, "0.000000", uncovered)
	replacement := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": replacement.ID, "amount": "10"}))
	require.Equal(t, 409, f.request("POST", "vcc-transactions/"+refund.ID+"/void", uuid.NewString(), map[string]string{"reason": "refund correction"}).Code)
}
func TestAccountingNewFundingCoversExistingOverdraft(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	consume := f.transaction(t, card, "CONSUME", "50", "")
	f.confirm(t, "vcc-transactions", consume)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	var used string
	require.NoError(t, integrationDB.QueryRow("SELECT used_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&used))
	require.Equal(t, "50.000000", used)
	refund := f.transaction(t, card, "REFUND", "20", consume.ID)
	f.confirm(t, "vcc-transactions", refund)
	require.NoError(t, integrationDB.QueryRow("SELECT used_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&used))
	require.Equal(t, "30.000000", used)
	accountingData(t, f.request("POST", "vcc-transactions/"+refund.ID+"/void", uuid.NewString(), map[string]string{"reason": "correction"}))
	require.NoError(t, integrationDB.QueryRow("SELECT used_amount::text FROM accounting_allocations WHERE card_id=$1", card.ID).Scan(&used))
	require.Equal(t, "50.000000", used)
}
func TestAccountingReconciliationPaginationAndSummaryPeriods(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	_, err := integrationDB.Exec("UPDATE accounting_vcc_cards SET occurred_at='2020-01-01' WHERE id=$1", card.ID)
	require.NoError(t, err)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "60"}))
	path := "summary?organization_id=" + receipt.OrganizationID + "&from=2026-10-01T00:00:00Z&to=2026-11-01T00:00:00Z"
	var summary []accounting.CurrencySummary
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", path, "", nil)), &summary))
	require.Len(t, summary, 1)
	require.Equal(t, "60.000000", summary[0].Allocated)
	require.Equal(t, "40.000000", summary[0].Unallocated)
	payload := map[string]string{"object_kind": "transfers", "object_id": receipt.ID, "currency": "CNY", "amount": "99", "reference": "BANK", "reason": "Bank fee"}
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), payload))
	var page struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "reconciliation?organization_id="+receipt.OrganizationID+"&status=DIFFERENCE&page_size=1", "", nil)), &page))
	require.Len(t, page.Items, 1)
	require.Equal(t, 1, page.Total)
	payload["amount"] = "100"
	payload["reason"] = ""
	accountingData(t, f.request("POST", "reconciliation/match", uuid.NewString(), payload))
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "reconciliation?organization_id="+receipt.OrganizationID+"&status=DIFFERENCE", "", nil)), &page))
	require.Zero(t, page.Total)
}
func TestAccountingAttachmentFailureExpiryAndSoftDeletion(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	file := f.proof(t, "transfers", receipt)
	deleteKey := uuid.NewString()
	first := accountingData(t, f.request("DELETE", "attachments/"+file, deleteKey, map[string]any{}))
	require.JSONEq(t, string(first), string(accountingData(t, f.request("DELETE", "attachments/"+file, deleteKey, map[string]any{}))))
	require.Equal(t, 404, f.request("GET", "attachments/"+file+"/download-url", "", nil).Code)
	var deleted bool
	require.NoError(t, integrationDB.QueryRow("SELECT deleted_at IS NOT NULL FROM accounting_attachments WHERE id=$1", file).Scan(&deleted))
	require.True(t, deleted)
	body := []byte("%PDF-1.7\nproof\n%%EOF")
	payload := map[string]any{"organization_id": receipt.OrganizationID, "object_kind": "transfers", "object_id": receipt.ID, "purpose": "PROOF", "filename": "receipt.pdf", "content_type": "application/pdf", "size_bytes": len(body), "sha256": fmt.Sprintf("%x", sha256.Sum256(body))}
	var intent struct {
		Attachment accounting.Attachment `json:"attachment"`
	}
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload)), &intent))
	_, err := integrationDB.Exec("UPDATE accounting_attachments SET expires_at=NOW()-interval '1 second' WHERE id=$1", intent.Attachment.ID)
	require.NoError(t, err)
	req := httptest.NewRequest("PUT", "/api/v1/admin/accounting/attachments/"+intent.Attachment.ID+"/content", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", uuid.NewString())
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 409, res.Code)
	payload["size_bytes"] = accounting.MaxAttachmentBytes + 1
	require.Equal(t, 400, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload).Code)
	f.storage.available = false
	require.Equal(t, 503, f.request("POST", "attachments/upload-intents", uuid.NewString(), payload).Code)
}

func TestAccountingBindingReassignmentRevokesOldAccountImmediately(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	replacement := mustCreateUser(t, integrationEntClient, &service.User{Email: "accounting-replacement-" + uuid.NewString() + "@example.test"})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("UPDATE desktop_organizations SET gateway_user_id=$2 WHERE id=$1", f.desktop.organization.ID, f.desktop.user.ID)
		_, _ = integrationDB.Exec("DELETE FROM users WHERE id=$1", replacement.ID)
	})
	_, _, err := f.desktop.repo.UpdateOrganization(context.Background(), f.desktop.organization.PublicID, service.DesktopUpdateOrganizationInput{GatewayUserID: &replacement.ID})
	require.NoError(t, err)
	read := func(actor int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/organization/accounting/transfers?organization_id="+receipt.OrganizationID, nil)
		req.Header.Set("X-Test-Actor", strconv.FormatInt(actor, 10))
		req.Header.Set("X-Test-Role", "user")
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	require.Equal(t, 404, read(f.desktop.user.ID).Code)
	current := read(replacement.ID)
	require.Equal(t, 200, current.Code)
	require.Contains(t, current.Body.String(), receipt.ID)
}
func TestAccountingCardIdentityCannotOverwriteFundingHistory(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	accountingData(t, f.request("POST", "transfers/"+receipt.ID+"/allocations", uuid.NewString(), map[string]string{"card_id": card.ID, "amount": "100"}))
	require.Equal(t, 409, f.request("PATCH", "vcc-cards/"+card.ID, uuid.NewString(), accounting.Input{Version: card.Version, Details: map[string]any{"provider": "different-provider"}}).Code)
}
func TestAccountingContractAndInvoiceSharingLifecycle(t *testing.T) {
	f := newAccountingFixture(t)
	contract := f.create(t, "contracts", "100", map[string]any{"name": "Contract", "contract_number": uuid.NewString(), "party_name": "Customer", "tax_rate": "0.06", "tax_inclusive": true}, "")
	f.proof(t, "contracts", contract)
	f.confirm(t, "contracts", contract)
	accountingData(t, f.request("PATCH", "contracts/"+contract.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	accountingData(t, f.request("POST", "contracts/"+contract.ID+"/expire", uuid.NewString(), map[string]any{}))
	sales := f.create(t, "invoices", "106", map[string]any{"invoice_type": "SALES", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Us", "recipient_name": "Customer", "net_amount": "100", "tax_amount": "6"}, "")
	proof := f.proof(t, "invoices", sales)
	f.confirm(t, "invoices", sales)
	accountingData(t, f.request("PATCH", "attachments/"+proof+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	req := httptest.NewRequest("GET", "/api/v1/organization/accounting/attachments/"+proof+"/download-url", nil)
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	require.Equal(t, 404, res.Code)
	accountingData(t, f.request("PATCH", "invoices/"+sales.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}))
	res = httptest.NewRecorder()
	f.router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/organization/accounting/attachments/"+proof+"/download-url", nil))
	require.Equal(t, 200, res.Code)
	internal := f.create(t, "invoices", "106", map[string]any{"invoice_type": "PURCHASE", "invoice_code": "CODE", "invoice_number": uuid.NewString(), "issuer_name": "Vendor", "recipient_name": "Us", "net_amount": "100", "tax_amount": "6"}, "")
	require.Equal(t, 409, f.request("PATCH", "invoices/"+internal.ID+"/sharing", uuid.NewString(), map[string]bool{"customer_visible": true}).Code)
	require.Equal(t, 409, f.request("POST", "invoices", uuid.NewString(), accounting.Input{OrganizationID: sales.OrganizationID, Currency: sales.Currency, Amount: sales.Amount, Details: sales.Details}).Code)
}
