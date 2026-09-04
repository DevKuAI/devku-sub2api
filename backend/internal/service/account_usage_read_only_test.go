package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountUsageServiceReadOnlyOpenAIUsageDoesNotProbeOrPersist(t *testing.T) {
	ctx := context.Background()
	parentID := int64(100)
	shadow := &Account{
		ID:              200,
		ParentAccountID: &parentID,
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		Status:          StatusActive,
		QuotaDimension:  QuotaDimensionSpark,
	}
	parent := &Account{
		ID:       parentID,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Credentials: map[string]any{
			"chatgpt_account_id": "org-parent",
		},
	}

	updates := make(chan map[string]any, 1)
	repo := &sparkShadowUsageTestRepo{
		accounts:      map[int64]*Account{shadow.ID: shadow, parent.ID: parent},
		updateExtraCh: updates,
	}
	tokenCache := &stubQuotaTokenCache{tokens: map[string]string{
		OpenAITokenCacheKey(parent): "fake-access-token",
	}}
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(OpenAIQuotaUsage{})
	}))
	defer server.Close()

	quotaService := NewOpenAIQuotaService(
		repo,
		nil,
		NewOpenAITokenProvider(repo, tokenCache, nil),
		newQuotaRedirectingFactory(server),
	)
	usageService := &AccountUsageService{
		accountRepo:        repo,
		openAIQuotaService: quotaService,
	}

	usage, err := usageService.GetReadOnlyUsageForAccount(ctx, shadow)

	require.NoError(t, err)
	require.Equal(t, "passive", usage.Source)
	require.Zero(t, requestCount.Load())
	select {
	case update := <-updates:
		t.Fatalf("read-only usage unexpectedly persisted account data: %#v", update)
	default:
	}
}

func TestAccountUsageServiceReadOnlyOpenAIUsagePreservesSnapshotTime(t *testing.T) {
	sampledAt := time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC)
	account := &Account{
		ID:       201,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Extra: map[string]any{
			"codex_usage_updated_at": sampledAt.Format(time.RFC3339),
			"codex_5h_used_percent":  25.0,
			"codex_5h_reset_at":      sampledAt.Add(5 * time.Hour).Format(time.RFC3339),
		},
	}

	usage, err := (&AccountUsageService{}).GetReadOnlyUsageForAccount(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, usage.UpdatedAt)
	require.Equal(t, sampledAt, *usage.UpdatedAt)
}
