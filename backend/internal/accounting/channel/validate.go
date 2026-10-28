package channel

import (
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// Validate checks rules that need only the lines of this upload. Pairing that may involve lines imported earlier
// is checked when booking.
func Validate(lines []Line) error {
	var problems Problems
	add := func(l *Line, column, message string) {
		if len(problems) < 50 {
			problems = append(problems, Problem{File: l.File, Row: l.Row, Column: column, Message: message})
		}
	}
	type dedupe struct {
		Key
		id string
	}
	seen := map[dedupe]bool{}
	withBalance := map[Key]int{}
	count := map[Key]int{}
	for i := range lines {
		l := &lines[i]
		sign, known := Sign[l.Type]
		switch {
		case !known:
			add(l, "type", "unknown type")
		case l.Amount.IsZero():
			add(l, "amount", "must not be zero")
		case sign > 0 && !l.Amount.IsPositive(), sign < 0 && !l.Amount.IsNegative():
			add(l, "amount", "sign does not match type "+l.Type)
		}
		digits := int32(payment.CurrencyMaxFractionDigits(l.Currency))
		if !l.Amount.Truncate(digits).Equal(l.Amount) {
			add(l, "amount", errPrecision.Error())
		}
		if l.FeeAmount != nil && l.FeeAmount.IsNegative() {
			add(l, "fee_amount", "must not be negative")
		}
		if (l.Type == Consume || l.Type == Refund) && l.CardRef == "" {
			add(l, "card_ref", "required for card spend and refunds")
		}
		if l.Type == Adjustment && l.Remark == "" {
			add(l, "remark", "required for adjustments")
		}
		if (l.Type == FXIn || l.Type == FXOut) && l.RelatedID == "" {
			add(l, "related_id", "required to pair the two sides of a conversion")
		}
		if l.OriginalAmount != nil && (l.OriginalCurrency == "" || !l.OriginalAmount.IsPositive()) {
			add(l, "original_amount", "must be positive and come with original_currency")
		}
		d := dedupe{l.Key(), l.ExternalID}
		if seen[d] {
			add(l, "external_id", "duplicated in this upload")
		}
		seen[d] = true
		count[l.Key()]++
		if l.BalanceAfter != nil {
			withBalance[l.Key()]++
		}
	}
	for key, n := range withBalance {
		if n != count[key] {
			problems = append(problems, Problem{Message: "balance_after must be given on all or none of the lines of " + key.String()})
		}
	}
	if len(problems) == 0 {
		problems = append(problems, continuity(lines)...)
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// Ordered returns line indexes per balance, sorted by time and then upload order.
func Ordered(lines []Line) map[Key][]int {
	out := map[Key][]int{}
	for i := range lines {
		out[lines[i].Key()] = append(out[lines[i].Key()], i)
	}
	for _, idx := range out {
		sort.SliceStable(idx, func(a, b int) bool { return lines[idx[a]].OccurredAt.Before(lines[idx[b]].OccurredAt) })
	}
	return out
}

func continuity(lines []Line) Problems {
	var problems Problems
	for _, idx := range Ordered(lines) {
		for n := 1; n < len(idx); n++ {
			prev, cur := &lines[idx[n-1]], &lines[idx[n]]
			if prev.BalanceAfter == nil || cur.BalanceAfter == nil {
				break
			}
			if !prev.BalanceAfter.Add(cur.Amount).Equal(*cur.BalanceAfter) {
				problems = append(problems, Problem{File: cur.File, Row: cur.Row, Column: "balance_after",
					Message: "previous balance " + prev.BalanceAfter.String() + " plus amount " + cur.Amount.String() + " does not equal " + cur.BalanceAfter.String()})
				if len(problems) >= 20 {
					return problems
				}
			}
		}
	}
	return problems
}
