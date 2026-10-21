package dto

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopWriteRejectsUnknownFieldsTrailingBodiesAndPreservesSizeStatus(t *testing.T) {
	for _, body := range []string{`{"name":"Member","position_id":"hidden"}`, `{"name":"Member"} {}`, `{"name":`} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		var input struct {
			Name string `json:"name"`
		}
		require.ErrorIs(t, BindDesktopWrite(c, &input), service.ErrDesktopValidation)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"long text"}`))
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5)
	var input struct {
		Name string `json:"name"`
	}
	require.Equal(t, http.StatusRequestEntityTooLarge, infraerrors.Code(BindDesktopWrite(c, &input)))
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok"}`+strings.Repeat(" ", 64)))
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 24)
	require.Equal(t, http.StatusRequestEntityTooLarge, infraerrors.Code(BindDesktopWrite(c, &input)))
}
