package eo

import "testing"

func TestSpecificObligationDerivation(t *testing.T) {
	imports := []Import{
		{BillOfEntryNo: "A", ClearedOn: MustDate("2024-05-01"), CIFValue: MustMoney("1,80,00,000"), DutySaved: MustMoney("45,00,000")},
		{BillOfEntryNo: "B", ClearedOn: MustDate("2024-09-01"), CIFValue: MustMoney("60,00,000"), DutySaved: MustMoney("15,00,000")},
	}

	cases := []struct {
		name     string
		category Category
		endorsed string
		want     string
		source   DateSource
		multiple Bips
		disagree bool
	}{
		{"ordinary: six times the duty saved", CategoryOrdinary, "0", "3,60,00,000", SourceDerived, Times(600), false},
		{"green technology: four and a half times", CategoryGreenTechnology, "0", "2,70,00,000", SourceDerived, Times(450), false},
		{"a unit in a specified area: one and a half times", CategorySpecifiedArea, "0", "90,00,000", SourceDerived, Times(150), false},
		{"indigenous sourcing: a quarter less, four and a half times", CategoryIndigenousSourcing, "0", "2,70,00,000", SourceDerived, Times(450), false},
		{"as endorsed, with the multiple agreeing", CategoryAsEndorsed, "3,60,00,000", "3,60,00,000", SourceEndorsed, 0, false},
		{"an endorsed figure governs over the multiple", CategoryOrdinary, "4,10,00,000", "4,10,00,000", SourceEndorsed, Times(600), true},
		{"an endorsed figure that matches is no disagreement", CategoryOrdinary, "3,60,00,000", "3,60,00,000", SourceEndorsed, Times(600), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := Licence{
				Scheme: SchemeEPCG, IssuedOn: MustDate("2024-04-01"),
				Category: c.category, SpecificEOEndorsed: MustMoney(c.endorsed),
				Imports: imports,
			}.Normalise()

			got := SpecificObligation(l, l.Policy)
			if got.Required != MustMoney(c.want) {
				t.Fatalf("required = %s, want %s", got.Required, c.want)
			}
			if got.Source != c.source {
				t.Fatalf("source = %q, want %q", got.Source, c.source)
			}
			if got.Multiple != c.multiple {
				t.Fatalf("multiple = %s, want %s", got.Multiple.Multiple(), c.multiple.Multiple())
			}
			if got.DutySaved != MustMoney("60,00,000") {
				t.Fatalf("duty saved = %s, want 60,00,000", got.DutySaved)
			}
			if (got.Disagreement != "") != c.disagree {
				t.Fatalf("disagreement = %q, want a disagreement: %v", got.Disagreement, c.disagree)
			}
		})
	}
}

func TestSpecificObligationForAdvanceAuthorisation(t *testing.T) {
	l := Licence{
		Scheme: SchemeAdvanceAuthorisation, IssuedOn: MustDate("2026-04-20"),
		EOValueEndorsed: MustMoney("1,38,00,000"),
		Imports: []Import{{CIFValue: MustMoney("92,00,000"), DutySaved: MustMoney("13,80,000"),
			ClearedOn: MustDate("2026-05-18")}},
	}.Normalise()

	got := SpecificObligation(l, l.Policy)
	if got.Required != MustMoney("1,38,00,000") || got.Source != SourceEndorsed {
		t.Fatalf("required = %s from %q", got.Required, got.Source)
	}
	// An Advance Authorisation obligation does not come from a multiple of
	// anything: it comes from the norms, and the authorisation states it.
	if got.MultipleApplies {
		t.Fatalf("no multiple applies to an Advance Authorisation")
	}
	if got.Derived != 0 {
		t.Fatalf("nothing is derived for an Advance Authorisation, got %s", got.Derived)
	}
}

func TestObligationFill(t *testing.T) {
	cases := []struct {
		name      string
		required  string
		counted   string
		condone   int
		passed    bool
		pct       string
		shortfall string
		condoned  string
		net       string
		status    Status
	}{
		{"nothing done, period running", "1,00,00,000", "0", 0, false, "0%", "1,00,00,000", "0", "1,00,00,000", StatusBehind},
		{"half done, period running", "1,00,00,000", "50,00,000", 0, false, "50%", "50,00,000", "0", "50,00,000", StatusBehind},
		{"done exactly", "1,00,00,000", "1,00,00,000", 0, false, "100%", "0", "0", "0", StatusMet},
		{"over-done", "1,00,00,000", "1,20,00,000", 0, false, "120%", "0", "0", "0", StatusMet},
		{"a paisa short", "1,00,00,000", "99,99,999.99", 0, false, "100%", "0.01", "0", "0.01", StatusBehind},
		{"short with the period over", "1,00,00,000", "90,00,000", 0, true, "90%", "10,00,000", "0", "10,00,000", StatusOverdue},
		{"a 5% condonation absorbs a 4% shortfall", "1,00,00,000", "96,00,000", 5, true, "96%", "4,00,000", "4,00,000", "0", StatusMet},
		{"a 5% condonation caps at 5% of a 10% shortfall", "1,00,00,000", "90,00,000", 5, true, "90%", "10,00,000", "5,00,000", "5,00,000", StatusOverdue},
		{"no obligation entered", "0", "50,00,000", 0, false, "0%", "0", "0", "0", StatusUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := ObligationView{Required: MustMoney(c.required)}.
				fill(MustMoney(c.counted), c.condone, c.passed)
			if v.FulfilledPct.Pct() != c.pct {
				t.Fatalf("fulfilled = %s, want %s", v.FulfilledPct.Pct(), c.pct)
			}
			if v.Shortfall != MustMoney(c.shortfall) {
				t.Fatalf("shortfall = %s, want %s", v.Shortfall, c.shortfall)
			}
			if v.Condoned != MustMoney(c.condoned) {
				t.Fatalf("condoned = %s, want %s", v.Condoned, c.condoned)
			}
			if v.NetShortfall != MustMoney(c.net) {
				t.Fatalf("net shortfall = %s, want %s", v.NetShortfall, c.net)
			}
			if v.Status != c.status {
				t.Fatalf("status = %q, want %q", v.Status, c.status)
			}
		})
	}
}

// blockLicence is an EPCG licence whose period runs 1 April 2024 to 31 March
// 2030, with block 1 closing 31 March 2028.
func blockLicence(shipments ...Shipment) Licence {
	return Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEOExempt:    true,
		Shipments:          shipments,
	}.Normalise()
}

func TestAssessBlocks(t *testing.T) {
	t.Run("the requirement is cumulative and the dates are right", func(t *testing.T) {
		l := blockLicence()
		plan := PlanPeriod(l)
		got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate("2026-11-24"))
		if len(got) != 2 {
			t.Fatalf("got %d blocks", len(got))
		}
		if got[0].Window.To.String() != "2028-03-31" {
			t.Fatalf("block 1 ends %s, want 2028-03-31", got[0].Window.To)
		}
		if got[0].Required != MustMoney("3,00,00,000") {
			t.Fatalf("block 1 requires %s, want half of six crore", got[0].Required)
		}
		if got[1].Required != MustMoney("6,00,00,000") {
			t.Fatalf("block 2 requires %s, want the whole obligation", got[1].Required)
		}
		// Block 2's clock runs from the START of the period, not from the start
		// of the block: the requirement is cumulative, so the time to meet it
		// is too.
		if got[1].Clock.Window.From.String() != "2024-04-01" {
			t.Fatalf("block 2's clock begins %s, want the start of the period", got[1].Clock.Window.From)
		}
		if got[1].Clock.NotStarted {
			t.Fatalf("block 2's cumulative clock has been running since the period began")
		}
	})

	t.Run("on track, behind, met and overdue", func(t *testing.T) {
		cases := []struct {
			name string
			fob  string
			asOf string
			want Status
		}{
			// Block 1 needs ₹3 crore by 31 March 2028. One year in, a quarter
			// of it done is the pace that reaches it.
			{"a quarter done after a quarter of the time", "75,00,000", "2025-03-31", StatusOnTrack},
			{"a tenth done after a quarter of the time", "30,00,000", "2025-03-31", StatusBehind},
			{"done early", "3,00,00,000", "2025-03-31", StatusMet},
			{"short on the closing day is not yet overdue", "1,00,00,000", "2028-03-31", StatusBehind},
			{"short the day after is overdue", "1,00,00,000", "2028-04-01", StatusOverdue},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				l := blockLicence(ship("2024-06-01", c.fob, true))
				plan := PlanPeriod(l)
				got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate(c.asOf))
				if got[0].Status != c.want {
					t.Fatalf("block 1 is %q with %s counted on %s, want %q",
						got[0].Status, got[0].Counted, c.asOf, c.want)
				}
			})
		}
	})

	t.Run("a block is counted cumulatively from the start of the period", func(t *testing.T) {
		l := blockLicence(
			ship("2025-06-01", "1,00,00,000", true), // inside block 1
			ship("2029-06-01", "2,00,00,000", true), // inside block 2
		)
		plan := PlanPeriod(l)
		got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate("2030-03-31"))
		if got[0].Counted != MustMoney("1,00,00,000") {
			t.Fatalf("block 1 counted %s — only what fell inside it", got[0].Counted)
		}
		if got[1].Counted != MustMoney("3,00,00,000") {
			t.Fatalf("block 2 counted %s — everything from the start of the period", got[1].Counted)
		}
	})

	t.Run("an open block counts only up to today", func(t *testing.T) {
		l := blockLicence(
			ship("2025-06-01", "1,00,00,000", true),
			ship("2027-06-01", "1,00,00,000", true), // still in block 1, but in the future
		)
		plan := PlanPeriod(l)
		got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate("2026-11-24"))
		if got[0].Counted != MustMoney("1,00,00,000") {
			t.Fatalf("counted %s — a future shipment is not a discharge", got[0].Counted)
		}
		if !got[0].Current {
			t.Fatalf("today falls inside block 1")
		}
		if got[0].Closed {
			t.Fatalf("block 1 has not closed")
		}
		if got[1].Current || got[1].Closed {
			t.Fatalf("block 2 is neither current nor closed on this day")
		}
	})

	t.Run("the run rate that closes a block", func(t *testing.T) {
		l := blockLicence(ship("2024-06-01", "1,80,00,000", true))
		plan := PlanPeriod(l)
		// 24 whole months from 31 March 2026 to 31 March 2028, ₹1.2 crore to go.
		got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate("2026-03-31"))
		if got[0].Shortfall != MustMoney("1,20,00,000") {
			t.Fatalf("shortfall = %s", got[0].Shortfall)
		}
		if got[0].MonthsLeft != 24 {
			t.Fatalf("months left = %d, want 24", got[0].MonthsLeft)
		}
		if got[0].NeededPerMonth != MustMoney("5,00,000") {
			t.Fatalf("needed per month = %s, want 5,00,000", got[0].NeededPerMonth)
		}
	})

	t.Run("less than a month left: it is all due now", func(t *testing.T) {
		l := blockLicence()
		plan := PlanPeriod(l)
		got := AssessBlocks(l, l.Policy, plan, MustMoney("6,00,00,000"), MustDate("2028-03-20"))
		if got[0].MonthsLeft != 0 {
			t.Fatalf("months left = %d", got[0].MonthsLeft)
		}
		if got[0].NeededPerMonth != got[0].Shortfall {
			t.Fatalf("needed per month = %s, shortfall = %s — with under a month left they are the same",
				got[0].NeededPerMonth, got[0].Shortfall)
		}
	})

	t.Run("no blocks on an Advance Authorisation", func(t *testing.T) {
		l := Licence{Scheme: SchemeAdvanceAuthorisation, IssuedOn: MustDate("2026-04-20"),
			EOValueEndorsed: MustMoney("1,00,00,000")}.Normalise()
		if got := AssessBlocks(l, l.Policy, PlanPeriod(l), MustMoney("1,00,00,000"), MustDate("2026-11-24")); got != nil {
			t.Fatalf("got %d blocks", len(got))
		}
	})
}

func TestQuantityObligation(t *testing.T) {
	running := ClockFor(Window{From: MustDate("2026-04-20"), To: MustDate("2027-10-19")}, MustDate("2026-11-24"))
	over := ClockFor(Window{From: MustDate("2026-04-20"), To: MustDate("2027-10-19")}, MustDate("2027-10-20"))

	cases := []struct {
		name     string
		scheme   Scheme
		required string
		counted  string
		clock    Clock
		applies  bool
		pct      string
		short    string
		status   Status
	}{
		{"on track half way through", SchemeAdvanceAuthorisation, "180000", "103000", running, true, "57.22%", "77000", StatusOnTrack},
		{"behind", SchemeAdvanceAuthorisation, "180000", "10000", running, true, "5.56%", "170000", StatusBehind},
		{"met", SchemeAdvanceAuthorisation, "180000", "180000", running, true, "100%", "0", StatusMet},
		{"short with the period over", SchemeAdvanceAuthorisation, "180000", "170000", over, true, "94.44%", "10000", StatusOverdue},
		{"a quantity obligation does not arise on EPCG", SchemeEPCG, "180000", "103000", running, false, "0%", "0", StatusUnknown},
		{"no quantity entered", SchemeAdvanceAuthorisation, "0", "103000", running, false, "0%", "0", StatusUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := Licence{Scheme: c.scheme, EOQuantity: MustQuantity(c.required), EOQuantityUnit: "pieces"}
			got := QuantityObligation(l, MustQuantity(c.counted), c.clock)
			if got.Applies != c.applies {
				t.Fatalf("applies = %v, want %v", got.Applies, c.applies)
			}
			if got.FulfilledPct.Pct() != c.pct {
				t.Fatalf("fulfilled = %s, want %s", got.FulfilledPct.Pct(), c.pct)
			}
			if got.Shortfall != MustQuantity(c.short) {
				t.Fatalf("shortfall = %s, want %s", got.Shortfall, c.short)
			}
			if got.Status != c.status {
				t.Fatalf("status = %q, want %q", got.Status, c.status)
			}
		})
	}
}

func TestValueAddition(t *testing.T) {
	// VA = ((A − B) ÷ B) × 100. With inputs of ₹92 lakh, an export realised at
	// ₹1.38 crore is exactly 50% value addition.
	base := Licence{
		Scheme:  SchemeAdvanceAuthorisation,
		Imports: []Import{{CIFValue: MustMoney("92,00,000"), DutySaved: MustMoney("13,80,000")}},
	}

	cases := []struct {
		name       string
		mut        func(*Licence)
		realised   string
		shipped    string
		measurable bool
		onRealised string
		onShipped  string
		met        bool
		status     Status
	}{
		{"fifty percent value addition", nil, "1,38,00,000", "1,38,00,000", true, "50%", "50%", true, StatusMet},
		{"exactly the minimum", nil, "1,05,80,000", "1,05,80,000", true, "15%", "15%", true, StatusMet},
		{"a paisa under the minimum", nil, "1,05,79,999.99", "1,05,79,999.99", true, "15%", "15%", false, StatusBehind},
		{"below the minimum with the exports finished", nil, "1,00,00,000", "1,00,00,000", true, "8.7%", "8.7%", false, StatusBehind},
		{"part way through, not yet measurable", nil, "41,00,000", "78,50,000", false, "-55.43%", "-14.67%", false, StatusUnknown},
		{
			"other imported inputs belong in the denominator and reduce the value addition",
			func(l *Licence) { l.OtherInputsCIF = MustMoney("28,00,000") },
			"1,38,00,000", "1,38,00,000", true, "15%", "15%", true, StatusMet,
		},
		{
			"no inputs entered: the formula has no denominator",
			func(l *Licence) { l.Imports = nil },
			"1,38,00,000", "1,38,00,000", true, "0%", "0%", false, StatusUnknown,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := base
			if c.mut != nil {
				c.mut(&l)
			}
			l = l.Normalise()
			got := ValueAddition(l, l.Policy, MustMoney(c.realised), MustMoney(c.shipped), c.measurable)
			if !got.Applies {
				t.Fatalf("value addition applies to an Advance Authorisation")
			}
			if got.MinRequired != Percent(15, 0) {
				t.Fatalf("minimum = %s, want 15%%", got.MinRequired.Pct())
			}
			if got.OnRealised.Pct() != c.onRealised {
				t.Fatalf("on realisation = %s, want %s", got.OnRealised.Pct(), c.onRealised)
			}
			if got.OnShipped.Pct() != c.onShipped {
				t.Fatalf("on shipping bills = %s, want %s", got.OnShipped.Pct(), c.onShipped)
			}
			if got.Met != c.met {
				t.Fatalf("met = %v, want %v", got.Met, c.met)
			}
			if got.Status != c.status {
				t.Fatalf("status = %q, want %q", got.Status, c.status)
			}
			if got.Status != StatusMet && got.Note == "" {
				t.Fatalf("a value addition that is not met must say why")
			}
		})
	}

	t.Run("does not arise on EPCG", func(t *testing.T) {
		l := Licence{Scheme: SchemeEPCG, Imports: base.Imports}.Normalise()
		got := ValueAddition(l, l.Policy, MustMoney("1,00,00,000"), MustMoney("1,00,00,000"), true)
		if got.Applies {
			t.Fatalf("value addition is an Advance Authorisation requirement")
		}
	})

	t.Run("the denominator is the sum of both input figures", func(t *testing.T) {
		l := base
		l.OtherInputsCIF = MustMoney("28,00,000")
		l = l.Normalise()
		got := ValueAddition(l, l.Policy, MustMoney("1,38,00,000"), MustMoney("1,38,00,000"), true)
		if got.DenominatorB != MustMoney("1,20,00,000") {
			t.Fatalf("denominator = %s, want 1,20,00,000", got.DenominatorB)
		}
	})
}

func TestPaceReaches(t *testing.T) {
	w := Window{From: MustDate("2024-01-01"), To: MustDate("2024-12-31")}
	cases := []struct {
		name     string
		counted  string
		required string
		asOf     string
		want     bool
	}{
		{"already there", "100", "100", "2024-06-30", true},
		{"over", "150", "100", "2024-06-30", true},
		{"exactly on pace half way", "50", "100", "2024-07-01", true},
		{"behind pace half way", "40", "100", "2024-07-01", false},
		{"nothing done at all", "0", "100", "2024-07-01", false},
		{"short on the last day", "99", "100", "2024-12-31", false},
		{"short after the period", "99", "100", "2025-01-01", false},
		{"before the period begins", "0", "100", "2023-12-01", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := paceReaches(MustMoney(c.counted), MustMoney(c.required), ClockFor(w, MustDate(c.asOf)))
			if got != c.want {
				t.Fatalf("paceReaches = %v, want %v", got, c.want)
			}
		})
	}
}
