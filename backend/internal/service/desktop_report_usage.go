package service

import "context"

type desktopReportUsageKey struct{}
type DesktopReportUsageCapture func(*UsageLog)

func WithDesktopReportUsageCapture(ctx context.Context, capture DesktopReportUsageCapture) context.Context {
	return context.WithValue(ctx, desktopReportUsageKey{}, capture)
}
func DesktopReportUsageCaptureFromContext(ctx context.Context) DesktopReportUsageCapture {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(desktopReportUsageKey{}).(DesktopReportUsageCapture)
	return capture
}

type DesktopReportUsageRecorder interface {
	RecordExecutionUsage(context.Context, int64, *UsageLog) error
}
