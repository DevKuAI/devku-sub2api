package repository

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newDesktopUpdateRepositoryForTest(t *testing.T) service.DesktopUpdateRepository {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec(`
		CREATE TABLE desktop_update_releases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			version TEXT NOT NULL UNIQUE,
			notes TEXT NOT NULL DEFAULT '',
			artifacts BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			created_by INTEGER,
			updated_by INTEGER,
			published_by INTEGER,
			withdrawn_by INTEGER,
			published_at DATETIME,
			withdrawn_at DATETIME,
			withdrawal_reason TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	driver := entsql.OpenDB(dialect.SQLite, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return NewDesktopUpdateRepository(client)
}

func repositoryDesktopUpdateArtifacts(version string) service.DesktopUpdateArtifacts {
	return service.DesktopUpdateArtifacts{
		service.DesktopUpdateDarwinARM64: repositoryDesktopUpdateArtifact(version, "arm.app.tar.gz", "arm-signature"),
		service.DesktopUpdateDarwinX64:   repositoryDesktopUpdateArtifact(version, "x64.app.tar.gz", "x64-signature"),
		service.DesktopUpdateWindowsX64:  repositoryDesktopUpdateArtifact(version, "windows.nsis.zip", "windows-signature"),
	}
}

func repositoryDesktopUpdateArtifact(version, fileName, signature string) service.DesktopUpdateArtifact {
	key := "desktop-updates/" + version + "/" + fileName
	return service.DesktopUpdateArtifact{
		URL: "https://example.com/" + key, Signature: signature, ObjectKey: key,
		FileName: fileName, Size: 1234, SHA256: strings.Repeat("a", 64),
	}
}

func createDesktopUpdateDraft(t *testing.T, repo service.DesktopUpdateRepository, publicID, version string) {
	t.Helper()
	require.NoError(t, repo.Create(context.Background(), &service.DesktopUpdateRelease{
		PublicID: publicID, Version: version, Notes: "Notes", Artifacts: repositoryDesktopUpdateArtifacts(version),
		Status: service.DesktopUpdateStatusDraft,
	}))
}

func TestDesktopUpdateRepositoryEnforcesLifecycleAndVersionOrdering(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	publishedAt := time.Now().UTC()

	createDesktopUpdateDraft(t, repo, "upd_100", "1.0.0")
	release, err := repo.UpdateDraft(ctx, "upd_100", service.DesktopUpdateDraftInput{
		Version: "1.0.1", Notes: "Updated notes", Artifacts: repositoryDesktopUpdateArtifacts("1.0.1"), ActorID: 7,
	})
	require.NoError(t, err)
	require.Equal(t, "1.0.1", release.Version)

	_, err = repo.Publish(ctx, "upd_100", 7, publishedAt)
	require.NoError(t, err)
	_, err = repo.UpdateDraft(ctx, "upd_100", service.DesktopUpdateDraftInput{
		Version: "1.0.2", Artifacts: repositoryDesktopUpdateArtifacts("1.0.2"),
	})
	require.ErrorIs(t, err, service.ErrDesktopUpdateInvalidState)

	createDesktopUpdateDraft(t, repo, "upd_older", "1.0.0")
	_, err = repo.Publish(ctx, "upd_older", 7, publishedAt.Add(time.Minute))
	require.ErrorIs(t, err, service.ErrDesktopUpdateNotNewer)

	withdrawn, err := repo.Withdraw(ctx, "upd_100", 7, "Rollback", publishedAt.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, service.DesktopUpdateStatusWithdrawn, withdrawn.Status)
	_, err = repo.Withdraw(ctx, "upd_100", 7, "Again", publishedAt.Add(3*time.Minute))
	require.ErrorIs(t, err, service.ErrDesktopUpdateInvalidState)
}

func TestDesktopUpdateRepositorySerializesConcurrentPublish(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	createDesktopUpdateDraft(t, repo, "upd_concurrent", "2.0.0")

	start := make(chan struct{})
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Publish(context.Background(), "upd_concurrent", 7, time.Now().UTC())
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errors)

	var succeeded, rejected int
	for err := range errors {
		switch {
		case err == nil:
			succeeded++
		case service.ErrDesktopUpdateInvalidState.Is(err):
			rejected++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, rejected)
}

func TestDesktopUpdateRepositoryWithdrawEnablesPublicFallback(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDesktopUpdateDraft(t, repo, "upd_previous", "1.1.0")
	createDesktopUpdateDraft(t, repo, "upd_latest", "1.2.0")
	_, err := repo.Publish(ctx, "upd_previous", 7, now)
	require.NoError(t, err)
	_, err = repo.Publish(ctx, "upd_latest", 7, now.Add(time.Minute))
	require.NoError(t, err)
	_, err = repo.Withdraw(ctx, "upd_latest", 7, "Rollback", now.Add(2*time.Minute))
	require.NoError(t, err)

	result, available, err := service.NewDesktopUpdateService(repo).Check(ctx, "darwin", "aarch64", "1.0.0")
	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, "1.1.0", result.Version)
}
