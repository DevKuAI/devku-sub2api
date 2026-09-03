package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	desktopManagedMemberCreateIdempotencyScope     = "desktop.organization.members.create"
	desktopManagedModelTokenRotateIdempotencyScope = "desktop.organization.model_tokens.rotate"
)

type desktopManagedOrganizationRequest struct {
	Name   *string `json:"name"`
	Status *string `json:"status"`
}

type desktopManagedTargetConfigRequest struct {
	TargetConfig service.DesktopTargetConfig `json:"target_config" binding:"required"`
}

type desktopManagedCreateMemberRequest struct {
	Name  string `json:"name" binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

type desktopManagedUpdateMemberRequest struct {
	Name   *string `json:"name"`
	Phone  *string `json:"phone"`
	Status *string `json:"status"`
}

type desktopManagedOrganizationDTO struct {
	PublicID             string                       `json:"public_id"`
	Code                 string                       `json:"code"`
	Name                 string                       `json:"name"`
	Status               string                       `json:"status"`
	GatewayUser          desktopManagedGatewayUserDTO `json:"gateway_user"`
	Group                desktopManagedGroupDTO       `json:"group"`
	MemberCount          int                          `json:"member_count"`
	TargetConfigAssigned bool                         `json:"target_config_assigned"`
	TargetConfig         *service.DesktopTargetConfig `json:"target_config,omitempty"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
}

type desktopManagedGatewayUserDTO struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type desktopManagedGroupDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type desktopManagedMemberDTO struct {
	PublicID         string                      `json:"public_id"`
	Name             string                      `json:"name"`
	Phone            string                      `json:"phone"`
	Status           string                      `json:"status"`
	ModelTokenStatus string                      `json:"model_token_status"`
	Usage            *service.DesktopMemberUsage `json:"usage,omitempty"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

func (h *DesktopHandler) GetManagedOrganization(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	organization, err := h.desktop.GetManagedOrganization(c.Request.Context(), userID)
	if errors.Is(err, service.ErrDesktopOrganizationNotFound) {
		c.Status(http.StatusNoContent)
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopManagedOrganizationFromService(organization))
}

func (h *DesktopHandler) UpdateManagedOrganization(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	var req desktopManagedOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopManagedBindingError(c, err)
		return
	}
	organization, err := h.desktop.UpdateManagedOrganization(c.Request.Context(), userID, service.DesktopUpdateOrganizationInput{
		Name: req.Name, Status: req.Status,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopManagedOrganizationFromService(organization))
}

func (h *DesktopHandler) UpdateManagedModelConfiguration(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	var req desktopManagedTargetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopManagedBindingError(c, err)
		return
	}
	organization, err := h.desktop.UpdateManagedTargetConfig(c.Request.Context(), userID, &req.TargetConfig)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopManagedOrganizationFromService(organization))
}

func (h *DesktopHandler) CreateManagedMember(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	var req desktopManagedCreateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopManagedBindingError(c, err)
		return
	}
	executeUserIdempotentJSON(c, desktopManagedMemberCreateIdempotencyScope, req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		member, err := h.desktop.CreateManagedMember(ctx, userID, req.Name, req.Phone)
		if err != nil {
			return nil, err
		}
		return desktopManagedMemberFromService(member), nil
	})
}

func (h *DesktopHandler) ListManagedMembers(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.desktop.ListManagedMembersWithUsage(c.Request.Context(), userID, pagination.PaginationParams{
		Page: page, PageSize: pageSize,
	}, service.DesktopMemberListFilters{
		Search: c.Query("search"), Status: c.Query("status"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	dtos := make([]desktopManagedMemberDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, desktopManagedMemberFromService(&items[i]))
	}
	c.Header("Cache-Control", "no-store")
	response.Paginated(c, dtos, result.Total, result.Page, result.PageSize)
}

func (h *DesktopHandler) UpdateManagedMember(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	var req desktopManagedUpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopManagedBindingError(c, err)
		return
	}
	member, err := h.desktop.UpdateManagedMember(c.Request.Context(), userID, c.Param("member_id"), service.DesktopUpdateMemberInput{
		Name: req.Name, Status: req.Status,
	}, req.Phone)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopManagedMemberFromService(member))
}

func (h *DesktopHandler) DeleteManagedMember(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	if err := h.desktop.DeleteManagedMember(c.Request.Context(), userID, c.Param("member_id")); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *DesktopHandler) RotateManagedModelToken(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	payload := struct {
		MemberID string `json:"member_id"`
	}{MemberID: c.Param("member_id")}
	executeUserIdempotentJSON(c, desktopManagedModelTokenRotateIdempotencyScope, payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		member, err := h.desktop.RotateManagedMemberAPIKey(ctx, userID, payload.MemberID)
		if err != nil {
			return nil, err
		}
		return desktopManagedMemberFromService(member), nil
	})
}

func desktopManagedUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, service.ErrDesktopUnauthenticated)
		return 0, false
	}
	return subject.UserID, true
}

func desktopManagedOrganizationFromService(value *service.DesktopOrganization) desktopManagedOrganizationDTO {
	return desktopManagedOrganizationDTO{
		PublicID: value.PublicID, Code: value.Code, Name: value.Name, Status: value.Status,
		GatewayUser: desktopManagedGatewayUserDTO{ID: value.GatewayUserID, Email: value.GatewayUserEmail, Username: value.GatewayUserName},
		Group:       desktopManagedGroupDTO{ID: value.GroupID, Name: value.GroupName}, MemberCount: value.MemberCount,
		TargetConfigAssigned: value.TargetConfigAssigned, TargetConfig: value.TargetConfig,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func desktopManagedMemberFromService(value *service.DesktopMember) desktopManagedMemberDTO {
	return desktopManagedMemberDTO{
		PublicID: value.PublicID, Name: value.Name, Phone: value.Phone, Status: value.Status,
		ModelTokenStatus: value.ModelTokenStatus(), Usage: value.Usage, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func desktopManagedBindingError(c *gin.Context, err error) {
	if middleware2.IsBodyTooLarge(err) {
		response.ErrorFrom(c, infraerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large"))
		return
	}
	response.ErrorFrom(c, service.ErrDesktopValidation.WithMetadata(map[string]string{"binding": strings.TrimSpace(err.Error())}))
}
