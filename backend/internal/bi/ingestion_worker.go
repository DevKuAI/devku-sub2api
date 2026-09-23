package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lib/pq"
)

var errLeaseLost = errors.New("import lease no longer belongs to this worker")

type importWork struct {
	ID         string
	Owner      string
	Connector  Connector
	Batch      ImportBatch
	ReceivedAt time.Time
	Attempt    int
}

type Worker struct {
	service *Service
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

func (w *Worker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil || w.service.db == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		var lastCleanup time.Time
		for {
			if time.Since(lastCleanup) >= time.Hour {
				cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				_, err := w.service.CleanupEphemeralState(cleanupCtx)
				cancel()
				lastCleanup = time.Now()
				if err != nil && ctx.Err() == nil {
					slog.ErrorContext(ctx, "BI temporary-state cleanup failed", "error_type", fmt.Sprintf("%T", err))
				}
			}
			processed, err := w.service.ProcessNextImport(ctx)
			reportProcessed, reportErr := w.service.ProcessNextReport(ctx)
			processed = processed || reportProcessed
			if err == nil {
				err = reportErr
			}
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				slog.ErrorContext(ctx, "BI import processing failed", "error_type", fmt.Sprintf("%T", err))
			}
			if processed && err == nil {
				continue
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (w *Worker) Stop() {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *Service) claimImport(ctx context.Context) (*importWork, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	w := &importWork{Owner: randomToken("lease_")}
	var payload, kinds []byte
	err = tx.QueryRowContext(ctx, `SELECT b.id,b.organization_id,b.source_id,b.payload,b.received_at,b.attempt_count,s.namespace,s.allowed_kinds
		FROM bi_import_batches b JOIN bi_connector_sources s ON s.organization_id=b.organization_id AND s.source_id=b.source_id
		WHERE b.status IN ('queued','validating') AND (b.lease_until IS NULL OR b.lease_until<$1)
		AND NOT EXISTS(SELECT 1 FROM bi_import_batches other WHERE other.organization_id=b.organization_id AND other.status='validating' AND other.id<>b.id)
		ORDER BY b.received_at,b.id LIMIT 1 FOR UPDATE OF b SKIP LOCKED`, s.now().UTC()).
		Scan(&w.ID, &w.Connector.OrganizationID, &w.Connector.SourceID, &payload, &w.ReceivedAt, &w.Attempt, &w.Connector.Namespace, &kinds)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(payload, &w.Batch); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(kinds, &w.Connector.AllowedKinds); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE bi_import_batches SET status='validating',lease_owner=$2,lease_until=$3,attempt_count=attempt_count+1 WHERE id=$1`, w.ID, w.Owner, s.now().UTC().Add(2*time.Minute))
	if err != nil {
		var pg *pq.Error
		if errors.As(err, &pg) && pg.Constraint == "bi_import_batches_publisher_per_organization" {
			return nil, nil
		}
		return nil, err
	}
	w.Attempt++
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) fencedTx(ctx context.Context, w *importWork, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var owns bool
	err = tx.QueryRowContext(ctx, `SELECT status='validating' AND lease_owner=$2 AND lease_until>$3 FROM bi_import_batches WHERE id=$1 FOR UPDATE`, w.ID, w.Owner, s.now().UTC()).Scan(&owns)
	if err != nil {
		return err
	}
	if !owns {
		return errLeaseLost
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ProcessNextImport resumes a durable stage after process or database failures.
func (s *Service) ProcessNextImport(ctx context.Context) (bool, error) {
	w, err := s.claimImport(ctx)
	if err != nil || w == nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	err = s.processImport(workCtx, w)
	if err == nil || errors.Is(err, errLeaseLost) {
		return true, nil
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
	defer cleanupCancel()
	if w.Attempt >= 5 {
		return true, s.rejectImport(cleanupCtx, w, []ImportError{{Code: "INTERNAL_ERROR", Message: "Import processing failed; retry with a new idempotency key"}})
	}
	_, releaseErr := s.db.ExecContext(cleanupCtx, `UPDATE bi_import_batches SET lease_until=$3 WHERE id=$1 AND lease_owner=$2 AND status='validating'`, w.ID, w.Owner, s.now().UTC().Add(5*time.Second))
	if releaseErr != nil {
		return true, releaseErr
	}
	return true, err
}

func (s *Service) processImport(ctx context.Context, w *importWork) error {
	revision, problems, err := s.stageImport(ctx, w)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return s.rejectImport(ctx, w, problems)
	}
	if err := s.buildImport(ctx, w, revision); err != nil {
		return err
	}
	return s.publishImport(ctx, w, revision)
}

func (s *Service) rejectImport(ctx context.Context, w *importWork, problems []ImportError) error {
	return s.fencedTx(ctx, w, func(tx *sql.Tx) error {
		raw, _ := json.Marshal(problems)
		if _, err := tx.ExecContext(ctx, `UPDATE bi_data_revisions SET status='rejected' WHERE batch_id=$1 AND status<>'published'`, w.ID); err != nil {
			return err
		}
		// Keep independently committed ACL denials until a valid repair is published.
		_, err := tx.ExecContext(ctx, `UPDATE bi_import_batches SET status='rejected',errors=$2,lease_owner=NULL,lease_until=NULL WHERE id=$1`, w.ID, string(raw))
		return err
	})
}
