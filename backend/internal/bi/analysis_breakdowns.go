package bi

import (
	"math/big"
	"sort"
)

func compareTokens(a, b *string) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	x, _ := new(big.Int).SetString(*a, 10)
	y, _ := new(big.Int).SetString(*b, 10)
	return -x.Cmp(y)
}

func (f *analysisFrame) tokenAnalysis(sel analysisSelection) TokenAnalysis {
	current := f.statistics(sel)
	previous := f.calculate(f.state.Context.ComparisonRange, sel)
	result := TokenAnalysis{ContextID: f.state.Context.ID, Totals: current.stats.Tokens, PreviousTotal: previous.stats.Tokens.Total, Models: []ModelTokens{}, ComparisonReason: ptr("incomplete_data")}
	if current.usageCoverage == "ready" && previous.usageCoverage == "ready" && current.stats.Tokens.UnmeasuredRequests == 0 && previous.stats.Tokens.UnmeasuredRequests == 0 {
		result.RelativeChange = tokenRelative(current.stats.Tokens.Total, previous.stats.Tokens.Total)
		if result.RelativeChange != nil {
			result.ComparisonReason = nil
		} else if result.PreviousTotal == nil || *result.PreviousTotal == "0" {
			result.ComparisonReason = ptr("no_baseline")
		}
	}
	accumulators := map[string]*tokenAccumulator{}
	counts := map[string]int64{}
	for _, u := range current.facts {
		if accumulators[u.RequestedModel] == nil {
			accumulators[u.RequestedModel] = new(tokenAccumulator)
		}
		accumulators[u.RequestedModel].add(u, false)
		counts[u.RequestedModel]++
	}
	for model, a := range accumulators {
		result.Models = append(result.Models, ModelTokens{Model: model, ModelSource: "requested", Tokens: a.totals(current.usageCoverage == "ready", false).Total, Requests: counts[model]})
	}
	sort.Slice(result.Models, func(i, j int) bool {
		cmp := compareTokens(result.Models[i].Tokens, result.Models[j].Tokens)
		if cmp == 0 {
			return result.Models[i].Model < result.Models[j].Model
		}
		return cmp < 0
	})
	return result
}

func (f *analysisFrame) feedback(sel analysisSelection) (FeedbackAnalysis, error) {
	current := f.statistics(sel)
	result := FeedbackAnalysis{ContextID: f.state.Context.ID, Rated: current.stats.RatedInteractions, Helpful: current.stats.HelpfulInteractions, Coverage: current.stats.FeedbackCoverage, ByApplication: []FeedbackAnalysisByApplicationItem{}}
	if result.Rated != nil && result.Helpful != nil {
		result.Negative = ptr(*result.Rated - *result.Helpful)
	}
	groups := map[string][3]int64{}
	ratings := f.ratings()
	for _, u := range current.facts {
		if u.ActorType != "human" {
			continue
		}
		rating, ok := ratings[u.ID]
		if !ok {
			continue
		}
		id := stringValue(u.ApplicationID)
		if id != "" {
			visible, err := f.visible("application", id)
			if err != nil {
				return result, err
			}
			if !visible {
				continue
			}
		}
		values := groups[id]
		values[0]++
		if rating == "helpful" {
			values[1]++
		} else {
			values[2]++
		}
		groups[id] = values
	}
	keys := []string{}
	for id := range groups {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		values := groups[id]
		result.ByApplication = append(result.ByApplication, FeedbackAnalysisByApplicationItem{Application: f.summary("application", id), Rated: ptr(values[0]), Helpful: ptr(values[1]), Negative: ptr(values[2])})
	}
	return result, nil
}

func (f *analysisFrame) memberRow(id string, sel analysisSelection) (MemberRow, bool, error) {
	result := f.statistics(sel)
	member := result.cohort[id]
	if member == nil {
		return MemberRow{}, false, nil
	}
	_, end := f.state.Context.Range.bounds()
	membership := f.membership(id, end.Add(-1))
	row := MemberRow{ID: id, Name: member.str("name"), Status: member.str("status")}
	if membership != nil {
		var err error
		row.Team, err = f.safeSummary("team", membership.str("team_id"))
		if err != nil {
			return row, false, err
		}
		row.Role, err = f.safeSummary("role", membership.str("role_id"))
		if err != nil {
			return row, false, err
		}
	}
	tokens := result.memberTokens[id]
	if tokens == nil {
		tokens = new(tokenAccumulator)
	}
	row.Tokens = tokens.totals(result.usageCoverage == "ready", true).Total
	if result.usageCoverage == "ready" {
		row.ActiveDays = ptr(int64(len(result.days[id])))
	}
	if result.stats.StableMembers.Status == "ready" {
		row.Stable = ptr(result.stable[id])
	}
	return row, true, nil
}

func (f *analysisFrame) memberDetail(id string) (MemberDetail, error) {
	allowed, err := f.visible("member", id)
	if err != nil {
		return MemberDetail{}, err
	}
	if !allowed {
		return MemberDetail{}, ErrNotFound
	}
	row, ok, err := f.memberRow(id, analysisSelection{})
	if err != nil {
		return MemberDetail{}, err
	}
	if !ok {
		return MemberDetail{}, ErrNotFound
	}
	result := MemberDetail{ContextID: f.state.Context.ID, Member: row, ApplicationUsage: []MemberDetailApplicationUsageItem{}}
	stats := f.statistics(analysisSelection{MemberID: id})
	groups := map[string]*tokenAccumulator{}
	counts := map[string]int64{}
	for _, u := range stats.facts {
		app := stringValue(u.ApplicationID)
		if app != "" {
			allowed, err := f.visible("application", app)
			if err != nil {
				return result, err
			}
			if !allowed {
				continue
			}
		}
		if groups[app] == nil {
			groups[app] = new(tokenAccumulator)
		}
		groups[app].add(u, true)
		counts[app]++
	}
	for id, tokens := range groups {
		result.ApplicationUsage = append(result.ApplicationUsage, MemberDetailApplicationUsageItem{Application: f.summary("application", id), Requests: counts[id], Tokens: tokens.totals(stats.usageCoverage == "ready", true).Total})
	}
	sort.Slice(result.ApplicationUsage, func(i, j int) bool {
		a, b := result.ApplicationUsage[i], result.ApplicationUsage[j]
		cmp := compareTokens(a.Tokens, b.Tokens)
		if cmp != 0 {
			return cmp < 0
		}
		if a.Application == nil {
			return b.Application != nil
		}
		if b.Application == nil {
			return false
		}
		return a.Application.ID < b.Application.ID
	})
	return result, nil
}
