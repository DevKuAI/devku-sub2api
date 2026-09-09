package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestDesktopResourcePersistenceAndTenantIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DESKTOP_RESOURCE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DESKTOP_RESOURCE_DSN to an isolated local test database")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE SCHEMA desktop_resource_test;SET search_path TO desktop_resource_test;CREATE TABLE desktop_organizations(id BIGINT PRIMARY KEY);CREATE TABLE desktop_members(id BIGINT PRIMARY KEY);INSERT INTO desktop_organizations VALUES(11),(22);INSERT INTO desktop_members VALUES(7)`)
	require.NoError(t, err)
	defer db.Exec(`DROP SCHEMA desktop_resource_test CASCADE`)
	migration, err := os.ReadFile("../../migrations/237_desktop_resources.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	repo := NewDesktopResourceRepository(db)
	ctx := context.Background()
	org := int64(11)
	member := int64(7)
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	m := service.DesktopResourceManifest{SchemaVersion: 1, Key: "sample", Kind: "mcp", Name: "Sample", Version: "0.1.0", Platform: "darwin-arm64"}
	public, err := repo.Publish(ctx, nil, nil, m, hash, []byte("public"))
	require.NoError(t, err)
	private, err := repo.Publish(ctx, &org, &member, m, hash, []byte("private"))
	require.NoError(t, err)
	require.NotEqual(t, public.ID, private.ID)
	for _, id := range []int64{11, 22} {
		rows, err := repo.List(ctx, id, service.DesktopResourceQuery{Scope: "public", Limit: 20})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, public.ID, rows[0].ID)
	}
	rows, err := repo.List(ctx, 22, service.DesktopResourceQuery{Scope: "enterprise", Limit: 20})
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = repo.List(ctx, 22, service.DesktopResourceQuery{Scope: "visible", ID: private.ID, Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, _, err = repo.GetArtifact(ctx, 22, private.ID, "0.1.0", "darwin-arm64")
	require.ErrorIs(t, err, service.ErrDesktopResourceNotFound)
	_, data, err := repo.GetArtifact(ctx, 11, private.ID, "0.1.0", "darwin-arm64")
	require.NoError(t, err)
	require.Equal(t, []byte("private"), data)
	_, err = repo.Publish(ctx, nil, nil, m, hash, []byte("overwrite"))
	require.ErrorIs(t, err, service.ErrDesktopResourceConflict)
	m.Platform = "windows-x64"
	_, err = repo.Publish(ctx, nil, nil, m, hash, []byte("windows"))
	require.NoError(t, err)
	rows, err = repo.List(ctx, 22, service.DesktopResourceQuery{Scope: "public", Limit: 20})
	require.NoError(t, err)
	require.Len(t, rows[0].Artifacts, 2)
	m.Version = "0.2.0"
	_, err = repo.Publish(ctx, nil, nil, m, hash, []byte("update"))
	require.NoError(t, err)
	m.Version = "0.1.0"
	_, err = repo.Publish(ctx, nil, nil, m, hash, []byte("downgrade"))
	require.ErrorIs(t, err, service.ErrDesktopResourceConflict)
	fresh := NewDesktopResourceRepository(db)
	a, data, err := fresh.GetArtifact(ctx, 22, public.ID, "0.2.0", "windows-x64")
	require.NoError(t, err)
	require.Equal(t, []byte("update"), data)
	require.Equal(t, "0.2.0", a.Manifest.Version)
}
