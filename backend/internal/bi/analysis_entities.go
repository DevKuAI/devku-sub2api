package bi

import (
	"sort"
	"strings"
)

func (f *analysisFrame) applicationRow(r *record) ApplicationRow {
	return ApplicationRow{ID: r.ID, Name: r.str("name"), Type: r.str("type"), Summary: r.str("summary"), CurrentVersionID: r.str("current_version_id"), Stats: f.statistics(analysisSelection{ApplicationID: r.ID}).stats}
}
func (f *analysisFrame) sceneRow(r *record) SceneRow {
	return SceneRow{ID: r.ID, Name: r.str("name"), Category: r.str("category"), Summary: r.str("summary"), Classification: r.str("classification"), Stats: f.statistics(analysisSelection{SceneID: r.ID}).stats}
}

func (f *analysisFrame) linkedKnowledge(id string) (*record, error) {
	if id == "" {
		return nil, nil
	}
	if !containsAll(f.state.Scope.Capabilities, []string{"knowledge:read"}) {
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
	if !f.contentMatches(r) {
		return nil, ErrNotFound
	}
	return r, nil
}

func (f *analysisFrame) applicationRows(kind, knowledgeID string) ([]ApplicationRow, error) {
	knowledge, err := f.linkedKnowledge(knowledgeID)
	if err != nil {
		return nil, err
	}
	result := []ApplicationRow{}
	for id, r := range f.records["application"] {
		if kind != "" && r.str("type") != kind {
			continue
		}
		if knowledge != nil && !recordListContains(knowledge, "application_ids", id) {
			continue
		}
		if f.state.Context.Filters.ApplicationID != "" && f.state.Context.Filters.ApplicationID != id {
			continue
		}
		if f.state.Context.Filters.SceneID != "" && !recordListContains(r, "scene_ids", f.state.Context.Filters.SceneID) {
			continue
		}
		visible, err := f.visible("application", id)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		result = append(result, f.applicationRow(r))
	}
	sort.Slice(result, func(i, j int) bool {
		cmp := compareTokens(result[i].Stats.Tokens.Total, result[j].Stats.Tokens.Total)
		if cmp == 0 {
			return result[i].ID < result[j].ID
		}
		return cmp < 0
	})
	return result, nil
}

func (f *analysisFrame) sceneRows(knowledgeID string) ([]SceneRow, error) {
	knowledge, err := f.linkedKnowledge(knowledgeID)
	if err != nil {
		return nil, err
	}
	result := []SceneRow{}
	for id, r := range f.records["scene"] {
		if knowledge != nil && !recordListContains(knowledge, "scene_ids", id) {
			continue
		}
		if f.state.Context.Filters.SceneID != "" && f.state.Context.Filters.SceneID != id {
			continue
		}
		if appID := f.state.Context.Filters.ApplicationID; appID != "" {
			app := f.records["application"][appID]
			if app == nil || !recordListContains(app, "scene_ids", id) {
				continue
			}
		}
		visible, err := f.visible("scene", id)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		result = append(result, f.sceneRow(r))
	}
	sort.Slice(result, func(i, j int) bool {
		cmp := compareTokens(result[i].Stats.Tokens.Total, result[j].Stats.Tokens.Total)
		if cmp == 0 {
			return result[i].ID < result[j].ID
		}
		return cmp < 0
	})
	return result, nil
}

func (f *analysisFrame) memberRows(query string, sel analysisSelection) ([]MemberRow, error) {
	if err := f.checkSelection(sel); err != nil {
		return nil, err
	}
	result := []MemberRow{}
	query = strings.ToLower(query)
	base := sel
	base.TeamID = ""
	base.RoleID = ""
	_, end := f.state.Context.Range.bounds()
	at := end.Add(-1)
	for id, member := range f.cohort(f.state.Context.Range, base) {
		if sel.TeamID != "" || sel.RoleID != "" {
			matched := false
			for _, membership := range f.memberships[id] {
				if inInterval(membership, at) && (sel.TeamID == "" || membership.str("team_id") == sel.TeamID) && (sel.RoleID == "" || membership.str("role_id") == sel.RoleID) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(member.str("name")), query) {
			continue
		}
		row, ok, err := f.memberRow(id, base)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, row)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		cmp := compareTokens(result[i].Tokens, result[j].Tokens)
		if cmp == 0 {
			return result[i].ID < result[j].ID
		}
		return cmp < 0
	})
	return result, nil
}

func (f *analysisFrame) comparisons(dimension, applicationID string) ([]ComparisonRow, error) {
	if err := f.checkSelection(analysisSelection{ApplicationID: applicationID}); err != nil {
		return nil, err
	}
	result := []ComparisonRow{}
	for id := range f.records[dimension] {
		visible, err := f.visible(dimension, id)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		sel := analysisSelection{ApplicationID: applicationID}
		if dimension == "team" {
			sel.TeamID = id
		} else {
			sel.RoleID = id
		}
		stats := f.statistics(sel).stats
		entity := f.summary(dimension, id)
		if entity == nil {
			continue
		}
		result = append(result, ComparisonRow{Entity: *entity, Stats: stats})
	}
	sort.Slice(result, func(i, j int) bool {
		cmp := compareTokens(result[i].Stats.Tokens.Total, result[j].Stats.Tokens.Total)
		if cmp == 0 {
			return result[i].Entity.ID < result[j].Entity.ID
		}
		return cmp < 0
	})
	return result, nil
}

func (f *analysisFrame) filteredRelationIDs(kind string, ids []string, capability string) ([]string, bool, error) {
	result := []string{}
	truncated := false
	if !containsAll(f.state.Scope.Capabilities, []string{capability}) {
		return result, false, nil
	}
	sort.Strings(ids)
	seen := map[string]bool{}
	for _, id := range ids {
		if kind == "application" && f.state.Context.Filters.ApplicationID != "" && f.state.Context.Filters.ApplicationID != id {
			continue
		}
		if kind == "scene" && f.state.Context.Filters.SceneID != "" && f.state.Context.Filters.SceneID != id {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		visible, err := f.visible(kind, id)
		if err != nil {
			return nil, false, err
		}
		if !visible {
			continue
		}
		if len(result) < 100 {
			result = append(result, id)
		} else {
			truncated = true
		}
	}
	return result, truncated, nil
}

func (f *analysisFrame) applicationDetail(id string) (ApplicationDetail, error) {
	if err := f.checkSelection(analysisSelection{ApplicationID: id}); err != nil {
		return ApplicationDetail{}, err
	}
	r := f.records["application"][id]
	if r == nil {
		return ApplicationDetail{}, ErrNotFound
	}
	result := ApplicationDetail{Application: f.applicationRow(r)}
	knowledge, evaluations := []string{}, []string{}
	for key, k := range f.records["knowledge"] {
		if recordListContains(k, "application_ids", id) {
			knowledge = append(knowledge, key)
		}
	}
	for key, e := range f.records["evaluation"] {
		if e.str("application_id") == id {
			evaluations = append(evaluations, key)
		}
	}
	var err error
	var cut bool
	result.SceneIDs, cut, err = f.filteredRelationIDs("scene", r.strings("scene_ids"), "analytics:read")
	if err != nil {
		return result, err
	}
	result.RelationsTruncated = cut
	result.TeamIDs, cut, err = f.filteredRelationIDs("team", r.strings("team_ids"), "analytics:read")
	if err != nil {
		return result, err
	}
	result.RelationsTruncated = result.RelationsTruncated || cut
	result.KnowledgeIDs, cut, err = f.filteredRelationIDs("knowledge", knowledge, "knowledge:read")
	if err != nil {
		return result, err
	}
	result.RelationsTruncated = result.RelationsTruncated || cut
	result.EvaluationIDs, cut, err = f.filteredRelationIDs("evaluation", evaluations, "evaluations:read")
	result.RelationsTruncated = result.RelationsTruncated || cut
	return result, err
}

func (f *analysisFrame) sceneDetail(id string) (SceneDetail, error) {
	if err := f.checkSelection(analysisSelection{SceneID: id}); err != nil {
		return SceneDetail{}, err
	}
	r := f.records["scene"][id]
	if r == nil {
		return SceneDetail{}, ErrNotFound
	}
	ids := []string{}
	for key, app := range f.records["application"] {
		if recordListContains(app, "scene_ids", id) {
			ids = append(ids, key)
		}
	}
	allowed, cut, err := f.filteredRelationIDs("application", ids, "analytics:read")
	return SceneDetail{Scene: f.sceneRow(r), ApplicationIDs: allowed, RelationsTruncated: cut}, err
}
