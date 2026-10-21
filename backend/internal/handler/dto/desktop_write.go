package dto

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// BindDesktopWrite rejects fields that belong to a different business operation.
func BindDesktopWrite(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return desktopWriteDecodeError(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return desktopWriteDecodeError(err)
	}
	return nil
}

func desktopWriteDecodeError(err error) error {
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		return infraerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large")
	}
	return service.ErrDesktopValidation
}

func DesktopRequestID(c *gin.Context) string {
	value, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	return value
}
