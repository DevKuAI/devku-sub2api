package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TiboResetMonitorHandler struct {
	monitor *service.TiboResetMonitorService
}

func NewTiboResetMonitorHandler(monitor *service.TiboResetMonitorService) *TiboResetMonitorHandler {
	return &TiboResetMonitorHandler{monitor: monitor}
}

func (h *TiboResetMonitorHandler) Get(c *gin.Context) {
	if h == nil || h.monitor == nil {
		response.Error(c, http.StatusServiceUnavailable, "Tibo reset monitor is not enabled")
		return
	}
	result, err := h.monitor.Get(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusBadGateway, "Failed to load Tibo reset monitor")
		return
	}
	response.Success(c, result)
}
