package eo

import "testing"

func TestNewDateRejectsDaysThatDoNotExist(t *testing.T) {
	cases := []struct {
		name    string
		y, m, d int
		ok      bool
	}{
		{"an ordinary day", 2026, 11, 24, true},
		{"the first day of a financial year", 2026, 4, 1, true},
		{"the last day of a financial year", 2027, 3, 31, true},
		{"29 February in a leap year", 2024, 2, 29, true},
		{"29 February in a common year", 2023, 2, 29, false},
		{"29 February in 2100, not a leap year", 2100, 2, 29, false},
		{"29 February in 2000, a leap year", 2000, 2, 29, true},
		{"31 April", 2026, 4, 31, false},
		{"31 June", 2026, 6, 31, false},
		{"month zero", 2026, 0, 10, false},
		{"month thirteen", 2026, 13, 10, false},
		{"day zero", 2026, 5, 0, false},
		{"year zero", 0, 5, 10, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NewDate(c.y, c.m, c.d)
			if ok != c.ok {
				t.Fatalf("NewDate(%d,%d,%d) ok = %v, want %v", c.y, c.m, c.d, ok, c.ok)
			}
			if ok && (got.Y != c.y || got.M != c.m || got.D != c.d) {
				t.Fatalf("NewDate(%d,%d,%d) = %v", c.y, c.m, c.d, got)
			}
			if !ok && !got.Zero() {
				t.Fatalf("a refused date should come back zero, got %v", got)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		err  bool
	}{
		{"iso", "2026-11-24", "2026-11-24", false},
		{"iso with spaces", "  2026-11-24  ", "2026-11-24", false},
		{"day first with slashes", "24/11/2026", "2026-11-24", false},
		{"day first with dots, as a bill of entry prints it", "24.11.2026", "2026-11-24", false},
		{"day first with dashes", "24-11-2026", "2026-11-24", false},
		{"a leap day", "29/02/2024", "2024-02-29", false},
		{"empty is not an error, it is absent", "", "", false},
		{"a leap day in a common year", "29/02/2023", "", true},
		{"two-digit year refused rather than guessed", "24/11/26", "", true},
		{"not a date at all", "sometime in November", "", true},
		{"one separator only", "2026-11", "", true},
		{"an empty component", "2026--24", "", true},
		{"letters inside", "2026-1a-24", "", true},
		{"31 April", "31/04/2026", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.err {
				if err == nil {
					t.Fatalf("ParseDate(%q) should have failed, got %v", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", c.in, err)
			}
			if got.String() != c.want {
				t.Fatalf("ParseDate(%q) = %q, want %q", c.in, got.String(), c.want)
			}
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"a day", `"2024-02-29"`, "2024-02-29"},
		{"empty string means absent", `""`, ""},
		{"null means absent", `null`, ""},
		{"day-first accepted on the way in", `"29/02/2024"`, "2024-02-29"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d Date
			if err := d.UnmarshalJSON([]byte(c.in)); err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if d.String() != c.want {
				t.Fatalf("unmarshal %s gave %q, want %q", c.in, d.String(), c.want)
			}
			out, err := d.MarshalJSON()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != `"`+c.want+`"` {
				t.Fatalf("marshal gave %s, want %q", out, c.want)
			}
		})
	}

	var bad Date
	if err := bad.UnmarshalJSON([]byte(`20260101`)); err == nil {
		t.Fatalf("a bare number is not a date and should be refused")
	}
}

func TestAddMonthsClampsInsteadOfRollingOver(t *testing.T) {
	cases := []struct {
		name string
		from string
		n    int
		want string
	}{
		{"31 January plus one month is 28 February in a common year", "2023-01-31", 1, "2023-02-28"},
		{"31 January plus one month is 29 February in a leap year", "2024-01-31", 1, "2024-02-29"},
		{"31 March plus one month is 30 April", "2026-03-31", 1, "2026-04-30"},
		{"30 April plus one month keeps the 30th", "2026-04-30", 1, "2026-05-30"},
		{"an 18-month Advance Authorisation period from 31 August", "2026-08-31", 18, "2028-02-29"},
		{"an 18-month period from 31 August the year before", "2025-08-31", 18, "2027-02-28"},
		{"crossing a year", "2026-11-24", 14, "2028-01-24"},
		{"backwards", "2026-03-31", -1, "2026-02-28"},
		{"backwards over a year end", "2026-01-15", -1, "2025-12-15"},
		{"zero is identity", "2024-02-29", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from := MustDate(c.from)
			want := c.want
			if want == "" {
				want = c.from
			}
			if got := from.AddMonths(c.n); got.String() != want {
				t.Fatalf("%s plus %d months = %s, want %s", c.from, c.n, got, want)
			}
		})
	}

	if got := (Date{}).AddMonths(6); !got.Zero() {
		t.Fatalf("an absent date stays absent, got %v", got)
	}
}

func TestAddYearsOverLeapDays(t *testing.T) {
	cases := []struct {
		name string
		from string
		n    int
		want string
	}{
		{"a six-year EPCG period from a leap day lands on 28 February", "2024-02-29", 6, "2030-02-28"},
		{"a four-year block from a leap day lands on a leap day", "2024-02-29", 4, "2028-02-29"},
		{"an ordinary six years", "2023-04-01", 6, "2029-04-01"},
		{"six years over a non-leap century", "2096-02-29", 4, "2100-02-28"},
		{"six years over a leap century", "1996-02-29", 4, "2000-02-29"},
		{"a two-year extension from a leap day", "2024-02-29", 2, "2026-02-28"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MustDate(c.from).AddYears(c.n); got.String() != c.want {
				t.Fatalf("%s plus %d years = %s, want %s", c.from, c.n, got, c.want)
			}
		})
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		name  string
		a, b  string
		want  int
		leapY bool
	}{
		{"same day", "2026-11-24", "2026-11-24", 0, false},
		{"one day", "2026-11-24", "2026-11-25", 1, false},
		{"across a month", "2026-03-01", "2026-03-31", 30, false},
		{"a common year", "2023-01-01", "2024-01-01", 365, false},
		{"a leap year", "2024-01-01", "2025-01-01", 366, true},
		{"across a leap day", "2024-02-28", "2024-03-01", 2, true},
		{"across a non-leap end of February", "2023-02-28", "2023-03-01", 1, false},
		{"backwards is negative", "2026-11-25", "2026-11-24", -1, false},
		{"a six-year EPCG period in days", "2023-04-01", "2029-04-01", 2192, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysBetween(MustDate(c.a), MustDate(c.b)); got != c.want {
				t.Fatalf("DaysBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}

	if got := DaysBetween(Date{}, MustDate("2026-01-01")); got != 0 {
		t.Fatalf("an absent date gives no day count, got %d", got)
	}
}

func TestMonthsBetweenCountsWholeMonthsOnly(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"31 January to 28 February is one whole month", "2023-01-31", "2023-02-28", 1},
		{"31 January to 27 February is none", "2023-01-31", "2023-02-27", 0},
		{"an 18-month period exactly", "2026-01-15", "2027-07-15", 18},
		{"one day short of 18 months", "2026-01-15", "2027-07-14", 17},
		{"same day", "2026-01-15", "2026-01-15", 0},
		{"backwards", "2026-07-15", "2026-01-15", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MonthsBetween(MustDate(c.a), MustDate(c.b)); got != c.want {
				t.Fatalf("MonthsBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestWindow(t *testing.T) {
	w := Window{From: MustDate("2024-02-01"), To: MustDate("2024-02-29")}
	if !w.Valid() {
		t.Fatalf("February 2024 is a valid window")
	}
	if got := w.Days(); got != 29 {
		t.Fatalf("February 2024 has 29 days counted inclusively, got %d", got)
	}
	if got := (Window{From: MustDate("2023-02-01"), To: MustDate("2023-02-28")}).Days(); got != 28 {
		t.Fatalf("February 2023 has 28 days, got %d", got)
	}

	for _, c := range []struct {
		day  string
		want bool
	}{
		{"2024-01-31", false},
		{"2024-02-01", true}, // both ends are inside
		{"2024-02-29", true},
		{"2024-03-01", false},
	} {
		if got := w.Contains(MustDate(c.day)); got != c.want {
			t.Fatalf("window %s contains %s = %v, want %v", w, c.day, got, c.want)
		}
	}

	if (Window{From: MustDate("2024-03-01"), To: MustDate("2024-02-01")}).Valid() {
		t.Fatalf("a window that ends before it begins is not valid")
	}
	if (Window{}).Contains(MustDate("2024-02-10")) {
		t.Fatalf("an empty window contains nothing")
	}
	if w.Contains(Date{}) {
		t.Fatalf("an absent day is in no window")
	}
}

func TestWindowOverlap(t *testing.T) {
	cases := []struct {
		name   string
		a, b   Window
		want   string
		wantOK bool
	}{
		{
			"a financial year inside an obligation period",
			Window{MustDate("2023-05-15"), MustDate("2029-05-14")},
			FinancialYear{2024}.Window(),
			"1 April 2024 to 31 March 2025", true,
		},
		{
			"the first, part financial year of an obligation period",
			Window{MustDate("2023-05-15"), MustDate("2029-05-14")},
			FinancialYear{2023}.Window(),
			"15 May 2023 to 31 March 2024", true,
		},
		{
			"the last, part financial year of an obligation period",
			Window{MustDate("2023-05-15"), MustDate("2029-05-14")},
			FinancialYear{2029}.Window(),
			"1 April 2029 to 14 May 2029", true,
		},
		{
			"a financial year wholly before the period",
			Window{MustDate("2023-05-15"), MustDate("2029-05-14")},
			FinancialYear{2022}.Window(),
			"", false,
		},
		{
			"touching on a single day",
			Window{MustDate("2023-04-01"), MustDate("2023-04-01")},
			FinancialYear{2023}.Window(),
			"1 April 2023 to 1 April 2023", true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := c.a.Overlap(c.b)
			if ok != c.wantOK {
				t.Fatalf("overlap ok = %v, want %v", ok, c.wantOK)
			}
			if ok && got.String() != c.want {
				t.Fatalf("overlap = %q, want %q", got.String(), c.want)
			}
		})
	}
}

func TestFinancialYearOf(t *testing.T) {
	cases := []struct {
		name string
		day  string
		want string
	}{
		{"1 April begins a new licensing year", "2024-04-01", "2024-25"},
		{"31 March ends one", "2024-03-31", "2023-24"},
		{"a day in January belongs to the year that began the previous April", "2025-01-15", "2024-25"},
		{"a leap day belongs to the year that began the previous April", "2024-02-29", "2023-24"},
		{"the last day of a leap financial year", "2025-03-31", "2024-25"},
		{"a day in December", "2026-12-31", "2026-27"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FinancialYearOf(MustDate(c.day)).String(); got != c.want {
				t.Fatalf("FinancialYearOf(%s) = %q, want %q", c.day, got, c.want)
			}
		})
	}
	if got := FinancialYearOf(Date{}); got.StartYear != 0 {
		t.Fatalf("an absent day has no financial year, got %v", got)
	}
}

func TestFinancialYearWindowAndNaming(t *testing.T) {
	cases := []struct {
		start int
		name  string
		from  string
		to    string
		days  int
	}{
		{2023, "2023-24", "2023-04-01", "2024-03-31", 366}, // contains 29 February 2024
		{2024, "2024-25", "2024-04-01", "2025-03-31", 365},
		{2099, "2099-00", "2099-04-01", "2100-03-31", 365}, // 2100 is not a leap year
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fy := FinancialYear{c.start}
			if fy.String() != c.name {
				t.Fatalf("name = %q, want %q", fy.String(), c.name)
			}
			w := fy.Window()
			if w.From.String() != c.from || w.To.String() != c.to {
				t.Fatalf("window = %s to %s, want %s to %s", w.From, w.To, c.from, c.to)
			}
			if w.Days() != c.days {
				t.Fatalf("days = %d, want %d", w.Days(), c.days)
			}
		})
	}
	if got := (FinancialYear{}).Window(); got.Valid() {
		t.Fatalf("an unset financial year has no window")
	}
}

func TestFinancialYearsOverlapping(t *testing.T) {
	// A six-year obligation period beginning mid-year touches SEVEN licensing
	// years, the first and last of them part years. Getting this wrong is how a
	// tool reports an average obligation as met in a year it was not measured in.
	got := FinancialYearsOverlapping(Window{MustDate("2023-05-15"), MustDate("2029-05-14")})
	want := []string{"2023-24", "2024-25", "2025-26", "2026-27", "2027-28", "2028-29", "2029-30"}
	if len(got) != len(want) {
		t.Fatalf("got %d years %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Fatalf("year %d = %q, want %q", i, got[i].String(), want[i])
		}
	}

	// A period beginning exactly on 1 April and ending on 31 March touches
	// exactly six.
	six := FinancialYearsOverlapping(Window{MustDate("2023-04-01"), MustDate("2029-03-31")})
	if len(six) != 6 {
		t.Fatalf("a period from 1 April to 31 March six years later touches 6 licensing years, got %d", len(six))
	}

	if FinancialYearsOverlapping(Window{}) != nil {
		t.Fatalf("an invalid window touches no years")
	}
}

func TestDateComparisonsAndHuman(t *testing.T) {
	a, b := MustDate("2026-02-28"), MustDate("2026-03-01")
	if !a.Before(b) || !b.After(a) || a.Equal(b) {
		t.Fatalf("28 February falls before 1 March")
	}
	if !a.OnOrBefore(a) || !a.OnOrAfter(a) {
		t.Fatalf("a day is on or before, and on or after, itself")
	}
	if got := MustDate("2024-02-29").Human(); got != "29 February 2024" {
		t.Fatalf("Human = %q", got)
	}
	if got := (Date{}).Human(); got != "—" {
		t.Fatalf("an absent day shows as a dash, got %q", got)
	}
	if got := (Date{Y: 2026, M: 2, D: 30}).Human(); got != "—" {
		t.Fatalf("a day that does not exist shows as a dash, got %q", got)
	}
	if got := (Date{}).String(); got != "" {
		t.Fatalf("an absent day has no string form, got %q", got)
	}
	// Comparison must order by year, then month, then day.
	if !MustDate("2025-12-31").Before(MustDate("2026-01-01")) {
		t.Fatalf("years order first")
	}
	if !MustDate("2026-01-31").Before(MustDate("2026-02-01")) {
		t.Fatalf("months order before days")
	}
}
