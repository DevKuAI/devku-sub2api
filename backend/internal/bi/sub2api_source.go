package bi

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"
)

type gatewayUsage struct {
	ID                                   int64
	CreatedAt                            time.Time
	RequestedModel                       *string
	Input, Output, CacheRead, CacheWrite int64
	Duration                             *int64
	BillingMode                          *string
	Images, Videos                       int64
}

func gatewayTokens(row gatewayUsage) RawTokens {
	unavailable := RawTokens{Encoding: "unavailable"}
	mode := stringValue(row.BillingMode)
	if mode != "" && mode != "token" {
		return unavailable
	}
	if mode == "" && (row.Images > 0 || row.Videos > 0) {
		return unavailable
	}
	if row.Input == 0 && row.Output == 0 && row.CacheRead == 0 && row.CacheWrite == 0 {
		return unavailable
	}
	return RawTokens{Encoding: "exclusive_buckets", Input: ptr(strconv.FormatInt(row.Input, 10)), Output: ptr(strconv.FormatInt(row.Output, 10)), CacheRead: ptr(strconv.FormatInt(row.CacheRead, 10)), CacheWrite: ptr(strconv.FormatInt(row.CacheWrite, 10))}
}

// PrepareSub2apiImport copies only attributable enterprise billing-log facts.
// Missing interaction identity, outcome, application and scene remain unknown.
// It does not claim complete request coverage from successful/billed usage logs.
func (s *Service) PrepareSub2apiImport(ctx context.Context, connector Connector, since time.Time, limit int) (ImportBatch, error) {
	if !slices.Contains(connector.AllowedKinds, "usage") {
		return ImportBatch{}, ErrForbidden
	}
	if limit < 1 || limit > 500 || since.IsZero() || since.After(s.now()) {
		return ImportBatch{}, invalid("range", "A past start time and limit of 1–500 are required")
	}
	if len(connector.Namespace) > 82 {
		return ImportBatch{}, invalid("namespace", "Namespace exceeds the adapter's 82-character limit")
	}
	checkpoint, err := s.ImportCheckpoint(ctx, connector, connector.SourceID)
	if err != nil {
		return ImportBatch{}, err
	}
	if checkpoint.InitialBackfillComplete || checkpoint.CompleteThrough != nil {
		return ImportBatch{}, apiError(409, "CONFLICT", "This source already declares completeness; use its authoritative producer")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ul.id,ul.created_at,ul.requested_model,ul.input_tokens,ul.output_tokens,ul.cache_read_tokens,ul.cache_creation_tokens,
		ul.duration_ms,ul.billing_mode,ul.image_count,ul.video_count
		FROM bi_organizations org JOIN desktop_members member ON member.organization_id=org.desktop_organization_id
		JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id JOIN usage_logs ul ON ul.api_key_id=assignment.api_key_id
		LEFT JOIN bi_external_mappings mapping ON mapping.organization_id=org.id AND mapping.source_id=$2 AND mapping.entity_kind='usage' AND mapping.external_id=ul.id::text
		WHERE org.id=$1 AND ul.created_at>=$3 AND NOT EXISTS(SELECT 1 FROM bi_record_receipts receipt JOIN bi_import_batches batch ON batch.id=receipt.batch_id
		WHERE receipt.organization_id=org.id AND receipt.kind='usage' AND receipt.entity_id=mapping.entity_id AND batch.status='applied')
		ORDER BY ul.id LIMIT $4`, connector.OrganizationID, connector.SourceID, since.UTC(), limit)
	if err != nil {
		return ImportBatch{}, err
	}
	logs := []gatewayUsage{}
	for rows.Next() {
		var row gatewayUsage
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.RequestedModel, &row.Input, &row.Output, &row.CacheRead, &row.CacheWrite, &row.Duration, &row.BillingMode, &row.Images, &row.Videos); err != nil {
			_ = rows.Close()
			return ImportBatch{}, err
		}
		logs = append(logs, row)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return ImportBatch{}, err
	}
	history := since.In(shanghai).Format("2006-01-02")
	if checkpoint.HistoryStartDate != nil && *checkpoint.HistoryStartDate < history {
		history = *checkpoint.HistoryStartDate
	}
	batch := ImportBatch{SourceID: connector.SourceID, SchemaVersion: "1", ExpectedCheckpoint: checkpoint.Checkpoint, Checkpoint: randomToken("gateway_"), Records: []ImportRecord{}, HistoryStartDate: history}
	for _, row := range logs {
		var id string
		err := s.db.QueryRowContext(ctx, `INSERT INTO bi_external_mappings(organization_id,source_id,entity_kind,external_id,entity_id) VALUES($1,$2,'usage',$3,$4)
			ON CONFLICT(organization_id,source_id,entity_kind,external_id) DO UPDATE SET external_id=EXCLUDED.external_id RETURNING entity_id`, connector.OrganizationID, connector.SourceID, strconv.FormatInt(row.ID, 10), connector.Namespace+":u:"+randomToken("")).Scan(&id)
		if err != nil {
			return ImportBatch{}, err
		}
		model := stringValue(row.RequestedModel)
		if model == "" {
			model = "unknown"
		}
		usage := UsageRecord{ID: id, OccurredAt: row.CreatedAt.UTC(), ActorType: "unknown", RequestedModel: model, Outcome: "unknown", DurationMS: row.Duration, Tokens: gatewayTokens(row)}
		payload, err := json.Marshal(usage)
		if err != nil {
			return ImportBatch{}, err
		}
		batch.Records = append(batch.Records, ImportRecord{Kind: "usage", Revision: json.Number("1"), Payload: payload})
	}
	return batch, nil
}
