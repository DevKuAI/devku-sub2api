//go:build integration

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *accountingFixture) newVendor(t *testing.T, kind, name string, aliases ...string) accounting.Vendor {
	t.Helper()
	var v accounting.Vendor
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("POST", "vendors", uuid.NewString(), accounting.VendorInput{Kind: kind, Name: name, Aliases: aliases})), &v))
	return v
}

func (f *accountingFixture) rawJSON(t *testing.T, method, path string, payload any) int {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return f.rawRequest(method, path, uuid.NewString(), body).Code
}

func TestAccountingRecordsChooseVendorsFromTheLists(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.order(t)
	org := f.desktop.organization.PublicID
	cardInput := func(provider string) accounting.Input {
		details := map[string]any{"alias": "Seat card", "provider_card_id": testCardID(), "last_four": "4242"}
		if provider != "" {
			details["provider_id"] = provider
		}
		return accounting.Input{OrganizationID: org, Currency: "USD", Amount: "0", OrderID: order, Details: details}
	}
	require.Equal(t, 400, f.rawJSON(t, "POST", "vcc-cards", cardInput("")), "a card must belong to a card provider")
	ai := seededVendor("AI_VENDOR", "OpenAI")
	require.Equal(t, 400, f.rawJSON(t, "POST", "vcc-cards", cardInput(ai)), "an AI vendor is not a card provider")

	provider := f.newVendor(t, "CARD_PROVIDER", "Provider "+uuid.NewString()[:8])
	var card accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.rawRequest("POST", "vcc-cards", uuid.NewString(), mustJSON(t, cardInput(provider.ID)))), &card))
	require.Equal(t, provider.Name, card.Details["provider"])

	spend := accounting.Input{OrganizationID: org, ParentID: card.ID, Currency: "USD", Amount: "20", OrderID: order,
		Details: map[string]any{"transaction_type": "CONSUME", "merchant": "free text", "provider_transaction_id": uuid.NewString()}}
	require.Equal(t, 400, f.rawJSON(t, "POST", "vcc-transactions", spend), "card spend must name an AI vendor")
	spend.Details["vendor_id"] = ai
	var created accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.rawRequest("POST", "vcc-transactions", uuid.NewString(), mustJSON(t, spend))), &created))
	require.Equal(t, "OpenAI", created.Details["merchant"], "the name comes from the list, not the request")

	// Renaming keeps what records saved; disabling stops new choices but not records that already had it.
	renamed := accounting.VendorInput{Name: provider.Name + " Ltd", Aliases: provider.Aliases, Status: "DISABLED"}
	require.Equal(t, 200, f.request("PATCH", "vendors/"+provider.ID, uuid.NewString(), renamed).Code)
	require.Equal(t, 400, f.rawJSON(t, "POST", "vcc-cards", cardInput(provider.ID)))
	update := accounting.Input{Currency: "USD", Amount: "0", Version: card.Version, Details: map[string]any{"alias": "Renamed card", "provider_id": provider.ID, "provider_card_id": card.Details["provider_card_id"], "last_four": "4242"}}
	require.Equal(t, 200, f.rawJSON(t, "PATCH", "vcc-cards/"+card.ID, update))
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-cards/"+card.ID, "", nil)), &current))
	require.Equal(t, provider.Name, current.Details["provider"])

	require.Equal(t, 409, f.request("POST", "vendors", uuid.NewString(), accounting.VendorInput{Kind: "AI_VENDOR", Name: "openai"}).Code)
	require.Equal(t, 400, f.request("POST", "vendors", uuid.NewString(), accounting.VendorInput{Kind: "AI_VENDOR", Name: "Short", Aliases: []string{"ab"}}).Code)
}

func TestAccountingClassifiesOlderFreeTextByAlias(t *testing.T) {
	f := newAccountingFixture(t)
	card := f.card(t)
	first := f.transaction(t, card, "CONSUME", "10", "")
	second := f.transaction(t, card, "CONSUME", "10", "")
	third := f.transaction(t, card, "CONSUME", "10", "")
	set := func(id, merchant string) {
		_, err := integrationDB.Exec(`UPDATE accounting_vcc_transactions SET details=(details - 'vendor_id') || jsonb_build_object('merchant', $2::text) WHERE id=$1`, id, merchant)
		require.NoError(t, err)
	}
	set(first.ID, "OPENAI *CHATGPT SUBSCR SAN FRANCISCO USA")
	set(second.ID, "ANTHROPIC* CLAUDE SUB SAN FRANCISCO USA")
	marker := "zz" + uuid.NewString()[:8]
	set(third.ID, "Unknown "+marker+" Store")

	// Adding a vendor reclassifies at once; a merchant matching nobody stays for someone to pick.
	vendor := f.newVendor(t, "AI_VENDOR", "Vendor "+marker)
	vendorOf := func(id string) string {
		var v string
		require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(details->>'vendor_id','') FROM accounting_vcc_transactions WHERE id=$1`, id).Scan(&v))
		return v
	}
	require.Equal(t, seededVendor("AI_VENDOR", "OpenAI"), vendorOf(first.ID))
	require.Equal(t, seededVendor("AI_VENDOR", "Anthropic"), vendorOf(second.ID))
	require.Empty(t, vendorOf(third.ID))
	require.Equal(t, 200, f.request("PATCH", "vendors/"+vendor.ID, uuid.NewString(), accounting.VendorInput{Name: vendor.Name, Aliases: []string{marker}}).Code)
	require.Equal(t, vendor.ID, vendorOf(third.ID))
	var merchant string
	require.NoError(t, integrationDB.QueryRow(`SELECT details->>'merchant' FROM accounting_vcc_transactions WHERE id=$1`, first.ID).Scan(&merchant))
	require.Equal(t, "OPENAI *CHATGPT SUBSCR SAN FRANCISCO USA", merchant, "classification keeps the original text")

	// Text matching two vendors stays unclassified rather than guessing.
	ambiguous := f.transaction(t, card, "CONSUME", "10", "")
	set(ambiguous.ID, "OpenAI and Anthropic bundle")
	f.newVendor(t, "AI_VENDOR", "Vendor "+uuid.NewString()[:8])
	require.Empty(t, vendorOf(ambiguous.ID))
}

func mustJSON(t *testing.T, payload any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}
