//go:build integration

package bi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOperationsStatisticsIncludeRetainedRevisionsAndUnknownUsage(t *testing.T) {
	f := independentFixture(t)
	var org, day, week int64
	require.NoError(t, f.db.QueryRow(`SELECT id FROM desktop_organizations WHERE public_id=$1`, f.org).Scan(&org))
	require.NoError(t, f.db.QueryRow(`INSERT INTO desktop_daily_report_tasks(organization_id,report_date,revision,status,summary,payload) VALUES($1,'2026-10-14',2,'completed','{"content":"PRIVATE_BODY"}','{"snapshots":"PRIVATE_INPUT"}') RETURNING id`, org).Scan(&day))
	require.NoError(t, f.db.QueryRow(`INSERT INTO desktop_period_report_tasks(public_id,organization_id,kind,period_start,period_end,revision,status) VALUES('period_stats',$1,'week','2026-10-05','2026-10-12',1,'failed') RETURNING id`, org).Scan(&week))
	_, err := f.db.Exec(`INSERT INTO desktop_daily_report_executions(task_id,organization_id,report_date,revision,kind,model,round,chunk,attempt,request_id,status,total_tokens,actual_cost) VALUES
 ($1,$2,'2026-10-14',1,'summary','model-a',0,1,1,'stats-old','completed',9007199254740993,0.0000000001),
 ($1,$2,'2026-10-14',2,'summary','model-a',0,1,2,'stats-current','failed',NULL,NULL),
 (NULL,$2,'2026-10-13',1,'summary','model-b',0,1,1,'stats-orphan','completed',0,0)`, day, org)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO desktop_period_report_executions(task_id,organization_id,kind,period_start,period_end,revision,model,round,chunk,attempt,request_id,status,total_tokens,actual_cost) VALUES($1,$2,'week','2026-10-05','2026-10-12',1,'model-b',0,1,1,'stats-week','blocked',7,1.25)`, week, org)
	require.NoError(t, err)
	router := gin.New()
	router.GET("/stats", Boundary, f.h.OperationsReportStatistics)
	router.GET("/executions", Boundary, f.h.OperationsExecutions)
	read := func(path string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), "PRIVATE")
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		return envelope.Data
	}
	all := read("/stats?page_size=1")
	require.Equal(t, "2", all["task_count"])
	require.Equal(t, "4", all["execution_count"])
	require.Equal(t, "9007199254741000", all["total_tokens"])
	require.Equal(t, "1.2500000001", all["actual_cost"])
	require.Equal(t, "1", all["missing_token_count"])
	require.Equal(t, "1", all["missing_cost_count"])
	require.Len(t, all["models"], 2)
	require.Equal(t, map[string]any{"completed": "1", "failed": "1"}, all["task_statuses"])
	dayStats := read("/stats?kind=day")
	require.Equal(t, "3", dayStats["execution_count"])
	require.Equal(t, "9007199254740993", dayStats["total_tokens"])
	require.Equal(t, "0.0000000001", dayStats["actual_cost"])
	weekStats := read("/stats?kind=week")
	require.Equal(t, "1", weekStats["execution_count"])
	missing := read("/stats?organization_id=org_missing")
	require.Equal(t, "0", missing["execution_count"])
	require.Nil(t, missing["actual_cost"])
	rows := read("/executions?page_size=1")
	require.Equal(t, "企业名称", rows["items"].([]any)[0].(map[string]any)["organization_name"])
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats?kind=invalid", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	_, err = f.db.Exec(`UPDATE desktop_organizations SET deleted_at=NOW() WHERE id=$1`, org)
	require.NoError(t, err)
	require.Equal(t, "0", read("/stats")["execution_count"])
}

func TestServiceOperationsOverviewSeparatesEnterpriseMembersFromMiniProgramUsers(t *testing.T) {
	f := independentFixture(t)
	f.s.config.Enabled = false
	_, err := f.db.Exec(`INSERT INTO desktop_members(public_id,organization_id,name,name_normalized,phone,deleted_at)
 SELECT 'member_active',id,'Active member','active member','+8613800000001',NULL FROM desktop_organizations WHERE public_id=$1
 UNION ALL SELECT 'member_deleted',id,'Deleted member','deleted member','+8613800000002',NOW() FROM desktop_organizations WHERE public_id=$1`, f.org)
	require.NoError(t, err)
	router := gin.New()
	router.GET("/overview", Boundary, f.h.OperationsOverview)
	router.GET("/organizations", Boundary, f.h.OperationsOrganizations)
	read := func(path string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		return envelope.Data
	}
	overview := read("/overview")
	require.Equal(t, false, overview["bi_enabled"])
	require.Equal(t, true, overview["desktop_enabled"])
	counts := overview["counts"].(map[string]any)
	require.Equal(t, float64(1), counts["organizations"])
	require.Equal(t, float64(1), counts["members"])
	require.Equal(t, float64(0), counts["users"])
	organization := read("/organizations")["items"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1), organization["members"])
	require.Equal(t, float64(0), organization["users"])
	_, err = f.db.Exec(`UPDATE desktop_organizations SET deleted_at=NOW() WHERE public_id=$1`, f.org)
	require.NoError(t, err)
	counts = read("/overview")["counts"].(map[string]any)
	require.Equal(t, float64(0), counts["organizations"])
	require.Equal(t, float64(0), counts["members"])
	require.Empty(t, read("/organizations")["items"])
}
