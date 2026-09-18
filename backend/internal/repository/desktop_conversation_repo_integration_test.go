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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopConversationPersistenceIsolationAndHistory(t *testing.T) {
	ctx := context.Background()
	one := newDesktopRepositoryFixture(t, "convone", 10)
	two := newDesktopRepositoryFixture(t, "convtwo", 10)
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
