//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type nativePlanRecorder struct {
	sqlExecutor
	query string
	args  []any
}

func (r *nativePlanRecorder) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.Contains(query, "LIMIT $6 OFFSET $7") {
		r.query = query
		r.args = args
	}
	return r.sqlExecutor.QueryContext(ctx, query, args...)
}

// Keep measurements informative; correctness must not depend on machine speed.
func TestDesktopNativeMemberQueryProfile(t *testing.T) {
	f := newDesktopRepositoryFixture(t, "nativeprofile", 40, true)
	ctx := context.Background()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "profile-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM usage_logs WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	for i := 0; i < 40; i++ {
		member := f.createMember(t, fmt.Sprintf("profile%d", i), int64(i+1))
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,billing_mode,created_at)
  SELECT $1,$2,$3,$4||g,'profile-model',100,20,'token',NOW()-(g%90)*INTERVAL '1 day' FROM generate_series(1,1000) g`, f.user.ID, *member.CurrentAPIKeyID, account.ID, uuid.NewString())
		require.NoError(t, err)
	}
	recorder := &nativePlanRecorder{sqlExecutor: integrationDB}
	repo := newUsageLogRepositoryWithSQL(integrationEntClient, recorder)
	for _, days := range []int{7, 30, 90} {
		selected, err := (service.DesktopAnalyticsRangeInput{Days: days, Timezone: "Asia/Shanghai"}).Resolve(time.Now())
		require.NoError(t, err)
		var timings []float64
		for i := 0; i < 12; i++ {
			started := time.Now()
			items, total, err := repo.NativeMembers(ctx, f.organization.ID, selected, "", "", 1, 20, true)
			require.NoError(t, err)
			require.Len(t, items, 20)
			require.EqualValues(t, 40, total)
			timings = append(timings, float64(time.Since(started).Microseconds())/1000)
		}
		slices.Sort(timings)
		var raw []byte
		require.NoError(t, integrationDB.QueryRowContext(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+recorder.query, recorder.args...).Scan(&raw))
		var plan []map[string]any
		require.NoError(t, json.Unmarshal(raw, &plan))
		t.Logf("native-member profile rows=40000 members=40 days=%d p50_ms=%.3f p95_ms=%.3f execution_ms=%v planning_ms=%v plan=%s", days, timings[len(timings)/2], timings[len(timings)-1], plan[0]["Execution Time"], plan[0]["Planning Time"], raw)
	}
}
