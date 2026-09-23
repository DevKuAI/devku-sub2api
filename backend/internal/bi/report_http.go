package bi

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateReport(c *gin.Context) {
	var raw json.RawMessage
	if !decodeRequest(c, &raw) {
		return
	}
	if err := validateSchema("CreateReport", raw); err != nil {
		WriteError(c, err)
		return
	}
	var input struct {
		ContextID string  `json:"context_id"`
		Title     *string `json:"title"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		WriteError(c, invalid("body", "Invalid report request"))
		return
	}
	result, err := h.service.CreateReport(c.Request.Context(), principal(c), c.Param("organization_id"), c.GetHeader("Idempotency-Key"), input.ContextID, input.Title)
	respond(c, 202, result, err)
}

func (h *Handler) GetReport(c *gin.Context) {
	result, err := h.service.ReadReport(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("report_id"), "reports:read")
	respond(c, 200, result, err)
}

func (h *Handler) GetReportText(c *gin.Context) {
	p := principal(c)
	org, id := c.Param("organization_id"), c.Param("report_id")
	if _, err := h.service.AuthorizeOrganization(c.Request.Context(), p, org, "reports:copy"); err != nil {
		WriteError(c, err)
		return
	}
	report, err := h.service.ReadReport(c.Request.Context(), p, org, id, "reports:copy")
	if err != nil {
		WriteError(c, err)
		return
	}
	if report.Report.Status != "ready" || report.Text == nil {
		WriteError(c, apiError(409, "CONFLICT", "Report is not ready"))
		return
	}
	stored, err := readStoredReport(c.Request.Context(), h.service.db, org, id)
	if err != nil {
		WriteError(c, err)
		return
	}
	respond(c, 200, ReportText{ReportID: id, Text: *report.Text, MimeType: "text/plain", GeneratedAt: stored.GeneratedAt.UTC().Format(time.RFC3339Nano)}, nil)
}

func (h *Handler) CreateShare(c *gin.Context) {
	var raw json.RawMessage
	if !decodeRequest(c, &raw) {
		return
	}
	if err := validateSchema("CreateShare", raw); err != nil {
		WriteError(c, err)
		return
	}
	var input struct {
		Seconds int64 `json:"expires_in_seconds"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		WriteError(c, invalid("body", "Invalid share request"))
		return
	}
	result, err := h.service.CreateShare(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("report_id"), c.GetHeader("Idempotency-Key"), input.Seconds)
	respond(c, 201, result, err)
}

func (h *Handler) DeleteShare(c *gin.Context) {
	err := h.service.RevokeShare(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("share_id"))
	respond(c, 204, nil, err)
}
func (h *Handler) ResolveShare(c *gin.Context) {
	result, err := h.service.ResolveShare(c.Request.Context(), principal(c), c.Param("share_id"))
	respond(c, 200, result, err)
}

func (h *Handler) ListReports(c *gin.Context) {
	p, org := principal(c), c.Param("organization_id")
	items, err := h.service.reportSummaries(c.Request.Context(), p, org)
	if err != nil {
		WriteError(c, err)
		return
	}
	f := &analysisFrame{state: analysisState{Principal: p, Context: AnalysisContext{ID: "archive", OrganizationID: org}}}
	page, err := analysisPage(h, c, f, "reports", items, func(item ReportSummary) error {
		_, err := h.service.ReadReport(c.Request.Context(), p, org, item.ID, "reports:read")
		if errors.Is(err, ErrNotFound) {
			return apiError(403, "CONTEXT_REVOKED", "Report access changed; reload the list")
		}
		return err
	})
	if errors.Is(err, errAlreadyResponded) {
		return
	}
	respond(c, 200, page, err)
}

func (h *Handler) ListShares(c *gin.Context) {
	p, org, id := principal(c), c.Param("organization_id"), c.Param("report_id")
	items, err := h.service.reportShares(c.Request.Context(), p, org, id)
	if err != nil {
		WriteError(c, err)
		return
	}
	f := &analysisFrame{state: analysisState{Principal: p, Context: AnalysisContext{ID: "archive", OrganizationID: org}}}
	page, err := analysisPage(h, c, f, "shares:"+id, items, func(item Share) error {
		var exists bool
		err := h.service.db.QueryRowContext(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM bi_report_shares WHERE id=$1 AND manager_id=$2 AND organization_id=$3 AND report_id=$4 AND revoked_at IS NULL AND expires_at>$5)`, item.ID, p.ManagerID, org, id, h.service.now().UTC()).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return apiError(403, "CONTEXT_REVOKED", "Share list changed; reload the list")
		}
		return nil
	})
	if errors.Is(err, errAlreadyResponded) {
		return
	}
	respond(c, 200, page, err)
}
