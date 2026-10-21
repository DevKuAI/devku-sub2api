//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func createDesktopTestPosition(t *testing.T, repo service.DesktopRepository, org string) *service.DesktopOrganizationUnit {
	t.Helper()
	ctx := context.Background()
	r := repo.(service.DesktopStructureRepository)
	existing, _, err := r.ListOrganizationUnits(ctx, org, service.DesktopPositionKind, pagination.DefaultPagination(), service.DesktopStructureFilter{})
	require.NoError(t, err)
	if len(existing) > 0 {
		return &existing[0]
	}
	d := createDesktopTestUnit(t, r, org, service.DesktopDepartmentKind, "", "Test Department")
	return createDesktopTestUnit(t, r, org, service.DesktopPositionKind, d.PublicID, "Test Position")
}
func createDesktopTestUnit(t *testing.T, r service.DesktopStructureRepository, org, kind, department, name string) *service.DesktopOrganizationUnit {
	t.Helper()
	in := service.DesktopSaveUnitInput{Name: &name, DepartmentID: department}
	require.NoError(t, service.ValidateDesktopUnitInput(kind, "", &in))
	unit, err := r.SaveOrganizationUnit(context.Background(), org, kind, "", in)
	require.NoError(t, err)
	return unit
}

func TestDesktopStructureAssignmentsIsolationHistoryAndCredentials(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "structure", 10)
	other := newDesktopRepositoryFixture(t, "structother", 10)
	r := f.repo.(service.DesktopStructureRepository)
	member := f.createMember(t, "structure", 1)
	require.Equal(t, int64(1), member.PositionRevision)
	require.Equal(t, f.position.PublicID, member.Position.PublicID)
	department := createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopDepartmentKind, "", "Delivery")
	position := createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopPositionKind, department.PublicID, "Project Manager")
	version := int64(1)
	_, err := r.AssignMemberPosition(ctx, f.organization.PublicID, member.PublicID, service.DesktopAssignPositionInput{PositionID: other.position.PublicID, ExpectedRevision: &version})
	require.ErrorIs(t, err, service.ErrDesktopStructureNotFound)
	_, err = f.repo.ScopedToGatewayUser(other.user.ID).(service.DesktopStructureRepository).AssignMemberPosition(ctx, f.organization.PublicID, member.PublicID, service.DesktopAssignPositionInput{PositionID: position.PublicID, ExpectedRevision: &version})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	updated, err := r.AssignMemberPosition(ctx, f.organization.PublicID, member.PublicID, service.DesktopAssignPositionInput{PositionID: position.PublicID, ExpectedRevision: &version, ActorID: f.user.ID, Reason: "Confirmed transfer"})
	require.NoError(t, err)
	require.Equal(t, member.CurrentAPIKeyID, updated.CurrentAPIKeyID)
	require.Equal(t, member.AuthVersion, updated.AuthVersion)
	require.Equal(t, int64(2), updated.PositionRevision)
	require.Equal(t, department.PublicID, updated.Department.PublicID)
	history, page, err := r.MemberPositionHistory(ctx, f.organization.PublicID, member.PublicID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Nil(t, history[0].EndedAt)
	require.NotNil(t, history[1].EndedAt)
	require.Equal(t, history[0].StartedAt, *history[1].EndedAt)
	_, err = r.AssignMemberPosition(ctx, f.organization.PublicID, member.PublicID, service.DesktopAssignPositionInput{PositionID: f.position.PublicID, ExpectedRevision: &version})
	require.ErrorIs(t, err, service.ErrDesktopStructureConflict)
	version = 2
	_, err = r.AssignMemberPosition(ctx, f.organization.PublicID, member.PublicID, service.DesktopAssignPositionInput{PositionID: position.PublicID, ExpectedRevision: &version})
	require.NoError(t, err)
	_, page, err = r.MemberPositionHistory(ctx, f.organization.PublicID, member.PublicID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	status, err := r.OrganizationStructureStatus(ctx, f.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, "complete", status.Status)
	_, err = f.repo.DeleteMember(ctx, f.organization.PublicID, member.PublicID)
	require.NoError(t, err)
	var open int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM desktop_member_position_history WHERE member_id=$1 AND ended_at IS NULL", member.ID).Scan(&open))
	require.Zero(t, open)
}

func TestDesktopStructureDisableNormalizationAndConcurrency(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "structrace", 10)
	r := f.repo.(service.DesktopStructureRepository)
	d := createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopDepartmentKind, "", "Café")
	name := " CAFE\u0301 "
	in := service.DesktopSaveUnitInput{Name: &name}
	require.NoError(t, service.ValidateDesktopUnitInput(service.DesktopDepartmentKind, "", &in))
	_, err := r.SaveOrganizationUnit(ctx, f.organization.PublicID, service.DesktopDepartmentKind, "", in)
	require.ErrorIs(t, err, service.ErrDesktopStructureNameExists)
	p := createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopPositionKind, d.PublicID, "Engineer")
	createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopPositionKind, d.PublicID, f.position.Name)
	m := f.createMember(t, "race", 1)
	revision := int64(1)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, position := range []string{p.PublicID, createDesktopTestUnit(t, r, f.organization.PublicID, service.DesktopPositionKind, d.PublicID, "Reviewer").PublicID} {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			<-start
			_, e := r.AssignMemberPosition(ctx, f.organization.PublicID, m.PublicID, service.DesktopAssignPositionInput{PositionID: target, ExpectedRevision: &revision})
			results <- e
		}(position)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, service.ErrDesktopStructureConflict) {
			conflict++
		} else {
			require.NoError(t, e)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	current, err := f.repo.GetMember(ctx, f.organization.PublicID, m.PublicID)
	require.NoError(t, err)
	disabled := service.DesktopStatusDisabled
	_, _, err = f.repo.UpdateMember(ctx, f.organization.PublicID, m.PublicID, service.DesktopUpdateMemberInput{Status: &disabled})
	require.NoError(t, err)
	_, err = r.SaveOrganizationUnit(ctx, f.organization.PublicID, service.DesktopPositionKind, current.Position.PublicID, service.DesktopSaveUnitInput{ExpectedRevision: 1, Status: &disabled})
	require.ErrorIs(t, err, service.ErrDesktopStructureInUse)
	_, err = r.SaveOrganizationUnit(ctx, f.organization.PublicID, service.DesktopDepartmentKind, d.PublicID, service.DesktopSaveUnitInput{ExpectedRevision: 1, Status: &disabled})
	require.ErrorIs(t, err, service.ErrDesktopStructureInUse)
	// Cross-enterprise foreign keys remain effective even outside the application.
	other := newDesktopRepositoryFixture(t, "structfk", 10)
	var foreignPosition int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM desktop_positions WHERE public_id=$1", other.position.PublicID).Scan(&foreignPosition))
	_, err = integrationDB.ExecContext(ctx, "UPDATE desktop_members SET position_id=$1 WHERE id=$2", foreignPosition, m.ID)
	require.Error(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE desktop_positions SET department_id=(SELECT id FROM desktop_departments WHERE public_id=$1) WHERE public_id=$2", other.position.Department.PublicID, p.PublicID)
	require.Error(t, err)
}

func TestDesktopStructureMigrationAssignmentAndDeferredConstraint(t *testing.T) {
	ctx := context.Background()
	f := newDesktopRepositoryFixture(t, "structlegacy", 10)
	r := f.repo.(service.DesktopStructureRepository)
	legacy, err := integrationEntClient.DesktopMember.Create().SetOrganizationID(f.organization.ID).SetPublicID("mem_" + f.organization.Code).SetName("Legacy").SetNameNormalized("legacy").SetPhone(desktopTestPhone(f.suffix + 9)).SetStatus("disabled").Save(ctx)
	require.NoError(t, err)
	status, err := r.OrganizationStructureStatus(ctx, f.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, 1, status.UnassignedMembers)
	revision := int64(0)
	_, err = r.AssignMemberPosition(ctx, f.organization.PublicID, legacy.PublicID, service.DesktopAssignPositionInput{PositionID: f.position.PublicID, ExpectedRevision: &revision})
	require.NoError(t, err)
	history, _, err := r.MemberPositionHistory(ctx, f.organization.PublicID, legacy.PublicID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.Equal(t, "migration_confirmed", history[0].Source)
	raw, err := os.ReadFile("../../migrations/deferred/desktop_member_position_required.sql")
	require.NoError(t, err)
	// Shadow production names to test enforcement without changing the shared test database.
	tx := testTx(t)
	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE desktop_organizations(id BIGINT) ON COMMIT DROP;
 CREATE TEMP TABLE desktop_members(id BIGINT,organization_id BIGINT,position_id BIGINT,position_revision BIGINT,deleted_at TIMESTAMPTZ) ON COMMIT DROP;
 CREATE TEMP TABLE desktop_positions(id BIGINT) ON COMMIT DROP;
 CREATE TEMP TABLE desktop_departments(id BIGINT) ON COMMIT DROP;
 CREATE TEMP TABLE desktop_member_position_history(member_id BIGINT,organization_id BIGINT,position_id BIGINT,position_revision BIGINT,ended_at TIMESTAMPTZ) ON COMMIT DROP;
 INSERT INTO desktop_members VALUES(1,1,1,1,NULL),(2,1,NULL,0,NOW());
 INSERT INTO desktop_member_position_history VALUES(1,1,1,1,NULL);`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SAVEPOINT incomplete_mapping")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO desktop_members VALUES(3,1,NULL,0,NULL)")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(raw))
	require.ErrorContains(t, err, "assignments are incomplete or inconsistent")
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT incomplete_mapping")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(raw))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(raw))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO desktop_members VALUES(3,1,NULL,0,NULL)")
	require.Error(t, err)
}
