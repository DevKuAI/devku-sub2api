//go:build integration

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// cardRow is one card line of a statement template for the fixture's test account.
func (f *accountingFixture) cardRow(t *testing.T, at, kind, amount, cardRef, related string) (string, string) {
	t.Helper()
	account := "test-" + f.desktop.organization.PublicID
	if !f.statementAccount {
		f.statementAccount = true
		cleanChannelAccounts(t, account)
		t.Cleanup(func() { cleanChannelAccounts(t, account) })
	}
	external := uuid.NewString()
	return external, strings.Join([]string{"TESTPAY", account, "CARD", external, at, kind, amount, "USD", "", "", cardRef, "4242", "", "OPENAI *CHATGPT SUBSCR", related, "", "", "statement line"}, ",")
}

func (f *accountingFixture) importRows(t *testing.T, rows ...string) accounting.ChannelImportResult {
	t.Helper()
	var result accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", templateUpload("card.csv", rows...))), &result))
	return result
}

func lineBooking(t *testing.T, external string) (booking, note, transaction string) {
	t.Helper()
	require.NoError(t, integrationDB.QueryRow("SELECT booking,booking_note,COALESCE(transaction_id::text,'') FROM accounting_channel_lines WHERE external_id=$1 AND status='ACTIVE'", external).Scan(&booking, &note, &transaction))
	return booking, note, transaction
}

func (f *accountingFixture) usdCard(t *testing.T, order, cardRef string) *accounting.Record {
	t.Helper()
	last := cardRef[len(cardRef)-4:]
	if strings.Trim(last, "0123456789") != "" {
		last = "4242"
	}
	details := map[string]any{"alias": "Card " + last, "provider": "SUNRATE", "last_four": last, "provider_card_id": cardRef}
	return f.createIn(t, order, "vcc-cards", "USD", "0", details, "")
}

// legacyCard registers a card the way cards were kept before card IDs: no platform card ID.
func (f *accountingFixture) legacyCard(t *testing.T, order string) string {
	t.Helper()
	id := uuid.NewString()
	_, err := integrationDB.Exec(`INSERT INTO accounting_vcc_cards(id,organization_id,status,currency,amount,occurred_at,details,created_by) VALUES($1,$2,'ACTIVE','USD',0,NOW(),'{"alias":"Legacy card","provider":"SUNRATE","last_four":"0956"}'::jsonb,$3)`, id, f.desktop.organization.ID, f.desktop.user.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO accounting_order_items(object_kind,object_id,order_id,organization_id,created_by) VALUES('vcc-cards',$1,$2,$3,$4)`, id, order, f.desktop.organization.ID, f.desktop.user.ID)
	require.NoError(t, err)
	return id
}

// handEntered stores card spend the way it was recorded before statements were imported.
func (f *accountingFixture) handEntered(t *testing.T, card, amount, at, reference string) string {
	t.Helper()
	id := uuid.NewString()
	details, _ := json.Marshal(map[string]any{"transaction_type": "CONSUME", "merchant": "OpenAI", "provider_transaction_id": reference})
	_, err := integrationDB.Exec(`INSERT INTO accounting_vcc_transactions(id,organization_id,parent_id,status,currency,amount,occurred_at,details,created_by) VALUES($1,$2,$3,'CONFIRMED','USD',$4,$5,$6,$7)`, id, f.desktop.organization.ID, card, amount, at, details, f.desktop.user.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO accounting_order_items(object_kind,object_id,order_id,organization_id,created_by) SELECT 'vcc-transactions',$1,order_id,organization_id,created_by FROM accounting_order_items WHERE object_kind='vcc-cards' AND object_id=$2`, id, card)
	require.NoError(t, err)
	return id
}

// cardPositions sums the card side of the organization's recorded positions in USD.
func (f *accountingFixture) cardPositions(t *testing.T) string {
	t.Helper()
	var total string
	require.NoError(t, integrationDB.QueryRow(`SELECT (COALESCE((SELECT (balances->'USD'->>'card')::numeric FROM accounting_position_baselines WHERE organization_id=$1),0)
 +COALESCE((SELECT SUM(card_delta) FROM accounting_position_events WHERE organization_id=$1 AND currency='USD'),0))::text`, f.desktop.organization.ID).Scan(&total))
	return total
}

func TestAccountingSunrateStatementBooksCardSpend(t *testing.T) {
	f := newChannelFixture(t, "72778936")
	order := f.order(t)
	cards := map[string]*accounting.Record{}
	for _, ref := range []string{"2094962280740556801", "2095346501199990785", "2113096199301369858"} {
		cards[ref] = f.usdCard(t, order, ref)
	}
	var result accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &result))
	// Spend on the three registered cards is booked; 0956 has no card with its card ID yet.
	require.Equal(t, map[string]int{"BOOKED": 5, "PENDING_CARD": 1}, result.Bookings)
	require.Equal(t, 1, result.Import.PendingRows)
	require.Equal(t, 1, result.Accounts[0].PendingCards)
	require.Equal(t, "400", f.cardSpent(t, cards["2094962280740556801"].ID))
	require.Equal(t, "400", f.cardSpent(t, cards["2095346501199990785"].ID))
	require.Equal(t, "200", f.cardSpent(t, cards["2113096199301369858"].ID))
	var vendor, merchant string
	require.NoError(t, integrationDB.QueryRow("SELECT details->>'vendor_id',details->>'merchant' FROM accounting_vcc_transactions WHERE parent_id=$1 AND status='CONFIRMED' LIMIT 1", cards["2095346501199990785"].ID).Scan(&vendor, &merchant))
	require.Equal(t, seededVendor("AI_VENDOR", "OpenAI"), vendor, "the statement merchant names the vendor")
	require.Equal(t, "OpenAI", merchant)
	totals := f.readOrder(t, order).Totals
	require.Equal(t, []accounting.OrderTotal{{Currency: "USD", Received: "0", Invoiced: "0", Spent: "1000", AICost: "0"}}, totals)
	assertAccountingAmount(t, "1000", f.cardPositions(t))

	// Importing the same statement again books nothing twice.
	var again accounting.ChannelImportResult
	require.NoError(t, json.Unmarshal(accountingData(t, f.upload(t, "channel-imports", uuid.NewString(), "", sunrateUpload(t))), &again))
	require.Equal(t, 36, again.Import.DuplicateRows)
	require.Empty(t, again.Bookings)
	require.Equal(t, "400", f.cardSpent(t, cards["2094962280740556801"].ID))

	// 0956 belongs to a card registered before card IDs were kept; binding it by hand books its line and keeps the ID.
	var channelCard string
	require.NoError(t, integrationDB.QueryRow("SELECT id::text FROM accounting_channel_cards WHERE card_ref='2108375553744965633'").Scan(&channelCard))
	mismatch := f.request("PATCH", "channel-cards/"+channelCard, uuid.NewString(), map[string]string{"vcc_card_id": cards["2113096199301369858"].ID})
	require.Equal(t, 409, mismatch.Code)
	require.Contains(t, mismatch.Body.String(), "card_id_mismatch")
	legacy := f.legacyCard(t, order)
	bound := accountingObject(t, f.request("PATCH", "channel-cards/"+channelCard, uuid.NewString(), map[string]string{"vcc_card_id": legacy}))
	require.Equal(t, legacy, bound["vcc_card_id"])
	require.Equal(t, "MANUAL", bound["bind_method"])
	require.EqualValues(t, 0, bound["pending"])
	require.Equal(t, "200", f.cardSpent(t, legacy))
	var cardID string
	require.NoError(t, integrationDB.QueryRow("SELECT details->>'provider_card_id' FROM accounting_vcc_cards WHERE id=$1", legacy).Scan(&cardID))
	require.Equal(t, "2108375553744965633", cardID)

	// Revoking takes the statement's spend back out of the organization books.
	var imports []accounting.ChannelImport
	rows, err := integrationDB.Query("SELECT id::text FROM accounting_channel_imports WHERE status='APPLIED' AND account_ids ? (SELECT id::text FROM accounting_channel_accounts WHERE account_ref='72778936') ORDER BY created_at DESC,id DESC")
	require.NoError(t, err)
	for rows.Next() {
		var im accounting.ChannelImport
		require.NoError(t, rows.Scan(&im.ID))
		imports = append(imports, im)
	}
	require.NoError(t, rows.Close())
	require.Len(t, imports, 2)
	for _, im := range imports {
		accountingData(t, f.request("POST", "channel-imports/"+im.ID+"/revoke", uuid.NewString(), map[string]string{"reason": "wrong period"}))
	}
	for _, card := range []string{cards["2094962280740556801"].ID, cards["2095346501199990785"].ID, legacy} {
		require.Equal(t, "0", f.cardSpent(t, card))
	}
	assertAccountingAmount(t, "0", f.cardPositions(t))
}

func TestAccountingStatementSpendWaitsForAnOrderAndMoves(t *testing.T) {
	f := newAccountingFixture(t)
	first := f.newOrder(t, "Closed seats", nil)
	ref := testCardID()
	card := f.usdCard(t, first.ID, ref)
	accountingData(t, f.request("POST", "orders/"+first.ID+"/close", uuid.NewString(), map[string]string{"reason": "done"}))

	// The card's order is closed and the organization has no open order: the spend waits.
	spendLine, row := f.cardRow(t, "2026-10-10 10:00:00+08:00", "CONSUME", "-50", ref, "AUTH-"+ref)
	f.importRows(t, row)
	booking, _, _ := lineBooking(t, spendLine)
	require.Equal(t, "PENDING_ORDER", booking)
	todo := accountingObject(t, f.request("GET", "workbench?organization_id="+f.desktop.organization.PublicID+"&reason=channel_pending_order", "", nil))
	require.Len(t, todo["items"].([]any), 1)

	// A new order picks it up; positions count it once, at the statement date.
	second := f.newOrder(t, "Open seats", nil)
	booking, _, spend := lineBooking(t, spendLine)
	require.Equal(t, "BOOKED", booking)
	require.Len(t, f.readOrder(t, second.ID).Items["vcc-transactions"], 1)
	assertAccountingAmount(t, "50", f.cardPositions(t))
	refundLine, row := f.cardRow(t, "2026-10-12 10:00:00+08:00", "REFUND", "10", ref, "AUTH-"+ref)
	f.importRows(t, row)
	booking, _, refund := lineBooking(t, refundLine)
	require.Equal(t, "BOOKED", booking)
	require.Equal(t, "40", f.cardSpent(t, card.ID))

	// Spend moves to another open order together with its refund.
	third := f.newOrder(t, "Other seats", nil)
	moved := f.request("POST", "orders/"+third.ID+"/links", uuid.NewString(), map[string]string{"object_kind": "vcc-transactions", "object_id": spend})
	accountingData(t, moved)
	ids := []string{}
	for _, item := range f.readOrder(t, third.ID).Items["vcc-transactions"] {
		ids = append(ids, item.ID)
	}
	require.ElementsMatch(t, []string{spend, refund}, ids)
	require.Empty(t, f.readOrder(t, second.ID).Items["vcc-transactions"])

	// Binding the statement card to another card moves its spend there.
	var channelCard string
	require.NoError(t, integrationDB.QueryRow("SELECT id::text FROM accounting_channel_cards WHERE card_ref=$1", ref).Scan(&channelCard))
	other := f.usdCard(t, third.ID, testCardID())
	require.Equal(t, 409, f.request("PATCH", "channel-cards/"+channelCard, uuid.NewString(), map[string]string{"vcc_card_id": other.ID}).Code)
	legacy := f.legacyCard(t, third.ID)
	accountingData(t, f.request("PATCH", "channel-cards/"+channelCard, uuid.NewString(), map[string]string{"vcc_card_id": legacy}))
	require.Equal(t, "0", f.cardSpent(t, card.ID))
	require.Equal(t, "40", f.cardSpent(t, legacy))
	var oldID, newID string
	require.NoError(t, integrationDB.QueryRow("SELECT COALESCE((SELECT details->>'provider_card_id' FROM accounting_vcc_cards WHERE id=$1),''),COALESCE((SELECT details->>'provider_card_id' FROM accounting_vcc_cards WHERE id=$2),'')", card.ID, legacy).Scan(&oldID, &newID))
	require.Equal(t, []string{"", ref}, []string{oldID, newID}, "the card ID moves with the binding")
	assertAccountingAmount(t, "40", f.cardPositions(t))
	var status string
	require.NoError(t, integrationDB.QueryRow("SELECT status FROM accounting_vcc_transactions WHERE id=$1", spend).Scan(&status))
	require.Equal(t, "VOID", status)
}

func TestAccountingStatementLinksHandEnteredSpend(t *testing.T) {
	f := newAccountingFixture(t)
	order := f.order(t)
	ref := testCardID()
	card := f.usdCard(t, order, ref)
	byReference := f.handEntered(t, card.ID, "30", "2026-10-10T02:00:00Z", "AUTH-1-"+ref)
	byAmount := f.handEntered(t, card.ID, "20", "2026-10-11T02:00:00Z", "")
	unmatched := f.handEntered(t, card.ID, "99", "2026-10-11T05:00:00Z", "")

	first, row1 := f.cardRow(t, "2026-10-10 10:00:00+08:00", "CONSUME", "-30", ref, "AUTH-1-"+ref)
	second, row2 := f.cardRow(t, "2026-10-11 18:00:00+08:00", "CONSUME", "-20", ref, "")
	third, row3 := f.cardRow(t, "2026-10-12 10:00:00+08:00", "CONSUME", "-15", ref, "")
	result := f.importRows(t, row1, row2, row3)
	require.Equal(t, map[string]int{"LINKED": 2, "BOOKED": 1}, result.Bookings)
	booking, note, linked := lineBooking(t, first)
	require.Equal(t, []string{"LINKED", "", byReference}, []string{booking, note, linked})
	booking, note, linked = lineBooking(t, second)
	require.Equal(t, []string{"LINKED", "amount_date", byAmount}, []string{booking, note, linked})
	booking, _, _ = lineBooking(t, third)
	require.Equal(t, "BOOKED", booking)
	// Nothing was counted twice: hand-entered 149 plus the one new purchase.
	require.Equal(t, "164", f.cardSpent(t, card.ID))
	todo := accountingObject(t, f.request("GET", "workbench?organization_id="+f.desktop.organization.PublicID+"&reason=manual_unmatched", "", nil))
	items := todo["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, unmatched, items[0].(map[string]any)["id"])

	// Hand-entered spend stays when its statement is revoked; only the link goes.
	accountingData(t, f.request("POST", "channel-imports/"+result.Import.ID+"/revoke", uuid.NewString(), map[string]string{"reason": "re-export"}))
	require.Equal(t, "149", f.cardSpent(t, card.ID))
	var link string
	require.NoError(t, integrationDB.QueryRow("SELECT COALESCE(details->>'channel_line_id','') FROM accounting_vcc_transactions WHERE id=$1", byAmount).Scan(&link))
	require.Empty(t, link)
}
