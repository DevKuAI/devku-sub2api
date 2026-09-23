package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

type reportWork struct {
	ID, OrganizationID, Owner string
	Attempt                   int
}

func (s *Service) claimReport(ctx context.Context) (*reportWork, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	w := &reportWork{Owner: randomToken("lease_")}
	err = tx.QueryRowContext(ctx, `SELECT id,organization_id,attempt_count FROM bi_reports WHERE status IN ('queued','running') AND (lease_until IS NULL OR lease_until<$1)
		ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, s.now().UTC()).Scan(&w.ID, &w.OrganizationID, &w.Attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE bi_reports SET status='running',lease_owner=$2,lease_until=$3,attempt_count=attempt_count+1 WHERE id=$1`, w.ID, w.Owner, s.now().UTC().Add(2*time.Minute))
	if err != nil {
		return nil, err
	}
	w.Attempt++
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) ProcessNextReport(ctx context.Context) (bool, error) {
	w, err := s.claimReport(ctx)
	if err != nil || w == nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	err = s.generateReport(workCtx, w)
	if err == nil || errors.Is(err, errLeaseLost) {
		return true, nil
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	failure := ""
	if errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthenticated) {
		failure = "CONTEXT_REVOKED"
	}
	var typed *Error
	if errors.As(err, &typed) && typed.Code == "DATA_UNAVAILABLE" {
		failure = "DATA_UNAVAILABLE"
	}
	if failure == "" && w.Attempt >= 5 {
		failure = "GENERATION_FAILED"
	}
	if failure != "" {
		_, finishErr := s.db.ExecContext(ctx, `UPDATE bi_reports SET status='failed',failure_code=$3,lease_owner=NULL,lease_until=NULL WHERE id=$1 AND lease_owner=$2 AND status='running'`, w.ID, w.Owner, failure)
		return true, finishErr
	}
	_, releaseErr := s.db.ExecContext(ctx, `UPDATE bi_reports SET lease_until=$3 WHERE id=$1 AND lease_owner=$2 AND status='running'`, w.ID, w.Owner, s.now().UTC().Add(5*time.Second))
	if releaseErr != nil {
		return true, releaseErr
	}
	return true, err
}

func (s *Service) reportPrincipal(ctx context.Context, managerID string) (Principal, error) {
	p := Principal{ManagerID: managerID}
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM bi_managers WHERE id=$1`, managerID).Scan(&p.UserID); err != nil {
		return p, err
	}
	_, err := s.activeUser(ctx, p.UserID)
	return p, err
}

func (s *Service) generateReport(ctx context.Context, w *reportWork) error {
	report, err := readStoredReport(ctx, s.db, w.OrganizationID, w.ID)
	if err != nil {
		return err
	}
	p, err := s.reportPrincipal(ctx, report.ManagerID)
	if err != nil {
		return err
	}
	current, err := s.reportAccess(ctx, p, report, "reports:create")
	if err != nil {
		return err
	}
	if !containsAll(current.Capabilities, report.State.Scope.Capabilities) {
		return ErrForbidden
	}
	report.State.Principal = p
	f, err := s.loadAnalysis(ctx, report.State)
	if err != nil {
		return err
	}
	stats := f.statistics(analysisSelection{}).stats
	if stats.Requests == nil && stats.Tokens.Total == nil && report.State.Context.Status != "not_observable" {
		return apiError(503, "DATA_UNAVAILABLE", "Usage data is unavailable for this report")
	}
	body, dependencies, required, evidence, err := f.renderReport(report.Summary.Title, stats)
	if err != nil {
		return err
	}
	// Recheck current authorization after generation, without extending the frozen data scope.
	report.Dependencies, report.RequiredCapabilities = dependencies, required
	if _, err := s.reportAccess(ctx, p, report, "reports:create"); err != nil {
		return err
	}
	deps, _ := json.Marshal(dependencies)
	caps, _ := json.Marshal(required)
	targets, _ := json.Marshal(evidence)
	result, err := s.db.ExecContext(ctx, `UPDATE bi_reports SET status='ready',text=$3,dependencies=$4,required_capabilities=$5,evidence=$6,generated_at=$7,lease_owner=NULL,lease_until=NULL
		WHERE id=$1 AND lease_owner=$2 AND status='running' AND lease_until>$7`, w.ID, w.Owner, body, string(deps), string(caps), string(targets), s.now().UTC())
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errLeaseLost
	}
	return nil
}

func displayCount(value *int64) string {
	if value == nil {
		return "未知"
	}
	return fmt.Sprint(*value)
}
func displayToken(value *string) string {
	if value == nil {
		return "未知"
	}
	return *value
}
func displayMetric(value Metric, ratio bool) string {
	if value.Value == nil {
		return "未知"
	}
	text := fmt.Sprintf("%.0f", *value.Value)
	if ratio {
		text = fmt.Sprintf("%.1f%%", *value.Value*100)
	}
	if value.Status != "ready" {
		text += "（部分数据）"
	}
	return text
}

func (f *analysisFrame) renderReport(title string, stats UsageStats) (string, []recordKey, []string, []Target, error) {
	_, end := f.state.Context.Range.bounds()
	period := f.state.Context.Range.StartDate + " 至 " + end.AddDate(0, 0, -1).Format("2006-01-02")
	if f.state.Context.Range.CompleteDays == 0 {
		period = "本周尚无完整观测日期"
	}
	lines := []string{title, "统计周期：" + period + "（Asia/Shanghai）", "",
		"人员采用：活跃 " + displayMetric(stats.ActiveMembers, false) + " / 可使用 " + displayMetric(stats.EligibleMembers, false) + "；采用率 " + displayMetric(stats.AdoptionRate, true) + "。",
		"稳定使用 " + displayMetric(stats.StableMembers, false) + " 人；至少两天重复使用 " + displayMetric(stats.RepeatMembers, false) + " 人。",
		"已观测 Token：" + displayToken(stats.Tokens.Total) + "；人员 " + displayToken(stats.Tokens.Human) + "，自动 " + displayToken(stats.Tokens.Automatic) + "，未知归因 " + displayToken(stats.Tokens.Unknown) + "。",
		fmt.Sprintf("已测量 %d 次，未测量 %d 次；失败 %s / 已观测调用 %s。", stats.Tokens.MeasuredRequests, stats.Tokens.UnmeasuredRequests, displayCount(stats.FailedRequests), displayCount(stats.Requests)),
		"使用评价：有帮助 " + displayCount(stats.HelpfulInteractions) + " / 已评价 " + displayCount(stats.RatedInteractions) + "；覆盖率 " + displayMetric(stats.FeedbackCoverage, true) + "。"}
	deps := map[recordKey]bool{}
	required := []string{}
	evidence := []Target{{Kind: "metric", ID: "adoption_rate"}, {Kind: "metric", ID: "total_tokens"}}
	for _, team := range f.state.Scope.TeamIDs {
		if !f.state.Scope.AllTeams {
			deps[recordKey{"team", team}] = true
		}
	}
	for kind, id := range map[string]string{"application": f.state.Context.Filters.ApplicationID, "scene": f.state.Context.Filters.SceneID} {
		if id != "" {
			deps[recordKey{kind, id}] = true
		}
	}
	if slices.Contains(f.state.Scope.Capabilities, "knowledge:read") {
		rows, err := f.knowledgeRows("", nil, nil)
		if err != nil {
			return "", nil, nil, nil, err
		}
		knowledgeDeps := map[recordKey]bool{}
		readable := true
		hasSources := false
		knowledgeIDs := map[string]bool{}
		versionIDs := map[string]bool{}
		for _, row := range rows {
			knowledgeIDs[row.ID] = true
			versionIDs[row.CurrentVersionID] = true
			knowledgeDeps[recordKey{"knowledge", row.ID}] = true
		}
		used := map[string]bool{}
		for _, usage := range f.statistics(analysisSelection{}).facts {
			if usage.Outcome == "succeeded" {
				used[usage.ID] = true
			}
		}
		for _, reference := range f.records["reference"] {
			if !used[reference.str("usage_event_id")] {
				continue
			}
			version := f.records["knowledge_version"][reference.str("knowledge_version_id")]
			if version != nil && knowledgeIDs[version.str("knowledge_id")] {
				versionIDs[version.ID] = true
			}
		}
		orderedVersions := []string{}
		for id := range versionIDs {
			orderedVersions = append(orderedVersions, id)
		}
		sort.Strings(orderedVersions)
		for _, versionID := range orderedVersions {
			version, err := f.readableVersion("", versionID)
			if err != nil {
				return "", nil, nil, nil, err
			}
			allowed, err := f.evidenceReadable(version)
			if err != nil {
				return "", nil, nil, nil, err
			}
			if !allowed {
				readable = false
				break
			}
			knowledgeDeps[recordKey{"knowledge_version", version.ID}] = true
			for _, id := range version.strings("source_ids") {
				knowledgeDeps[recordKey{"source", id}] = true
				hasSources = true
			}
		}
		if readable {
			assets, err := f.assets(analysisSelection{})
			if err != nil {
				return "", nil, nil, nil, err
			}
			lines = append(lines, "知识沉淀：有效 "+displayCount(assets.ValidKnowledge)+"，本期复用 "+displayCount(assets.ReusedValidKnowledge)+"，当前过期仍被引用 "+displayCount(assets.StaleReferencedNow)+"。")
			required = append(required, "knowledge:read")
			if hasSources {
				required = append(required, "sources:read")
			}
			for key := range knowledgeDeps {
				deps[key] = true
			}
			for i, row := range rows {
				if i >= 3 {
					break
				}
				evidence = append(evidence, Target{Kind: "knowledge", ID: row.ID})
			}
		} else {
			lines = append(lines, "知识证据权限不足，本报告未包含知识统计。")
		}
	}
	for _, source := range f.state.Context.Sources {
		if source.Status != "ready" || len(source.MissingDimensions) > 0 {
			lines = append(lines, "数据覆盖提示：来源 "+source.SourceID+" 状态为 "+source.Status+"；缺失信息不按零值处理。")
		}
	}
	lines = append(lines, "自动执行与未知归因用量不代表人员采用；调用成功不等同于业务质量通过。")
	dependencies := []recordKey{}
	for key := range deps {
		dependencies = append(dependencies, key)
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Kind != dependencies[j].Kind {
			return dependencies[i].Kind < dependencies[j].Kind
		}
		return dependencies[i].ID < dependencies[j].ID
	})
	return strings.Join(lines, "\n"), dependencies, required, evidence, nil
}
