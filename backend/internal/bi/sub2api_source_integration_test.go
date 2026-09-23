//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSub2apiAdapterPreservesHistoricalOwnershipWithoutInventingIdentity(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	ctx := context.Background()
	var orgID, userID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT d.id,d.gateway_user_id FROM desktop_organizations d JOIN bi_organizations o ON o.desktop_organization_id=d.id WHERE o.id=$1`, c.OrganizationID).Scan(&orgID, &userID))
	var accountID, memberID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO accounts(name,platform,type,credentials) VALUES($1,'openai','apikey','{}') RETURNING id`, randomToken("account_")).Scan(&accountID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO desktop_members(public_id,organization_id,name,name_normalized,phone,deleted_at) VALUES($1,$2,'Sensitive employee','Sensitive employee','+8613800000000',NOW()) RETURNING id`, "mem_"+randomToken("")[:24], orgID).Scan(&memberID))
	keys := []int64{}
	secrets := []string{}
	for _, name := range []string{"retired", "current", "unrelated-carrier-key"} {
		secret := "sk_private_" + randomToken("")[:32]
		var id int64
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,$3) RETURNING id`, userID, secret, name).Scan(&id))
		keys = append(keys, id)
		secrets = append(secrets, secret)
		if name == "unrelated-carrier-key" {
			continue
		}
		var retired any
		if name == "retired" {
			retired = s.now()
		}
		_, err := integrationDB.Exec(`INSERT INTO desktop_member_api_keys(member_id,api_key_id,retired_at) VALUES($1,$2,$3)`, memberID, id, retired)
		require.NoError(t, err)
	}
	var reservedID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT nextval(pg_get_serial_sequence('usage_logs','id'))`).Scan(&reservedID))
	when := s.now().UTC().Add(-time.Hour)
	for _, key := range keys {
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,created_at,billing_mode)
			VALUES($1,$2,$3,$4,'upstream-model','requested-model',100,20,40,10,$5,'token')`, userID, key, accountID, randomToken("req_"), when)
		require.NoError(t, err)
	}
	batch, err := s.PrepareSub2apiImport(ctx, c, when.Add(-time.Hour), 500)
	require.NoError(t, err)
	require.Len(t, batch.Records, 2)
	require.Nil(t, batch.CompleteThrough)
	require.False(t, batch.InitialBackfillComplete)
	raw, err := json.Marshal(batch)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "Sensitive employee")
	require.NotContains(t, string(raw), "+8613800000000")
	for _, secret := range secrets {
		require.NotContains(t, string(raw), secret)
	}
	for _, record := range batch.Records {
		var usage UsageRecord
		require.NoError(t, json.Unmarshal(record.Payload, &usage))
		require.Equal(t, "unknown", usage.ActorType)
		require.Nil(t, usage.MemberID)
		require.Nil(t, usage.TeamID)
		require.Equal(t, "unknown", usage.Outcome)
		require.Equal(t, "requested-model", usage.RequestedModel)
		standard, err := NormalizeTokens(usage.Tokens)
		require.NoError(t, err)
		require.Equal(t, "150", *standard.Input)
	}
	job := applyImport(t, s, c, raw)
	require.Equal(t, "applied", job.Status, job.Errors)
	empty, err := s.PrepareSub2apiImport(ctx, c, when.Add(-time.Hour), 500)
	require.NoError(t, err)
	require.Empty(t, empty.Records)
	// A lower sequence ID can commit later. The adapter must not skip it.
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(id,user_id,api_key_id,account_id,request_id,model,created_at,billing_mode,image_count,image_size)
		VALUES($1,$2,$3,$4,$5,'image-model',$6,'image',1,'1K')`, reservedID, userID, keys[1], accountID, randomToken("req_"), when)
	require.NoError(t, err)
	late, err := s.PrepareSub2apiImport(ctx, c, when.Add(-time.Hour), 500)
	require.NoError(t, err)
	require.Len(t, late.Records, 1)
	var usage UsageRecord
	require.NoError(t, json.Unmarshal(late.Records[0].Payload, &usage))
	require.Equal(t, "unavailable", usage.Tokens.Encoding)
	require.Nil(t, usage.Tokens.Input)
	require.Equal(t, "unknown", usage.RequestedModel)
	raw, err = json.Marshal(late)
	require.NoError(t, err)
	job = applyImport(t, s, c, raw)
	require.Equal(t, "applied", job.Status)
	empty, err = s.PrepareSub2apiImport(ctx, c, when.Add(-time.Hour), 500)
	require.NoError(t, err)
	require.Empty(t, empty.Records)
}
