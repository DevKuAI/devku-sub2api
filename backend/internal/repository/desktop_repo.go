package repository

import (
	"context"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/desktopmember"
	"github.com/Wei-Shaw/sub2api/ent/desktopmemberapikey"
	"github.com/Wei-Shaw/sub2api/ent/desktoporganization"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type desktopRepository struct {
	client  *dbent.Client
	apiKeys service.APIKeyRepository
}

func (r *desktopRepository) createAPIKeyWithinLimit(ctx context.Context, key *service.APIKey) error {
	repo, ok := r.apiKeys.(service.APIKeyCreationLimitRepository)
	if !ok {
		return errors.New("API key repository does not support atomic creation limits")
	}
	return repo.CreateWithinUserLimit(ctx, key)
}

func NewDesktopRepository(client *dbent.Client, apiKeys service.APIKeyRepository) service.DesktopRepository {
	return &desktopRepository{client: client, apiKeys: apiKeys}
}

func (r *desktopRepository) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *desktopRepository) CreateOrganization(ctx context.Context, input service.DesktopCreateOrganizationInput) (*service.DesktopOrganization, error) {
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		gatewayUser, err := client.User.Query().Where(user.IDEQ(input.GatewayUserID)).WithAllowedGroups().ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}
		groupEntity, err := client.Group.Query().Where(group.IDEQ(input.GroupID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrGroupNotFound, nil)
		}
		if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
			return err
		}
		assigned, err := client.DesktopOrganization.Query().
			Where(desktoporganization.GatewayUserIDEQ(input.GatewayUserID)).
			Exist(txCtx)
		if err != nil {
			return err
		}
		if assigned {
			return service.ErrDesktopGatewayUserAssigned
		}
		_, err = client.DesktopOrganization.Create().
			SetPublicID(input.PublicID).
			SetCode(input.Code).
			SetName(input.Name).
			SetGatewayUserID(input.GatewayUserID).
			SetGroupID(input.GroupID).
			Save(txCtx)
		return translatePersistenceError(err, nil, service.ErrDesktopGatewayUserAssigned)
	})
	if err != nil {
		return nil, err
	}
	return r.GetOrganization(ctx, input.PublicID)
}

func (r *desktopRepository) ListOrganizations(ctx context.Context, params pagination.PaginationParams, filters service.DesktopOrganizationListFilters) ([]service.DesktopOrganization, *pagination.PaginationResult, error) {
	q := r.client.DesktopOrganization.Query()
	if filters.Search != "" {
		q = q.Where(desktoporganization.Or(
			desktoporganization.NameContainsFold(filters.Search),
			desktoporganization.CodeContainsFold(filters.Search),
		))
	}
	if filters.Status != "" {
		q = q.Where(desktoporganization.StatusEQ(filters.Status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.WithGatewayUser().WithGroup().WithMembers().
		Order(dbent.Desc(desktoporganization.FieldUpdatedAt), dbent.Desc(desktoporganization.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]service.DesktopOrganization, 0, len(rows))
	for _, row := range rows {
		item, err := desktopOrganizationEntityToService(row)
		if err != nil {
			return nil, nil, err
		}
		result = append(result, *item)
	}
	return result, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopRepository) GetOrganization(ctx context.Context, publicID string) (*service.DesktopOrganization, error) {
	row, err := r.client.DesktopOrganization.Query().
		Where(desktoporganization.PublicIDEQ(publicID)).
		WithGatewayUser().WithGroup().WithMembers().Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	return desktopOrganizationEntityToService(row)
}

func (r *desktopRepository) UpdateOrganization(ctx context.Context, publicID string, input service.DesktopUpdateOrganizationInput) (*service.DesktopOrganization, []string, error) {
	current, err := r.client.DesktopOrganization.Query().Where(desktoporganization.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		return nil, nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	newUserID, newGroupID := current.GatewayUserID, current.GroupID
	if input.GatewayUserID != nil {
		newUserID = *input.GatewayUserID
	}
	if input.GroupID != nil {
		newGroupID = *input.GroupID
	}
	var invalidated []string
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		gatewayUser, err := client.User.Query().Where(user.IDEQ(newUserID)).WithAllowedGroups().ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}
		groupEntity, err := client.Group.Query().Where(group.IDEQ(newGroupID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrGroupNotFound, nil)
		}
		organization, err := client.DesktopOrganization.Query().Where(desktoporganization.PublicIDEQ(publicID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
		}
		carrierChanged := organization.GatewayUserID != newUserID || organization.GroupID != newGroupID
		if carrierChanged {
			memberCount, err := client.DesktopMember.Query().Where(desktopmember.OrganizationIDEQ(organization.ID)).Count(txCtx)
			if err != nil {
				return err
			}
			if memberCount > 0 {
				return service.ErrDesktopProvisioningLocked
			}
			if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
				return err
			}
			assigned, err := client.DesktopOrganization.Query().Where(
				desktoporganization.GatewayUserIDEQ(newUserID),
				desktoporganization.IDNEQ(organization.ID),
			).Exist(txCtx)
			if err != nil {
				return err
			}
			if assigned {
				return service.ErrDesktopGatewayUserAssigned
			}
		}
		builder := client.DesktopOrganization.UpdateOne(organization)
		if input.Name != nil {
			builder.SetName(*input.Name)
		}
		if carrierChanged {
			builder.SetGatewayUserID(newUserID).SetGroupID(newGroupID)
		}
		if input.Status != nil && *input.Status != organization.Status {
			if *input.Status == service.DesktopStatusActive {
				if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
					return err
				}
			}
			keys, err := r.applyOrganizationStatus(txCtx, client, organization.ID, *input.Status)
			if err != nil {
				return err
			}
			invalidated = append(invalidated, keys...)
			builder.SetStatus(*input.Status).AddAuthVersion(1)
		}
		_, err = builder.Save(txCtx)
		return translatePersistenceError(err, nil, service.ErrDesktopGatewayUserAssigned)
	})
	if err != nil {
		return nil, nil, err
	}
	updated, err := r.GetOrganization(ctx, publicID)
	return updated, invalidated, err
}

func (r *desktopRepository) applyOrganizationStatus(ctx context.Context, client *dbent.Client, organizationID int64, status string) ([]string, error) {
	members, err := client.DesktopMember.Query().Where(desktopmember.OrganizationIDEQ(organizationID)).ForUpdate().All(ctx)
	if err != nil {
		return nil, err
	}
	memberIDs := make([]int64, 0, len(members))
	memberByID := make(map[int64]*dbent.DesktopMember, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.ID)
		memberByID[member.ID] = member
	}
	if len(memberIDs) == 0 {
		return nil, nil
	}
	assignments, err := client.DesktopMemberAPIKey.Query().Where(
		desktopmemberapikey.MemberIDIn(memberIDs...),
		desktopmemberapikey.RetiredAtIsNil(),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	keyIDs := make([]int64, 0, len(assignments))
	assignmentByKey := make(map[int64]int64, len(assignments))
	for _, assignment := range assignments {
		keyIDs = append(keyIDs, assignment.APIKeyID)
		assignmentByKey[assignment.APIKeyID] = assignment.MemberID
	}
	if len(keyIDs) == 0 {
		return nil, nil
	}
	keys, err := client.APIKey.Query().Where(apikey.IDIn(keyIDs...)).ForUpdate().All(ctx)
	if err != nil {
		return nil, err
	}
	invalidated := make([]string, 0, len(keys))
	for _, key := range keys {
		member := memberByID[assignmentByKey[key.ID]]
		if status == service.DesktopStatusDisabled {
			if member.Status == service.DesktopStatusActive && key.Status == service.StatusAPIKeyActive {
				if _, err := client.APIKey.UpdateOne(key).SetStatus(service.StatusAPIKeyDisabled).Save(ctx); err != nil {
					return nil, err
				}
				if _, err := client.DesktopMember.UpdateOne(member).SetAPIKeySuspendedByOrganization(true).Save(ctx); err != nil {
					return nil, err
				}
				invalidated = append(invalidated, key.Key)
			}
			continue
		}
		if member.Status == service.DesktopStatusActive && member.APIKeySuspendedByOrganization {
			if _, err := client.APIKey.UpdateOne(key).SetStatus(service.StatusAPIKeyActive).Save(ctx); err != nil {
				return nil, err
			}
			if _, err := client.DesktopMember.UpdateOne(member).SetAPIKeySuspendedByOrganization(false).Save(ctx); err != nil {
				return nil, err
			}
			invalidated = append(invalidated, key.Key)
		}
	}
	return invalidated, nil
}

func (r *desktopRepository) UpdateTargetConfig(ctx context.Context, publicID string, target *service.DesktopTargetConfig) (*service.DesktopOrganization, error) {
	raw, err := target.CanonicalJSON()
	if err != nil {
		return nil, err
	}
	_, err = r.client.DesktopOrganization.Update().Where(desktoporganization.PublicIDEQ(publicID)).SetTargetConfig(raw).Save(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	return r.GetOrganization(ctx, publicID)
}

func (r *desktopRepository) CreateMember(ctx context.Context, organizationPublicID string, input service.DesktopCreateMemberInput) (*service.DesktopMember, error) {
	organization, err := r.client.DesktopOrganization.Query().Where(desktoporganization.PublicIDEQ(organizationPublicID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		gatewayUser, groupEntity, lockedOrganization, err := lockDesktopCarrier(txCtx, client, organization.GatewayUserID, organization.GroupID, organization.ID)
		if err != nil {
			return err
		}
		if lockedOrganization.GatewayUserID != organization.GatewayUserID || lockedOrganization.GroupID != organization.GroupID {
			return service.ErrDesktopRotationConflict
		}
		if lockedOrganization.Status != service.DesktopStatusActive {
			return service.ErrDesktopOrganizationDisabled
		}
		if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
			return err
		}
		member, err := client.DesktopMember.Create().
			SetPublicID(input.Member.PublicID).
			SetOrganizationID(lockedOrganization.ID).
			SetName(input.Member.Name).
			SetNameNormalized(input.Member.NameNormalized).
			SetPhone(input.Member.Phone).
			Save(txCtx)
		if err != nil {
			return translatePersistenceError(err, nil, service.ErrDesktopValidation)
		}
		input.APIKey.UserID = lockedOrganization.GatewayUserID
		input.APIKey.GroupID = &lockedOrganization.GroupID
		if err := r.createAPIKeyWithinLimit(txCtx, input.APIKey); err != nil {
			return err
		}
		_, err = client.DesktopMemberAPIKey.Create().SetMemberID(member.ID).SetAPIKeyID(input.APIKey.ID).Save(txCtx)
		return translatePersistenceError(err, nil, service.ErrDesktopRotationConflict)
	})
	if err != nil {
		return nil, err
	}
	return r.GetMember(ctx, organizationPublicID, input.Member.PublicID)
}

func (r *desktopRepository) ListMembers(ctx context.Context, organizationPublicID string, params pagination.PaginationParams, filters service.DesktopMemberListFilters) ([]service.DesktopMember, *pagination.PaginationResult, error) {
	organization, err := r.client.DesktopOrganization.Query().Where(desktoporganization.PublicIDEQ(organizationPublicID)).Only(ctx)
	if err != nil {
		return nil, nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	q := r.client.DesktopMember.Query().Where(desktopmember.OrganizationIDEQ(organization.ID))
	if filters.Phone != "" {
		q = q.Where(desktopmember.PhoneEQ(filters.Phone))
	} else if filters.Search != "" {
		q = q.Where(desktopmember.NameContainsFold(filters.Search))
	}
	if filters.Status != "" {
		q = q.Where(desktopmember.StatusEQ(filters.Status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.WithAPIKeyAssignments(func(aq *dbent.DesktopMemberAPIKeyQuery) {
		aq.Where(desktopmemberapikey.RetiredAtIsNil()).WithAPIKey()
	}).Order(dbent.Desc(desktopmember.FieldUpdatedAt), dbent.Desc(desktopmember.FieldID)).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]service.DesktopMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, *desktopMemberEntityToService(row))
	}
	return result, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopRepository) GetMember(ctx context.Context, organizationPublicID, memberPublicID string) (*service.DesktopMember, error) {
	row, err := r.client.DesktopMember.Query().Where(
		desktopmember.PublicIDEQ(memberPublicID),
		desktopmember.HasOrganizationWith(desktoporganization.PublicIDEQ(organizationPublicID)),
	).WithAPIKeyAssignments(func(q *dbent.DesktopMemberAPIKeyQuery) {
		q.Where(desktopmemberapikey.RetiredAtIsNil()).WithAPIKey()
	}).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
	}
	return desktopMemberEntityToService(row), nil
}

func (r *desktopRepository) UpdateMember(ctx context.Context, organizationPublicID, memberPublicID string, input service.DesktopUpdateMemberInput) (*service.DesktopMember, []string, error) {
	organization, member, err := r.loadOrganizationAndMember(ctx, organizationPublicID, memberPublicID)
	if err != nil {
		return nil, nil, err
	}
	var invalidated []string
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		gatewayUser, groupEntity, lockedOrganization, err := lockDesktopCarrier(txCtx, client, organization.GatewayUserID, organization.GroupID, organization.ID)
		if err != nil {
			return err
		}
		lockedMember, err := client.DesktopMember.Query().Where(desktopmember.IDEQ(member.ID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
		}
		assignment, key, err := lockCurrentDesktopAPIKey(txCtx, client, lockedMember.ID)
		if err != nil {
			return err
		}
		_ = assignment
		builder := client.DesktopMember.UpdateOne(lockedMember)
		changed := false
		if input.Name != nil {
			builder.SetName(*input.Name).SetNameNormalized(*input.NameNormalized)
			changed = true
		}
		if input.Phone != nil {
			builder.SetPhone(*input.Phone)
			changed = true
		}
		if input.Status != nil && *input.Status != lockedMember.Status {
			if *input.Status == service.DesktopStatusActive {
				if lockedOrganization.Status != service.DesktopStatusActive {
					return service.ErrDesktopOrganizationDisabled
				}
				if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
					return err
				}
				if key != nil {
					if _, err := client.APIKey.UpdateOne(key).SetStatus(service.StatusAPIKeyActive).Save(txCtx); err != nil {
						return err
					}
					invalidated = append(invalidated, key.Key)
				}
			} else if key != nil {
				if _, err := client.APIKey.UpdateOne(key).SetStatus(service.StatusAPIKeyDisabled).Save(txCtx); err != nil {
					return err
				}
				invalidated = append(invalidated, key.Key)
			}
			builder.SetStatus(*input.Status).SetAPIKeySuspendedByOrganization(false)
			changed = true
		}
		if changed || input.RevokeCredential {
			builder.AddAuthVersion(1)
		}
		_, err = builder.Save(txCtx)
		return translatePersistenceError(err, nil, service.ErrDesktopValidation)
	})
	if err != nil {
		return nil, nil, err
	}
	updated, err := r.GetMember(ctx, organizationPublicID, memberPublicID)
	return updated, invalidated, err
}

func (r *desktopRepository) DeleteMember(ctx context.Context, organizationPublicID, memberPublicID string) ([]string, error) {
	organization, member, err := r.loadOrganizationAndMember(ctx, organizationPublicID, memberPublicID)
	if err != nil {
		return nil, err
	}
	var invalidated []string
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		_, _, _, err := lockDesktopCarrier(txCtx, client, organization.GatewayUserID, organization.GroupID, organization.ID)
		if err != nil {
			return err
		}
		lockedMember, err := client.DesktopMember.Query().Where(desktopmember.IDEQ(member.ID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
		}
		assignment, key, err := lockCurrentDesktopAPIKey(txCtx, client, lockedMember.ID)
		if err != nil {
			return err
		}
		if key != nil {
			invalidated = append(invalidated, key.Key)
			if err := r.apiKeys.DeleteWithAudit(txCtx, key.ID); err != nil {
				return err
			}
		}
		if assignment != nil {
			if _, err := client.DesktopMemberAPIKey.UpdateOne(assignment).SetRetiredAt(time.Now()).Save(txCtx); err != nil {
				return err
			}
		}
		_, err = client.DesktopMember.UpdateOne(lockedMember).
			SetStatus(service.DesktopStatusDisabled).
			SetAPIKeySuspendedByOrganization(false).
			AddAuthVersion(1).Save(txCtx)
		if err != nil {
			return err
		}
		return client.DesktopMember.DeleteOne(lockedMember).Exec(txCtx)
	})
	return invalidated, err
}

func (r *desktopRepository) RotateMemberAPIKey(ctx context.Context, organizationPublicID, memberPublicID string, newKey *service.APIKey) (*service.DesktopMember, []string, error) {
	organization, member, err := r.loadOrganizationAndMember(ctx, organizationPublicID, memberPublicID)
	if err != nil {
		return nil, nil, err
	}
	var invalidated []string
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		gatewayUser, groupEntity, lockedOrganization, err := lockDesktopCarrier(txCtx, client, organization.GatewayUserID, organization.GroupID, organization.ID)
		if err != nil {
			return err
		}
		if lockedOrganization.Status != service.DesktopStatusActive {
			return service.ErrDesktopOrganizationDisabled
		}
		if err := validateDesktopCarrier(txCtx, client, gatewayUser, groupEntity); err != nil {
			return err
		}
		lockedMember, err := client.DesktopMember.Query().Where(desktopmember.IDEQ(member.ID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
		}
		if lockedMember.Status != service.DesktopStatusActive {
			return service.ErrDesktopMemberDisabled
		}
		assignment, oldKey, err := lockCurrentDesktopAPIKey(txCtx, client, lockedMember.ID)
		if err != nil {
			return err
		}
		if assignment != nil {
			if _, err := client.DesktopMemberAPIKey.UpdateOne(assignment).SetRetiredAt(time.Now()).Save(txCtx); err != nil {
				return err
			}
		}
		if oldKey != nil {
			invalidated = append(invalidated, oldKey.Key)
			if err := r.apiKeys.DeleteWithAudit(txCtx, oldKey.ID); err != nil {
				return err
			}
		}
		newKey.UserID = lockedOrganization.GatewayUserID
		newKey.GroupID = &lockedOrganization.GroupID
		if err := r.createAPIKeyWithinLimit(txCtx, newKey); err != nil {
			return err
		}
		if _, err := client.DesktopMemberAPIKey.Create().SetMemberID(lockedMember.ID).SetAPIKeyID(newKey.ID).Save(txCtx); err != nil {
			return translatePersistenceError(err, nil, service.ErrDesktopRotationConflict)
		}
		_, err = client.DesktopMember.UpdateOne(lockedMember).SetAPIKeySuspendedByOrganization(false).Save(txCtx)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	updated, err := r.GetMember(ctx, organizationPublicID, memberPublicID)
	return updated, invalidated, err
}

func (r *desktopRepository) FindActiveOrganizationByCode(ctx context.Context, code string) (*service.DesktopOrganization, error) {
	row, err := r.client.DesktopOrganization.Query().Where(
		desktoporganization.CodeEQ(code),
		desktoporganization.StatusEQ(service.DesktopStatusActive),
	).WithGatewayUser().WithGroup().WithMembers().Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	return desktopOrganizationEntityToService(row)
}

func (r *desktopRepository) FindMemberByPhone(ctx context.Context, organizationID int64, phone string) (*service.DesktopMember, error) {
	row, err := r.client.DesktopMember.Query().Where(
		desktopmember.OrganizationIDEQ(organizationID), desktopmember.PhoneEQ(phone),
	).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
	}
	return desktopMemberEntityToService(row), nil
}

func (r *desktopRepository) GetAuthorizedMember(ctx context.Context, memberPublicID string) (*service.DesktopAuthorizedMember, error) {
	memberEntity, err := r.client.DesktopMember.Query().Where(desktopmember.PublicIDEQ(memberPublicID)).
		WithAPIKeyAssignments(func(q *dbent.DesktopMemberAPIKeyQuery) {
			q.Where(desktopmemberapikey.RetiredAtIsNil()).WithAPIKey()
		}).
		WithOrganization(func(q *dbent.DesktopOrganizationQuery) {
			q.WithGatewayUser(func(userQuery *dbent.UserQuery) { userQuery.WithAllowedGroups() }).WithGroup()
		}).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopMembershipRevoked, nil)
	}
	organizationEntity := memberEntity.Edges.Organization
	if organizationEntity == nil || organizationEntity.Edges.GatewayUser == nil {
		return nil, service.ErrDesktopMembershipRevoked
	}
	member := desktopMemberEntityToService(memberEntity)
	organization, err := desktopOrganizationEntityToService(organizationEntity)
	if err != nil {
		return nil, err
	}
	gatewayUser := userEntityToService(organizationEntity.Edges.GatewayUser)
	return &service.DesktopAuthorizedMember{Member: member, Organization: organization, GatewayUser: gatewayUser}, nil
}

func (r *desktopRepository) ListMemberAPIKeyIDs(ctx context.Context, memberID int64) ([]int64, error) {
	values, err := r.client.DesktopMemberAPIKey.Query().Where(desktopmemberapikey.MemberIDEQ(memberID)).Select(desktopmemberapikey.FieldAPIKeyID).Ints(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]int64, len(values))
	for i, value := range values {
		result[i] = int64(value)
	}
	return result, nil
}

func (r *desktopRepository) IsManagedAPIKey(ctx context.Context, apiKeyID int64) (bool, error) {
	return r.client.DesktopMemberAPIKey.Query().Where(desktopmemberapikey.APIKeyIDEQ(apiKeyID)).Exist(ctx)
}

func (r *desktopRepository) HasOrganizationForUser(ctx context.Context, userID int64) (bool, error) {
	return r.client.DesktopOrganization.Query().Where(desktoporganization.GatewayUserIDEQ(userID)).Exist(ctx)
}

func (r *desktopRepository) HasOrganizationForGroup(ctx context.Context, groupID int64) (bool, error) {
	return r.client.DesktopOrganization.Query().Where(desktoporganization.GroupIDEQ(groupID)).Exist(ctx)
}

func (r *desktopRepository) DesktopOrganizationGroupIDForUser(ctx context.Context, userID int64) (int64, bool, error) {
	row, err := r.client.DesktopOrganization.Query().
		Where(desktoporganization.GatewayUserIDEQ(userID)).
		Select(desktoporganization.FieldGroupID).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return row.GroupID, true, nil
}

func (r *desktopRepository) ListAvailableGatewayUsers(ctx context.Context, params pagination.PaginationParams, search string) ([]service.User, *pagination.PaginationResult, error) {
	q := r.client.User.Query().Where(user.StatusEQ(domain.StatusActive), user.Not(user.HasDesktopOrganizations()))
	if search != "" {
		q = q.Where(user.Or(user.EmailContainsFold(search), user.UsernameContainsFold(search)))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.Order(dbent.Asc(user.FieldEmail)).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]service.User, 0, len(rows))
	for _, row := range rows {
		result = append(result, *userEntityToService(row))
	}
	return result, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopRepository) loadOrganizationAndMember(ctx context.Context, organizationPublicID, memberPublicID string) (*dbent.DesktopOrganization, *dbent.DesktopMember, error) {
	organization, err := r.client.DesktopOrganization.Query().Where(desktoporganization.PublicIDEQ(organizationPublicID)).Only(ctx)
	if err != nil {
		return nil, nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	member, err := r.client.DesktopMember.Query().Where(desktopmember.PublicIDEQ(memberPublicID), desktopmember.OrganizationIDEQ(organization.ID)).Only(ctx)
	if err != nil {
		return nil, nil, translatePersistenceError(err, service.ErrDesktopMemberNotFound, nil)
	}
	return organization, member, nil
}

func lockDesktopCarrier(ctx context.Context, client *dbent.Client, userID, groupID, organizationID int64) (*dbent.User, *dbent.Group, *dbent.DesktopOrganization, error) {
	gatewayUser, err := client.User.Query().Where(user.IDEQ(userID)).WithAllowedGroups().ForUpdate().Only(ctx)
	if err != nil {
		return nil, nil, nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	groupEntity, err := client.Group.Query().Where(group.IDEQ(groupID)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, nil, nil, translatePersistenceError(err, service.ErrGroupNotFound, nil)
	}
	organization, err := client.DesktopOrganization.Query().Where(desktoporganization.IDEQ(organizationID)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, nil, nil, translatePersistenceError(err, service.ErrDesktopOrganizationNotFound, nil)
	}
	return gatewayUser, groupEntity, organization, nil
}

func validateDesktopCarrier(ctx context.Context, client *dbent.Client, gatewayUser *dbent.User, groupEntity *dbent.Group) error {
	if gatewayUser.Status != domain.StatusActive {
		return service.ErrDesktopMembershipRevoked
	}
	if groupEntity.Status != domain.StatusActive {
		return service.ErrGroupNotAllowed
	}
	allowedGroups := make([]int64, 0, len(gatewayUser.Edges.AllowedGroups))
	for _, allowed := range gatewayUser.Edges.AllowedGroups {
		allowedGroups = append(allowedGroups, allowed.ID)
	}
	carrier := &service.User{
		ID:                   gatewayUser.ID,
		Status:               gatewayUser.Status,
		AllowedGroups:        allowedGroups,
		RestrictPublicGroups: gatewayUser.RestrictPublicGroups,
	}
	if !carrier.CanBindGroup(groupEntity.ID, groupEntity.IsExclusive) {
		return service.ErrGroupNotAllowed
	}
	if groupEntity.SubscriptionType == domain.SubscriptionTypeSubscription {
		now := time.Now()
		hasSubscription, err := client.UserSubscription.Query().Where(
			usersubscription.UserIDEQ(gatewayUser.ID),
			usersubscription.GroupIDEQ(groupEntity.ID),
			usersubscription.StatusEQ(domain.SubscriptionStatusActive),
			usersubscription.StartsAtLTE(now),
			usersubscription.ExpiresAtGT(now),
		).Exist(ctx)
		if err != nil {
			return err
		}
		if !hasSubscription {
			return service.ErrGroupNotAllowed
		}
	}
	return nil
}

func lockCurrentDesktopAPIKey(ctx context.Context, client *dbent.Client, memberID int64) (*dbent.DesktopMemberAPIKey, *dbent.APIKey, error) {
	assignment, err := client.DesktopMemberAPIKey.Query().Where(
		desktopmemberapikey.MemberIDEQ(memberID), desktopmemberapikey.RetiredAtIsNil(),
	).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	key, err := client.APIKey.Query().Where(apikey.IDEQ(assignment.APIKeyID)).ForUpdate().Only(mixins.SkipSoftDelete(ctx))
	if dbent.IsNotFound(err) {
		return assignment, nil, nil
	}
	return assignment, key, err
}

func desktopOrganizationEntityToService(row *dbent.DesktopOrganization) (*service.DesktopOrganization, error) {
	var target *service.DesktopTargetConfig
	if len(row.TargetConfig) > 0 {
		decoded, err := service.DecodeDesktopTargetConfig(row.TargetConfig)
		if err != nil {
			return nil, err
		}
		target = decoded
	}
	result := &service.DesktopOrganization{
		ID: row.ID, PublicID: row.PublicID, Code: row.Code, Name: row.Name, Status: row.Status,
		AuthVersion: row.AuthVersion, GatewayUserID: row.GatewayUserID, GroupID: row.GroupID,
		TargetConfig: target, TargetConfigAssigned: target != nil, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		MemberCount: len(row.Edges.Members),
	}
	if row.Edges.GatewayUser != nil {
		result.GatewayUserEmail = row.Edges.GatewayUser.Email
		result.GatewayUserName = row.Edges.GatewayUser.Username
	}
	if row.Edges.Group != nil {
		result.GroupName = row.Edges.Group.Name
	}
	return result, nil
}

func desktopMemberEntityToService(row *dbent.DesktopMember) *service.DesktopMember {
	result := &service.DesktopMember{
		ID: row.ID, PublicID: row.PublicID, OrganizationID: row.OrganizationID, Name: row.Name,
		NameNormalized: row.NameNormalized, Phone: row.Phone, Status: row.Status, AuthVersion: row.AuthVersion,
		APIKeySuspendedByOrganization: row.APIKeySuspendedByOrganization, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	for _, assignment := range row.Edges.APIKeyAssignments {
		if assignment.RetiredAt != nil {
			continue
		}
		result.CurrentAPIKeyID = &assignment.APIKeyID
		if assignment.Edges.APIKey != nil {
			result.CurrentAPIKey = assignment.Edges.APIKey.Key
			result.CurrentAPIKeyStatus = assignment.Edges.APIKey.Status
		} else {
			result.CurrentAPIKeyDeleted = true
		}
		break
	}
	return result
}
