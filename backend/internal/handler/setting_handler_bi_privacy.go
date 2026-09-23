package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetBIPrivacyNotice(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	notice, err := h.settingService.GetBIPrivacyNotice(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !notice.Published() {
		response.NotFound(c, "BI privacy notice has not been published")
		return
	}
	response.Success(c, notice)
}
