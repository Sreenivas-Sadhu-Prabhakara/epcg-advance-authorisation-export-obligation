package eo

import "testing"

// countLicence builds an EPCG licence whose obligation period lines up exactly
// with the licensing years — issued 1 April 2024, so the period runs 1 April
// 2024 to 31 March 2030 and every measurement year is a whole one. That removes
// the part-year convention from the arithmetic under test.
func countLicence(averageEO string, shipments ...Shipment) Licence {
	return Licence{
		Scheme:             SchemeEPCG,
		AuthorisationNo:    "COUNT-1",
		IssuedOn:           MustDate("2024-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEO:          MustMoney(averageEO),
		Shipments:          shipments,
	}.Normalise()
}

func ship(date, fob string, sameOrSimilar bool) Shipment {
	return Shipment{
		ShippingBillNo:        "SB-" + date,
		ShippedOn:             MustDate(date),
		FOBValue:              MustMoney(fob),
		AuthorisationEndorsed: true,
		SameOrSimilar:         sameOrSimilar,
	}
}

func TestCountNetsTheAverageYearByYear(t *testing.T) {
	// The arithmetic the whole scheme turns on. An average of ₹1 crore a year,
	// and three years of exports:
	//   2024-25  ₹1.5 crore of the same product   → ₹50 lakh counts
	//   2025-26  ₹80 lakh of the same product     → NOTHING counts, and the
	//                                                year closed ₹20 lakh short
	//   2026-27  ₹40 lakh of a different product  → ₹40 lakh counts in full
	// Gross exports ₹2.7 crore; counted toward the obligation ₹90 lakh.
	l := countLicence("1,00,00,000",
		ship("2024-06-01", "1,50,00,000", true),
		ship("2025-06-01", "80,00,000", true),
		ship("2026-06-01", "40,00,000", false),
	)
	w := Window{From: MustDate("2024-04-01"), To: MustDate("2027-03-31")}
	got := Count(l, l.Policy, w, Window{From: MustDate("2024-04-01"), To: MustDate("2030-03-31")})

	if got.TotalFOB != MustMoney("2,70,00,000") {
		t.Fatalf("gross counted FOB = %s, want 2,70,00,000", got.TotalFOB)
	}
	if got.CountedToSpecific != MustMoney("90,00,000") {
		t.Fatalf("counted toward the obligation = %s, want 90,00,000", got.CountedToSpecific)
	}
	if got.AverageShortfallTotal != MustMoney("1,20,00,000") {
		// ₹20 lakh in 2025-26 plus the whole ₹1 crore in 2026-27, in which
		// nothing of the same or a similar product was exported at all.
		t.Fatalf("average shortfall = %s, want 1,20,00,000", got.AverageShortfallTotal)
	}

	if len(got.Years) != 3 {
		t.Fatalf("got %d years, want 3", len(got.Years))
	}
	want := []struct {
		label    string
		same     string
		other    string
		due      string
		met      bool
		specific string
	}{
		{"2024-25", "1,50,00,000", "0", "1,00,00,000", true, "50,00,000"},
		{"2025-26", "80,00,000", "0", "1,00,00,000", false, "0"},
		{"2026-27", "0", "40,00,000", "1,00,00,000", false, "40,00,000"},
	}
	for i, w := range want {
		y := got.Years[i]
		if y.Label != w.label {
			t.Fatalf("year %d label = %q, want %q", i, y.Label, w.label)
		}
		if y.SameOrSimilar != MustMoney(w.same) {
			t.Fatalf("%s same/similar = %s, want %s", w.label, y.SameOrSimilar, w.same)
		}
		if y.OtherProducts != MustMoney(w.other) {
			t.Fatalf("%s other products = %s, want %s", w.label, y.OtherProducts, w.other)
		}
		if y.AverageDue != MustMoney(w.due) {
			t.Fatalf("%s average due = %s, want %s", w.label, y.AverageDue, w.due)
		}
		if y.AverageMet != w.met {
			t.Fatalf("%s average met = %v, want %v", w.label, y.AverageMet, w.met)
		}
		if y.CountedToSpecific != MustMoney(w.specific) {
			t.Fatalf("%s counted to specific = %s, want %s", w.label, y.CountedToSpecific, w.specific)
		}
		if y.PartYear {
			t.Fatalf("%s should be a whole year", w.label)
		}
	}
}

func TestCountWithoutAnAverageCountsEverything(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Licence)
	}{
		{"no average entered", func(l *Licence) { l.AverageEO = 0 }},
		{"exempt from maintaining one", func(l *Licence) { l.AverageEOExempt = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := countLicence("1,00,00,000",
				ship("2024-06-01", "1,50,00,000", true),
				ship("2025-06-01", "80,00,000", true),
				ship("2026-06-01", "40,00,000", false),
			)
			c.mut(&l)
			l = l.Normalise()
			w := Window{From: MustDate("2024-04-01"), To: MustDate("2027-03-31")}
			got := Count(l, l.Policy, w, w)
			if got.AverageApplies {
				t.Fatalf("the average must not apply")
			}
			if got.CountedToSpecific != MustMoney("2,70,00,000") {
				t.Fatalf("counted = %s, want the whole 2,70,00,000", got.CountedToSpecific)
			}
			if got.AverageShortfallTotal != 0 {
				t.Fatalf("there can be no average shortfall where no average applies, got %s", got.AverageShortfallTotal)
			}
			for _, y := range got.Years {
				if !y.AverageMet {
					t.Fatalf("%s reported short of an average that does not apply", y.Label)
				}
			}
		})
	}
}

func TestCountAverageExactlyMetCountsNothingToTheObligation(t *testing.T) {
	// The boundary that catches people out: exporting exactly the average
	// discharges none of the specific obligation. One rupee more discharges one
	// rupee of it.
	cases := []struct {
		name      string
		fob       string
		counted   string
		met       bool
		shortfall string
	}{
		{"one paisa under the average", "99,99,999.99", "0", false, "0.01"},
		{"exactly the average", "1,00,00,000", "0", true, "0"},
		{"one paisa over the average", "1,00,00,000.01", "0.01", true, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := countLicence("1,00,00,000", ship("2024-06-01", c.fob, true))
			w := Window{From: MustDate("2024-04-01"), To: MustDate("2025-03-31")}
			got := Count(l, l.Policy, w, w)
			if got.CountedToSpecific != MustMoney(c.counted) {
				t.Fatalf("counted = %s, want %s", got.CountedToSpecific, c.counted)
			}
			if got.Years[0].AverageMet != c.met {
				t.Fatalf("average met = %v, want %v", got.Years[0].AverageMet, c.met)
			}
			if got.AverageShortfallTotal != MustMoney(c.shortfall) {
				t.Fatalf("shortfall = %s, want %s", got.AverageShortfallTotal, c.shortfall)
			}
		})
	}
}

func TestCountPartYearProRata(t *testing.T) {
	// A one-year obligation period beginning 1 October 2024 straddles two
	// licensing years: 182 days of 2024-25 and 183 days of 2025-26, both of
	// which are 365-day years. The pro-rated shares therefore add up to exactly
	// one annual average, which is the property that makes the convention
	// defensible.
	twelve := Policy{EPCGPeriodMonths: 12}
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-10-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEO:          MustMoney("1,00,00,000"),
		Policy:             twelve,
	}.Normalise()

	plan := PlanPeriod(l)
	if plan.Ends.String() != "2025-09-30" {
		t.Fatalf("period ends %s, want 2025-09-30", plan.Ends)
	}

	got := Count(l, l.Policy, plan.PeriodWindow(), plan.PeriodWindow())
	if len(got.Years) != 2 {
		t.Fatalf("got %d years, want 2", len(got.Years))
	}
	if got.Years[0].DaysCounted != 182 || got.Years[0].DaysInYear != 365 {
		t.Fatalf("2024-25 counted %d of %d days", got.Years[0].DaysCounted, got.Years[0].DaysInYear)
	}
	if got.Years[1].DaysCounted != 183 || got.Years[1].DaysInYear != 365 {
		t.Fatalf("2025-26 counted %d of %d days", got.Years[1].DaysCounted, got.Years[1].DaysInYear)
	}
	if !got.Years[0].PartYear || !got.Years[1].PartYear {
		t.Fatalf("both years are part years here")
	}
	if got.Years[0].AverageDue != MustMoney("49,86,301.37") {
		t.Fatalf("2024-25 average due = %s, want 49,86,301.37 (182/365 of a crore)", got.Years[0].AverageDue)
	}
	if got.Years[1].AverageDue != MustMoney("50,13,698.63") {
		t.Fatalf("2025-26 average due = %s, want 50,13,698.63 (183/365 of a crore)", got.Years[1].AverageDue)
	}
	if sum := got.Years[0].AverageDue + got.Years[1].AverageDue; sum != MustMoney("1,00,00,000") {
		t.Fatalf("the pro-rated shares come to %s, want exactly one annual average", sum)
	}

	// With the convention off, each part year carries a whole year's average,
	// so a twelve-month period owes two years of it.
	no := false
	off := l
	off.Policy.PartYearProRata = &no
	off = off.Normalise()
	gotOff := Count(off, off.Policy, plan.PeriodWindow(), plan.PeriodWindow())
	if sum := gotOff.AverageDueTotal; sum != MustMoney("2,00,00,000") {
		t.Fatalf("with pro-rating off the total due is %s, want 2,00,00,000", sum)
	}
}

func TestCountAcrossTheFinancialYearEnd(t *testing.T) {
	// 31 March and 1 April are a day apart and in different licensing years.
	// Which side of midnight a shipping bill falls on decides which year's
	// average it is measured against, and therefore how much of it counts.
	cases := []struct {
		name    string
		a, b    string
		counted string
	}{
		{
			"both in one licensing year: only the excess over one average counts",
			"2025-06-01", "2026-03-31", "60,00,000",
		},
		{
			"split across the year end: each year nets its own average",
			"2025-06-01", "2026-04-01", "0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := countLicence("1,00,00,000",
				ship(c.a, "80,00,000", true),
				ship(c.b, "80,00,000", true),
			)
			w := Window{From: MustDate("2024-04-01"), To: MustDate("2027-03-31")}
			got := Count(l, l.Policy, w, w)
			if got.CountedToSpecific != MustMoney(c.counted) {
				t.Fatalf("counted = %s, want %s", got.CountedToSpecific, c.counted)
			}
			if got.TotalFOB != MustMoney("1,60,00,000") {
				t.Fatalf("gross = %s; the same exports either way", got.TotalFOB)
			}
		})
	}
}

func TestCountAcrossALeapDay(t *testing.T) {
	// 2023-24 contains 29 February 2024, so it is a 366-day licensing year and
	// a part year in it is pro-rated over 366, not 365.
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-02-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEO:          MustMoney("3,66,00,000"),
		Policy:             Policy{EPCGPeriodMonths: 12},
		Shipments: []Shipment{
			ship("2024-02-29", "1,00,00,000", true), // a shipment on a leap day
		},
	}.Normalise()

	plan := PlanPeriod(l)
	got := Count(l, l.Policy, plan.PeriodWindow(), plan.PeriodWindow())
	if got.Years[0].Label != "2023-24" {
		t.Fatalf("first year = %q, want 2023-24", got.Years[0].Label)
	}
	if got.Years[0].DaysInYear != 366 {
		t.Fatalf("2023-24 has %d days, want 366", got.Years[0].DaysInYear)
	}
	// 1 February to 31 March 2024 inclusive: 29 + 31 = 60 days.
	if got.Years[0].DaysCounted != 60 {
		t.Fatalf("counted %d days, want 60", got.Years[0].DaysCounted)
	}
	// ₹3.66 crore × 60/366 = ₹60 lakh exactly.
	if got.Years[0].AverageDue != MustMoney("60,00,000") {
		t.Fatalf("average due = %s, want 60,00,000", got.Years[0].AverageDue)
	}
	if got.Years[0].SameOrSimilar != MustMoney("1,00,00,000") {
		t.Fatalf("the leap-day shipment was not counted: %s", got.Years[0].SameOrSimilar)
	}
	if got.Years[0].CountedToSpecific != MustMoney("40,00,000") {
		t.Fatalf("counted to specific = %s, want 40,00,000", got.Years[0].CountedToSpecific)
	}
}

func TestCountClassifiesWhatItLeavesOut(t *testing.T) {
	period := Window{From: MustDate("2024-04-01"), To: MustDate("2030-03-31")}
	asOf := Window{From: MustDate("2024-04-01"), To: MustDate("2026-11-24")}

	l := countLicence("0",
		ship("2024-06-01", "10,00,000", true), // counted
		func() Shipment { s := ship("2024-07-01", "11,00,000", true); s.AuthorisationEndorsed = false; return s }(),
		func() Shipment {
			s := ship("2024-08-01", "12,00,000", true)
			s.Excluded = true
			s.ExcludedReason = "counted against another licence"
			return s
		}(),
		ship("2023-01-01", "13,00,000", true), // before the period
		ship("2031-01-01", "14,00,000", true), // after the period
		ship("2027-01-01", "15,00,000", true), // inside the period, after today
	)
	// An undated shipment with no value: a row begun and not finished.
	l.Shipments = append(l.Shipments, Shipment{ShippingBillNo: "SB-BLANK", AuthorisationEndorsed: true})
	l = l.Normalise()

	got := Count(l, l.Policy, asOf, period)

	if got.CountedToSpecific != MustMoney("10,00,000") {
		t.Fatalf("counted = %s, want only the one good shipment", got.CountedToSpecific)
	}
	if got.Shipments != 1 {
		t.Fatalf("counted %d shipments, want 1", got.Shipments)
	}
	for _, c := range []struct {
		name string
		list []int
		want int
	}{
		{"not endorsed", got.NotEndorsed, 1},
		{"excluded", got.Excluded, 1},
		{"outside the period", got.OutsidePeriod, 2},
		{"inside the period but not yet", got.NotYet, 1},
		{"undated", got.Undated, 1},
	} {
		if len(c.list) != c.want {
			t.Fatalf("%s: %d rows, want %d (%v)", c.name, len(c.list), c.want, c.list)
		}
	}

	// Every shipment is accounted for exactly once.
	total := got.Shipments + len(got.NotEndorsed) + len(got.Excluded) +
		len(got.OutsidePeriod) + len(got.NotYet) + len(got.Undated)
	if total != len(l.Shipments) {
		t.Fatalf("%d shipments accounted for, %d entered", total, len(l.Shipments))
	}
}

func TestCountRealisation(t *testing.T) {
	realised := ship("2024-06-01", "50,00,000", true)
	realised.RealisedOn = MustDate("2024-09-01")
	realised.RealisedValue = MustMoney("50,00,000")

	shortRealised := ship("2024-07-01", "40,00,000", true)
	shortRealised.RealisedOn = MustDate("2024-10-01")
	shortRealised.RealisedValue = MustMoney("38,50,000")

	unrealised := ship("2024-08-01", "30,00,000", true)

	l := countLicence("0", realised, shortRealised, unrealised)
	w := Window{From: MustDate("2024-04-01"), To: MustDate("2025-03-31")}
	got := Count(l, l.Policy, w, w)

	if got.RealisedTotal != MustMoney("88,50,000") {
		t.Fatalf("realised = %s, want 88,50,000", got.RealisedTotal)
	}
	if got.Unrealised != MustMoney("30,00,000") || got.UnrealisedCount != 1 {
		t.Fatalf("unrealised = %s on %d bills", got.Unrealised, got.UnrealisedCount)
	}
	if got.RealisedShortfall != MustMoney("1,50,000") {
		t.Fatalf("realisation shortfall = %s, want 1,50,000", got.RealisedShortfall)
	}
	// The shipping bill value is what counts; the shortfall is reported, not
	// netted off, because which figure a Regional Authority takes is its own
	// question.
	if got.CountedToSpecific != MustMoney("1,20,00,000") {
		t.Fatalf("counted = %s, want the full 1,20,00,000 of shipping bill value", got.CountedToSpecific)
	}
}

func TestCountByAuthorisationYear(t *testing.T) {
	// Measured against anniversary years, the years line up with the period
	// exactly, so a mid-year issue date produces whole years and no part-year
	// convention is used at all.
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2023-05-15"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEO:          MustMoney("1,00,00,000"),
		Policy:             Policy{AverageBasis: AverageByAuthorisationYear},
		Shipments: []Shipment{
			ship("2023-06-01", "1,20,00,000", true), // year 1
			ship("2024-06-01", "90,00,000", true),   // year 2, short
		},
	}.Normalise()

	plan := PlanPeriod(l)
	got := Count(l, l.Policy, Window{From: plan.Start, To: MustDate("2025-05-14")}, plan.PeriodWindow())

	if len(got.Years) != 2 {
		t.Fatalf("got %d years, want 2", len(got.Years))
	}
	if got.Years[0].Label != "Year 1" || got.Years[1].Label != "Year 2" {
		t.Fatalf("labels %q and %q", got.Years[0].Label, got.Years[1].Label)
	}
	if got.Years[0].FullWindow.From.String() != "2023-05-15" || got.Years[0].FullWindow.To.String() != "2024-05-14" {
		t.Fatalf("year 1 = %s", got.Years[0].FullWindow)
	}
	if got.Years[0].PartYear || got.Years[1].PartYear {
		t.Fatalf("anniversary years inside the period are whole years")
	}
	if got.Years[0].AverageDue != MustMoney("1,00,00,000") || got.Years[1].AverageDue != MustMoney("1,00,00,000") {
		t.Fatalf("average due %s and %s", got.Years[0].AverageDue, got.Years[1].AverageDue)
	}
	if got.CountedToSpecific != MustMoney("20,00,000") {
		t.Fatalf("counted = %s, want 20,00,000", got.CountedToSpecific)
	}
}

func TestCountOnNothing(t *testing.T) {
	l := countLicence("1,00,00,000")
	if got := Count(l, l.Policy, Window{}, Window{}); got.CountedToSpecific != 0 || len(got.Years) != 0 {
		t.Fatalf("an invalid window counts nothing, got %+v", got)
	}
	w := Window{From: MustDate("2024-04-01"), To: MustDate("2025-03-31")}
	got := Count(l, l.Policy, w, w)
	if got.CountedToSpecific != 0 {
		t.Fatalf("no shipments, nothing counted, got %s", got.CountedToSpecific)
	}
	if len(got.Years) != 1 {
		t.Fatalf("a year with no exports is still a year to measure, got %d", len(got.Years))
	}
	if got.Years[0].AverageShortfall != MustMoney("1,00,00,000") {
		t.Fatalf("a year with no exports is the whole average short, got %s", got.Years[0].AverageShortfall)
	}
}
