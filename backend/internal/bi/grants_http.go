package bi

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AdminListGrants(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 100000 {
		adminRespond(c, nil, invalid("page", "Page must be between 1 and 100000"))
		return
	}
	items, err := h.service.ListGrants(c.Request.Context(), c.Param("organization_id"), (page-1)*50, 51)
	if err != nil {
		adminRespond(c, nil, err)
		return
	}
	hasMore := len(items) > 50
	if hasMore {
		items = items[:50]
	}
	adminRespond(c, gin.H{"items": items, "page": page, "has_more": hasMore, "capabilities": Capabilities}, nil)
}

func (h *Handler) AdminSaveGrant(c *gin.Context, actorUserID int64) {
	var input GrantInput
	if !decodeRequest(c, &input) {
		return
	}
	result, err := h.service.SaveGrant(c.Request.Context(), c.Param("organization_id"), input, actorUserID, c.GetString("bi.request_id"))
	adminRespond(c, result, err)
}

func (h *Handler) AdminRevokeGrant(c *gin.Context, actorUserID int64) {
	var input struct {
		ExpectedRevision *int64 `json:"expected_revision"`
	}
	if !decodeRequest(c, &input) {
		return
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		adminRespond(c, nil, invalid("expected_revision", "The current grant revision is required"))
		return
	}
	err := h.service.RevokeGrant(c.Request.Context(), c.Param("organization_id"), c.Param("manager_id"), *input.ExpectedRevision, actorUserID, c.GetString("bi.request_id"))
	adminRespond(c, gin.H{"revoked": true}, err)
}
