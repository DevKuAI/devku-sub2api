package bi

import (
	"encoding/json"
	"time"
)

var RecordKinds = []string{"team", "role", "member", "membership", "eligibility", "application", "application_version", "scene", "usage", "rating", "reference", "knowledge", "knowledge_version", "source", "case", "evaluation", "tombstone"}

type Connector struct {
	ID             string
	OrganizationID string
	SourceID       string
	Namespace      string
	AllowedKinds   []string
}

type ImportRecord struct {
	Kind        string          `json:"kind"`
	Revision    json.Number     `json:"revision"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	EntityKind  string          `json:"entity_kind,omitempty"`
	EntityID    string          `json:"entity_id,omitempty"`
	EffectiveAt *time.Time      `json:"effective_at,omitempty"`
	Reason      string          `json:"reason,omitempty"`
}

type ImportBatch struct {
	SourceID                string         `json:"source_id"`
	SchemaVersion           string         `json:"schema_version"`
	ExpectedCheckpoint      *string        `json:"expected_checkpoint"`
	Checkpoint              string         `json:"checkpoint"`
	Records                 []ImportRecord `json:"records"`
	CompleteThrough         *time.Time     `json:"complete_through"`
	HistoryStartDate        string         `json:"history_start_date"`
	InitialBackfillComplete bool           `json:"initial_backfill_complete"`
}

type ImportError struct {
	RecordIndex *int   `json:"record_index"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type ImportJob struct {
	ID           string        `json:"id"`
	SourceID     string        `json:"source_id"`
	Status       string        `json:"status"`
	ReceivedAt   time.Time     `json:"received_at"`
	AppliedAt    *time.Time    `json:"applied_at"`
	Checkpoint   *string       `json:"checkpoint"`
	DataRevision *string       `json:"data_revision"`
	Errors       []ImportError `json:"errors"`
}

type Checkpoint struct {
	SourceID                string     `json:"source_id"`
	Checkpoint              *string    `json:"checkpoint"`
	LastAppliedAt           *time.Time `json:"last_applied_at"`
	DataRevision            *string    `json:"data_revision"`
	CompleteThrough         *time.Time `json:"complete_through"`
	HistoryStartDate        *string    `json:"history_start_date"`
	InitialBackfillComplete bool       `json:"initial_backfill_complete"`
}
