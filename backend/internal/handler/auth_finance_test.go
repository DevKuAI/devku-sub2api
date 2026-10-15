package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBackendModeAllowsFinanceLogin(t *testing.T) {
	repo := &passkeySwitchSettingRepo{value: "true", values: map[string]string{service.SettingKeyBackendModeEnabled: "true"}}
	settings := service.NewSettingService(repo, &config.Config{})
	auth := &AuthHandler{settingSvc: settings}
	passkeys := &PasskeyHandler{settingSvc: settings}
	for _, role := range []string{service.RoleAdmin, service.RoleFinance, service.RoleUser} {
		user := &service.User{ID: 7, Role: role, Status: service.StatusActive}
		for _, check := range []func(context.Context, *service.User) error{auth.ensureBackendModeAllowsUser, passkeys.ensureBackendModeAllowsUser} {
			err := check(context.Background(), user)
			if role == service.RoleUser {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
}
