package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func boundSubscriptionAccount(c *gin.Context, adminService service.AdminService) (*service.Account, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return nil, false
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return nil, false
	}
	account, err := adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	if account == nil || account.BoundUserID == nil || *account.BoundUserID != subject.UserID {
		response.NotFound(c, "Subscription account not found")
		return nil, false
	}
	if !account.IsOpenAI() || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "This operation requires an OpenAI OAuth subscription account")
		return nil, false
	}
	return account, true
}

func subscriptionAccountResetCredits(account *service.Account) *service.OpenAIRateLimitResetCredits {
	if !account.IsOpenAI() || account.Type != service.AccountTypeOAuth {
		return nil
	}
	snapshot := account.Extra["codex_reset_credit_snapshot"]
	if snapshot == nil {
		return nil
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	var credits service.OpenAIRateLimitResetCredits
	if json.Unmarshal(raw, &credits) != nil {
		return nil
	}
	return &credits
}

// Keep upstream identities and account credentials out of user-facing responses.
func subscriptionQuotaUsage(usage *service.OpenAIQuotaUsage) *service.OpenAIQuotaUsage {
	if usage == nil {
		return nil
	}
	return &service.OpenAIQuotaUsage{
		FetchedAt:             usage.FetchedAt,
		RateLimitResetCredits: usage.RateLimitResetCredits,
	}
}

func (h *AccountHandler) GetMySubscriptionAccountUsage(c *gin.Context) {
	h.mySubscriptionAccountUsage(c, false)
}

func (h *AccountHandler) RefreshMySubscriptionAccountUsage(c *gin.Context) {
	h.mySubscriptionAccountUsage(c, true)
}

func (h *AccountHandler) mySubscriptionAccountUsage(c *gin.Context, refresh bool) {
	account, ok := boundSubscriptionAccount(c, h.adminService)
	if !ok {
		return
	}
	if h.accountUsageService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account usage service is not enabled")
		return
	}
	var usage *service.UsageInfo
	var err error
	if refresh {
		usage, err = h.accountUsageService.GetUsageForAccount(c.Request.Context(), account, true)
	} else {
		usage, err = h.accountUsageService.GetReadOnlyUsageForAccount(c.Request.Context(), account)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subscriptionAccountUsageFromService(usage))
}

func (h *OpenAIOAuthHandler) RefreshMySubscriptionQuota(c *gin.Context) {
	if _, ok := boundSubscriptionAccount(c, h.adminService); !ok {
		return
	}
	h.refreshQuota(c, true)
}

func (h *OpenAIOAuthHandler) ResetMySubscriptionQuota(c *gin.Context) {
	account, ok := boundSubscriptionAccount(c, h.adminService)
	if !ok {
		return
	}
	if account.IsShadow() {
		response.ErrorFrom(c, service.ErrSparkShadowResetNotSupported)
		return
	}
	h.resetQuota(c, true)
}
