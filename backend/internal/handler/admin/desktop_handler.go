package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopHandler struct {
	desktop *service.DesktopService
}

const (
	desktopOrganizationCreateIdempotencyScope = "admin.desktop.organizations.create"
	desktopMemberCreateIdempotencyScope       = "admin.desktop.members.create"
	desktopModelTokenRotateIdempotencyScope   = "admin.desktop.model_tokens.rotate"
)

func NewDesktopHandler(desktop *service.DesktopService) *DesktopHandler {
	return &DesktopHandler{desktop: desktop}
}

type desktopCreateOrganizationRequest struct {
	Code          string `json:"code" binding:"required"`
	Name          string `json:"name" binding:"required"`
	GatewayUserID int64  `json:"gateway_user_id" binding:"required"`
	GroupID       int64  `json:"group_id" binding:"required"`
}

type desktopUpdateOrganizationRequest struct {
	Name          *string `json:"name"`
	Status        *string `json:"status"`
	GatewayUserID *int64  `json:"gateway_user_id"`
	GroupID       *int64  `json:"group_id"`
}

type desktopTargetConfigRequest struct {
	TargetConfig service.DesktopTargetConfig `json:"target_config" binding:"required"`
}

type desktopCreateMemberRequest struct {
	Name  string `json:"name" binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

type desktopUpdateMemberRequest struct {
	Name   *string `json:"name"`
	Phone  *string `json:"phone"`
	Status *string `json:"status"`
}

type desktopOrganizationDTO struct {
	PublicID             string                       `json:"public_id"`
	Code                 string                       `json:"code"`
	Name                 string                       `json:"name"`
	Status               string                       `json:"status"`
	GatewayUser          desktopGatewayUserDTO        `json:"gateway_user"`
	Group                desktopGroupDTO              `json:"group"`
	MemberCount          int                          `json:"member_count"`
	TargetConfigAssigned bool                         `json:"target_config_assigned"`
	TargetConfig         *service.DesktopTargetConfig `json:"target_config,omitempty"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
}

type desktopGatewayUserDTO struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type desktopGroupDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type desktopMemberDTO struct {
	PublicID         string                      `json:"public_id"`
	Name             string                      `json:"name"`
	Phone            string                      `json:"phone"`
	Status           string                      `json:"status"`
	ModelTokenStatus string                      `json:"model_token_status"`
	Usage            *service.DesktopMemberUsage `json:"usage,omitempty"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

func (h *DesktopHandler) CreateOrganization(c *gin.Context) {
	var req desktopCreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	result, err := executeAdminIdempotent(c, desktopOrganizationCreateIdempotencyScope, req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		organization, execErr := h.desktop.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
			Code: req.Code, Name: req.Name, GatewayUserID: req.GatewayUserID, GroupID: req.GroupID,
		})
		if execErr != nil {
			return nil, execErr
		}
		return desktopOrganizationFromService(organization, true), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, true)
}

func (h *DesktopHandler) ListOrganizations(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.desktop.ListOrganizations(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, service.DesktopOrganizationListFilters{
		Search: c.Query("search"), Status: c.Query("status"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	dtos := make([]desktopOrganizationDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, desktopOrganizationFromService(&items[i], false))
	}
	response.Paginated(c, dtos, result.Total, result.Page, result.PageSize)
}

func (h *DesktopHandler) GetOrganization(c *gin.Context) {
	organization, err := h.desktop.GetOrganization(c.Request.Context(), c.Param("organization_id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopOrganizationFromService(organization, true))
}

func (h *DesktopHandler) UpdateOrganization(c *gin.Context) {
	var req desktopUpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	organization, err := h.desktop.UpdateOrganization(c.Request.Context(), c.Param("organization_id"), service.DesktopUpdateOrganizationInput{
		Name: req.Name, Status: req.Status, GatewayUserID: req.GatewayUserID, GroupID: req.GroupID,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopOrganizationFromService(organization, true))
}

func (h *DesktopHandler) UpdateModelConfiguration(c *gin.Context) {
	var req desktopTargetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	organization, err := h.desktop.UpdateTargetConfig(c.Request.Context(), c.Param("organization_id"), &req.TargetConfig)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopOrganizationFromService(organization, true))
}

func (h *DesktopHandler) CreateMember(c *gin.Context) {
	var req desktopCreateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	organizationID := c.Param("organization_id")
	payload := struct {
		OrganizationID string                     `json:"organization_id"`
		Member         desktopCreateMemberRequest `json:"member"`
	}{OrganizationID: organizationID, Member: req}
	result, err := executeAdminIdempotent(c, desktopMemberCreateIdempotencyScope, payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		member, execErr := h.desktop.CreateMember(ctx, organizationID, req.Name, req.Phone)
		if execErr != nil {
			return nil, execErr
		}
		return desktopMemberFromService(member), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, true)
}

func (h *DesktopHandler) ListMembers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.desktop.ListMembersWithUsage(c.Request.Context(), c.Param("organization_id"), pagination.PaginationParams{Page: page, PageSize: pageSize}, service.DesktopMemberListFilters{
		Search: c.Query("search"), Status: c.Query("status"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	dtos := make([]desktopMemberDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, desktopMemberFromService(&items[i]))
	}
	response.Paginated(c, dtos, result.Total, result.Page, result.PageSize)
}

func (h *DesktopHandler) UpdateMember(c *gin.Context) {
	var req desktopUpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	member, err := h.desktop.UpdateMember(c.Request.Context(), c.Param("organization_id"), c.Param("member_id"), service.DesktopUpdateMemberInput{
		Name: req.Name, Status: req.Status,
	}, req.Phone)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopMemberFromService(member))
}

func (h *DesktopHandler) DeleteMember(c *gin.Context) {
	if err := h.desktop.DeleteMember(c.Request.Context(), c.Param("organization_id"), c.Param("member_id")); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *DesktopHandler) RotateModelToken(c *gin.Context) {
	payload := struct {
		OrganizationID string `json:"organization_id"`
		MemberID       string `json:"member_id"`
	}{OrganizationID: c.Param("organization_id"), MemberID: c.Param("member_id")}
	result, err := executeAdminIdempotent(c, desktopModelTokenRotateIdempotencyScope, payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		member, execErr := h.desktop.RotateMemberAPIKey(ctx, payload.OrganizationID, payload.MemberID)
		if execErr != nil {
			return nil, execErr
		}
		return desktopMemberFromService(member), nil
	})
	if adminDesktopIdempotencyError(c, err) {
		return
	}
	adminDesktopIdempotencyResponse(c, result, false)
}

func adminDesktopIdempotencyError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	response.ErrorFrom(c, err)
	return true
}

func adminDesktopIdempotencyResponse(c *gin.Context, result *service.IdempotencyExecuteResult, created bool) {
	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	if created {
		response.Created(c, result.Data)
		return
	}
	response.Success(c, result.Data)
}

func desktopOrganizationFromService(value *service.DesktopOrganization, includeConfig bool) desktopOrganizationDTO {
	dto := desktopOrganizationDTO{
		PublicID: value.PublicID, Code: value.Code, Name: value.Name, Status: value.Status,
		GatewayUser: desktopGatewayUserDTO{ID: value.GatewayUserID, Email: value.GatewayUserEmail, Username: value.GatewayUserName},
		Group:       desktopGroupDTO{ID: value.GroupID, Name: value.GroupName}, MemberCount: value.MemberCount,
		TargetConfigAssigned: value.TargetConfigAssigned, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if includeConfig {
		dto.TargetConfig = value.TargetConfig
	}
	return dto
}

func desktopMemberFromService(value *service.DesktopMember) desktopMemberDTO {
	return desktopMemberDTO{
		PublicID: value.PublicID, Name: value.Name, Phone: value.Phone, Status: value.Status,
		ModelTokenStatus: value.ModelTokenStatus(), Usage: value.Usage, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func adminDesktopBindingError(c *gin.Context, err error) {
	if middleware.IsBodyTooLarge(err) {
		response.ErrorFrom(c, infraerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large"))
		return
	}
	response.ErrorFrom(c, service.ErrDesktopValidation.WithMetadata(map[string]string{"binding": strings.TrimSpace(err.Error())}))
}
