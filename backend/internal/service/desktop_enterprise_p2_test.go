package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type checkpointFailureRepo struct{ *reportTestRepo }

func (r *checkpointFailureRepo) CompleteExecution(context.Context, *DesktopReportTask, *DesktopReportExecution) error {
	return errors.New("commit acknowledgement lost")
}
func TestDesktopP2UncertainCheckpointDoesNotOverwriteStoredOutcome(t *testing.T) {
	s, base, ai, _ := reportFixture()
	repo := &checkpointFailureRepo{base}
	s.repo = repo
	base.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	s.run(context.Background(), task)
	require.Len(t, ai.calls, 1)
	require.Len(t, base.executions, 1)
	require.Equal(t, "running", base.executions[0].Status)
	require.Empty(t, task.Payload.Snapshots[0].Work.Outputs)
	require.Equal(t, 2, base.saved, "initial snapshot and pre-call attempt only; an uncertain commit must never be overwritten")
}

type maintenanceTestRepo struct {
	*reportTestRepo
	next time.Time
}

func (r *maintenanceTestRepo) ReportScanDate(_ context.Context, _ string, first time.Time) (time.Time, error) {
	if r.next.Before(first) {
		r.next = first
	}
	return r.next, nil
}
func (r *maintenanceTestRepo) AdvanceReportScan(_ context.Context, _ string, next time.Time) error {
	r.next = next
	return nil
}
func TestDesktopP2BackfillResumesAndDoesNotAdvancePastFailure(t *testing.T) {
	s, base, _, _ := reportFixture()
	repo := &maintenanceTestRepo{reportTestRepo: base}
	s.repo = repo
	first := time.Date(2026, 9, 1, 0, 0, 0, 0, s.loc)
	last := first.AddDate(0, 0, 12)
	dates := []time.Time{}
	step := func(d time.Time) time.Time { return d.AddDate(0, 0, 1) }
	scan := func(d time.Time) error { dates = append(dates, d); return nil }
	s.backfill(context.Background(), "day", first, last, step, scan)
	require.Len(t, dates, 1)
	require.Equal(t, first.AddDate(0, 0, 1), repo.next)
	restarted := NewDesktopReportService(s.desktop, repo)
	restarted.backfill(context.Background(), "day", first, last, step, scan)
	require.Len(t, dates, 2)
	require.Equal(t, first.AddDate(0, 0, 2), repo.next)
	failed := repo.next
	restarted.backfill(context.Background(), "day", first, last, step, func(time.Time) error { return errors.New("database unavailable") })
	require.Equal(t, failed, repo.next)
	restarted.backfill(context.Background(), "day", first, last, step, scan)
	require.Equal(t, failed, dates[2])
}

type queueOrderRepo struct {
	*periodTestRepo
	order []string
}

func (r *queueOrderRepo) Claim(context.Context) (*DesktopReportTask, error) {
	r.order = append(r.order, "day")
	return nil, nil
}
func (r *queueOrderRepo) ClaimPeriod(context.Context) (*DesktopPeriodTask, error) {
	r.order = append(r.order, "period")
	return nil, nil
}
func TestDesktopP2QueuePriorityAlternatesWithoutStarvingPeriods(t *testing.T) {
	s, base, _, _ := reportFixture()
	repo := &queueOrderRepo{periodTestRepo: &periodTestRepo{reportTestRepo: base}}
	s.repo = repo
	ran, err := s.processNext(context.Background(), true)
	require.NoError(t, err)
	require.False(t, ran)
	require.Equal(t, []string{"period", "day"}, repo.order)
	repo.order = nil
	ran, err = s.processNext(context.Background(), false)
	require.NoError(t, err)
	require.False(t, ran)
	require.Equal(t, []string{"day", "period"}, repo.order)
}

type exportTestRepo struct {
	*reportTestIdentity
	members []DesktopMember
	failure error
}

func (r *exportTestRepo) StreamMembersWithUsage(_ context.Context, _ string, _ DesktopMemberListFilters, _ time.Time, write func(*DesktopOrganization, []DesktopMember) error) error {
	if r.failure != nil {
		return r.failure
	}
	return write(r.organization, r.members)
}
func TestDesktopP2CSVPreservesFormattingAndEscapesUntrustedValues(t *testing.T) {
	repo := &exportTestRepo{reportTestIdentity: &reportTestIdentity{organization: &DesktopOrganization{PublicID: "org_one", Name: "=SUM(A1)", Code: "company"}}}
	repo.members = []DesktopMember{{PublicID: "mem_one", Name: "\t=HYPERLINK(1)", Phone: "+8613800138000", Status: "active", Usage: &DesktopMemberUsage{TotalTokens: 123, TodayTokenStatus: "unavailable"}}}
	s := &DesktopService{repo: repo, now: time.Now}
	format := DesktopMemberExportFormat{Headers: strings.Split(strings.Repeat("Column,", 18)+"Column", ","), Active: "Active", Disabled: "Disabled", TokenLabels: map[string]string{"active": "Active", "disabled": "Disabled", "missing": "Missing"}}
	var out bytes.Buffer
	require.NoError(t, s.ExportMemberUsage(context.Background(), "org_one", 0, DesktopMemberListFilters{}, format, &out))
	require.True(t, strings.HasPrefix(out.String(), "\uFEFF"))
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\uFEFF")))
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Len(t, rows[1], 19)
	require.Equal(t, "'=SUM(A1)", rows[1][0])
	require.Equal(t, "'\t=HYPERLINK(1)", rows[1][3])
	require.Equal(t, "'+8613800138000", rows[1][7])
	require.Empty(t, rows[1][13])
	require.Equal(t, "123", rows[1][15])
	repo.members = nil
	require.ErrorIs(t, s.ExportMemberUsage(context.Background(), "org_one", 0, DesktopMemberListFilters{}, format, io.Discard), ErrDesktopExportEmpty)
	format.Headers = format.Headers[:1]
	require.ErrorIs(t, s.ExportMemberUsage(context.Background(), "org_one", 0, DesktopMemberListFilters{}, format, io.Discard), ErrDesktopValidation)
}

type drainingReportRepo struct {
	*reportTestRepo
	cancel context.CancelFunc
}

func (r *drainingReportRepo) Claim(context.Context) (*DesktopReportTask, error) {
	r.claims++
	if r.claims > 7 {
		r.cancel()
		return nil, nil
	}
	return &DesktopReportTask{ID: int64(r.claims), OrganizationID: 1, Date: "2026-10-09", Status: "running", Revision: 1}, nil
}
func TestDesktopP2WorkerKeepsClaimingWithoutWaitingForTheNextMaintenanceTick(t *testing.T) {
	s, base, ai, organization := reportFixture()
	organization.ConversationSummaryEnabled = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &drainingReportRepo{reportTestRepo: base, cancel: cancel}
	s.repo = repo
	s.wg.Add(1)
	s.worker(ctx, false)
	s.wg.Wait()
	require.Equal(t, 7, repo.saved)
	require.Empty(t, ai.calls)
}
