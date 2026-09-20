//go:build integration

package repository

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresource"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func resourcePublication(key, version, platform string) (desktopresource.Package, service.DesktopResourceArtifact) {
	m := desktopresource.Manifest{SchemaVersion: 1, Key: key, Kind: "mcp", Name: "Resource", Version: version, Platform: platform, Entry: "bin/server", Targets: []string{"chatgpt_codex"}, Files: map[string]string{"bin/server": strings.Repeat("a", 64)}}
	p := desktopresource.Package{Manifest: m, SHA256: strings.Repeat("b", 64), SizeBytes: 100}
	a := service.DesktopResourceArtifact{Manifest: m, Platform: platform, SHA256: p.SHA256, SizeBytes: p.SizeBytes, ObjectKey: uuid.NewString()}
	return p, a
}

func TestDesktopResourceRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewDesktopResourceRepository(integrationDB)
	key := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	p, a := resourcePublication(key, "1.2.0", "darwin-arm64")
	first, err := repo.Publish(ctx, p, a, 42)
	require.NoError(t, err)
	require.Equal(t, "active", first.Status)
	require.Len(t, first.Artifacts, 1)
	_, err = repo.Publish(ctx, p, a, 42)
	require.ErrorIs(t, err, service.ErrResourceConflict)
	p, a = resourcePublication(key, "1.2.0", "windows-x64")
	second, err := repo.Publish(ctx, p, a, 43)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Len(t, second.Artifacts, 1)
	current, err := repo.Get(ctx, first.ID, true)
	require.NoError(t, err)
	require.Len(t, current.Artifacts, 2)
	p, a = resourcePublication(key, "1.2.0", "linux-x64")
	p.Manifest.Name = "mismatch"
	_, err = repo.Publish(ctx, p, a, 43)
	require.ErrorIs(t, err, service.ErrResourceConflict)
	p, a = resourcePublication(key, "1.10.0", "darwin-arm64")
	_, err = repo.Publish(ctx, p, a, 43)
	require.NoError(t, err)
	current, err = repo.Get(ctx, first.ID, true)
	require.NoError(t, err)
	require.Equal(t, "1.10.0", current.Version)
	require.Len(t, current.Artifacts, 1)
	_, err = repo.Artifact(ctx, first.ID, "1.2.0", "windows-x64")
	require.NoError(t, err)
	_, err = repo.Artifact(ctx, first.ID, "1.10.0", "windows-x64")
	require.ErrorIs(t, err, service.ErrResourceNotFound)
	p, a = resourcePublication(key, "1.2.0", "linux-x64")
	_, err = repo.Publish(ctx, p, a, 43)
	require.ErrorIs(t, err, service.ErrResourceConflict)
	_, err = repo.SetStatus(ctx, first.ID, "disabled", "investigate", 43)
	require.NoError(t, err)
	_, err = repo.Get(ctx, first.ID, true)
	require.ErrorIs(t, err, service.ErrResourceNotFound)
	_, err = repo.Artifact(ctx, first.ID, "1.2.0", "darwin-arm64")
	require.ErrorIs(t, err, service.ErrResourceNotFound)
	rows, total, err := repo.List(ctx, service.DesktopResourceFilter{Key: key, Status: "active", Limit: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
	p, a = resourcePublication(key, "2.0.0", "darwin-arm64")
	updated, err := repo.Publish(ctx, p, a, 44)
	require.NoError(t, err)
	require.Equal(t, "disabled", updated.Status)
	history, total, err := repo.Versions(ctx, first.ID, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, "1.10.0", history[1].Version)
	require.Equal(t, strings.Repeat("b", 64), history[2].Artifacts[0].SHA256)
	_, err = repo.SetStatus(ctx, first.ID, "active", "", 44)
	require.NoError(t, err)
	_, err = repo.Artifact(ctx, first.ID, "1.2.0", "darwin-arm64")
	require.NoError(t, err)
}

func TestDesktopResourceConcurrentPublication(t *testing.T) {
	ctx := context.Background()
	repo := NewDesktopResourceRepository(integrationDB)
	for _, duplicate := range []bool{false, true} {
		key := "race_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		start := make(chan struct{})
		errors := make(chan error, 2)
		var wg sync.WaitGroup
		for _, platform := range []string{"darwin-arm64", "windows-x64"} {
			if duplicate {
				platform = "darwin-arm64"
			}
			wg.Add(1)
			go func(platform string) {
				defer wg.Done()
				<-start
				p, a := resourcePublication(key, "1.0.0", platform)
				_, err := repo.Publish(ctx, p, a, 1)
				errors <- err
			}(platform)
		}
		close(start)
		wg.Wait()
		close(errors)
		conflicts := 0
		for err := range errors {
			if err != nil {
				require.ErrorIs(t, err, service.ErrResourceConflict)
				conflicts++
			}
		}
		if duplicate {
			require.Equal(t, 1, conflicts)
		} else {
			require.Zero(t, conflicts)
		}
		rows, total, err := repo.List(ctx, service.DesktopResourceFilter{Key: key, Limit: 20})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, rows[0].Artifacts, 2-conflicts)
	}
}

func TestDesktopResourceRollbackOnArtifactFailure(t *testing.T) {
	ctx := context.Background()
	repo := NewDesktopResourceRepository(integrationDB)
	key := "rollback_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	p, a := resourcePublication(key, "1.0.0", "darwin-arm64")
	p.SizeBytes = -1
	_, err := repo.Publish(ctx, p, a, 1)
	require.Error(t, err)
	items, total, err := repo.List(ctx, service.DesktopResourceFilter{Key: key, Limit: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
}
