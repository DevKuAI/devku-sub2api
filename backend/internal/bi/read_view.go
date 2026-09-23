package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	"github.com/lib/pq"
)

const metadataPayloadSQL = `(v.payload - ARRAY['body','problem','steps','result','samples']::text[]) ||
 CASE WHEN v.kind='evaluation' THEN jsonb_build_object('_sample_count',jsonb_array_length(v.payload->'samples'))
 WHEN v.kind='case' THEN jsonb_build_object('_snippet',left(v.payload->>'problem',240)) ELSE '{}'::jsonb END`

func recordAt(ctx context.Context, q queryer, org, kind, id string, revision int64) (*record, error) {
	return readRecordAt(ctx, q, org, kind, id, revision, false)
}
func recordHeaderAt(ctx context.Context, q queryer, org, kind, id string, revision int64) (*record, error) {
	return readRecordAt(ctx, q, org, kind, id, revision, true)
}

func readRecordAt(ctx context.Context, q queryer, org, kind, id string, revision int64, metadata bool) (*record, error) {
	r := &record{recordKey: recordKey{kind, id}}
	err := q.QueryRowContext(ctx, `SELECT v.revision::text,v.data_revision,CASE WHEN $5 THEN `+metadataPayloadSQL+` ELSE v.payload END,v.payload_hash,v.tombstone,e.source_id
		FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision JOIN bi_entities e
		ON e.organization_id=v.organization_id AND e.kind=v.kind AND e.id=v.entity_id
		WHERE v.organization_id=$1 AND v.kind=$2 AND v.entity_id=$3 AND d.status='published' AND ($4::bigint<0 OR v.data_revision<=$4)
		ORDER BY v.data_revision DESC,v.revision DESC LIMIT 1`, org, kind, id, revision, metadata).
		Scan(&r.Revision, &r.DataRevision, &r.Raw, &r.Hash, &r.Deleted, &r.SourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
		return nil, err
	}
	return r, nil
}

func recordsAt(ctx context.Context, q queryer, org string, kinds []string, revision int64) ([]*record, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT ON (v.kind,v.entity_id) v.kind,v.entity_id,v.revision::text,v.data_revision,`+metadataPayloadSQL+`,v.payload_hash,v.tombstone,e.source_id
		FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision JOIN bi_entities e
		ON e.organization_id=v.organization_id AND e.kind=v.kind AND e.id=v.entity_id
		WHERE v.organization_id=$1 AND v.kind=ANY($2) AND d.status='published' AND ($3::bigint<0 OR v.data_revision<=$3)
		ORDER BY v.kind,v.entity_id,v.data_revision DESC,v.revision DESC`, org, pq.Array(kinds), revision)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []*record{}
	for rows.Next() {
		r := new(record)
		if err := rows.Scan(&r.Kind, &r.ID, &r.Revision, &r.DataRevision, &r.Raw, &r.Hash, &r.Deleted, &r.SourceID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
			return nil, err
		}
		if !r.Deleted {
			result = append(result, r)
		}
	}
	return result, rows.Err()
}

func (state analysisState) teamAllowed(id string) bool {
	if !state.Scope.AllTeams && !slices.Contains(state.Scope.TeamIDs, id) {
		return false
	}
	return len(state.Context.Filters.TeamIDs) == 0 || slices.Contains(state.Context.Filters.TeamIDs, id)
}

func aclAllows(raw []byte, scope OrganizationScope, managerID string) bool {
	var acl ACL
	if json.Unmarshal(raw, &acl) != nil {
		return false
	}
	if acl.OrganizationReadable || slices.Contains(acl.ManagerIDs, managerID) {
		return true
	}
	for _, team := range acl.TeamIDs {
		if scope.AllTeams || slices.Contains(scope.TeamIDs, team) {
			return true
		}
	}
	return false
}

// visibleRecord checks both snapshot permission and current restrictions.
// Historical source bodies require the additional parent relationship check.
func visibleRecord(ctx context.Context, q queryer, state analysisState, kind, id string) (*record, error) {
	aclScope := state.Scope
	if len(state.Context.Filters.TeamIDs) > 0 {
		aclScope.AllTeams = false
		aclScope.TeamIDs = state.Context.Filters.TeamIDs
	}
	r, err := recordHeaderAt(ctx, q, state.Context.OrganizationID, kind, id, state.Revision)
	if err != nil {
		return nil, err
	}
	if r == nil || r.Deleted {
		return nil, ErrNotFound
	}
	if kind == "team" && !state.teamAllowed(id) {
		return nil, ErrNotFound
	}
	if acl, ok := r.Payload["acl"]; ok && !aclAllows(acl, aclScope, state.Principal.ManagerID) {
		return nil, ErrNotFound
	}
	current, err := recordHeaderAt(ctx, q, state.Context.OrganizationID, kind, id, -1)
	if err != nil {
		return nil, err
	}
	if current == nil || current.Deleted {
		return nil, ErrNotFound
	}
	if acl, ok := current.Payload["acl"]; ok && !aclAllows(acl, aclScope, state.Principal.ManagerID) {
		return nil, ErrNotFound
	}
	rows, err := q.QueryContext(ctx, `SELECT deny_all,acl FROM bi_acl_denials WHERE organization_id=$1 AND kind=$2 AND entity_id=$3`, state.Context.OrganizationID, kind, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var deny bool
		var acl []byte
		if err := rows.Scan(&deny, &acl); err != nil {
			return nil, err
		}
		if deny || !aclAllows(acl, aclScope, state.Principal.ManagerID) {
			return nil, ErrNotFound
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
