package desktopresponse

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type successEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error desktopError `json:"error"`
}

type desktopError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Retryable bool              `json:"retryable"`
	Details   map[string]string `json:"details"`
}

func Success(c *gin.Context, data any) {
	SetHeaders(c)
	c.JSON(http.StatusOK, successEnvelope{Data: data})
}

func Error(c *gin.Context, err error) {
	SetHeaders(c)
	statusCode, status := infraerrors.ToHTTP(err)
	details := status.Metadata
	if details == nil {
		details = map[string]string{}
	}
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	c.JSON(statusCode, errorEnvelope{Error: desktopError{
		Code: status.Reason, Message: status.Message, RequestID: requestID,
		Retryable: statusCode >= 500 || errors.Is(err, service.ErrDesktopRateLimited), Details: details,
	}})
}

func PayloadTooLarge(c *gin.Context) {
	Error(c, infraerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large"))
}

func SetRetryAfter(c *gin.Context, err error) {
	status := infraerrors.FromError(err)
	if value := status.Metadata["retry_after"]; value != "" {
		if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds > 0 {
			c.Header("Retry-After", strconv.Itoa(seconds))
		}
	}
}

func SetHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if requestID == "" {
		requestID = uuid.NewString()
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID))
	}
	c.Header("X-Request-ID", requestID)
}
