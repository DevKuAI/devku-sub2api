//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type usageQueryPlan struct {
	Loops       float64          `json:"Actual Loops"`
	JoinRemoved float64          `json:"Rows Removed by Join Filter"`
	JoinFilter  string           `json:"Join Filter"`
	Plans       []usageQueryPlan `json:"Plans"`
}

func (p usageQueryPlan) rejectedEventJoinRows() float64 {
	count := float64(0)
	if strings.Contains(p.JoinFilter, "entity_id") {
		count = p.Loops * p.JoinRemoved
	}
	for _, child := range p.Plans {
		count += child.rejectedEventJoinRows()
	}
	return count
}

func TestPeriodUsageQueryAvoidsQuadraticWorkAfterBulkImport(t *testing.T) {
	s, c, p, snapshot := analysisFixture(t)
	ctx := context.Background()
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	const calls = 10000
	// Seed the exact read-table shape directly; this test targets query planning,
	// while ingestion atomicity is covered by separate integration tests.
	_, err = integrationDB.Exec(`INSERT INTO bi_entities(organization_id,kind,id,source_id)
		SELECT $1,'usage','test:query-plan:'||n,$2 FROM generate_series(1,$3) n`, c.OrganizationID, c.SourceID, calls)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO bi_usage_facts(organization_id,entity_id,data_revision,entity_revision,occurred_at,actor_type,member_id,team_id,
		application_id,application_version_id,scene_id,requested_model,outcome,duration_ms,clock_skew,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens)
		SELECT f.organization_id,'test:query-plan:'||n,f.data_revision,1,f.occurred_at,f.actor_type,f.member_id,f.team_id,
		f.application_id,f.application_version_id,f.scene_id,f.requested_model,f.outcome,f.duration_ms,f.clock_skew,f.input_tokens,f.output_tokens,f.cache_read_tokens,f.cache_write_tokens
		FROM bi_usage_facts f CROSS JOIN generate_series(1,$2) n WHERE f.organization_id=$1 AND f.entity_id='test:automatic:1'`, c.OrganizationID, calls)
	require.NoError(t, err)
	start, _ := state.Context.ComparisonRange.bounds()
	_, end := state.Context.Range.bounds()
	for _, analyze := range []bool{false, true} {
		name := "before_analyze"
		if analyze {
			name = "after_analyze"
			_, err = integrationDB.Exec(`ANALYZE bi_usage_facts,bi_entities`)
			require.NoError(t, err)
		}
		t.Run(name, func(t *testing.T) {
			var raw []byte
			require.NoError(t, integrationDB.QueryRowContext(ctx, `EXPLAIN (ANALYZE,TIMING OFF,FORMAT JSON) `+usageFactsQuery(true), c.OrganizationID, state.Revision, start, end).Scan(&raw))
			var plan []struct{ Plan usageQueryPlan }
			require.NoError(t, json.Unmarshal(raw, &plan))
			require.Len(t, plan, 1)
			// Measure event-ID joins, excluding small revision-catalog lookups.
			// A work bound detects the regression independently of CI wall-clock load.
			require.Less(t, plan[0].Plan.rejectedEventJoinRows(), float64(calls*10))
			facts, err := s.readUsageFacts(ctx, state, &start, &end)
			require.NoError(t, err)
			require.Len(t, facts, calls+10)
		})
	}
}
