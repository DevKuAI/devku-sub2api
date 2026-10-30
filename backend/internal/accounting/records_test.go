package accounting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func transferInput(sales, received, rate string, inclusive bool) Input {
	return Input{Amount: received, Currency: "CNY", Details: map[string]any{"payer_name": "Customer", "receiving_account": "Bank", "bank_reference": "bank-one", "sales_amount": sales, "tax_inclusive": inclusive, "tax_rate": rate}}
}
func TestTaxCalculationAndCurrencyPrecision(t *testing.T) {
	for _, tt := range []struct {
		name, sales, received, rate, tax, net, total string
		inclusive                                    bool
	}{{"inclusive", "58000", "58000", "0.06", "3283.02", "54716.98", "58000", true}, {"exclusive", "42000", "44520", "0.06", "2520", "42000", "44520", false}, {"rounding", "0.01", "0.01", "0.06", "0", "0.01", "0.01", true}} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := prepare("transfers", transferInput(tt.sales, tt.received, tt.rate, tt.inclusive), nil)
			require.NoError(t, err)
			require.Equal(t, tt.tax, detail(r, "tax_amount"))
			require.Equal(t, tt.net, detail(r, "net_sales_amount"))
			require.Equal(t, tt.total, detail(r, "receivable_amount"))
		})
	}
	_, err := prepare("transfers", transferInput("100", "99", "0", false), nil)
	require.Error(t, err)
	in := transferInput("100", "99", "0", false)
	in.Details["difference_reason"] = "Bank fee"
	_, err = prepare("transfers", in, nil)
	require.NoError(t, err)
	_, err = number("0.001", "CNY", true)
	require.Error(t, err)
	_, err = number("0.001", "KWD", true)
	require.NoError(t, err)
	_, err = number("1.1", "JPY", true)
	require.Error(t, err)
}
func TestTransferPreservesContractReference(t *testing.T) {
	in := transferInput("100", "100", "0.06", true)
	in.Details["contract_id"] = "13c6bcf6-14e7-4e49-8a24-27e83af7e643"
	r, err := prepare("transfers", in, nil)
	require.NoError(t, err)
	require.Equal(t, in.Details["contract_id"], r.Details["contract_id"])
}
func TestPublicDTOAndQueryWhitelist(t *testing.T) {
	r := &Record{Details: map[string]any{"sales_amount": "100", "receiving_account": "hidden", "remark": "private", "provider_token": "secret"}, Amount: "100"}
	body, err := json.Marshal(r.Public("transfers"))
	require.NoError(t, err)
	require.Contains(t, string(body), "sales_amount")
	for _, secret := range []string{"receiving_account", "hidden", "remark", "private", "provider_token", "secret"} {
		require.NotContains(t, string(body), secret)
	}
	columns := customerColumns("transfers")
	require.NotContains(t, columns, "receiving_account")
	require.NotContains(t, columns, "r.details,")
	require.NotContains(t, customerColumns("vcc-cards"), "provider")
}
func TestCustomerSeesOnlyFinalRecordStates(t *testing.T) {
	for kind, states := range map[string]map[string]bool{
		"transfers":        {"DRAFT": false, "RECEIVED": true, "VOID": false},
		"vcc-cards":        {"ACTIVE": true, "CLOSED": true},
		"vcc-transactions": {"DRAFT": false, "CONFIRMED": true, "VOID": false},
		"contracts":        {"DRAFT": false, "SIGNED": true, "EXPIRED": true, "VOID": false},
		"invoices":         {"DRAFT": false, "VERIFIED": true, "VOID": false},
		"expenses":         {"PAID": false},
	} {
		for status, visible := range states {
			require.Equal(t, visible, customerVisibleStatus(kind, status), "%s %s", kind, status)
		}
	}
	require.Equal(t, " AND r.status IN ('RECEIVED')", customerStatusClause("transfers"))
	require.Equal(t, " AND FALSE", customerStatusClause("expenses"))
}
func TestAccountingRoutesRejectUnprivilegedUsersAndExposeNoOrganizationWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewHandler(&Engine{desktopEnabled: true})
	h.RegisterAdmin(router.Group("/api/v1/admin"))
	h.RegisterOrganization(router.Group("/api/v1"))
	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounting/summary", nil))
	require.Equal(t, http.StatusForbidden, result.Code)
	result = httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/v1/organization/accounting/summary", nil))
	require.Equal(t, http.StatusUnauthorized, result.Code)
	for _, route := range router.Routes() {
		if len(route.Path) >= len("/api/v1/organization/accounting") && route.Path[:len("/api/v1/organization/accounting")] == "/api/v1/organization/accounting" {
			require.Equal(t, http.MethodGet, route.Method)
		}
	}
}
func TestAttachmentValidation(t *testing.T) {
	for _, tt := range []struct {
		name, mime string
		valid      bool
	}{{"receipt.pdf", "application/pdf", true}, {"receipt.png", "image/png", true}, {"receipt.jpg", "image/jpeg", true}, {"receipt.webp", "image/webp", true}, {"receipt.svg", "image/svg+xml", false}, {"receipt.png", "application/pdf", false}} {
		require.Equal(t, tt.valid, validAttachment(tt.name, tt.mime))
	}
}
func TestPermissionMappings(t *testing.T) {
	require.Equal(t, PermissionRead, permissionFor("GET", "/transfers"))
	require.Equal(t, PermissionWrite, permissionFor("POST", "/transfers"))
	require.Equal(t, PermissionConfirm, permissionFor("POST", "/transfers/id/confirm"))
	require.Equal(t, PermissionReconcile, permissionFor("POST", "/reconciliation/match"))
	require.Equal(t, PermissionManage, permissionFor("POST", "/transfers/id/void"))
}

func TestAccountingPermissionsAllowFinanceWithoutAdminPrivileges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{domain.RoleAdmin, domain.RoleFinance, domain.RoleUser, ""} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserRole), role)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			})
			router.Use(requireAccountingPermission)
			router.Handle(method, "/accounting/transfers", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, "/accounting/transfers", nil))
			if role == domain.RoleAdmin || role == domain.RoleFinance {
				require.Equal(t, http.StatusNoContent, w.Code)
			} else {
				require.Equal(t, http.StatusForbidden, w.Code)
			}
		}
	}
}

func TestWorkflowDecimalBoundsAndComputedMetadata(t *testing.T) {
	for _, raw := range []string{"1e100000000", "1e-100000000", "-1", "0"} {
		_, err := parseRate(raw)
		require.Error(t, err)
	}
	_, err := number("1e100000000", "CNY", true)
	require.Error(t, err)
	in := transferInput("100", "100", "0", true)
	in.Details["effective_amount"] = "900000"
	r, err := prepare("transfers", in, nil)
	require.NoError(t, err)
	require.NotContains(t, r.Details, "effective_amount")
	in.Details["funding_type"] = "FX"
	_, err = prepare("transfers", in, nil)
	require.Error(t, err)
	r = &Record{Details: map[string]any{"card_number": "SECRET", "provider_token": "SECRET", "alias": "Card"}}
	require.Equal(t, map[string]any{"alias": "Card"}, auditFields(r, []string{"card_number", "provider_token", "alias"}))
}

func TestAccountingCSVPreservesNumericAdjustmentsAndEscapesFormulaText(t *testing.T) {
	require.Equal(t, "-12.34", csvField("adjustment_amount", "-12.340000"))
	require.Equal(t, "'=SUM(A1:A2)", csvField("remark", "=SUM(A1:A2)"))
	require.Equal(t, "'-invoice", csvField("invoice_number", "-invoice"))
}

func TestCardsNeedCardIDAndLastFourButNoSecrets(t *testing.T) {
	card := func(details map[string]any) error {
		_, err := prepare("vcc-cards", Input{Currency: "USD", Details: details}, nil)
		return err
	}
	valid := map[string]any{"alias": "Seat card", "provider": "SUNRATE", "provider_card_id": "2095346501199990785", "last_four": "3926", "expiry": "2030-12"}
	require.NoError(t, card(valid))
	for field, value := range map[string]any{"provider_card_id": "", "last_four": "39a6", "expiry": "12/30"} {
		details := map[string]any{}
		for k, v := range valid {
			details[k] = v
		}
		details[field] = value
		require.Error(t, card(details), field)
	}
	for _, secret := range []string{"card_number", "cvv", "provider_token"} {
		details := map[string]any{secret: "4111111111111111"}
		for k, v := range valid {
			details[k] = v
		}
		require.Error(t, card(details), secret)
	}
}
