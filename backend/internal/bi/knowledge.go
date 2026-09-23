package bi

import (
	"encoding/json"
	"errors"
	"sort"
	"time"
)

func knowledgeRow(r *record) (KnowledgeRow, error) {
	var row KnowledgeRow
	err := json.Unmarshal(r.Raw, &row)
	return row, err
}

func (f *analysisFrame) knowledgeVersion(knowledgeID, versionID string) (KnowledgeVersion, error) {
	_, err := f.readableVersion(knowledgeID, versionID)
	if err != nil {
		return KnowledgeVersion{}, err
	}
	r, err := recordAt(f.ctx, f.service.db, f.state.Context.OrganizationID, "knowledge_version", versionID, f.state.Revision)
	if err != nil {
		return KnowledgeVersion{}, err
	}
	if r == nil {
		return KnowledgeVersion{}, ErrNotFound
	}
	var result KnowledgeVersion
	if err := json.Unmarshal(r.Raw, &result); err != nil {
		return result, err
	}
	result.Sources, err = f.sourceLinks(r)
	return result, err
}

type knowledgeStatus struct {
	EffectiveAt time.Time
	Status      string
}

func (f *analysisFrame) expiredKnowledgeAt(id string, at time.Time) (bool, error) {
	if f.knowledgeHistory == nil {
		f.knowledgeHistory = map[string][]knowledgeStatus{}
	}
	history, exists := f.knowledgeHistory[id]
	if !exists {
		rows, err := f.service.db.QueryContext(f.ctx, `SELECT (v.payload->>'status_effective_at')::timestamptz,v.payload->>'status'
			FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision
			WHERE v.organization_id=$1 AND v.kind='knowledge' AND v.entity_id=$2 AND v.data_revision<=$3 AND NOT v.tombstone AND d.status='published'
			ORDER BY (v.payload->>'status_effective_at')::timestamptz DESC,v.data_revision DESC,v.revision DESC`, f.state.Context.OrganizationID, id, f.state.Revision)
		if err != nil {
			return false, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var entry knowledgeStatus
			if err := rows.Scan(&entry.EffectiveAt, &entry.Status); err != nil {
				return false, err
			}
			history = append(history, entry)
		}
		if err := rows.Err(); err != nil {
			return false, err
		}
		// The cache belongs to one frozen frame; current object ACLs are checked
		// separately on each request before evidence can be returned.
		f.knowledgeHistory[id] = history
	}
	index := sort.Search(len(history), func(i int) bool { return !history[i].EffectiveAt.After(at) })
	return index < len(history) && history[index].Status == "expired", nil
}

func (f *analysisFrame) knowledgeEvidence(id string) ([]ReferenceEvidence, error) {
	if _, err := f.readableKnowledge(id); err != nil {
		return nil, err
	}
	usages := map[string]usageFact{}
	facts, err := f.periodFacts(analysisSelection{})
	if err != nil {
		return nil, err
	}
	for _, u := range facts {
		if u.Outcome == "succeeded" {
			usages[u.ID] = u
		}
	}
	ratings := f.ratings()
	result := []ReferenceEvidence{}
	seen := map[[2]string]bool{}
	for _, reference := range f.records["reference"] {
		u, ok := usages[reference.str("usage_event_id")]
		if !ok {
			continue
		}
		version := f.records["knowledge_version"][reference.str("knowledge_version_id")]
		if version == nil || version.str("knowledge_id") != id {
			continue
		}
		key := [2]string{u.ID, version.ID}
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := f.readableVersion(id, version.ID); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		item := ReferenceEvidence{EventID: u.ID, OccurredAt: u.OccurredAt.UTC().Format(time.RFC3339Nano), KnowledgeVersionID: version.ID}
		var err error
		item.Application, err = f.safeSummary("application", stringValue(u.ApplicationID))
		if err != nil {
			return nil, err
		}
		item.Team, err = f.safeSummary("team", stringValue(u.TeamID))
		if err != nil {
			return nil, err
		}
		item.ExpiredAtUse, err = f.expiredKnowledgeAt(id, u.OccurredAt)
		if err != nil {
			return nil, err
		}
		if rating, ok := ratings[u.ID]; ok {
			item.Rating = ptr(rating)
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OccurredAt != result[j].OccurredAt {
			return result[i].OccurredAt > result[j].OccurredAt
		}
		if result[i].EventID != result[j].EventID {
			return result[i].EventID < result[j].EventID
		}
		return result[i].KnowledgeVersionID < result[j].KnowledgeVersionID
	})
	return result, nil
}

func (f *analysisFrame) knowledgeUsage(id string) (KnowledgeDetailUsage, error) {
	result := KnowledgeDetailUsage{ContextID: f.state.Context.ID}
	cover := coverage(f.state.Sources, f.state.Context.Range, "usage", "reference")
	if cover == "unavailable" {
		return result, nil
	}
	evidence, err := f.knowledgeEvidence(id)
	if err != nil {
		return result, err
	}
	events, teams, rated, helpful, expired := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range evidence {
		events[item.EventID] = true
		if item.Team != nil {
			teams[item.Team.ID] = true
		}
		if item.Rating != nil {
			rated[item.EventID] = true
			if *item.Rating == "helpful" {
				helpful[item.EventID] = true
			}
		}
		if item.ExpiredAtUse {
			expired[item.EventID] = true
		}
	}
	result.References = nullableObserved(int64(len(events)), cover)
	result.Teams = nullableObserved(int64(len(teams)), cover)
	result.ExpiredAtUseReferences = nullableObserved(int64(len(expired)), cover)
	ratingCover := coverage(f.state.Sources, f.state.Context.Range, "rating", "usage", "reference")
	result.Rated = nullableObserved(int64(len(rated)), ratingCover)
	result.Helpful = nullableObserved(int64(len(helpful)), ratingCover)
	return result, nil
}

func (f *analysisFrame) knowledgeDetail(id string) (KnowledgeDetail, error) {
	r, err := f.readableKnowledge(id)
	if err != nil {
		return KnowledgeDetail{}, err
	}
	result := KnowledgeDetail{}
	result.Knowledge, err = knowledgeRow(r)
	if err != nil {
		return result, err
	}
	result.Version, err = f.knowledgeVersion(id, r.str("current_version_id"))
	if err != nil {
		return result, err
	}
	var cut bool
	result.ApplicationIDs, cut, err = f.filteredRelationIDs("application", r.strings("application_ids"), "analytics:read")
	if err != nil {
		return result, err
	}
	result.RelationsTruncated = cut
	result.SceneIDs, cut, err = f.filteredRelationIDs("scene", r.strings("scene_ids"), "analytics:read")
	if err != nil {
		return result, err
	}
	result.RelationsTruncated = result.RelationsTruncated || cut
	result.Usage, err = f.knowledgeUsage(id)
	if err != nil {
		return result, err
	}
	err = f.service.db.QueryRowContext(f.ctx, `SELECT EXISTS(SELECT 1 FROM bi_favorites WHERE manager_id=$1 AND organization_id=$2 AND knowledge_id=$3)`, f.state.Principal.ManagerID, f.state.Context.OrganizationID, id).Scan(&result.Favorite)
	if err != nil {
		return result, err
	}
	result.MyNote, err = readNote(f.ctx, f.service.db, f.state.Principal, f.state.Context.OrganizationID, id)
	if errors.Is(err, ErrNotFound) {
		err = nil
	}
	return result, err
}

func (f *analysisFrame) knowledgeRows(status string, reused, stale *bool) ([]KnowledgeRow, error) {
	if (reused != nil && !*reused) || (stale != nil && !*stale) {
		if coverage(f.state.Sources, f.state.Context.Range, "usage", "reference") != "ready" {
			return nil, apiError(503, "DATA_UNAVAILABLE", "Reference coverage is insufficient to prove absence")
		}
	}
	result := []KnowledgeRow{}
	for id, r := range f.records["knowledge"] {
		if status != "" && r.str("status") != status {
			continue
		}
		if _, err := f.readableKnowledge(id); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		if reused != nil || stale != nil {
			evidence, err := f.knowledgeEvidence(id)
			if err != nil {
				return nil, err
			}
			isReused := r.str("status") == "valid" && len(evidence) > 0
			isStale := r.str("status") == "expired" && len(evidence) > 0
			if reused != nil && isReused != *reused {
				continue
			}
			if stale != nil && isStale != *stale {
				continue
			}
		}
		row, err := knowledgeRow(r)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt != result[j].UpdatedAt {
			return result[i].UpdatedAt > result[j].UpdatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (f *analysisFrame) knowledgeVersions(id string) ([]VersionSummary, error) {
	if _, err := f.readableKnowledge(id); err != nil {
		return nil, err
	}
	result := []VersionSummary{}
	for versionID, r := range f.records["knowledge_version"] {
		if r.str("knowledge_id") != id {
			continue
		}
		if _, err := f.readableVersion(id, versionID); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		var row VersionSummary
		if err := json.Unmarshal(r.Raw, &row); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt > result[j].CreatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (f *analysisFrame) favorites() ([]Favorite, error) {
	rows, err := f.service.db.QueryContext(f.ctx, `SELECT knowledge_id,created_at FROM bi_favorites WHERE manager_id=$1 AND organization_id=$2 ORDER BY created_at DESC,knowledge_id`, f.state.Principal.ManagerID, f.state.Context.OrganizationID)
	if err != nil {
		return nil, err
	}
	type entry struct {
		id      string
		created time.Time
	}
	entries := []entry{}
	for rows.Next() {
		var value entry
		if err := rows.Scan(&value.id, &value.created); err != nil {
			_ = rows.Close()
			return nil, err
		}
		entries = append(entries, value)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	result := []Favorite{}
	for _, entry := range entries {
		r, err := f.readableKnowledge(entry.id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		row, err := knowledgeRow(r)
		if err != nil {
			return nil, err
		}
		result = append(result, Favorite{Knowledge: row, CreatedAt: entry.created.UTC().Format(time.RFC3339Nano)})
	}
	return result, nil
}
