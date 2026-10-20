package accounting

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Operation struct {
	Actor     int64
	Org       int64
	Global    bool
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
	if !op.Global {
		var org int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM desktop_organizations WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", op.Org).Scan(&org)
		if err == sql.ErrNoRows {
			return nil, false, ErrNotFound
		}
		if err != nil {
			return nil, false, err
		}
	}
	date := time.Time{}
	beforePositions := map[string]Position{}
	before := map[string]any{}
	if !op.Global {
		date, err = operationDate(ctx, tx, op)
		if err != nil {
			return nil, false, err
		}
		if err = protectPeriod(ctx, tx, op, date); err != nil {
			return nil, false, err
		}
		beforePositions, err = currentPositions(ctx, tx, op.Org)
		if err != nil {
			return nil, false, err
		}
		if err = ensurePositionBaseline(ctx, tx, op.Org, beforePositions); err != nil {
			return nil, false, err
		}
		if _, ok := specs[op.Kind]; ok && op.Object != "" {
			if r, readErr := get(ctx, tx, op.Kind, op.Object, op.Org, false); readErr == nil {
				before = map[string]any{"amount": r.Amount, "currency": r.Currency, "status": r.Status, "version": r.Version}
			}
		}
		if fields, ok := specs[op.Kind]; ok && op.Object != "" {
			if r, readErr := get(ctx, tx, op.Kind, op.Object, op.Org, false); readErr == nil {
				before["fields"] = auditFields(r, fields.fields)
				if op.Kind == "transfers" || op.Kind == "expenses" || op.Kind == "receivables" {
					v, valueErr := effective(ctx, tx, op.Kind, r)
					if valueErr != nil {
						return nil, false, valueErr
					}
					before["effective_amount"] = v.String()
				}
			}
		}
	}
	output, err := fn(tx)
	if err != nil {
		return nil, false, translate(err)
	}
	if !op.Global {
		afterPositions, positionErr := currentPositions(ctx, tx, op.Org)
		if positionErr != nil {
			return nil, false, positionErr
		}
		if err = recordPositionChanges(ctx, tx, op, date, beforePositions, afterPositions); err != nil {
			return nil, false, err
		}
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
	if !op.Global {
		summary := operationAudit(encoded, op, before)
		if fields, ok := specs[op.Kind]; ok && op.Object != "" {
			if r, readErr := get(ctx, tx, op.Kind, op.Object, op.Org, false); readErr == nil {
				after := map[string]any{"status": r.Status, "amount": r.Amount, "currency": r.Currency, "fields": auditFields(r, fields.fields)}
				if op.Kind == "transfers" || op.Kind == "expenses" || op.Kind == "receivables" {
					v, valueErr := effective(ctx, tx, op.Kind, r)
					if valueErr != nil {
						return nil, false, valueErr
					}
					after["effective_amount"] = v.String()
				}
				summary["after"] = after
			}
		}
		if err = audit(ctx, tx, op.Org, op.Kind, object, op.Action, op.Actor, op.RequestID, summary); err != nil {
			return nil, false, err
		}
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

// duplicateFields names the field behind a uniqueness conflict so the form can point at it.
var duplicateFields = map[string]string{
	"idx_accounting_bank_reference":       "duplicate_bank_reference",
	"idx_accounting_card_reference":       "duplicate_transaction",
	"idx_accounting_invoice_number":       "duplicate_invoice",
	"idx_accounting_vcc_card_fingerprint": "card_registered",
}

func translate(err error) error {
	var pg *pq.Error
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			metadata := map[string]string{"constraint": pg.Constraint}
			if field := duplicateFields[pg.Constraint]; field != "" {
				metadata["field"] = field
			}
			return ErrState.WithMetadata(metadata)
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

func operationAudit(body []byte, op Operation, before map[string]any) map[string]any {
	summary := auditSummary(body, op.Action)
	summary["before"] = before
	payload := map[string]any{}
	raw, _ := json.Marshal(op.Payload)
	_ = json.Unmarshal(raw, &payload)
	for _, key := range []string{"reason", "reference", "amount", "card_id", "transfer_id", "invoice_id", "source_kind", "source_id"} {
		if value, ok := payload[key]; ok {
			summary[key] = value
		}
	}
	return summary
}

func auditFields(r *Record, fields []string) map[string]any {
	values := selected(r.Details, fields)
	for _, key := range []string{"card_number", "cvv", "expiry", "provider_token"} {
		delete(values, key)
	}
	return values
}
