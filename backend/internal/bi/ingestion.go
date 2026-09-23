package bi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
	"unicode/utf8"
)

func (s *Service) SubmitImport(ctx context.Context, connector Connector, key string, raw []byte) (ImportJob, error) {
	if len(raw) > 2<<20 {
		return ImportJob{}, apiError(413, "PAYLOAD_TOO_LARGE", "Import payload exceeds 2 MiB")
	}
	if !utf8.ValidString(key) || utf8.RuneCountInString(key) < 16 || utf8.RuneCountInString(key) > 128 {
		return ImportJob{}, invalid("Idempotency-Key", "An idempotency key of 16–128 characters is required")
	}
	if err := validateSchema("ImportBatch", raw); err != nil {
		return ImportJob{}, err
	}
	var batch ImportBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return ImportJob{}, invalid("body", "Invalid import batch")
	}
	if batch.SourceID != connector.SourceID {
		return ImportJob{}, ErrForbidden
	}
	for _, record := range batch.Records {
		if !slices.Contains(connector.AllowedKinds, record.Kind) || (record.Kind == "tombstone" && !slices.Contains(connector.AllowedKinds, record.EntityKind)) {
			return ImportJob{}, ErrForbidden
		}
	}
	// Map encoding removes whitespace/property-order differences without rounding numbers.
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return ImportJob{}, invalid("body", "Invalid import batch")
	}
	payloadHash := tokenHash(string(canonical))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportJob{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var checkpoint sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT checkpoint FROM bi_connector_sources WHERE organization_id=$1 AND source_id=$2 FOR UPDATE`, connector.OrganizationID, connector.SourceID).Scan(&checkpoint)
	if errors.Is(err, sql.ErrNoRows) {
		return ImportJob{}, ErrNotFound
	}
	if err != nil {
		return ImportJob{}, err
	}
	var existingID, existingHash string
	err = tx.QueryRowContext(ctx, `SELECT id,payload_hash FROM bi_import_batches WHERE organization_id=$1 AND source_id=$2 AND idempotency_hash=$3`, connector.OrganizationID, connector.SourceID, tokenHash(key)).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != payloadHash {
			return ImportJob{}, apiError(409, "IDEMPOTENCY_CONFLICT", "Idempotency key was used for different content")
		}
		job, readErr := readImportJob(ctx, tx, connector, existingID)
		if readErr != nil {
			return ImportJob{}, readErr
		}
		job.Status = "queued"
		job.AppliedAt, job.Checkpoint, job.DataRevision = nil, nil, nil
		job.Errors = []ImportError{}
		return job, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ImportJob{}, err
	}
	var busy bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM bi_import_batches WHERE organization_id=$1 AND source_id=$2 AND status IN ('queued','validating'))`, connector.OrganizationID, connector.SourceID).Scan(&busy)
	if err != nil {
		return ImportJob{}, err
	}
	if busy {
		return ImportJob{}, apiError(409, "IMPORT_BUSY", "This source already has an in-flight import")
	}
	if (batch.ExpectedCheckpoint == nil) != (!checkpoint.Valid) || (batch.ExpectedCheckpoint != nil && *batch.ExpectedCheckpoint != checkpoint.String) {
		return ImportJob{}, apiError(409, "CHECKPOINT_CONFLICT", "Read the current checkpoint before submitting")
	}
	var reused bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM bi_import_batches WHERE organization_id=$1 AND source_id=$2 AND checkpoint=$3 AND status='applied')`, connector.OrganizationID, connector.SourceID, batch.Checkpoint).Scan(&reused)
	if err != nil {
		return ImportJob{}, err
	}
	if reused || (checkpoint.Valid && checkpoint.String == batch.Checkpoint) {
		return ImportJob{}, apiError(409, "CHECKPOINT_CONFLICT", "Checkpoint was already applied")
	}
	job := ImportJob{ID: randomToken("bij_"), SourceID: connector.SourceID, Status: "queued", ReceivedAt: s.now().UTC().Truncate(time.Microsecond), Errors: []ImportError{}}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_import_batches(id,organization_id,source_id,idempotency_hash,payload_hash,payload,received_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, job.ID, connector.OrganizationID, connector.SourceID, tokenHash(key), payloadHash, string(canonical), job.ReceivedAt)
	if err != nil {
		return ImportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportJob{}, err
	}
	return job, nil
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func readImportJob(ctx context.Context, q queryer, connector Connector, id string) (ImportJob, error) {
	var job ImportJob
	var rawErrors []byte
	err := q.QueryRowContext(ctx, `SELECT b.id,b.source_id,b.status,b.received_at,b.applied_at,b.checkpoint,r.public_id,b.errors
		FROM bi_import_batches b LEFT JOIN bi_data_revisions r ON r.id=b.data_revision
		WHERE b.id=$1 AND b.organization_id=$2 AND b.source_id=$3`, id, connector.OrganizationID, connector.SourceID).
		Scan(&job.ID, &job.SourceID, &job.Status, &job.ReceivedAt, &job.AppliedAt, &job.Checkpoint, &job.DataRevision, &rawErrors)
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrNotFound
	}
	if err != nil {
		return job, err
	}
	if err := json.Unmarshal(rawErrors, &job.Errors); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) ImportJob(ctx context.Context, connector Connector, id string) (ImportJob, error) {
	return readImportJob(ctx, s.db, connector, id)
}

func (s *Service) ImportCheckpoint(ctx context.Context, connector Connector, sourceID string) (Checkpoint, error) {
	if sourceID != connector.SourceID {
		return Checkpoint{}, ErrNotFound
	}
	var result Checkpoint
	err := s.db.QueryRowContext(ctx, `SELECT s.source_id,s.checkpoint,s.last_applied_at,r.public_id,s.complete_through,
		to_char(s.history_start_date,'YYYY-MM-DD'),s.initial_backfill_complete FROM bi_connector_sources s
		LEFT JOIN bi_data_revisions r ON r.id=s.data_revision WHERE s.organization_id=$1 AND s.source_id=$2`, connector.OrganizationID, sourceID).
		Scan(&result.SourceID, &result.Checkpoint, &result.LastAppliedAt, &result.DataRevision, &result.CompleteThrough, &result.HistoryStartDate, &result.InitialBackfillComplete)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	return result, err
}
