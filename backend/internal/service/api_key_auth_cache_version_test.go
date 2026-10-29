package service

import (
	"context"
	"testing"
)

func TestAPIKeyService_RejectsV10AuthSnapshotWithoutModelAllowlist(t *testing.T) {
	groupID := int64(9)
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-models-list", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  10,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:               groupID,
				Name:             "openai",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1,
			},
		},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatalf("expected v10 auth snapshot to be rejected after model_allowlist was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_RejectsV15AuthSnapshotWithoutReasoningEffortPolicy(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-reasoning-mappings", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 15},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatal("expected v15 auth snapshot to be rejected after reasoning effort policy was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_RejectsV25AuthSnapshotBeforeForkMerge(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-before-fork-merge", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 25},
	})
	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok || apiKey != nil {
		t.Fatal("expected v25 snapshot to be rejected because each parent added different fields")
	}
}

func TestAPIKeyService_AuthSnapshotPreservesMergedFields(t *testing.T) {
	svc := &APIKeyService{}
	groupID := int64(9)
	snapshot := svc.snapshotFromAPIKey(context.Background(), &APIKey{
		ID:        1,
		UserID:    2,
		GroupID:   &groupID,
		ManagedBy: "desktop_analysis",
		User:      &User{ID: 2},
		Group:     &Group{ID: groupID, ImageCountBilling: true},
	})
	apiKey, ok, err := svc.applyAuthCacheEntry("k-merged-fields", &APIKeyAuthCacheEntry{Snapshot: snapshot})
	if err != nil || !ok || apiKey == nil {
		t.Fatalf("expected current snapshot to be accepted, got key=%v ok=%v err=%v", apiKey, ok, err)
	}
	if apiKey.ManagedBy != "desktop_analysis" || apiKey.Group == nil || !apiKey.Group.ImageCountBilling {
		t.Fatalf("expected managed identity and image billing to survive the cache round trip, got %#v", apiKey)
	}
}
