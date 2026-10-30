package accounting

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) registerVendors(g *gin.RouterGroup) {
	g.GET("/vendors", h.listVendors)
	g.GET("/vendors/:id", h.vendor)
	g.POST("/vendors", h.createVendor)
	g.PATCH("/vendors/:id", h.updateVendor)
}

func (h *Handler) listVendors(c *gin.Context) {
	vendors, err := h.engine.ListVendors(c.Request.Context(), c.Query("kind"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, vendors)
}

func (h *Handler) vendor(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	v, err := getVendor(c.Request.Context(), h.engine.db, c.Param("id"), false)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, v)
}

func (h *Handler) createVendor(c *gin.Context) {
	var in VendorInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Global: true, Kind: "vendors", Action: "create", Payload: in}, func(tx *sql.Tx) (any, error) {
		return h.engine.CreateVendor(c.Request.Context(), tx, in, actor(c))
	})
}

func (h *Handler) updateVendor(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	var in VendorInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	id := c.Param("id")
	h.run(c, Operation{Global: true, Kind: "vendors", Object: id, Action: "update", Payload: in}, func(tx *sql.Tx) (any, error) {
		return h.engine.UpdateVendor(c.Request.Context(), tx, id, in)
	})
}
