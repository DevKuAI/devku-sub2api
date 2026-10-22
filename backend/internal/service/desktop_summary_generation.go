package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Both daily and period jobs use the same checkpointed generation and billing path.
type reportGenerationHooks struct {
	Model        string
	Stage        string
	Revision     int
	MemberID     string
	Organization func(context.Context) (*DesktopOrganization, error)
	Validate     func(*DesktopOrganization) error
	Save         func(context.Context) error
	Begin        func(context.Context, *DesktopReportExecution) error
	Finish       func(context.Context, *DesktopReportExecution) error
	Complete     func(context.Context, *DesktopReportExecution) error
	RecordUsage  func(context.Context, int64, *UsageLog) error
}

func (s *DesktopReportService) generateSummary(ctx context.Context, key *APIKey, work *DesktopReportWork, prompt, input, label string, hooks reportGenerationHooks) (string, error) {
	if len(work.Inputs) == 0 {
		work.Inputs = reportTextInputs(input, hooks.Model)
	}
	for {
		for len(work.Outputs) < len(work.Inputs) {
			o, err := hooks.Organization(ctx)
			if err != nil {
				return "", err
			}
			if err = hooks.Validate(o); err != nil {
				return "", err
			}
			if key.RateLimit1d != o.ManagementPolicy.AnalysisLimit1d {
				key, err = s.repo.AnalysisKey(ctx, o)
				if err != nil {
					return "", err
				}
				if s.desktop.apiKeys != nil {
					s.desktop.apiKeys.InvalidateAuthCacheByKey(ctx, key.Key)
				}
			}
			if err = s.check(ctx, o, key, hooks.Model); err != nil {
				return "", err
			}
			if work.Attempts >= 3 {
				return "", errors.New("report attempts exhausted")
			}
			work.Attempts++
			if err = hooks.Save(ctx); err != nil {
				return "", err
			}
			requestID := "desktop-report-" + hooks.Stage + "-" + uuid.NewString()
			execution := DesktopReportExecution{Revision: hooks.Revision, Kind: "summary", Model: hooks.Model, Round: work.Round, Chunk: len(work.Outputs) + 1, Attempt: work.Attempts, RequestID: requestID, Status: "running", StartedAt: s.now().UTC()}
			if hooks.MemberID != "" {
				execution.Kind = "member"
				execution.MemberID = hooks.MemberID
			}
			if err = hooks.Begin(ctx, &execution); err != nil {
				work.Attempts--
				if errors.Is(err, ErrDesktopReportLeaseLost) {
					return "", err
				}
				return "", NewDesktopReportUnavailable("execution_storage_unavailable")
			}
			callCtx := ctx
			if hooks.RecordUsage != nil {
				executionID := execution.ID
				callCtx = WithDesktopReportUsageCapture(ctx, func(usage *UsageLog) {
					usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
					defer cancel()
					if err := hooks.RecordUsage(usageCtx, executionID, usage); err != nil {
						slog.Error("desktop report usage checkpoint failed", "execution_id", executionID, "error", err)
					}
				})
			}
			text, callErr := s.ai.Generate(callCtx, o, key, hooks.Model, prompt, label+"\n"+work.Inputs[len(work.Outputs)], requestID)
			if callErr == nil && strings.TrimSpace(text) == "" {
				callErr = errors.New("empty report output")
			}
			execution.Status = "completed"
			if callErr != nil {
				execution.Status, execution.Error = "failed", "upstream_failed"
				var blocked *DesktopReportUnavailable
				if errors.As(callErr, &blocked) {
					execution.Status, execution.Error = "blocked", blocked.Reason
					work.Attempts--
				}
			}
			finished := s.now().UTC()
			execution.FinishedAt = &finished
			// A successful chunk and its execution status commit together, even during shutdown.
			finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if callErr != nil {
				finishErr := hooks.Finish(finishCtx, &execution)
				finishCancel()
				if finishErr != nil {
					return "", finishErr
				}
				return "", callErr
			}
			previous := *work
			chunk := len(work.Outputs)
			consumed := work.Inputs[chunk]
			work.Outputs = append(work.Outputs, text)
			work.Inputs[chunk] = ""
			work.Attempts = 0
			work.RetryAt = time.Time{}
			completeErr := hooks.Complete(finishCtx, &execution)
			finishCancel()
			if completeErr != nil {
				slog.ErrorContext(ctx, "desktop report checkpoint outcome uncertain", "execution_id", execution.ID, "error", completeErr)
				work.Inputs[chunk] = consumed
				*work = previous
				// Do not overwrite a transaction whose commit acknowledgement may have been lost.
				return "", fmt.Errorf("%w: %w", ErrDesktopReportCheckpointUncertain, completeErr)
			}
		}
		if len(work.Outputs) == 1 {
			return work.Outputs[0], nil
		}
		merged := strings.Join(work.Outputs, "\n\n")
		if work.Round >= 8 {
			work.Attempts = 3
			return "", errors.New("report reduction did not converge")
		}
		work.Inputs = reportTextInputs(merged, hooks.Model)
		work.Outputs = nil
		work.Round++
		if err := hooks.Save(ctx); err != nil {
			return "", err
		}
	}
}
