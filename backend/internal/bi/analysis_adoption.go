package bi

import (
	"slices"
	"sort"
	"time"
)

func (f *analysisFrame) adoption(sel analysisSelection) Adoption {
	result := f.statistics(sel)
	buckets := []FrequencyBucket{{MinDays: 0, MaxDays: ptr(int64(0))}, {MinDays: 1, MaxDays: ptr(int64(1))}, {MinDays: 2, MaxDays: ptr(int64(3))}, {MinDays: 4}}
	if result.cohortCoverage == "ready" && result.usageCoverage == "ready" && result.stats.ActiveMembers.Status == "ready" {
		counts := [4]int64{}
		for id := range result.cohort {
			days := len(result.days[id])
			index := 0
			switch {
			case days == 1:
				index = 1
			case days >= 2 && days <= 3:
				index = 2
			case days >= 4:
				index = 3
			}
			counts[index]++
		}
		for index := range buckets {
			buckets[index].Members = ptr(counts[index])
		}
	}
	return Adoption{ContextID: f.state.Context.ID, Stats: result.stats, StableRange: stableRange(f.state.Context.Range), Frequency: buckets, Retention: f.retention(sel)}
}

func (f *analysisFrame) retention(sel analysisSelection) []RetentionCohort {
	result := []RetentionCohort{}
	_, end := f.state.Context.Range.bounds()
	if !f.historyComplete(end, "usage", "member", "membership") {
		return result
	}
	observedSelection := sel
	observedSelection.TeamID = ""
	observedSelection.RoleID = ""
	first := map[string]usageFact{}
	for _, u := range f.facts {
		if u.ActorType != "human" || u.Outcome != "succeeded" || u.MemberID == nil || !u.OccurredAt.Before(end) || !f.matchesUsage(u, observedSelection, false) {
			continue
		}
		old, ok := first[*u.MemberID]
		if !ok || u.OccurredAt.Before(old.OccurredAt) || (u.OccurredAt.Equal(old.OccurredAt) && u.ID < old.ID) {
			first[*u.MemberID] = u
		}
	}
	lastWeek := weekStart(end.Add(-time.Nanosecond))
	oldest := lastWeek.AddDate(0, 0, -77)
	cohorts := map[string][]string{}
	for member, u := range first {
		week := weekStart(u.OccurredAt)
		if week.Before(oldest) || !f.state.teamAllowed(stringValue(u.TeamID)) || (sel.TeamID != "" && stringValue(u.TeamID) != sel.TeamID) {
			continue
		}
		if sel.RoleID != "" {
			role := f.membership(member, u.OccurredAt)
			if role == nil || role.str("role_id") != sel.RoleID {
				continue
			}
		}
		key := week.Format("2006-01-02")
		cohorts[key] = append(cohorts[key], member)
	}
	keys := []string{}
	for key := range cohorts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		members := cohorts[key]
		firstWeek, _ := time.ParseInLocation("2006-01-02", key, shanghai)
		cohort := RetentionCohort{FirstWeek: key, Size: int64(len(members)), Cells: []RetentionCell{}}
		for offset := 1; offset <= 3; offset++ {
			start, stop := firstWeek.AddDate(0, 0, 7*offset), firstWeek.AddDate(0, 0, 7*(offset+1))
			period := dateRange(start, stop, "")
			cell := RetentionCell{WeekOffset: int64(offset), ObservedRange: period}
			switch {
			case stop.After(end):
				cell.Rate = Metric{Status: "not_observable", Reason: ptr("period_incomplete")}
			case coverage(f.state.Sources, period, "usage", "membership") != "ready":
				cell.Rate = unavailableMetric("history_incomplete")
			case !f.observeMembers(members, period):
				cell.Rate = unavailableMetric("restricted_population")
			default:
				memberSet := map[string]bool{}
				for _, id := range members {
					memberSet[id] = true
				}
				retained := map[string]bool{}
				for _, u := range f.facts {
					if u.ActorType == "human" && u.Outcome == "succeeded" && memberSet[stringValue(u.MemberID)] && eventWithin(u, period) && f.matchesUsage(u, observedSelection, false) {
						retained[*u.MemberID] = true
					}
				}
				cell.Retained = ptr(int64(len(retained)))
				cell.Rate = ratioMetric(*cell.Retained, cohort.Size)
			}
			cohort.Cells = append(cohort.Cells, cell)
		}
		result = append(result, cohort)
	}
	return result
}

func (f *analysisFrame) observeMembers(members []string, period DateRange) bool {
	if f.state.Scope.AllTeams {
		return true
	}
	start, end := period.bounds()
	ids := map[string]bool{}
	for _, id := range members {
		ids[id] = true
	}
	for _, usage := range f.facts {
		if usage.ActorType == "human" && ids[stringValue(usage.MemberID)] && eventWithin(usage, period) && !slices.Contains(f.state.Scope.TeamIDs, stringValue(usage.TeamID)) {
			return false
		}
	}
	for _, membership := range f.records["membership"] {
		if !ids[membership.str("member_id")] || !membership.boolean("primary") {
			continue
		}
		from, to := membership.instant("valid_from"), membership.instant("valid_to")
		if from == nil || !from.Before(end) || (to != nil && !to.After(start)) {
			continue
		}
		allowed := false
		for _, team := range f.state.Scope.TeamIDs {
			if team == membership.str("team_id") {
				allowed = true
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}
