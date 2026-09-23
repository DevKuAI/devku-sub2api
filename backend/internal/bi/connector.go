package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

type ConnectorRegistration struct {
	OrganizationID string
	SourceID       string
	Namespace      string
	AllowedKinds   []string
	ExpiresAt      time.Time
}

// RegisterConnector is an operations-only API. The bearer is returned once.
// Reissuing credentials for the same source preserves its checkpoint and idempotency domain.
func RegisterConnector(ctx context.Context, db *sql.DB, input ConnectorRegistration) (string, string, error) {
	if !requestIDPattern.MatchString(input.OrganizationID) || !requestIDPattern.MatchString(input.SourceID) ||
		!requestIDPattern.MatchString(input.Namespace) || len(input.AllowedKinds) == 0 || !input.ExpiresAt.After(time.Now()) {
		return "", "", invalid("connector", "Organization, source, namespace, kinds and future expiry are required")
	}
	for _, kind := range input.AllowedKinds {
		if !slices.Contains(RecordKinds, kind) {
			return "", "", invalid("allowed_kinds", "Unknown record kind")
		}
	}
	slices.Sort(input.AllowedKinds)
	input.AllowedKinds = slices.Compact(input.AllowedKinds)
	kinds, _ := json.Marshal(input.AllowedKinds)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT d.status='active' AND d.deleted_at IS NULL FROM bi_organizations o JOIN desktop_organizations d ON d.id=o.desktop_organization_id WHERE o.id=$1 FOR UPDATE OF o`, input.OrganizationID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_connector_sources(organization_id,source_id,namespace,allowed_kinds) VALUES($1,$2,$3,$4) ON CONFLICT(organization_id,source_id) DO NOTHING`, input.OrganizationID, input.SourceID, input.Namespace, string(kinds))
	if err != nil {
		return "", "", err
	}
	var namespace string
	var existingKinds []byte
	err = tx.QueryRowContext(ctx, `SELECT namespace,allowed_kinds FROM bi_connector_sources WHERE organization_id=$1 AND source_id=$2 FOR UPDATE`, input.OrganizationID, input.SourceID).Scan(&namespace, &existingKinds)
	if err != nil {
		return "", "", err
	}
	var allowed []string
	if err := json.Unmarshal(existingKinds, &allowed); err != nil {
		return "", "", err
	}
	if namespace != input.Namespace || !containsAll(allowed, input.AllowedKinds) {
		return "", "", apiError(409, "CONFLICT", "Credential rotation cannot change source ownership or expand its record kinds")
	}
	id, token := randomToken("bic_id_"), randomToken("bic_")
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_connector_credentials(id,token_hash,audience,organization_id,source_id,allowed_kinds,expires_at) VALUES($1,$2,'bi-connector',$3,$4,$5,$6)`,
		id, tokenHash(token), input.OrganizationID, input.SourceID, string(kinds), input.ExpiresAt.UTC())
	if err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_security_events(action,target_id,request_id) VALUES('connector.issue',$1,$2)`, id, randomToken("ops_")); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, token, nil
}

func RevokeConnector(ctx context.Context, db *sql.DB, id string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE bi_connector_credentials SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_security_events(action,target_id,request_id) VALUES('connector.revoke',$1,$2)`, id, randomToken("ops_")); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) AuthorizeConnector(ctx context.Context, raw string) (Connector, error) {
	var connector Connector
	if len(raw) < 32 || len(raw) > 2048 {
		return connector, ErrUnauthenticated
	}
	var rawKinds, sourceKinds []byte
	err := s.db.QueryRowContext(ctx, `SELECT c.id,c.organization_id,c.source_id,s.namespace,c.allowed_kinds,s.allowed_kinds
		FROM bi_connector_credentials c JOIN bi_connector_sources s ON s.organization_id=c.organization_id AND s.source_id=c.source_id
		JOIN bi_organizations o ON o.id=c.organization_id JOIN desktop_organizations d ON d.id=o.desktop_organization_id
		WHERE c.token_hash=$1 AND c.audience='bi-connector' AND c.revoked_at IS NULL AND c.expires_at>$2
		AND d.status='active' AND d.deleted_at IS NULL`, tokenHash(raw), s.now().UTC()).Scan(&connector.ID, &connector.OrganizationID, &connector.SourceID, &connector.Namespace, &rawKinds, &sourceKinds)
	if errors.Is(err, sql.ErrNoRows) {
		return connector, ErrUnauthenticated
	}
	if err != nil {
		return connector, err
	}
	var allowed []string
	if err := json.Unmarshal(rawKinds, &connector.AllowedKinds); err != nil {
		return connector, err
	}
	if err := json.Unmarshal(sourceKinds, &allowed); err != nil {
		return connector, err
	}
	connector.AllowedKinds = slices.DeleteFunc(connector.AllowedKinds, func(kind string) bool { return !slices.Contains(allowed, kind) })
	return connector, nil
}
