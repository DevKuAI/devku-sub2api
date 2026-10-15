package bi

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// Statistics include every retained execution, including retries, old revisions
// and executions whose task was removed. Report bodies are never selected.
const operationsStatisticsSQL = `WITH tasks AS (
 SELECT o.public_id AS organization_id,'day'::text AS kind,d.status
 FROM desktop_daily_report_tasks d JOIN desktop_organizations o ON o.id=d.organization_id WHERE o.deleted_at IS NULL
 UNION ALL
 SELECT o.public_id,p.kind,p.status
 FROM desktop_period_report_tasks p JOIN desktop_organizations o ON o.id=p.organization_id WHERE o.deleted_at IS NULL
), filtered_tasks AS (
 SELECT * FROM tasks WHERE ($1='' OR organization_id=$1) AND ($2='' OR kind=$2)
), executions AS (` + executionUnion + `), filtered_executions AS (
 SELECT organization_id,model,status,total_tokens::numeric AS total_tokens,actual_cost
 FROM executions WHERE ($1='' OR organization_id=$1) AND ($2='' OR kind=$2)
), task_statuses AS (
 SELECT status,COUNT(*)::text AS count FROM filtered_tasks GROUP BY status
), execution_statuses AS (
 SELECT status,COUNT(*)::text AS count FROM filtered_executions GROUP BY status
), models AS (
 SELECT model,COUNT(*)::text AS execution_count,SUM(total_tokens)::text AS total_tokens,SUM(actual_cost)::text AS actual_cost,
 COUNT(*) FILTER(WHERE total_tokens IS NULL)::text AS missing_token_count,
 COUNT(*) FILTER(WHERE actual_cost IS NULL)::text AS missing_cost_count
 FROM filtered_executions GROUP BY model
)
SELECT json_build_object(
 'task_count',(SELECT COUNT(*)::text FROM filtered_tasks),
 'organization_count',(SELECT COUNT(*)::text FROM (SELECT organization_id FROM filtered_tasks UNION SELECT organization_id FROM filtered_executions) organizations),
 'execution_count',(SELECT COUNT(*)::text FROM filtered_executions),
 'total_tokens',(SELECT SUM(total_tokens)::text FROM filtered_executions),
 'actual_cost',(SELECT SUM(actual_cost)::text FROM filtered_executions),
 'missing_token_count',(SELECT COUNT(*) FILTER(WHERE total_tokens IS NULL)::text FROM filtered_executions),
 'missing_cost_count',(SELECT COUNT(*) FILTER(WHERE actual_cost IS NULL)::text FROM filtered_executions),
 'task_statuses',COALESCE((SELECT json_object_agg(status,count) FROM task_statuses),'{}'::json),
 'execution_statuses',COALESCE((SELECT json_object_agg(status,count) FROM execution_statuses),'{}'::json),
 'models',COALESCE((SELECT json_agg(models ORDER BY actual_cost::numeric DESC NULLS LAST,model) FROM models),'[]'::json)
)`

func (h *Handler) OperationsReportStatistics(c *gin.Context) {
	kind := c.Query("kind")
	if kind != "" && kind != "day" && kind != "week" && kind != "month" {
		WriteError(c, invalid("kind", "Invalid task kind"))
		return
	}
	var result json.RawMessage
	err := h.service.db.QueryRowContext(c.Request.Context(), operationsStatisticsSQL, c.Query("organization_id"), kind).Scan(&result)
	adminRespond(c, result, err)
}
