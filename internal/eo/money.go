package eo

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// MinorUnitsPerMajor is how many minor units make one major unit of the
// currencies this tool is used with: 100 paise to the rupee, 100 cents to the
// US dollar. Both of the currencies an Indian authorisation is denominated in
// have two decimal places, so one figure serves.
const MinorUnitsPerMajor = 100

// Money is an amount of money held as a whole number of MINOR UNITS — paise for
// rupees, cents for dollars.
//
// Nothing in this package holds money as a floating-point number. A duty-saved
// figure of a few crore, multiplied by an obligation multiple and then
// apportioned over a shortfall, goes through enough arithmetic for binary
// floating point to lose paise; and the number this tool exists to produce is a
// liability that somebody may have to pay, so a paise lost here is a paise
// wrong on a demand. Every multiplication and division below is exact integer
// arithmetic with one documented rounding step at the end.
type Money int64

// Zero is the absent amount. It is distinguishable from a deliberate nil only
// by context, and every caller that needs the distinction says so in its own
// field.
const Zero Money = 0

// Major returns the amount in whole major units, truncated. It is for sorting
// and for coarse comparisons, never for arithmetic on a liability.
func (m Money) Major() int64 { return int64(m) / MinorUnitsPerMajor }

// String renders the amount in major units with two decimals and no grouping:
// 1234567.89. This is the form the JSON API and the data file use.
func (m Money) String() string {
	neg := m < 0
	v := int64(m)
	if neg {
		v = -v
	}
	out := fmt.Sprintf("%d.%02d", v/MinorUnitsPerMajor, v%MinorUnitsPerMajor)
	if neg {
		return "-" + out
	}
	return out
}

// Indian renders the amount in the digit grouping used on Indian invoices and
// in DGFT filings: 12,34,567.89 — thousand, then lakh, then crore — rather than
// 1,234,567.89.
func (m Money) Indian() string {
	s := m.String()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	dot := strings.IndexByte(s, '.')
	whole, frac := s[:dot], s[dot:]
	out := groupIndian(whole) + frac
	if neg {
		return "-" + out
	}
	return out
}

// groupIndian inserts a separator after the last three digits and every two
// digits before that.
func groupIndian(whole string) string {
	if len(whole) <= 3 {
		return whole
	}
	head, tail := whole[:len(whole)-3], whole[len(whole)-3:]
	var parts []string
	for len(head) > 2 {
		parts = append([]string{head[len(head)-2:]}, parts...)
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append([]string{head}, parts...)
	}
	return strings.Join(parts, ",") + "," + tail
}

// Words renders a large amount the way an Indian exporter reads it aloud —
// "₹1.24 crore", "₹18.50 lakh" — so a headline figure can be taken in at a
// glance. The precise figure is always shown beside it; this is never the only
// place a number appears.
func (m Money) Words() string {
	neg := m < 0
	v := int64(m)
	if neg {
		v = -v
	}
	const lakh = int64(100000) * MinorUnitsPerMajor
	const crore = int64(10000000) * MinorUnitsPerMajor
	var out string
	switch {
	case v >= crore:
		out = trimZeros(fmt.Sprintf("%.2f", float64(v)/float64(crore))) + " crore"
	case v >= lakh:
		out = trimZeros(fmt.Sprintf("%.2f", float64(v)/float64(lakh))) + " lakh"
	default:
		return Money(v).signed(neg).Indian()
	}
	if neg {
		return "-" + out
	}
	return out
}

func (m Money) signed(neg bool) Money {
	if neg {
		return -m
	}
	return m
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// MarshalJSON writes the amount as a plain decimal string in major units, so
// the data file is readable and no JSON parser can turn paise into a float.
func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// UnmarshalJSON accepts the decimal string form, a bare JSON number of major
// units, an empty string or null.
func (m *Money) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*m = 0
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		parsed, perr := ParseMoney(s)
		if perr != nil {
			return perr
		}
		*m = parsed
		return nil
	}
	// A bare number: accept it, but read it through the same decimal parser so
	// that 12.345 is refused rather than silently truncated.
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return fmt.Errorf("an amount must be a number or a quoted decimal: %w", err)
	}
	parsed, err := ParseMoney(n.String())
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// ParseMoney reads an amount as it is written on paperwork: with a currency
// symbol or code, with Indian or Western thousands separators, with or without
// decimals. It refuses anything it cannot read in full rather than taking the
// part it understood, and it refuses more than two decimal places rather than
// rounding somebody's figure without telling them.
func ParseMoney(s string) (Money, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, nil
	}

	core, ok := stripCurrencyLabels(t)
	if !ok {
		return 0, fmt.Errorf("%q is not an amount", s)
	}

	var b strings.Builder
	for _, r := range core {
		switch {
		case r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		case r == ',', r == ' ', r == ' ', r == ' ', r == '\'':
			// a thousands separator, in any of the groupings used in practice
		default:
			return 0, fmt.Errorf("%q is not an amount", s)
		}
	}

	cleaned := b.String()
	neg := strings.HasPrefix(cleaned, "-")
	cleaned = strings.TrimPrefix(cleaned, "-")
	if strings.Contains(cleaned, "-") {
		return 0, fmt.Errorf("%q is not an amount", s)
	}

	whole, frac := cleaned, ""
	if i := strings.IndexByte(cleaned, '.'); i >= 0 {
		whole, frac = cleaned[:i], cleaned[i+1:]
		if strings.Contains(frac, ".") {
			return 0, fmt.Errorf("%q is not an amount", s)
		}
	}
	if whole == "" {
		whole = "0"
	}
	if whole == "" && frac == "" {
		return 0, fmt.Errorf("%q is not an amount", s)
	}
	if len(frac) > 2 {
		return 0, fmt.Errorf("%q has more than two decimal places; amounts are in rupees and paise", s)
	}
	for len(frac) < 2 {
		frac += "0"
	}

	major, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", s)
	}
	minor, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", s)
	}
	if major > (1<<62)/MinorUnitsPerMajor {
		return 0, fmt.Errorf("%q is larger than this tool handles", s)
	}

	v := Money(major*MinorUnitsPerMajor + minor)
	if neg {
		v = -v
	}
	return v, nil
}

// stripCurrencyLabels removes a currency symbol or code written before or after
// the figure, together with the full stop that abbreviates it — "Rs. 5,000",
// "₹1,25,00,000", "42,00,000.75 INR" — and refuses anything else outside the
// figure rather than reading the part it understood.
//
// The abbreviating full stop is why this cannot be done by filtering
// characters: "Rs. 5,000" filtered character by character leaves ".5000", which
// is four decimal places of nothing. The label has to be removed as a run,
// before any decimal point is looked for.
func stripCurrencyLabels(t string) (string, bool) {
	r := []rune(t)
	start := 0
	// A figure may begin with a digit, a sign or a bare decimal point. Anything
	// else at the front is a label, and is skipped up to the first digit or
	// sign — never up to a point, which at that position belongs to the label.
	if !(isDigit(r[0]) || r[0] == '-' || r[0] == '.') {
		for start < len(r) && !(isDigit(r[start]) || r[start] == '-') {
			if !isCurrencyLabelRune(r[start]) {
				return "", false
			}
			start++
		}
	}
	end := len(r)
	for end > start && !isDigit(r[end-1]) {
		if !isCurrencyLabelRune(r[end-1]) {
			return "", false
		}
		end--
	}
	core := string(r[start:end])
	if !strings.ContainsFunc(core, isDigit) {
		return "", false
	}
	return core, true
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// isCurrencyLabelRune reports whether a character may appear in the currency
// label beside a figure: a letter of a code such as INR or Rs, one of the
// symbols those codes are written as, the full stop that abbreviates them, or
// the space between label and figure.
func isCurrencyLabelRune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		return true
	case r == '₹', r == '$', r == '€', r == '£', r == '¥', r == '₱':
		return true
	case r == '.', r == ' ', r == ' ', r == ' ':
		return true
	}
	return false
}

// MustMoney is for tests and the worked examples.
func MustMoney(s string) Money {
	m, err := ParseMoney(s)
	if err != nil {
		panic(err)
	}
	return m
}

// MulDiv returns m × num / den as a whole number of minor units, rounded HALF
// AWAY FROM ZERO — the commercial rounding a customs demand uses. The
// multiplication is done in arbitrary precision so that a duty of several crore
// multiplied by an interest rate and a day count cannot overflow on the way to
// a result that fits comfortably.
//
// A zero denominator returns zero rather than panicking: every caller here
// divides by a figure the user supplied, and "nothing to divide by" is a state
// those callers report rather than crash on.
func (m Money) MulDiv(num, den int64) Money {
	if den == 0 {
		return 0
	}
	n := new(big.Int).Mul(big.NewInt(int64(m)), big.NewInt(num))
	d := big.NewInt(den)
	return Money(divRoundHalfAway(n, d).Int64())
}

// divRoundHalfAway divides and rounds half away from zero. big.Int.QuoRem
// truncates toward zero, so the remainder carries the sign of the numerator and
// the comparison below is on magnitudes.
func divRoundHalfAway(n, d *big.Int) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, d, r)
	if r.Sign() == 0 {
		return q
	}
	twice := new(big.Int).Abs(new(big.Int).Mul(r, big.NewInt(2)))
	if twice.Cmp(new(big.Int).Abs(d)) >= 0 {
		if (n.Sign() < 0) != (d.Sign() < 0) {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}

// Bips is a rate or a proportion in BASIS POINTS: one basis point is one
// hundredth of one percent, so 10,000 bips is 100% and 60,000 bips is six
// times.
//
// Every rate and multiple in the Foreign Trade Policy is held this way rather
// than as a float, because they are not all whole numbers — an export
// obligation reduced to three quarters of the normal multiple is 4.5 times, and
// 4.5 has no exact binary representation. 45,000 bips does.
type Bips int64

// Percent builds a rate from whole and fractional percent, e.g. Percent(15, 0)
// for 15% and Percent(0, 50) for half a percent.
func Percent(whole, hundredthsOfPercent int64) Bips {
	return Bips(whole*100 + hundredthsOfPercent)
}

// Times builds a multiple from hundredths: Times(600) is six times, Times(450)
// is four and a half times.
func Times(hundredths int64) Bips { return Bips(hundredths * 100) }

// Of applies the rate to an amount.
func (b Bips) Of(m Money) Money { return m.MulDiv(int64(b), 10000) }

// Pct renders the rate as a percentage, without a trailing ".00": "15%",
// "0.5%", "33.33%".
func (b Bips) Pct() string { return trimZeros(fmt.Sprintf("%.2f", float64(b)/100)) + "%" }

// Multiple renders the rate as a multiple of one: "6 times", "4.5 times".
func (b Bips) Multiple() string {
	return trimZeros(fmt.Sprintf("%.2f", float64(b)/10000)) + " times"
}

// Proportion returns part as a proportion of whole, in basis points, rounded
// half away from zero. A zero whole returns zero: there is no proportion of
// nothing, and reporting 100% for it would be a lie in the dangerous direction.
func Proportion(part, whole Money) Bips {
	if whole == 0 {
		return 0
	}
	n := new(big.Int).Mul(big.NewInt(int64(part)), big.NewInt(10000))
	return Bips(divRoundHalfAway(n, big.NewInt(int64(whole))).Int64())
}

// ProportionOfCount returns part as a proportion of whole for plain counts.
func ProportionOfCount(part, whole int) Bips {
	if whole == 0 {
		return 0
	}
	return Bips(divRoundHalfAway(big.NewInt(int64(part)*10000), big.NewInt(int64(whole))).Int64())
}

// MeetsRate reports whether part is at least the given rate of whole, compared
// EXACTLY rather than on a rounded percentage.
//
// This distinction decides money. A value addition of 14.9999% rounds to 15.00%
// for display, and comparing the rounded figure against a 15% minimum would
// report a requirement as met that is not — a liability understated, which is
// the dangerous direction to be wrong in. The comparison is therefore made on
// the cross-multiplied integers, where no rounding happens at all.
func MeetsRate(part, whole Money, rate Bips) bool {
	if whole <= 0 {
		return false
	}
	left := new(big.Int).Mul(big.NewInt(int64(part)), big.NewInt(10000))
	right := new(big.Int).Mul(big.NewInt(int64(rate)), big.NewInt(int64(whole)))
	return left.Cmp(right) >= 0
}

// Quantity is a physical quantity held as a whole number of THOUSANDTHS of its
// unit, so that 1.234 MT and 0.005 kg are both exact. An Advance Authorisation
// obligation is stated in a quantity of the resultant product as well as a
// value, and a quantity rounded in software is a quantity short on an
// authorisation.
type Quantity int64

// QuantityScale is the number of thousandths in one unit.
const QuantityScale = 1000

// String renders the quantity with up to three decimals and no trailing zeros.
func (q Quantity) String() string {
	neg := q < 0
	v := int64(q)
	if neg {
		v = -v
	}
	out := trimZeros(fmt.Sprintf("%d.%03d", v/QuantityScale, v%QuantityScale))
	if neg {
		return "-" + out
	}
	return out
}

// MarshalJSON writes the quantity as a decimal string.
func (q Quantity) MarshalJSON() ([]byte, error) { return json.Marshal(q.String()) }

// UnmarshalJSON accepts a decimal string, a bare number, "" or null.
func (q *Quantity) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*q = 0
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		var n json.Number
		if err2 := json.Unmarshal(raw, &n); err2 != nil {
			return fmt.Errorf("a quantity must be a number or a quoted decimal: %w", err2)
		}
		s = n.String()
	}
	parsed, err := ParseQuantity(s)
	if err != nil {
		return err
	}
	*q = parsed
	return nil
}

// ParseQuantity reads a quantity with up to three decimal places. It refuses a
// fourth rather than rounding it away.
func ParseQuantity(s string) (Quantity, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, nil
	}
	var b strings.Builder
	for _, r := range t {
		switch {
		case r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		case r == ',', r == ' ', r == ' ', r == '\'':
			// a separator
		default:
			return 0, fmt.Errorf("%q is not a quantity", s)
		}
	}
	cleaned := b.String()
	neg := strings.HasPrefix(cleaned, "-")
	cleaned = strings.TrimPrefix(cleaned, "-")
	if strings.Contains(cleaned, "-") {
		return 0, fmt.Errorf("%q is not a quantity", s)
	}
	whole, frac := cleaned, ""
	if i := strings.IndexByte(cleaned, '.'); i >= 0 {
		whole, frac = cleaned[:i], cleaned[i+1:]
		if strings.Contains(frac, ".") {
			return 0, fmt.Errorf("%q is not a quantity", s)
		}
	}
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 3 {
		return 0, fmt.Errorf("%q has more than three decimal places", s)
	}
	for len(frac) < 3 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a quantity", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a quantity", s)
	}
	if w > (1<<62)/QuantityScale {
		return 0, fmt.Errorf("%q is larger than this tool handles", s)
	}
	v := Quantity(w*QuantityScale + f)
	if neg {
		v = -v
	}
	return v, nil
}

// MustQuantity is for tests and the worked examples.
func MustQuantity(s string) Quantity {
	q, err := ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

// ProportionOfQuantity returns part as a proportion of whole, in basis points.
func ProportionOfQuantity(part, whole Quantity) Bips {
	if whole == 0 {
		return 0
	}
	n := new(big.Int).Mul(big.NewInt(int64(part)), big.NewInt(10000))
	return Bips(divRoundHalfAway(n, big.NewInt(int64(whole))).Int64())
}

// plural picks a word form for a count and prints the count with it.
func plural(n int, one, many string) string {
	if n == 1 || n == -1 {
		return strconv.Itoa(n) + " " + one
	}
	return strconv.Itoa(n) + " " + many
}
