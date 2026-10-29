package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting/channel"
	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrChannelFiles = appErrors.BadRequest("ACCOUNTING_CHANNEL_FILES_INVALID", "Channel statement files are invalid")
	// ErrChannelConflict means a platform reference was imported before with different content.
	ErrChannelConflict = appErrors.Conflict("ACCOUNTING_CHANNEL_LINE_CONFLICT", "A platform reference was imported before with different content")
)

// channelFilesError carries the problems found in an upload to the client.
func channelFilesError(err error) error {
	problems, ok := err.(channel.Problems)
	if !ok {
		return err
	}
	if len(problems) > 50 {
		problems = problems[:50]
	}
	body, _ := json.Marshal(problems)
	return ErrChannelFiles.WithMetadata(map[string]string{"problems": string(body)})
}

type ChannelFile struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Size      int    `json:"size"`
	ObjectKey string `json:"object_key,omitempty"`
}

type ChannelImport struct {
	ID               string            `json:"id"`
	Converter        string            `json:"converter"`
	Files            []ChannelFile     `json:"files"`
	ReportedBalances []channel.Balance `json:"reported_balances"`
	TotalRows        int               `json:"total_rows"`
	NewRows          int               `json:"new_rows"`
	DuplicateRows    int               `json:"duplicate_rows"`
	PendingRows      int               `json:"pending_rows"`
	Status           string            `json:"status"`
	AccountIDs       []string          `json:"account_ids"`
	CreatedBy        int64             `json:"created_by"`
	CreatedAt        time.Time         `json:"created_at"`
	RevokedBy        *int64            `json:"revoked_by,omitempty"`
	RevokedAt        *time.Time        `json:"revoked_at,omitempty"`
	RevokeReason     string            `json:"revoke_reason,omitempty"`
}

// ChannelImportResult is what an import reports back: the batch plus the state of every balance it touched.
type ChannelImportResult struct {
	Import   ChannelImport           `json:"import"`
	Accounts []ChannelAccountSummary `json:"accounts"`
}

type importedLine struct {
	id   string
	line *channel.Line
}

// ImportChannel books converted lines in one transaction. Lines already imported are skipped; a platform
// reference that comes back with a different amount or type rejects the whole upload.
func (e *Engine) ImportChannel(ctx context.Context, tx *sql.Tx, importID string, result *channel.Result, files []ChannelFile, actor int64) (*ChannelImportResult, error) {
	accounts, err := lockChannelAccounts(ctx, tx, result, actor)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(accounts))
	for _, id := range accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	filesJSON, _ := json.Marshal(files)
	balancesJSON, _ := json.Marshal(result.Balances)
	idsJSON, _ := json.Marshal(ids)
	if _, err = tx.ExecContext(ctx, `INSERT INTO accounting_channel_imports(id,converter,files,reported_balances,account_ids,total_rows,created_by) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		importID, result.Converter, filesJSON, balancesJSON, idsJSON, len(result.Lines), actor); err != nil {
		return nil, err
	}
	var inserted []importedLine
	duplicates := 0
	for i := range result.Lines {
		l := &result.Lines[i]
		account := accounts[channelAccountKey(l.Platform, l.AccountRef)]
		var existingID, existingType, existingAmount string
		err = tx.QueryRowContext(ctx, `SELECT id::text,type,amount::text FROM accounting_channel_lines
 WHERE channel_account_id=$1 AND wallet=$2 AND currency=$3 AND external_id=$4 AND status='ACTIVE'`, account, l.Wallet, l.Currency, l.ExternalID).Scan(&existingID, &existingType, &existingAmount)
		if err == nil {
			if existingType != l.Type || !dec(existingAmount).Equal(l.Amount) {
				return nil, ErrChannelConflict.WithMetadata(map[string]string{"external_id": l.ExternalID, "wallet": l.Wallet, "currency": l.Currency})
			}
			duplicates++
			continue
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		id, insertErr := insertChannelLine(ctx, tx, importID, account, l)
		if insertErr != nil {
			return nil, insertErr
		}
		inserted = append(inserted, importedLine{id: id, line: l})
	}
	if err = pairConversions(ctx, tx, inserted); err != nil {
		return nil, err
	}
	if err = pairTransfers(ctx, tx, inserted); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE accounting_channel_imports SET new_rows=$2,duplicate_rows=$3 WHERE id=$1`, importID, len(inserted), duplicates); err != nil {
		return nil, err
	}
	imported, err := getChannelImport(ctx, tx, importID)
	if err != nil {
		return nil, err
	}
	out := &ChannelImportResult{Import: *imported}
	for _, id := range ids {
		summary, summaryErr := channelAccountSummary(ctx, tx, id)
		if summaryErr != nil {
			return nil, summaryErr
		}
		out.Accounts = append(out.Accounts, *summary)
	}
	return out, nil
}

func channelAccountKey(platform, ref string) string { return platform + "/" + ref }

// lockChannelAccounts creates unknown accounts and locks every account of the upload, in a fixed order, so
// concurrent imports into one account run one after another.
func lockChannelAccounts(ctx context.Context, tx *sql.Tx, result *channel.Result, actor int64) (map[string]string, error) {
	keys := map[string][2]string{}
	for _, l := range result.Lines {
		keys[channelAccountKey(l.Platform, l.AccountRef)] = [2]string{l.Platform, l.AccountRef}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	out := map[string]string{}
	for _, k := range ordered {
		var id string
		err := tx.QueryRowContext(ctx, `INSERT INTO accounting_channel_accounts(id,platform,account_ref,name,created_by) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT (platform,account_ref) DO UPDATE SET name=CASE WHEN accounting_channel_accounts.name='' THEN EXCLUDED.name ELSE accounting_channel_accounts.name END
 RETURNING id::text`, uuid.NewString(), keys[k][0], keys[k][1], result.Accounts[k], actor).Scan(&id)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, "SELECT 1 FROM accounting_channel_accounts WHERE id=$1 FOR UPDATE", id); err != nil {
			return nil, err
		}
		out[k] = id
	}
	return out, nil
}

func insertChannelLine(ctx context.Context, tx *sql.Tx, importID, account string, l *channel.Line) (string, error) {
	var card sql.NullString
	if l.CardRef != "" {
		var id string
		if err := tx.QueryRowContext(ctx, `INSERT INTO accounting_channel_cards(id,channel_account_id,card_ref,card_last4,card_name) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT (channel_account_id,card_ref) DO UPDATE SET card_last4=COALESCE(NULLIF(EXCLUDED.card_last4,''),accounting_channel_cards.card_last4),
 card_name=COALESCE(NULLIF(EXCLUDED.card_name,''),accounting_channel_cards.card_name),updated_at=NOW()
 RETURNING id::text`, uuid.NewString(), account, l.CardRef, l.CardLast4, l.CardName).Scan(&id); err != nil {
			return "", err
		}
		card = sql.NullString{String: id, Valid: true}
	}
	source, _ := json.Marshal(l.Source)
	if l.Source == nil {
		source = []byte("{}")
	}
	id := uuid.NewString()
	_, err := tx.ExecContext(ctx, `INSERT INTO accounting_channel_lines(id,channel_account_id,import_id,wallet,external_id,occurred_at,type,amount,currency,balance_after,fee_amount,
 card_ref,card_last4,card_name,merchant,related_id,original_amount,original_currency,remark,source_row,channel_card_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		id, account, importID, l.Wallet, l.ExternalID, l.OccurredAt, l.Type, l.Amount.String(), l.Currency, nullDecimal(l.BalanceAfter), nullDecimal(l.FeeAmount),
		l.CardRef, l.CardLast4, l.CardName, l.Merchant, l.RelatedID, nullDecimal(l.OriginalAmount), l.OriginalCurrency, l.Remark, source, card)
	return id, err
}

func nullDecimal(d *decimal.Decimal) any {
	if d == nil {
		return nil
	}
	return d.String()
}

func linkPair(ctx context.Context, tx *sql.Tx, a, b string) error {
	_, err := tx.ExecContext(ctx, `UPDATE accounting_channel_lines SET pair_line_id=CASE WHEN id=$1 THEN $2::uuid ELSE $1::uuid END WHERE id IN ($1,$2)`, a, b)
	return err
}

// pairConversions links the sold and bought sides of each conversion. A side whose partner is neither in this
// upload nor imported earlier rejects the upload: the conversion would otherwise look like lost money.
func pairConversions(ctx context.Context, tx *sql.Tx, lines []importedLine) error {
	for _, il := range lines {
		l := il.line
		if l.Type != channel.FXIn && l.Type != channel.FXOut {
			continue
		}
		opposite := channel.FXIn
		if l.Type == channel.FXIn {
			opposite = channel.FXOut
		}
		var paired sql.NullString
		if err := tx.QueryRowContext(ctx, "SELECT pair_line_id::text FROM accounting_channel_lines WHERE id=$1", il.id).Scan(&paired); err != nil {
			return err
		}
		if paired.Valid {
			continue
		}
		var partner string
		err := tx.QueryRowContext(ctx, `SELECT p.id::text FROM accounting_channel_lines p JOIN accounting_channel_lines l ON l.id=$1
 WHERE p.channel_account_id=l.channel_account_id AND p.related_id=l.related_id AND p.type=$2 AND p.currency<>l.currency
 AND p.status='ACTIVE' AND p.pair_line_id IS NULL ORDER BY p.seq LIMIT 1`, il.id, opposite).Scan(&partner)
		if err == sql.ErrNoRows {
			body, _ := json.Marshal(channel.Problems{{File: l.File, Row: l.Row, Column: "related_id", Message: "the other side of conversion " + l.RelatedID + " is missing"}})
			return ErrChannelFiles.WithMetadata(map[string]string{"problems": string(body)})
		}
		if err != nil {
			return err
		}
		if err = linkPair(ctx, tx, il.id, partner); err != nil {
			return err
		}
	}
	return nil
}

// pairTransfers links both sides of a transfer between balances, first by reference and otherwise by currency,
// opposite amount and a timestamp within a day. Unpaired transfers stay visible on the account.
func pairTransfers(ctx context.Context, tx *sql.Tx, lines []importedLine) error {
	for _, il := range lines {
		l := il.line
		if l.Type != channel.TransferIn && l.Type != channel.TransferOut {
			continue
		}
		opposite := channel.TransferIn
		if l.Type == channel.TransferIn {
			opposite = channel.TransferOut
		}
		var paired sql.NullString
		if err := tx.QueryRowContext(ctx, "SELECT pair_line_id::text FROM accounting_channel_lines WHERE id=$1", il.id).Scan(&paired); err != nil {
			return err
		}
		if paired.Valid {
			continue
		}
		var partner string
		err := tx.QueryRowContext(ctx, `SELECT p.id::text FROM accounting_channel_lines p JOIN accounting_channel_lines l ON l.id=$1
 WHERE p.channel_account_id=l.channel_account_id AND p.type=$2 AND p.currency=l.currency AND p.amount=-l.amount
 AND p.status='ACTIVE' AND p.pair_line_id IS NULL AND p.id<>l.id
 AND ((l.related_id<>'' AND p.external_id=l.related_id) OR (p.related_id<>'' AND p.related_id=l.external_id)
   OR (l.related_id='' AND p.related_id='' AND p.occurred_at BETWEEN l.occurred_at-INTERVAL '1 day' AND l.occurred_at+INTERVAL '1 day'))
 ORDER BY (l.related_id<>'' AND p.external_id=l.related_id) DESC, ABS(EXTRACT(EPOCH FROM p.occurred_at-l.occurred_at)), p.seq LIMIT 1`, il.id, opposite).Scan(&partner)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if err = linkPair(ctx, tx, il.id, partner); err != nil {
			return err
		}
	}
	return nil
}

// RevokeChannelImport withdraws a batch that is the newest of every account it covered. Lines stay for the record but
// no longer count, and their platform references may be imported again.
func (e *Engine) RevokeChannelImport(ctx context.Context, tx *sql.Tx, importID, reason string, actor int64) (*ChannelImport, error) {
	var status string
	err := tx.QueryRowContext(ctx, "SELECT status FROM accounting_channel_imports WHERE id=$1 FOR UPDATE", importID).Scan(&status)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "APPLIED" {
		return nil, ErrState
	}
	// A newer batch may have skipped lines because this one booked them first, so it must go first.
	var later bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_channel_imports this JOIN accounting_channel_imports newer
 ON newer.status='APPLIED' AND newer.account_ids ?| ARRAY(SELECT jsonb_array_elements_text(this.account_ids))
 AND (newer.created_at, newer.id) > (this.created_at, this.id) WHERE this.id=$1)`, importID).Scan(&later); err != nil {
		return nil, err
	}
	if later {
		return nil, ErrState.WithMetadata(map[string]string{"field": "not_latest_import"})
	}
	for _, q := range []string{
		"UPDATE accounting_channel_lines SET pair_line_id=NULL WHERE pair_line_id IN (SELECT id FROM accounting_channel_lines WHERE import_id=$1)",
		"UPDATE accounting_channel_lines SET status='REVOKED',pair_line_id=NULL WHERE import_id=$1",
	} {
		if _, err = tx.ExecContext(ctx, q, importID); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE accounting_channel_imports SET status='REVOKED',revoked_by=$2,revoked_at=NOW(),revoke_reason=$3 WHERE id=$1", importID, actor, reason); err != nil {
		return nil, err
	}
	return getChannelImport(ctx, tx, importID)
}
