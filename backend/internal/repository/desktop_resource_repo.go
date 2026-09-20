package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresource"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"golang.org/x/mod/semver"
)

type desktopResourceRepository struct{ db *sql.DB }

func NewDesktopResourceRepository(db *sql.DB) service.DesktopResourceRepository {
	return &desktopResourceRepository{db: db}
}

const resourceArtifactsSQL = `(SELECT COALESCE(jsonb_agg(jsonb_build_object(
    'platform', a.platform, 'sha256', a.sha256, 'sizeBytes', a.size_bytes, 'manifest', a.manifest,
    'createdBy', a.created_by, 'createdAt', a.created_at
    ) ORDER BY a.platform), '[]'::jsonb) FROM desktop_resource_artifacts a
    WHERE a.resource_id = v.resource_id AND a.version = v.version)`

const resourceSelectSQL = `SELECT r.id, r.resource_key, r.kind, v.name, v.description, v.source_url,
    r.current_version, r.status, r.status_reason, r.created_by, r.updated_by, r.created_at, r.updated_at, ` + resourceArtifactsSQL + `
    FROM desktop_resources r JOIN desktop_resource_versions v ON v.resource_id = r.id AND v.version = r.current_version`

func scanDesktopResource(row interface{ Scan(...any) error }) (*service.DesktopResourceRecord, error) {
	var item service.DesktopResourceRecord
	var artifacts []byte
	err := row.Scan(&item.ID, &item.Key, &item.Kind, &item.Name, &item.Description, &item.SourceURL, &item.Version,
		&item.Status, &item.StatusReason, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &artifacts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrResourceNotFound
	}
	if err != nil {
		return nil, err
	}
	item.Scope = "public"
	if err := json.Unmarshal(artifacts, &item.Artifacts); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *desktopResourceRepository) Publish(ctx context.Context, pkg desktopresource.Package, artifact service.DesktopResourceArtifact, actor int64) (*service.DesktopResourceRecord, error) {
	m := pkg.Manifest
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	id := "res_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	// ON CONFLICT waits for a concurrent first publication before taking the row lock below.
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_resources
        (id, resource_key, kind, current_version, created_by, updated_by) VALUES ($1,$2,$3,$4,$5,$5)
        ON CONFLICT (resource_key) DO NOTHING`, id, m.Key, m.Kind, m.Version, actor)
	if err != nil {
		return nil, err
	}
	var current, kind string
	err = tx.QueryRowContext(ctx, `SELECT id, current_version, kind FROM desktop_resources WHERE resource_key=$1 FOR UPDATE`, m.Key).Scan(&id, &current, &kind)
	if err != nil {
		return nil, err
	}
	if kind != m.Kind || semver.Compare("v"+m.Version, "v"+current) < 0 {
		return nil, service.ErrResourceConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_resource_versions
        (resource_id, version, name, description, source_url, created_by) VALUES ($1,$2,$3,$4,$5,$6)
        ON CONFLICT (resource_id, version) DO NOTHING`, id, m.Version, m.Name, m.Description, m.SourceURL, actor)
	if err != nil {
		return nil, err
	}
	var name, description, sourceURL string
	err = tx.QueryRowContext(ctx, `SELECT name, description, source_url FROM desktop_resource_versions WHERE resource_id=$1 AND version=$2`, id, m.Version).Scan(&name, &description, &sourceURL)
	if err != nil {
		return nil, err
	}
	if name != m.Name || description != m.Description || sourceURL != m.SourceURL {
		return nil, service.ErrResourceConflict
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_resource_artifacts
        (resource_id, version, platform, sha256, size_bytes, manifest, object_key, created_by)
        VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8)`, id, m.Version, m.Platform, pkg.SHA256, pkg.SizeBytes, raw, artifact.ObjectKey, actor)
	if isUniqueConstraintViolation(err) {
		return nil, service.ErrResourceConflict
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE desktop_resources SET current_version=$2, updated_by=$3, updated_at=clock_timestamp() WHERE id=$1`, id, m.Version, actor)
	if err != nil {
		return nil, err
	}
	result, err := scanDesktopResource(tx.QueryRowContext(ctx, resourceSelectSQL+` WHERE r.id=$1`, id))
	if err != nil {
		return nil, err
	}
	// The upload receipt identifies exactly this immutable artifact even if another upload follows.
	for _, item := range result.Artifacts {
		if item.Platform == m.Platform {
			result.Artifacts = []service.DesktopResourceArtifact{item}
			break
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *desktopResourceRepository) List(ctx context.Context, f service.DesktopResourceFilter) ([]service.DesktopResourceRecord, int64, error) {
	where := ` WHERE ($1='' OR r.kind=$1) AND ($2='' OR r.status=$2) AND ($3='' OR r.resource_key=$3)
        AND ($4='' OR v.name ILIKE '%' || $4 || '%' OR v.description ILIKE '%' || $4 || '%')`
	args := []any{f.Kind, f.Status, f.Key, f.Search}
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM desktop_resources r JOIN desktop_resource_versions v ON v.resource_id=r.id AND v.version=r.current_version`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, resourceSelectSQL+where+` ORDER BY r.updated_at DESC, r.id ASC LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.DesktopResourceRecord, 0)
	for rows.Next() {
		item, err := scanDesktopResource(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *desktopResourceRepository) Get(ctx context.Context, id string, public bool) (*service.DesktopResourceRecord, error) {
	return scanDesktopResource(r.db.QueryRowContext(ctx, resourceSelectSQL+` WHERE r.id=$1 AND (NOT $2 OR r.status='active')`, id, public))
}

func (r *desktopResourceRepository) Versions(ctx context.Context, id string, limit, offset int) ([]service.DesktopResourceVersion, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM desktop_resource_versions WHERE resource_id=$1`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT v.version, v.name, v.description, v.source_url, v.created_by, v.created_at, `+resourceArtifactsSQL+`
        FROM desktop_resource_versions v WHERE v.resource_id=$1 ORDER BY string_to_array(v.version, '.')::int[] DESC LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.DesktopResourceVersion, 0)
	for rows.Next() {
		var item service.DesktopResourceVersion
		var raw []byte
		if err := rows.Scan(&item.Version, &item.Name, &item.Description, &item.SourceURL, &item.CreatedBy, &item.CreatedAt, &raw); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(raw, &item.Artifacts); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *desktopResourceRepository) Artifact(ctx context.Context, id, version, platform string) (*service.DesktopResourceArtifact, error) {
	var artifact service.DesktopResourceArtifact
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT a.platform, a.sha256, a.size_bytes, a.manifest, a.object_key FROM desktop_resource_artifacts a
        JOIN desktop_resources r ON r.id=a.resource_id WHERE r.id=$1 AND r.status='active' AND a.version=$2 AND a.platform=$3`, id, version, platform).
		Scan(&artifact.Platform, &artifact.SHA256, &artifact.SizeBytes, &raw, &artifact.ObjectKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrResourceNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &artifact.Manifest); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *desktopResourceRepository) SetStatus(ctx context.Context, id, status, reason string, actor int64) (*service.DesktopResourceRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE desktop_resources SET status=$2, status_reason=$3, updated_by=$4, updated_at=clock_timestamp() WHERE id=$1`, id, status, reason, actor)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, service.ErrResourceNotFound
	}
	item, err := scanDesktopResource(tx.QueryRowContext(ctx, resourceSelectSQL+` WHERE r.id=$1`, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}
