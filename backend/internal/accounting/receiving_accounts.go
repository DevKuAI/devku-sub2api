package accounting

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/google/uuid"
)

type ReceivingAccount struct {
	ID          string    `json:"id"`
	Currency    string    `json:"currency"`
	AccountName string    `json:"account_name"`
	BankName    string    `json:"bank_name"`
	BankAccount string    `json:"bank_account"`
	BankBranch  string    `json:"bank_branch"`
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ReceivingAccountInput struct {
	Currency    string `json:"currency"`
	AccountName string `json:"account_name"`
	BankName    string `json:"bank_name"`
	BankAccount string `json:"bank_account"`
	BankBranch  string `json:"bank_branch"`
	Remark      string `json:"remark"`
}

func normalizeReceivingAccountInput(in ReceivingAccountInput) (ReceivingAccountInput, error) {
	in.AccountName = strings.TrimSpace(in.AccountName)
	in.BankName = strings.TrimSpace(in.BankName)
	in.BankAccount = strings.TrimSpace(in.BankAccount)
	in.BankBranch = strings.TrimSpace(in.BankBranch)
	in.Remark = strings.TrimSpace(in.Remark)
	currency, err := payment.NormalizePaymentCurrency(in.Currency)
	if err != nil {
		return in, invalid("currency")
	}
	in.Currency = currency
	if in.AccountName == "" || utf8.RuneCountInString(in.AccountName) > 200 {
		return in, invalid("account_name")
	}
	if in.BankName == "" || utf8.RuneCountInString(in.BankName) > 200 {
		return in, invalid("bank_name")
	}
	if in.BankAccount == "" || utf8.RuneCountInString(in.BankAccount) > 100 {
		return in, invalid("bank_account")
	}
	if utf8.RuneCountInString(in.BankBranch) > 200 || utf8.RuneCountInString(in.Remark) > 1000 {
		return in, ErrInvalid
	}
	return in, nil
}

const receivingAccountColumns = "id::text,currency,account_name,bank_name,bank_account,bank_branch,remark,created_at,updated_at"

func scanReceivingAccount(row interface{ Scan(...any) error }) (*ReceivingAccount, error) {
	account := &ReceivingAccount{}
	err := row.Scan(&account.ID, &account.Currency, &account.AccountName, &account.BankName, &account.BankAccount, &account.BankBranch, &account.Remark, &account.CreatedAt, &account.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return account, err
}

func getReceivingAccount(ctx context.Context, q dbQuery, id string, lock bool) (*ReceivingAccount, error) {
	query := "SELECT " + receivingAccountColumns + " FROM accounting_receiving_accounts WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	return scanReceivingAccount(q.QueryRowContext(ctx, query, id))
}

func (e *Engine) ListReceivingAccounts(ctx context.Context) ([]*ReceivingAccount, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT "+receivingAccountColumns+" FROM accounting_receiving_accounts ORDER BY currency,account_name,bank_account,id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	accounts := []*ReceivingAccount{}
	for rows.Next() {
		account, scanErr := scanReceivingAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func createReceivingAccount(ctx context.Context, tx *sql.Tx, actor int64, in ReceivingAccountInput) (*ReceivingAccount, error) {
	in, err := normalizeReceivingAccountInput(in)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	result, err := tx.ExecContext(ctx, `INSERT INTO accounting_receiving_accounts(id,currency,account_name,bank_name,bank_account,bank_branch,remark,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (currency,bank_account) DO NOTHING`, id, in.Currency, in.AccountName, in.BankName, in.BankAccount, in.BankBranch, in.Remark, actor)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, ErrState.WithMetadata(map[string]string{"field": "bank_account"})
	}
	now := time.Now().UTC()
	return &ReceivingAccount{ID: id, Currency: in.Currency, AccountName: in.AccountName, BankName: in.BankName, BankAccount: in.BankAccount, BankBranch: in.BankBranch, Remark: in.Remark, CreatedAt: now, UpdatedAt: now}, nil
}

func updateReceivingAccount(ctx context.Context, tx *sql.Tx, current *ReceivingAccount, in ReceivingAccountInput) (*ReceivingAccount, error) {
	in.Currency = current.Currency
	in, err := normalizeReceivingAccountInput(in)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounting_receiving_accounts SET account_name=$2,bank_name=$3,bank_account=$4,bank_branch=$5,remark=$6,updated_at=NOW() WHERE id=$1`, current.ID, in.AccountName, in.BankName, in.BankAccount, in.BankBranch, in.Remark)
	if err != nil {
		return nil, err
	}
	return getReceivingAccount(ctx, tx, current.ID, false)
}

func hydrateReceivingAccountSnapshot(ctx context.Context, q dbQuery, r *Record) error {
	id := detail(r, "receiving_account_id")
	if id == "" {
		return invalid("receiving_account_id")
	}
	if _, err := uuid.Parse(id); err != nil {
		return invalid("receiving_account_id")
	}
	account, err := getReceivingAccount(ctx, q, id, false)
	if err != nil {
		return err
	}
	if account.Currency != r.Currency {
		return ErrCurrency
	}
	r.Details["receiving_account"] = account.BankAccount
	r.Details["receiving_account_name"] = account.AccountName
	r.Details["receiving_bank_name"] = account.BankName
	r.Details["receiving_bank_branch"] = account.BankBranch
	return nil
}
