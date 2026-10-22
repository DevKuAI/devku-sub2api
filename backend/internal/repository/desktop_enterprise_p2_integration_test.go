//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/desktopposition"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func p2ReportRepository(t *testing.T, f *desktopRepositoryFixture) *desktopReportRepository {
	t.Helper()
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM desktop_daily_report_executions WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.Exec(`DELETE FROM desktop_period_report_executions WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.Exec(`DELETE FROM desktop_daily_report_tasks WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.Exec(`DELETE FROM desktop_period_report_tasks WHERE organization_id=$1`, f.organization.ID)
	})
	return NewDesktopReportRepository(integrationDB, integrationEntClient).(*desktopReportRepository)
}

func TestDesktopP2AtomicOutputAndExecutionRollback(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "p2atomic", 2, true)
	repo := p2ReportRepository(t, f)
	task, err := repo.Enqueue(ctx, f.organization.ID, "2026-10-01", "generate")
	require.NoError(t, err)
	task.LeaseToken = uuid.NewString()
	task.Status = "running"
	task.Checkpoint = "summary"
	_, err = integrationDB.Exec(`UPDATE desktop_daily_report_tasks SET lease_token=$2,lease_until=NOW()+INTERVAL '3 minutes',status='running' WHERE id=$1`, task.ID, task.LeaseToken)
	require.NoError(t, err)
	execution := service.DesktopReportExecution{Kind: "summary", Revision: 1, Model: "model", Chunk: 1, Attempt: 1, RequestID: uuid.NewString(), Status: "running"}
	require.NoError(t, repo.BeginExecution(ctx, task, &execution))
	at := time.Now()
	execution.Status = "completed"
	execution.FinishedAt = &at
	task.Payload.SummaryWork.Outputs = []string{"paid response"}
	invalid := execution
	invalid.ID = -1
	require.ErrorIs(t, repo.CompleteExecution(ctx, task, &invalid), service.ErrDesktopReportLeaseLost)
	stored, err := repo.Task(ctx, f.organization.ID, task.Date)
	require.NoError(t, err)
	require.Empty(t, stored.Payload.SummaryWork.Outputs)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM desktop_daily_report_executions WHERE id=$1`, execution.ID).Scan(&status))
	require.Equal(t, "running", status)
	require.NoError(t, repo.CompleteExecution(ctx, task, &execution))
	stored, err = repo.Task(ctx, f.organization.ID, task.Date)
	require.NoError(t, err)
	require.Equal(t, []string{"paid response"}, stored.Payload.SummaryWork.Outputs)
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM desktop_daily_report_executions WHERE id=$1`, execution.ID).Scan(&status))
	require.Equal(t, "completed", status)
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	period, err := repo.EnqueuePeriod(ctx, f.organization.ID, "week", start, start.AddDate(0, 0, 7), "generate")
	require.NoError(t, err)
	period.LeaseToken = uuid.NewString()
	period.Status = "running"
	_, err = integrationDB.Exec(`UPDATE desktop_period_report_tasks SET lease_token=$2,lease_until=NOW()+INTERVAL '3 minutes',status='running' WHERE id=$1`, period.InternalID, period.LeaseToken)
	require.NoError(t, err)
	pe := service.DesktopReportExecution{Revision: 1, Model: "model", Chunk: 1, Attempt: 1, RequestID: uuid.NewString(), Status: "running"}
	require.NoError(t, repo.BeginPeriodExecution(ctx, period, &pe))
	pe.Status = "completed"
	pe.FinishedAt = &at
	period.Work.Outputs = []string{"period response"}
	invalid = pe
	invalid.ID = -1
	require.ErrorIs(t, repo.CompletePeriodExecution(ctx, period, &invalid), service.ErrDesktopReportLeaseLost)
	before, err := repo.ReadPeriod(ctx, f.organization.ID, period.ID, true)
	require.NoError(t, err)
	require.Empty(t, before.Work.Outputs)
	require.NoError(t, repo.CompletePeriodExecution(ctx, period, &pe))
	after, err := repo.ReadPeriod(ctx, f.organization.ID, period.ID, true)
	require.NoError(t, err)
	require.Equal(t, []string{"period response"}, after.Work.Outputs)
}

func TestDesktopP2ExportFreezesMutableMemberPages(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "p2export", 250)
	p, err := integrationEntClient.DesktopPosition.Query().Where(desktopposition.PublicIDEQ(f.position.PublicID)).Only(ctx)
	require.NoError(t, err)
	builders := make([]*dbent.DesktopMemberCreate, 205)
	at := time.Now().Add(-time.Hour)
	for i := range builders {
		builders[i] = integrationEntClient.DesktopMember.Create().SetPublicID(fmt.Sprintf("mem_p2_%d_%d", f.suffix, i)).SetOrganizationID(f.organization.ID).SetName(fmt.Sprintf("Original %03d", i)).SetNameNormalized(fmt.Sprintf("original %03d", i)).SetPhone(desktopTestPhone(f.suffix + int64(i) + 1)).SetPositionID(p.ID).SetPositionRevision(1).SetUpdatedAt(at)
	}
	originals, err := integrationEntClient.DesktopMember.CreateBulk(builders...).Save(ctx)
	require.NoError(t, err)
	seen := map[string]string{}
	calls := 0
	err = f.repo.(service.DesktopMemberUsageExporter).StreamMembersWithUsage(ctx, f.organization.PublicID, service.DesktopMemberListFilters{}, time.Now(), func(org *service.DesktopOrganization, members []service.DesktopMember) error {
		require.Equal(t, f.organization.PublicID, org.PublicID)
		calls++
		for _, m := range members {
			require.NotContains(t, seen, m.PublicID)
			seen[m.PublicID] = m.Name
			require.NotNil(t, m.Usage)
		}
		if calls == 1 {
			_, e := integrationDB.Exec(`UPDATE desktop_members SET name='Changed during export',updated_at=NOW(),deleted_at=NOW() WHERE id=$1`, originals[0].ID)
			require.NoError(t, e)
			_, e = integrationEntClient.DesktopMember.Create().SetOrganizationID(f.organization.ID).SetPublicID("mem_added_" + fmt.Sprint(f.suffix)).SetName("Added during export").SetNameNormalized("added").SetPhone(desktopTestPhone(f.suffix + 300)).SetPositionID(p.ID).SetPositionRevision(1).Save(ctx)
			require.NoError(t, e)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
	require.Len(t, seen, 205)
	require.Equal(t, "Original 000", seen[originals[0].PublicID])
	require.NotContains(t, seen, "mem_added_"+fmt.Sprint(f.suffix))
	stop := errors.New("writer failed")
	err = f.repo.(service.DesktopMemberUsageExporter).StreamMembersWithUsage(ctx, f.organization.PublicID, service.DesktopMemberListFilters{}, time.Now(), func(*service.DesktopOrganization, []service.DesktopMember) error { return stop })
	require.ErrorIs(t, err, stop)
	called := false
	scoped := f.repo.ScopedToGatewayUser(f.user.ID + 1).(service.DesktopMemberUsageExporter)
	err = scoped.StreamMembersWithUsage(ctx, f.organization.PublicID, service.DesktopMemberListFilters{}, time.Now(), func(*service.DesktopOrganization, []service.DesktopMember) error { called = true; return nil })
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	require.False(t, called)
}

func TestDesktopP2WebBIUsageMeasurementAndBoundedDetails(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "p2stats", 4, true)
	member := f.createMember(t, "measured", 1)
	unknown := f.createMember(t, "unknown", 2)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "p2-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	at := time.Now().Add(-time.Second)
	_, err := integrationEntClient.UsageLog.Create().SetUserID(f.user.ID).SetAPIKeyID(*member.CurrentAPIKeyID).SetAccountID(account.ID).SetRequestID(uuid.NewString()).SetModel("large").SetInputTokens(2147483647).SetOutputTokens(2147483647).SetCacheReadTokens(3).SetActualCost(2).SetCreatedAt(at).Save(ctx)
	require.NoError(t, err)
	_, err = integrationEntClient.UsageLog.Create().SetUserID(f.user.ID).SetAPIKeyID(*unknown.CurrentAPIKeyID).SetAccountID(account.ID).SetRequestID(uuid.NewString()).SetModel("unmeasured").SetActualCost(1).SetCreatedAt(at).Save(ctx)
	require.NoError(t, err)
	now := time.Now()
	selected, err := (service.DesktopAnalyticsRangeInput{Days: 7}).Resolve(now)
	require.NoError(t, err)
	window := service.DesktopUsageWindows{Today: timezone.StartOfDay(now).UTC(), Week: timezone.StartOfWeek(now).UTC(), Month: timezone.StartOfMonth(now).UTC(), AsOf: now, Selected: selected, Query: service.DesktopUsageQuery{Page: 1, PageSize: 1}}
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	web, err := repo.GetDesktopOrganizationUsage(ctx, f.organization.ID, window)
	require.NoError(t, err)
	native, err := repo.NativeUsage(ctx, f.organization.ID, selected)
	require.NoError(t, err)
	require.Equal(t, "4294967297", *native.Totals.Total)
	require.EqualValues(t, 4294967297, web.Selected.TotalTokens)
	require.Equal(t, native.Totals.Status, web.Selected.TokenStatus)
	require.Equal(t, "partial", web.Selected.TokenStatus)
	require.Equal(t, 3.0, web.Selected.ActualCost)
	require.EqualValues(t, 2, web.MemberModelsPage.TotalMembers)
	require.Len(t, web.MemberModels, 1)
	window.Query.Page = 2
	window.MemberModelsOnly = true
	page, err := repo.GetDesktopOrganizationUsage(ctx, f.organization.ID, window)
	require.NoError(t, err)
	require.Len(t, page.MemberModels, 1)
	require.Equal(t, "unavailable", page.MemberModels[0].TokenStatus)
	window.MemberModelsOnly = false
	window.SummaryOnly = true
	summary, err := repo.GetDesktopOrganizationUsage(ctx, f.organization.ID, window)
	require.NoError(t, err)
	require.Empty(t, summary.MemberModels)
	require.Empty(t, summary.Models)
	require.Empty(t, summary.Daily)
	require.Nil(t, summary.MemberModelsPage)
	require.Equal(t, web.Total.TotalTokens, summary.Total.TotalTokens)
	members, err := repo.GetDesktopMembersUsage(ctx, []int64{member.ID, unknown.ID}, window.Today, selected.Start, now)
	require.NoError(t, err)
	require.EqualValues(t, 4294967297, members[member.ID].TotalTokens)
	require.Equal(t, "unavailable", members[unknown.ID].TotalTokenStatus)
}

func TestDesktopP2RetentionCursorFailureIsolationAndCatchup(t *testing.T) {
	ctx := context.Background()
	bad := newDesktopRepositoryFixture(t, "p2badcleanup", 1, true)
	good := newDesktopRepositoryFixture(t, "p2goodcleanup", 1, true)
	member := good.createMember(t, "cleanup", 1)
	repo := p2ReportRepository(t, good)
	_, err := integrationDB.Exec(`UPDATE desktop_organizations SET management_policy='{"conversation_retention_days":"invalid"}' WHERE id=$1`, bad.organization.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE desktop_organizations SET management_policy='{"conversation_retention_days":1}' WHERE id=$1`, good.organization.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO desktop_maintenance_progress(name,organization_id) VALUES('retention',0) ON CONFLICT(name) DO UPDATE SET organization_id=0`)
	require.NoError(t, err)
	old := time.Now().AddDate(0, 0, -10)
	_, err = NewDesktopConversationRepository(integrationEntClient).Create(ctx, good.organization.ID, member.ID, &service.DesktopConversationInput{RecordID: uuid.NewString(), InstallationID: uuid.NewString(), Client: "workbuddy", SessionID: "cleanup", StartedAt: old.Add(-time.Minute), StoppedAt: old, Prompts: []service.DesktopTextSegment{{Text: "expired"}}, CaptureStatus: "response_missing"})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE desktop_conversation_records SET received_at=$2 WHERE organization_id=$1`, good.organization.ID, old)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM desktop_conversation_records WHERE organization_id=$1`, good.organization.ID)
	})
	// Clone a valid row to cross the per-batch limit and verify bounded continuation.
	columnRows, e := integrationDB.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='desktop_conversation_records' AND column_name NOT IN ('id','record_id') ORDER BY ordinal_position`)
	require.NoError(t, e)
	columns := []string{}
	for columnRows.Next() {
		var column string
		require.NoError(t, columnRows.Scan(&column))
		columns = append(columns, `"`+column+`"`)
	}
	require.NoError(t, columnRows.Err())
	require.NoError(t, columnRows.Close())
	names := strings.Join(columns, ",")
	_, err = integrationDB.Exec(`INSERT INTO desktop_conversation_records(record_id,`+names+`) SELECT gen_random_uuid(),`+names+` FROM desktop_conversation_records CROSS JOIN generate_series(1,1100) WHERE organization_id=$1`, good.organization.ID)
	require.NoError(t, err)
	// An active scanner lease prevents a second instance from advancing the same cursor.
	_, err = integrationDB.Exec(`UPDATE desktop_maintenance_progress SET lease_token='other-instance',lease_until=NOW()+INTERVAL '45 seconds' WHERE name='retention'`)
	require.NoError(t, err)
	require.NoError(t, repo.Cleanup(ctx, time.Now()))
	var beforeCleanup int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM desktop_conversation_records WHERE organization_id=$1`, good.organization.ID).Scan(&beforeCleanup))
	require.Equal(t, 1101, beforeCleanup)
	_, err = integrationDB.Exec(`UPDATE desktop_maintenance_progress SET lease_until=NOW()-INTERVAL '1 second' WHERE name='retention'`)
	require.NoError(t, err)
	failed := false
	for i := 0; i < 20; i++ {
		if err := repo.Cleanup(ctx, time.Now()); err != nil {
			failed = true
		}
		var count int
		require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM desktop_conversation_records WHERE organization_id=$1`, good.organization.ID).Scan(&count))
		if count == 0 {
			break
		}
	}
	require.True(t, failed, "the invalid enterprise must be reported without stopping the rest")
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM desktop_conversation_records WHERE organization_id=$1`, good.organization.ID).Scan(&count))
	require.Zero(t, count)
	kind := "test-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM desktop_maintenance_progress WHERE name=$1`, "backfill:"+kind)
	})
	first := time.Date(2026, 9, 1, 0, 0, 0, 0, timezone.Location())
	next, err := repo.ReportScanDate(ctx, kind, first)
	require.NoError(t, err)
	require.True(t, first.Equal(next))
	require.NoError(t, repo.AdvanceReportScan(ctx, kind, first.AddDate(0, 0, 4)))
	require.NoError(t, repo.AdvanceReportScan(ctx, kind, first.AddDate(0, 0, 1)))
	resumed, err := repo.ReportScanDate(ctx, kind, first)
	require.NoError(t, err)
	require.True(t, first.AddDate(0, 0, 4).Equal(resumed))
}

func TestDesktopP2UsageResponseSnapshotReconcilesWithLateWrites(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "p2snapshot", 2, true)
	member := f.createMember(t, "snapshot", 1)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "p2snapshot-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	at := time.Now().Add(-time.Second)
	now := time.Now()
	selected, err := (service.DesktopAnalyticsRangeInput{Days: 7}).Resolve(now)
	require.NoError(t, err)
	window := service.DesktopUsageWindows{Today: timezone.StartOfDay(now).UTC(), Week: timezone.StartOfWeek(now).UTC(), Month: timezone.StartOfMonth(now).UTC(), AsOf: now, Selected: selected}
	insert := func(i int) error {
		_, e := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,billing_mode,created_at) VALUES($1,$2,$3,$4,'snapshot',1,1,'token',$5)`, f.user.ID, *member.CurrentAPIKeyID, account.ID, fmt.Sprintf("p2snapshot-%d-%d", account.ID, i), at)
		return e
	}
	require.NoError(t, insert(-1))
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 250; i++ {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			if err := insert(i); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	t.Cleanup(func() { close(stop); require.NoError(t, <-done) })
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, integrationDB)
	for i := 0; i < 10; i++ {
		out, err := repo.GetDesktopOrganizationUsage(ctx, f.organization.ID, window)
		require.NoError(t, err)
		var tokens int64
		for _, day := range out.Daily {
			tokens += day.TotalTokens
		}
		require.Equal(t, out.Selected.TotalTokens, tokens)
		require.Equal(t, out.Selected.TotalTokens, out.MemberModelsPage.TotalTokens)
		require.EqualValues(t, out.Selected.MeasuredRequests*2, tokens)
		require.NotContains(t, strings.Join(out.MemberModelsPage.Models, ","), "unrelated")
	}
}
