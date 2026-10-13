package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminSub2apiSyncInput struct {
	OrganizationID string    `json:"organization_id"`
	ConnectorToken string    `json:"connector_token"`
	Since          time.Time `json:"since"`
	Limit          int       `json:"limit"`
}

type AdminSub2apiSyncResult struct {
	Status  string     `json:"status"`
	Records int        `json:"records"`
	Job     *ImportJob `json:"job"`
}

func (h *Handler) AdminSyncSub2api(c *gin.Context, actorID int64) {
	var input AdminSub2apiSyncInput
	if !decodeRequest(c, &input) {
		return
	}
	result, err := h.service.syncAdminSub2api(c.Request.Context(), c.Param("source_id"), input, actorID, adminRequestID(c))
	adminRespond(c, result, err)
}

func (s *Service) syncAdminSub2api(ctx context.Context, sourceID string, input AdminSub2apiSyncInput, actorID int64, requestID string) (AdminSub2apiSyncResult, error) {
	result := AdminSub2apiSyncResult{}
	connector, err := s.AuthorizeConnector(ctx, input.ConnectorToken)
	if errors.Is(err, ErrUnauthenticated) {
		return result, apiError(400, "INVALID_CONNECTOR_CREDENTIAL", "Connector credential is invalid, expired or revoked")
	}
	if err != nil {
		return result, err
	}
	if input.OrganizationID != connector.OrganizationID || sourceID != connector.SourceID {
		return result, ErrForbidden
	}
	batch, err := s.PrepareSub2apiImport(ctx, connector, input.Since, input.Limit)
	if err != nil {
		return result, err
	}
	metadata := map[string]any{"since": input.Since.UTC(), "limit": input.Limit, "records": len(batch.Records), "credential_id": connector.ID}
	if len(batch.Records) == 0 {
		metadata["status"] = "idle"
		err := s.insertSecurityEvent(ctx, actorID, "source.sync_sub2api", sourceID, connector.OrganizationID, requestID, metadata)
		return AdminSub2apiSyncResult{Status: "idle"}, err
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		return result, err
	}
	job, err := s.submitImport(ctx, connector, batch.Checkpoint, raw, func(tx *sql.Tx) error {
		// Lock credential before source, matching rotate's order and serializing revocation.
		var active bool
		err := tx.QueryRowContext(ctx, `SELECT revoked_at IS NULL AND expires_at>$2 FROM bi_connector_credentials WHERE id=$1 FOR UPDATE`, connector.ID, s.now()).Scan(&active)
		if err != nil {
			return err
		}
		if !active {
			return apiError(400, "INVALID_CONNECTOR_CREDENTIAL", "Connector credential is invalid, expired or revoked")
		}
		return nil
	}, func(tx *sql.Tx, job ImportJob) error {
		metadata["batch_id"], metadata["status"] = job.ID, job.Status
		return insertAdminSecurityEvent(ctx, tx, actorID, "source.sync_sub2api", sourceID, connector.OrganizationID, requestID, metadata)
	})
	if err != nil {
		return result, err
	}
	return AdminSub2apiSyncResult{Status: job.Status, Records: len(batch.Records), Job: &job}, nil
}
