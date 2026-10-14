//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDesktopPeriodPersistenceSourcesVersionsLeaseAndBilling(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "period", 2, true)
	repo := NewDesktopReportRepository(integrationDB, integrationEntClient).(service.DesktopPeriodRepository)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM desktop_period_report_executions WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.Exec(`DELETE FROM desktop_period_report_tasks WHERE organization_id=$1`, f.organization.ID)
		_, _ = integrationDB.Exec(`DELETE FROM desktop_daily_report_tasks WHERE organization_id=$1`, f.organization.ID)
	})
	start, end, e := reportBounds("2026-09-28")
	require.NoError(t, e)
	end = start.AddDate(0, 0, 7)
	_, e = integrationDB.Exec(`INSERT INTO desktop_daily_report_tasks(organization_id,report_date,status,summary,payload) VALUES($1,'2026-09-28','completed','{"status":"completed","content":"DAILY_ONLY","generated_at":"2026-09-29T00:00:00Z"}','{"snapshots":"FORBIDDEN_MEMBER_RAW"}'),($1,'2026-09-29','skipped','{}','{}')`, f.organization.ID)
	require.NoError(t, e)
	sources, coverage, e := repo.PeriodSources(ctx, f.organization.ID, start, end)
	require.NoError(t, e)
	require.Len(t, sources, 1)
	require.Equal(t, "DAILY_ONLY", sources[0].Content)
	require.Len(t, coverage.Missing, 6)
	require.Len(t, sources[0].Hash, 64)
	var wg sync.WaitGroup
	ids := make(chan string, 4)
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := repo.EnqueuePeriod(ctx, f.organization.ID, "week", start, end, "generate")
			if e != nil {
				errs <- e
				return
			}
			ids <- v.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var id string
	for v := range ids {
		if id != "" {
			require.Equal(t, id, v)
		}
		id = v
	}
	task, e := repo.ClaimPeriod(ctx)
	require.NoError(t, e)
	require.NotNil(t, task)
	require.Equal(t, id, task.ID)
	task.Sources = sources
	task.Coverage = coverage
	task.Configuration = service.DesktopPeriodConfiguration{Captured: true, Model: "model", UserID: f.user.ID, GroupID: f.group.ID}
	task.Work.Inputs = []string{"FROZEN_INPUT"}
	task.Work.Outputs = []string{"CHECKPOINT"}
	require.NoError(t, repo.SavePeriod(ctx, task, time.Now(), false))
	require.NoError(t, repo.RenewPeriod(ctx, task))
	e1 := &service.DesktopReportExecution{Model: "model", Round: 1, Chunk: 1, Attempt: 1, RequestID: "period-request-" + task.ID}
	require.NoError(t, repo.BeginPeriodExecution(ctx, task, e1))
	require.NoError(t, repo.RecordPeriodExecutionUsage(ctx, e1.ID, &service.UsageLog{InputTokens: 12, OutputTokens: 3, ActualCost: 0.2}))
	e1.Status = "completed"
	now := time.Now()
	e1.FinishedAt = &now
	require.NoError(t, repo.FinishPeriodExecution(ctx, e1))
	task.Status = "failed"
	require.NoError(t, repo.SavePeriod(ctx, task, time.Now(), true))
	retry, e := repo.EnqueuePeriod(ctx, f.organization.ID, "week", start, end, "retry")
	require.NoError(t, e)
	require.Equal(t, id, retry.ID)
	require.Equal(t, []string{"CHECKPOINT"}, retry.Work.Outputs)
	require.Equal(t, "DAILY_ONLY", retry.Sources[0].Content)
	task, e = repo.ClaimPeriod(ctx)
	require.NoError(t, e)
	text := "ARCHIVED_RESULT"
	task.Content = &text
	task.GeneratedAt = &now
	task.Status = "partial"
	require.NoError(t, repo.SavePeriod(ctx, task, time.Now(), true))
	_, e = integrationDB.Exec(`UPDATE desktop_daily_report_tasks SET revision=revision+1,summary='{"status":"completed","content":"DAILY_UPDATED","generated_at":"2026-09-30T00:00:00Z"}' WHERE organization_id=$1 AND report_date='2026-09-28'`, f.organization.ID)
	require.NoError(t, e)
	meta, e := repo.ReadPeriod(ctx, f.organization.ID, id, false)
	require.NoError(t, e)
	require.True(t, meta.SourceChanged)
	require.Nil(t, meta.Content)
	require.Empty(t, meta.Sources)
	list, _, e := repo.ListPeriods(ctx, f.organization.ID, "week", "", 1, 20)
	require.NoError(t, e)
	raw, e := json.Marshal(list)
	require.NoError(t, e)
	require.NotContains(t, string(raw), "ARCHIVED_RESULT")
	require.NotContains(t, string(raw), "FROZEN_INPUT")
	old, e := repo.ReadPeriod(ctx, f.organization.ID, id, true)
	require.NoError(t, e)
	require.Equal(t, "ARCHIVED_RESULT", *old.Content)
	source, e := repo.ReadPeriodSource(ctx, f.organization.ID, id, "2026-09-28")
	require.NoError(t, e)
	require.Equal(t, "DAILY_ONLY", source.Content)
	regenerated, e := repo.EnqueuePeriod(ctx, f.organization.ID, "week", start, end, "regenerate")
	require.NoError(t, e)
	require.Equal(t, 2, regenerated.Revision)
	require.NotEqual(t, id, regenerated.ID)
	require.Empty(t, regenerated.Sources)
	readable, total, e := repo.ListPeriods(ctx, f.organization.ID, "week", "", 1, 20, true)
	require.NoError(t, e)
	require.EqualValues(t, 1, total)
	require.Equal(t, id, readable[0].ID)
	// Pending regeneration must not replace the most recent readable version.

	_, e = integrationDB.Exec(`UPDATE desktop_period_report_tasks SET status='running',lease_token='abandoned-token',lease_until=NOW()-INTERVAL '1 minute' WHERE id=$1`, regenerated.InternalID)
	require.NoError(t, e)
	recovered, e := repo.ClaimPeriod(ctx)
	require.NoError(t, e)
	require.Equal(t, regenerated.ID, recovered.ID)
	require.NotEqual(t, "abandoned-token", recovered.LeaseToken)
	_, e = integrationDB.Exec(`UPDATE desktop_organizations SET management_policy='{"report_retention_days":1,"execution_retention_days":2}' WHERE id=$1`, f.organization.ID)
	require.NoError(t, e)
	cleaner := NewDesktopReportRepository(integrationDB, integrationEntClient).(interface {
		Cleanup(context.Context, time.Time) error
	})
	require.NoError(t, cleaner.Cleanup(ctx, time.Now()))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM desktop_period_report_tasks WHERE public_id=$1`, recovered.ID).Scan(&count))
	require.Equal(t, 1, count)
	var taskRef *int64
	var tokens *int64
	require.NoError(t, integrationDB.QueryRow(`SELECT task_id,total_tokens FROM desktop_period_report_executions WHERE id=$1`, e1.ID).Scan(&taskRef, &tokens))
	require.Nil(t, taskRef)
	require.EqualValues(t, 15, *tokens)
	old.LeaseToken = "invalid"
	require.ErrorIs(t, repo.SavePeriod(ctx, old, time.Now(), false), service.ErrDesktopReportLeaseLost)
}
func TestBIUpgradeDropsOnlyLegacyDataAndKeepsSources(t *testing.T) {
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	_, e = tx.Exec(`CREATE SCHEMA bi_upgrade_check;SET LOCAL search_path=bi_upgrade_check;CREATE TABLE users(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE desktop_organizations(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE desktop_members(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE usage_logs(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE desktop_conversation_records(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE desktop_daily_report_tasks(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE desktop_daily_report_executions(id BIGINT PRIMARY KEY,payload TEXT);CREATE TABLE settings(id BIGINT PRIMARY KEY,payload TEXT)`)
	require.NoError(t, e)
	for _, table := range []string{"users", "desktop_organizations", "desktop_members", "usage_logs", "desktop_conversation_records", "desktop_daily_report_tasks", "desktop_daily_report_executions", "settings"} {
		_, e = tx.Exec(`INSERT INTO ` + table + ` VALUES(1,'SOURCE_UNCHANGED')`)
		require.NoError(t, e)
	}
	entries, e := migrations.FS.ReadDir(".")
	require.NoError(t, e)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "_bi_") && entry.Name() < "257" {
			sql, e := migrations.FS.ReadFile(entry.Name())
			require.NoError(t, e)
			_, e = tx.Exec(string(sql))
			require.NoError(t, e, entry.Name())
		}
	}
	_, e = tx.Exec(`INSERT INTO bi_managers(id,user_id) VALUES('legacy',1);INSERT INTO bi_organizations(id,desktop_organization_id) VALUES('legacy_org',1);INSERT INTO bi_manager_grants(manager_id,organization_id,role) VALUES('legacy','legacy_org','viewer')`)
	require.NoError(t, e)
	migration, e := migrations.FS.ReadFile("257_bi_independent_users.sql")
	require.NoError(t, e)
	require.NotContains(t, strings.ToUpper(string(migration)), "DROP TABLE IF EXISTS USERS")
	_, e = tx.Exec(string(migration))
	require.NoError(t, e)
	for _, table := range []string{"users", "desktop_organizations", "desktop_members", "usage_logs", "desktop_conversation_records", "desktop_daily_report_tasks", "desktop_daily_report_executions", "settings"} {
		var payload string
		require.NoError(t, tx.QueryRow(`SELECT payload FROM `+table+` WHERE id=1`).Scan(&payload))
		require.Equal(t, "SOURCE_UNCHANGED", payload)
	}
	var oldExists bool
	require.NoError(t, tx.QueryRow(`SELECT to_regclass('bi_upgrade_check.bi_managers') IS NOT NULL`).Scan(&oldExists))
	require.False(t, oldExists)
	var count int
	require.NoError(t, tx.QueryRow(`SELECT COUNT(*) FROM bi_users`).Scan(&count))
	require.Zero(t, count)
}
