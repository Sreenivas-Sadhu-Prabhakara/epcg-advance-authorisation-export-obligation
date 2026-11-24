package eo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar day with no clock time and no time zone.
//
// Every period in this domain is a period of calendar days, months or years
// counted from a day printed on a licence. An export obligation period of six
// years from 29 February 2024 ends on a particular day, and which day that is
// must not depend on anybody's clock, locale or daylight-saving rules. Holding
// the day as three integers makes that impossible to get wrong by accident.
//
// The zero Date means "not given". That is a real state here: an authorisation
// may not yet be redeemed, an import may not yet be cleared, and a period end
// date may not be endorsed on the licence at all.
type Date struct {
	Y int `json:"y"`
	M int `json:"m"`
	D int `json:"d"`
}

// NewDate builds a Date and reports whether it names a day that exists.
// 31 April and 29 February 2023 do not.
func NewDate(y, m, d int) (Date, bool) {
	if y < 1 || m < 1 || m > 12 || d < 1 {
		return Date{}, false
	}
	if d > DaysInMonth(y, m) {
		return Date{}, false
	}
	return Date{y, m, d}, true
}

// Zero reports whether the date is unset.
func (d Date) Zero() bool { return d.Y == 0 && d.M == 0 && d.D == 0 }

// Valid reports whether the date names a real calendar day.
func (d Date) Valid() bool {
	_, ok := NewDate(d.Y, d.M, d.D)
	return ok
}

// String renders the date as yyyy-mm-dd. An unset date renders as "".
func (d Date) String() string {
	if d.Zero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Y, d.M, d.D)
}

// Human renders the date the way an authorisation prints it: 29 February 2024.
func (d Date) Human() string {
	if d.Zero() || !d.Valid() {
		return "—"
	}
	return fmt.Sprintf("%d %s %d", d.D, time.Month(d.M).String(), d.Y)
}

// MarshalJSON writes a yyyy-mm-dd string, or "" when unset, so the data file
// reads as plainly as the screen shows it.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON accepts a yyyy-mm-dd string, an empty string or null.
func (d *Date) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("a date must be written as yyyy-mm-dd in quotes: %w", err)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// ParseDate reads the date forms that appear on Indian trade paperwork: the ISO
// form 2024-02-29, and the day-first forms 29/02/2024 and 29.02.2024 that DGFT
// authorisations, shipping bills and bills of entry all print. A slashed or
// dotted date is read DAY FIRST, which is how it is written on every one of
// those documents; the reading is applied consistently and stated in the
// documentation rather than guessed at per value.
func ParseDate(s string) (Date, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Date{}, nil
	}

	var sep string
	switch {
	case strings.Count(t, "-") == 2:
		sep = "-"
	case strings.Count(t, "/") == 2:
		sep = "/"
	case strings.Count(t, ".") == 2:
		sep = "."
	default:
		return Date{}, fmt.Errorf("%q is not a date — write it as yyyy-mm-dd, for example 2024-02-29", s)
	}

	parts := strings.Split(t, sep)
	nums := make([]int, 3)
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return Date{}, fmt.Errorf("%q is not a date — write it as yyyy-mm-dd, for example 2024-02-29", s)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Date{}, fmt.Errorf("%q is not a date — write it as yyyy-mm-dd, for example 2024-02-29", s)
		}
		parts[i], nums[i] = p, n
	}

	var y, m, dd int
	switch {
	case len(parts[0]) == 4:
		y, m, dd = nums[0], nums[1], nums[2] // yyyy-mm-dd
	case len(parts[2]) == 4:
		dd, m, y = nums[0], nums[1], nums[2] // dd/mm/yyyy, read day first
	default:
		return Date{}, fmt.Errorf("%q needs a four-digit year — write it as yyyy-mm-dd, for example 2024-02-29", s)
	}

	out, ok := NewDate(y, m, dd)
	if !ok {
		return Date{}, fmt.Errorf("%s is not a day that exists", s)
	}
	return out, nil
}

// MustDate is for tests and for the worked examples. It panics on a day that
// does not exist, which in those two callers is a programming mistake.
func MustDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// DaysInMonth returns the length of a month, leap years included.
func DaysInMonth(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if LeapYear(y) {
			return 29
		}
		return 28
	}
	return 0
}

// LeapYear applies the Gregorian rule: every fourth year, except centuries,
// except every fourth century. 1900 was not a leap year; 2000 was.
func LeapYear(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func (d Date) time() time.Time {
	return time.Date(d.Y, time.Month(d.M), d.D, 0, 0, 0, 0, time.UTC)
}

func fromTime(t time.Time) Date { return Date{t.Year(), int(t.Month()), t.Day()} }

// AddDays moves a date by a number of calendar days; n may be negative.
func (d Date) AddDays(n int) Date {
	if d.Zero() {
		return d
	}
	return fromTime(d.time().AddDate(0, 0, n))
}

// AddMonths moves a date by whole calendar months, CLAMPED to the last day of
// the target month: 31 January plus one month is 28 February, or 29 February in
// a leap year — never 2 or 3 March. Go's own AddDate rolls over instead, which
// would push an obligation deadline into the following month and is the single
// most common date bug in software that counts statutory periods.
func (d Date) AddMonths(n int) Date {
	if d.Zero() {
		return d
	}
	// Move from the first of the month so no rollover can happen, then clamp.
	first := time.Date(d.Y, time.Month(d.M), 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
	y, m := first.Year(), int(first.Month())
	day := d.D
	if last := DaysInMonth(y, m); day > last {
		day = last
	}
	return Date{y, m, day}
}

// AddYears moves a date by whole years, clamping 29 February in a leap year to
// 28 February in a year that has no 29th: a licence issued on 29 February 2024
// with a six-year obligation period runs to 28 February 2030, because 29
// February 2030 does not exist.
func (d Date) AddYears(n int) Date { return d.AddMonths(12 * n) }

// Before, After and Equal compare two days.
func (d Date) Before(o Date) bool { return d.compare(o) < 0 }
func (d Date) After(o Date) bool  { return d.compare(o) > 0 }
func (d Date) Equal(o Date) bool  { return d.compare(o) == 0 }

// OnOrBefore and OnOrAfter read the way a deadline does: a period "up to and
// including" a date still has that date inside it.
func (d Date) OnOrBefore(o Date) bool { return d.compare(o) <= 0 }
func (d Date) OnOrAfter(o Date) bool  { return d.compare(o) >= 0 }

func (d Date) compare(o Date) int {
	switch {
	case d.Y != o.Y:
		return sign(d.Y - o.Y)
	case d.M != o.M:
		return sign(d.M - o.M)
	case d.D != o.D:
		return sign(d.D - o.D)
	}
	return 0
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// DaysBetween counts whole calendar days from d to o, positive when o is later.
// The day d itself is not counted, so 1 March to 31 March is 30 days.
func DaysBetween(d, o Date) int {
	if d.Zero() || o.Zero() {
		return 0
	}
	return int(o.time().Sub(d.time()) / (24 * time.Hour))
}

// MonthsBetween counts whole calendar months from d to o, so that a part month
// at the end is not counted. 31 January to 28 February is one month; 31 January
// to 27 February is none.
func MonthsBetween(d, o Date) int {
	if d.Zero() || o.Zero() || !o.After(d) {
		return 0
	}
	n := 0
	for d.AddMonths(n + 1).OnOrBefore(o) {
		n++
	}
	return n
}

// Window is a closed interval of calendar days, both ends included. Every
// obligation period and every block in this domain is one of these, and the
// inclusivity of both ends is the convention stated in the documentation.
type Window struct {
	From Date `json:"from"`
	To   Date `json:"to"`
}

// Contains reports whether a day falls inside the window, both ends included.
func (w Window) Contains(d Date) bool {
	if w.From.Zero() || w.To.Zero() || d.Zero() {
		return false
	}
	return d.OnOrAfter(w.From) && d.OnOrBefore(w.To)
}

// Days is the length of the window in days, counting both end days.
func (w Window) Days() int {
	if w.From.Zero() || w.To.Zero() || w.To.Before(w.From) {
		return 0
	}
	return DaysBetween(w.From, w.To) + 1
}

// Valid reports whether the window is a real interval.
func (w Window) Valid() bool {
	return !w.From.Zero() && !w.To.Zero() && w.From.Valid() && w.To.Valid() && w.To.OnOrAfter(w.From)
}

func (w Window) String() string {
	if !w.Valid() {
		return ""
	}
	return w.From.Human() + " to " + w.To.Human()
}

// FinancialYear is an Indian financial year, named by the calendar year it
// begins in: FinancialYear{2024} is 1 April 2024 to 31 March 2025, written
// "2024-25".
//
// This matters because the two clocks in an export obligation do not run
// together. The obligation period and its blocks are counted from the date the
// authorisation was issued, so they turn over on an anniversary. The average
// export obligation is measured against a LICENSING YEAR, which under the
// Foreign Trade Policy is the financial year — 1 April to 31 March. A tool that
// used one clock for both would report an obligation as met in a year in which
// it was not.
type FinancialYear struct {
	StartYear int `json:"startYear"`
}

// FinancialYearOf returns the financial year a date falls in, using the 1 April
// start date fixed by the Foreign Trade (Development and Regulation) framework.
func FinancialYearOf(d Date) FinancialYear {
	if d.Zero() {
		return FinancialYear{}
	}
	if d.M >= FinancialYearStartMonth {
		return FinancialYear{d.Y}
	}
	return FinancialYear{d.Y - 1}
}

// Window returns 1 April to 31 March for the financial year.
func (fy FinancialYear) Window() Window {
	if fy.StartYear < 1 {
		return Window{}
	}
	from, _ := NewDate(fy.StartYear, FinancialYearStartMonth, FinancialYearStartDay)
	to := from.AddYears(1).AddDays(-1)
	return Window{From: from, To: to}
}

// String renders the year the way Indian filings write it: 2024-25.
func (fy FinancialYear) String() string {
	if fy.StartYear < 1 {
		return ""
	}
	return fmt.Sprintf("%d-%02d", fy.StartYear, (fy.StartYear+1)%100)
}

// Next returns the following financial year.
func (fy FinancialYear) Next() FinancialYear { return FinancialYear{fy.StartYear + 1} }

// FinancialYearsOverlapping lists every financial year any part of which falls
// inside the window, in order. A six-year obligation period beginning in
// mid-year touches seven financial years, and the first and the last of them are
// part years — which is why this returns the years rather than a count.
func FinancialYearsOverlapping(w Window) []FinancialYear {
	if !w.Valid() {
		return nil
	}
	out := []FinancialYear{}
	for fy := FinancialYearOf(w.From); fy.StartYear <= FinancialYearOf(w.To).StartYear; fy = fy.Next() {
		out = append(out, fy)
		if len(out) > 200 { // a licence cannot run two centuries; refuse to loop
			break
		}
	}
	return out
}

// Overlap returns the part of two windows that is common to both, and whether
// there is any. It is what turns a financial year into the part of that year
// which actually falls inside an obligation period.
func (w Window) Overlap(o Window) (Window, bool) {
	if !w.Valid() || !o.Valid() {
		return Window{}, false
	}
	from, to := w.From, w.To
	if o.From.After(from) {
		from = o.From
	}
	if o.To.Before(to) {
		to = o.To
	}
	if to.Before(from) {
		return Window{}, false
	}
	return Window{From: from, To: to}, true
}

// Today is the only place this package reads the clock, and only the calendar
// day, so that a report can say which day it was produced on.
func Today() Date { return fromTime(time.Now()) }
