package accounting

import (
	"context"
	"database/sql"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type OrganizationContact struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Role           string    `json:"role"`
	Phone          string    `json:"phone"`
	Email          string    `json:"email"`
	WeChat         string    `json:"wechat"`
	Remark         string    `json:"remark"`
	IsPrimary      bool      `json:"is_primary"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Org            int64     `json:"-"`
}

type OrganizationContactInput struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	WeChat         string `json:"wechat"`
	Remark         string `json:"remark"`
	IsPrimary      bool   `json:"is_primary"`
}

func normalizeContactInput(in OrganizationContactInput) (OrganizationContactInput, error) {
	in.OrganizationID = strings.TrimSpace(in.OrganizationID)
	in.Name = strings.TrimSpace(in.Name)
	in.Role = strings.ToUpper(strings.TrimSpace(in.Role))
	in.Phone = strings.TrimSpace(in.Phone)
	in.Email = strings.TrimSpace(in.Email)
	in.WeChat = strings.TrimSpace(in.WeChat)
	in.Remark = strings.TrimSpace(in.Remark)
	if in.Role == "" {
		in.Role = "BUSINESS"
	}
	roles := map[string]bool{"BUSINESS": true, "FINANCE": true, "LEGAL": true, "TECHNICAL": true, "OTHER": true}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		return in, invalid("name")
	}
	if !roles[in.Role] {
		return in, invalid("role")
	}
	if utf8.RuneCountInString(in.Phone) > 40 || utf8.RuneCountInString(in.WeChat) > 100 || utf8.RuneCountInString(in.Remark) > 1000 {
		return in, ErrInvalid
	}
	if in.Email != "" {
		address, err := mail.ParseAddress(in.Email)
		if err != nil || address.Address != in.Email || len(in.Email) > 254 {
			return in, invalid("email")
		}
	}
	return in, nil
}

func scanContact(row interface{ Scan(...any) error }) (*OrganizationContact, error) {
	contact := &OrganizationContact{}
	err := row.Scan(&contact.ID, &contact.OrganizationID, &contact.Name, &contact.Role, &contact.Phone, &contact.Email, &contact.WeChat, &contact.Remark, &contact.IsPrimary, &contact.CreatedAt, &contact.UpdatedAt, &contact.Org)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return contact, err
}

const contactColumns = "c.id::text,o.public_id,c.name,c.role,c.phone,c.email,c.wechat,c.remark,c.is_primary,c.created_at,c.updated_at,c.organization_id"

func getContact(ctx context.Context, q dbQuery, id string, org int64, lock bool) (*OrganizationContact, error) {
	query := "SELECT " + contactColumns + " FROM accounting_organization_contacts c JOIN desktop_organizations o ON o.id=c.organization_id WHERE c.id=$1 AND c.deleted_at IS NULL AND o.deleted_at IS NULL AND ($2::bigint=0 OR c.organization_id=$2)"
	if lock {
		query += " FOR UPDATE OF c"
	}
	return scanContact(q.QueryRowContext(ctx, query, id, org))
}

func (e *Engine) ListContacts(ctx context.Context, organizationID string) ([]*OrganizationContact, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT "+contactColumns+" FROM accounting_organization_contacts c JOIN desktop_organizations o ON o.id=c.organization_id WHERE o.public_id=$1 AND o.deleted_at IS NULL AND c.deleted_at IS NULL ORDER BY c.is_primary DESC,c.name,c.id", organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	contacts := []*OrganizationContact{}
	for rows.Next() {
		contact, scanErr := scanContact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func createContact(ctx context.Context, tx *sql.Tx, org, actor int64, publicID string, in OrganizationContactInput) (*OrganizationContact, error) {
	in, err := normalizeContactInput(in)
	if err != nil {
		return nil, err
	}
	if in.IsPrimary {
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_organization_contacts SET is_primary=FALSE,updated_at=NOW() WHERE organization_id=$1 AND deleted_at IS NULL AND is_primary", org); err != nil {
			return nil, err
		}
	}
	id := uuid.NewString()
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_organization_contacts(id,organization_id,name,role,phone,email,wechat,remark,is_primary,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, org, in.Name, in.Role, in.Phone, in.Email, in.WeChat, in.Remark, in.IsPrimary, actor)
	if err != nil {
		return nil, err
	}
	return &OrganizationContact{ID: id, OrganizationID: publicID, Name: in.Name, Role: in.Role, Phone: in.Phone, Email: in.Email, WeChat: in.WeChat, Remark: in.Remark, IsPrimary: in.IsPrimary, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Org: org}, nil
}

func updateContact(ctx context.Context, tx *sql.Tx, current *OrganizationContact, in OrganizationContactInput) (*OrganizationContact, error) {
	in, err := normalizeContactInput(in)
	if err != nil {
		return nil, err
	}
	if in.IsPrimary {
		if _, err = tx.ExecContext(ctx, "UPDATE accounting_organization_contacts SET is_primary=FALSE,updated_at=NOW() WHERE organization_id=$1 AND id<>$2 AND deleted_at IS NULL AND is_primary", current.Org, current.ID); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounting_organization_contacts SET name=$2,role=$3,phone=$4,email=$5,wechat=$6,remark=$7,is_primary=$8,updated_at=NOW() WHERE id=$1`, current.ID, in.Name, in.Role, in.Phone, in.Email, in.WeChat, in.Remark, in.IsPrimary)
	if err != nil {
		return nil, err
	}
	return getContact(ctx, tx, current.ID, current.Org, false)
}

func hydrateContactSnapshot(ctx context.Context, q dbQuery, r *Record) error {
	id := detail(r, "contact_id")
	if id == "" {
		return nil
	}
	if _, err := uuid.Parse(id); err != nil {
		return invalid("contact_id")
	}
	contact, err := getContact(ctx, q, id, r.Org, false)
	if err != nil {
		return err
	}
	r.Details["contact_name"] = contact.Name
	r.Details["contact_phone"] = contact.Phone
	r.Details["contact_email"] = contact.Email
	r.Details["contact_wechat"] = contact.WeChat
	return nil
}
