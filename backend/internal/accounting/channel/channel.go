// Package channel turns platform statements into standard channel lines: every line is one money movement in
// one balance (account + wallet + currency) of a channel account we own. Platform specific converters produce
// the same lines as the standard template, so booking never depends on the source format.
package channel

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Line types. Signs are relative to the balance the line belongs to.
const (
	FundingIn   = "FUNDING_IN"
	FundingOut  = "FUNDING_OUT"
	TransferIn  = "TRANSFER_IN"
	TransferOut = "TRANSFER_OUT"
	FXIn        = "FX_IN"
	FXOut       = "FX_OUT"
	Consume     = "CONSUME"
	Refund      = "REFUND"
	Fee         = "FEE"
	Adjustment  = "ADJUSTMENT"
)

// Sign is +1 when the type must add to the balance, -1 when it must take from it and 0 when either is allowed.
var Sign = map[string]int{
	FundingIn: 1, FundingOut: -1, TransferIn: 1, TransferOut: -1, FXIn: 1, FXOut: -1,
	Consume: -1, Refund: 1, Fee: -1, Adjustment: 0,
}

const (
	MaxFileBytes  = 20 << 20
	MaxFiles      = 20
	MaxLines      = 50000
	TemplateName  = "TEMPLATE"
	DefaultWallet = "MAIN"
)

type Line struct {
	Platform         string            `json:"platform"`
	AccountRef       string            `json:"account_ref"`
	Wallet           string            `json:"wallet"`
	ExternalID       string            `json:"external_id"`
	OccurredAt       time.Time         `json:"occurred_at"`
	Type             string            `json:"type"`
	Amount           decimal.Decimal   `json:"amount"`
	Currency         string            `json:"currency"`
	BalanceAfter     *decimal.Decimal  `json:"balance_after,omitempty"`
	FeeAmount        *decimal.Decimal  `json:"fee_amount,omitempty"`
	CardRef          string            `json:"card_ref,omitempty"`
	CardLast4        string            `json:"card_last4,omitempty"`
	CardName         string            `json:"card_name,omitempty"`
	Merchant         string            `json:"merchant,omitempty"`
	RelatedID        string            `json:"related_id,omitempty"`
	OriginalAmount   *decimal.Decimal  `json:"original_amount,omitempty"`
	OriginalCurrency string            `json:"original_currency,omitempty"`
	Remark           string            `json:"remark,omitempty"`
	Source           map[string]string `json:"source,omitempty"`
	// File and Row locate the line in the uploaded files for error messages.
	File string `json:"-"`
	Row  int    `json:"-"`
}

// Key identifies the balance a line belongs to.
type Key struct {
	Platform   string `json:"platform"`
	AccountRef string `json:"account_ref"`
	Wallet     string `json:"wallet"`
	Currency   string `json:"currency"`
}

func (l *Line) Key() Key { return Key{l.Platform, l.AccountRef, l.Wallet, l.Currency} }
func (k Key) String() string {
	return k.Platform + "/" + k.AccountRef + "/" + k.Wallet + "/" + k.Currency
}

// Balance is a closing balance the platform reported for one balance, e.g. a statement's closing balance.
type Balance struct {
	Key
	Amount decimal.Decimal `json:"amount"`
	AsOf   time.Time       `json:"as_of"`
	Source string          `json:"source"`
}

type Result struct {
	Converter string            `json:"converter"`
	Lines     []Line            `json:"lines"`
	Balances  []Balance         `json:"balances"`
	Accounts  map[string]string `json:"accounts"` // "PLATFORM/ACCOUNT_REF" -> account name when the source names it
}

type SourceFile struct {
	Name string
	Data []byte
}

func (f SourceFile) Ext() string { return strings.ToLower(path.Ext(f.Name)) }

type Converter interface {
	Name() string
	Detect(files []SourceFile) bool
	Convert(files []SourceFile) (*Result, error)
}

var converters = []Converter{sunrate{}}

// Problem is one reason an upload was rejected, located at a file row or column when possible.
type Problem struct {
	File    string `json:"file,omitempty"`
	Row     int    `json:"row,omitempty"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

type Problems []Problem

func (p Problems) Error() string {
	parts := make([]string, 0, len(p))
	for _, item := range p {
		at := item.File
		if item.Row > 0 {
			at += fmt.Sprintf(":%d", item.Row)
		}
		if item.Column != "" {
			at += " " + item.Column
		}
		parts = append(parts, strings.TrimSpace(at+" "+item.Message))
	}
	return strings.Join(parts, "; ")
}

func problem(file string, row int, column, format string, args ...any) Problems {
	return Problems{{File: file, Row: row, Column: column, Message: fmt.Sprintf(format, args...)}}
}

// Convert turns an upload into standard lines. Standard templates are read directly; otherwise exactly one
// registered converter must recognise the files. converter forces a converter by name when not empty.
func Convert(files []SourceFile, converter string) (*Result, error) {
	if len(files) == 0 {
		return nil, problem("", 0, "", "no files")
	}
	if len(files) > MaxFiles {
		return nil, problem("", 0, "", "at most %d files per import", MaxFiles)
	}
	for _, f := range files {
		if len(f.Data) == 0 || len(f.Data) > MaxFileBytes {
			return nil, problem(f.Name, 0, "", "file must be between 1 byte and %d MiB", MaxFileBytes>>20)
		}
	}
	var result *Result
	var err error
	switch {
	case strings.EqualFold(converter, TemplateName) || (converter == "" && allTemplates(files)):
		result, err = readTemplates(files)
	default:
		var picked Converter
		for _, c := range converters {
			if strings.EqualFold(c.Name(), converter) || (converter == "" && c.Detect(files)) {
				picked = c
				break
			}
		}
		if picked == nil {
			return nil, problem("", 0, "", "files are neither standard templates nor a known platform export")
		}
		result, err = picked.Convert(files)
	}
	if err != nil {
		return nil, err
	}
	// Readers expect one timeline; balances keep their own order because the sort is stable.
	sort.SliceStable(result.Lines, func(i, j int) bool { return result.Lines[i].OccurredAt.Before(result.Lines[j].OccurredAt) })
	if len(result.Lines) > MaxLines {
		return nil, problem("", 0, "", "at most %d lines per import", MaxLines)
	}
	if err = Validate(result.Lines); err != nil {
		return nil, err
	}
	return result, nil
}
