package service

import "math"

// Zero keeps existing unlimited budgets and indefinite retention.
type DesktopManagementPolicy struct {
	AnalysisLimit1d           float64 `json:"analysis_limit_1d"`
	ConversationRetentionDays int     `json:"conversation_retention_days"`
	ReportRetentionDays       int     `json:"report_retention_days"`
	ExecutionRetentionDays    int     `json:"execution_retention_days"`
}

func (p DesktopManagementPolicy) Validate() error {
	if !desktopBudgetAmount(p.AnalysisLimit1d) {
		return ErrDesktopValidation
	}
	for _, days := range []int{p.ConversationRetentionDays, p.ReportRetentionDays, p.ExecutionRetentionDays} {
		if days < 0 || days > 3650 {
			return ErrDesktopValidation
		}
	}
	return nil
}

type DesktopMemberBudget struct {
	Quota     float64 `json:"quota"`
	Limit1d   float64 `json:"limit_1d"`
	Limit7d   float64 `json:"limit_7d"`
	QuotaUsed float64 `json:"quota_used"`
	Usage1d   float64 `json:"usage_1d"`
	Usage7d   float64 `json:"usage_7d"`
}

func desktopBudgetAmount(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (b DesktopMemberBudget) Validate() error {
	for _, value := range []float64{b.Quota, b.Limit1d, b.Limit7d} {
		if !desktopBudgetAmount(value) {
			return ErrDesktopValidation
		}
	}
	return nil
}

func (b DesktopMemberBudget) Apply(key *APIKey) {
	key.Quota, key.RateLimit1d, key.RateLimit7d = b.Quota, b.Limit1d, b.Limit7d
}

func DesktopMemberBudgetFromKey(key *APIKey) *DesktopMemberBudget {
	return &DesktopMemberBudget{Quota: key.Quota, Limit1d: key.RateLimit1d, Limit7d: key.RateLimit7d, QuotaUsed: key.QuotaUsed, Usage1d: key.EffectiveUsage1d(), Usage7d: key.EffectiveUsage7d()}
}
