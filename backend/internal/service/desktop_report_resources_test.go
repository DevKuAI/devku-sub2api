package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type boundedSourceRepo struct {
	*reportTestRepo
	requested [][]string
	missing   bool
	metadata  []DesktopReportSnapshot
}

func (r *boundedSourceRepo) SnapshotMetadata(context.Context, int64, string) ([]DesktopReportSnapshot, error) {
	return r.metadata, nil
}

func (r *boundedSourceRepo) SnapshotSources(_ context.Context, _ int64, ids []string) ([]DesktopReportSource, error) {
	r.requested = append(r.requested, append([]string{}, ids...))
	if len(ids) > desktopReportSourceBatchSize {
		return nil, errors.New("decoded source batch exceeds budget")
	}
	sources := []DesktopReportSource{}
	for _, id := range ids {
		sources = append(sources, DesktopReportSource{RecordID: id, Prompts: []DesktopTextSegment{{Text: "evidence " + id}}})
	}
	if r.missing {
		return sources[:len(sources)-1], nil
	}
	return sources, nil
}

func TestDesktopReportMemberInputsReadBoundedBodiesWithoutLosingEvidence(t *testing.T) {
	s, base, _, _ := reportFixture()
	repo := &boundedSourceRepo{reportTestRepo: base}
	s.repo = repo
	ids := []string{}
	for i := 0; i < 19; i++ {
		ids = append(ids, fmt.Sprintf("source-%03d", i))
	}
	snapshot := &DesktopReportSnapshot{Member: DesktopReportMember{SourceIDs: ids, RecordCount: len(ids)}}
	inputs, err := s.memberInputs(context.Background(), 1, snapshot, "test-model")
	require.NoError(t, err)
	require.Len(t, repo.requested, 5)
	expected := []DesktopReportSource{}
	for _, id := range ids {
		expected = append(expected, DesktopReportSource{RecordID: id, Prompts: []DesktopTextSegment{{Text: "evidence " + id}}})
	}
	require.Equal(t, reportSourceInputs(expected, "test-model"), inputs, "database batches must not introduce extra model calls")
	all := strings.Join(inputs, "\n")
	for _, id := range ids {
		require.Contains(t, all, "evidence "+id)
	}
	require.Empty(t, snapshot.Sources)
	repo.missing = true
	inputs, err = s.memberInputs(context.Background(), 1, snapshot, "test-model")
	require.Error(t, err)
	require.Nil(t, inputs, "incomplete evidence must never become a checkpoint")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := len(repo.requested)
	_, err = s.memberInputs(ctx, 1, snapshot, "test-model")
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, repo.requested, before)
}

func TestDesktopReportConsumedInputsCommitWithOutputsOrRestoreOnUncertainCommit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			s, _, _, org := reportFixture()
			work := DesktopReportWork{Inputs: []string{"source evidence"}}
			uncertain := errors.New("commit acknowledgement lost")
			hooks := reportGenerationHooks{
				Model:        org.AnalysisModel,
				Organization: func(context.Context) (*DesktopOrganization, error) { return org, nil },
				Validate:     func(*DesktopOrganization) error { return nil },
				Save:         func(context.Context) error { return nil },
				Begin:        func(context.Context, *DesktopReportExecution) error { return nil },
				Complete: func(context.Context, *DesktopReportExecution) error {
					require.Equal(t, []string{""}, work.Inputs)
					require.Len(t, work.Outputs, 1)
					if fail {
						return uncertain
					}
					return nil
				},
			}
			content, err := s.generateSummary(context.Background(), &APIKey{}, &work, "prompt", "", "", hooks)
			if fail {
				require.ErrorIs(t, err, ErrDesktopReportCheckpointUncertain)
				require.ErrorIs(t, err, uncertain)
				require.Equal(t, []string{"source evidence"}, work.Inputs)
				require.Empty(t, work.Outputs)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, content)
			require.Equal(t, []string{""}, work.Inputs)
		})
	}
}

func TestDesktopReportWorkerPreparesLargeMemberInBoundedBatches(t *testing.T) {
	s, base, ai, _ := reportFixture()
	ids := []string{}
	for i := 0; i < 19; i++ {
		ids = append(ids, fmt.Sprintf("record-%d", i))
	}
	repo := &boundedSourceRepo{reportTestRepo: base, metadata: []DesktopReportSnapshot{{Member: DesktopReportMember{
		MemberID: "member", Name: "Member", RecordCount: len(ids), SourceIDs: ids, Status: "pending",
	}}}}
	s.repo = repo
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1, Status: "pending"}
	s.run(context.Background(), task)
	require.Equal(t, "completed", task.Status)
	require.Equal(t, "completed", task.Payload.Snapshots[0].Member.Status)
	require.Len(t, repo.requested, 5)
	require.Len(t, ai.calls, 2, "one member chunk and one enterprise summary")
	require.Empty(t, task.Payload.Snapshots[0].Sources)
	require.Empty(t, task.Payload.Snapshots[0].Work.Inputs)
}

type noDayAggregationRepo struct{ *reportTestRepo }

func (r *noDayAggregationRepo) Day(context.Context, int64, string) ([]DesktopReportMember, error) {
	panic("automatic scans must not aggregate member source IDs")
}

func TestDesktopReportAutomaticBatchDefersCursorUntilRemainingOrganizationsAreQueued(t *testing.T) {
	s, base, _, _ := reportFixture()
	base.ids = make([]string, DesktopReportScanBatchSize+3)
	for i := range base.ids {
		base.ids[i] = fmt.Sprint(i)
	}
	s.repo = &noDayAggregationRepo{base}
	err := s.enqueueAutomaticDate(context.Background(), "2026-10-09")
	require.ErrorIs(t, err, errDesktopReportScanDeferred)
	require.Equal(t, DesktopReportScanBatchSize, base.enqueued)
	repo := &maintenanceTestRepo{reportTestRepo: base}
	s.repo = repo
	first := s.now().AddDate(0, 0, -2)
	step := func(d time.Time) time.Time { return d.AddDate(0, 0, 1) }
	scan := func(d time.Time) error { return s.enqueueAutomaticDate(context.Background(), d.Format("2006-01-02")) }
	s.backfill(context.Background(), "day", first, first.AddDate(0, 0, 2), step, scan)
	require.Equal(t, first, repo.next)
	base.ids = nil
	s.backfill(context.Background(), "day", first, first.AddDate(0, 0, 2), step, scan)
	require.Equal(t, step(first), repo.next)
}

func TestDesktopReportResumeReusesCommittedOutputWithReleasedInput(t *testing.T) {
	s, _, ai, _ := reportFixture()
	for _, input := range []string{"", "legacy input"} {
		work := DesktopReportWork{Inputs: []string{input}, Outputs: []string{"committed output"}}
		content, err := s.generateSummary(context.Background(), nil, &work, "", "", "", reportGenerationHooks{})
		require.NoError(t, err)
		require.Equal(t, "committed output", content)
	}
	require.Empty(t, ai.calls)
}

type idleReportRepo struct {
	*reportTestRepo
	claimed chan struct{}
	once    sync.Once
}

func (r *idleReportRepo) Claim(context.Context) (*DesktopReportTask, error) {
	r.once.Do(func() { close(r.claimed) })
	return nil, nil
}

func TestDesktopReportStopCancelsIdleBackoff(t *testing.T) {
	s, base, ai, _ := reportFixture()
	repo := &idleReportRepo{reportTestRepo: base, claimed: make(chan struct{})}
	s.repo = repo
	s.Start(ai)
	defer s.Stop()
	select {
	case <-repo.claimed:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown waited for the idle timer")
	}
}

type boundedPeriodScanRepo struct {
	*periodTestRepo
	queued map[string]int
}

func (r *boundedPeriodScanRepo) DuePeriodOrganizations(context.Context, string, time.Time) ([]string, error) {
	return make([]string, DesktopReportScanBatchSize+3), nil
}

func (r *boundedPeriodScanRepo) EnqueuePeriod(_ context.Context, _ int64, kind string, _, _ time.Time, _ string) (*DesktopPeriodTask, error) {
	r.queued[kind]++
	return &DesktopPeriodTask{}, nil
}

func TestDesktopReportPeriodScansBoundBothQueues(t *testing.T) {
	s, base, _, _ := reportFixture()
	repo := &boundedPeriodScanRepo{periodTestRepo: &periodTestRepo{reportTestRepo: base}, queued: map[string]int{}}
	s.repo = repo
	s.enqueueAutomaticPeriods(context.Background())
	require.Equal(t, DesktopReportScanBatchSize, repo.queued["week"])
	require.Equal(t, DesktopReportScanBatchSize, repo.queued["month"])
}
