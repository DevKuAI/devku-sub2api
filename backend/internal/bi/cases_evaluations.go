package bi

import (
	"encoding/json"
	"errors"
	"sort"
)

func (f *analysisFrame) caseRows(knowledgeID string) ([]CaseRow, error) {
	if _, err := f.linkedKnowledge(knowledgeID); err != nil {
		return nil, err
	}
	result := []CaseRow{}
	for id := range f.records["case"] {
		r, err := f.readableCase(id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if knowledgeID != "" {
			linked := false
			for _, versionID := range r.strings("knowledge_version_ids") {
				version := f.records["knowledge_version"][versionID]
				if version != nil && version.str("knowledge_id") == knowledgeID {
					linked = true
					break
				}
			}
			if !linked {
				continue
			}
		}
		var row CaseRow
		if err := json.Unmarshal(r.Raw, &row); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OccurredAt != result[j].OccurredAt {
			return result[i].OccurredAt > result[j].OccurredAt
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (f *analysisFrame) caseDetail(id string) (CaseDetail, error) {
	if _, err := f.readableCase(id); err != nil {
		return CaseDetail{}, err
	}
	r, err := recordAt(f.ctx, f.service.db, f.state.Context.OrganizationID, "case", id, f.state.Revision)
	if err != nil {
		return CaseDetail{}, err
	}
	if r == nil {
		return CaseDetail{}, ErrNotFound
	}
	result := CaseDetail{KnowledgeVersions: []KnowledgeVersionLink{}}
	if err := json.Unmarshal(r.Raw, &result); err != nil {
		return result, err
	}
	if err := json.Unmarshal(r.Raw, &result.Case); err != nil {
		return result, err
	}
	for _, relation := range []struct {
		kind, id string
		target   **string
	}{{"application", r.str("application_id"), &result.ApplicationID}, {"scene", r.str("scene_id"), &result.SceneID}} {
		*relation.target = nil
		if relation.id == "" {
			continue
		}
		allowed, err := f.visible(relation.kind, relation.id)
		if err != nil {
			return result, err
		}
		if allowed {
			*relation.target = ptr(relation.id)
		}
	}
	seen := map[string]bool{}
	for _, versionID := range r.strings("knowledge_version_ids") {
		if seen[versionID] {
			continue
		}
		seen[versionID] = true
		version, err := f.readableVersion("", versionID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return result, err
		}
		result.KnowledgeVersions = append(result.KnowledgeVersions, KnowledgeVersionLink{KnowledgeID: version.str("knowledge_id"), VersionID: versionID})
	}
	result.Sources, err = f.sourceLinks(r)
	return result, err
}

func evaluationRow(r *record) (EvaluationRow, error) {
	var result EvaluationRow
	if err := json.Unmarshal(r.Raw, &result); err != nil {
		return result, err
	}
	if err := json.Unmarshal(r.Payload["_sample_count"], &result.SampleCount); err != nil {
		return result, err
	}
	return result, nil
}

func (f *analysisFrame) evaluationRows(applicationID string) ([]EvaluationRow, error) {
	if err := f.checkSelection(analysisSelection{ApplicationID: applicationID}); err != nil {
		return nil, err
	}
	result := []EvaluationRow{}
	for id, r := range f.records["evaluation"] {
		if applicationID != "" && r.str("application_id") != applicationID {
			continue
		}
		if _, err := f.readableEvaluation(id); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		row, err := evaluationRow(r)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].EvaluatedAt != result[j].EvaluatedAt {
			return result[i].EvaluatedAt > result[j].EvaluatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (f *analysisFrame) evaluationSamples(id string) ([]EvaluationSample, []EvaluationSample, error) {
	if _, err := f.readableEvaluation(id); err != nil {
		return nil, nil, err
	}
	r, err := recordAt(f.ctx, f.service.db, f.state.Context.OrganizationID, "evaluation", id, f.state.Revision)
	if err != nil {
		return nil, nil, err
	}
	if r == nil {
		return nil, nil, ErrNotFound
	}
	var all []EvaluationSample
	if err := json.Unmarshal(r.Payload["samples"], &all); err != nil {
		return nil, nil, err
	}
	visible := []EvaluationSample{}
	caseAccess := map[string]bool{}
	for _, sample := range all {
		if sample.CaseLink.ID == nil {
			sample.CaseLink = SourceLink{Label: "来源不可访问", Availability: "deleted"}
			visible = append(visible, sample)
			continue
		}
		if sample.CaseLink.Availability != "available" {
			continue
		}
		caseID := *sample.CaseLink.ID
		allowed, checked := caseAccess[caseID]
		if !checked {
			parent, err := f.readableCase(caseID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return nil, nil, err
			}
			allowed = err == nil
			if allowed {
				allowed, err = f.evidenceReadable(parent)
				if err != nil {
					return nil, nil, err
				}
			}
			caseAccess[caseID] = allowed
		}
		if !allowed {
			continue
		}
		sample.CaseLink = SourceLink{ID: ptr(caseID), Label: f.records["case"][caseID].str("title"), Availability: "available"}
		visible = append(visible, sample)
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	return all, visible, nil
}

func evaluationCounts(version string, samples []EvaluationSample, candidate bool) EvaluationResult {
	result := EvaluationResult{ApplicationVersionID: version}
	for _, sample := range samples {
		status := sample.Baseline
		if candidate {
			status = sample.Candidate
		}
		switch status {
		case "passed":
			result.Passed++
		case "failed":
			result.Failed++
		default:
			result.NotRun++
		}
	}
	return result
}

func (f *analysisFrame) evaluationDetail(id string) (EvaluationDetail, error) {
	r, err := f.readableEvaluation(id)
	if err != nil {
		return EvaluationDetail{}, err
	}
	all, visible, err := f.evaluationSamples(id)
	if err != nil {
		return EvaluationDetail{}, err
	}
	row, err := evaluationRow(r)
	if err != nil {
		return EvaluationDetail{}, err
	}
	result := EvaluationDetail{Evaluation: row, Criterion: r.str("criterion"), Baseline: evaluationCounts(r.str("baseline_application_version_id"), all, false), Candidate: evaluationCounts(r.str("candidate_application_version_id"), all, true), Comparable: true, SampleVisibility: "full"}
	if len(visible) < len(all) {
		result.SampleVisibility = "restricted"
	}
	return result, nil
}
