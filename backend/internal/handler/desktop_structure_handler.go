package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopStructureHandler struct {
	desktop *service.DesktopService
	managed bool
}

func NewDesktopStructureHandler(desktop *service.DesktopService, managed bool) *DesktopStructureHandler {
	return &DesktopStructureHandler{desktop: desktop, managed: managed}
}

// Register shares the production routes with the real HTTP client fixture.
func (h *DesktopStructureHandler) Register(group *gin.RouterGroup) {
	for _, kind := range []string{service.DesktopDepartmentKind, service.DesktopPositionKind} {
		group.GET("/"+kind, h.Unit(kind))
		group.POST("/"+kind, h.Unit(kind))
		group.GET("/"+kind+"/:unit_id", h.Unit(kind))
		group.PATCH("/"+kind+"/:unit_id", h.Unit(kind))
	}
	group.GET("/organization-structure/status", h.Status)
	group.POST("/members/usage-export", h.ExportMemberUsage)
	group.GET("/members/:member_id", h.GetMember)
	group.PUT("/members/:member_id/position", h.Assign)
	group.GET("/members/:member_id/position-history", h.History)
}

func (h *DesktopStructureHandler) scope(c *gin.Context) (service.DesktopStructureRepository, string, bool) {
	c.Header("Cache-Control", "no-store")
	managerID := int64(0)
	if h.managed {
		var ok bool
		managerID, ok = desktopManagedUserID(c)
		if !ok {
			return nil, "", false
		}
	}
	repo, org, err := h.desktop.StructureScope(c.Request.Context(), c.Param("organization_id"), managerID)
	if response.ErrorFrom(c, err) {
		return nil, "", false
	}
	if c.Request.Method != "GET" {
		organization, e := h.desktop.GetOrganization(c.Request.Context(), org)
		if response.ErrorFrom(c, e) {
			return nil, "", false
		}
		if organization.Status != service.DesktopStatusActive {
			response.ErrorFrom(c, service.ErrDesktopOrganizationDisabled)
			return nil, "", false
		}
	}
	return repo, org, true
}

func structurePage(c *gin.Context) pagination.PaginationParams {
	page, size := response.ParsePagination(c)
	return pagination.PaginationParams{Page: page, PageSize: size}
}

func (h *DesktopStructureHandler) Unit(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo, org, ok := h.scope(c)
		if !ok {
			return
		}
		id := c.Param("unit_id")
		if c.Request.Method == "GET" {
			if id != "" {
				unit, err := repo.GetOrganizationUnit(c.Request.Context(), org, kind, id)
				if !response.ErrorFrom(c, err) {
					response.Success(c, unit)
				}
				return
			}
			includeMemberCount := c.Query("include_member_count")
			if includeMemberCount != "" && includeMemberCount != "true" && includeMemberCount != "false" {
				response.ErrorFrom(c, service.ErrDesktopValidation)
				return
			}
			filter := service.DesktopStructureFilter{ExcludeMemberCount: includeMemberCount == "false", Search: strings.TrimSpace(c.Query("search")), Status: c.Query("status"), DepartmentID: c.Query("department_id")}
			if (filter.Status != "" && filter.Status != "active" && filter.Status != "disabled") || len(filter.Search) > 400 {
				response.ErrorFrom(c, service.ErrDesktopValidation)
				return
			}
			rows, page, err := repo.ListOrganizationUnits(c.Request.Context(), org, kind, structurePage(c), filter)
			if !response.ErrorFrom(c, err) {
				response.Paginated(c, rows, page.Total, page.Page, page.PageSize)
			}
			return
		}
		var in service.DesktopSaveUnitInput
		if id != "" {
			var patch struct {
				Name             *string `json:"name"`
				Status           *string `json:"status"`
				ExpectedRevision int64   `json:"expected_revision"`
			}
			if err := dto.BindDesktopWrite(c, &patch); response.ErrorFrom(c, err) {
				return
			}
			in.Name, in.Status, in.ExpectedRevision = patch.Name, patch.Status, patch.ExpectedRevision
		} else if err := dto.BindDesktopWrite(c, &in); response.ErrorFrom(c, err) {
			return
		}
		if err := service.ValidateDesktopUnitInput(kind, id, &in); response.ErrorFrom(c, err) {
			return
		}
		payload := struct {
			OrganizationID string                       `json:"organization_id"`
			UnitID         string                       `json:"unit_id"`
			Input          service.DesktopSaveUnitInput `json:"input"`
		}{org, id, in}
		save := func(ctx context.Context) (any, error) { return repo.SaveOrganizationUnit(ctx, org, kind, id, in) }
		if id == "" {
			executeUserIdempotentJSON(c, "desktop.structure."+kind+".create", payload, service.DefaultWriteIdempotencyTTL(), save)
			return
		}
		result, err := save(c.Request.Context())
		if !response.ErrorFrom(c, err) {
			response.Success(c, result)
		}
	}
}

func (h *DesktopStructureHandler) Status(c *gin.Context) {
	repo, org, ok := h.scope(c)
	if !ok {
		return
	}
	status, err := repo.OrganizationStructureStatus(c.Request.Context(), org)
	if !response.ErrorFrom(c, err) {
		response.Success(c, status)
	}
}

func (h *DesktopStructureHandler) GetMember(c *gin.Context) {
	_, org, ok := h.scope(c)
	if !ok {
		return
	}
	managerID := int64(0)
	if h.managed {
		managerID, _ = desktopManagedUserID(c)
	}
	member, err := h.desktop.GetWebMember(c.Request.Context(), org, c.Param("member_id"), managerID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, desktopManagedMemberFromService(member))
	}
}

func (h *DesktopStructureHandler) Assign(c *gin.Context) {
	repo, org, ok := h.scope(c)
	if !ok {
		return
	}
	var in service.DesktopAssignPositionInput
	if err := dto.BindDesktopWrite(c, &in); response.ErrorFrom(c, err) {
		return
	}
	if err := service.ValidateDesktopAssignment(&in); response.ErrorFrom(c, err) {
		return
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists {
		in.ActorID = subject.UserID
	}
	in.RequestID = dto.DesktopRequestID(c)
	memberID := c.Param("member_id")
	if _, err := h.desktop.GetWebMember(c.Request.Context(), org, memberID, 0); response.ErrorFrom(c, err) {
		return
	}
	payload := struct {
		OrganizationID string                             `json:"organization_id"`
		MemberID       string                             `json:"member_id"`
		Input          service.DesktopAssignPositionInput `json:"input"`
	}{org, memberID, in}
	executeUserIdempotentJSON(c, "desktop.members.position", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		member, err := repo.AssignMemberPosition(ctx, org, memberID, in)
		if err != nil {
			return nil, err
		}
		return desktopManagedMemberFromService(member), nil
	})
}

func (h *DesktopStructureHandler) History(c *gin.Context) {
	repo, org, ok := h.scope(c)
	if !ok {
		return
	}
	rows, page, err := repo.MemberPositionHistory(c.Request.Context(), org, c.Param("member_id"), structurePage(c))
	if !response.ErrorFrom(c, err) {
		response.Paginated(c, rows, page.Total, page.Page, page.PageSize)
	}
}
