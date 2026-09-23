package bi

import (
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateAnalysisContext(c *gin.Context) {
	var raw json.RawMessage
	if !decodeRequest(c, &raw) {
		return
	}
	if err := validateSchema("Filters", raw); err != nil {
		WriteError(c, err)
		return
	}
	var input Filters
	if err := json.Unmarshal(raw, &input); err != nil {
		WriteError(c, invalid("body", "Invalid analysis filters"))
		return
	}
	result, err := h.service.CreateAnalysisContext(c.Request.Context(), principal(c), c.Param("organization_id"), input)
	respond(c, 201, result, err)
}

func (h *Handler) GetAnalysisContext(c *gin.Context) {
	state, err := h.service.analysisContext(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("context_id"), "analytics:read")
	respond(c, 200, state.Context, err)
}

func (h *Handler) DataStatus(c *gin.Context) {
	org := c.Param("organization_id")
	_, err := h.service.AuthorizeOrganization(c.Request.Context(), principal(c), org, "analytics:read")
	if err != nil {
		WriteError(c, err)
		return
	}
	sources, err := readSourceStates(c.Request.Context(), h.service.db, org)
	period, _, _ := resolvePeriod("current", h.service.now())
	respond(c, 200, gin.H{"organization_id": org, "as_of": h.service.now().UTC(), "sources": sourceFreshness(sources, period)}, err)
}

func queryEnum(c *gin.Context, key, fallback string, allowed ...string) (string, error) {
	value := c.DefaultQuery(key, fallback)
	if !slices.Contains(allowed, value) {
		return "", invalid(key, "Unsupported parameter value")
	}
	return value, nil
}

func pageOperation(operation string, f *analysisFrame, query url.Values) string {
	copy := url.Values{}
	for key, values := range query {
		if key != "cursor" && key != "limit" {
			copy[key] = values
		}
	}
	scope, _ := json.Marshal(struct{ Operation, Organization, Context, Query string }{operation, f.state.Context.OrganizationID, f.state.Context.ID, copy.Encode()})
	return tokenHash(string(scope))
}

func analysisPage[T any](h *Handler, c *gin.Context, f *analysisFrame, operation string, items []T, validate func(T) error) (Page[T], error) {
	limit, ok := pageLimit(c)
	if !ok {
		return Page[T]{}, errAlreadyResponded
	}
	key := pageOperation(operation, f, c.Request.URL.Query())
	return snapshotPage(c.Request.Context(), h.service, f.state.Principal.UserID, key, limit, c.Query("cursor"), func() ([]T, error) { return items, nil }, func(values []T) error {
		for _, value := range values {
			if err := validate(value); err != nil {
				return err
			}
		}
		return nil
	})
}

var errAlreadyResponded = errors.New("response already written")

func (f *analysisFrame) pageVisible(kind, id string) error {
	allowed, err := f.visible(kind, id)
	if err != nil {
		return err
	}
	if !allowed {
		return apiError(403, "CONTEXT_REVOKED", "Pagination contains an entity whose access was revoked")
	}
	return nil
}

func (h *Handler) QueryAnalysis(operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("context_id")
		if !requestIDPattern.MatchString(id) {
			WriteError(c, invalid("context_id", "A valid analysis context is required"))
			return
		}
		capability := "analytics:read"
		if operation == "assets" {
			capability = "knowledge:read"
		}
		if operation == "members" || operation == "member" {
			capability = "members:read"
		}
		state, err := h.service.analysisContext(c.Request.Context(), principal(c), c.Param("organization_id"), id, capability)
		if err != nil {
			WriteError(c, err)
			return
		}
		var f *analysisFrame
		if operation == "assets" {
			f, err = h.service.loadMetadata(c.Request.Context(), state)
		} else {
			f, err = h.service.loadAnalysis(c.Request.Context(), state)
		}
		if err != nil {
			WriteError(c, err)
			return
		}
		value, err := h.analysisResult(c, f, operation)
		if errors.Is(err, errAlreadyResponded) {
			return
		}
		respond(c, 200, value, err)
	}
}

func (h *Handler) analysisResult(c *gin.Context, f *analysisFrame, operation string) (any, error) {
	switch operation {
	case "overview":
		return f.overview()
	case "adoption":
		return f.adoption(analysisSelection{}), nil
	case "assets":
		return f.assets(analysisSelection{})
	case "trend":
		bucket, err := queryEnum(c, "bucket", "day", "day", "seven_day")
		if err != nil {
			return nil, err
		}
		return f.trend(analysisSelection{}, bucket), nil
	case "tokens":
		result := f.tokenAnalysis(analysisSelection{})
		page, err := analysisPage(h, c, f, operation, result.Models, func(ModelTokens) error { return nil })
		result.Models, result.NextCursor, result.HasMore = page.Items, page.NextCursor, page.HasMore
		return result, err
	case "feedback":
		result, err := f.feedback(analysisSelection{})
		if err != nil {
			return nil, err
		}
		page, err := analysisPage(h, c, f, operation, result.ByApplication, func(item FeedbackAnalysisByApplicationItem) error {
			if item.Application == nil {
				return nil
			}
			return f.pageVisible("application", item.Application.ID)
		})
		result.ByApplication, result.NextCursor, result.HasMore = page.Items, page.NextCursor, page.HasMore
		return result, err
	case "members":
		return h.memberList(c, f)
	case "member":
		result, err := f.memberDetail(c.Param("member_id"))
		if err != nil {
			return nil, err
		}
		page, err := analysisPage(h, c, f, operation+":"+result.Member.ID, result.ApplicationUsage, func(item MemberDetailApplicationUsageItem) error {
			if item.Application == nil {
				return nil
			}
			return f.pageVisible("application", item.Application.ID)
		})
		result.ApplicationUsage, result.NextCursor, result.HasMore = page.Items, page.NextCursor, page.HasMore
		return result, err
	case "comparisons":
		dimension, err := queryEnum(c, "dimension", "team", "team", "role")
		if err != nil {
			return nil, err
		}
		rows, err := f.comparisons(dimension, c.Query("application_id"))
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item ComparisonRow) error { return f.pageVisible(dimension, item.Entity.ID) })
	case "team":
		id := c.Param("team_id")
		sel := analysisSelection{TeamID: id}
		if err := f.checkSelection(sel); err != nil {
			return nil, err
		}
		team := f.summary("team", id)
		if team == nil {
			return nil, ErrNotFound
		}
		return TeamDetail{Team: *team, Stats: f.statistics(sel).stats, Adoption: f.adoption(sel)}, nil
	case "applications":
		kind := c.Query("type")
		if kind != "" && !slices.Contains([]string{"tool", "agent", "skill", "workflow"}, kind) {
			return nil, invalid("type", "Unsupported application type")
		}
		rows, err := f.applicationRows(kind, c.Query("knowledge_id"))
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item ApplicationRow) error { return f.pageVisible("application", item.ID) })
	case "application":
		return f.applicationDetail(c.Param("application_id"))
	case "scenes":
		rows, err := f.sceneRows(c.Query("knowledge_id"))
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item SceneRow) error { return f.pageVisible("scene", item.ID) })
	case "scene":
		return f.sceneDetail(c.Param("scene_id"))
	default:
		return nil, ErrNotFound
	}
}

func (h *Handler) memberList(c *gin.Context, f *analysisFrame) (any, error) {
	query := c.Query("query")
	if utf8.RuneCountInString(query) > 100 {
		return nil, invalid("query", "Query must not exceed 100 characters")
	}
	rows, err := f.memberRows(query, analysisSelection{TeamID: c.Query("team_id"), RoleID: c.Query("role_id")})
	if err != nil {
		return nil, err
	}
	return analysisPage(h, c, f, "members", rows, func(item MemberRow) error {
		if err := f.pageVisible("member", item.ID); err != nil {
			return err
		}
		if item.Team != nil {
			if err := f.pageVisible("team", item.Team.ID); err != nil {
				return err
			}
		}
		if item.Role != nil {
			if err := f.pageVisible("role", item.Role.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (h *Handler) FilterOptions(c *gin.Context) {
	kind, err := queryEnum(c, "kind", "", "team", "role", "application", "scene")
	if err != nil {
		WriteError(c, err)
		return
	}
	var state analysisState
	if id := c.Query("context_id"); id != "" {
		state, err = h.service.analysisContext(c.Request.Context(), principal(c), c.Param("organization_id"), id, "analytics:read")
	} else {
		state, err = h.initialAnalysisState(c)
	}
	if err != nil {
		WriteError(c, err)
		return
	}
	f, err := h.service.loadMetadata(c.Request.Context(), state)
	if err != nil {
		WriteError(c, err)
		return
	}
	f.facts, err = f.periodFacts(analysisSelection{})
	if err != nil {
		WriteError(c, err)
		return
	}
	items := []FilterOption{}
	for id := range f.records[kind] {
		if !f.relevantOption(kind, id) {
			continue
		}
		if kind == "application" && state.Context.Filters.ApplicationID != "" && state.Context.Filters.ApplicationID != id {
			continue
		}
		if kind == "scene" && state.Context.Filters.SceneID != "" && state.Context.Filters.SceneID != id {
			continue
		}
		allowed, visibleErr := f.visible(kind, id)
		if visibleErr != nil {
			WriteError(c, visibleErr)
			return
		}
		if !allowed {
			continue
		}
		entity := f.summary(kind, id)
		if entity != nil {
			items = append(items, FilterOption{Kind: kind, Entity: *entity})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Entity.ID < items[j].Entity.ID })
	page, err := analysisPage(h, c, f, "filterOptions", items, func(item FilterOption) error { return f.pageVisible(kind, item.Entity.ID) })
	if errors.Is(err, errAlreadyResponded) {
		return
	}
	respond(c, 200, page, err)
}

func (h *Handler) initialAnalysisState(c *gin.Context) (analysisState, error) {
	p := principal(c)
	org := c.Param("organization_id")
	scope, err := h.service.AuthorizeOrganization(c.Request.Context(), p, org, "analytics:read")
	if err != nil {
		return analysisState{}, err
	}
	period, previous, _ := resolvePeriod("week", h.service.now())
	state := analysisState{Principal: p, Scope: scope, Context: AnalysisContext{ID: "initial", OrganizationID: org, Range: period, ComparisonRange: previous, Filters: Filters{Period: "week", TeamIDs: []string{}}}}
	err = h.service.db.QueryRowContext(c.Request.Context(), `SELECT COALESCE(MAX(id),0) FROM bi_data_revisions WHERE organization_id=$1 AND status='published'`, org).Scan(&state.Revision)
	return state, err
}

func (h *Handler) MetricDefinitions(c *gin.Context) {
	state, err := h.initialAnalysisState(c)
	if err != nil {
		WriteError(c, err)
		return
	}
	f := &analysisFrame{state: state}
	page, err := analysisPage(h, c, f, "metricDefinitions", metricDefinitions(), func(MetricDefinition) error { return nil })
	if errors.Is(err, errAlreadyResponded) {
		return
	}
	respond(c, 200, page, err)
}
