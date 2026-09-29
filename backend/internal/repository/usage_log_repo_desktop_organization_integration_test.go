//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopOrganizationUsageIncludesHistoricalKeysAndIsolatesOrganizations(t *testing.T) {
	ctx := context.Background()
	one := newDesktopRepositoryFixture(t, "usageone", 10)
	two := newDesktopRepositoryFixture(t, "otherusage", 10)
	member := one.createMember(t, "usageone", 1)
	second := one.createMember(t, "usagetwo", 2)
	outside := two.createMember(t, "usageoutside", 1)
	require.NotNil(t, member.CurrentAPIKeyID)
	oldKey := *member.CurrentAPIKeyID
	rotated, _, err := one.repo.RotateMemberAPIKey(ctx, one.organization.PublicID, member.PublicID, &service.APIKey{UserID: one.user.ID, Key: "sk-rotated-" + uuid.NewString(), Name: "rotated", Status: service.StatusAPIKeyActive})
	require.NoError(t, err)
	require.NotNil(t, rotated.CurrentAPIKeyID)
	unrelated := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: one.user.ID, Key: "sk-unrelated-" + uuid.NewString(), Name: "unrelated"})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "usage-statistics-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE account_id = $1", account.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", account.ID)
	})
	today := time.Date(2026, 9, 21, 16, 0, 0, 0, time.UTC)
	week := time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC)
	month := time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	insert := func(userID, keyID int64, unit int, at time.Time) {
		builder := integrationEntClient.UsageLog.Create().SetUserID(userID).SetAPIKeyID(keyID).SetAccountID(account.ID).
			SetRequestID(uuid.NewString()).SetModel("model").SetInputTokens(unit).SetOutputTokens(unit * 2).
			SetCacheCreationTokens(unit * 3).SetCacheReadTokens(unit * 4).SetActualCost(float64(unit) / 10).
			SetTotalCost(float64(unit)).SetCreatedAt(at)
		if unit == 6 {
			builder.SetRequestedModel("requested-model")
		}
		_, err := builder.Save(ctx)
		require.NoError(t, err)
	}
	insert(one.user.ID, oldKey, 1, month.Add(-time.Second))
	insert(one.user.ID, oldKey, 2, month)
	insert(one.user.ID, oldKey, 3, week)
	insert(one.user.ID, oldKey, 4, today)
	insert(one.user.ID, *rotated.CurrentAPIKeyID, 5, end.Add(-time.Second))
	insert(one.user.ID, *second.CurrentAPIKeyID, 6, today)
	insert(one.user.ID, *second.CurrentAPIKeyID, 7, end)
	insert(one.user.ID, *second.CurrentAPIKeyID, 8, end.Add(time.Second))
	insert(one.user.ID, unrelated.ID, 9, today)
	insert(two.user.ID, *outside.CurrentAPIKeyID, 10, today)
	_, err = one.repo.DeleteMember(ctx, one.organization.PublicID, member.PublicID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE desktop_members SET status='disabled' WHERE id=$1", second.ID)
	require.NoError(t, err)
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	result, err := repo.GetDesktopOrganizationUsage(ctx, one.organization.ID, today, week, month, end)
	require.NoError(t, err)
	for _, tc := range []struct {
		period service.DesktopOrganizationUsagePeriod
		tokens int64
		cost   float64
	}{
		{result.Today, 150, 1.5}, {result.Week, 180, 1.8}, {result.Month, 200, 2.0}, {result.Total, 210, 2.1},
	} {
		require.Equal(t, tc.tokens, tc.period.TotalTokens)
		require.InDelta(t, tc.cost, tc.period.ActualCost, 0.000001)
	}
	require.Len(t, result.Daily, 30)
	require.Equal(t, "2026-08-24", result.Daily[0].Date)
	require.Equal(t, "2026-09-22", result.Daily[29].Date)
	require.EqualValues(t, 210, result.Last30Days.TotalTokens)
	require.InDelta(t, 2.1, result.Last30Days.ActualCost, 0.000001)
	require.Equal(t, service.DesktopUsageBreakdown{InputTokens: 21, OutputTokens: 42, CacheCreationTokens: 63, CacheReadTokens: 84}, result.Breakdown)
	require.EqualValues(t, 2, result.ObservedMembers)
	require.Len(t, result.Models, 2)
	require.Equal(t, "model", result.Models[0].Model)
	require.EqualValues(t, 5, result.Models[0].Requests)
	require.Equal(t, "requested-model", result.Models[1].Model)
	require.EqualValues(t, 1, result.Models[1].Requests)
	require.Len(t, result.Members, 2)
	require.Equal(t, member.PublicID, result.Members[0].MemberID)
	require.True(t, result.Members[0].Deleted)
	require.EqualValues(t, 5, result.Members[0].Requests)
	require.Equal(t, second.PublicID, result.Members[1].MemberID)
	result, err = repo.GetDesktopOrganizationUsage(ctx, two.organization.ID, today, week, month, end)
	require.NoError(t, err)
	require.EqualValues(t, 100, result.Total.TotalTokens)
	require.InDelta(t, 1.0, result.Total.ActualCost, 0.000001)
	require.EqualValues(t, 1, result.ObservedMembers)
	result, err = repo.GetDesktopOrganizationUsage(ctx, -1, today, week, month, end)
	require.NoError(t, err)
	require.Zero(t, result.Total)
	require.Zero(t, result.Last30Days)
	require.Len(t, result.Daily, 30)
	require.Empty(t, result.Models)
	require.Empty(t, result.Members)
}
