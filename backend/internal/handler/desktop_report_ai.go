package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// The adapter dispatches through the registered gateway, keeping authentication,
// provider conversion, concurrency limits and billing identical to normal calls.
type desktopReportAI struct {
	gateway       *GatewayHandler
	router        http.Handler
	composite     *service.CompositeRouteResolver
	subscriptions *service.SubscriptionService
}

func (h *GatewayHandler) DesktopReportAI(router *gin.Engine, resolver *service.CompositeRouteResolver, subscriptions *service.SubscriptionService) service.DesktopReportAI {
	return &desktopReportAI{gateway: h, router: router, composite: resolver, subscriptions: subscriptions}
}

func (a *desktopReportAI) Check(ctx context.Context, o *service.DesktopOrganization, key *service.APIKey, model string) error {
	user, err := a.gateway.userService.GetByID(ctx, o.GatewayUserID)
	if err != nil || user.Status != service.StatusActive {
		return service.NewDesktopReportUnavailable("user_unavailable")
	}
	group, err := a.gateway.gatewayService.ResolveGroupByID(ctx, o.GroupID)
	if err != nil || group.Status != service.StatusActive {
		return service.NewDesktopReportUnavailable("group_unavailable")
	}
	if !group.ModelAllowlist.Allows(model) {
		return service.NewDesktopReportUnavailable("model_unsupported")
	}
	platform, upstreamModel := group.Platform, model
	if platform == service.PlatformComposite {
		decision, e := a.composite.Resolve(ctx, group.ID, model, service.CompositeRouteEndpointChatCompletions)
		if e != nil || !decision.Matched {
			return service.NewDesktopReportUnavailable("model_unsupported")
		}
		platform, upstreamModel = decision.TargetPlatform, decision.UpstreamModel
		ctx = service.WithCompositeRouteDecision(ctx, decision)
	}
	var subscription *service.UserSubscription
	if group.IsSubscriptionType() && (a.gateway.cfg == nil || a.gateway.cfg.RunMode != config.RunModeSimple) {
		subscription, err = a.subscriptions.GetActiveSubscription(ctx, user.ID, group.ID)
		if err != nil {
			return service.NewDesktopReportUnavailable("billing_unavailable")
		}
	}
	if err = a.gateway.billingCacheService.CheckBillingReadiness(ctx, user, key, group, subscription, platform); err != nil {
		return service.NewDesktopReportUnavailable("billing_unavailable")
	}
	var account *service.Account
	if domain.UsesOpenAIGateway(platform) {
		account, err = a.gateway.openAIGatewayService.SelectAccountForTokenCount(ctx, &o.GroupID, "", upstreamModel, service.OpenAIEndpointCapabilityChatCompletions, platform)
	} else {
		account, err = a.gateway.gatewayService.SelectAccountForModel(ctx, &o.GroupID, "", model)
	}
	if err != nil || account == nil {
		return service.NewDesktopReportUnavailable("model_unavailable")
	}
	return nil
}

type desktopReportResponseWriter struct {
	header   http.Header
	body     bytes.Buffer
	code     int
	overflow bool
}

func (w *desktopReportResponseWriter) Header() http.Header { return w.header }
func (w *desktopReportResponseWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *desktopReportResponseWriter) Write(data []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	if w.body.Len()+len(data) > 4<<20 {
		w.overflow = true
		return 0, io.ErrShortBuffer
	}
	return w.body.Write(data)
}
func (w *desktopReportResponseWriter) Flush() {
	if w.code == 0 {
		w.code = 200
	}
}

func (a *desktopReportAI) Generate(ctx context.Context, o *service.DesktopOrganization, key *service.APIKey, model, prompt, input, requestID string) (string, error) {
	if err := a.Check(ctx, o, key, model); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(service.WithDesktopReportRequest(ctx), 2*time.Minute)
	defer cancel()
	body, err := json.Marshal(map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": input}}, "stream": false, "max_tokens": 4096})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+key.Key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Sub2API-Enterprise-Analysis/1.0")
	request.Header.Set("X-Client-Request-Id", requestID)
	request.RemoteAddr = "127.0.0.1:0"
	writer := &desktopReportResponseWriter{header: make(http.Header)}
	a.router.ServeHTTP(writer, request)
	if writer.overflow {
		return "", fmt.Errorf("report response exceeded size limit")
	}
	if writer.code == http.StatusTooManyRequests || writer.code == http.StatusServiceUnavailable || writer.code == http.StatusPaymentRequired || writer.code == http.StatusForbidden {
		return "", service.NewDesktopReportUnavailable("model_unavailable")
	}
	if writer.code != 200 {
		return "", fmt.Errorf("report gateway returned HTTP %d", writer.code)
	}
	return desktopReportResponseText(writer.body.Bytes())
}
func desktopReportResponseText(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		OutputText string `json:"output_text"`
		Status     string `json:"status"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if len(response.Choices) > 0 {
		choice := response.Choices[0]
		if choice.FinishReason == "length" || strings.TrimSpace(choice.Message.Content) == "" {
			return "", fmt.Errorf("report completion was empty or truncated")
		}
		return choice.Message.Content, nil
	}
	if response.Status != "" && response.Status != "completed" {
		return "", fmt.Errorf("report response was not completed")
	}
	if strings.TrimSpace(response.OutputText) != "" {
		return response.OutputText, nil
	}
	var texts []string
	for _, item := range response.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				texts = append(texts, part.Text)
			}
		}
	}
	text := strings.Join(texts, "\n")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("report response contains no output text")
	}
	return text, nil
}
