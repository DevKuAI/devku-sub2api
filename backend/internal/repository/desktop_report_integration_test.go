//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopReportPersistenceLeaseSourcesAndBilling(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "reports", 1, true)
	other := newDesktopRepositoryFixture(t, "reportsother", 1, true)
	member := fixture.createMember(t, "reports", 1)
	outside := other.createMember(t, "reportsother", 2)
	enabled := true
	model := "test-model"
	organization, _, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{ConversationSummaryEnabled: &enabled, AnalysisModel: &model})
	require.NoError(t, err)
	require.NotNil(t, organization.SummaryEnabledAt)
	repo := NewDesktopReportRepository(integrationDB, integrationEntClient)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_daily_report_executions WHERE organization_id=$1`, organization.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_daily_report_tasks WHERE organization_id=$1`, organization.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_conversation_records WHERE organization_id IN ($1,$2)`, organization.ID, other.organization.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE desktop_analysis_organization_id=$1`, organization.ID)
	})
	date := time.Now().In(time.FixedZone("Shanghai", 8*3600)).Format("2006-01-02")
	start, end, err := reportBounds(date)
	require.NoError(t, err)
	conversation := NewDesktopConversationRepository(integrationEntClient)
	insert := func(org, person int64, at time.Time) string {
		id := uuid.NewString()
		_, err := conversation.Create(ctx, org, person, &service.DesktopConversationInput{RecordID: id, InstallationID: uuid.NewString(), Client: "workbuddy", SessionID: "report-session", StartedAt: at.Add(-time.Minute), StoppedAt: at, Prompts: []service.DesktopTextSegment{{Text: "source evidence"}}, CaptureStatus: "response_missing"})
		require.NoError(t, err)
		return id
	}
	insert(organization.ID, member.ID, start.Add(-time.Second))
	first := insert(organization.ID, member.ID, start)
	second := insert(organization.ID, member.ID, end.Add(-time.Second))
	insert(organization.ID, member.ID, end)
	insert(other.organization.ID, outside.ID, start)
	day, err := repo.Day(ctx, organization.ID, date)
	require.NoError(t, err)
	require.Len(t, day, 1)
	require.Equal(t, 2, day[0].RecordCount)
	require.Equal(t, []string{first, second}, day[0].SourceIDs)
	snapshots, err := repo.Snapshot(ctx, organization.ID, date)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Len(t, snapshots[0].Sources, 2)
	require.Equal(t, "source evidence", snapshots[0].Sources[0].Prompts[0].Text)
	task, err := repo.Enqueue(ctx, organization.ID, date, "generate")
	require.NoError(t, err)
	var wg sync.WaitGroup
	claimed := make(chan *service.DesktopReportTask, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, e := repo.Claim(ctx)
			if e != nil {
				failures <- e
			}
			if value != nil {
				claimed <- value
			}
		}()
	}
	wg.Wait()
	close(claimed)
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	require.Len(t, claimed, 1)
	lease := <-claimed
	require.Equal(t, task.ID, lease.ID)
	execution := service.DesktopReportExecution{Kind: "member", MemberID: member.PublicID, Model: model, Revision: lease.Revision, Chunk: 1, Attempt: 1, RequestID: uuid.NewString(), Status: "running"}
	require.NoError(t, repo.BeginExecution(ctx, lease, &execution))
	finished := time.Now()
	execution.Status = "completed"
	execution.FinishedAt = &finished
	require.NoError(t, repo.FinishExecution(ctx, &execution))
	history, err := repo.Executions(ctx, organization.ID, date)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, model, history[0].Model)
	otherHistory, err := repo.Executions(ctx, other.organization.ID, date)
	require.NoError(t, err)
	require.Empty(t, otherHistory)
	_, err = repo.Enqueue(ctx, organization.ID, date, "regenerate")
	require.ErrorIs(t, err, service.ErrDesktopRotationConflict)
	lease.Payload = service.DesktopReportPayload{Model: model, GroupID: organization.GroupID, UserID: organization.GatewayUserID, Snapshots: snapshots, Summary: service.DesktopGeneratedReport{Status: "completed", Content: "old summary", Model: model}}
	lease.Payload.Snapshots[0].Member.Content = "old report"
	lease.Payload.Snapshots[0].Member.Status = "completed"
	require.NoError(t, repo.Save(ctx, lease, time.Now(), false))
	_, err = integrationDB.ExecContext(ctx, `UPDATE desktop_daily_report_tasks SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	resumed, err := repo.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, resumed)
	require.NotEqual(t, lease.LeaseToken, resumed.LeaseToken)
	require.Len(t, resumed.Payload.Snapshots[0].Sources, 2)
	require.ErrorIs(t, repo.Save(ctx, lease, time.Now(), true), service.ErrDesktopReportLeaseLost)
	resumed.Status = "completed"
	require.NoError(t, repo.Save(ctx, resumed, time.Now(), true))
	regenerated, err := repo.Enqueue(ctx, organization.ID, date, "regenerate")
	require.NoError(t, err)
	require.Equal(t, 2, regenerated.Revision)
	require.Empty(t, regenerated.Payload.Model)
	require.Equal(t, "old report", regenerated.Payload.Snapshots[0].Member.Content)
	require.Equal(t, "old summary", regenerated.Payload.Summary.Content)
	require.Empty(t, regenerated.Payload.Snapshots[0].Sources)
	history, err = repo.Executions(ctx, organization.ID, date)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, model, history[0].Model)
	stale := execution
	stale.RequestID = uuid.NewString()
	require.ErrorIs(t, repo.BeginExecution(ctx, lease, &stale), service.ErrDesktopReportLeaseLost)
	retry, err := repo.Enqueue(ctx, organization.ID, date, "retry")
	require.NoError(t, err)
	require.Equal(t, "pending", retry.Status)

	key, err := repo.AnalysisKey(ctx, organization)
	require.NoError(t, err)
	require.Equal(t, "desktop_analysis", key.ManagedBy)
	require.Equal(t, organization.GatewayUserID, key.UserID)
	count, err := fixture.apiKeys.CountByUserID(ctx, fixture.user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	listed, _, err := fixture.apiKeys.ListByUserID(ctx, fixture.user.ID, pagination.DefaultPagination(), service.APIKeyListFilters{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotEqual(t, key.ID, listed[0].ID)
	loaded, err := fixture.apiKeys.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, "desktop_analysis", loaded.ManagedBy)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "report-billing-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	insertUsage := func(keyID int64, tokens int, cost float64) {
		_, err := integrationEntClient.UsageLog.Create().SetUserID(fixture.user.ID).SetAPIKeyID(keyID).SetAccountID(account.ID).SetRequestID(uuid.NewString()).SetModel(model).SetInputTokens(tokens).SetActualCost(cost).SetCreatedAt(time.Now()).Save(ctx)
		require.NoError(t, err)
	}
	insertUsage(key.ID, 100, 0.25)
	insertUsage(*member.CurrentAPIKeyID, 50, 0.1)
	selection, err := (service.DesktopAnalyticsRangeInput{}).Resolve(time.Now().Add(time.Second))
	require.NoError(t, err)
	usage, err := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB).GetDesktopOrganizationUsage(ctx, organization.ID, service.DesktopUsageWindows{Today: start, Week: start, Month: start, AsOf: time.Now().Add(time.Second), Selected: selection})
	require.NoError(t, err)
	require.EqualValues(t, 150, usage.Total.TotalTokens)
	require.InDelta(t, 0.35, usage.Total.ActualCost, 0.000001)
	require.EqualValues(t, 100, usage.Analysis.TotalTokens)
	require.Len(t, usage.Members, 1)
	require.EqualValues(t, 50, usage.Members[0].TotalTokens)
}
