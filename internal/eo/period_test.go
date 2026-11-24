package eo

import "testing"

func TestDerivePeriodEnd(t *testing.T) {
	cases := []struct {
		name          string
		start         string
		months        int
		endsDayBefore bool
		want          string
	}{
		{"a six-year EPCG period, the day before the anniversary", "2023-05-15", 72, true, "2029-05-14"},
		{"a six-year EPCG period, on the anniversary", "2023-05-15", 72, false, "2029-05-15"},
		{"an 18-month Advance Authorisation period", "2026-04-20", 18, true, "2027-10-19"},
		{"an 18-month period on the anniversary", "2026-04-20", 18, false, "2027-10-20"},

		// Leap days. A licence issued on 29 February has no sixth anniversary,
		// so the month arithmetic clamps to the 28th and the convention takes a
		// day off that.
		{"six years from a leap day", "2024-02-29", 72, true, "2030-02-27"},
		{"six years from a leap day, on the anniversary", "2024-02-29", 72, false, "2030-02-28"},
		{"four years from a leap day lands on a leap day", "2024-02-29", 48, false, "2028-02-29"},
		{"four years from a leap day, day before", "2024-02-29", 48, true, "2028-02-28"},

		// Long months. 31 August plus 18 months is the end of February, not the
		// start of March.
		{"18 months from 31 August into a leap February", "2026-08-31", 18, false, "2028-02-29"},
		{"18 months from 31 August into a common February", "2025-08-31", 18, false, "2027-02-28"},
		{"18 months from 31 August, day before", "2026-08-31", 18, true, "2028-02-28"},

		// A period beginning on 1 April ends on 31 March, which is what makes
		// the licensing years line up exactly.
		{"a six-year period from 1 April", "2024-04-01", 72, true, "2030-03-31"},

		{"a period of no months is no period at all", "2024-04-01", 0, true, ""},
		{"a negative period is refused", "2024-04-01", -12, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DerivePeriodEnd(MustDate(c.start), c.months, c.endsDayBefore)
			if got.String() != c.want {
				t.Fatalf("DerivePeriodEnd(%s, %d, %v) = %q, want %q",
					c.start, c.months, c.endsDayBefore, got.String(), c.want)
			}
		})
	}

	if got := DerivePeriodEnd(Date{}, 72, true); !got.Zero() {
		t.Fatalf("no start date means no end date, got %v", got)
	}
}

// epcgSkeleton is a minimal EPCG licence for the clock tests: no exports, no
// imports, just the dates.
func epcgSkeleton(issued string) Licence {
	return Licence{
		Scheme:             SchemeEPCG,
		AuthorisationNo:    "TEST-1",
		IssuedOn:           MustDate(issued),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
	}.Normalise()
}

func TestPlanPeriodPrecedence(t *testing.T) {
	t.Run("derived when nothing is endorsed", func(t *testing.T) {
		plan := PlanPeriod(epcgSkeleton("2023-05-15"))
		if plan.Ends.String() != "2029-05-14" || plan.EndsSource != SourceDerived {
			t.Fatalf("ends %s from %s, want 2029-05-14 derived", plan.Ends, plan.EndsSource)
		}
		if plan.PeriodMonths != 72 {
			t.Fatalf("period = %d months", plan.PeriodMonths)
		}
	})

	t.Run("an endorsed end date governs over the derived one", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.EOPeriodEndEndorsed = MustDate("2029-05-15")
		plan := PlanPeriod(l)
		if plan.Ends.String() != "2029-05-15" || plan.EndsSource != SourceEndorsed {
			t.Fatalf("ends %s from %s", plan.Ends, plan.EndsSource)
		}
		// One day apart is exactly the period-end convention, so it is not
		// reported as a disagreement.
		if plan.Disagreement != "" {
			t.Fatalf("a one-day difference is the convention, not a disagreement: %q", plan.Disagreement)
		}
	})

	t.Run("an endorsed date well away from the derived one is reported", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.EOPeriodEndEndorsed = MustDate("2027-05-14") // a four-year period
		plan := PlanPeriod(l)
		if plan.Ends.String() != "2027-05-14" {
			t.Fatalf("the endorsed date must govern, got %s", plan.Ends)
		}
		if plan.Disagreement == "" {
			t.Fatalf("a two-year difference must be reported out loud")
		}
	})

	t.Run("a granted extension moves the end later", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{{
			Kind: ExtensionOverall, GrantedOn: MustDate("2029-04-01"),
			NewEndsOn: MustDate("2031-05-14"), Reference: "RA/EXT/114",
		}}
		plan := PlanPeriod(l.Normalise())
		if plan.Ends.String() != "2031-05-14" || plan.EndsSource != SourceExtended {
			t.Fatalf("ends %s from %s, want 2031-05-14 extended", plan.Ends, plan.EndsSource)
		}
		if plan.ExtendedTo.String() != "2031-05-14" {
			t.Fatalf("ExtendedTo = %s", plan.ExtendedTo)
		}
	})

	t.Run("an extension cannot shorten the period", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{{Kind: ExtensionOverall, NewEndsOn: MustDate("2025-01-01")}}
		plan := PlanPeriod(l.Normalise())
		if plan.Ends.String() != "2029-05-14" || plan.EndsSource != SourceDerived {
			t.Fatalf("ends %s from %s — a backwards extension must be ignored", plan.Ends, plan.EndsSource)
		}
	})

	t.Run("the latest of several extensions wins", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{
			{Kind: ExtensionOverall, NewEndsOn: MustDate("2030-05-14")},
			{Kind: ExtensionOverall, NewEndsOn: MustDate("2031-05-14")},
		}
		plan := PlanPeriod(l.Normalise())
		if plan.Ends.String() != "2031-05-14" {
			t.Fatalf("ends %s", plan.Ends)
		}
	})

	t.Run("an Advance Authorisation gets the shorter period and no blocks", func(t *testing.T) {
		l := Licence{
			Scheme: SchemeAdvanceAuthorisation, IssuedOn: MustDate("2026-04-20"),
			EOValueEndorsed: MustMoney("1,38,00,000"),
		}.Normalise()
		plan := PlanPeriod(l)
		if plan.Ends.String() != "2027-10-19" {
			t.Fatalf("ends %s, want 2027-10-19", plan.Ends)
		}
		if len(plan.Blocks) != 0 {
			t.Fatalf("an Advance Authorisation has no block-wise milestone, got %d", len(plan.Blocks))
		}
	})
}

func TestPlanBlocks(t *testing.T) {
	t.Run("the published two blocks, contiguous and inclusive", func(t *testing.T) {
		plan := PlanPeriod(epcgSkeleton("2023-05-15"))
		if len(plan.Blocks) != 2 {
			t.Fatalf("got %d blocks", len(plan.Blocks))
		}
		b1, b2 := plan.Blocks[0], plan.Blocks[1]
		if b1.Window.From.String() != "2023-05-15" || b1.Window.To.String() != "2027-05-14" {
			t.Fatalf("block 1 = %s", b1.Window)
		}
		if b2.Window.From.String() != "2027-05-15" || b2.Window.To.String() != "2029-05-14" {
			t.Fatalf("block 2 = %s", b2.Window)
		}
		// No gap and no overlap: the day after block 1 ends begins block 2.
		if DaysBetween(b1.Window.To, b2.Window.From) != 1 {
			t.Fatalf("the blocks are not contiguous: %s then %s", b1.Window, b2.Window)
		}
		if b1.CumulativePercent != 50 || b2.CumulativePercent != 100 {
			t.Fatalf("cumulative percentages %d and %d", b1.CumulativePercent, b2.CumulativePercent)
		}
	})

	t.Run("blocks from a leap day", func(t *testing.T) {
		plan := PlanPeriod(epcgSkeleton("2024-02-29"))
		if got := plan.Blocks[0].Window.To.String(); got != "2028-02-28" {
			t.Fatalf("block 1 ends %s, want 2028-02-28 (48 months from a leap day, day before the anniversary)", got)
		}
		if got := plan.Blocks[1].Window.From.String(); got != "2028-02-29" {
			t.Fatalf("block 2 begins %s, want 2028-02-29 — which exists, 2028 being a leap year", got)
		}
		if got := plan.Blocks[1].Window.To.String(); got != "2030-02-27" {
			t.Fatalf("block 2 ends %s, want 2030-02-27", got)
		}
	})

	t.Run("an endorsed block end date governs", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.BlockEndsEndorsed = []Date{MustDate("2027-05-15")}
		plan := PlanPeriod(l.Normalise())
		if plan.Blocks[0].Window.To.String() != "2027-05-15" || plan.Blocks[0].EndsSource != SourceEndorsed {
			t.Fatalf("block 1 ends %s from %s", plan.Blocks[0].Window.To, plan.Blocks[0].EndsSource)
		}
		if plan.Blocks[1].Window.From.String() != "2027-05-16" {
			t.Fatalf("block 2 must begin the day after block 1, got %s", plan.Blocks[1].Window.From)
		}
	})

	t.Run("a block extension moves only that block", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{{Kind: ExtensionBlock, BlockNo: 1, NewEndsOn: MustDate("2027-11-14")}}
		plan := PlanPeriod(l.Normalise())
		if plan.Blocks[0].Window.To.String() != "2027-11-14" || plan.Blocks[0].EndsSource != SourceExtended {
			t.Fatalf("block 1 ends %s from %s", plan.Blocks[0].Window.To, plan.Blocks[0].EndsSource)
		}
		if plan.Ends.String() != "2029-05-14" {
			t.Fatalf("extending a block must not extend the period, got %s", plan.Ends)
		}
		if plan.Blocks[1].Window.To.String() != "2029-05-14" {
			t.Fatalf("block 2 still closes the period, got %s", plan.Blocks[1].Window.To)
		}
	})

	t.Run("a block cannot be extended past the period", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{{Kind: ExtensionBlock, BlockNo: 1, NewEndsOn: MustDate("2035-01-01")}}
		plan := PlanPeriod(l.Normalise())
		if plan.Blocks[0].Window.To.String() != "2029-05-14" {
			t.Fatalf("block 1 ends %s, must be clamped to the end of the period", plan.Blocks[0].Window.To)
		}
	})

	t.Run("extending the period moves the last block with it", func(t *testing.T) {
		l := epcgSkeleton("2023-05-15")
		l.Extensions = []Extension{{Kind: ExtensionOverall, NewEndsOn: MustDate("2031-05-14")}}
		plan := PlanPeriod(l.Normalise())
		if plan.Blocks[1].Window.To.String() != "2031-05-14" {
			t.Fatalf("the last block must close the extended period, got %s", plan.Blocks[1].Window.To)
		}
		if plan.Blocks[1].EndsSource != SourceExtended {
			t.Fatalf("source = %s", plan.Blocks[1].EndsSource)
		}
	})

	t.Run("no blocks without a date of issue", func(t *testing.T) {
		l := Licence{Scheme: SchemeEPCG, SpecificEOEndorsed: MustMoney("1,00,000")}.Normalise()
		if got := PlanPeriod(l); len(got.Blocks) != 0 {
			t.Fatalf("got %d blocks without a date of issue", len(got.Blocks))
		}
	})
}

func TestClockFor(t *testing.T) {
	w := Window{From: MustDate("2023-05-15"), To: MustDate("2029-05-14")}
	cases := []struct {
		name       string
		asOf       string
		total      int
		elapsed    int
		left       int
		expired    bool
		notStarted bool
	}{
		{"the first day of the period counts as elapsed", "2023-05-15", 2192, 1, 2192, false, false},
		{"the last day of the period is still a day you have", "2029-05-14", 2192, 2192, 1, false, false},
		{"the day after is expired", "2029-05-15", 2192, 2192, 0, true, false},
		{"well before the period", "2023-01-01", 2192, 0, 2192 + 134, false, true},
		{"half way", "2026-05-14", 2192, 1096, 1097, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClockFor(w, MustDate(c.asOf))
			if got.DaysTotal != c.total {
				t.Fatalf("total = %d, want %d", got.DaysTotal, c.total)
			}
			if got.DaysElapsed != c.elapsed {
				t.Fatalf("elapsed = %d, want %d", got.DaysElapsed, c.elapsed)
			}
			if got.DaysLeft != c.left {
				t.Fatalf("left = %d, want %d", got.DaysLeft, c.left)
			}
			if got.Expired != c.expired {
				t.Fatalf("expired = %v, want %v", got.Expired, c.expired)
			}
			if got.NotStarted != c.notStarted {
				t.Fatalf("notStarted = %v, want %v", got.NotStarted, c.notStarted)
			}
		})
	}

	// The elapsed and remaining days of a period always account for every day
	// of it exactly once, which is what makes "days left" trustworthy.
	for _, day := range []string{"2023-05-15", "2024-02-29", "2026-05-14", "2029-05-14"} {
		got := ClockFor(w, MustDate(day))
		if got.DaysElapsed+got.DaysLeft != got.DaysTotal+1 {
			t.Fatalf("on %s, %d elapsed + %d left does not reconcile with %d total",
				day, got.DaysElapsed, got.DaysLeft, got.DaysTotal)
		}
	}

	if got := ClockFor(Window{}, MustDate("2026-01-01")); got.DaysTotal != 0 {
		t.Fatalf("an invalid window has no clock, got %+v", got)
	}
	if got := ClockFor(w, Date{}); got.DaysTotal != 0 {
		t.Fatalf("no as-of date means no clock, got %+v", got)
	}
}

func TestUrgencyOf(t *testing.T) {
	w := Window{From: MustDate("2023-05-15"), To: MustDate("2029-05-14")}
	cases := []struct {
		asOf string
		want Urgency
	}{
		{"2026-01-01", UrgencyNone},
		{"2028-11-15", UrgencyNone},   // 181 days left, one outside the warning
		{"2028-11-16", UrgencySoon},   // 180 days left, the warning boundary
		{"2029-03-15", UrgencySoon},   // 61 days left, one outside urgent
		{"2029-03-16", UrgencyUrgent}, // 60 days left, the urgent boundary
		{"2029-05-14", UrgencyUrgent},
		{"2029-05-15", UrgencyExpired},
	}
	for _, c := range cases {
		t.Run(c.asOf, func(t *testing.T) {
			clock := ClockFor(w, MustDate(c.asOf))
			if got := UrgencyOf(clock); got != c.want {
				t.Fatalf("on %s, %d days left, urgency = %q, want %q", c.asOf, clock.DaysLeft, got, c.want)
			}
		})
	}
	if got := UrgencyOf(ClockFor(Window{}, MustDate("2026-01-01"))); got != UrgencyNone {
		t.Fatalf("no window, no urgency, got %q", got)
	}
}

func TestStatusAndSourceTitles(t *testing.T) {
	for _, s := range []Status{StatusMet, StatusOnTrack, StatusBehind, StatusOverdue, StatusUnknown} {
		if s.Title() == "" {
			t.Fatalf("status %q has no title", s)
		}
	}
	if (Status("nonsense")).Title() != "Not enough entered" {
		t.Fatalf("an unknown status must read as unknown, not as met")
	}
	for _, s := range []DateSource{SourceEndorsed, SourceDerived, SourceExtended} {
		if s.Title() == "" {
			t.Fatalf("source %q has no title", s)
		}
	}
	if got := (DateSource("x")).Title(); got != "x" {
		t.Fatalf("got %q", got)
	}
}
