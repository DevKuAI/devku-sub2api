package admin

import (
	"net/http"

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
	response.Success(c, notice)
}

func (h *SettingHandler) UpdateBIPrivacyNotice(c *gin.Context) {
	var input struct {
		SiteURL   string `json:"site_url"`
		Title     string `json:"title"`
		Version   string `json:"version"`
		ContentMD string `json:"content_md"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid privacy notice payload")
		return
	}
	notice, err := h.settingService.UpdateBIPrivacyNotice(c.Request.Context(), input.Title, input.Version, input.ContentMD, input.SiteURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, notice)
}
