package bi

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func (h *Handler) PutFavorite(c *gin.Context) {
	err := h.service.SetFavorite(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("knowledge_id"))
	respond(c, 200, gin.H{"ok": true}, err)
}
func (h *Handler) DeleteFavorite(c *gin.Context) {
	err := h.service.RemoveFavorite(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("knowledge_id"))
	respond(c, 204, nil, err)
}

func (h *Handler) GetNote(c *gin.Context) {
	note, err := h.service.ReadNote(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("knowledge_id"))
	if err == nil {
		c.Header("ETag", `"`+note.Revision+`"`)
	}
	respond(c, 200, note, err)
}

func (h *Handler) PutNote(c *gin.Context) {
	var raw json.RawMessage
	if !decodeRequest(c, &raw) {
		return
	}
	if err := validateSchema("SaveNote", raw); err != nil {
		WriteError(c, err)
		return
	}
	var input struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		WriteError(c, invalid("body", "Invalid note"))
		return
	}
	note, err := h.service.mutateNote(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("knowledge_id"), input.Text, c.GetHeader("If-Match"), c.GetHeader("If-None-Match"), false)
	if err == nil {
		c.Header("ETag", `"`+note.Revision+`"`)
	}
	respond(c, 200, note, err)
}

func (h *Handler) DeleteNote(c *gin.Context) {
	_, err := h.service.mutateNote(c.Request.Context(), principal(c), c.Param("organization_id"), c.Param("knowledge_id"), "", c.GetHeader("If-Match"), c.GetHeader("If-None-Match"), true)
	respond(c, 204, nil, err)
}
