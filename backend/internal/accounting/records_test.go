package accounting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
