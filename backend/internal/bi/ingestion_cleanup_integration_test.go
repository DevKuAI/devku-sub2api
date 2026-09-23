//go:build integration

package bi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectedImportDiscardsUnpublishedProjectionsAndPreservesDenials(t *testing.T) {
	for _, built := range []bool{false, true} {
		stage := "staged"
		if built {
			stage = "built"
		}
		t.Run(stage, func(t *testing.T) {
			s, c, p, snapshot, source := contentFixture(t)
			ctx := context.Background()
			source["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
			usage := acceptancePayload(t, s, c, "usage", "test:one:2")
			usage["tokens"] = map[string]any{"encoding": "exclusive_buckets", "input": "25", "output": "0", "cache_read": "0", "cache_write": "0"}
			version := acceptancePayload(t, s, c, "knowledge_version", "test:knowledge:v1")
			version["id"], version["version"] = "test:knowledge:v2", "2"
			body := completeBatch(t, "content-seed", "projection-repair", []any{
				map[string]any{"kind": "source", "revision": 2, "payload": source},
				map[string]any{"kind": "usage", "revision": 2, "payload": usage},
				analyticRecord("knowledge_version", version),
			})
			key := randomToken("idem_")
			job, err := s.SubmitImport(ctx, c, key, body)
			require.NoError(t, err)
			w, err := s.claimImport(ctx)
			require.NoError(t, err)
			require.NotNil(t, w)
			require.Equal(t, job.ID, w.ID)
			revision, problems, err := s.stageImport(ctx, w)
			require.NoError(t, err)
			require.Empty(t, problems)
			if built {
				require.NoError(t, s.buildImport(ctx, w, revision))
				var groups int
				require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_usage_daily WHERE data_revision=$1`, revision).Scan(&groups))
				require.Positive(t, groups)
			}
			require.NoError(t, s.rejectImport(ctx, w, []ImportError{{Code: "INTERNAL_ERROR", Message: "Simulated unrecoverable publication failure"}}))
			for _, projection := range []struct{ table, key string }{
				{"bi_entity_versions", "data_revision"}, {"bi_usage_facts", "data_revision"},
				{"bi_usage_daily", "data_revision"}, {"bi_usage_day_revisions", "data_revision"},
				{"bi_source_version_links", "parent_data_revision"}, {"bi_ingestion_outbox", "data_revision"},
			} {
				var count int
				require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM `+projection.table+` WHERE `+projection.key+`=$1`, revision).Scan(&count))
				require.Zero(t, count, projection.table)
			}
			var receipts, denials int
			require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_record_receipts WHERE batch_id=$1`, job.ID).Scan(&receipts))
			require.Zero(t, receipts)
			require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_acl_denials WHERE data_revision=$1`, revision).Scan(&denials))
			require.Equal(t, 1, denials)
			var status string
			require.NoError(t, integrationDB.QueryRow(`SELECT status FROM bi_data_revisions WHERE id=$1`, revision).Scan(&status))
			require.Equal(t, "rejected", status)
			final, err := s.ImportJob(ctx, c, job.ID)
			require.NoError(t, err)
			require.Equal(t, "rejected", final.Status)
			require.Equal(t, "INTERNAL_ERROR", final.Errors[0].Code)
			checkpoint, err := s.ImportCheckpoint(ctx, c, c.SourceID)
			require.NoError(t, err)
			require.Equal(t, "content-seed", *checkpoint.Checkpoint)
			f := contentFrame(t, s, c, p, snapshot)
			require.Equal(t, "1500", *f.statistics(analysisSelection{}).stats.Tokens.Total)
			_, err = f.sourceDetail("knowledge_version", "test:knowledge:v1", "test:source")
			require.ErrorIs(t, err, ErrNotFound)
			replay, err := s.SubmitImport(ctx, c, key, body)
			require.NoError(t, err)
			require.Equal(t, job.ID, replay.ID)
			repair := applyImport(t, s, c, body)
			require.Equal(t, "applied", repair.Status, repair.Errors)
			require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_acl_denials WHERE organization_id=$1`, c.OrganizationID).Scan(&denials))
			require.Zero(t, denials)
			require.Equal(t, "1500", *contentFrame(t, s, c, p, snapshot).statistics(analysisSelection{}).stats.Tokens.Total)
		})
	}
}
