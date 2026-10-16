//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

func TestDesktopNativeTokensPaginationHistoricalKeysAndUnknown(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "nativeusage", 4, true)
	member := f.createMember(t, "nativeusage", 1)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "native-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM api_keys WHERE desktop_analysis_organization_id=$1`, f.organization.ID)
	})
	at := time.Now()
	selection, e := (service.DesktopAnalyticsRangeInput{Days: 7, Timezone: "Asia/Shanghai"}).Resolve(at.Add(time.Minute))
	require.NoError(t, e)
	insert := func(key int64, model string, input, output, cache int, mode string) {
		_, e := integrationEntClient.UsageLog.Create().SetUserID(f.user.ID).SetAPIKeyID(key).SetAccountID(account.ID).SetRequestID(uuid.NewString()).SetModel(model).SetInputTokens(input).SetOutputTokens(output).SetCacheReadTokens(cache).SetBillingMode(mode).SetCreatedAt(at).Save(ctx)
		require.NoError(t, e)
	}
	insert(*member.CurrentAPIKeyID, "big", 2147483647, 2147483647, 3, "")
	for i := 0; i < 120; i++ {
		insert(*member.CurrentAPIKeyID, fmt.Sprintf("model-%03d", i), 1, 0, 0, "token")
	}
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	usage, e := repo.NativeUsage(ctx, f.organization.ID, selection)
	require.NoError(t, e)
	require.Equal(t, "4294967417", *usage.Totals.Total)
	require.Equal(t, "2147483770", *usage.Totals.Input)
	require.EqualValues(t, 1, usage.ObservedMembers)
	require.Equal(t, "Asia/Shanghai", usage.Range.Timezone)
	first, total, e := repo.NativeModels(ctx, f.organization.ID, selection, "", 1, 100)
	require.NoError(t, e)
	require.EqualValues(t, 121, total)
	require.Len(t, first, 100)
	second, total, e := repo.NativeModels(ctx, f.organization.ID, selection, "", 2, 100)
	require.NoError(t, e)
	require.Len(t, second, 21)
	require.EqualValues(t, 121, total)
	_, e = f.repo.DeleteMember(ctx, f.organization.PublicID, member.PublicID)
	require.NoError(t, e)
	items, total, e := repo.NativeMembers(ctx, f.organization.ID, selection, "", "", 1, 20, true)
	require.NoError(t, e)
	require.EqualValues(t, 1, total)
	require.True(t, items[0].Deleted)
	require.Equal(t, *usage.Totals.Total, *items[0].Tokens)
	hidden, _, e := repo.NativeMembers(ctx, f.organization.ID, selection, "", "", 1, 20, false)
	require.NoError(t, e)
	require.Nil(t, hidden[0].Tokens)
	require.Nil(t, hidden[0].Requests)
	other := newDesktopRepositoryFixture(t, "nativeunknown", 1, true)
	unknown := other.createMember(t, "nativeunknown", 1)
	_, e = integrationEntClient.UsageLog.Create().SetUserID(other.user.ID).SetAPIKeyID(*unknown.CurrentAPIKeyID).SetAccountID(account.ID).SetRequestID(uuid.NewString()).SetModel("unmeasured").SetCreatedAt(at).Save(ctx)
	require.NoError(t, e)
	unknownUsage, e := repo.NativeUsage(ctx, other.organization.ID, selection)
	require.NoError(t, e)
	require.Nil(t, unknownUsage.Totals.Total)
	require.Equal(t, "unavailable", unknownUsage.Totals.Status)
}

func TestDesktopNativeUsageSnapshotReconcilesDuringConcurrentBackfill(t *testing.T) {
	f := newDesktopRepositoryFixture(t, "snapshot", 2, true)
	member := f.createMember(t, "snapshot", 1)
	ctx := context.Background()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "snapshot-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	at := time.Now().Add(-time.Minute)
	selected, err := (service.DesktopAnalyticsRangeInput{Days: 7, Timezone: "Asia/Shanghai"}).Resolve(time.Now())
	require.NoError(t, err)
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 1000; i++ {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,billing_mode,created_at) VALUES($1,$2,$3,$4,'snapshot',1,1,'token',$5)`, f.user.ID, *member.CurrentAPIKeyID, account.ID, fmt.Sprintf("snapshot-%s-%d", account.Name, i), at)
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	t.Cleanup(func() { close(stop); require.NoError(t, <-done) })
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	for i := 0; i < 12; i++ {
		usage, err := repo.NativeUsage(ctx, f.organization.ID, selected)
		require.NoError(t, err)
		var sum int64
		for _, day := range usage.Daily {
			if day.Tokens != nil {
				value, err := strconv.ParseInt(*day.Tokens, 10, 64)
				require.NoError(t, err)
				sum += value
			}
		}
		total, err := strconv.ParseInt(*usage.Totals.Total, 10, 64)
		require.NoError(t, err)
		require.Equal(t, total, sum, "one response must use one snapshot even for late committed rows")
	}
}
