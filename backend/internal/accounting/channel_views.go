package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting/channel"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ChannelAccount struct {
	ID         string    `json:"id"`
	Platform   string    `json:"platform"`
	AccountRef string    `json:"account_ref"`
	Name       string    `json:"name"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChannelReported compares a balance the platform printed with the books at the same moment.
type ChannelReported struct {
	Amount     string    `json:"amount"`
	AsOf       time.Time `json:"as_of"`
	Source     string    `json:"source"`
	ImportID   string    `json:"import_id"`
	Books      string    `json:"books"`
	Difference string    `json:"difference"`
	Matched    bool      `json:"matched"`
}

// ChannelGap is a break in the platform's running balance: lines between the two are missing from the books.
type ChannelGap struct {
	AfterID    string    `json:"after_external_id"`
	AfterAt    time.Time `json:"after_at"`
	BeforeID   string    `json:"before_external_id"`
	BeforeAt   time.Time `json:"before_at"`
	Expected   string    `json:"expected_balance"`
	Reported   string    `json:"reported_balance"`
	Difference string    `json:"difference"`
}

type ChannelSpan struct {
	From   time.Time  `json:"from"`
	To     *time.Time `json:"to,omitempty"`
	Lowest string     `json:"lowest"`
}

type ChannelWallet struct {
	Wallet        string           `json:"wallet"`
	Currency      string           `json:"currency"`
	Opening       string           `json:"opening"`
	Balance       string           `json:"balance"`
	Lines         int              `json:"lines"`
	FirstAt       time.Time        `json:"first_at"`
	LastAt        time.Time        `json:"last_at"`
	Reported      *ChannelReported `json:"reported,omitempty"`
	Gaps          []ChannelGap     `json:"gaps"`
	NegativeSpans []ChannelSpan    `json:"negative_spans"`
}

type ChannelAmount struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type ChannelAccountSummary struct {
	Account           ChannelAccount  `json:"account"`
	Wallets           []ChannelWallet `json:"wallets"`
	Positions         []ChannelAmount `json:"positions"`
	FeeCosts          []ChannelAmount `json:"fee_costs"`
	UnpairedTransfers int             `json:"unpaired_transfers"`
	MissingProofs     int             `json:"missing_proofs"`
	Cards             int             `json:"cards"`
	LastImportAt      *time.Time      `json:"last_import_at,omitempty"`
}

type ChannelMonth struct {
	Month    string            `json:"month"`
	Currency string            `json:"currency"`
	Totals   map[string]string `json:"totals"`
	FeeCost  string            `json:"fee_cost"`
}

type ChannelConversion struct {
	At             time.Time `json:"at"`
	Reference      string    `json:"reference"`
	Sold           string    `json:"sold"`
	SoldCurrency   string    `json:"sold_currency"`
	Bought         string    `json:"bought"`
	BoughtCurrency string    `json:"bought_currency"`
	Rate           string    `json:"rate"`
}

type ChannelCardSummary struct {
	ID       string     `json:"id"`
	CardRef  string     `json:"card_ref"`
	Last4    string     `json:"card_last4"`
	Name     string     `json:"card_name"`
	Currency string     `json:"currency"`
	Spends   int        `json:"spends"`
	Spend    string     `json:"spend"`
	Refund   string     `json:"refund"`
	LastAt   *time.Time `json:"last_at,omitempty"`
}

type ChannelAccountDetail struct {
	ChannelAccountSummary
	Months      []ChannelMonth       `json:"months"`
	Conversions []ChannelConversion  `json:"conversions"`
	CardSpend   []ChannelCardSummary `json:"card_spend"`
	Unpaired    []ChannelLine        `json:"unpaired"`
	Imports     []ChannelImport      `json:"imports"`
}

type ChannelLine struct {
	ID               string          `json:"id"`
	Wallet           string          `json:"wallet"`
	ExternalID       string          `json:"external_id"`
	OccurredAt       time.Time       `json:"occurred_at"`
	Type             string          `json:"type"`
	Amount           string          `json:"amount"`
	Currency         string          `json:"currency"`
	BalanceAfter     *string         `json:"balance_after,omitempty"`
	FeeAmount        *string         `json:"fee_amount,omitempty"`
	CardRef          string          `json:"card_ref,omitempty"`
	CardLast4        string          `json:"card_last4,omitempty"`
	CardName         string          `json:"card_name,omitempty"`
	Merchant         string          `json:"merchant,omitempty"`
	RelatedID        string          `json:"related_id,omitempty"`
	OriginalAmount   *string         `json:"original_amount,omitempty"`
	OriginalCurrency string          `json:"original_currency,omitempty"`
	Remark           string          `json:"remark,omitempty"`
	Booking          string          `json:"booking"`
	ImportID         string          `json:"import_id"`
	Pair             *ChannelLineRef `json:"pair,omitempty"`
	Proofs           []ChannelProof  `json:"proofs"`
}

type ChannelLineRef struct {
	ID         string `json:"id"`
	Wallet     string `json:"wallet"`
	Currency   string `json:"currency"`
	ExternalID string `json:"external_id"`
	Amount     string `json:"amount"`
}

type ChannelLineFilter struct {
	Wallet, Currency, Type, Card, From, To, Search string
	Page, Size                                     int
}

const channelAccountColumns = "id::text,platform,account_ref,name,remark,created_at"

func scanChannelAccount(row interface{ Scan(...any) error }) (*ChannelAccount, error) {
	a := &ChannelAccount{}
	err := row.Scan(&a.ID, &a.Platform, &a.AccountRef, &a.Name, &a.Remark, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

func getChannelAccount(ctx context.Context, q dbQuery, id string) (*ChannelAccount, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	return scanChannelAccount(q.QueryRowContext(ctx, "SELECT "+channelAccountColumns+" FROM accounting_channel_accounts WHERE id=$1", id))
}

func (e *Engine) ListChannelAccounts(ctx context.Context) ([]ChannelAccountSummary, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT id::text FROM accounting_channel_accounts ORDER BY platform,account_ref")
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []ChannelAccountSummary{}
	for _, id := range ids {
		s, summaryErr := channelAccountSummary(ctx, e.db, id)
		if summaryErr != nil {
			return nil, summaryErr
		}
		out = append(out, *s)
	}
	return out, nil
}

func (e *Engine) UpdateChannelAccount(ctx context.Context, tx *sql.Tx, id, name, remark string) (*ChannelAccount, error) {
	if len(name) > 200 || len(remark) > 2000 {
		return nil, ErrInvalid
	}
	res, err := tx.ExecContext(ctx, "UPDATE accounting_channel_accounts SET name=$2,remark=$3,updated_at=NOW() WHERE id=$1", id, strings.TrimSpace(name), strings.TrimSpace(remark))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return getChannelAccount(ctx, tx, id)
}

type walletLine struct {
	wallet, currency, externalID string
	at                           time.Time
	amount                       decimal.Decimal
	balance                      *decimal.Decimal
}

// channelAccountSummary walks every active line of the account once to derive balances, breaks in the
// platform's running balance and periods spent below zero.
func channelAccountSummary(ctx context.Context, q dbQuery, id string) (*ChannelAccountSummary, error) {
	account, err := getChannelAccount(ctx, q, id)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT wallet,currency,external_id,occurred_at,amount::text,balance_after::text FROM accounting_channel_lines
 WHERE channel_account_id=$1 AND status='ACTIVE' ORDER BY wallet,currency,occurred_at,seq`, id)
	if err != nil {
		return nil, err
	}
	var lines []walletLine
	for rows.Next() {
		var l walletLine
		var amount string
		var balance sql.NullString
		if err = rows.Scan(&l.wallet, &l.currency, &l.externalID, &l.at, &amount, &balance); err != nil {
			_ = rows.Close()
			return nil, err
		}
		l.amount = dec(amount)
		if balance.Valid {
			b := dec(balance.String)
			l.balance = &b
		}
		lines = append(lines, l)
	}
	_ = rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	reported, err := latestReported(ctx, q, account)
	if err != nil {
		return nil, err
	}
	summary := &ChannelAccountSummary{Account: *account, Wallets: []ChannelWallet{}, Positions: []ChannelAmount{}, FeeCosts: []ChannelAmount{}}
	positions := map[string]decimal.Decimal{}
	for start := 0; start < len(lines); {
		end := start
		for end < len(lines) && lines[end].wallet == lines[start].wallet && lines[end].currency == lines[start].currency {
			end++
		}
		w := walletSummary(lines[start:end], reported[lines[start].wallet+"/"+lines[start].currency])
		summary.Wallets = append(summary.Wallets, w)
		positions[w.Currency] = positions[w.Currency].Add(dec(w.Balance))
		start = end
	}
	// Money flows from the balance that saw activity first; that order reads like the funds chain.
	sort.SliceStable(summary.Wallets, func(i, j int) bool { return summary.Wallets[i].FirstAt.Before(summary.Wallets[j].FirstAt) })
	summary.Positions = sortedAmounts(positions)
	fees := map[string]decimal.Decimal{}
	feeRows, err := q.QueryContext(ctx, `SELECT currency,(COALESCE(SUM(CASE WHEN type='FEE' THEN -amount ELSE 0 END),0)+COALESCE(SUM(fee_amount),0))::text
 FROM accounting_channel_lines WHERE channel_account_id=$1 AND status='ACTIVE' GROUP BY currency`, id)
	if err != nil {
		return nil, err
	}
	for feeRows.Next() {
		var currency, total string
		if err = feeRows.Scan(&currency, &total); err != nil {
			_ = feeRows.Close()
			return nil, err
		}
		if d := dec(total); !d.IsZero() {
			fees[currency] = d
		}
	}
	_ = feeRows.Close()
	if err = feeRows.Err(); err != nil {
		return nil, err
	}
	summary.FeeCosts = sortedAmounts(fees)
	var last sql.NullTime
	if err = q.QueryRowContext(ctx, `SELECT
 (SELECT COUNT(*) FROM accounting_channel_lines WHERE channel_account_id=$1 AND status='ACTIVE' AND type IN ('TRANSFER_IN','TRANSFER_OUT') AND pair_line_id IS NULL),
 (SELECT COUNT(*) FROM accounting_channel_lines l WHERE l.channel_account_id=$1 AND l.status='ACTIVE' AND l.type='FUNDING_IN'
   AND NOT EXISTS(SELECT 1 FROM accounting_channel_attachments a WHERE a.channel_account_id=l.channel_account_id AND a.wallet=l.wallet
   AND a.currency=l.currency AND a.external_id=l.external_id AND a.deleted_at IS NULL)),
 (SELECT COUNT(*) FROM accounting_channel_cards WHERE channel_account_id=$1),
 (SELECT MAX(created_at) FROM accounting_channel_imports WHERE status='APPLIED' AND account_ids ? $1::text)`, id).
		Scan(&summary.UnpairedTransfers, &summary.MissingProofs, &summary.Cards, &last); err != nil {
		return nil, err
	}
	if last.Valid {
		summary.LastImportAt = &last.Time
	}
	return summary, nil
}

func sortedAmounts(m map[string]decimal.Decimal) []ChannelAmount {
	out := make([]ChannelAmount, 0, len(m))
	for currency, amount := range m {
		out = append(out, ChannelAmount{Currency: currency, Amount: amount.StringFixed(2)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

func walletSummary(lines []walletLine, reported *channelReportedBalance) ChannelWallet {
	first := lines[0]
	opening := decimal.Zero
	if first.balance != nil {
		opening = first.balance.Sub(first.amount)
	}
	w := ChannelWallet{Wallet: first.wallet, Currency: first.currency, Lines: len(lines), FirstAt: first.at, LastAt: lines[len(lines)-1].at, Gaps: []ChannelGap{}, NegativeSpans: []ChannelSpan{}}
	running := opening
	var span *ChannelSpan
	lowest := decimal.Zero
	atReport := opening
	for i, l := range lines {
		running = running.Add(l.amount)
		if reported != nil && !l.at.After(reported.AsOf) {
			atReport = running
		}
		if i > 0 && l.balance != nil && lines[i-1].balance != nil && !lines[i-1].balance.Add(l.amount).Equal(*l.balance) {
			expected := lines[i-1].balance.Add(l.amount)
			w.Gaps = append(w.Gaps, ChannelGap{AfterID: lines[i-1].externalID, AfterAt: lines[i-1].at, BeforeID: l.externalID, BeforeAt: l.at,
				Expected: expected.StringFixed(2), Reported: l.balance.StringFixed(2), Difference: l.balance.Sub(expected).StringFixed(2)})
		}
		// The platform's own balance is the truth for overdrafts; fall back to the running total without it.
		current := running
		if l.balance != nil {
			current = *l.balance
		}
		switch {
		case current.IsNegative() && span == nil:
			span, lowest = &ChannelSpan{From: l.at}, current
		case current.IsNegative():
			lowest = decimal.Min(lowest, current)
		case span != nil:
			at := l.at
			span.To, span.Lowest = &at, lowest.StringFixed(2)
			w.NegativeSpans = append(w.NegativeSpans, *span)
			span = nil
		}
	}
	if span != nil {
		span.Lowest = lowest.StringFixed(2)
		w.NegativeSpans = append(w.NegativeSpans, *span)
	}
	w.Opening, w.Balance = opening.StringFixed(2), running.StringFixed(2)
	if reported != nil {
		diff := atReport.Sub(reported.Amount)
		w.Reported = &ChannelReported{Amount: reported.Amount.StringFixed(2), AsOf: reported.AsOf, Source: reported.Source, ImportID: reported.importID,
			Books: atReport.StringFixed(2), Difference: diff.StringFixed(2), Matched: diff.IsZero()}
	}
	return w
}

type channelReportedBalance struct {
	channel.Balance
	importID string
}

// latestReported returns, per wallet and currency, the newest balance the platform reported for the account.
func latestReported(ctx context.Context, q dbQuery, account *ChannelAccount) (map[string]*channelReportedBalance, error) {
	filter, _ := json.Marshal([]map[string]string{{"platform": account.Platform, "account_ref": account.AccountRef}})
	rows, err := q.QueryContext(ctx, `SELECT id::text,reported_balances FROM accounting_channel_imports WHERE status='APPLIED'
 AND reported_balances @> $1::jsonb ORDER BY created_at,id`, string(filter))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]*channelReportedBalance{}
	for rows.Next() {
		var id string
		var body []byte
		if err = rows.Scan(&id, &body); err != nil {
			return nil, err
		}
		var balances []channel.Balance
		if err = json.Unmarshal(body, &balances); err != nil {
			return nil, err
		}
		for _, b := range balances {
			if b.Platform != account.Platform || b.AccountRef != account.AccountRef {
				continue
			}
			key := b.Wallet + "/" + b.Currency
			if current := out[key]; current == nil || !b.AsOf.Before(current.AsOf) {
				out[key] = &channelReportedBalance{Balance: b, importID: id}
			}
		}
	}
	return out, rows.Err()
}

func (e *Engine) ChannelAccountDetail(ctx context.Context, id string) (*ChannelAccountDetail, error) {
	summary, err := channelAccountSummary(ctx, e.db, id)
	if err != nil {
		return nil, err
	}
	d := &ChannelAccountDetail{ChannelAccountSummary: *summary, Months: []ChannelMonth{}, Conversions: []ChannelConversion{}, CardSpend: []ChannelCardSummary{}}
	if d.Months, err = channelMonths(ctx, e.db, id); err != nil {
		return nil, err
	}
	if d.Conversions, err = channelConversions(ctx, e.db, id); err != nil {
		return nil, err
	}
	if d.CardSpend, err = channelCardSpend(ctx, e.db, id); err != nil {
		return nil, err
	}
	unpaired, _, err := e.ChannelLines(ctx, id, ChannelLineFilter{Type: "UNPAIRED_TRANSFER", Page: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	d.Unpaired = unpaired
	if d.Imports, _, err = e.ListChannelImports(ctx, id, 1, 20); err != nil {
		return nil, err
	}
	return d, nil
}

func channelMonths(ctx context.Context, q dbQuery, id string) ([]ChannelMonth, error) {
	rows, err := q.QueryContext(ctx, `SELECT to_char(occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM') m,currency,type,SUM(amount)::text,COALESCE(SUM(fee_amount),0)::text
 FROM accounting_channel_lines WHERE channel_account_id=$1 AND status='ACTIVE' GROUP BY m,currency,type ORDER BY m DESC,currency`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChannelMonth{}
	index := map[string]int{}
	for rows.Next() {
		var month, currency, kind, total, fee string
		if err = rows.Scan(&month, &currency, &kind, &total, &fee); err != nil {
			return nil, err
		}
		key := month + "/" + currency
		i, ok := index[key]
		if !ok {
			out = append(out, ChannelMonth{Month: month, Currency: currency, Totals: map[string]string{}, FeeCost: "0.00"})
			i = len(out) - 1
			index[key] = i
		}
		out[i].Totals[kind] = dec(total).StringFixed(2)
		cost := dec(out[i].FeeCost).Add(dec(fee))
		if kind == channel.Fee {
			cost = cost.Sub(dec(total))
		}
		out[i].FeeCost = cost.StringFixed(2)
	}
	return out, rows.Err()
}

func channelConversions(ctx context.Context, q dbQuery, id string) ([]ChannelConversion, error) {
	rows, err := q.QueryContext(ctx, `SELECT o.occurred_at,o.related_id,(-o.amount)::text,o.currency,i.amount::text,i.currency
 FROM accounting_channel_lines o JOIN accounting_channel_lines i ON i.id=o.pair_line_id
 WHERE o.channel_account_id=$1 AND o.status='ACTIVE' AND o.type='FX_OUT' ORDER BY o.occurred_at DESC,o.seq DESC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChannelConversion{}
	for rows.Next() {
		c := ChannelConversion{}
		var sold, bought string
		if err = rows.Scan(&c.At, &c.Reference, &sold, &c.SoldCurrency, &bought, &c.BoughtCurrency); err != nil {
			return nil, err
		}
		c.Sold, c.Bought = dec(sold).StringFixed(2), dec(bought).StringFixed(2)
		if b := dec(bought); b.IsPositive() {
			c.Rate = dec(sold).Div(b).StringFixed(4)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func channelCardSpend(ctx context.Context, q dbQuery, id string) ([]ChannelCardSummary, error) {
	rows, err := q.QueryContext(ctx, `SELECT c.id::text,c.card_ref,c.card_last4,c.card_name,l.currency,
 COUNT(*) FILTER (WHERE l.type='CONSUME'),COALESCE(-SUM(l.amount) FILTER (WHERE l.type='CONSUME'),0)::text,
 COALESCE(SUM(l.amount) FILTER (WHERE l.type='REFUND'),0)::text,MAX(l.occurred_at)
 FROM accounting_channel_cards c JOIN accounting_channel_lines l ON l.channel_card_id=c.id AND l.status='ACTIVE'
 WHERE c.channel_account_id=$1 GROUP BY c.id,c.card_ref,c.card_last4,c.card_name,l.currency
 ORDER BY COALESCE(-SUM(l.amount) FILTER (WHERE l.type='CONSUME'),0) DESC,c.card_ref`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChannelCardSummary{}
	for rows.Next() {
		c := ChannelCardSummary{}
		var spend, refund string
		var last sql.NullTime
		if err = rows.Scan(&c.ID, &c.CardRef, &c.Last4, &c.Name, &c.Currency, &c.Spends, &spend, &refund, &last); err != nil {
			return nil, err
		}
		c.Spend, c.Refund = dec(spend).StringFixed(2), dec(refund).StringFixed(2)
		if last.Valid {
			c.LastAt = &last.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (e *Engine) ChannelLines(ctx context.Context, account string, f ChannelLineFilter) ([]ChannelLine, int, error) {
	if _, err := getChannelAccount(ctx, e.db, account); err != nil {
		return nil, 0, err
	}
	clauses := []string{"l.channel_account_id=$1", "l.status='ACTIVE'"}
	args := []any{account}
	add := func(expr string, v any) {
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf(expr, len(args)))
	}
	if f.Wallet != "" {
		add("l.wallet=$%d", f.Wallet)
	}
	if f.Currency != "" {
		add("l.currency=$%d", f.Currency)
	}
	switch {
	case f.Type == "UNPAIRED_TRANSFER":
		clauses = append(clauses, "l.type IN ('TRANSFER_IN','TRANSFER_OUT') AND l.pair_line_id IS NULL")
	case f.Type == "MISSING_PROOF":
		clauses = append(clauses, `l.type='FUNDING_IN' AND NOT EXISTS(SELECT 1 FROM accounting_channel_attachments a WHERE a.channel_account_id=l.channel_account_id
 AND a.wallet=l.wallet AND a.currency=l.currency AND a.external_id=l.external_id AND a.deleted_at IS NULL)`)
	case f.Type != "":
		if _, ok := channel.Sign[f.Type]; !ok {
			return nil, 0, invalid("type")
		}
		add("l.type=$%d", f.Type)
	}
	if f.Card != "" {
		if _, err := uuid.Parse(f.Card); err != nil {
			return nil, 0, invalid("card")
		}
		add("l.channel_card_id=$%d", f.Card)
	}
	for _, bound := range []struct{ raw, expr string }{{f.From, "l.occurred_at>=$%d"}, {f.To, "l.occurred_at<$%d"}} {
		if bound.raw == "" {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", bound.raw, accountingZone)
		if err != nil {
			return nil, 0, invalid("date")
		}
		if bound.expr == "l.occurred_at<$%d" {
			day = day.AddDate(0, 0, 1)
		}
		add(bound.expr, day)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		add("(l.external_id ILIKE $%[1]d OR l.merchant ILIKE $%[1]d OR l.card_name ILIKE $%[1]d OR l.related_id ILIKE $%[1]d OR l.remark ILIKE $%[1]d)", "%"+s+"%")
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_channel_lines l WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Size, (f.Page-1)*f.Size)
	rows, err := e.db.QueryContext(ctx, `SELECT l.id::text,l.wallet,l.external_id,l.occurred_at,l.type,l.amount::text,l.currency,l.balance_after::text,l.fee_amount::text,
 l.card_ref,l.card_last4,l.card_name,l.merchant,l.related_id,l.original_amount::text,l.original_currency,l.remark,l.booking,l.import_id::text,
 p.id::text,p.wallet,p.currency,p.external_id,p.amount::text
 FROM accounting_channel_lines l LEFT JOIN accounting_channel_lines p ON p.id=l.pair_line_id
 WHERE `+where+fmt.Sprintf(" ORDER BY l.occurred_at DESC,l.seq DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChannelLine{}
	for rows.Next() {
		l := ChannelLine{}
		var balance, fee, original, pairID, pairWallet, pairCurrency, pairExternal, pairAmount sql.NullString
		if err = rows.Scan(&l.ID, &l.Wallet, &l.ExternalID, &l.OccurredAt, &l.Type, &l.Amount, &l.Currency, &balance, &fee, &l.CardRef, &l.CardLast4, &l.CardName,
			&l.Merchant, &l.RelatedID, &original, &l.OriginalCurrency, &l.Remark, &l.Booking, &l.ImportID, &pairID, &pairWallet, &pairCurrency, &pairExternal, &pairAmount); err != nil {
			return nil, 0, err
		}
		l.Amount = dec(l.Amount).StringFixed(2)
		l.BalanceAfter, l.FeeAmount, l.OriginalAmount = fixed(balance), fixed(fee), fixed(original)
		if pairID.Valid {
			l.Pair = &ChannelLineRef{ID: pairID.String, Wallet: pairWallet.String, Currency: pairCurrency.String, ExternalID: pairExternal.String, Amount: dec(pairAmount.String).StringFixed(2)}
		}
		out = append(out, l)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	if err = attachChannelProofs(ctx, e.db, account, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func fixed(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := dec(v.String).StringFixed(2)
	return &s
}

const channelImportColumns = `i.id::text,i.converter,i.files,i.reported_balances,i.total_rows,i.new_rows,i.duplicate_rows,i.pending_rows,i.status,
 i.account_ids,
 i.created_by,i.created_at,i.revoked_by,i.revoked_at,i.revoke_reason`

func scanChannelImport(row interface{ Scan(...any) error }) (*ChannelImport, error) {
	im := &ChannelImport{}
	var files, balances, accounts []byte
	err := row.Scan(&im.ID, &im.Converter, &files, &balances, &im.TotalRows, &im.NewRows, &im.DuplicateRows, &im.PendingRows, &im.Status, &accounts,
		&im.CreatedBy, &im.CreatedAt, &im.RevokedBy, &im.RevokedAt, &im.RevokeReason)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(files, &im.Files); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(balances, &im.ReportedBalances); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(accounts, &im.AccountIDs); err != nil {
		return nil, err
	}
	for i := range im.Files {
		im.Files[i].ObjectKey = ""
	}
	return im, nil
}

func getChannelImport(ctx context.Context, q dbQuery, id string) (*ChannelImport, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	return scanChannelImport(q.QueryRowContext(ctx, "SELECT "+channelImportColumns+" FROM accounting_channel_imports i WHERE i.id=$1", id))
}

// ListChannelImports lists batches, newest first, optionally only those covering one account.
func (e *Engine) ListChannelImports(ctx context.Context, account string, page, size int) ([]ChannelImport, int, error) {
	where, args := "TRUE", []any{}
	if account != "" {
		if _, err := uuid.Parse(account); err != nil {
			return nil, 0, ErrNotFound
		}
		where, args = "i.account_ids ? $1", []any{account}
	}
	var total int
	if err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounting_channel_imports i WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := e.db.QueryContext(ctx, "SELECT "+channelImportColumns+" FROM accounting_channel_imports i WHERE "+where+
		fmt.Sprintf(" ORDER BY i.created_at DESC,i.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChannelImport{}
	for rows.Next() {
		im, scanErr := scanChannelImport(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		out = append(out, *im)
	}
	return out, total, rows.Err()
}

// channelFileObject returns the storage key of one original file of a batch.
func channelFileObject(ctx context.Context, q dbQuery, id string, index int) (*ChannelFile, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	var body []byte
	if err := q.QueryRowContext(ctx, "SELECT files FROM accounting_channel_imports WHERE id=$1", id).Scan(&body); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var files []ChannelFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, err
	}
	if index < 0 || index >= len(files) || files[index].ObjectKey == "" {
		return nil, ErrNotFound
	}
	return &files[index], nil
}
