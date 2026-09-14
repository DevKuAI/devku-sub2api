package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	tiboResetMonitorURL      = "https://aihot.news/api/v1/codex-resets"
	TiboResetMonitorCacheTTL = time.Hour
	tiboResetMonitorLimit    = 2 << 20
)

var tiboResetMonitorEndpoint = tiboResetMonitorURL

type TiboResetMonitorCache interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, value []byte, ttl time.Duration) error
}

type TiboResetMonitorSchedule struct {
	Precision string `json:"precision"`
	From      string `json:"from"`
	Through   string `json:"through"`
	Label     string `json:"label"`
}

type TiboResetMonitorPost struct {
	ID          string `json:"id"`
	PublishedAt string `json:"publishedAt"`
	Stage       string `json:"stage"`
	Text        string `json:"text"`
	URL         string `json:"url"`
}

type TiboResetMonitorEvent struct {
	ID                string                    `json:"id"`
	Type              string                    `json:"type"`
	Label             string                    `json:"label"`
	Status            string                    `json:"status"`
	Title             string                    `json:"title"`
	Scope             string                    `json:"scope"`
	CreatedAt         string                    `json:"createdAt"`
	UpdatedAt         string                    `json:"updatedAt"`
	ConfirmedAt       *string                   `json:"confirmedAt"`
	OccurredOn        *string                   `json:"occurredOn"`
	ConfirmationBasis string                    `json:"confirmationBasis"`
	Schedule          *TiboResetMonitorSchedule `json:"schedule"`
	Posts             []TiboResetMonitorPost    `json:"posts"`
	URL               string                    `json:"url"`
}

type TiboResetMonitor struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Timezone      string                  `json:"timezone"`
	CheckedAt     string                  `json:"checkedAt"`
	HistoryFrom   string                  `json:"historyFrom"`
	Count         int                     `json:"count"`
	Events        []TiboResetMonitorEvent `json:"events"`
}

type TiboResetMonitorService struct {
	cache      TiboResetMonitorCache
	httpClient *http.Client
	mu         sync.Mutex
}

func NewTiboResetMonitorService(cache TiboResetMonitorCache) *TiboResetMonitorService {
	return &TiboResetMonitorService{
		cache:      cache,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *TiboResetMonitorService) Get(ctx context.Context) (*TiboResetMonitor, error) {
	if s == nil {
		return nil, fmt.Errorf("tibo reset monitor service is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if cached := s.readCache(ctx); cached != nil {
		return cached, nil
	}
	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tiboResetMonitorEndpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch tibo reset monitor: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch tibo reset monitor: upstream status %s", resp.Status)
	}
	var result TiboResetMonitor
	decoder := json.NewDecoder(io.LimitReader(resp.Body, tiboResetMonitorLimit))
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode tibo reset monitor: %w", err)
	}
	if result.SchemaVersion == 0 {
		return nil, fmt.Errorf("decode tibo reset monitor: missing schema version")
	}
	if s.cache != nil {
		if raw, err := json.Marshal(result); err == nil {
			_ = s.cache.Write(ctx, raw, TiboResetMonitorCacheTTL)
		}
	}
	return &result, nil
}

func (s *TiboResetMonitorService) readCache(ctx context.Context) *TiboResetMonitor {
	if s.cache == nil {
		return nil
	}
	raw, err := s.cache.Read(ctx)
	if err != nil {
		return nil
	}
	var result TiboResetMonitor
	if json.Unmarshal(raw, &result) != nil || result.SchemaVersion == 0 {
		return nil
	}
	return &result
}
