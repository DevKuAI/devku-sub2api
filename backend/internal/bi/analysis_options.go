package bi

func (f *analysisFrame) relevantOption(kind, id string) bool {
	filters := f.state.Context.Filters
	switch kind {
	case "application":
		r := f.records[kind][id]
		if r == nil {
			return false
		}
		if filters.ApplicationID != "" && filters.ApplicationID != id {
			return false
		}
		if filters.SceneID != "" && !recordListContains(r, "scene_ids", filters.SceneID) {
			return false
		}
		teams := r.strings("team_ids")
		if len(teams) == 0 {
			return true
		}
		for _, team := range teams {
			if f.state.teamAllowed(team) {
				return true
			}
		}
		if len(f.cohort(f.state.Context.Range, analysisSelection{ApplicationID: id})) > 0 {
			return true
		}
		for _, u := range f.facts {
			if eventWithin(u, f.state.Context.Range) && f.matchesUsage(u, analysisSelection{ApplicationID: id}, true) {
				return true
			}
		}
		return false
	case "scene":
		if filters.SceneID != "" && filters.SceneID != id {
			return false
		}
		if filters.ApplicationID != "" {
			app := f.records["application"][filters.ApplicationID]
			return app != nil && recordListContains(app, "scene_ids", id)
		}
		if f.state.Scope.AllTeams && len(filters.TeamIDs) == 0 {
			return true
		}
		for appID, app := range f.records["application"] {
			if recordListContains(app, "scene_ids", id) && f.relevantOption("application", appID) {
				return true
			}
		}
		for _, u := range f.facts {
			if eventWithin(u, f.state.Context.Range) && f.matchesUsage(u, analysisSelection{SceneID: id}, true) {
				return true
			}
		}
		return len(f.cohort(f.state.Context.Range, analysisSelection{SceneID: id})) > 0
	case "team", "role":
		if filters.ApplicationID == "" && filters.SceneID == "" {
			return true
		}
		_, end := f.state.Context.Range.bounds()
		for member := range f.cohort(f.state.Context.Range, analysisSelection{}) {
			membership := f.membership(member, end.Add(-1))
			if membership != nil && membership.str(kind+"_id") == id {
				return true
			}
		}
		for _, u := range f.facts {
			if !eventWithin(u, f.state.Context.Range) || !f.matchesUsage(u, analysisSelection{}, true) {
				continue
			}
			if kind == "team" && stringValue(u.TeamID) == id {
				return true
			}
			if kind == "role" {
				m := f.membership(stringValue(u.MemberID), u.OccurredAt)
				if m != nil && m.str("role_id") == id {
					return true
				}
			}
		}
		return false
	}
	return true
}
