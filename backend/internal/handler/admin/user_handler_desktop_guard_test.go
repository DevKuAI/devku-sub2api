package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type desktopUserGuardStub struct {
	assignedGroupID      int64
	called               bool
	restrictPublicGroups bool
	allowedGroups        []int64
}

func (s *desktopUserGuardStub) ListAvailableGatewayUsers(context.Context, pagination.PaginationParams, string) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *desktopUserGuardStub) EnsureGatewayUserGroupAccess(_ context.Context, _ int64, restrictPublicGroups bool, allowedGroups []int64) error {
	s.called = true
	s.restrictPublicGroups = restrictPublicGroups
	s.allowedGroups = append([]int64(nil), allowedGroups...)
	if !restrictPublicGroups {
		return nil
	}
	for _, groupID := range allowedGroups {
		if groupID == s.assignedGroupID {
			return nil
		}
	}
	return service.ErrDesktopDependency
}

func (s *desktopUserGuardStub) EnsureGatewayUserCanBeDeleted(context.Context, int64) error {
	return nil
}

type desktopGuardAdminService struct {
	*stubAdminService
	updateCalls int
}

func (s *desktopGuardAdminService) UpdateUser(ctx context.Context, id int64, input *service.UpdateUserInput) (*service.User, error) {
	s.updateCalls++
	return s.stubAdminService.UpdateUser(ctx, id, input)
}

func TestUpdateUserRestrictPublicGroupsValidatesDesktopCarrier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := newStubAdminService()
	base.users[0].AllowedGroups = []int64{8}
	adminService := &desktopGuardAdminService{stubAdminService: base}
	desktopService := &desktopUserGuardStub{assignedGroupID: 7}
	handler := NewUserHandler(adminService, nil, nil, nil, nil, nil, nil)
	handler.desktopService = desktopService

	router := gin.New()
	router.PUT("/api/v1/admin/users/:id", handler.Update)
	recorder := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/1", map[string]any{
		"restrict_public_groups": true,
	})

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.True(t, desktopService.called)
	require.True(t, desktopService.restrictPublicGroups)
	require.Equal(t, []int64{8}, desktopService.allowedGroups)
	require.Zero(t, adminService.updateCalls)
}
