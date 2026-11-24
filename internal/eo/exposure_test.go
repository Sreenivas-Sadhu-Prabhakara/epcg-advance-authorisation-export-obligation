package eo

import "testing"

// exposureLicence: an EPCG licence with a ₹60 lakh obligation and ₹10 lakh of
// duty saved on a single consignment cleared on the day it was issued, so the
// interest arithmetic has exactly one clean period in it.
func exposureLicence() Licence {
	return Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-01-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("60,00,000"),
		AverageEOExempt:    true,
		Imports: []Import{{
			BillOfEntryNo: "1111111",
			ClearedOn:     MustDate("2024-01-01"),
			CIFValue:      MustMoney("40,00,000"),
			DutySaved:     MustMoney("10,00,000"),
		}},
		Shipments: []Shipment{ship("2024-06-01", "30,00,000", true)},
	}.Normalise()
}

func TestExposureArithmetic(t *testing.T) {
	l := exposureLicence()
	// Half the obligation discharged, so half the duty saved becomes payable:
	// ₹5,00,000. Interest at 15% for the 366 days of 2024 — a leap year — is
	// ₹5,00,000 × 15% × 366/365 = ₹75,205.48.
	got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
		MustDate("2025-01-01"), false)

	if !got.Applies {
		t.Fatalf("an exposure applies on a shortfall")
	}
	if got.ShortfallPct.Pct() != "50%" {
		t.Fatalf("unfulfilled proportion = %s, want 50%%", got.ShortfallPct.Pct())
	}
	if got.DutyPayable != MustMoney("5,00,000") {
		t.Fatalf("duty payable = %s, want 5,00,000", got.DutyPayable)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("got %d lines", len(got.Lines))
	}
	if got.Lines[0].Days != 366 {
		t.Fatalf("days = %d, want 366 — 2024 is a leap year", got.Lines[0].Days)
	}
	if got.Interest != MustMoney("75,205.48") {
		t.Fatalf("interest = %s, want 75,205.48", got.Interest)
	}
	if got.Total != MustMoney("5,75,205.48") {
		t.Fatalf("total = %s, want 5,75,205.48", got.Total)
	}
	if got.Daily() != MustMoney("205.48") {
		t.Fatalf("a day of delay = %s, want 205.48", got.Daily())
	}
	if !got.Hypothetical {
		t.Fatalf("the period is still running, so the figure is hypothetical")
	}
	if got.Understated != "" {
		t.Fatalf("nothing is missing here: %q", got.Understated)
	}
	// The lines must add up to the totals, or the report contradicts itself.
	var duty, interest Money
	for _, line := range got.Lines {
		duty += line.Proportionate
		interest += line.Interest
		if line.Total != line.Proportionate+line.Interest {
			t.Fatalf("line total %s does not match %s + %s", line.Total, line.Proportionate, line.Interest)
		}
	}
	if duty != got.DutyPayable || interest != got.Interest {
		t.Fatalf("lines sum to %s + %s, totals are %s + %s", duty, interest, got.DutyPayable, got.Interest)
	}
}

func TestExposureInterestRunsFromEachClearanceDate(t *testing.T) {
	// The reason the exposure is computed consignment by consignment: a
	// container cleared a year earlier has a year more interest on it. One
	// total and one date would understate the older one.
	l := exposureLicence()
	l.Imports = []Import{
		{BillOfEntryNo: "EARLY", ClearedOn: MustDate("2024-01-01"), DutySaved: MustMoney("5,00,000")},
		{BillOfEntryNo: "LATE", ClearedOn: MustDate("2025-01-01"), DutySaved: MustMoney("5,00,000")},
	}
	l = l.Normalise()

	got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
		MustDate("2026-01-01"), false)

	if len(got.Lines) != 2 {
		t.Fatalf("got %d lines", len(got.Lines))
	}
	early, late := got.Lines[0], got.Lines[1]
	if early.BillOfEntryNo != "EARLY" {
		t.Fatalf("lines are out of order: %q first", early.BillOfEntryNo)
	}
	if early.Proportionate != late.Proportionate {
		t.Fatalf("equal duty saved must give equal duty payable: %s and %s", early.Proportionate, late.Proportionate)
	}
	if early.Days != 731 || late.Days != 365 {
		t.Fatalf("days = %d and %d, want 731 and 365", early.Days, late.Days)
	}
	if early.Interest <= late.Interest {
		t.Fatalf("the earlier consignment must carry more interest: %s vs %s", early.Interest, late.Interest)
	}
	// ₹2,50,000 × 15% × 731/365 = ₹75,102.74; × 365/365 = ₹37,500.00.
	if early.Interest != MustMoney("75,102.74") {
		t.Fatalf("early interest = %s, want 75,102.74", early.Interest)
	}
	if late.Interest != MustMoney("37,500") {
		t.Fatalf("late interest = %s, want 37,500", late.Interest)
	}
}

func TestExposureReportsWhatItCannotCompute(t *testing.T) {
	t.Run("a consignment with no clearance date understates the exposure", func(t *testing.T) {
		l := exposureLicence()
		l.Imports = []Import{{BillOfEntryNo: "NODATE", DutySaved: MustMoney("10,00,000")}}
		l = l.Normalise()
		got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
			MustDate("2026-01-01"), false)
		if got.Interest != 0 {
			t.Fatalf("no clearance date, no period to run interest over, got %s", got.Interest)
		}
		if got.DutyPayable != MustMoney("5,00,000") {
			t.Fatalf("the duty itself is still payable: %s", got.DutyPayable)
		}
		if got.Understated == "" {
			t.Fatalf("a figure lower than the real exposure must say so")
		}
		if got.Lines[0].Problem == "" {
			t.Fatalf("the line must say why it has no interest on it")
		}
	})

	t.Run("no duty saved entered at all", func(t *testing.T) {
		l := exposureLicence()
		l.Imports = nil
		l = l.Normalise()
		got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
			MustDate("2026-01-01"), false)
		if got.Total != 0 {
			t.Fatalf("nothing to apply the proportion to, got %s", got.Total)
		}
		if got.Understated == "" {
			t.Fatalf("an exposure of zero that is not really zero must say so")
		}
	})

	t.Run("a consignment cleared after the day computed for carries no interest yet", func(t *testing.T) {
		l := exposureLicence()
		l.Imports = []Import{{BillOfEntryNo: "FUTURE", ClearedOn: MustDate("2027-01-01"), DutySaved: MustMoney("10,00,000")}}
		l = l.Normalise()
		got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
			MustDate("2026-01-01"), false)
		if got.Interest != 0 {
			t.Fatalf("interest cannot run backwards, got %s", got.Interest)
		}
		if got.Lines[0].Problem == "" {
			t.Fatalf("the line must explain itself")
		}
		if got.Understated != "" {
			t.Fatalf("nothing is missing here — the clearance simply has not happened: %q", got.Understated)
		}
	})
}

func TestExposureWhenNothingIsOwed(t *testing.T) {
	l := exposureLicence()

	t.Run("obligation discharged", func(t *testing.T) {
		got := Exposure(l, l.Policy, MustMoney("60,00,000"), 0, MustDate("2026-01-01"), true)
		if got.Applies || got.Total != 0 {
			t.Fatalf("nothing is payable on a discharged obligation: %+v", got)
		}
		if got.Basis == "" {
			t.Fatalf("even a nil exposure must say why it is nil")
		}
		if got.Daily() != 0 {
			t.Fatalf("a day of delay costs nothing, got %s", got.Daily())
		}
	})

	t.Run("no obligation figure to measure against", func(t *testing.T) {
		got := Exposure(l, l.Policy, 0, MustMoney("10,00,000"), MustDate("2026-01-01"), true)
		if got.Applies {
			t.Fatalf("a shortfall of an unknown obligation is not a figure")
		}
		if got.Basis == "" {
			t.Fatalf("it must say what is missing")
		}
	})
}

func TestExposureStopsBeingHypotheticalWhenThePeriodEnds(t *testing.T) {
	l := exposureLicence()
	running := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"), MustDate("2026-01-01"), false)
	ended := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"), MustDate("2026-01-01"), true)

	if !running.Hypothetical {
		t.Fatalf("while the period runs, the figure is what WOULD be payable")
	}
	if ended.Hypothetical {
		t.Fatalf("once the period has ended short, it is not hypothetical")
	}
	if running.Total != ended.Total {
		t.Fatalf("the arithmetic does not change, only what it means: %s vs %s", running.Total, ended.Total)
	}
	if running.Basis == ended.Basis {
		t.Fatalf("the two must be worded differently")
	}
}

func TestExposureAtAnEditedRate(t *testing.T) {
	l := exposureLicence()
	l.Policy.InterestRatePercent = 18
	l.Policy.InterestDayBasis = 360
	l = l.Normalise()

	got := Exposure(l, l.Policy, MustMoney("60,00,000"), MustMoney("30,00,000"),
		MustDate("2025-01-01"), false)
	if got.RateApplied != Percent(18, 0) {
		t.Fatalf("rate applied = %s, want 18%%", got.RateApplied.Pct())
	}
	if got.DayBasis != 360 {
		t.Fatalf("day basis = %d, want 360", got.DayBasis)
	}
	// ₹5,00,000 × 18% × 366/360 = ₹91,500.00.
	if got.Interest != MustMoney("91,500") {
		t.Fatalf("interest = %s, want 91,500", got.Interest)
	}
	if !l.Policy.Edited {
		t.Fatalf("the policy must be marked as edited so the report can say so")
	}
}

func TestExposureRoundsToWholePaise(t *testing.T) {
	// A shortfall that divides badly, to check that nothing comes out with a
	// fraction of a paisa in it and that the parts still add up.
	l := exposureLicence()
	l.Imports = []Import{
		{BillOfEntryNo: "A", ClearedOn: MustDate("2024-01-01"), DutySaved: MustMoney("3,33,333.33")},
		{BillOfEntryNo: "B", ClearedOn: MustDate("2024-02-29"), DutySaved: MustMoney("3,33,333.33")},
		{BillOfEntryNo: "C", ClearedOn: MustDate("2024-03-31"), DutySaved: MustMoney("3,33,333.34")},
	}
	l = l.Normalise()

	got := Exposure(l, l.Policy, MustMoney("70,00,000"), MustMoney("23,33,333.33"),
		MustDate("2026-11-24"), true)

	var duty, interest Money
	for _, line := range got.Lines {
		duty += line.Proportionate
		interest += line.Interest
	}
	if duty != got.DutyPayable {
		t.Fatalf("lines sum to %s, total is %s", duty, got.DutyPayable)
	}
	if interest != got.Interest {
		t.Fatalf("interest lines sum to %s, total is %s", interest, got.Interest)
	}
	if got.Total != got.DutyPayable+got.Interest {
		t.Fatalf("total %s does not equal %s + %s", got.Total, got.DutyPayable, got.Interest)
	}
	// Every figure is a whole number of paise by construction of the type; the
	// test that matters is that the proportion is right to the paisa.
	wantPct := Proportion(MustMoney("23,33,333.33"), MustMoney("70,00,000"))
	if got.ShortfallPct != wantPct {
		t.Fatalf("proportion = %d bips, want %d", got.ShortfallPct, wantPct)
	}
}
