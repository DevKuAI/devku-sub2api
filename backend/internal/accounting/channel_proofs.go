package accounting

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// maxChannelProofs bounds the proofs kept for one platform reference.
const maxChannelProofs = 20

// ChannelProof is a file kept as evidence for a channel line, such as a bank transfer screenshot for a collection.
type ChannelProof struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type channelProofUpload struct {
	ID, Filename, ContentType, SHA256, ObjectKey string
	Size                                         int64
}

// AddChannelProof keeps a proof for the platform reference of an active line.
func (e *Engine) AddChannelProof(ctx context.Context, tx *sql.Tx, lineID string, p channelProofUpload, actor int64) (*ChannelProof, error) {
	if _, err := uuid.Parse(lineID); err != nil {
		return nil, ErrNotFound
	}
	var account, wallet, currency, external string
	err := tx.QueryRowContext(ctx, `SELECT channel_account_id::text,wallet,currency,external_id FROM accounting_channel_lines WHERE id=$1 AND status='ACTIVE'`, lineID).
		Scan(&account, &wallet, &currency, &external)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "SELECT 1 FROM accounting_channel_accounts WHERE id=$1 FOR UPDATE", account); err != nil {
		return nil, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounting_channel_attachments WHERE channel_account_id=$1 AND wallet=$2 AND currency=$3 AND external_id=$4 AND deleted_at IS NULL`,
		account, wallet, currency, external).Scan(&count); err != nil {
		return nil, err
	}
	if count >= maxChannelProofs {
		return nil, ErrState.WithMetadata(map[string]string{"field": "too_many_proofs"})
	}
	proof := &ChannelProof{ID: p.ID, Filename: p.Filename, ContentType: p.ContentType, Size: p.Size, CreatedBy: actor}
	err = tx.QueryRowContext(ctx, `INSERT INTO accounting_channel_attachments(id,channel_account_id,wallet,currency,external_id,filename,content_type,size_bytes,sha256,object_key,created_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING created_at`, p.ID, account, wallet, currency, external, p.Filename, p.ContentType, p.Size, p.SHA256, p.ObjectKey, actor).Scan(&proof.CreatedAt)
	return proof, err
}

func (e *Engine) DeleteChannelProof(ctx context.Context, tx *sql.Tx, id string, actor int64) (*ChannelProof, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	proof := &ChannelProof{ID: id}
	err := tx.QueryRowContext(ctx, `UPDATE accounting_channel_attachments SET deleted_at=NOW(),deleted_by=$2 WHERE id=$1 AND deleted_at IS NULL
 RETURNING filename,content_type,size_bytes,created_by,created_at`, id, actor).Scan(&proof.Filename, &proof.ContentType, &proof.Size, &proof.CreatedBy, &proof.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return proof, err
}

func channelProofObject(ctx context.Context, q dbQuery, id string) (filename, contentType, key string, err error) {
	if _, err = uuid.Parse(id); err != nil {
		return "", "", "", ErrNotFound
	}
	err = q.QueryRowContext(ctx, "SELECT filename,content_type,object_key FROM accounting_channel_attachments WHERE id=$1 AND deleted_at IS NULL", id).Scan(&filename, &contentType, &key)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return filename, contentType, key, err
}

// attachChannelProofs fills the proofs of listed lines with one query.
func attachChannelProofs(ctx context.Context, q dbQuery, account string, lines []ChannelLine) error {
	if len(lines) == 0 {
		return nil
	}
	refs := make([]string, 0, len(lines))
	index := map[string][]int{}
	for i := range lines {
		ref := lines[i].Wallet + "/" + lines[i].Currency + "/" + lines[i].ExternalID
		if _, seen := index[ref]; !seen {
			refs = append(refs, ref)
		}
		index[ref] = append(index[ref], i)
		lines[i].Proofs = []ChannelProof{}
	}
	rows, err := q.QueryContext(ctx, `SELECT wallet||'/'||currency||'/'||external_id,id::text,filename,content_type,size_bytes,created_by,created_at FROM accounting_channel_attachments
 WHERE channel_account_id=$1 AND deleted_at IS NULL AND wallet||'/'||currency||'/'||external_id = ANY($2) ORDER BY created_at,id`, account, pq.Array(refs))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ref string
		var p ChannelProof
		if err = rows.Scan(&ref, &p.ID, &p.Filename, &p.ContentType, &p.Size, &p.CreatedBy, &p.CreatedAt); err != nil {
			return err
		}
		for _, i := range index[ref] {
			lines[i].Proofs = append(lines[i].Proofs, p)
		}
	}
	return rows.Err()
}

// channelProofName keeps a safe display name for an uploaded proof.
func channelProofName(name string) string {
	name = strings.TrimSpace(strings.NewReplacer("/", "_", "\\", "_", "\r", "", "\n", "").Replace(name))
	for len(name) > 255 {
		runes := []rune(name)
		name = string(runes[1:])
	}
	return name
}
