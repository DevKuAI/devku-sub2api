package bi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

type ACL struct {
	OrganizationReadable bool     `json:"organization_readable"`
	TeamIDs              []string `json:"team_ids"`
	ManagerIDs           []string `json:"manager_ids"`
}

type fieldReference struct {
	field, kind string
	many        bool
}

var recordReferences = map[string][]fieldReference{
	"membership":          {{"member_id", "member", false}, {"team_id", "team", false}, {"role_id", "role", false}},
	"eligibility":         {{"member_id", "member", false}, {"application_id", "application", false}, {"scene_id", "scene", false}},
	"application":         {{"current_version_id", "application_version", false}, {"team_ids", "team", true}, {"scene_ids", "scene", true}},
	"application_version": {{"application_id", "application", false}, {"knowledge_version_ids", "knowledge_version", true}},
	"usage":               {{"member_id", "member", false}, {"team_id", "team", false}, {"application_id", "application", false}, {"application_version_id", "application_version", false}, {"scene_id", "scene", false}, {"retry_of", "usage", false}},
	"rating":              {{"usage_event_id", "usage", false}, {"member_id", "member", false}},
	"reference":           {{"usage_event_id", "usage", false}, {"knowledge_version_id", "knowledge_version", false}},
	"knowledge":           {{"current_version_id", "knowledge_version", false}, {"owner_id", "member", false}, {"application_ids", "application", true}, {"scene_ids", "scene", true}},
	"knowledge_version":   {{"knowledge_id", "knowledge", false}, {"source_ids", "source", true}},
	"case":                {{"team_id", "team", false}, {"application_id", "application", false}, {"scene_id", "scene", false}, {"knowledge_version_ids", "knowledge_version", true}, {"source_ids", "source", true}},
	"evaluation":          {{"application_id", "application", false}, {"baseline_application_version_id", "application_version", false}, {"candidate_application_version_id", "application_version", false}},
}

func (v *batchView) validateRecords() ([]ImportError, error) {
	for _, r := range v.receipts {
		if err := v.validateRecord(r); err != nil {
			var typed *recordError
			if errors.As(err, &typed) {
				return recordImportError(r, err), nil
			}
			return nil, err
		}
	}
	for _, r := range v.updates {
		if r.Kind != "usage" || r.Deleted {
			continue
		}
		if err := v.validateUsageDependents(r); err != nil {
			var typed *recordError
			if errors.As(err, &typed) {
				return recordImportError(r, err), nil
			}
			return nil, err
		}
	}
	return nil, nil
}

func (v *batchView) validateRecord(r *record) error {
	if r.Deleted {
		if at := r.instant("effective_at"); at == nil || at.After(v.now.Add(5*time.Minute)) {
			return errRecord("INVALID_INTERVAL", "Invalid retraction time")
		}
		return nil
	}
	for _, ref := range recordReferences[r.Kind] {
		ids := []string{r.str(ref.field)}
		if ref.many {
			ids = r.strings(ref.field)
		}
		for _, id := range ids {
			if _, err := v.reference(ref.kind, id); err != nil {
				return err
			}
		}
	}
	if err := v.validateACL(r); err != nil {
		return err
	}
	if start := r.instant("valid_from"); start != nil {
		if end := r.instant("valid_to"); end != nil && !end.After(*start) {
			return errRecord("INVALID_INTERVAL", "Validity end must be after its start")
		}
	}
	switch r.Kind {
	case "member":
		start, end := r.instant("enabled_at"), r.instant("disabled_at")
		if start == nil || (end != nil && !end.After(*start)) {
			return errRecord("INVALID_INTERVAL", "Invalid member lifetime")
		}
	case "membership":
		return v.validateMembership(r)
	case "application":
		version, err := v.reference("application_version", r.str("current_version_id"))
		if err != nil {
			return err
		}
		if version.str("application_id") != r.ID {
			return errRecord("INVALID_REFERENCE", "Current version belongs to another application")
		}
	case "knowledge":
		version, err := v.reference("knowledge_version", r.str("current_version_id"))
		if err != nil {
			return err
		}
		if version.str("knowledge_id") != r.ID {
			return errRecord("INVALID_REFERENCE", "Current version belongs to another knowledge item")
		}
	case "usage":
		return v.validateUsage(r)
	case "rating":
		return v.validateRating(r)
	case "reference":
		return v.validateReference(r)
	case "source":
		return v.validateSourceVersion(r)
	case "evaluation":
		return v.validateEvaluation(r)
	case "case":
		if at := r.instant("occurred_at"); at == nil || at.After(v.now.Add(5*time.Minute)) {
			return errRecord("INVALID_INTERVAL", "Case occurrence is in the future")
		}
	}
	return nil
}

func (v *batchView) validateACL(r *record) error {
	if _, ok := r.Payload["acl"]; !ok {
		return nil
	}
	var acl ACL
	if err := json.Unmarshal(r.Payload["acl"], &acl); err != nil {
		return err
	}
	for _, teamID := range acl.TeamIDs {
		if _, err := v.reference("team", teamID); err != nil {
			return err
		}
	}
	for _, managerID := range acl.ManagerIDs {
		var exists bool
		err := v.q.QueryRowContext(v.ctx, `SELECT EXISTS(SELECT 1 FROM bi_manager_grants WHERE organization_id=$1 AND manager_id=$2)`, v.connector.OrganizationID, managerID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return errRecord("INVALID_REFERENCE", "ACL refers to an unknown enterprise manager")
		}
	}
	return nil
}

func (v *batchView) related(kind, field, id string) ([]*record, error) {
	// field and kind are internal constants, not caller-provided SQL identifiers.
	rows, err := v.q.QueryContext(v.ctx, `SELECT DISTINCT ON (v.entity_id) v.entity_id,v.revision::text,v.data_revision,v.payload,v.payload_hash,v.tombstone,e.source_id
		FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision JOIN bi_entities e
		ON e.organization_id=v.organization_id AND e.kind=v.kind AND e.id=v.entity_id
		WHERE v.organization_id=$1 AND v.kind=$2 AND d.status='published'
		AND v.entity_id IN (SELECT entity_id FROM bi_entity_versions WHERE organization_id=$1 AND kind=$2 AND payload->>$3=$4)
		ORDER BY v.entity_id,v.data_revision DESC,v.revision DESC`, v.connector.OrganizationID, kind, field, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	found := map[recordKey]*record{}
	for rows.Next() {
		r := &record{recordKey: recordKey{Kind: kind}}
		if err := rows.Scan(&r.ID, &r.Revision, &r.DataRevision, &r.Raw, &r.Hash, &r.Deleted, &r.SourceID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(r.Raw, &r.Payload); err != nil {
			return nil, err
		}
		found[r.recordKey] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for key, r := range v.current {
		if key.Kind == kind {
			found[key] = r
		}
	}
	result := []*record{}
	for _, r := range found {
		if !r.Deleted && r.str(field) == id {
			result = append(result, r)
		}
	}
	return result, nil
}

func overlap(a, b *record) bool {
	as, ae, bs, be := a.instant("valid_from"), a.instant("valid_to"), b.instant("valid_from"), b.instant("valid_to")
	return as != nil && bs != nil && (ae == nil || bs.Before(*ae)) && (be == nil || as.Before(*be))
}

func (v *batchView) validateMembership(r *record) error {
	if !r.boolean("primary") {
		return nil
	}
	others, err := v.related("membership", "member_id", r.str("member_id"))
	if err != nil {
		return err
	}
	for _, other := range others {
		if other.ID != r.ID && other.boolean("primary") && overlap(r, other) {
			return errRecord("INVALID_INTERVAL", "Primary membership intervals overlap")
		}
	}
	return nil
}

func (v *batchView) validateUsage(r *record) error {
	at := r.instant("occurred_at")
	if at == nil || at.After(v.now.Add(5*time.Minute)) {
		return errRecord("INVALID_INTERVAL", "Usage occurrence is in the future")
	}
	var tokens RawTokens
	if err := json.Unmarshal(r.Payload["tokens"], &tokens); err != nil {
		return err
	}
	if _, err := NormalizeTokens(tokens); err != nil {
		return errRecord("INVALID_TOKENS", "Invalid token buckets")
	}
	if id := r.str("application_version_id"); id != "" {
		version, err := v.reference("application_version", id)
		if err != nil {
			return err
		}
		if version.str("application_id") != r.str("application_id") {
			return errRecord("INVALID_REFERENCE", "Application version does not belong to the attributed application")
		}
		if released := version.instant("released_at"); released == nil || released.After(*at) {
			return errRecord("INVALID_INTERVAL", "Application version was not available at usage time")
		}
	}
	if id := r.str("retry_of"); id != "" {
		previous, err := v.reference("usage", id)
		if err != nil {
			return err
		}
		if err := validateRetryOrder(r, previous); err != nil {
			return err
		}
	}
	return nil
}

func validateRetryOrder(retry, previous *record) error {
	at, before := retry.instant("occurred_at"), previous.instant("occurred_at")
	if retry.ID == previous.ID || at == nil || before == nil || !before.Before(*at) {
		return errRecord("INVALID_REFERENCE", "Retry must refer to an earlier attempt")
	}
	return nil
}

// Corrections must preserve the constraints of existing records owned by any source.
// Retractions keep their historical dependents, while same-batch replacements use
// the final pending view and never rewrite another source's records implicitly.
func (v *batchView) validateUsageDependents(usage *record) error {
	previous, err := v.published(usage.recordKey)
	if err != nil || previous == nil {
		return err
	}
	identityChanged := usage.str("actor_type") != previous.str("actor_type") || usage.str("member_id") != previous.str("member_id")
	before, after := previous.instant("occurred_at"), usage.instant("occurred_at")
	timeChanged := before == nil || after == nil || !before.Equal(*after)
	if !identityChanged && !timeChanged {
		return nil
	}
	ratings, err := v.related("rating", "usage_event_id", usage.ID)
	if err != nil {
		return err
	}
	for _, rating := range ratings {
		if err := validateRatingInteraction(rating, usage, v.now); err != nil {
			return err
		}
	}
	if !timeChanged {
		return nil
	}
	references, err := v.related("reference", "usage_event_id", usage.ID)
	if err != nil {
		return err
	}
	for _, reference := range references {
		version, err := v.get("knowledge_version", reference.str("knowledge_version_id"))
		if err != nil {
			return err
		}
		// A retracted version remains historical evidence; its original validity
		// still constrains a corrected event time.
		version, err = v.historicalContent(recordKey{"knowledge_version", reference.str("knowledge_version_id")}, version)
		if err != nil {
			return err
		}
		// A lifecycle close followed by a tombstone in this batch must still
		// constrain corrected events; the published version may still be open.
		for _, pending := range v.receipts {
			if pending.Kind == "knowledge_version" && pending.ID == reference.str("knowledge_version_id") && !pending.Deleted &&
				(version == nil || compareRevision(pending.Revision, version.Revision) > 0) {
				version = pending
			}
		}
		if err := validateReferenceTime(usage, version); err != nil {
			return err
		}
	}
	retries, err := v.related("usage", "retry_of", usage.ID)
	if err != nil {
		return err
	}
	for _, retry := range retries {
		if err := validateRetryOrder(retry, usage); err != nil {
			return err
		}
	}
	return nil
}

func (v *batchView) validateRating(r *record) error {
	usage, err := v.reference("usage", r.str("usage_event_id"))
	if err != nil {
		return err
	}
	if err := validateRatingInteraction(r, usage, v.now); err != nil {
		return err
	}
	others, err := v.related("rating", "usage_event_id", usage.ID)
	if err != nil {
		return err
	}
	for _, other := range others {
		if other.ID != r.ID {
			return errRecord("REVISION_CONFLICT", "An interaction may have only one current rating")
		}
	}
	return nil
}

func validateRatingInteraction(rating, usage *record, now time.Time) error {
	if usage.str("actor_type") != "human" || usage.str("member_id") != rating.str("member_id") {
		return errRecord("INVALID_IDENTITY", "Rating does not belong to the human interaction")
	}
	if rated, occurred := rating.instant("rated_at"), usage.instant("occurred_at"); rated == nil || occurred == nil || rated.Before(*occurred) || rated.After(now.Add(5*time.Minute)) {
		return errRecord("INVALID_INTERVAL", "Invalid rating time")
	}
	return nil
}

func (v *batchView) validateReference(r *record) error {
	usage, err := v.reference("usage", r.str("usage_event_id"))
	if err != nil {
		return err
	}
	version, err := v.reference("knowledge_version", r.str("knowledge_version_id"))
	if err != nil {
		return err
	}
	return validateReferenceTime(usage, version)
}

func validateReferenceTime(usage, version *record) error {
	if version == nil {
		return errRecord("INVALID_REFERENCE", "A referenced entity is unavailable")
	}
	at, start, end := usage.instant("occurred_at"), version.instant("valid_from"), version.instant("valid_to")
	if at == nil || start == nil || at.Before(*start) || (end != nil && !at.Before(*end)) {
		return errRecord("INVALID_REFERENCE", "Knowledge version was not valid when referenced")
	}
	return nil
}

func (v *batchView) validateSourceVersion(r *record) error {
	for _, other := range v.receipts {
		if other.Kind == "source" && !other.Deleted && other.ID == r.ID && other.str("version_id") == r.str("version_id") {
			for _, field := range []string{"title", "type", "body"} {
				if !equalJSON(other.Payload[field], r.Payload[field]) {
					return errRecord("IMMUTABLE_VERSION", "Source version content is immutable")
				}
			}
		}
	}
	var previous []byte
	err := v.q.QueryRowContext(v.ctx, `SELECT v.payload FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision
		WHERE v.organization_id=$1 AND v.kind='source' AND v.entity_id=$2 AND v.payload->>'version_id'=$3
		AND NOT v.tombstone AND d.status='published' ORDER BY v.data_revision LIMIT 1`, v.connector.OrganizationID, r.ID, r.str("version_id")).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(previous, &body); err != nil {
		return err
	}
	for _, field := range []string{"title", "type", "body"} {
		if !equalJSON(body[field], r.Payload[field]) {
			return errRecord("IMMUTABLE_VERSION", "Source version content is immutable")
		}
	}
	return nil
}

func (v *batchView) validateEvaluation(r *record) error {
	for _, field := range []string{"baseline_application_version_id", "candidate_application_version_id"} {
		version, err := v.reference("application_version", r.str(field))
		if err != nil {
			return err
		}
		if version.str("application_id") != r.str("application_id") {
			return errRecord("INVALID_REFERENCE", "Evaluation versions must belong to the same application")
		}
	}
	var samples []struct {
		ID   string `json:"id"`
		Case struct {
			ID           *string `json:"id"`
			Availability string  `json:"availability"`
		} `json:"case_link"`
	}
	if err := json.Unmarshal(r.Payload["samples"], &samples); err != nil {
		return err
	}
	ids := []string{}
	for _, sample := range samples {
		if slices.Contains(ids, sample.ID) {
			return errRecord("REVISION_CONFLICT", "Evaluation sample IDs must be unique")
		}
		ids = append(ids, sample.ID)
		if sample.Case.Availability == "restricted" {
			return errRecord("INVALID_REFERENCE", "Restricted case links are output-only")
		}
		if sample.Case.Availability == "available" && sample.Case.ID != nil {
			if _, err := v.reference("case", *sample.Case.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
