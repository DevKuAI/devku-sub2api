package service

import (
	"context"
	"time"
)

type DesktopPeriodCoverage struct {
	Available []string          `json:"available_dates"`
	Partial   []string          `json:"partial_dates"`
	Missing   []string          `json:"missing_dates"`
	NoRecords []string          `json:"no_record_dates"`
	Reasons   map[string]string `json:"reasons"`
}
type DesktopPeriodSource struct {
	Date          string     `json:"date"`
	Revision      int        `json:"revision"`
	Status        string     `json:"status"`
	GeneratedAt   *time.Time `json:"generated_at"`
	Hash          string     `json:"content_hash"`
	Content       string     `json:"content,omitempty"`
	CurrentStatus string     `json:"current_status,omitempty"`
}
type DesktopPeriodMetadata struct {
	ID            string                `json:"id"`
	Kind          string                `json:"kind"`
	Range         DesktopNativeRange    `json:"range"`
	Revision      int                   `json:"revision"`
	Status        string                `json:"status"`
	Reason        string                `json:"reason"`
	GeneratedAt   *time.Time            `json:"generated_at"`
	Coverage      DesktopPeriodCoverage `json:"coverage"`
	SourceChanged bool                  `json:"source_changed"`
	Model         string                `json:"model"`
}
type DesktopPeriodConfiguration struct {
	Model    string `json:"model"`
	UserID   int64  `json:"user_id"`
	GroupID  int64  `json:"group_id"`
	Captured bool   `json:"captured"`
}
type DesktopPeriodTask struct {
	DesktopPeriodMetadata
	InternalID     int64                      `json:"-"`
	OrganizationID int64                      `json:"-"`
	Start          time.Time                  `json:"-"`
	End            time.Time                  `json:"-"`
	Sources        []DesktopPeriodSource      `json:"-"`
	Work           DesktopReportWork          `json:"-"`
	Configuration  DesktopPeriodConfiguration `json:"-"`
	Content        *string                    `json:"-"`
	LeaseToken     string                     `json:"-"`
}
type DesktopPeriodRepository interface {
	EnqueuePeriod(context.Context, int64, string, time.Time, time.Time, string) (*DesktopPeriodTask, error)
	ReadPeriod(context.Context, int64, string, bool) (*DesktopPeriodTask, error)
	ListPeriods(context.Context, int64, string, string, int, int, ...bool) ([]DesktopPeriodMetadata, int64, error)
	ListPeriodSources(context.Context, int64, string) ([]DesktopPeriodSource, error)
	ReadPeriodSource(context.Context, int64, string, string) (DesktopPeriodSource, error)
	PeriodSources(context.Context, int64, time.Time, time.Time) ([]DesktopPeriodSource, DesktopPeriodCoverage, error)
	ClaimPeriod(context.Context) (*DesktopPeriodTask, error)
	SavePeriod(context.Context, *DesktopPeriodTask, time.Time, bool) error
	RenewPeriod(context.Context, *DesktopPeriodTask) error
	DuePeriodOrganizations(context.Context, string, time.Time) ([]string, error)
	BeginPeriodExecution(context.Context, *DesktopPeriodTask, *DesktopReportExecution) error
	FinishPeriodExecution(context.Context, *DesktopReportExecution) error
	RecordPeriodExecutionUsage(context.Context, int64, *UsageLog) error
}
