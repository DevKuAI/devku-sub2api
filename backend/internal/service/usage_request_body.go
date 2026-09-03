package service

import (
	"context"
	"sync"
)

type usageRequestBodyContextKey struct{}

type usageRequestBodyHolder struct {
	mu   sync.RWMutex
	body *string
}

// CaptureUsageRequestBody returns a redacted request-body snapshot only while
// the risk-control center is enabled. Runtime-setting failures fail closed.
func (s *ContentModerationService) CaptureUsageRequestBody(ctx context.Context, body []byte, contentType string) *string {
	if s == nil || s.settingRepo == nil || len(body) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	enabled, err := s.settingRepo.GetValue(ctx, SettingKeyRiskControlEnabled)
	if err != nil || enabled != "true" {
		return nil
	}
	redacted := RedactAuditBody(body, contentType)
	if redacted == "" {
		return nil
	}
	return &redacted
}

// WithUsageRequestBodySnapshot stores a mutable holder in ctx. Reusing the
// holder lets WebSocket turns replace their snapshot without retaining a chain
// of prior request bodies.
func WithUsageRequestBodySnapshot(ctx context.Context, body *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if holder, ok := ctx.Value(usageRequestBodyContextKey{}).(*usageRequestBodyHolder); ok && holder != nil {
		holder.set(body)
		return ctx
	}
	return context.WithValue(ctx, usageRequestBodyContextKey{}, &usageRequestBodyHolder{body: cloneStringPtr(body)})
}

func UsageRequestBodyFromContext(ctx context.Context) *string {
	if ctx == nil {
		return nil
	}
	holder, _ := ctx.Value(usageRequestBodyContextKey{}).(*usageRequestBodyHolder)
	if holder == nil {
		return nil
	}
	return holder.get()
}

func ResolveUsageRequestBody(ctx context.Context, explicit *string) *string {
	if explicit != nil {
		return cloneStringPtr(explicit)
	}
	return UsageRequestBodyFromContext(ctx)
}

func (h *usageRequestBodyHolder) set(body *string) {
	h.mu.Lock()
	h.body = cloneStringPtr(body)
	h.mu.Unlock()
}

func (h *usageRequestBodyHolder) get() *string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return cloneStringPtr(h.body)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
