package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

func validateWatermark(batch ImportBatch, current Checkpoint, received time.Time) []ImportError {
	fail := func(message string) []ImportError {
		return []ImportError{{Code: "INVALID_WATERMARK", Message: message}}
	}
	start, err := time.ParseInLocation("2006-01-02", batch.HistoryStartDate, shanghai)
	if err != nil {
		return fail("Invalid history start date")
	}
	if batch.CompleteThrough == nil {
		if batch.InitialBackfillComplete || current.CompleteThrough != nil {
			return fail("Established completeness cannot be cleared")
		}
	} else {
		if batch.CompleteThrough.After(received) {
			return fail("Completeness is later than batch receipt")
		}
		if start.After(batch.CompleteThrough.In(shanghai)) {
			return fail("History begins after completeness")
		}
		if current.CompleteThrough != nil && batch.CompleteThrough.Before(*current.CompleteThrough) {
			return fail("Completeness cannot move backwards")
		}
	}
	if current.InitialBackfillComplete {
		if !batch.InitialBackfillComplete {
			return fail("Completed backfill cannot be reset")
		}
		if current.HistoryStartDate != nil && batch.HistoryStartDate > *current.HistoryStartDate {
			return fail("Known history cannot be shortened")
		}
	}
	return nil
}

func (s *Service) stageImport(ctx context.Context, w *importWork) (int64, []ImportError, error) {
	var revision int64
	var problems []ImportError
	err := s.fencedTx(ctx, w, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT id FROM bi_data_revisions WHERE batch_id=$1 AND status<>'rejected'`, w.ID).Scan(&revision)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var current Checkpoint
		err = tx.QueryRowContext(ctx, `SELECT checkpoint,complete_through,to_char(history_start_date,'YYYY-MM-DD'),initial_backfill_complete
			FROM bi_connector_sources WHERE organization_id=$1 AND source_id=$2 FOR UPDATE`, w.Connector.OrganizationID, w.Connector.SourceID).
			Scan(&current.Checkpoint, &current.CompleteThrough, &current.HistoryStartDate, &current.InitialBackfillComplete)
		if err != nil {
			return err
		}
		problems = validateWatermark(w.Batch, current, w.ReceivedAt)
		if len(problems) > 0 {
			return nil
		}
		v, issues, err := prepareBatchView(ctx, tx, w.Connector, w.Batch, w.ReceivedAt)
		if err != nil {
			return err
		}
		if len(issues) > 0 {
			problems = issues
			return nil
		}
		problems, err = v.validateRecords()
		if err != nil || len(problems) > 0 {
			return err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO bi_data_revisions(public_id,organization_id,batch_id,status) VALUES($1,$2,$3,'staged') RETURNING id`, randomToken("rev_"), w.Connector.OrganizationID, w.ID).Scan(&revision)
		if err != nil {
			return err
		}
		for _, r := range v.receipts {
			r.DataRevision = revision
			if err := stageRecord(ctx, tx, w.Connector, revision, r); err != nil {
				return err
			}
		}
		for _, r := range v.receipts {
			if _, err := tx.ExecContext(ctx, `INSERT INTO bi_record_receipts(organization_id,kind,entity_id,revision,payload_hash,batch_id) VALUES($1,$2,$3,$4,$5,$6)`,
				w.Connector.OrganizationID, r.Kind, r.ID, r.Revision, r.Hash, w.ID); err != nil {
				return err
			}
		}
		for _, r := range v.updates {
			if err := stageRestrictions(ctx, tx, v, r, revision); err != nil {
				return err
			}
			if r.Kind == "case" {
				if err := stageSourceLinks(ctx, tx, v, r, revision); err != nil {
					return err
				}
			}
		}
		for _, r := range v.receipts {
			if r.Kind == "knowledge_version" {
				if err := stageSourceLinks(ctx, tx, v, r, revision); err != nil {
					return err
				}
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bi_ingestion_outbox(data_revision) VALUES($1)`, revision)
		return err
	})
	return revision, problems, err
}

func stageRecord(ctx context.Context, tx *sql.Tx, connector Connector, revision int64, r *record) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_entities(organization_id,kind,id,source_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, connector.OrganizationID, r.Kind, r.ID, connector.SourceID); err != nil {
		return err
	}
	var sourceID string
	if err := tx.QueryRowContext(ctx, `SELECT source_id FROM bi_entities WHERE organization_id=$1 AND kind=$2 AND id=$3`, connector.OrganizationID, r.Kind, r.ID).Scan(&sourceID); err != nil {
		return err
	}
	if sourceID != connector.SourceID {
		return errRecord("FORBIDDEN_SOURCE", "Entity belongs to another source")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO bi_entity_versions(organization_id,kind,entity_id,revision,data_revision,payload,payload_hash,tombstone) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		connector.OrganizationID, r.Kind, r.ID, r.Revision, revision, string(r.Raw), r.Hash, r.Deleted)
	return err
}

func stageRestrictions(ctx context.Context, tx *sql.Tx, v *batchView, r *record, revision int64) error {
	acl, hasACL := r.Payload["acl"]
	if !r.Deleted && !hasACL {
		return nil
	}
	previous, err := v.published(r.recordKey)
	if err != nil {
		return err
	}
	if previous == nil {
		return nil
	}
	var value any
	if hasACL {
		value = string(acl)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_acl_denials(organization_id,kind,entity_id,data_revision,revision,acl,deny_all) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		v.connector.OrganizationID, r.Kind, r.ID, revision, r.Revision, value, r.Deleted)
	return err
}

func stageSourceLinks(ctx context.Context, tx *sql.Tx, v *batchView, r *record, revision int64) error {
	if r.Deleted || (r.Kind != "knowledge_version" && r.Kind != "case") {
		return nil
	}
	previous, err := v.published(r.recordKey)
	if err != nil {
		return err
	}
	previous, err = v.historicalContent(r.recordKey, previous)
	if err != nil {
		return err
	}
	if previous != nil && !previous.Deleted && r.Kind == "knowledge_version" {
		_, err := tx.ExecContext(ctx, `INSERT INTO bi_source_version_links(organization_id,parent_kind,parent_id,parent_data_revision,source_id,source_version_id,source_data_revision)
			SELECT organization_id,parent_kind,parent_id,$4,source_id,source_version_id,source_data_revision FROM bi_source_version_links
			WHERE organization_id=$1 AND parent_kind=$2 AND parent_id=$3 AND parent_data_revision=$5 ON CONFLICT DO NOTHING`, v.connector.OrganizationID, r.Kind, r.ID, revision, previous.DataRevision)
		return err
	}
	for _, id := range r.strings("source_ids") {
		source, err := v.reference("source", id)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bi_source_version_links(organization_id,parent_kind,parent_id,parent_data_revision,source_id,source_version_id,source_data_revision)
			VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, v.connector.OrganizationID, r.Kind, r.ID, revision, id, source.str("version_id"), source.DataRevision)
		if err != nil {
			return err
		}
	}
	return nil
}

type UsageRecord struct {
	ID                   string    `json:"id"`
	OccurredAt           time.Time `json:"occurred_at"`
	ActorType            string    `json:"actor_type"`
	MemberID             *string   `json:"member_id"`
	TeamID               *string   `json:"team_id"`
	ApplicationID        *string   `json:"application_id"`
	ApplicationVersionID *string   `json:"application_version_id"`
	SceneID              *string   `json:"scene_id"`
	RequestedModel       string    `json:"requested_model"`
	Outcome              string    `json:"outcome"`
	DurationMS           *int64    `json:"duration_ms"`
	RetryOf              *string   `json:"retry_of"`
	Tokens               RawTokens `json:"tokens"`
}

func decodeUsage(raw []byte) (UsageRecord, error) {
	var usage UsageRecord
	err := json.Unmarshal(raw, &usage)
	return usage, err
}
