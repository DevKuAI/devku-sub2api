package repository

import (
	"context"
	"encoding/json"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/desktopconversationrecord"
	"github.com/Wei-Shaw/sub2api/ent/desktopmember"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type desktopConversationRepository struct{ client *dbent.Client }

func NewDesktopConversationRepository(client *dbent.Client) service.DesktopConversationRepository {
	return &desktopConversationRepository{client: client}
}

func (r *desktopConversationRepository) Create(ctx context.Context, organizationID, memberID int64, input *service.DesktopConversationInput) (*service.DesktopConversationReceipt, error) {
	prompts, err := json.Marshal(input.Prompts)
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	defer func() { _ = tx.Rollback() }()
	builder := tx.DesktopConversationRecord.Create().
		SetRecordID(uuid.MustParse(input.RecordID)).SetOrganizationID(organizationID).SetMemberID(memberID).
		SetInstallationID(uuid.MustParse(input.InstallationID)).SetClient(input.Client).
		SetSourceSessionID(input.SessionID).SetNillableSourceTurnID(input.SourceTurnID).
		SetStartedAt(input.StartedAt).SetStoppedAt(input.StoppedAt).SetNillableCwd(input.CWD).
		SetPrompts(prompts).SetPromptCount(len(input.Prompts)).SetCaptureStatus(input.CaptureStatus)
	if input.Response != nil {
		body, marshalErr := json.Marshal(input.Response)
		if marshalErr != nil {
			return nil, service.ErrDesktopConversationStorage
		}
		builder.SetResponse(body)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, service.ErrDesktopConversationExists
		}
		return nil, service.ErrDesktopConversationStorage
	}
	if err := tx.Commit(); err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	return &service.DesktopConversationReceipt{RecordID: row.RecordID.String(), ReceivedAt: row.ReceivedAt.UTC()}, nil
}

func (r *desktopConversationRepository) query(organizationID int64, filters service.DesktopConversationFilters) *dbent.DesktopConversationRecordQuery {
	q := r.client.DesktopConversationRecord.Query().Where(desktopconversationrecord.OrganizationIDEQ(organizationID))
	if filters.MemberID != "" {
		q.Where(desktopconversationrecord.HasMemberWith(desktopmember.PublicIDEQ(filters.MemberID)))
	}
	if filters.MemberSearch != "" {
		q.Where(desktopconversationrecord.HasMemberWith(desktopmember.Or(desktopmember.NameContainsFold(filters.MemberSearch), desktopmember.PublicIDContainsFold(filters.MemberSearch))))
	}
	if filters.Client != "" {
		q.Where(desktopconversationrecord.ClientEQ(filters.Client))
	}
	if filters.CaptureStatus != "" {
		q.Where(desktopconversationrecord.CaptureStatusEQ(filters.CaptureStatus))
	}
	if filters.RecordID != "" {
		q.Where(desktopconversationrecord.RecordIDEQ(uuid.MustParse(filters.RecordID)))
	}
	if filters.SourceSessionID != "" {
		q.Where(desktopconversationrecord.SourceSessionIDEQ(filters.SourceSessionID))
	}
	if filters.InstallationID != "" {
		q.Where(desktopconversationrecord.InstallationIDEQ(uuid.MustParse(filters.InstallationID)))
	}
	if filters.ReceivedFrom != nil {
		q.Where(desktopconversationrecord.ReceivedAtGTE(*filters.ReceivedFrom))
	}
	if filters.ReceivedTo != nil {
		q.Where(desktopconversationrecord.ReceivedAtLT(*filters.ReceivedTo))
	}
	return q
}

func (r *desktopConversationRepository) List(ctx context.Context, organizationID int64, params pagination.PaginationParams, filters service.DesktopConversationFilters) ([]service.DesktopConversationMetadata, *pagination.PaginationResult, error) {
	// Organization authorization is completed before opting into historical members.
	ctx = mixins.SkipSoftDelete(ctx)
	q := r.query(organizationID, filters)
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, service.ErrDesktopConversationStorage
	}
	columns := make([]string, 0, len(desktopconversationrecord.Columns)-2)
	for _, column := range desktopconversationrecord.Columns {
		if column != desktopconversationrecord.FieldPrompts && column != desktopconversationrecord.FieldResponse {
			columns = append(columns, column)
		}
	}
	order := dbent.Desc
	if params.SortOrder == pagination.SortOrderAsc {
		order = dbent.Asc
	}
	rows, err := q.Select(columns...).WithMember().
		Order(order(desktopconversationrecord.FieldReceivedAt), order(desktopconversationrecord.FieldID)).
		Offset(params.Offset()).Limit(params.PageSize).All(ctx)
	if err != nil {
		return nil, nil, service.ErrDesktopConversationStorage
	}
	items := make([]service.DesktopConversationMetadata, 0, len(rows))
	for _, row := range rows {
		items = append(items, desktopConversationMetadata(row))
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopConversationRepository) Get(ctx context.Context, organizationID int64, recordID string) (*service.DesktopConversationDetail, error) {
	row, err := r.client.DesktopConversationRecord.Query().Where(
		desktopconversationrecord.OrganizationIDEQ(organizationID), desktopconversationrecord.RecordIDEQ(uuid.MustParse(recordID)),
	).WithMember().Only(mixins.SkipSoftDelete(ctx))
	if dbent.IsNotFound(err) {
		return nil, service.ErrDesktopConversationNotFound
	}
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	detail := &service.DesktopConversationDetail{DesktopConversationMetadata: desktopConversationMetadata(row), SchemaVersion: 2}
	if json.Unmarshal(row.Prompts, &detail.Prompts) != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	if len(row.Response) > 0 && json.Unmarshal(row.Response, &detail.Response) != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	return detail, nil
}

func desktopConversationMetadata(row *dbent.DesktopConversationRecord) service.DesktopConversationMetadata {
	result := service.DesktopConversationMetadata{
		RecordID: row.RecordID.String(), InstallationID: row.InstallationID.String(), Client: row.Client,
		SourceSessionID: row.SourceSessionID, SourceTurnID: row.SourceTurnID, StartedAt: row.StartedAt.UTC(),
		StoppedAt: row.StoppedAt.UTC(), ReceivedAt: row.ReceivedAt.UTC(), CWD: row.Cwd,
		CaptureStatus: row.CaptureStatus, PromptCount: row.PromptCount,
	}
	if member := row.Edges.Member; member != nil {
		result.MemberID, result.MemberName, result.MemberDeleted = member.PublicID, member.Name, member.DeletedAt != nil
	}
	return result
}
