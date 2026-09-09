package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"golang.org/x/mod/semver"
)

type desktopResourceRepository struct{ db *sql.DB }

func NewDesktopResourceRepository(db *sql.DB) service.DesktopResourceRepository {
	return &desktopResourceRepository{db: db}
}
func (r *desktopResourceRepository) List(ctx context.Context, organizationID int64, q service.DesktopResourceQuery) ([]service.DesktopResource, error) {
	rows, err := r.db.QueryContext(ctx, `WITH page AS (
 SELECT * FROM desktop_resources WHERE
 (($2='public' AND organization_id IS NULL) OR ($2='enterprise' AND organization_id=$1) OR ($2='visible' AND (organization_id IS NULL OR organization_id=$1)))
 AND ($7='' OR id=$7)
 AND ($3='' OR kind=$3) AND ($4='' OR name ILIKE '%' || $4 || '%' OR description ILIKE '%' || $4 || '%')
 ORDER BY updated_at DESC,id LIMIT $5 OFFSET $6)
 SELECT p.id,p.resource_key,p.kind,p.name,p.description,p.source_url,p.current_version,p.organization_id IS NULL,
 a.platform,a.sha256,a.size_bytes,a.manifest
 FROM page p JOIN desktop_resource_artifacts a ON a.resource_id=p.id AND a.version=p.current_version
 ORDER BY p.updated_at DESC,p.id,a.platform`, organizationID, q.Scope, q.Kind, q.Search, q.Limit, q.Offset, q.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.DesktopResource{}
	indices := map[string]int{}
	for rows.Next() {
		var resource service.DesktopResource
		var artifact service.DesktopResourceArtifact
		var manifest []byte
		var isPublic bool
		if err := rows.Scan(&resource.ID, &resource.Key, &resource.Kind, &resource.Name, &resource.Description, &resource.SourceURL, &resource.Version, &isPublic, &artifact.Platform, &artifact.SHA256, &artifact.SizeBytes, &manifest); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(manifest, &artifact.Manifest); err != nil {
			return nil, err
		}
		index, ok := indices[resource.ID]
		if !ok {
			resource.Scope = "enterprise"
			if isPublic {
				resource.Scope = "public"
			}
			resource.Artifacts = []service.DesktopResourceArtifact{}
			result = append(result, resource)
			index = len(result) - 1
			indices[resource.ID] = index
		}
		result[index].Artifacts = append(result[index].Artifacts, artifact)
	}
	return result, rows.Err()
}
func (r *desktopResourceRepository) GetArtifact(ctx context.Context, organizationID int64, id, version, platform string) (*service.DesktopResourceArtifact, []byte, error) {
	var artifact service.DesktopResourceArtifact
	var data, manifest []byte
	err := r.db.QueryRowContext(ctx, `SELECT a.platform,a.sha256,a.size_bytes,a.manifest,a.artifact_data
 FROM desktop_resource_artifacts a JOIN desktop_resources r ON r.id=a.resource_id
 WHERE r.id=$1 AND a.version=$2 AND a.platform=$3 AND (r.organization_id IS NULL OR r.organization_id=$4)`, id, version, platform, organizationID).Scan(&artifact.Platform, &artifact.SHA256, &artifact.SizeBytes, &manifest, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, service.ErrDesktopResourceNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if err = json.Unmarshal(manifest, &artifact.Manifest); err != nil {
		return nil, nil, err
	}
	return &artifact, data, nil
}
func (r *desktopResourceRepository) Publish(ctx context.Context, organizationID, memberID *int64, manifest service.DesktopResourceManifest, checksum string, data []byte) (*service.DesktopResource, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	scope := "public"
	if organizationID != nil {
		scope = strconv.FormatInt(*organizationID, 10)
	}
	// Serialize all platforms and versions of the same resource across server instances.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope+":"+manifest.Key); err != nil {
		return nil, err
	}
	id := "res_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_resources(id,resource_key,organization_id,created_by,kind,name,description,source_url,current_version)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, id, manifest.Key, organizationID, memberID, manifest.Kind, manifest.Name, manifest.Description, manifest.SourceURL, manifest.Version)
	if err != nil {
		return nil, err
	}
	var currentVersion, kind, name, description, sourceURL string
	err = tx.QueryRowContext(ctx, `SELECT id,current_version,kind,name,description,source_url FROM desktop_resources WHERE organization_id IS NOT DISTINCT FROM $1 AND resource_key=$2 FOR UPDATE`, organizationID, manifest.Key).Scan(&id, &currentVersion, &kind, &name, &description, &sourceURL)
	if err != nil {
		return nil, err
	}
	if kind != manifest.Kind || semver.Compare("v"+manifest.Version, "v"+currentVersion) < 0 || (currentVersion == manifest.Version && (name != manifest.Name || description != manifest.Description || sourceURL != manifest.SourceURL)) {
		return nil, service.ErrDesktopResourceConflict
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO desktop_resource_artifacts(resource_id,version,platform,manifest,sha256,size_bytes,artifact_data)
 VALUES($1,$2,$3,$4::jsonb,$5,$6,$7) ON CONFLICT DO NOTHING`, id, manifest.Version, manifest.Platform, encoded, checksum, len(data), data)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, service.ErrDesktopResourceConflict
	}
	_, err = tx.ExecContext(ctx, `UPDATE desktop_resources SET name=$2,description=$3,source_url=$4,current_version=$5,updated_at=NOW() WHERE id=$1`, id, manifest.Name, manifest.Description, manifest.SourceURL, manifest.Version)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	publicScope := "public"
	if organizationID != nil {
		publicScope = "enterprise"
	}
	return &service.DesktopResource{ID: id, Key: manifest.Key, Kind: kind, Name: manifest.Name, Description: manifest.Description, SourceURL: manifest.SourceURL, Version: manifest.Version, Scope: publicScope, Artifacts: []service.DesktopResourceArtifact{{Platform: manifest.Platform, SHA256: checksum, SizeBytes: int64(len(data)), Manifest: manifest}}}, nil
}
