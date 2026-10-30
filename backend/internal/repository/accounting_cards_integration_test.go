//go:build integration

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *accountingFixture) updateCard(t *testing.T, card *accounting.Record, details map[string]any) int {
	t.Helper()
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-cards/"+card.ID, "", nil)), &current))
	merged := map[string]any{}
	for k, v := range current.Details {
		if k == "alias" || k == "provider" || k == "provider_card_id" || k == "last_four" || k == "remark" || k == "expiry" {
			merged[k] = v
		}
	}
	for k, v := range details {
		merged[k] = v
	}
	in := accounting.Input{Currency: current.Currency, Amount: "0", Version: current.Version, Details: merged}
	return f.request("PATCH", "vcc-cards/"+card.ID, uuid.NewString(), in).Code
}

// Cards registered with a full number carry no card ID yet. They may add it once, correcting the last four
// digits at the same time, even after spending; afterwards the identity is fixed.
func TestAccountingLegacyCardAddsCardIDOnce(t *testing.T) {
	f := newAccountingFixture(t)
	card := f.card(t)
	f.confirm(t, "vcc-transactions", f.transaction(t, card, "CONSUME", "10", ""))
	_, err := integrationDB.Exec(`UPDATE accounting_vcc_cards SET details=(details - 'provider_card_id') || '{"last_four":"0785"}'::jsonb WHERE id=$1`, card.ID)
	require.NoError(t, err)

	require.Equal(t, 400, f.updateCard(t, card, map[string]any{"alias": "Renamed"}), "a legacy card must add its card ID before other edits")
	require.Equal(t, 200, f.updateCard(t, card, map[string]any{"provider_card_id": "2095346501199990785", "last_four": "3926"}))
	var current accounting.Record
	require.NoError(t, json.Unmarshal(accountingData(t, f.request("GET", "vcc-cards/"+card.ID, "", nil)), &current))
	require.Equal(t, "2095346501199990785", current.Details["provider_card_id"])
	require.Equal(t, "3926", current.Details["last_four"])

	require.Equal(t, 409, f.updateCard(t, card, map[string]any{"last_four": "1111"}))
	require.Equal(t, 409, f.updateCard(t, card, map[string]any{"provider_card_id": "another-id"}))
	require.Equal(t, 200, f.updateCard(t, card, map[string]any{"alias": "Renamed"}))

	// One platform card ID belongs to one card across all organizations.
	other := newAccountingFixture(t)
	duplicate := accounting.Input{OrganizationID: other.desktop.organization.PublicID, Currency: "CNY", OrderID: other.order(t),
		Details: map[string]any{"alias": "Copy", "provider": "Provider", "provider_card_id": "2095346501199990785", "last_four": "3926"}}
	res := other.request("POST", "vcc-cards", uuid.NewString(), duplicate)
	require.Equal(t, 409, res.Code)
	require.Contains(t, res.Body.String(), "card_registered")
}

func TestAccountingEnginePurgesStoredCardSecrets(t *testing.T) {
	f := newAccountingFixture(t)
	card := f.card(t)
	cfg := &config.Config{}
	cfg.Totp.EncryptionKey = strings.Repeat("ab", 32)
	cfg.Totp.EncryptionKeyConfigured = true
	encryptor, err := NewAESEncryptor(cfg)
	require.NoError(t, err)
	cipher, err := encryptor.Encrypt(`{"card_number":"4111111111111111","cvv":"123","expiry":"2030-12","provider_token":"tok"}`)
	require.NoError(t, err)
	unreadable := f.card(t)
	_, err = integrationDB.Exec(`UPDATE accounting_vcc_cards SET secret_ciphertext=$2,card_fingerprint=$3,details=details - 'expiry' WHERE id=$1`, card.ID, cipher, "fingerprint-"+card.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE accounting_vcc_cards SET secret_ciphertext='not-decryptable',card_fingerprint=$2 WHERE id=$1`, unreadable.ID, "fingerprint-"+unreadable.ID)
	require.NoError(t, err)

	accounting.NewEngine(integrationDB, nil, nil, encryptor, cfg)

	for _, id := range []string{card.ID, unreadable.ID} {
		var stored string
		var fingerprint *string
		require.NoError(t, integrationDB.QueryRow("SELECT secret_ciphertext,card_fingerprint FROM accounting_vcc_cards WHERE id=$1", id).Scan(&stored, &fingerprint))
		require.Empty(t, stored)
		require.Nil(t, fingerprint)
	}
	var details string
	require.NoError(t, integrationDB.QueryRow("SELECT details::text FROM accounting_vcc_cards WHERE id=$1", card.ID).Scan(&details))
	require.Contains(t, details, `"expiry": "2030-12"`)
	require.NotContains(t, details, "4111111111111111")
}
