package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

var Capabilities = []string{
	"analytics:read", "knowledge:read", "sources:read", "evaluations:read", "members:read",
	"reports:read", "reports:create", "reports:copy", "reports:share",
}

type GrantInput struct {
	UserID           int64    `json:"user_id"`
	Role             string   `json:"role"`
	AllTeams         bool     `json:"all_teams"`
	TeamIDs          []string `json:"team_ids"`
	Capabilities     []string `json:"capabilities"`
	ExpectedRevision *int64   `json:"expected_revision"`
}

type Grant struct {
	ManagerID    string   `json:"manager_id"`
	UserID       int64    `json:"user_id"`
	DisplayName  string   `json:"display_name"`
	Role         string   `json:"role"`
	AllTeams     bool     `json:"all_teams"`
	TeamIDs      []string `json:"team_ids"`
	Capabilities []string `json:"capabilities"`
	Revision     int64    `json:"revision"`
	Revoked      bool     `json:"revoked"`
}

func validateGrant(input GrantInput) error {
	if input.UserID <= 0 || input.ExpectedRevision == nil || *input.ExpectedRevision < 0 {
		return invalid("grant", "A user and an expected revision are required")
	}
	if !slices.Contains([]string{"org_admin", "team_manager", "viewer"}, input.Role) {
		return invalid("role", "Unknown BI role")
	}
	if input.TeamIDs == nil || len(input.TeamIDs) > 100 || input.Capabilities == nil || len(input.Capabilities) == 0 {
		return invalid("grant", "Explicit team and capability arrays are required")
	}
	if input.AllTeams && len(input.TeamIDs) != 0 {
		return invalid("team_ids", "All-teams grants must not include individual team IDs")
	}
	for _, id := range input.TeamIDs {
		if !requestIDPattern.MatchString(id) {
			return invalid("team_ids", "Invalid team ID")
		}
	}
	for _, capability := range input.Capabilities {
		if !slices.Contains(Capabilities, capability) {
			return invalid("capabilities", "Unknown BI capability")
		}
	}
	return nil
}

func (s *Service) SaveGrant(ctx context.Context, organizationID string, input GrantInput, actorUserID int64, requestID string) (Grant, error) {
	if err := validateGrant(input); err != nil {
		return Grant{}, err
	}
	u, err := s.activeUser(ctx, input.UserID)
	if err != nil {
		return Grant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Grant{}, err
	}
	defer func() { _ = tx.Rollback() }()
	managers, err := lockGrantManagers(ctx, tx, input.UserID, actorUserID)
	if err != nil {
		return Grant{}, err
	}
	managerID, actor := managers[input.UserID], managers[actorUserID]
	var desktopID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM desktop_organizations WHERE public_id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, organizationID).Scan(&desktopID)
	if errors.Is(err, sql.ErrNoRows) {
		return Grant{}, ErrNotFound
	}
	if err != nil {
		return Grant{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_organizations(id,desktop_organization_id) VALUES($1,$2) ON CONFLICT(id) DO NOTHING`, organizationID, desktopID); err != nil {
		return Grant{}, err
	}
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM bi_manager_grants WHERE manager_id=$1 AND organization_id=$2 FOR UPDATE`, managerID, organizationID).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Grant{}, err
	}
	if current != *input.ExpectedRevision {
		return Grant{}, apiError(409, "CONFLICT", "Grant changed; reload before saving")
	}
	slices.Sort(input.TeamIDs)
	slices.Sort(input.Capabilities)
	input.TeamIDs = slices.Compact(input.TeamIDs)
	input.Capabilities = slices.Compact(input.Capabilities)
	teams, _ := json.Marshal(input.TeamIDs)
	caps, _ := json.Marshal(input.Capabilities)
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_manager_grants(manager_id,organization_id,role,all_teams,team_ids,capabilities,revision)
		VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(manager_id,organization_id) DO UPDATE SET
		role=EXCLUDED.role,all_teams=EXCLUDED.all_teams,team_ids=EXCLUDED.team_ids,capabilities=EXCLUDED.capabilities,
		revision=EXCLUDED.revision,revoked_at=NULL,updated_at=NOW()`, managerID, organizationID, input.Role, input.AllTeams, string(teams), string(caps), current+1)
	if err != nil {
		return Grant{}, err
	}
	if err := bumpAuthorization(ctx, tx, managerID, organizationID); err != nil {
		return Grant{}, err
	}
	if err := recordSecurityEvent(ctx, tx, actor, "grant.save", managerID+":"+organizationID, requestID); err != nil {
		return Grant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Grant{}, err
	}
	return Grant{ManagerID: managerID, UserID: input.UserID, DisplayName: u.Username, Role: input.Role,
		AllTeams: input.AllTeams, TeamIDs: input.TeamIDs, Capabilities: input.Capabilities, Revision: current + 1}, nil
}

func bumpAuthorization(ctx context.Context, tx *sql.Tx, managerID, organizationID string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE bi_managers SET authorization_version=authorization_version+1 WHERE id=$1`, managerID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE bi_organizations SET authorization_version=authorization_version+1 WHERE id=$1`, organizationID)
	return err
}

func (s *Service) RevokeGrant(ctx context.Context, organizationID, managerID string, expected int64, actorUserID int64, requestID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var targetUserID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM bi_managers WHERE id=$1`, managerID).Scan(&targetUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	managers, err := lockGrantManagers(ctx, tx, targetUserID, actorUserID)
	if err != nil {
		return err
	}
	actor := managers[actorUserID]
	// Manager rows are locked in user-ID order before the organization row.
	var desktopID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM desktop_organizations WHERE public_id=$1 AND deleted_at IS NULL FOR UPDATE`, organizationID).Scan(&desktopID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var current int64
	var revoked sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT revision,revoked_at FROM bi_manager_grants WHERE manager_id=$1 AND organization_id=$2 FOR UPDATE`, managerID, organizationID).Scan(&current, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current != expected {
		return apiError(409, "CONFLICT", "Grant changed; reload before revoking")
	}
	if revoked.Valid {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bi_manager_grants SET revoked_at=NOW(),revision=revision+1,updated_at=NOW() WHERE manager_id=$1 AND organization_id=$2`, managerID, organizationID); err != nil {
		return err
	}
	if err := bumpAuthorization(ctx, tx, managerID, organizationID); err != nil {
		return err
	}
	if err := recordSecurityEvent(ctx, tx, actor, "grant.revoke", managerID+":"+organizationID, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockGrantManagers(ctx context.Context, tx *sql.Tx, userIDs ...int64) (map[int64]string, error) {
	slices.Sort(userIDs)
	result := map[int64]string{}
	for _, id := range slices.Compact(userIDs) {
		manager, err := ensureManager(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		result[id] = manager
	}
	return result, nil
}

func (s *Service) ListGrants(ctx context.Context, organizationID string, offset, limit int) ([]Grant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.manager_id,m.user_id,COALESCE(u.username,''),g.role,g.all_teams,g.team_ids,g.capabilities,g.revision,g.revoked_at IS NOT NULL
		FROM bi_manager_grants g JOIN bi_managers m ON m.id=g.manager_id JOIN users u ON u.id=m.user_id
		WHERE g.organization_id=$1 ORDER BY g.manager_id LIMIT $2 OFFSET $3`, organizationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Grant{}
	for rows.Next() {
		var g Grant
		var teams, caps []byte
		if err := rows.Scan(&g.ManagerID, &g.UserID, &g.DisplayName, &g.Role, &g.AllTeams, &teams, &caps, &g.Revision, &g.Revoked); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(teams, &g.TeamIDs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(caps, &g.Capabilities); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func adminRespond(c *gin.Context, result any, err error) {
	if err != nil {
		var typed *Error
		if !errors.As(err, &typed) {
			typed = apiError(500, "INTERNAL_ERROR", "BI operation failed")
		}
		response.ErrorWithDetails(c, typed.Status, typed.Message, typed.Code, nil)
		return
	}
	response.Success(c, result)
}
