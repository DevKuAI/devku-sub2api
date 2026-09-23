package bi

import (
	"errors"
	"slices"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func queryBool(c *gin.Context, key string) (*bool, error) {
	raw, exists := c.GetQuery(key)
	if !exists {
		return nil, nil
	}
	if raw == "true" {
		return ptr(true), nil
	}
	if raw == "false" {
		return ptr(false), nil
	}
	return nil, invalid(key, "Expected true or false")
}

func (h *Handler) QueryContent(operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("context_id")
		if !requestIDPattern.MatchString(id) {
			WriteError(c, invalid("context_id", "A valid analysis context is required"))
			return
		}
		capability := "knowledge:read"
		if operation == "source" {
			capability = "analytics:read"
		}
		if slices.Contains([]string{"evaluations", "evaluation", "samples"}, operation) {
			capability = "evaluations:read"
		}
		state, err := h.service.analysisContext(c.Request.Context(), principal(c), c.Param("organization_id"), id, capability)
		if err != nil {
			WriteError(c, err)
			return
		}
		f, err := h.service.loadMetadata(c.Request.Context(), state)
		if err != nil {
			WriteError(c, err)
			return
		}
		result, err := h.contentResult(c, f, operation)
		if errors.Is(err, errAlreadyResponded) {
			return
		}
		respond(c, 200, result, err)
	}
}

func (f *analysisFrame) contentPageVisible(kind, id string) error {
	var err error
	switch kind {
	case "knowledge":
		_, err = f.readableKnowledge(id)
	case "knowledge_version":
		_, err = f.readableVersion("", id)
	case "case":
		_, err = f.readableCase(id)
	case "evaluation":
		_, err = f.readableEvaluation(id)
	default:
		return f.pageVisible(kind, id)
	}
	if errors.Is(err, ErrNotFound) {
		return apiError(403, "CONTEXT_REVOKED", "Content access changed; reload the snapshot")
	}
	return err
}

func (h *Handler) contentResult(c *gin.Context, f *analysisFrame, operation string) (any, error) {
	knowledgeID := c.Param("knowledge_id")
	switch operation {
	case "knowledge":
		status := c.Query("status")
		if status != "" && !slices.Contains([]string{"valid", "draft", "expired"}, status) {
			return nil, invalid("status", "Unsupported knowledge status")
		}
		reused, err := queryBool(c, "reused")
		if err != nil {
			return nil, err
		}
		stale, err := queryBool(c, "stale_referenced")
		if err != nil {
			return nil, err
		}
		rows, err := f.knowledgeRows(status, reused, stale)
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item KnowledgeRow) error { return f.contentPageVisible("knowledge", item.ID) })
	case "knowledge_detail":
		return f.knowledgeDetail(knowledgeID)
	case "versions":
		rows, err := f.knowledgeVersions(knowledgeID)
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation+":"+knowledgeID, rows, func(item VersionSummary) error { return f.contentPageVisible("knowledge_version", item.ID) })
	case "version":
		return f.knowledgeVersion(knowledgeID, c.Param("version_id"))
	case "references":
		rows, err := f.knowledgeEvidence(knowledgeID)
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation+":"+knowledgeID, rows, func(item ReferenceEvidence) error {
			if err := f.contentPageVisible("knowledge_version", item.KnowledgeVersionID); err != nil {
				return err
			}
			if item.Application != nil {
				if err := f.pageVisible("application", item.Application.ID); err != nil {
					return err
				}
			}
			if item.Team != nil {
				if err := f.pageVisible("team", item.Team.ID); err != nil {
					return err
				}
			}
			return nil
		})
	case "cases":
		rows, err := f.caseRows(c.Query("knowledge_id"))
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item CaseRow) error { return f.contentPageVisible("case", item.ID) })
	case "case":
		return f.caseDetail(c.Param("case_id"))
	case "source":
		kind, err := queryEnum(c, "parent_kind", "", "knowledge_version", "case")
		if err != nil {
			return nil, err
		}
		if !requestIDPattern.MatchString(c.Query("parent_id")) {
			return nil, invalid("parent_id", "Parent identity is required")
		}
		return f.sourceDetail(kind, c.Query("parent_id"), c.Param("source_id"))
	case "evaluations":
		rows, err := f.evaluationRows(c.Query("application_id"))
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item EvaluationRow) error { return f.contentPageVisible("evaluation", item.ID) })
	case "evaluation":
		return f.evaluationDetail(c.Param("evaluation_id"))
	case "samples":
		id := c.Param("evaluation_id")
		_, rows, err := f.evaluationSamples(id)
		if err != nil {
			return nil, err
		}
		allowed := map[string]bool{}
		for _, item := range rows {
			allowed[item.ID] = true
		}
		return analysisPage(h, c, f, operation+":"+id, rows, func(item EvaluationSample) error {
			if !allowed[item.ID] {
				return apiError(403, "CONTEXT_REVOKED", "Sample access was revoked")
			}
			return nil
		})
	case "search":
		kind, err := queryEnum(c, "type", "all", "all", "knowledge", "case", "application")
		if err != nil {
			return nil, err
		}
		query := c.Query("query")
		if utf8.RuneCountInString(query) > 100 {
			return nil, invalid("query", "Query must not exceed 100 characters")
		}
		rows, err := f.searchContent(query, kind)
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item SearchHit) error { return f.contentPageVisible(item.Target.Kind, item.Target.ID) })
	case "favorites":
		rows, err := f.favorites()
		if err != nil {
			return nil, err
		}
		return analysisPage(h, c, f, operation, rows, func(item Favorite) error { return f.contentPageVisible("knowledge", item.Knowledge.ID) })
	default:
		return nil, ErrNotFound
	}
}
