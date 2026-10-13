package accounting

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxAttachmentBytes int64 = 20 << 20

type Attachment struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organization_id"`
	ObjectKind      string     `json:"object_kind"`
	ObjectID        string     `json:"object_id"`
	Purpose         string     `json:"purpose"`
	Filename        string     `json:"filename"`
	ContentType     string     `json:"content_type"`
	Size            int64      `json:"size_bytes"`
	SHA256          string     `json:"sha256"`
	Status          string     `json:"status"`
	CustomerVisible bool       `json:"customer_visible"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	Org             int64      `json:"-"`
	Key             string     `json:"-"`
}

func attachment(ctx context.Context, q dbQuery, id string, org int64) (*Attachment, error) {
	return lookupAttachment(ctx, q, id, org, false)
}

func lookupAttachment(ctx context.Context, q dbQuery, id string, org int64, includeDeleted bool) (*Attachment, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	a := &Attachment{}
	err := q.QueryRowContext(ctx, `SELECT a.id::text,o.public_id,a.object_kind,a.object_id::text,a.purpose,a.filename,a.content_type,a.size_bytes,a.sha256,a.status,a.customer_visible,a.expires_at,a.created_at,a.deleted_at,a.organization_id,a.object_key
 FROM accounting_attachments a JOIN desktop_organizations o ON o.id=a.organization_id WHERE a.id=$1 AND ($2::bigint=0 OR a.organization_id=$2) AND ($3=true OR a.deleted_at IS NULL)`, id, org, includeDeleted).Scan(&a.ID, &a.OrganizationID, &a.ObjectKind, &a.ObjectID, &a.Purpose, &a.Filename, &a.ContentType, &a.Size, &a.SHA256, &a.Status, &a.CustomerVisible, &a.ExpiresAt, &a.CreatedAt, &a.DeletedAt, &a.Org, &a.Key)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}
func validAttachment(filename, mime string) bool {
	ext := strings.ToLower(path.Ext(filename))
	types := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".pdf": "application/pdf"}
	return types[ext] == mime && mime != ""
}
func (e *Engine) Intent(ctx context.Context, tx *sql.Tx, a Attachment, actor int64) (any, error) {
	if e.storage == nil || !e.storage.Ready() {
		return nil, ErrStorage
	}
	if a.Size <= 0 || a.Size > MaxAttachmentBytes || !validAttachment(a.Filename, a.ContentType) || len(a.Filename) > 255 || strings.ContainsAny(a.Filename, "/\\\r\n") || len(a.SHA256) != 64 {
		return nil, ErrInvalid
	}
	if _, err := hex.DecodeString(a.SHA256); err != nil {
		return nil, invalid("sha256")
	}
	a.SHA256 = strings.ToLower(a.SHA256)
	record, err := get(ctx, tx, a.ObjectKind, a.ObjectID, a.Org, true)
	if err != nil {
		return nil, err
	}
	if record.Status == "VOID" || (a.ObjectKind == "vendor-invoices" && record.Status != "DRAFT") {
		return nil, ErrState
	}
	if a.Purpose == "" || (a.ObjectKind == "vendor-invoices" && a.Purpose != "INVOICE") {
		return nil, invalid("purpose")
	}
	a.ID = uuid.NewString()
	a.Status = "PENDING"
	a.CustomerVisible = false
	a.ExpiresAt = time.Now().UTC().Add(10 * time.Minute)
	a.Key = path.Join("accounting", record.OrganizationID, a.ID, a.SHA256+strings.ToLower(path.Ext(a.Filename)))
	_, err = tx.ExecContext(ctx, "INSERT INTO accounting_attachments(id,organization_id,object_kind,object_id,purpose,filename,content_type,size_bytes,sha256,object_key,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", a.ID, a.Org, a.ObjectKind, a.ObjectID, a.Purpose, a.Filename, a.ContentType, a.Size, a.SHA256, a.Key, a.ExpiresAt, actor)
	return map[string]any{"attachment": a, "upload_url": "/api/v1/admin/accounting/attachments/" + a.ID + "/content"}, err
}
func (e *Engine) Upload(ctx context.Context, tx *sql.Tx, a *Attachment, body []byte) error {
	if a.Status != "PENDING" || time.Now().After(a.ExpiresAt) {
		return ErrState
	}
	if int64(len(body)) != a.Size || hash(body) != a.SHA256 || http.DetectContentType(body) != a.ContentType {
		return ErrInvalid
	}
	if a.ContentType == "application/pdf" && !bytes.Contains(body[max(0, len(body)-2048):], []byte("%%EOF")) {
		return ErrInvalid
	}
	if e.storage == nil || !e.storage.Ready() {
		return ErrStorage
	}
	if err := e.storage.Upload(ctx, a.Key, bytes.NewReader(body), a.Size, a.ContentType); err != nil {
		return ErrStorage
	}
	_, err := tx.ExecContext(ctx, "UPDATE accounting_attachments SET status='UPLOADED' WHERE id=$1 AND status='PENDING'", a.ID)
	a.Status = "UPLOADED"
	return err
}
func completeAttachment(ctx context.Context, tx *sql.Tx, a *Attachment) error {
	if a.Status != "UPLOADED" || time.Now().After(a.ExpiresAt) {
		return ErrState
	}
	_, err := tx.ExecContext(ctx, "UPDATE accounting_attachments SET status='COMPLETE' WHERE id=$1", a.ID)
	a.Status = "COMPLETE"
	return err
}
func (e *Engine) Download(ctx context.Context, a *Attachment, actor int64, request string, customer bool) (any, error) {
	if a.Status != "COMPLETE" {
		return nil, ErrNotFound
	}
	if customer {
		if !a.CustomerVisible {
			return nil, ErrNotFound
		}
		r, err := get(ctx, e.db, a.ObjectKind, a.ObjectID, a.Org, false)
		if err != nil {
			return nil, err
		}
		if a.ObjectKind == "contracts" || a.ObjectKind == "invoices" {
			if !r.CustomerVisible {
				return nil, ErrNotFound
			}
		}
		if a.ObjectKind == "invoices" && detail(r, "invoice_type") != "SALES" {
			return nil, ErrNotFound
		}
		if a.ObjectKind == "vcc-transactions" && detail(r, "transaction_type") != "CONSUME" && detail(r, "transaction_type") != "REFUND" {
			return nil, ErrNotFound
		}
		if a.ObjectKind != "contracts" && a.ObjectKind != "invoices" && a.ObjectKind != "transfers" && a.ObjectKind != "vcc-transactions" {
			return nil, ErrNotFound
		}
	}
	if e.storage == nil || !e.storage.Ready() {
		return nil, ErrStorage
	}
	url, err := e.storage.Presign(ctx, a.Key, a.Filename, a.ContentType, 5*time.Minute)
	if err != nil {
		return nil, ErrStorage
	}
	if err = audit(ctx, e.db, a.Org, "attachments", a.ID, "download", actor, request, map[string]any{"sha256": a.SHA256}); err != nil {
		return nil, err
	}
	return map[string]any{"url": url, "expires_at": time.Now().UTC().Add(5 * time.Minute), "expires_in": 300, "sha256": a.SHA256}, nil
}
func (e *Engine) Attachments(ctx context.Context, kind, id string, org int64, customer bool) ([]*Attachment, error) {
	record, err := get(ctx, e.db, kind, id, org, false)
	if err != nil {
		return nil, err
	}
	if customer && ((kind == "contracts" || kind == "invoices") && !record.CustomerVisible) {
		return nil, ErrNotFound
	}
	if customer && kind == "invoices" && detail(record, "invoice_type") != "SALES" {
		return nil, ErrNotFound
	}
	rows, err := e.db.QueryContext(ctx, "SELECT id::text FROM accounting_attachments WHERE organization_id=$1 AND object_kind=$2 AND object_id=$3 AND deleted_at IS NULL AND ($4=false OR (customer_visible AND status='COMPLETE')) ORDER BY created_at", org, kind, id, customer)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, value)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out := []*Attachment{}
	for _, value := range ids {
		a, err := attachment(ctx, e.db, value, org)
		if err != nil {
			return nil, err
		}
		if customer {
			a.Purpose = ""
			a.ExpiresAt = time.Time{}
		}
		out = append(out, a)
	}
	return out, nil
}
func readUpload(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, MaxAttachmentBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxAttachmentBytes {
		return nil, fmt.Errorf("%w", ErrInvalid)
	}
	return body, nil
}
