package accounting

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Operation struct {
	Actor     int64
	Org       int64
	Kind      string
	Object    string
	Action    string
	Key       string
	RequestID string
	Payload   any
}

func hash(data []byte) string { v := sha256.Sum256(data); return hex.EncodeToString(v[:]) }
func (e *Engine) write(ctx context.Context, op Operation, fn func(*sql.Tx) (any, error)) (json.RawMessage, bool, error) {
	key, err := service.NormalizeIdempotencyKey(op.Key)
	if err != nil {
		return nil, false, err
	}
	if key == "" {
		return nil, false, service.ErrIdempotencyKeyRequired
	}
	body, err := json.Marshal(op.Payload)
	if err != nil {
		return nil, false, ErrInvalid
	}
	fingerprint := hash(body)
	operation := op.Kind + ":" + op.Action + ":" + op.Object
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock the operation independently of organization ownership so key conflicts cannot race.
	_, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", operation+":"+hash([]byte(key))+":"+decimalActor(op.Actor))
	if err != nil {
		return nil, false, err
	}
	var existingFingerprint string
	var result []byte
	err = tx.QueryRowContext(ctx, "SELECT fingerprint,response FROM accounting_operation_results WHERE actor_id=$1 AND operation=$2 AND key_hash=$3", op.Actor, operation, hash([]byte(key))).Scan(&existingFingerprint, &result)
	if err == nil {
		if existingFingerprint != fingerprint {
			return nil, false, service.ErrIdempotencyKeyConflict
		}
		return result, true, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}
	var org int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM desktop_organizations WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", op.Org).Scan(&org)
	if err == sql.ErrNoRows {
		return nil, false, ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	output, err := fn(tx)
	if err != nil {
		return nil, false, translate(err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, false, err
	}
	object := op.Object
	if object == "" {
		var id struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(encoded, &id)
		object = id.ID
		if object == "" {
			var nested struct {
				Attachment struct {
					ID string `json:"id"`
				} `json:"attachment"`
			}
			_ = json.Unmarshal(encoded, &nested)
			object = nested.Attachment.ID
		}
	}
	if err = audit(ctx, tx, op.Org, op.Kind, object, op.Action, op.Actor, op.RequestID, auditSummary(encoded, op.Action)); err != nil {
		return nil, false, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO accounting_operation_results(actor_id,operation,key_hash,fingerprint,response) VALUES($1,$2,$3,$4,$5)", op.Actor, operation, hash([]byte(key)), fingerprint, encoded)
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return encoded, false, nil
}
func decimalActor(actor int64) string { b, _ := json.Marshal(actor); return string(b) }
func audit(ctx context.Context, q dbQuery, org int64, kind, id, action string, actor int64, request string, summary map[string]any) error {
	if request == "" {
		request = uuid.NewString()
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, "INSERT INTO accounting_audit_logs(organization_id,object_kind,object_id,action,actor_id,request_id,summary) VALUES($1,$2,$3,$4,$5,$6,$7)", org, kind, id, action, actor, request, body)
	return err
}
func translate(err error) error {
	var pg *pq.Error
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return ErrState.WithMetadata(map[string]string{"constraint": pg.Constraint})
		case "22P02", "23503", "23514", "22003":
			return ErrInvalid
		}
	}
	return err
}

func auditSummary(body []byte, action string) map[string]any {
	record := map[string]any{}
	_ = json.Unmarshal(body, &record)
	summary := selected(record, []string{"status", "amount", "currency", "effective_amount", "customer_visible", "version"})
	summary["operation"] = action
	return summary
}
