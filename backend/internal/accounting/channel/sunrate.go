package channel

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// SUNRATE exports: Statement of Payment Balance PDFs (one per currency) and the card balance ledger carry the
// money; authorisations, collection and conversion records only add detail to those lines.
const (
	sunratePlatform  = "SUNRATE"
	sunratePayment   = "PAYMENT"
	sunrateCard      = "CARD"
	sunrateTimeShift = 8 * 3600
)

var (
	sunrateZone        = time.FixedZone("GMT+8", sunrateTimeShift)
	sunrateAccountFile = regexp.MustCompile(`_(\d+)_\d+\.xlsx$`)
	statementAmount    = regexp.MustCompile(`^[+-]?[0-9][0-9,]*\.[0-9]+$`)
	statementDate      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	statementClosing   = regexp.MustCompile(`^Closing Balance On (.+) :$`)
)

const (
	sunrateStatementPrefix  = "SUNRATE_STATEMENT_PAYMENT_BALANCE_"
	sunrateCardPrefix       = "SUNRATE_CARD_BALANCES_"
	sunrateAuthPrefix       = "SUNRATE_AUTHORISATIONS_"
	sunrateCollectionPrefix = "SUNRATE_RECORD_COLLECTION_"
	sunrateFXPrefix         = "SUNRATE_RECORD_CURRENCY_CONVERSION_"
	sunrateFeesPrefix       = "SUNRATE_FEES_"
)

var sunrateCardTypes = map[string]string{"Transfer In": TransferIn, "Settlement": Consume, "Authorization Fee": Fee}

var sunrateStatementTypes = map[string]string{
	"Collection-Credit": FundingIn, "Sell": FXOut, "Buy": FXIn, "Transfer to Card Balance": TransferOut,
}

var (
	sunrateCardColumns       = []string{"Account Name", "Serial No.", "Card ID", "Currency", "Transaction Amount", "Balance", "Transaction Type", "Reference Number", "Transaction Time (GMT+8:00)"}
	sunrateAuthColumns       = []string{"Card ID", "Transaction ID", "Card No.", "Card Nickname", "Transaction Currency", "Transaction Amount", "Billing Currency", "Merchant Name", "Settlement Time (GMT+8:00)"}
	sunrateCollectionColumns = []string{"订单编号", "汇款方", "币种", "收款金额", "手续费", "实际入账金额", "提交时间"}
	sunrateFXColumns         = []string{"交易编号", "买入币种", "买入金额", "卖出币种", "卖出金额", "汇率", "提交时间"}
)

type sunrate struct{}

func (sunrate) Name() string { return sunratePlatform }

func (sunrate) Detect(files []SourceFile) bool {
	money := false
	for _, f := range files {
		name := strings.ToUpper(f.Name)
		if !strings.HasPrefix(name, "SUNRATE_") {
			return false
		}
		money = money || strings.HasPrefix(name, sunrateStatementPrefix) || strings.HasPrefix(name, sunrateCardPrefix)
	}
	return money
}

type sunrateStatement struct {
	file        string
	accountID   string
	accountName string
	currency    string
	closing     decimal.Decimal
	closingAt   time.Time
	rows        []sunrateStatementRow // newest first, as printed
}

type sunrateStatementRow struct {
	id, amount, balance, date, description string
	row                                    int
}

type sunrateInputs struct {
	statements  []*sunrateStatement
	cardFiles   []SourceFile
	auth        map[string]map[string]string // card id + settlement time -> authorisation
	collections map[string]map[string]string // 订单编号 -> collection record
	conversions map[string]map[string]string // 交易编号 -> conversion record
}

func (s sunrate) Convert(files []SourceFile) (*Result, error) {
	in := sunrateInputs{auth: map[string]map[string]string{}, collections: map[string]map[string]string{}, conversions: map[string]map[string]string{}}
	var problems Problems
	for _, f := range files {
		name := strings.ToUpper(f.Name)
		var err error
		switch {
		case strings.HasPrefix(name, sunrateStatementPrefix) && f.Ext() == ".pdf":
			var st *sunrateStatement
			if st, err = readSunrateStatement(f); err == nil {
				in.statements = append(in.statements, st)
			}
		case strings.HasPrefix(name, sunrateCardPrefix) && f.Ext() == ".xlsx":
			in.cardFiles = append(in.cardFiles, f)
		case strings.HasPrefix(name, sunrateAuthPrefix) && f.Ext() == ".xlsx":
			err = readSunrateRecords(f, sunrateAuthColumns, func(r map[string]string) {
				if r["Settlement Time (GMT+8:00)"] != "" {
					in.auth[r["Card ID"]+"|"+r["Settlement Time (GMT+8:00)"]] = r
				}
			})
		case strings.HasPrefix(name, sunrateCollectionPrefix) && f.Ext() == ".xlsx":
			err = readSunrateRecords(f, sunrateCollectionColumns, func(r map[string]string) { in.collections[r["订单编号"]] = r })
		case strings.HasPrefix(name, sunrateFXPrefix) && f.Ext() == ".xlsx":
			err = readSunrateRecords(f, sunrateFXColumns, func(r map[string]string) { in.conversions[r["交易编号"]] = r })
		case strings.HasPrefix(name, sunrateFeesPrefix):
			// Fee details are not booked: the card ledger already carries the fees it charged.
		default:
			err = problem(f.Name, 0, "", "not a recognised SUNRATE export")
		}
		if err != nil {
			problems = append(problems, asProblems(f.Name, err)...)
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	// Statements are independent per currency; a fixed order keeps the output identical for any upload order.
	sort.SliceStable(in.statements, func(i, j int) bool { return in.statements[i].currency < in.statements[j].currency })
	result := &Result{Converter: sunratePlatform, Accounts: map[string]string{}}
	card, err := s.cardLines(&in, result)
	if err != nil {
		return nil, err
	}
	cardCount := len(result.Lines)
	if err = s.statementLines(&in, result, card); err != nil {
		return nil, err
	}
	// Payment lines first, so a transfer reads "out of payment, into card" when both sides share a timestamp.
	result.Lines = append(result.Lines[cardCount:], result.Lines[:cardCount]...)
	return result, nil
}

func asProblems(file string, err error) Problems {
	if p, ok := err.(Problems); ok {
		return p
	}
	return problem(file, 0, "", "%v", err)
}

func readSunrateRecords(f SourceFile, columns []string, each func(map[string]string)) error {
	t, err := readTable(f)
	if err != nil {
		return err
	}
	for _, c := range columns {
		if t.column(c) < 0 {
			return problem(f.Name, 1, c, "missing column")
		}
	}
	for _, row := range t.rows {
		each(t.record(row))
	}
	return nil
}

func readSunrateStatement(f SourceFile) (*sunrateStatement, error) {
	rows, err := pdfRows(f.Data)
	if err != nil {
		return nil, problem(f.Name, 0, "", "%v", err)
	}
	st := &sunrateStatement{file: f.Name}
	nameX := -1.0
	var nameParts []string
	headerDone := false
	for i, r := range rows {
		cells := r.texts()
		switch {
		case len(cells) >= 2 && cells[0] == "Account ID :":
			st.accountID = cells[1]
			for _, c := range r.Cells {
				if c.Text == "Account Name :" {
					nameX = c.X
				}
			}
		case len(cells) >= 2 && cells[0] == "Currency :":
			st.currency = cells[1]
			headerDone = true
		case len(cells) == 4 && strings.HasPrefix(cells[0], "Opening Balance On"):
			m := statementClosing.FindStringSubmatch(cells[2])
			if m == nil {
				return nil, problem(f.Name, 0, "", "closing balance not found")
			}
			if st.closingAt, err = time.ParseInLocation("Jan 2 2006", m[1], sunrateZone); err != nil {
				return nil, problem(f.Name, 0, "", "unreadable closing date %q", m[1])
			}
			// The closing balance is the balance at the end of that day.
			st.closingAt = st.closingAt.Add(24*time.Hour - time.Second)
			if st.closing, err = statementMoney(cells[3]); err != nil {
				return nil, problem(f.Name, 0, "", "unreadable closing balance %q", cells[3])
			}
		case len(cells) == 6 && statementDate.MatchString(cells[4]) && statementAmount.MatchString(cells[2]):
			if cells[1] != st.currency {
				return nil, problem(f.Name, i+1, "Currency", "row currency %s differs from statement currency %s", cells[1], st.currency)
			}
			st.rows = append(st.rows, sunrateStatementRow{id: cells[0], amount: cells[2], balance: cells[3], date: cells[4], description: cells[5], row: i + 1})
		case !headerDone && nameX >= 0 && r.Page == 1:
			for _, c := range r.Cells {
				if c.X > nameX+50 {
					nameParts = append(nameParts, c.Text)
				}
			}
		}
		// The account name is printed beside its label, starting above the label row.
		if !headerDone && nameX < 0 && r.Page == 1 && len(cells) == 1 && i > 0 && cells[0] != "SUNRATE STATEMENT OF PAYMENT BALANCE" {
			nameParts = append(nameParts, cells[0])
		}
	}
	st.accountName = strings.Join(nameParts, " ")
	if st.accountID == "" || !currencyPattern.MatchString(st.currency) || st.closingAt.IsZero() {
		return nil, problem(f.Name, 0, "", "not a SUNRATE Statement of Payment Balance")
	}
	return st, nil
}

func statementMoney(raw string) (decimal.Decimal, error) {
	return decimal.NewFromString(strings.ReplaceAll(strings.TrimPrefix(raw, "+"), ",", ""))
}

func sunrateTime(raw string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(raw), sunrateZone)
}

// cardLines converts the card balance ledger and returns the transfers into the card balance so the payment
// statements can pair with them.
func (sunrate) cardLines(in *sunrateInputs, result *Result) ([]int, error) {
	var transfers []int
	for _, f := range in.cardFiles {
		m := sunrateAccountFile.FindStringSubmatch(f.Name)
		if m == nil {
			return nil, problem(f.Name, 0, "", "file name does not carry the account number")
		}
		t, err := readTable(f)
		if err != nil {
			return nil, err
		}
		for _, c := range sunrateCardColumns {
			if t.column(c) < 0 {
				return nil, problem(f.Name, 1, c, "missing column")
			}
		}
		var problems Problems
		start := len(result.Lines)
		// The export lists the newest movement first.
		for i := len(t.rows) - 1; i >= 0; i-- {
			row := i + 2
			r := t.record(t.rows[i])
			bad := func(column, format string, args ...any) {
				problems = append(problems, Problem{File: f.Name, Row: row, Column: column, Message: fmt.Sprintf(format, args...)})
			}
			kind, ok := sunrateCardTypes[r["Transaction Type"]]
			if !ok {
				bad("Transaction Type", "unsupported type %q", r["Transaction Type"])
				continue
			}
			at, err := sunrateTime(r["Transaction Time (GMT+8:00)"])
			if err != nil {
				bad("Transaction Time (GMT+8:00)", "unreadable time")
				continue
			}
			l := Line{Platform: sunratePlatform, AccountRef: m[1], Wallet: sunrateCard, ExternalID: r["Serial No."], OccurredAt: at,
				Type: kind, Currency: r["Currency"], Source: r, File: f.Name, Row: row}
			if l.Amount, err = Money(r["Transaction Amount"], l.Currency); err != nil {
				bad("Transaction Amount", "%s", err.Error())
				continue
			}
			balance, err := Money(r["Balance"], l.Currency)
			if err != nil {
				bad("Balance", "%s", err.Error())
				continue
			}
			l.BalanceAfter = &balance
			switch kind {
			case Consume:
				l.CardRef = r["Card ID"]
				if a := in.auth[r["Card ID"]+"|"+r["Transaction Time (GMT+8:00)"]]; a != nil {
					l.CardLast4 = lastDigits(a["Card No."])
					l.CardName, l.Merchant, l.RelatedID = a["Card Nickname"], a["Merchant Name"], a["Transaction ID"]
					if a["Transaction Currency"] != a["Billing Currency"] {
						if original, parseErr := Money(strings.TrimPrefix(a["Transaction Amount"], "-"), a["Transaction Currency"]); parseErr == nil && original.IsPositive() {
							l.OriginalAmount, l.OriginalCurrency = &original, a["Transaction Currency"]
						}
					}
				}
			case Fee:
				l.Remark = "授权费（平台按日汇总）"
			}
			if name := r["Account Name"]; name != "" {
				result.Accounts[sunratePlatform+"/"+m[1]] = name
			}
			result.Lines = append(result.Lines, l)
		}
		if len(problems) > 0 {
			return nil, problems
		}
		if n := len(result.Lines); n > start {
			last := result.Lines[n-1]
			result.Balances = append(result.Balances, Balance{Key: last.Key(), Amount: *last.BalanceAfter, AsOf: last.OccurredAt, Source: f.Name})
		}
	}
	for i := range result.Lines {
		if result.Lines[i].Type == TransferIn {
			transfers = append(transfers, i)
		}
	}
	return transfers, nil
}

func lastDigits(masked string) string {
	digits := strings.TrimLeft(masked, "*")
	if len(digits) >= 4 && last4Pattern.MatchString(digits[len(digits)-4:]) {
		return digits[len(digits)-4:]
	}
	return ""
}

func (sunrate) statementLines(in *sunrateInputs, result *Result, cardTransfers []int) error {
	used := map[int]bool{}
	var problems Problems
	for _, st := range in.statements {
		for _, l := range result.Lines {
			if l.AccountRef != st.accountID {
				return problem(st.file, 0, "", "account %s differs from card ledger account %s", st.accountID, l.AccountRef)
			}
		}
		key := sunratePlatform + "/" + st.accountID
		if st.accountName != "" {
			result.Accounts[key] = st.accountName
		}
		var previous time.Time
		var last *decimal.Decimal
		for i := len(st.rows) - 1; i >= 0; i-- {
			row := st.rows[i]
			bad := func(column, format string, args ...any) {
				problems = append(problems, Problem{File: st.file, Row: row.row, Column: column, Message: fmt.Sprintf(format, args...)})
			}
			kind, ok := sunrateStatementTypes[row.description]
			if !ok {
				bad("Description", "unsupported description %q", row.description)
				continue
			}
			day, err := time.ParseInLocation("2006-01-02", row.date, sunrateZone)
			if err != nil {
				bad("Date", "unreadable date")
				continue
			}
			amount, err := statementMoney(row.amount)
			balance, balanceErr := statementMoney(row.balance)
			if err != nil || balanceErr != nil {
				bad("Amount", "unreadable amount or balance")
				continue
			}
			l := Line{Platform: sunratePlatform, AccountRef: st.accountID, Wallet: sunratePayment, ExternalID: row.id, OccurredAt: day,
				Type: kind, Amount: amount, Currency: st.currency, BalanceAfter: &balance, File: st.file, Row: row.row,
				Source: map[string]string{"Transaction ID": row.id, "Amount": row.amount, "Balance": row.balance, "Date": row.date, "Description": row.description}}
			precise := time.Time{}
			switch kind {
			case FundingIn:
				if c := in.collections[row.id]; c != nil {
					net, netErr := Money(c["实际入账金额"], c["币种"])
					fee, feeErr := Money(c["手续费"], c["币种"])
					if netErr != nil || feeErr != nil || c["币种"] != st.currency || !net.Equal(amount) {
						bad("Amount", "collection record %s does not match the statement", row.id)
						continue
					}
					l.FeeAmount = &fee
					l.Remark = fmt.Sprintf("入金毛额 %s，汇款方 %s", c["收款金额"], c["汇款方"])
					precise, _ = sunrateTime(c["提交时间"])
				}
			case FXOut, FXIn:
				l.RelatedID = row.id
				if c := in.conversions[row.id]; c != nil {
					side, sideCurrency := c["卖出金额"], c["卖出币种"]
					if kind == FXIn {
						side, sideCurrency = c["买入金额"], c["买入币种"]
					}
					expected, parseErr := Money(side, sideCurrency)
					if parseErr != nil || sideCurrency != st.currency || !expected.Equal(amount.Abs()) {
						bad("Amount", "conversion record %s does not match the statement", row.id)
						continue
					}
					l.Remark = fmt.Sprintf("购汇 %s→%s @%s", c["卖出币种"], c["买入币种"], c["汇率"])
					precise, _ = sunrateTime(c["提交时间"])
				}
			case TransferOut:
				for _, idx := range cardTransfers {
					target := &result.Lines[idx]
					if !used[idx] && target.Currency == st.currency && target.Amount.Equal(amount.Neg()) && target.OccurredAt.In(sunrateZone).Format("2006-01-02") == row.date {
						used[idx] = true
						l.RelatedID, target.RelatedID = target.ExternalID, l.ExternalID
						precise = target.OccurredAt
						break
					}
				}
				l.Remark = "划转到卡余额"
			}
			// Detail records give the exact time on the statement date; keep the statement order within the day.
			if !precise.IsZero() && precise.In(sunrateZone).Format("2006-01-02") == row.date {
				l.OccurredAt = precise
			}
			if l.OccurredAt.Before(previous) {
				l.OccurredAt = previous
			}
			previous = l.OccurredAt
			last = l.BalanceAfter
			result.Lines = append(result.Lines, l)
		}
		if last != nil && !last.Equal(st.closing) {
			problems = append(problems, Problem{File: st.file, Message: "last balance " + last.String() + " differs from closing balance " + st.closing.String()})
		}
		result.Balances = append(result.Balances, Balance{Key: Key{sunratePlatform, st.accountID, sunratePayment, st.currency}, Amount: st.closing, AsOf: st.closingAt, Source: st.file})
	}
	for _, idx := range cardTransfers {
		result.Lines[idx].Remark = "来自收付款余额"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}
