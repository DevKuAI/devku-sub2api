package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	desktopUpdateCreateIdempotencyScope   = "admin.desktop.updates.create"
	desktopUpdatePublishIdempotencyScope  = "admin.desktop.updates.publish"
	desktopUpdateWithdrawIdempotencyScope = "admin.desktop.updates.withdraw"
)

type DesktopUpdateHandler struct {
	updates *service.DesktopUpdateService
}

func NewDesktopUpdateHandler(updates *service.DesktopUpdateService) *DesktopUpdateHandler {
	return &DesktopUpdateHandler{updates: updates}
}

type desktopUpdateDraftRequest struct {
	Version   string                         `json:"version" binding:"required"`
	Notes     string                         `json:"notes"`
	Artifacts service.DesktopUpdateArtifacts `json:"artifacts" binding:"required"`
}

type desktopUpdateWithdrawRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type desktopUpdateDTO struct {
	PublicID         string                         `json:"public_id"`
	Version          string                         `json:"version"`
	Notes            string                         `json:"notes"`
	Artifacts        service.DesktopUpdateArtifacts `json:"artifacts"`
	Status           string                         `json:"status"`
	CreatedBy        *int64                         `json:"created_by,omitempty"`
	UpdatedBy        *int64                         `json:"updated_by,omitempty"`
	PublishedBy      *int64                         `json:"published_by,omitempty"`
	WithdrawnBy      *int64                         `json:"withdrawn_by,omitempty"`
	PublishedAt      *time.Time                     `json:"published_at,omitempty"`
	WithdrawnAt      *time.Time                     `json:"withdrawn_at,omitempty"`
	WithdrawalReason *string                        `json:"withdrawal_reason,omitempty"`
	CreatedAt        time.Time                      `json:"created_at"`
	UpdatedAt        time.Time                      `json:"updated_at"`
}

func (h *DesktopUpdateHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.updates.List(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, service.DesktopUpdateListFilters{Status: c.Query("status")})
	if response.ErrorFrom(c, err) {
		return
	}
	dtos := make([]desktopUpdateDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, desktopUpdateFromService(&items[i]))
	}
	response.Paginated(c, dtos, result.Total, result.Page, result.PageSize)
}

func (h *DesktopUpdateHandler) Get(c *gin.Context) {
	release, err := h.updates.Get(c.Request.Context(), c.Param("release_id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopUpdateFromService(release))
}

func (h *DesktopUpdateHandler) Create(c *gin.Context) {
	var req desktopUpdateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	input := service.DesktopUpdateDraftInput{Version: req.Version, Notes: req.Notes, Artifacts: req.Artifacts, ActorID: adminDesktopUpdateActorID(c)}
	result, err := executeAdminStrictIdempotent(c, desktopUpdateCreateIdempotencyScope, req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		release, execErr := h.updates.CreateDraft(ctx, input)
		if execErr != nil {
			return nil, execErr
		}
		return desktopUpdateFromService(release), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, true)
}

func (h *DesktopUpdateHandler) Update(c *gin.Context) {
	var req desktopUpdateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	release, err := h.updates.UpdateDraft(c.Request.Context(), c.Param("release_id"), service.DesktopUpdateDraftInput{
		Version: req.Version, Notes: req.Notes, Artifacts: req.Artifacts, ActorID: adminDesktopUpdateActorID(c),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopUpdateFromService(release))
}

func (h *DesktopUpdateHandler) UploadArtifact(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	}
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.ErrorFrom(c, service.ErrDesktopUpdateUploadTooLarge)
			return
		}
		adminDesktopBindingError(c, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "file"}))
		return
	}
	defer func() { _ = file.Close() }()

	artifact, err := h.updates.UploadArtifact(
		c.Request.Context(), c.Param("release_id"), c.Param("platform"), fileHeader.Filename,
		fileHeader.Header.Get("Content-Type"), fileHeader.Size, file,
	)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, artifact)
}

func (h *DesktopUpdateHandler) Publish(c *gin.Context) {
	payload := struct {
		ReleaseID string `json:"release_id"`
	}{ReleaseID: strings.TrimSpace(c.Param("release_id"))}
	result, err := executeAdminStrictIdempotent(c, desktopUpdatePublishIdempotencyScope, payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		release, execErr := h.updates.Publish(ctx, payload.ReleaseID, adminDesktopUpdateActorID(c))
		if execErr != nil {
			return nil, execErr
		}
		return desktopUpdateFromService(release), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, false)
}

func (h *DesktopUpdateHandler) Withdraw(c *gin.Context) {
	var req desktopUpdateWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	payload := struct {
		ReleaseID string `json:"release_id"`
		Reason    string `json:"reason"`
	}{ReleaseID: strings.TrimSpace(c.Param("release_id")), Reason: strings.TrimSpace(req.Reason)}
	result, err := executeAdminStrictIdempotent(c, desktopUpdateWithdrawIdempotencyScope, payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		release, execErr := h.updates.Withdraw(ctx, payload.ReleaseID, adminDesktopUpdateActorID(c), payload.Reason)
		if execErr != nil {
			return nil, execErr
		}
		return desktopUpdateFromService(release), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, false)
}

func adminDesktopUpdateActorID(c *gin.Context) int64 {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}

func desktopUpdateFromService(value *service.DesktopUpdateRelease) desktopUpdateDTO {
	return desktopUpdateDTO{
		PublicID: value.PublicID, Version: value.Version, Notes: value.Notes, Artifacts: value.Artifacts,
		Status: value.Status, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		PublishedBy: value.PublishedBy, WithdrawnBy: value.WithdrawnBy,
		PublishedAt: value.PublishedAt, WithdrawnAt: value.WithdrawnAt,
		WithdrawalReason: value.WithdrawalReason, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
