package bi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

func (f *analysisFrame) readableKnowledge(id string) (*record, error) {
	if !slices.Contains(f.state.Scope.Capabilities, "knowledge:read") {
		return nil, ErrNotFound
	}
	allowed, err := f.visible("knowledge", id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound
	}
	r := f.records["knowledge"][id]
	if r == nil || !f.contentMatches(r) {
		return nil, ErrNotFound
	}
	return r, nil
}

func (f *analysisFrame) readableVersion(knowledgeID, versionID string) (*record, error) {
	r := f.records["knowledge_version"][versionID]
	if r == nil || (knowledgeID != "" && r.str("knowledge_id") != knowledgeID) {
		return nil, ErrNotFound
	}
	if _, err := f.readableKnowledge(r.str("knowledge_id")); err != nil {
		return nil, err
	}
	allowed, err := f.visible("knowledge_version", versionID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound
	}
	return r, nil
}

func (f *analysisFrame) readableCase(id string) (*record, error) {
	if !slices.Contains(f.state.Scope.Capabilities, "knowledge:read") {
		return nil, ErrNotFound
	}
	allowed, err := f.visible("case", id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound
	}
	r := f.records["case"][id]
	if r == nil || !f.state.teamAllowed(r.str("team_id")) {
		return nil, ErrNotFound
	}
	filters := f.state.Context.Filters
	if filters.ApplicationID != "" && r.str("application_id") != filters.ApplicationID {
		return nil, ErrNotFound
	}
	if filters.SceneID != "" && r.str("scene_id") != filters.SceneID {
		return nil, ErrNotFound
	}
	return r, nil
}

func (f *analysisFrame) sourceParent(kind, id string) (*record, error) {
	switch kind {
	case "knowledge_version":
		return f.readableVersion("", id)
	case "case":
		return f.readableCase(id)
	default:
		return nil, invalid("parent_kind", "Expected knowledge_version or case")
	}
}

func (f *analysisFrame) sourceDetail(kind, parentID, sourceID string) (SourceDetail, error) {
	return f.sourceRecord(kind, parentID, sourceID, true)
}

func (f *analysisFrame) sourceRecord(kind, parentID, sourceID string, body bool) (SourceDetail, error) {
	if !slices.Contains(f.state.Scope.Capabilities, "sources:read") {
		return SourceDetail{}, ErrNotFound
	}
	parent, err := f.sourceParent(kind, parentID)
	if err != nil {
		return SourceDetail{}, err
	}
	if !slices.Contains(parent.strings("source_ids"), sourceID) {
		return SourceDetail{}, ErrNotFound
	}
	allowed, err := f.visible("source", sourceID)
	if err != nil {
		return SourceDetail{}, err
	}
	if !allowed {
		return SourceDetail{}, ErrNotFound
	}
	var raw []byte
	err = f.service.db.QueryRowContext(f.ctx, `SELECT CASE WHEN $6 THEN v.payload ELSE v.payload-'body' END FROM bi_source_version_links l JOIN bi_entity_versions v
		ON v.organization_id=l.organization_id AND v.kind='source' AND v.entity_id=l.source_id AND v.data_revision=l.source_data_revision
		JOIN bi_data_revisions d ON d.id=v.data_revision WHERE l.organization_id=$1 AND l.parent_kind=$2 AND l.parent_id=$3
		AND l.parent_data_revision=$4 AND l.source_id=$5 AND v.payload->>'version_id'=l.source_version_id AND NOT v.tombstone AND d.status='published'
		ORDER BY v.revision DESC LIMIT 1`, f.state.Context.OrganizationID, kind, parentID, parent.DataRevision, sourceID, body).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceDetail{}, ErrNotFound
	}
	if err != nil {
		return SourceDetail{}, err
	}
	var value SourceDetail
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	value.Availability = "available"
	return value, nil
}

func (f *analysisFrame) sourceLinks(parent *record) ([]SourceLink, error) {
	result := []SourceLink{}
	seen := map[string]bool{}
	for _, id := range parent.strings("source_ids") {
		if seen[id] {
			continue
		}
		seen[id] = true
		source, err := f.sourceRecord(parent.Kind, parent.ID, id, false)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) {
			result = append(result, SourceLink{Label: "来源不可访问", Availability: "restricted"})
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, SourceLink{ID: ptr(id), Label: source.Title, Availability: "available"})
	}
	return result, nil
}

func (f *analysisFrame) readableEvaluation(id string) (*record, error) {
	if !slices.Contains(f.state.Scope.Capabilities, "evaluations:read") {
		return nil, ErrNotFound
	}
	allowed, err := f.visible("evaluation", id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound
	}
	r := f.records["evaluation"][id]
	if r == nil {
		return nil, ErrNotFound
	}
	if err := f.checkSelection(analysisSelection{ApplicationID: r.str("application_id")}); err != nil {
		return nil, err
	}
	for _, field := range []string{"baseline_application_version_id", "candidate_application_version_id"} {
		allowed, err := f.visible("application_version", r.str(field))
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrNotFound
		}
	}
	return r, nil
}

func (f *analysisFrame) evidenceReadable(parent *record) (bool, error) {
	links, err := f.sourceLinks(parent)
	if err != nil {
		return false, err
	}
	for _, link := range links {
		if link.Availability != "available" {
			return false, nil
		}
	}
	if parent.Kind == "case" {
		for _, id := range parent.strings("knowledge_version_ids") {
			version, err := f.readableVersion("", id)
			if errors.Is(err, ErrNotFound) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			allowed, err := f.evidenceReadable(version)
			if err != nil || !allowed {
				return false, err
			}
		}
	}
	return true, nil
}
