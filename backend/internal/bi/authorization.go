package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

type OrganizationScope struct {
	Organization
	GrantRevision        int64
	AuthorizationVersion int64
}

// AuthorizeOrganization never uses the capability union exposed by /me.
func (s *Service) AuthorizeOrganization(ctx context.Context, p Principal, organizationID, capability string) (OrganizationScope, error) {
	return authorizeOrganization(ctx, s.db, p, organizationID, capability)
}

func authorizeOrganization(ctx context.Context, q queryer, p Principal, organizationID, capability string) (OrganizationScope, error) {
	var scope OrganizationScope
	var teams, caps []byte
	err := q.QueryRowContext(ctx, `SELECT o.id,d.name,d.status,g.role,g.all_teams,g.team_ids,g.capabilities,g.revision,o.authorization_version
		FROM bi_manager_grants g JOIN bi_organizations o ON o.id=g.organization_id JOIN desktop_organizations d ON d.id=o.desktop_organization_id
		WHERE g.manager_id=$1 AND o.id=$2 AND g.revoked_at IS NULL AND d.deleted_at IS NULL AND d.status='active'`,
		p.ManagerID, organizationID).Scan(&scope.ID, &scope.Name, &scope.Status, &scope.Role, &scope.AllTeams, &teams, &caps, &scope.GrantRevision, &scope.AuthorizationVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return scope, ErrNotFound
	}
	if err != nil {
		return scope, err
	}
	if err := json.Unmarshal(teams, &scope.TeamIDs); err != nil {
		return scope, err
	}
	if err := json.Unmarshal(caps, &scope.Capabilities); err != nil {
		return scope, err
	}
	if !slices.Contains(scope.Capabilities, capability) {
		return scope, ErrForbidden
	}
	return scope, nil
}
