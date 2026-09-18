//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopConversationPersistenceIsolationAndHistory(t *testing.T) {
	ctx := context.Background()
	one := newDesktopRepositoryFixture(t, "convone", 10)
	two := newDesktopRepositoryFixture(t, "convtwo", 10)
	enableDesktopConversationReporting(t, one)
	enableDesktopConversationReporting(t, two)
	member := one.createMember(t, "convone", 1)
	otherMember := two.createMember(t, "convtwo", 1)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_conversation_records WHERE organization_id IN ($1,$2)", one.organization.ID, two.organization.ID)
	})
	repo := NewDesktopConversationRepository(integrationEntClient)
	input := &service.DesktopConversationInput{
		SchemaVersion: 2, RecordID: uuid.NewString(), Client: "workbuddy", InstallationID: uuid.NewString(), SessionID: strings.Repeat("文", 512),
		StartedAt: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), StoppedAt: time.Now().UTC(),
		Prompts:  []service.DesktopTextSegment{{Text: "first\nline"}, {Text: "second", Truncated: true}},
		Response: &service.DesktopTextSegment{Text: "<script>never execute</script>"}, CaptureStatus: "captured",
	}
	receipt, err := repo.Create(ctx, one.organization.ID, member.ID, input)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), receipt.ReceivedAt, time.Minute)
	_, err = repo.Create(ctx, one.organization.ID, member.ID, input)
	require.ErrorIs(t, err, service.ErrDesktopConversationExists)
	_, err = repo.Get(ctx, two.organization.ID, input.RecordID)
	require.ErrorIs(t, err, service.ErrDesktopConversationNotFound)
	// Record IDs are unique within an organization, not globally.
	_, err = repo.Create(ctx, two.organization.ID, otherMember.ID, input)
	require.NoError(t, err)
	detail, err := repo.Get(ctx, one.organization.ID, input.RecordID)
	require.NoError(t, err)
	require.Equal(t, input.Prompts, detail.Prompts)
	require.Equal(t, input.Response, detail.Response)
	params := pagination.DefaultPagination()
	items, result, err := repo.List(ctx, one.organization.ID, params, service.DesktopConversationFilters{})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "first\\nline")
	require.NotContains(t, string(raw), "never execute")
	_, err = one.repo.DeleteMember(ctx, one.organization.PublicID, member.PublicID)
	require.NoError(t, err)
	detail, err = repo.Get(ctx, one.organization.ID, input.RecordID)
	require.NoError(t, err)
	require.True(t, detail.MemberDeleted)
	items, result, err = repo.List(ctx, one.organization.ID, params, service.DesktopConversationFilters{MemberSearch: "convone"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.True(t, items[0].MemberDeleted)
}

func TestDesktopConversationThreadTupleAndStablePages(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "convpage", 10)
	enableDesktopConversationReporting(t, fixture)
	member := fixture.createMember(t, "convpage", 1)
	other := fixture.createMember(t, "convother", 2)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_conversation_records WHERE organization_id = $1", fixture.organization.ID)
	})
	repo := NewDesktopConversationRepository(integrationEntClient)
	installation := uuid.NewString()
	input := &service.DesktopConversationInput{Client: "workbuddy", InstallationID: installation, SessionID: "same-session", StartedAt: time.Now().UTC(), StoppedAt: time.Now().UTC(), Prompts: []service.DesktopTextSegment{{Text: "prompt"}}, CaptureStatus: "response_missing"}
	var expected []string
	for i := range 5 {
		input.RecordID = uuid.NewString()
		memberID := member.ID
		if i == 2 {
			input.InstallationID = uuid.NewString()
		} else {
			input.InstallationID = installation
		}
		if i == 3 {
			input.Client = "chatgpt_codex"
		} else {
			input.Client = "workbuddy"
		}
		if i == 4 {
			memberID = other.ID
		}
		_, err := repo.Create(ctx, fixture.organization.ID, memberID, input)
		require.NoError(t, err)
		if i < 2 {
			expected = append(expected, input.RecordID)
		}
	}
	_, err := integrationDB.ExecContext(ctx, "UPDATE desktop_conversation_records SET received_at = '2026-09-18T00:00:00Z' WHERE organization_id = $1", fixture.organization.ID)
	require.NoError(t, err)
	filters := service.DesktopConversationFilters{MemberID: member.PublicID, Client: "workbuddy", InstallationID: installation, SourceSessionID: "same-session"}
	for i, id := range expected {
		items, page, err := repo.List(ctx, fixture.organization.ID, pagination.PaginationParams{Page: i + 1, PageSize: 1, SortOrder: "asc"}, filters)
		require.NoError(t, err)
		require.EqualValues(t, 2, page.Total)
		require.Len(t, items, 1)
		require.Equal(t, id, items[0].RecordID)
		detail, err := repo.Get(ctx, fixture.organization.ID, id)
		require.NoError(t, err)
		require.Nil(t, detail.Response)
	}
}

func TestDesktopConversationConcurrentDuplicateIsNotOverwritten(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "convrace", 10)
	enableDesktopConversationReporting(t, fixture)
	member := fixture.createMember(t, "convrace", 1)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_conversation_records WHERE organization_id = $1", fixture.organization.ID)
	})
	repo := NewDesktopConversationRepository(integrationEntClient)
	input := &service.DesktopConversationInput{RecordID: uuid.NewString(), Client: "workbuddy", InstallationID: uuid.NewString(), SessionID: "same", StartedAt: time.Now().UTC(), StoppedAt: time.Now().UTC(), Prompts: []service.DesktopTextSegment{{Text: "original"}}, CaptureStatus: "response_missing"}
	errors := make(chan error, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := repo.Create(ctx, fixture.organization.ID, member.ID, input)
			errors <- err
		}()
	}
	workers.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrDesktopConversationExists)
		}
	}
	require.Equal(t, 1, successes)
	input.Prompts[0].Text = "overwrite"
	_, err := repo.Create(ctx, fixture.organization.ID, member.ID, input)
	require.ErrorIs(t, err, service.ErrDesktopConversationExists)
	detail, err := repo.Get(ctx, fixture.organization.ID, input.RecordID)
	require.NoError(t, err)
	require.Equal(t, "original", detail.Prompts[0].Text)
}

func enableDesktopConversationReporting(t *testing.T, fixture *desktopRepositoryFixture) {
	t.Helper()
	enabled := true
	updated, _, err := fixture.repo.UpdateOrganization(context.Background(), fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{ConversationReportingEnabled: &enabled})
	require.NoError(t, err)
	require.True(t, updated.ConversationReportingEnabled)
}

func TestDesktopConversationReportingDefaultToggleAndRetention(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "convgate", 10)
	require.False(t, fixture.organization.ConversationReportingEnabled)
	member := fixture.createMember(t, "convgate", 1)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_conversation_records WHERE organization_id = $1", fixture.organization.ID)
	})
	repo := NewDesktopConversationRepository(integrationEntClient)
	input := &service.DesktopConversationInput{RecordID: uuid.NewString(), Client: "workbuddy", InstallationID: uuid.NewString(), SessionID: "source", StartedAt: time.Now().UTC(), StoppedAt: time.Now().UTC(), Prompts: []service.DesktopTextSegment{{Text: "prompt"}}, CaptureStatus: "response_missing"}
	_, err := repo.Create(ctx, fixture.organization.ID, member.ID, input)
	require.ErrorIs(t, err, service.ErrDesktopConversationReportingDisabled)
	enableDesktopConversationReporting(t, fixture)
	_, err = repo.Create(ctx, fixture.organization.ID, member.ID, input)
	require.NoError(t, err)
	disabled := false
	updated, _, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{ConversationReportingEnabled: &disabled})
	require.NoError(t, err)
	require.False(t, updated.ConversationReportingEnabled)
	_, err = repo.Get(ctx, fixture.organization.ID, input.RecordID)
	require.NoError(t, err)
	input.RecordID = uuid.NewString()
	_, err = repo.Create(ctx, fixture.organization.ID, member.ID, input)
	require.ErrorIs(t, err, service.ErrDesktopConversationReportingDisabled)
	_, _, err = fixture.repo.ScopedToGatewayUser(fixture.user.ID).UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{ConversationReportingEnabled: &disabled})
	require.ErrorIs(t, err, service.ErrDesktopValidation)
}

func TestDesktopConversationInsertRechecksConcurrentOptOut(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "convlock", 10)
	enableDesktopConversationReporting(t, fixture)
	member := fixture.createMember(t, "convlock", 1)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_conversation_records WHERE organization_id = $1", fixture.organization.ID)
	})
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, "UPDATE desktop_organizations SET conversation_reporting_enabled = FALSE WHERE id = $1", fixture.organization.ID)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, createErr := NewDesktopConversationRepository(integrationEntClient).Create(ctx, fixture.organization.ID, member.ID, &service.DesktopConversationInput{RecordID: uuid.NewString(), Client: "workbuddy", InstallationID: uuid.NewString(), SessionID: "source", StartedAt: time.Now().UTC(), StoppedAt: time.Now().UTC(), Prompts: []service.DesktopTextSegment{{Text: "prompt"}}, CaptureStatus: "response_missing"})
		result <- createErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE '%desktop_organizations%FOR SHARE%')").Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, tx.Commit())
	select {
	case err := <-result:
		require.ErrorIs(t, err, service.ErrDesktopConversationReportingDisabled)
	case <-time.After(5 * time.Second):
		t.Fatal("conversation write remained blocked after opt-out commit")
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM desktop_conversation_records WHERE organization_id = $1", fixture.organization.ID).Scan(&count))
	require.Zero(t, count)
}

func TestDesktopOrganizationCreatesReportingOptIn(t *testing.T) {
	ctx := context.Background()
	for _, enabled := range []bool{false, true} {
		fixture := newDesktopRepositoryFixture(t, "convnew", 10, enabled)
		require.Equal(t, enabled, fixture.organization.ConversationReportingEnabled)
		loaded, err := fixture.repo.GetOrganization(ctx, fixture.organization.PublicID)
		require.NoError(t, err)
		require.Equal(t, enabled, loaded.ConversationReportingEnabled)
		member := fixture.createMember(t, "convnew", 1)
		auth, err := fixture.repo.GetAuthorizedMember(ctx, member.PublicID)
		require.NoError(t, err)
		require.Equal(t, enabled, auth.Organization.ConversationReportingEnabled)
	}
}

func TestDesktopConversationReportingMigrationDefaultsExistingRowsOff(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, "CREATE TEMP TABLE desktop_organizations (id BIGINT PRIMARY KEY) ON COMMIT DROP; INSERT INTO desktop_organizations VALUES (1)")
	require.NoError(t, err)
	sql, err := migrations.FS.ReadFile("240_desktop_conversation_reporting_enabled.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sql))
	require.NoError(t, err)
	var enabled bool
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT conversation_reporting_enabled FROM desktop_organizations WHERE id=1").Scan(&enabled))
	require.False(t, enabled)
	_, err = tx.ExecContext(ctx, "UPDATE desktop_organizations SET conversation_reporting_enabled=TRUE WHERE id=1")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sql))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT conversation_reporting_enabled FROM desktop_organizations WHERE id=1").Scan(&enabled))
	require.True(t, enabled)
	require.NoError(t, tx.QueryRowContext(ctx, "INSERT INTO desktop_organizations(id) VALUES (2) RETURNING conversation_reporting_enabled").Scan(&enabled))
	require.False(t, enabled)
}
