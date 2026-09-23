package bi

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Service) buildImport(ctx context.Context, w *importWork, revision int64) error {
	return s.fencedTx(ctx, w, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM bi_data_revisions WHERE id=$1 AND batch_id=$2 FOR UPDATE`, revision, w.ID).Scan(&status); err != nil {
			return err
		}
		if status == "built" {
			return nil
		}
		if status != "staged" {
			return errors.New("invalid import projection state")
		}
		rows, err := tx.QueryContext(ctx, `SELECT entity_id,payload,tombstone,revision::text FROM bi_entity_versions WHERE organization_id=$1 AND data_revision=$2 AND kind='usage' ORDER BY entity_id,revision`, w.Connector.OrganizationID, revision)
		if err != nil {
			return err
		}
		type item struct {
			id       string
			payload  []byte
			deleted  bool
			revision string
		}
		items := []item{}
		for rows.Next() {
			var value item
			if err := rows.Scan(&value.id, &value.payload, &value.deleted, &value.revision); err != nil {
				_ = rows.Close()
				return err
			}
			items = append(items, value)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		for _, value := range items {
			if value.deleted {
				_, err = tx.ExecContext(ctx, `INSERT INTO bi_usage_facts(organization_id,entity_id,data_revision,occurred_at,actor_type,member_id,team_id,
					application_id,application_version_id,scene_id,requested_model,outcome,duration_ms,clock_skew,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,tombstone,entity_revision)
					SELECT f.organization_id,f.entity_id,$3,f.occurred_at,f.actor_type,f.member_id,f.team_id,f.application_id,f.application_version_id,f.scene_id,
					f.requested_model,f.outcome,f.duration_ms,f.clock_skew,f.input_tokens,f.output_tokens,f.cache_read_tokens,f.cache_write_tokens,TRUE,$4::numeric
					FROM bi_usage_facts f JOIN bi_data_revisions d ON d.id=f.data_revision
					WHERE f.organization_id=$1 AND f.entity_id=$2 AND (d.status='published' OR (f.data_revision=$3 AND f.entity_revision<$4::numeric))
					ORDER BY f.data_revision DESC,f.entity_revision DESC LIMIT 1`, w.Connector.OrganizationID, value.id, revision, value.revision)
			} else {
				usage, decodeErr := decodeUsage(value.payload)
				if decodeErr != nil {
					return decodeErr
				}
				tokens, tokenErr := NormalizeTokens(usage.Tokens)
				if tokenErr != nil {
					return tokenErr
				}
				_, err = tx.ExecContext(ctx, `INSERT INTO bi_usage_facts(organization_id,entity_id,data_revision,occurred_at,actor_type,member_id,team_id,
					application_id,application_version_id,scene_id,requested_model,outcome,duration_ms,clock_skew,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,entity_revision)
					VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, w.Connector.OrganizationID, usage.ID, revision, usage.OccurredAt.UTC(), usage.ActorType,
					usage.MemberID, usage.TeamID, usage.ApplicationID, usage.ApplicationVersionID, usage.SceneID, usage.RequestedModel, usage.Outcome, usage.DurationMS,
					usage.OccurredAt.After(w.ReceivedAt), tokens.Input, tokens.Output, tokens.CacheRead, tokens.CacheWrite, value.revision)
			}
			if err != nil {
				return err
			}
		}
		if err := buildDailyUsage(ctx, tx, w.Connector.OrganizationID, revision); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_ingestion_outbox SET completed_at=NOW() WHERE data_revision=$1`, revision); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE bi_data_revisions SET status='built' WHERE id=$1`, revision)
		return err
	})
}

func (s *Service) publishImport(ctx context.Context, w *importWork, revision int64) error {
	return s.fencedTx(ctx, w, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM bi_data_revisions WHERE id=$1 AND batch_id=$2 FOR UPDATE`, revision, w.ID).Scan(&status); err != nil {
			return err
		}
		if status != "built" {
			return errors.New("cannot publish an incomplete import view")
		}
		var checkpoint *string
		if err := tx.QueryRowContext(ctx, `SELECT checkpoint FROM bi_connector_sources WHERE organization_id=$1 AND source_id=$2 FOR UPDATE`, w.Connector.OrganizationID, w.Connector.SourceID).Scan(&checkpoint); err != nil {
			return err
		}
		if (checkpoint == nil) != (w.Batch.ExpectedCheckpoint == nil) || (checkpoint != nil && *checkpoint != *w.Batch.ExpectedCheckpoint) {
			return errors.New("source checkpoint changed outside the import protocol")
		}
		now := s.now().UTC()
		_, err := tx.ExecContext(ctx, `UPDATE bi_connector_sources SET checkpoint=$3,complete_through=$4,history_start_date=$5,
			initial_backfill_complete=$6,last_applied_at=$7,data_revision=$8 WHERE organization_id=$1 AND source_id=$2`, w.Connector.OrganizationID, w.Connector.SourceID,
			w.Batch.Checkpoint, w.Batch.CompleteThrough, w.Batch.HistoryStartDate, w.Batch.InitialBackfillComplete, now, revision)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_data_revisions SET status='published',published_at=$2 WHERE id=$1`, revision, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE bi_import_batches SET status='applied',applied_at=$2,checkpoint=$3,data_revision=$4,lease_owner=NULL,lease_until=NULL WHERE id=$1`, w.ID, now, w.Batch.Checkpoint, revision)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM bi_acl_denials denial USING bi_entity_versions v WHERE v.data_revision=$1
			AND denial.organization_id=v.organization_id AND denial.kind=v.kind AND denial.entity_id=v.entity_id
			AND denial.revision<=v.revision AND denial.data_revision<=$1`, revision)
		return err
	})
}
