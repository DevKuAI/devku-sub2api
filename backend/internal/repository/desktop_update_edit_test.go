package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDesktopUpdateEditReturnsPublishedAndWithdrawnReleasesToDraft(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDesktopUpdateDraft(t, repo, "previous", "1.0.0")
	createDesktopUpdateDraft(t, repo, "current", "1.1.0")
	_, err := repo.Publish(ctx, "previous", 1, now)
	require.NoError(t, err)
	_, err = repo.Publish(ctx, "current", 1, now, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{7}})
	require.NoError(t, err)
	for _, status := range []string{"published", "withdrawn"} {
		t.Run(status, func(t *testing.T) {
			if status == "withdrawn" {
				_, err := repo.Withdraw(ctx, "current", 2, "test rollback", now)
				require.NoError(t, err)
			}
			artifacts := repositoryDesktopUpdateArtifacts("1.1.0")
			windows := artifacts[service.DesktopUpdateWindowsX64]
			windows.Signature = "replacement-signature"
			artifacts[service.DesktopUpdateWindowsX64] = windows
			release, err := repo.UpdateDraft(ctx, "current", service.DesktopUpdateDraftInput{
				Version: "1.1.0", Notes: "replacement", Artifacts: artifacts, ActorID: 3, ResetPublished: true,
			})
			require.NoError(t, err)
			require.Equal(t, "draft", release.Status)
			require.Nil(t, release.PublishedAt)
			require.Nil(t, release.PublishedBy)
			require.Nil(t, release.WithdrawnAt)
			require.Nil(t, release.WithdrawnBy)
			require.Nil(t, release.WithdrawalReason)
			require.Equal(t, []int64{7}, release.GatewayUserIDs)
			updates := service.NewDesktopUpdateService(repo)
			check, _, err := updates.Check(ctx, "windows", "x86_64", "0.9.0", 7)
			require.NoError(t, err)
			require.Equal(t, "1.0.0", check.Version)
			_, err = updates.Republish(ctx, "current", 3, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{7}})
			require.NoError(t, err)
			check, _, err = updates.Check(ctx, "windows", "x86_64", "0.9.0", 7)
			require.NoError(t, err)
			require.Equal(t, "1.1.0", check.Version)
			require.Equal(t, "replacement-signature", check.Signature)
			_, available, err := updates.Check(ctx, "windows", "x86_64", "1.1.0", 7)
			require.NoError(t, err)
			require.False(t, available, "installed same versions do not trigger another update")
		})
	}
}

func TestDesktopUpdateRepublishAcceptsSameAndOlderVersionsAndValidatesAudience(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i, version := range []string{"1.0.0", "2.0.0"} {
		createDesktopUpdateDraft(t, repo, version, version)
		_, err := repo.Publish(ctx, version, 1, now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	updates := service.NewDesktopUpdateService(repo)
	full, err := updates.Republish(ctx, "1.0.0", 2, service.DesktopUpdatePublishInput{})
	require.NoError(t, err)
	require.Equal(t, "published", full.Status)
	require.Equal(t, "1.0.0", full.Version)
	check, _, err := updates.Check(ctx, "windows", "x86_64", "0.9.0")
	require.NoError(t, err)
	require.Equal(t, "2.0.0", check.Version, "republishing an older version cannot cause a downgrade")
	_, err = repo.Withdraw(ctx, "1.0.0", 2, "retry", now)
	require.NoError(t, err)
	_, err = updates.Republish(ctx, "1.0.0", 2, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{999}})
	require.ErrorIs(t, err, service.ErrDesktopUpdateInvalidFields)
	unchanged, err := repo.Get(ctx, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "withdrawn", unchanged.Status)
	restored, err := updates.Republish(ctx, "1.0.0", 2, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{7}})
	require.NoError(t, err)
	require.Nil(t, restored.WithdrawnAt)
	require.Nil(t, restored.WithdrawalReason)
	require.Equal(t, full.Artifacts, restored.Artifacts)
}

func TestDesktopUpdateDeleteFallsBackAndReleasesVersionNumber(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDesktopUpdateDraft(t, repo, "previous", "1.0.0")
	createDesktopUpdateDraft(t, repo, "current", "1.1.0")
	_, err := repo.Publish(ctx, "previous", 1, now)
	require.NoError(t, err)
	_, err = repo.Publish(ctx, "current", 1, now)
	require.NoError(t, err)
	updates := service.NewDesktopUpdateService(repo)
	require.NoError(t, updates.Delete(ctx, " current "))
	_, err = repo.Get(ctx, "current")
	require.ErrorIs(t, err, service.ErrDesktopUpdateNotFound)
	check, _, err := updates.Check(ctx, "windows", "x86_64", "0.9.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", check.Version)
	createDesktopUpdateDraft(t, repo, "replacement", "1.1.0")
	require.NoError(t, updates.Delete(ctx, "replacement"))
	_, err = repo.Withdraw(ctx, "previous", 1, "test", now)
	require.NoError(t, err)
	require.NoError(t, updates.Delete(ctx, "previous"))
	require.ErrorIs(t, updates.Delete(ctx, "previous"), service.ErrDesktopUpdateNotFound)
}
