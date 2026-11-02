//go:build integration

package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Migration 273 drops card balances: queued provider top-ups are voided and statement positions are rebuilt as
// receipts less exchanges (cash) and net card spend (card), leaving legacy top-ups out.
func TestAccountingCardSpendOnlyMigrationRebuildsPositions(t *testing.T) {
	f := newAccountingFixture(t)
	receipt := f.transfer(t, "100")
	f.proof(t, "transfers", receipt)
	f.confirm(t, "transfers", receipt)
	card := f.card(t)
	consume := f.transaction(t, card, "CONSUME", "30", "")
	f.confirm(t, "vcc-transactions", consume)
	f.confirm(t, "vcc-transactions", f.transaction(t, card, "REFUND", "10", consume.ID))

	var org int64
	require.NoError(t, integrationDB.QueryRow("SELECT organization_id FROM accounting_vcc_cards WHERE id=$1", card.ID).Scan(&org))
	// State left behind by the balance model: a queued top-up, a legacy top-up receipt and stale position events.
	fund := uuid.NewString()
	_, err := integrationDB.Exec(`INSERT INTO accounting_external_actions(id,organization_id,object_kind,object_id,card_id,action,amount,currency,occurred_at) VALUES($1,$2,'allocations',$3,$4,'FUND',50,'CNY',NOW())`, fund, org, uuid.NewString(), card.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO accounting_transfers(id,organization_id,status,currency,amount,occurred_at,details,created_by) VALUES($1,$2,'RECEIVED','CNY',80,NOW(),'{"funding_type":"TOPUP"}'::jsonb,1)`, uuid.NewString(), org)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO accounting_position_events(id,organization_id,currency,cash_delta,card_delta,occurred_at,object_kind,object_id,action) VALUES($1,$2,'CNY',-60,60,NOW(),'transfers',$3,'allocate')`, uuid.NewString(), org, receipt.ID)
	require.NoError(t, err)

	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "273_accounting_card_spend_only.sql"))
	require.NoError(t, err)
	tx, err := integrationDB.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(string(body))
	if err != nil {
		_ = tx.Rollback()
	}
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var status, reason string
	require.NoError(t, integrationDB.QueryRow("SELECT status,reason FROM accounting_external_actions WHERE id=$1", fund).Scan(&status, &reason))
	require.Equal(t, "VOID", status)
	require.NotEmpty(t, reason)

	var cash, spent string
	require.NoError(t, integrationDB.QueryRow(`SELECT (COALESCE((SELECT (b.balances->'CNY'->>'cash')::numeric FROM accounting_position_baselines b WHERE b.organization_id=$1),0)+COALESCE(SUM(e.cash_delta),0))::text,
 (COALESCE((SELECT (b.balances->'CNY'->>'card')::numeric FROM accounting_position_baselines b WHERE b.organization_id=$1),0)+COALESCE(SUM(e.card_delta),0))::text
 FROM accounting_position_events e WHERE e.organization_id=$1 AND e.currency='CNY'`, org).Scan(&cash, &spent))
	assertAccountingAmount(t, "100", cash)
	assertAccountingAmount(t, "20", spent)

	// Positions recorded after the migration continue from the rebuilt ones.
	f.confirm(t, "vcc-transactions", f.transaction(t, card, "CONSUME", "5", ""))
	require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(SUM(card_delta),0)::text FROM accounting_position_events WHERE organization_id=$1 AND currency='CNY'`, org).Scan(&spent))
	assertAccountingAmount(t, "25", spent)
}
