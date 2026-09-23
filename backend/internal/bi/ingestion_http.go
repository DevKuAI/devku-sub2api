package bi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const connectorKey = "bi.connector"

func (h *Handler) ConnectorAuth(c *gin.Context) {
	connector, err := h.service.AuthorizeConnector(c.Request.Context(), bearerToken(c.GetHeader("Authorization")))
	if err != nil {
		WriteError(c, err)
		return
	}
	c.Set(connectorKey, connector)
	c.Next()
}

func connectorPrincipal(c *gin.Context) Connector {
	value, _ := c.Get(connectorKey)
	connector, _ := value.(Connector)
	return connector
}

func (h *Handler) SubmitImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(c, apiError(413, "PAYLOAD_TOO_LARGE", "Import payload exceeds 2 MiB"))
		} else {
			WriteError(c, invalid("body", "Unable to read import body"))
		}
		return
	}
	job, err := h.service.SubmitImport(c.Request.Context(), connectorPrincipal(c), c.GetHeader("Idempotency-Key"), raw)
	respond(c, 202, job, err)
}

func (h *Handler) GetImport(c *gin.Context) {
	job, err := h.service.ImportJob(c.Request.Context(), connectorPrincipal(c), c.Param("batch_id"))
	respond(c, 200, job, err)
}

func (h *Handler) GetCheckpoint(c *gin.Context) {
	checkpoint, err := h.service.ImportCheckpoint(c.Request.Context(), connectorPrincipal(c), c.Param("source_id"))
	respond(c, 200, checkpoint, err)
}
