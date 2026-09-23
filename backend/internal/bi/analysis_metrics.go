package bi

import (
	"math"
	"math/big"
)

type tokenAccumulator struct {
	input, output, read, write, human, automatic, unknown, nonCohort big.Int
	measured, unmeasured                                             int64
}

func (a *tokenAccumulator) add(u usageFact, inCohort bool) {
	if u.Normalized.Input == nil {
		a.unmeasured++
		return
	}
	a.measured++
	input, _ := new(big.Int).SetString(*u.Normalized.Input, 10)
	output, _ := new(big.Int).SetString(*u.Normalized.Output, 10)
	read, _ := new(big.Int).SetString(*u.Normalized.CacheRead, 10)
	write, _ := new(big.Int).SetString(*u.Normalized.CacheWrite, 10)
	a.input.Add(&a.input, input)
	a.output.Add(&a.output, output)
	a.read.Add(&a.read, read)
	a.write.Add(&a.write, write)
	total := new(big.Int).Add(input, output)
	switch u.ActorType {
	case "human":
		a.human.Add(&a.human, total)
		if !inCohort {
			a.nonCohort.Add(&a.nonCohort, total)
		}
	case "automatic":
		a.automatic.Add(&a.automatic, total)
	default:
		a.unknown.Add(&a.unknown, total)
	}
}

func (a *tokenAccumulator) totals(complete, cohortKnown bool) TokenTotals {
	result := TokenTotals{MeasuredRequests: a.measured, UnmeasuredRequests: a.unmeasured}
	if a.measured == 0 && (a.unmeasured > 0 || !complete) {
		return result
	}
	result.Input = ptr(a.input.String())
	result.Output = ptr(a.output.String())
	result.CacheRead = ptr(a.read.String())
	result.CacheWrite = ptr(a.write.String())
	result.Total = ptr(new(big.Int).Add(&a.input, &a.output).String())
	result.Human = ptr(a.human.String())
	result.Automatic = ptr(a.automatic.String())
	result.Unknown = ptr(a.unknown.String())
	if cohortKnown {
		result.NonCohortHumanTokens = ptr(a.nonCohort.String())
	}
	return result
}

type metricResult struct {
	stats                         UsageStats
	cohort                        map[string]*record
	days                          map[string]map[string]bool
	stable                        map[string]bool
	facts                         []usageFact
	memberTokens                  map[string]*tokenAccumulator
	usageCoverage, cohortCoverage string
}

func partialMetric(metric Metric, reason string) Metric {
	metric.Status = "partial"
	metric.Reason = ptr(reason)
	return metric
}
func nullableObserved(count int64, status string) *int64 {
	if status == "ready" || (status == "partial" && count > 0) {
		return ptr(count)
	}
	return nil
}

func stableRange(period DateRange) DateRange {
	_, end := period.bounds()
	last := weekStart(end)
	return dateRange(last.AddDate(0, 0, -28), last, "最近四个完整周")
}

func (f *analysisFrame) calculate(period DateRange, sel analysisSelection) metricResult {
	start, end := period.bounds()
	cohort := f.cohort(period, sel)
	usageCoverage := coverage(f.state.Sources, period, "usage")
	cohortCoverage := coverage(f.state.Sources, period, "member", "membership", "eligibility")
	ratingCoverage := coverage(f.state.Sources, period, "rating")
	result := metricResult{cohort: cohort, days: map[string]map[string]bool{}, stable: map[string]bool{}, facts: []usageFact{}, memberTokens: map[string]*tokenAccumulator{}, usageCoverage: usageCoverage, cohortCoverage: cohortCoverage}
	stats := UsageStats{ContextID: f.state.Context.ID, EffectiveFilters: f.filters(sel)}
	for _, target := range []*Metric{&stats.EligibleMembers, &stats.ActivatedMembers, &stats.ActiveMembers, &stats.StableMembers, &stats.RepeatMembers, &stats.AdoptionRate, &stats.FailureRate, &stats.HelpfulRate, &stats.FeedbackCoverage} {
		*target = unavailableMetric("source_unavailable")
	}
	stats.AdoptionComparison = Comparison{Reason: ptr("incomplete_data")}
	activated := map[string]bool{}
	weeks := map[string]map[string]bool{}
	stableWindow := stableRange(period)
	stableStart, stableEnd := stableWindow.bounds()
	apps := map[string]bool{}
	ratings := f.ratings()
	var totals tokenAccumulator
	var requests, failed, unknown, human, rated, helpful, unknownIdentity int64
	for _, u := range f.facts {
		if !f.matchesUsage(u, sel, true) || !u.OccurredAt.Before(end) {
			continue
		}
		member := stringValue(u.MemberID)
		_, isCohort := cohort[member]
		if isCohort && u.ActorType == "human" && u.Outcome == "succeeded" {
			activated[member] = true
			if !u.OccurredAt.Before(stableStart) && u.OccurredAt.Before(stableEnd) {
				if weeks[member] == nil {
					weeks[member] = map[string]bool{}
				}
				weeks[member][weekStart(u.OccurredAt).Format("2006-01-02")] = true
			}
			if !u.OccurredAt.Before(start) {
				if result.days[member] == nil {
					result.days[member] = map[string]bool{}
				}
				result.days[member][dayStart(u.OccurredAt).Format("2006-01-02")] = true
			}
		}
		if u.OccurredAt.Before(start) {
			continue
		}
		result.facts = append(result.facts, u)
		requests++
		totals.add(u, isCohort)
		if u.Outcome == "failed" {
			failed++
		}
		if u.Outcome == "unknown" {
			unknown++
		}
		if u.ActorType == "unknown" {
			unknownIdentity++
		}
		if u.Outcome == "succeeded" && u.ApplicationID != nil {
			apps[*u.ApplicationID] = true
		}
		if u.ActorType == "human" {
			if result.memberTokens[member] == nil {
				result.memberTokens[member] = new(tokenAccumulator)
			}
			result.memberTokens[member].add(u, isCohort)
			human++
			if rating, ok := ratings[u.ID]; ok {
				rated++
				if rating == "helpful" {
					helpful++
				}
			}
		}
	}
	for member, activeWeeks := range weeks {
		result.stable[member] = len(activeWeeks) >= 3
	}
	stats.Tokens = totals.totals(usageCoverage == "ready", cohortCoverage == "ready")
	stats.Requests = nullableObserved(requests, usageCoverage)
	stats.FailedRequests = nullableObserved(failed, usageCoverage)
	stats.UnknownOutcomeRequests = nullableObserved(unknown, usageCoverage)
	stats.HumanRequests = nullableObserved(human, usageCoverage)
	stats.ActiveApplications = nullableObserved(int64(len(apps)), usageCoverage)
	if usageCoverage == "partial" && requests > 0 {
		stats.FailedRequests = ptr(failed)
		stats.UnknownOutcomeRequests = ptr(unknown)
		stats.HumanRequests = ptr(human)
		stats.ActiveApplications = ptr(int64(len(apps)))
	}
	if usageCoverage != "unavailable" && (requests > 0 || usageCoverage == "ready") {
		stats.FailureRate = ratioMetric(failed, requests-unknown)
		if usageCoverage != "ready" {
			stats.FailureRate = partialMetric(stats.FailureRate, "period_incomplete")
		}
		if unknown > 0 {
			stats.FailureRate = partialMetric(stats.FailureRate, "unknown_outcome")
		}
	}
	if ratingCoverage != "unavailable" && usageCoverage != "unavailable" {
		combined := ratingCoverage
		if usageCoverage != "ready" {
			combined = "partial"
		}
		stats.RatedInteractions = nullableObserved(rated, combined)
		stats.HelpfulInteractions = nullableObserved(helpful, combined)
		stats.HelpfulRate = ratioMetric(helpful, rated)
		stats.FeedbackCoverage = ratioMetric(rated, human)
		if combined != "ready" {
			stats.HelpfulRate = partialMetric(stats.HelpfulRate, "period_incomplete")
			stats.FeedbackCoverage = partialMetric(stats.FeedbackCoverage, "period_incomplete")
		}
	}
	switch cohortCoverage {
	case "ready":
		stats.EligibleMembers = countMetric(int64(len(cohort)))
		if usageCoverage != "unavailable" {
			var repeat, stable int64
			for _, days := range result.days {
				if len(days) >= 2 {
					repeat++
				}
			}
			for _, yes := range result.stable {
				if yes {
					stable++
				}
			}
			stats.ActiveMembers = countMetric(int64(len(result.days)))
			stats.RepeatMembers = countMetric(repeat)
			stats.AdoptionRate = ratioMetric(int64(len(result.days)), int64(len(cohort)))
			if usageCoverage != "ready" {
				stats.ActiveMembers = partialMetric(stats.ActiveMembers, "period_incomplete")
				stats.RepeatMembers = partialMetric(stats.RepeatMembers, "period_incomplete")
				stats.AdoptionRate = partialMetric(stats.AdoptionRate, "period_incomplete")
			}
			if unknownIdentity > 0 {
				stats.ActiveMembers = partialMetric(stats.ActiveMembers, "unknown_identity")
				stats.AdoptionRate = partialMetric(stats.AdoptionRate, "unknown_identity")
			}
			if f.historyComplete(end, "usage", "member", "membership") {
				stats.ActivatedMembers = countMetric(int64(len(activated)))
			} else {
				stats.ActivatedMembers = unavailableMetric("history_incomplete")
			}
			if coverage(f.state.Sources, stableWindow, "usage", "member", "membership") == "ready" {
				stats.StableMembers = countMetric(stable)
			} else {
				stats.StableMembers = unavailableMetric("history_incomplete")
			}
		}
	case "partial":
		stats.EligibleMembers = unavailableMetric("history_incomplete")
	}
	if period.CompleteDays == 0 {
		stats.Requests, stats.FailedRequests, stats.UnknownOutcomeRequests, stats.HumanRequests, stats.ActiveApplications, stats.RatedInteractions, stats.HelpfulInteractions = nil, nil, nil, nil, nil, nil, nil
		stats.Tokens = TokenTotals{}
		for _, metric := range []*Metric{&stats.ActiveMembers, &stats.RepeatMembers, &stats.AdoptionRate, &stats.FailureRate, &stats.HelpfulRate, &stats.FeedbackCoverage} {
			*metric = Metric{Status: "not_observable", Reason: ptr("period_incomplete")}
		}
	}
	result.stats = stats
	return result
}

func (f *analysisFrame) statistics(sel analysisSelection) metricResult {
	if f.statisticsCache == nil {
		f.statisticsCache = map[analysisSelection]metricResult{}
	}
	if cached, ok := f.statisticsCache[sel]; ok {
		return cached
	}
	current := f.calculate(f.state.Context.Range, sel)
	previous := f.calculate(f.state.Context.ComparisonRange, sel)
	now, old := current.stats.AdoptionRate, previous.stats.AdoptionRate
	comparison := Comparison{Previous: old.Value, Reason: ptr("incomplete_data")}
	if now.Status == "ready" && old.Status == "ready" && now.Value != nil && old.Value != nil {
		comparison.Comparable = true
		comparison.Reason = nil
		comparison.RateDifference = ptr(*now.Value - *old.Value)
		if *old.Value > 0 {
			comparison.RelativeChange = ptr((*now.Value - *old.Value) / *old.Value)
		} else {
			comparison.Reason = ptr("no_baseline")
		}
	}
	current.stats.AdoptionComparison = comparison
	f.statisticsCache[sel] = current
	return current
}

func tokenRelative(current, previous *string) *float64 {
	if current == nil || previous == nil {
		return nil
	}
	now, ok1 := new(big.Int).SetString(*current, 10)
	old, ok2 := new(big.Int).SetString(*previous, 10)
	if !ok1 || !ok2 || old.Sign() == 0 {
		return nil
	}
	value, _ := new(big.Rat).SetFrac(new(big.Int).Sub(now, old), old).Float64()
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return nil
	}
	return &value
}

func (f *analysisFrame) trend(sel analysisSelection, bucket string) Trend {
	result := Trend{ContextID: f.state.Context.ID, Bucket: bucket, Points: []TrendPoint{}}
	start, end := f.state.Context.Range.bounds()
	step := 1
	if bucket == "seven_day" {
		step = 7
	}
	for cursor := start; cursor.Before(end); cursor = cursor.AddDate(0, 0, step) {
		stop := cursor.AddDate(0, 0, step)
		if stop.After(end) {
			stop = end
		}
		period := dateRange(cursor, stop, "")
		stats := f.calculate(period, sel)
		point := TrendPoint{StartDate: period.StartDate, EndDateExclusive: period.EndDateExclusive, Tokens: stats.stats.Tokens.Total, Status: stats.usageCoverage}
		if stats.cohortCoverage == "ready" && stats.usageCoverage != "unavailable" {
			point.ActiveMembers = ptr(int64(len(stats.days)))
			if stats.usageCoverage == "ready" && stats.stats.ActiveMembers.Status == "ready" {
				var days int
				for _, value := range stats.days {
					days += len(value)
				}
				point.AverageDailyActive = ptr(float64(days) / float64(period.CompleteDays))
			}
		}
		if stats.stats.ActiveMembers.Status == "partial" {
			point.Status = "partial"
		}
		result.Points = append(result.Points, point)
	}
	return result
}

func eventWithin(u usageFact, period DateRange) bool {
	start, end := period.bounds()
	return !u.OccurredAt.Before(start) && u.OccurredAt.Before(end)
}
