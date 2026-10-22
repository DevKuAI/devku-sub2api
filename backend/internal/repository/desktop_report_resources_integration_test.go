//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopReportAutomaticScansAreBoundedAndDrainWithoutAggregatingSources(t *testing.T) {
	ctx := context.Background()
	date := "1998-01-04"
	start, _, err := reportBounds(date)
	require.NoError(t, err)
	all := []*desktopRepositoryFixture{}
	expected := []string{}
	for i := 0; i < service.DesktopReportScanBatchSize+2; i++ {
		f := newDesktopRepositoryFixture(t, fmt.Sprintf("boundedreport%d", i), 1, true)
		all = append(all, f)
		_, err = integrationDB.Exec(`UPDATE desktop_organizations SET conversation_summary_enabled=TRUE,weekly_summary_enabled=TRUE,monthly_summary_enabled=TRUE,summary_enabled_at=$2,analysis_model='model' WHERE id=$1`, f.organization.ID, start.AddDate(0, 0, -1))
		require.NoError(t, err)
		p2ReportRepository(t, f)
		t.Cleanup(func() {
			_, _ = integrationDB.Exec(`DELETE FROM desktop_conversation_records WHERE organization_id=$1`, f.organization.ID)
		})
		if i == 0 {
			continue
		}
		member := f.createMember(t, fmt.Sprint(i), int64(i))
		_, err = NewDesktopConversationRepository(integrationEntClient).Create(ctx, f.organization.ID, member.ID, &service.DesktopConversationInput{
			RecordID: uuid.NewString(), InstallationID: uuid.NewString(), Client: "workbuddy", SessionID: "bounded",
			StartedAt: start, StoppedAt: start.Add(time.Minute), CaptureStatus: "response_missing",
			Prompts: []service.DesktopTextSegment{{Text: "evidence"}},
		})
		require.NoError(t, err)
		if i == 1 {
			_, err = integrationDB.Exec(`UPDATE desktop_members SET deleted_at=NOW() WHERE id=$1`, member.ID)
			require.NoError(t, err)
		}
		expected = append(expected, f.organization.PublicID)
	}
	repo := NewDesktopReportRepository(integrationDB, integrationEntClient)
	ids, err := repo.DueOrganizations(ctx, date)
	require.NoError(t, err)
	require.Equal(t, expected[:service.DesktopReportScanBatchSize], ids)
	require.NotContains(t, ids, all[0].organization.PublicID, "organizations without source records must not load member metadata")
	for _, f := range all[1 : service.DesktopReportScanBatchSize+1] {
		_, err = repo.Enqueue(ctx, f.organization.ID, date, "automatic")
		require.NoError(t, err)
	}
	ids, err = repo.DueOrganizations(ctx, date)
	require.NoError(t, err)
	require.Equal(t, expected[service.DesktopReportScanBatchSize:], ids)
	_, err = repo.Enqueue(ctx, all[len(all)-1].organization.ID, date, "automatic")
	require.NoError(t, err)
	ids, err = repo.DueOrganizations(ctx, date)
	require.NoError(t, err)
	require.Empty(t, ids)
	_, err = repo.DueOrganizations(ctx, "invalid")
	require.Error(t, err)
	periods := repo.(service.DesktopPeriodRepository)
	for _, kind := range []string{"week", "month"} {
		ids, err = periods.DuePeriodOrganizations(ctx, kind, start)
		require.NoError(t, err)
		require.Len(t, ids, service.DesktopReportScanBatchSize)
	}
}
