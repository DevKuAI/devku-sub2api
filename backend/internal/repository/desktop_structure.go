package repository

import (
	"context"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/desktopdepartment"
	"github.com/Wei-Shaw/sub2api/ent/desktopmember"
	"github.com/Wei-Shaw/sub2api/ent/desktopmemberpositionhistory"
	"github.com/Wei-Shaw/sub2api/ent/desktoporganization"
	"github.com/Wei-Shaw/sub2api/ent/desktopposition"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func departmentUnit(row *dbent.DesktopDepartment) service.DesktopOrganizationUnit {
	result := service.DesktopOrganizationUnit{DesktopStructureRef: service.DesktopStructureRef{PublicID: row.PublicID, Name: row.Name}, Status: row.Status, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	for _, p := range row.Edges.Positions {
		result.MemberCount += len(p.Edges.Members)
	}
	return result
}
func positionUnit(row *dbent.DesktopPosition) service.DesktopOrganizationUnit {
	result := service.DesktopOrganizationUnit{DesktopStructureRef: service.DesktopStructureRef{PublicID: row.PublicID, Name: row.Name}, Status: row.Status, Revision: row.Revision, MemberCount: len(row.Edges.Members), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if d := row.Edges.Department; d != nil {
		result.Department = &service.DesktopStructureRef{PublicID: d.PublicID, Name: d.Name}
	}
	return result
}
func structureMembers(q *dbent.DesktopMemberQuery) { q.Select(desktopmember.FieldID) }

func (r *desktopRepository) ListOrganizationUnits(ctx context.Context, org, kind string, params pagination.PaginationParams, filter service.DesktopStructureFilter) ([]service.DesktopOrganizationUnit, *pagination.PaginationResult, error) {
	organization, err := r.GetOrganization(ctx, org)
	if err != nil {
		return nil, nil, err
	}
	out := []service.DesktopOrganizationUnit{}
	if kind == service.DesktopDepartmentKind {
		q := r.client.DesktopDepartment.Query().Where(desktopdepartment.OrganizationIDEQ(organization.ID))
		if filter.Search != "" {
			q.Where(desktopdepartment.NameContainsFold(filter.Search))
		}
		if filter.Status != "" {
			q.Where(desktopdepartment.StatusEQ(filter.Status))
		}
		count, e := q.Count(ctx)
		if e != nil {
			return nil, nil, e
		}
		rows, e := q.WithPositions(func(q *dbent.DesktopPositionQuery) { q.WithMembers(structureMembers) }).Order(dbent.Asc(desktopdepartment.FieldNameNormalized), dbent.Asc(desktopdepartment.FieldID)).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
		if e != nil {
			return nil, nil, e
		}
		for _, row := range rows {
			out = append(out, departmentUnit(row))
		}
		return out, paginationResultFromTotal(int64(count), params), nil
	}
	q := r.client.DesktopPosition.Query().Where(desktopposition.OrganizationIDEQ(organization.ID))
	if filter.Search != "" {
		q.Where(desktopposition.NameContainsFold(filter.Search))
	}
	if filter.Status != "" {
		q.Where(desktopposition.StatusEQ(filter.Status))
	}
	if filter.DepartmentID != "" {
		q.Where(desktopposition.HasDepartmentWith(desktopdepartment.PublicIDEQ(filter.DepartmentID)))
	}
	count, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.WithDepartment().WithMembers(structureMembers).Order(dbent.Asc(desktopposition.FieldNameNormalized), dbent.Asc(desktopposition.FieldID)).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		out = append(out, positionUnit(row))
	}
	return out, paginationResultFromTotal(int64(count), params), nil
}

func (r *desktopRepository) GetOrganizationUnit(ctx context.Context, org, kind, id string) (*service.DesktopOrganizationUnit, error) {
	organization, err := r.GetOrganization(ctx, org)
	if err != nil {
		return nil, err
	}
	if kind == service.DesktopDepartmentKind {
		row, e := r.client.DesktopDepartment.Query().Where(desktopdepartment.OrganizationIDEQ(organization.ID), desktopdepartment.PublicIDEQ(id)).WithPositions(func(q *dbent.DesktopPositionQuery) { q.WithMembers(structureMembers) }).Only(ctx)
		if e != nil {
			return nil, translatePersistenceError(e, service.ErrDesktopStructureNotFound, nil)
		}
		result := departmentUnit(row)
		return &result, nil
	}
	row, e := r.client.DesktopPosition.Query().Where(desktopposition.OrganizationIDEQ(organization.ID), desktopposition.PublicIDEQ(id)).WithDepartment().WithMembers(structureMembers).Only(ctx)
	if e != nil {
		return nil, translatePersistenceError(e, service.ErrDesktopStructureNotFound, nil)
	}
	result := positionUnit(row)
	return &result, nil
}

func (r *desktopRepository) lockStructureOrganization(ctx context.Context, c *dbent.Client, org string) (*dbent.DesktopOrganization, error) {
	row, err := c.DesktopOrganization.Query().Where(r.organizationPredicates(desktoporganization.PublicIDEQ(org))...).ForUpdate().Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	if row.Status != service.DesktopStatusActive {
		return nil, service.ErrDesktopOrganizationDisabled
	}
	return row, nil
}

func (r *desktopRepository) SaveOrganizationUnit(ctx context.Context, org, kind, id string, in service.DesktopSaveUnitInput) (*service.DesktopOrganizationUnit, error) {
	err := r.withTx(ctx, func(txCtx context.Context, c *dbent.Client) error {
		organization, e := r.lockStructureOrganization(txCtx, c, org)
		if e != nil {
			return e
		}
		if kind == service.DesktopDepartmentKind {
			return saveDesktopDepartment(txCtx, c, organization.ID, id, in)
		}
		return saveDesktopPosition(txCtx, c, organization.ID, id, in)
	})
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, service.ErrDesktopStructureNameExists
		}
		return nil, err
	}
	if id == "" {
		id = in.PublicID
	}
	return r.GetOrganizationUnit(ctx, org, kind, id)
}

func saveDesktopDepartment(ctx context.Context, c *dbent.Client, org int64, id string, in service.DesktopSaveUnitInput) error {
	if id == "" {
		_, err := c.DesktopDepartment.Create().SetPublicID(in.PublicID).SetOrganizationID(org).SetName(*in.Name).SetNameNormalized(in.NameNormalized).Save(ctx)
		return err
	}
	row, err := c.DesktopDepartment.Query().Where(desktopdepartment.OrganizationIDEQ(org), desktopdepartment.PublicIDEQ(id)).Only(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrDesktopStructureNotFound, nil)
	}
	if row.Revision != in.ExpectedRevision {
		return service.ErrDesktopStructureConflict
	}
	if in.Status != nil && *in.Status == service.DesktopStatusDisabled {
		active, e := c.DesktopPosition.Query().Where(desktopposition.DepartmentIDEQ(row.ID), desktopposition.StatusEQ(service.DesktopStatusActive)).Exist(ctx)
		if e != nil {
			return e
		}
		members, e := c.DesktopMember.Query().Where(desktopmember.HasPositionWith(desktopposition.DepartmentIDEQ(row.ID))).Exist(ctx)
		if e != nil {
			return e
		}
		if active || members {
			return service.ErrDesktopStructureInUse
		}
	}
	u := c.DesktopDepartment.UpdateOne(row).AddRevision(1)
	if in.Name != nil {
		u.SetName(*in.Name).SetNameNormalized(in.NameNormalized)
	}
	if in.Status != nil {
		u.SetStatus(*in.Status)
	}
	_, err = u.Save(ctx)
	return err
}

func saveDesktopPosition(ctx context.Context, c *dbent.Client, org int64, id string, in service.DesktopSaveUnitInput) error {
	if id == "" {
		d, err := c.DesktopDepartment.Query().Where(desktopdepartment.OrganizationIDEQ(org), desktopdepartment.PublicIDEQ(in.DepartmentID)).Only(ctx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopStructureNotFound, nil)
		}
		if d.Status != service.DesktopStatusActive {
			return service.ErrDesktopStructureDisabled
		}
		_, err = c.DesktopPosition.Create().SetPublicID(in.PublicID).SetOrganizationID(org).SetDepartmentID(d.ID).SetName(*in.Name).SetNameNormalized(in.NameNormalized).Save(ctx)
		return err
	}
	row, err := c.DesktopPosition.Query().Where(desktopposition.OrganizationIDEQ(org), desktopposition.PublicIDEQ(id)).WithDepartment().Only(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrDesktopStructureNotFound, nil)
	}
	if row.Revision != in.ExpectedRevision {
		return service.ErrDesktopStructureConflict
	}
	if in.Status != nil {
		if *in.Status == service.DesktopStatusDisabled {
			used, e := c.DesktopMember.Query().Where(desktopmember.PositionIDEQ(row.ID)).Exist(ctx)
			if e != nil {
				return e
			}
			if used {
				return service.ErrDesktopStructureInUse
			}
		}
		if *in.Status == service.DesktopStatusActive && row.Edges.Department.Status != service.DesktopStatusActive {
			return service.ErrDesktopStructureDisabled
		}
	}
	u := c.DesktopPosition.UpdateOne(row).AddRevision(1)
	if in.Name != nil {
		u.SetName(*in.Name).SetNameNormalized(in.NameNormalized)
	}
	if in.Status != nil {
		u.SetStatus(*in.Status)
	}
	_, err = u.Save(ctx)
	return err
}

func desktopAssignmentPosition(ctx context.Context, c *dbent.Client, org int64, id string) (*dbent.DesktopPosition, error) {
	if id == "" {
		return nil, service.ErrDesktopValidation.WithMetadata(map[string]string{"field": "position_id"})
	}
	p, err := c.DesktopPosition.Query().Where(desktopposition.OrganizationIDEQ(org), desktopposition.PublicIDEQ(id)).WithDepartment().Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopStructureNotFound, nil)
	}
	if p.Status != service.DesktopStatusActive || p.Edges.Department.Status != service.DesktopStatusActive {
		return nil, service.ErrDesktopStructureDisabled
	}
	return p, nil
}

func desktopStructureTime(ctx context.Context, c *dbent.Client) (time.Time, error) {
	rows, err := c.QueryContext(ctx, "SELECT clock_timestamp()")
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return time.Time{}, errors.New("missing database clock")
	}
	var at time.Time
	err = rows.Scan(&at)
	return at, err
}

func createDesktopPositionHistory(ctx context.Context, c *dbent.Client, member *dbent.DesktopMember, p *dbent.DesktopPosition, source string, in service.DesktopAssignPositionInput, at time.Time) error {
	u := c.DesktopMemberPositionHistory.Create().SetOrganizationID(member.OrganizationID).SetMemberID(member.ID).SetPositionID(p.ID).SetPositionRevision(member.PositionRevision).SetDepartmentNameSnapshot(p.Edges.Department.Name).SetPositionNameSnapshot(p.Name).SetStartedAt(at).SetSource(source).SetReason(in.Reason).SetRequestID(in.RequestID)
	if in.ActorID > 0 {
		u.SetChangedByUserID(in.ActorID)
	}
	_, err := u.Save(ctx)
	return err
}

func closeDesktopPositionHistory(ctx context.Context, c *dbent.Client, member *dbent.DesktopMember, at time.Time) (time.Time, error) {
	rows, err := c.DesktopMemberPositionHistory.Query().Where(desktopmemberpositionhistory.MemberIDEQ(member.ID), desktopmemberpositionhistory.EndedAtIsNil()).All(ctx)
	if err != nil {
		return at, err
	}
	if member.PositionID == nil {
		if len(rows) != 0 {
			return at, service.ErrDesktopStructureConflict
		}
		return at, nil
	}
	if len(rows) != 1 || rows[0].PositionID != *member.PositionID || rows[0].PositionRevision != member.PositionRevision {
		return at, service.ErrDesktopStructureConflict
	}
	if !at.After(rows[0].StartedAt) {
		at = rows[0].StartedAt.Add(time.Microsecond)
	}
	_, err = c.DesktopMemberPositionHistory.UpdateOne(rows[0]).SetEndedAt(at).Save(ctx)
	return at, err
}

func (r *desktopRepository) AssignMemberPosition(ctx context.Context, org, memberID string, in service.DesktopAssignPositionInput) (*service.DesktopMember, error) {
	if err := service.ValidateDesktopAssignment(&in); err != nil {
		return nil, err
	}
	err := r.withTx(ctx, func(txCtx context.Context, c *dbent.Client) error {
		organization, e := r.lockStructureOrganization(txCtx, c, org)
		if e != nil {
			return e
		}
		m, e := c.DesktopMember.Query().Where(desktopmember.OrganizationIDEQ(organization.ID), desktopmember.PublicIDEQ(memberID)).ForUpdate().Only(txCtx)
		if e != nil {
			return translatePersistenceError(e, service.ErrDesktopMemberNotFound, nil)
		}
		if m.PositionRevision != *in.ExpectedRevision {
			return service.ErrDesktopStructureConflict
		}
		p, e := desktopAssignmentPosition(txCtx, c, organization.ID, in.PositionID)
		if e != nil {
			return e
		}
		if m.PositionID != nil && *m.PositionID == p.ID {
			return nil
		}
		at, e := desktopStructureTime(txCtx, c)
		if e != nil {
			return e
		}
		at, e = closeDesktopPositionHistory(txCtx, c, m, at)
		if e != nil {
			return e
		}
		source := "transfer"
		if m.PositionID == nil {
			source = "migration_confirmed"
		}
		m, e = c.DesktopMember.UpdateOne(m).SetPositionID(p.ID).AddPositionRevision(1).Save(txCtx)
		if e != nil {
			return e
		}
		return createDesktopPositionHistory(txCtx, c, m, p, source, in, at)
	})
	if err != nil {
		return nil, err
	}
	return r.GetMember(ctx, org, memberID)
}

func (r *desktopRepository) MemberPositionHistory(ctx context.Context, org, memberID string, params pagination.PaginationParams) ([]service.DesktopPositionHistory, *pagination.PaginationResult, error) {
	member, err := r.GetMember(ctx, org, memberID)
	if err != nil {
		return nil, nil, err
	}
	q := r.client.DesktopMemberPositionHistory.Query().Where(desktopmemberpositionhistory.MemberIDEQ(member.ID))
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.Order(dbent.Desc(desktopmemberpositionhistory.FieldPositionRevision)).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	positions, err := r.client.DesktopPosition.Query().Where(desktopposition.OrganizationIDEQ(member.OrganizationID)).WithDepartment().All(ctx)
	if err != nil {
		return nil, nil, err
	}
	byID := map[int64]*dbent.DesktopPosition{}
	for _, p := range positions {
		byID[p.ID] = p
	}
	out := []service.DesktopPositionHistory{}
	for _, h := range rows {
		p := byID[h.PositionID]
		if p == nil {
			return nil, nil, service.ErrDesktopStructureConflict
		}
		out = append(out, service.DesktopPositionHistory{Position: service.DesktopStructureRef{PublicID: p.PublicID, Name: h.PositionNameSnapshot}, Department: service.DesktopStructureRef{PublicID: p.Edges.Department.PublicID, Name: h.DepartmentNameSnapshot}, Revision: h.PositionRevision, StartedAt: h.StartedAt, EndedAt: h.EndedAt, Source: h.Source, Reason: h.Reason})
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopRepository) OrganizationStructureStatus(ctx context.Context, org string) (*service.DesktopStructureStatus, error) {
	organization, err := r.GetOrganization(ctx, org)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.QueryContext(ctx, `SELECT COUNT(*) FILTER(WHERE m.position_id IS NULL),COUNT(*) FILTER(WHERE m.position_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM desktop_member_position_history h WHERE h.member_id=m.id AND h.position_id=m.position_id AND h.position_revision=m.position_revision AND h.ended_at IS NULL)),EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='desktop_members'::regclass AND conname='desktop_members_position_required' AND convalidated) FROM desktop_members m WHERE m.organization_id=$1 AND m.deleted_at IS NULL`, organization.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, errors.New("missing organization structure counts")
	}
	out := &service.DesktopStructureStatus{Status: "complete"}
	if err = rows.Scan(&out.UnassignedMembers, &out.InconsistentMembers, &out.ConstraintEnabled); err != nil {
		return nil, err
	}
	if out.UnassignedMembers > 0 || out.InconsistentMembers > 0 {
		out.Status = "needs_assignment"
	}
	return out, nil
}
