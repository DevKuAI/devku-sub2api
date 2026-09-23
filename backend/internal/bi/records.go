package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"
)

type recordKey struct{ Kind, ID string }

type record struct {
	recordKey
	Revision     string
	DataRevision int64
	SourceID     string
	Payload      map[string]json.RawMessage
	Raw          []byte
	Hash         string
	Deleted      bool
	Index        int
}

func (r *record) str(key string) string {
	var value string
	_ = json.Unmarshal(r.Payload[key], &value)
	return value
}
func (r *record) strings(key string) []string {
	var value []string
	_ = json.Unmarshal(r.Payload[key], &value)
	return value
}
func (r *record) boolean(key string) bool {
	var value bool
	_ = json.Unmarshal(r.Payload[key], &value)
	return value
}
func (r *record) instant(key string) *time.Time {
	var value *time.Time
	_ = json.Unmarshal(r.Payload[key], &value)
	return value
}

func integerRevision(value string) (string, error) {
	// JSON integers may use decimal/exponent notation. Keep exact integer precision.
	if len(value) > 1000 || !boundedJSONNumbers(json.Number(value)) {
		return "", invalid("revision", "Revision exceeds supported numeric precision")
	}
	number, ok := new(big.Rat).SetString(value)
	if !ok || !number.IsInt() || number.Sign() <= 0 || len(number.Num().String()) > 1000 {
		return "", invalid("revision", "Expected a positive integer revision")
	}
	return number.Num().String(), nil
}

func compareRevision(a, b string) int {
	left, _ := new(big.Int).SetString(a, 10)
	right, _ := new(big.Int).SetString(b, 10)
	return left.Cmp(right)
}

func decodeRecord(input ImportRecord, index int) (*record, error) {
	revision, err := integerRevision(input.Revision.String())
	if err != nil {
		return nil, err
	}
	r := &record{recordKey: recordKey{Kind: input.Kind}, Revision: revision, Index: index, Deleted: input.Kind == "tombstone"}
	if r.Deleted {
		r.Kind, r.ID = input.EntityKind, input.EntityID
		r.Raw, _ = json.Marshal(map[string]any{"effective_at": input.EffectiveAt, "reason": input.Reason})
	} else {
		r.Raw, err = canonicalJSON(input.Payload)
		if err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
		return nil, err
	}
	if !r.Deleted {
		r.ID = r.str("id")
	}
	r.Hash = tokenHash(string(r.Raw))
	return r, nil
}

type batchView struct {
	ctx       context.Context
	q         queryer
	connector Connector
	current   map[recordKey]*record
	previous  map[recordKey]*record
	updates   []*record
	receipts  []*record
	now       time.Time
}

func (v *batchView) published(key recordKey) (*record, error) {
	if value, ok := v.previous[key]; ok {
		return value, nil
	}
	var r record
	r.recordKey = key
	err := v.q.QueryRowContext(v.ctx, `SELECT v.revision::text,v.data_revision,v.payload,v.payload_hash,v.tombstone,e.source_id
		FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision JOIN bi_entities e
		ON e.organization_id=v.organization_id AND e.kind=v.kind AND e.id=v.entity_id
		WHERE v.organization_id=$1 AND v.kind=$2 AND v.entity_id=$3 AND d.status='published' ORDER BY v.data_revision DESC,v.revision DESC LIMIT 1`,
		v.connector.OrganizationID, key.Kind, key.ID).Scan(&r.Revision, &r.DataRevision, &r.Raw, &r.Hash, &r.Deleted, &r.SourceID)
	if errors.Is(err, sql.ErrNoRows) {
		v.previous[key] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
		return nil, err
	}
	v.previous[key] = &r
	return &r, nil
}

func (v *batchView) get(kind, id string) (*record, error) {
	key := recordKey{kind, id}
	if value, ok := v.current[key]; ok {
		return value, nil
	}
	return v.published(key)
}

func (v *batchView) historicalContent(key recordKey, latest *record) (*record, error) {
	if latest == nil || !latest.Deleted {
		return latest, nil
	}
	r := &record{recordKey: key}
	err := v.q.QueryRowContext(v.ctx, `SELECT v.revision::text,v.data_revision,v.payload,v.payload_hash FROM bi_entity_versions v
		JOIN bi_data_revisions d ON d.id=v.data_revision WHERE v.organization_id=$1 AND v.kind=$2 AND v.entity_id=$3
		AND NOT v.tombstone AND d.status='published' ORDER BY v.data_revision DESC,v.revision DESC LIMIT 1`,
		v.connector.OrganizationID, key.Kind, key.ID).Scan(&r.Revision, &r.DataRevision, &r.Raw, &r.Hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
		return nil, err
	}
	return r, nil
}

func (v *batchView) reference(kind, id string) (*record, error) {
	if id == "" {
		return nil, nil
	}
	r, err := v.get(kind, id)
	if err != nil {
		return nil, err
	}
	if r == nil || r.Deleted {
		return nil, errRecord("INVALID_REFERENCE", "A referenced entity is unavailable")
	}
	return r, nil
}

type recordError struct{ code, message string }

func (e *recordError) Error() string       { return e.code + ": " + e.message }
func errRecord(code, message string) error { return &recordError{code, message} }

func prepareBatchView(ctx context.Context, q queryer, connector Connector, batch ImportBatch, now time.Time) (*batchView, []ImportError, error) {
	v := &batchView{ctx: ctx, q: q, connector: connector, current: map[recordKey]*record{}, previous: map[recordKey]*record{}, now: now}
	groups := map[recordKey][]*record{}
	for index, input := range batch.Records {
		r, err := decodeRecord(input, index)
		if err != nil {
			return v, []ImportError{{RecordIndex: &index, Code: "REVISION_CONFLICT", Message: "Invalid entity revision"}}, nil
		}
		if !strings.HasPrefix(r.ID, connector.Namespace+":") {
			return v, []ImportError{{RecordIndex: &index, Code: "FORBIDDEN_SOURCE", Message: "Entity is outside the source namespace"}}, nil
		}
		groups[r.recordKey] = append(groups[r.recordKey], r)
	}
	for key, group := range groups {
		sort.SliceStable(group, func(i, j int) bool { return compareRevision(group[i].Revision, group[j].Revision) < 0 })
		previous, err := v.published(key)
		if err != nil {
			return v, nil, err
		}
		if previous != nil && previous.SourceID != connector.SourceID {
			index := group[0].Index
			return v, []ImportError{{RecordIndex: &index, Code: "FORBIDDEN_SOURCE", Message: "Entity belongs to another source"}}, nil
		}
		seen := map[string]string{}
		lastContent, err := v.historicalContent(key, previous)
		if err != nil {
			return v, nil, err
		}
		for _, r := range group {
			if hash, ok := seen[r.Revision]; ok {
				if hash != r.Hash {
					return v, recordImportError(r, errRecord("REVISION_CONFLICT", "Same revision has different content")), nil
				}
				continue
			}
			seen[r.Revision] = r.Hash
			var hash string
			err = v.q.QueryRowContext(ctx, `SELECT r.payload_hash FROM bi_record_receipts r JOIN bi_import_batches b ON b.id=r.batch_id
				WHERE r.organization_id=$1 AND r.kind=$2 AND r.entity_id=$3 AND r.revision=$4 AND b.status='applied' LIMIT 1`,
				connector.OrganizationID, key.Kind, key.ID, r.Revision).Scan(&hash)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return v, nil, err
			}
			if err == nil && hash != r.Hash {
				return v, recordImportError(r, errRecord("REVISION_CONFLICT", "Same revision has different content")), nil
			}
			if previous != nil && compareRevision(r.Revision, previous.Revision) <= 0 {
				continue
			}
			if r.Deleted && previous == nil {
				return v, recordImportError(r, errRecord("INVALID_REFERENCE", "Cannot retract an unknown entity")), nil
			}
			if err := validateImmutable(lastContent, r); err != nil {
				return v, recordImportError(r, err), nil
			}
			v.receipts = append(v.receipts, r)
			previous = r
			if !r.Deleted {
				lastContent = r
			}
			v.current[key] = r
		}
	}
	// Every batch ID is available before validating any relationship.
	for _, r := range v.current {
		v.updates = append(v.updates, r)
	}
	sort.Slice(v.updates, func(i, j int) bool { return v.updates[i].Index < v.updates[j].Index })
	return v, nil, nil
}

func recordImportError(r *record, err error) []ImportError {
	var typed *recordError
	if !errors.As(err, &typed) {
		typed = &recordError{"INTERNAL_ERROR", "Unable to validate record"}
	}
	index := r.Index
	return []ImportError{{RecordIndex: &index, Code: typed.code, Message: typed.message}}
}

func validateImmutable(previous, current *record) error {
	if previous == nil || current.Deleted || previous.Deleted {
		return nil
	}
	switch current.Kind {
	case "application_version":
		if current.Hash != previous.Hash {
			return errRecord("IMMUTABLE_VERSION", "Application version content is immutable")
		}
	case "knowledge_version":
		for field, value := range previous.Payload {
			if field != "valid_to" && !equalJSON(value, current.Payload[field]) {
				return errRecord("IMMUTABLE_VERSION", "Knowledge version content is immutable")
			}
		}
		oldEnd, newEnd := previous.instant("valid_to"), current.instant("valid_to")
		if oldEnd != nil && (newEnd == nil || !oldEnd.Equal(*newEnd)) {
			return errRecord("IMMUTABLE_VERSION", "Closed version validity cannot be changed")
		}
	case "evaluation":
		for _, key := range []string{"application_id", "dataset_version", "criterion_version", "criterion", "baseline_application_version_id", "candidate_application_version_id"} {
			if previous.str(key) != current.str(key) {
				return errRecord("IMMUTABLE_VERSION", "Changed evaluation conditions require a new evaluation ID")
			}
		}
		oldSamples, err := evaluationDataset(previous.Payload["samples"])
		if err != nil {
			return err
		}
		newSamples, err := evaluationDataset(current.Payload["samples"])
		if err != nil {
			return err
		}
		if !equalJSON(oldSamples, newSamples) {
			return errRecord("IMMUTABLE_VERSION", "Changed evaluation samples require a new evaluation ID")
		}
	}
	return nil
}

func evaluationDataset(raw []byte) ([]byte, error) {
	var samples []struct {
		ID       string `json:"id"`
		Question string `json:"question"`
		Expected string `json:"expected"`
	}
	if err := json.Unmarshal(raw, &samples); err != nil {
		return nil, err
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].ID < samples[j].ID })
	return json.Marshal(samples)
}

func equalJSON(left, right []byte) bool {
	a, errA := canonicalJSON(left)
	b, errB := canonicalJSON(right)
	return errA == nil && errB == nil && string(a) == string(b)
}
