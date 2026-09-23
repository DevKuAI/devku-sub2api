package bi

import (
	"fmt"
	"slices"
	"time"
)

func (f *analysisFrame) contentMatches(r *record) bool {
	filters := f.state.Context.Filters
	if filters.ApplicationID != "" && !recordListContains(r, "application_ids", filters.ApplicationID) {
		return false
	}
	if filters.SceneID != "" && !recordListContains(r, "scene_ids", filters.SceneID) {
		return false
	}
	return true
}

func (f *analysisFrame) assets(sel analysisSelection) (AssetStats, error) {
	result := AssetStats{ContextID: f.state.Context.ID, ContentAsOf: f.state.Context.ContentAsOf.Format(time.RFC3339Nano), ReuseRate: unavailableMetric("source_unavailable")}
	_, periodEnd := f.state.Context.Range.bounds()
	cover := contentCoverage(f.state.Sources, periodEnd, "knowledge", "knowledge_version")
	if cover == "unavailable" {
		return result, nil
	}
	valid, draft, expired, added, updated := int64(0), int64(0), int64(0), int64(0), int64(0)
	visible := map[string]*record{}
	start, end := f.state.Context.Range.bounds()
	for id, r := range f.records["knowledge"] {
		if !f.contentMatches(r) {
			continue
		}
		if sel.ApplicationID != "" && !recordListContains(r, "application_ids", sel.ApplicationID) {
			continue
		}
		if sel.SceneID != "" && !recordListContains(r, "scene_ids", sel.SceneID) {
			continue
		}
		allowed, err := f.visible("knowledge", id)
		if err != nil {
			return result, err
		}
		if !allowed {
			continue
		}
		visible[id] = r
		switch r.str("status") {
		case "valid":
			valid++
			created := r.instant("created_at")
			if created != nil && !created.Before(start) && created.Before(end) {
				added++
				continue
			}
			if created != nil && created.Before(start) {
				for _, version := range f.records["knowledge_version"] {
					at := version.instant("created_at")
					if version.str("knowledge_id") == id && at != nil && !at.Before(start) && at.Before(end) {
						updated++
						break
					}
				}
			}
		case "draft":
			draft++
		case "expired":
			expired++
		}
	}
	result.ValidKnowledge = nullableObserved(valid, cover)
	result.DraftKnowledge = nullableObserved(draft, cover)
	result.ExpiredKnowledge = nullableObserved(expired, cover)
	result.AddedValid = nullableObserved(added, cover)
	versionCoverage := coverage(f.state.Sources, f.state.Context.Range, "knowledge_version")
	result.UpdatedValid = nullableObserved(updated, versionCoverage)
	referenceCoverage := coverage(f.state.Sources, f.state.Context.Range, "usage", "reference")
	if referenceCoverage == "unavailable" {
		return result, nil
	}
	usages := map[string]usageFact{}
	facts, err := f.periodFacts(sel)
	if err != nil {
		return result, err
	}
	for _, u := range facts {
		if u.Outcome == "succeeded" {
			usages[u.ID] = u
		}
	}
	reused, stale, expiredAtUse := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, reference := range f.records["reference"] {
		u, ok := usages[reference.str("usage_event_id")]
		if !ok {
			continue
		}
		version := f.records["knowledge_version"][reference.str("knowledge_version_id")]
		if version == nil {
			continue
		}
		knowledge := visible[version.str("knowledge_id")]
		if knowledge == nil {
			continue
		}
		if knowledge.str("status") == "valid" {
			reused[knowledge.ID] = true
		}
		if knowledge.str("status") == "expired" {
			stale[knowledge.ID] = true
		}
		expired, err := f.expiredKnowledgeAt(knowledge.ID, u.OccurredAt)
		if err != nil {
			return result, err
		}
		if expired {
			expiredAtUse[knowledge.ID] = true
		}
	}
	combined := cover
	if referenceCoverage != "ready" {
		combined = "partial"
	}
	result.ReusedValidKnowledge = nullableObserved(int64(len(reused)), combined)
	result.StaleReferencedNow = nullableObserved(int64(len(stale)), combined)
	result.ExpiredAtUse = nullableObserved(int64(len(expiredAtUse)), combined)
	result.ReuseRate = ratioMetric(int64(len(reused)), valid)
	if combined != "ready" {
		result.ReuseRate = partialMetric(result.ReuseRate, "period_incomplete")
	}
	return result, nil
}

func (f *analysisFrame) overview() (Overview, error) {
	result := Overview{ContextID: f.state.Context.ID, Stats: f.statistics(analysisSelection{}).stats, Findings: []Finding{}}
	if slices.Contains(f.state.Scope.Capabilities, "knowledge:read") {
		assets, err := f.assets(analysisSelection{})
		if err != nil {
			return result, err
		}
		result.Assets = &assets
	}
	if result.Stats.ActiveMembers.Status == "ready" && result.Stats.ActiveMembers.Numerator != nil && result.Stats.EligibleMembers.Numerator != nil {
		result.Findings = append(result.Findings, Finding{ID: "adoption_observed", Title: "人员采用", Fact: fmt.Sprintf("期末可使用人员 %d 人，本期成功交互人员 %d 人。", *result.Stats.EligibleMembers.Numerator, *result.Stats.ActiveMembers.Numerator), InterpretationStatus: "none", EvidenceMetricKeys: []string{"eligible_members", "active_members"}, Target: Target{Kind: "metric", ID: "adoption_rate"}})
	}
	if result.Stats.Tokens.Total != nil {
		result.Findings = append(result.Findings, Finding{ID: "tokens_observed", Title: "已观测用量", Fact: fmt.Sprintf("已测量 %d 次调用，共 %s Token；另有 %d 次调用未测量 Token。", result.Stats.Tokens.MeasuredRequests, *result.Stats.Tokens.Total, result.Stats.Tokens.UnmeasuredRequests), InterpretationStatus: "none", EvidenceMetricKeys: []string{"total_tokens"}, Target: Target{Kind: "metric", ID: "total_tokens"}})
	}
	return result, nil
}
