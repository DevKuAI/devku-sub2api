package repository

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/desktopupdaterelease"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

const desktopUpdatePublicationLockID int64 = 0x445550445445

type desktopUpdateRepository struct {
	client    *dbent.Client
	publishMu sync.Mutex
}

func NewDesktopUpdateRepository(client *dbent.Client) service.DesktopUpdateRepository {
	return &desktopUpdateRepository{client: client}
}

func (r *desktopUpdateRepository) Create(ctx context.Context, release *service.DesktopUpdateRelease) error {
	raw, err := json.Marshal(release.Artifacts)
	if err != nil {
		return err
	}
	builder := clientFromContext(ctx, r.client).DesktopUpdateRelease.Create().
		SetPublicID(release.PublicID).
		SetVersion(release.Version).
		SetNotes(release.Notes).
		SetArtifacts(raw).
		SetStatus(release.Status).
		SetNillableCreatedBy(release.CreatedBy).
		SetNillableUpdatedBy(release.UpdatedBy)
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrDesktopUpdateVersionExists)
	}
	applyDesktopUpdateEntity(release, created)
	return nil
}

func (r *desktopUpdateRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.DesktopUpdateListFilters) ([]service.DesktopUpdateRelease, *pagination.PaginationResult, error) {
	q := r.client.DesktopUpdateRelease.Query()
	if filters.Status != "" {
		q = q.Where(desktopupdaterelease.StatusEQ(filters.Status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.Order(dbent.Desc(desktopupdaterelease.FieldCreatedAt), dbent.Desc(desktopupdaterelease.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := desktopUpdateEntities(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopUpdateRepository) Get(ctx context.Context, publicID string) (*service.DesktopUpdateRelease, error) {
	row, err := clientFromContext(ctx, r.client).DesktopUpdateRelease.Query().
		Where(desktopupdaterelease.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrDesktopUpdateNotFound, nil)
	}
	return desktopUpdateEntity(row)
}

func (r *desktopUpdateRepository) UpdateDraft(ctx context.Context, publicID string, input service.DesktopUpdateDraftInput) (*service.DesktopUpdateRelease, error) {
	raw, err := json.Marshal(input.Artifacts)
	if err != nil {
		return nil, err
	}
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		query := client.DesktopUpdateRelease.Query().Where(desktopupdaterelease.PublicIDEQ(publicID))
		row, queryErr := desktopUpdateForUpdate(client, query).Only(txCtx)
		if queryErr != nil {
			return translatePersistenceError(queryErr, service.ErrDesktopUpdateNotFound, nil)
		}
		if row.Status != service.DesktopUpdateStatusDraft {
			return service.ErrDesktopUpdateInvalidState
		}
		builder := client.DesktopUpdateRelease.UpdateOne(row).
			SetVersion(input.Version).
			SetNotes(input.Notes).
			SetArtifacts(raw)
		if input.ActorID > 0 {
			builder.SetUpdatedBy(input.ActorID)
		}
		_, saveErr := builder.Save(txCtx)
		return translatePersistenceError(saveErr, service.ErrDesktopUpdateNotFound, service.ErrDesktopUpdateVersionExists)
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, publicID)
}

func (r *desktopUpdateRepository) Publish(ctx context.Context, publicID string, actorID int64, publishedAt time.Time) (*service.DesktopUpdateRelease, error) {
	r.publishMu.Lock()
	defer r.publishMu.Unlock()

	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if err := lockDesktopUpdatePublication(txCtx, client); err != nil {
			return err
		}
		query := client.DesktopUpdateRelease.Query().Where(desktopupdaterelease.PublicIDEQ(publicID))
		candidate, err := desktopUpdateForUpdate(client, query).Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopUpdateNotFound, nil)
		}
		if candidate.Status != service.DesktopUpdateStatusDraft {
			return service.ErrDesktopUpdateInvalidState
		}
		candidateModel, err := desktopUpdateEntity(candidate)
		if err != nil {
			return err
		}
		if _, err := service.NormalizeDesktopUpdateVersion(candidateModel.Version); err != nil {
			return service.ErrDesktopUpdateMetadata.WithCause(err)
		}
		if _, err := service.NormalizeDesktopUpdateArtifacts(candidateModel.Artifacts); err != nil {
			return service.ErrDesktopUpdateMetadata.WithCause(err)
		}
		publishedQuery := client.DesktopUpdateRelease.Query().
			Where(desktopupdaterelease.StatusEQ(service.DesktopUpdateStatusPublished))
		published, err := desktopUpdateForUpdate(client, publishedQuery).All(txCtx)
		if err != nil {
			return err
		}
		for _, existing := range published {
			comparison, compareErr := service.CompareDesktopUpdateVersions(candidate.Version, existing.Version)
			if compareErr != nil {
				return service.ErrDesktopUpdateMetadata.WithCause(compareErr)
			}
			if comparison <= 0 {
				return service.ErrDesktopUpdateNotNewer
			}
		}
		builder := client.DesktopUpdateRelease.UpdateOne(candidate).
			SetStatus(service.DesktopUpdateStatusPublished).
			SetPublishedAt(publishedAt)
		if actorID > 0 {
			builder.SetPublishedBy(actorID).SetUpdatedBy(actorID)
		}
		_, err = builder.Save(txCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, publicID)
}

func (r *desktopUpdateRepository) Withdraw(ctx context.Context, publicID string, actorID int64, reason string, withdrawnAt time.Time) (*service.DesktopUpdateRelease, error) {
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		query := client.DesktopUpdateRelease.Query().Where(desktopupdaterelease.PublicIDEQ(publicID))
		row, err := desktopUpdateForUpdate(client, query).Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrDesktopUpdateNotFound, nil)
		}
		if row.Status != service.DesktopUpdateStatusPublished {
			return service.ErrDesktopUpdateInvalidState
		}
		builder := client.DesktopUpdateRelease.UpdateOne(row).
			SetStatus(service.DesktopUpdateStatusWithdrawn).
			SetWithdrawnAt(withdrawnAt).
			SetWithdrawalReason(reason)
		if actorID > 0 {
			builder.SetWithdrawnBy(actorID).SetUpdatedBy(actorID)
		}
		_, err = builder.Save(txCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, publicID)
}

func (r *desktopUpdateRepository) ListPublished(ctx context.Context) ([]service.DesktopUpdateRelease, error) {
	rows, err := r.client.DesktopUpdateRelease.Query().
		Where(desktopupdaterelease.StatusEQ(service.DesktopUpdateStatusPublished)).All(ctx)
	if err != nil {
		return nil, err
	}
	return desktopUpdateEntities(rows)
}

func (r *desktopUpdateRepository) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
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

func lockDesktopUpdatePublication(ctx context.Context, client *dbent.Client) error {
	if client == nil || client.Driver().Dialect() != dialect.Postgres {
		return nil
	}
	var rows entsql.Rows
	if err := client.Driver().Query(ctx, "SELECT pg_advisory_xact_lock($1)", []any{desktopUpdatePublicationLockID}, &rows); err != nil {
		return err
	}
	return rows.Close()
}

func desktopUpdateForUpdate(client *dbent.Client, query *dbent.DesktopUpdateReleaseQuery) *dbent.DesktopUpdateReleaseQuery {
	if client == nil || client.Driver().Dialect() == dialect.SQLite {
		return query
	}
	return query.ForUpdate()
}

func desktopUpdateEntities(rows []*dbent.DesktopUpdateRelease) ([]service.DesktopUpdateRelease, error) {
	items := make([]service.DesktopUpdateRelease, 0, len(rows))
	for _, row := range rows {
		item, err := desktopUpdateEntity(row)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

func desktopUpdateEntity(row *dbent.DesktopUpdateRelease) (*service.DesktopUpdateRelease, error) {
	var artifacts service.DesktopUpdateArtifacts
	if err := json.Unmarshal(row.Artifacts, &artifacts); err != nil {
		return nil, service.ErrDesktopUpdateMetadata.WithCause(err)
	}
	return &service.DesktopUpdateRelease{
		ID: row.ID, PublicID: row.PublicID, Version: row.Version, Notes: row.Notes,
		Artifacts: artifacts, Status: row.Status, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy,
		PublishedBy: row.PublishedBy, WithdrawnBy: row.WithdrawnBy,
		PublishedAt: row.PublishedAt, WithdrawnAt: row.WithdrawnAt, WithdrawalReason: row.WithdrawalReason,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func applyDesktopUpdateEntity(target *service.DesktopUpdateRelease, row *dbent.DesktopUpdateRelease) {
	if target == nil || row == nil {
		return
	}
	target.ID, target.PublicID, target.Version, target.Notes = row.ID, row.PublicID, row.Version, row.Notes
	target.Status, target.CreatedBy, target.UpdatedBy = row.Status, row.CreatedBy, row.UpdatedBy
	target.PublishedBy, target.WithdrawnBy = row.PublishedBy, row.WithdrawnBy
	target.PublishedAt, target.WithdrawnAt, target.WithdrawalReason = row.PublishedAt, row.WithdrawnAt, row.WithdrawalReason
	target.CreatedAt, target.UpdatedAt = row.CreatedAt, row.UpdatedAt
}
