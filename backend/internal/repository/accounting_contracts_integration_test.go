//go:build integration

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Sales contracts are document records of an organization; orders name the contract they are done under.
func TestAccountingOrdersNameASalesContract(t *testing.T) {
	f := newAccountingFixture(t)
	org := f.desktop.organization.PublicID
	in := accounting.Input{OrganizationID: org, Currency: "CNY", Details: map[string]any{"name": "Framework agreement", "contract_number": "HT-" + uuid.NewString()[:8], "signed_at": "2026-10-01"}}
	var contract accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "contracts", uuid.NewString(), in)), &contract), "the amount is optional")
	require.True(t, decimal.RequireFromString(contract.Amount).IsZero())
	f.confirm(t, "contracts", &contract)
	var signed accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "contracts/"+contract.ID, "", nil)), &signed))
	require.Equal(t, "SIGNED", signed.Status)
	edit := accounting.Input{Currency: "CNY", Amount: "120000", Version: signed.Version, Details: map[string]any{"name": "Framework agreement 2027", "contract_number": signed.Details["contract_number"]}}
	require.Equal(t, 200, f.request("PATCH", "contracts/"+contract.ID, uuid.NewString(), edit).Code, "signed contracts stay editable")
	negative := accounting.Input{OrganizationID: org, Currency: "CNY", Amount: "-1", Details: map[string]any{"name": "Bad", "contract_number": uuid.NewString()}}
	require.Equal(t, 400, f.request("POST", "contracts", uuid.NewString(), negative).Code)

	var order accounting.Order
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "orders", uuid.NewString(), map[string]any{"organization_id": org, "name": "Seats", "contract_id": contract.ID})), &order))
	require.NotNil(t, order.Contract)
	require.Equal(t, "Framework agreement 2027", order.Contract.Name)
	require.Equal(t, "SIGNED", order.Contract.Status)

	other := newAccountingFixture(t)
	var foreign accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, other.request("POST", "contracts", uuid.NewString(), accounting.Input{OrganizationID: other.desktop.organization.PublicID, Currency: "CNY", Details: map[string]any{"name": "Other", "contract_number": uuid.NewString()}})), &foreign))
	update := func(contractID string, version int64) int {
		return f.request("PATCH", "orders/"+order.ID, uuid.NewString(), map[string]any{"name": order.Name, "version": version, "contract_id": contractID}).Code
	}
	require.Equal(t, 400, update(foreign.ID, order.Version), "only the order's own organization's contracts")

	// A contract an order names cannot be voided; clearing the link frees it.
	require.Equal(t, 409, f.request("POST", "contracts/"+contract.ID+"/void", uuid.NewString(), map[string]string{"reason": "Replaced"}).Code)
	require.Equal(t, 200, update("", order.Version))
	current := f.readOrder(t, order.ID)
	require.Nil(t, current.Contract)
	accountingData(t, f.request("POST", "contracts/"+contract.ID+"/void", uuid.NewString(), map[string]string{"reason": "Replaced"}))
	require.Equal(t, 400, update(contract.ID, current.Version), "voided contracts cannot be named")
}
