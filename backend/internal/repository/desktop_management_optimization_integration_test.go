//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopReportMigrationAcceptsLegacyPendingPayloads(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE desktop_daily_report_tasks (LIKE public.desktop_daily_report_tasks INCLUDING DEFAULTS); CREATE TEMP TABLE desktop_daily_report_members (LIKE public.desktop_daily_report_members INCLUDING DEFAULTS); INSERT INTO desktop_daily_report_tasks(id,organization_id,report_date,payload) VALUES (1,1,'2026-10-01','{"snapshots":null}'),(2,1,'2026-10-02','{}'),(3,1,'2026-10-03','{"snapshots":[{"member":{"member_id":"legacy-member","content":"preserved report"},"sources":[],"work":{}}]}')`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("255_desktop_management_optimization.sql")
	require.NoError(t, err)
	sql := string(content)
	start := strings.Index(sql, "INSERT INTO desktop_daily_report_members")
	end := strings.Index(sql, "CREATE INDEX idx_desktop_report_execution_member")
	require.Greater(t, start, 0)
	require.Greater(t, end, start)
	_, err = tx.ExecContext(ctx, sql[start:end])
	require.NoError(t, err)
	var text string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT member->>'content' FROM desktop_daily_report_members WHERE task_id=3`).Scan(&text))
	require.Equal(t, "preserved report", text)
	var remaining int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_daily_report_tasks WHERE payload ? 'snapshots'`).Scan(&remaining))
	require.Zero(t, remaining)
}

func TestDesktopBudgetRotationPreservesLimitsAndWindows(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "budgetrotation", 2, true)
	m := f.createMember(t, "budgetrotation", 1)
	budget := &service.DesktopMemberBudget{Quota: 10, Limit1d: 3, Limit7d: 15}
	m, _, err := f.repo.UpdateMember(ctx, f.organization.PublicID, m.PublicID, service.DesktopUpdateMemberInput{Budget: budget})
	require.NoError(t, err)
	window := time.Now().Add(-time.Hour)
	_, err = integrationEntClient.APIKey.UpdateOneID(*m.CurrentAPIKeyID).SetQuotaUsed(4).SetUsage1d(1).SetUsage7d(2).SetWindow1dStart(window).SetWindow7dStart(window).Save(ctx)
	require.NoError(t, err)
	key := &service.APIKey{UserID: f.organization.GatewayUserID, GroupID: &f.organization.GroupID, Name: "rotated-budget", Key: "sk-" + uuid.NewString(), Status: service.StatusAPIKeyActive}
	rotated, _, err := f.repo.RotateMemberAPIKey(ctx, f.organization.PublicID, m.PublicID, key)
	require.NoError(t, err)
	require.NotEqual(t, *m.CurrentAPIKeyID, *rotated.CurrentAPIKeyID)
	require.Equal(t, 10.0, rotated.Budget.Quota)
	require.Equal(t, 4.0, rotated.Budget.QuotaUsed)
	require.Equal(t, 1.0, rotated.Budget.Usage1d)
	require.Equal(t, 2.0, rotated.Budget.Usage7d)
	require.Equal(t, 3.0, rotated.Budget.Limit1d)
}

func TestDesktopReportReadsAndRetentionPreserveIsolation(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "optimizedreports", 3, true)
	m := f.createMember(t, "optimizedreports", 1)
	f.createMember(t, "optimizedsecond", 2)
	other := newDesktopRepositoryFixture(t, "optimizedoutside", 1, true)
	repo := NewDesktopReportRepository(integrationDB, integrationEntClient).(*desktopReportRepository)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_daily_report_executions WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_daily_report_tasks WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM desktop_conversation_records WHERE organization_id=$1`, f.organization.ID)
	})
	now := time.Now()
	old := now.AddDate(0, 0, -10)
	date := old.In(time.FixedZone("Shanghai", 8*3600)).Format("2006-01-02")
	id := uuid.NewString()
	_, err := NewDesktopConversationRepository(integrationEntClient).Create(ctx, f.organization.ID, m.ID, &service.DesktopConversationInput{RecordID: id, InstallationID: uuid.NewString(), Client: "workbuddy", SessionID: "report-session", StartedAt: old.Add(-time.Minute), StoppedAt: old, Prompts: []service.DesktopTextSegment{{Text: "PRIVATE-SOURCE-TEXT"}}, CaptureStatus: "response_missing"})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE desktop_conversation_records SET received_at=$2 WHERE record_id=$1::uuid`, id, old)
	require.NoError(t, err)
	_, err = repo.Enqueue(ctx, f.organization.ID, date, "generate")
	require.NoError(t, err)
	lease, err := repo.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, lease)
	snapshots, err := repo.Snapshot(ctx, f.organization.ID, date)
	require.NoError(t, err)
	snapshots[0].Member.Status = "completed"
	snapshots[0].Member.Content = "PRIVATE-REPORT-TEXT"
	snapshots[0].Work.Inputs = []string{"PRIVATE-CHECKPOINT-TEXT"}
	lease.Payload = service.DesktopReportPayload{Model: "test-model", GroupID: f.organization.GroupID, UserID: f.organization.GatewayUserID, Snapshots: snapshots, Summary: service.DesktopGeneratedReport{Status: "completed", Content: "summary"}}
	require.NoError(t, repo.Save(ctx, lease, now, false))
	execution := service.DesktopReportExecution{Kind: "member", MemberID: m.PublicID, Model: "test-model", Chunk: 1, Attempt: 1, RequestID: uuid.NewString(), Status: "running"}
	require.NoError(t, repo.BeginExecution(ctx, lease, &execution))
	execution.Status = "completed"
	execution.FinishedAt = &now
	require.NoError(t, repo.FinishExecution(ctx, &execution))
	require.NoError(t, repo.RecordExecutionUsage(ctx, execution.ID, &service.UsageLog{InputTokens: 10, OutputTokens: 20, ActualCost: 0.25}))
	var taskBody string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload::text FROM desktop_daily_report_tasks WHERE id=$1`, lease.ID).Scan(&taskBody))
	require.NotContains(t, taskBody, "PRIVATE-")
	query := service.DesktopReportQuery{Section: "members", Page: 1, PageSize: 1, ExecutionPage: 1}
	list, err := repo.ReadDay(ctx, f.organization.ID, date, query)
	require.NoError(t, err)
	require.Len(t, list.Members, 1)
	require.Equal(t, 2, list.TotalMembers)
	require.Empty(t, list.Members[0].Content)
	require.Empty(t, list.Members[0].SourceIDs)
	query.MemberID = m.PublicID
	detail, err := repo.ReadDay(ctx, f.organization.ID, date, query)
	require.NoError(t, err)
	require.Equal(t, "PRIVATE-REPORT-TEXT", detail.Members[0].Content)
	require.Len(t, detail.Members[0].Executions, 1)
	require.EqualValues(t, 30, *detail.Members[0].Executions[0].TotalTokens)
	require.Equal(t, 0.25, *detail.Members[0].Executions[0].ActualCost)
	_, err = repo.ReadDay(ctx, other.organization.ID, date, query)
	require.ErrorIs(t, err, service.ErrDesktopMemberNotFound)
	policy := &service.DesktopManagementPolicy{ConversationRetentionDays: 1, ReportRetentionDays: 20, ExecutionRetentionDays: 30}
	_, _, err = f.repo.UpdateOrganization(ctx, f.organization.PublicID, service.DesktopUpdateOrganizationInput{ManagementPolicy: policy})
	require.NoError(t, err)
	require.NoError(t, repo.Cleanup(ctx, now))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_conversation_records WHERE record_id=$1::uuid`, id).Scan(&count))
	require.Equal(t, 1, count, "active source must survive retention")
	lease.Status = "completed"
	require.NoError(t, repo.Save(ctx, lease, now, true))
	require.NoError(t, repo.Cleanup(ctx, now))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_conversation_records WHERE record_id=$1::uuid`, id).Scan(&count))
	require.Zero(t, count)
	resumed, err := repo.Task(ctx, f.organization.ID, date)
	require.NoError(t, err)
	require.Empty(t, resumed.Payload.Snapshots[0].Sources)
	require.Empty(t, resumed.Payload.Snapshots[0].Work.Inputs)
	require.Equal(t, "PRIVATE-REPORT-TEXT", resumed.Payload.Snapshots[0].Member.Content)
	policy.ReportRetentionDays = 1
	_, _, err = f.repo.UpdateOrganization(ctx, f.organization.PublicID, service.DesktopUpdateOrganizationInput{ManagementPolicy: policy})
	require.NoError(t, err)
	require.NoError(t, repo.Cleanup(ctx, now))
	removed, err := repo.Task(ctx, f.organization.ID, date)
	require.NoError(t, err)
	require.Nil(t, removed)
	history, err := repo.Executions(ctx, f.organization.ID, date)
	require.NoError(t, err)
	require.Len(t, history, 1, "safe execution history has its own retention")
}
