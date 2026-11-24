package eo

import (
	"strings"
	"testing"
)

func alertKeys(a Assessment) map[string]Alert {
	out := map[string]Alert{}
	for _, al := range a.Alerts {
		out[al.Key] = al
	}
	return out
}

func requirement(v EODCView, key string) (Requirement, bool) {
	for _, r := range v.Requirements {
		if r.Key == key {
			return r, true
		}
	}
	return Requirement{}, false
}

func TestAssessTheWorkedEPCGExample(t *testing.T) {
	l := SampleEPCG().Normalise()
	if err := l.Validate(); err != nil {
		t.Fatalf("the worked example must be a valid licence: %v", err)
	}
	a := Assess(l, MustDate("2026-11-24"))

	// The point of the example: gross exports exceed the obligation, and less
	// than half of it is discharged, because the average nets most of it off.
	if a.Tally.TotalFOB <= a.Specific.Required {
		t.Fatalf("gross exports %s should exceed the obligation %s", a.Tally.TotalFOB, a.Specific.Required)
	}
	if a.Specific.Counted >= a.Tally.TotalFOB {
		t.Fatalf("counted %s cannot be the gross %s once the average is netted", a.Specific.Counted, a.Tally.TotalFOB)
	}
	if a.Specific.Required != MustMoney("3,60,00,000") {
		t.Fatalf("obligation = %s", a.Specific.Required)
	}
	if a.Specific.Source != SourceEndorsed {
		t.Fatalf("the example endorses its obligation, source = %q", a.Specific.Source)
	}
	if a.Specific.Counted != MustMoney("1,60,17,965.42") {
		t.Fatalf("counted = %s, want 1,60,17,965.42", a.Specific.Counted)
	}
	if a.Specific.FulfilledPct.Pct() != "44.49%" {
		t.Fatalf("discharged = %s, want 44.49%%", a.Specific.FulfilledPct.Pct())
	}

	// The clock.
	if a.Period.PeriodWindow().String() != "15 May 2023 to 14 May 2029" {
		t.Fatalf("period = %s", a.Period.PeriodWindow())
	}
	if a.Period.EndsSource != SourceDerived {
		t.Fatalf("no end date is endorsed in the example, source = %q", a.Period.EndsSource)
	}
	if a.Clock.DaysLeft != 903 {
		t.Fatalf("days left = %d, want 903", a.Clock.DaysLeft)
	}

	// Block 1 closes 14 May 2027 needing half the obligation.
	if len(a.Blocks) != 2 {
		t.Fatalf("got %d blocks", len(a.Blocks))
	}
	if a.Blocks[0].Window.To.String() != "2027-05-14" {
		t.Fatalf("block 1 ends %s", a.Blocks[0].Window.To)
	}
	if a.Blocks[0].Required != MustMoney("1,80,00,000") {
		t.Fatalf("block 1 requires %s", a.Blocks[0].Required)
	}
	if !a.Blocks[0].Current {
		t.Fatalf("today falls inside block 1")
	}

	// A licensing year closed below the average, and it is reported as such.
	if a.Average.YearsClosedShort != 1 {
		t.Fatalf("%d closed years short, want 1", a.Average.YearsClosedShort)
	}
	if a.Average.ClosedShortfall != MustMoney("10,00,000") {
		t.Fatalf("closed shortfall = %s, want 10,00,000", a.Average.ClosedShortfall)
	}
	if a.Average.StillNeededThisYear != MustMoney("21,00,000") {
		t.Fatalf("still needed this year = %s, want 21,00,000", a.Average.StillNeededThisYear)
	}
	if a.Average.CurrentYearCloses.String() != "2027-03-31" {
		t.Fatalf("the current licensing year closes %s", a.Average.CurrentYearCloses)
	}
	// No year beyond today may appear: a year that has not happened cannot
	// have been failed.
	for _, y := range a.Average.Years {
		if y.Window.From.After(a.AsOf) {
			t.Fatalf("year %s begins after today", y.Label)
		}
	}

	// The unendorsed shipping bill is reported and not counted.
	if len(a.Tally.NotEndorsed) != 1 {
		t.Fatalf("%d unendorsed bills, want 1", len(a.Tally.NotEndorsed))
	}
	if _, ok := alertKeys(a)["not-endorsed"]; !ok {
		t.Fatalf("an unendorsed shipping bill must raise an alert")
	}

	// The exposure is hypothetical but real in size, and interest is running.
	if !a.Exposure.Applies || !a.Exposure.Hypothetical {
		t.Fatalf("exposure = %+v", a.Exposure)
	}
	if a.Exposure.Interest <= 0 {
		t.Fatalf("interest has been running since 2023, got %s", a.Exposure.Interest)
	}
	if a.Exposure.Total <= a.Exposure.DutyPayable {
		t.Fatalf("total %s must exceed the duty %s", a.Exposure.Total, a.Exposure.DutyPayable)
	}
	if a.Exposure.Daily() <= 0 {
		t.Fatalf("a day of delay must cost something")
	}

	// The licence is not closeable, and the tool says what is in the way.
	if a.EODC.Ready {
		t.Fatalf("this licence is nowhere near closeable")
	}
	if a.EODC.BlockingCount == 0 {
		t.Fatalf("something must be blocking it")
	}
	for _, key := range []string{"specific-eo", "average-eo", "blocks", "period",
		"endorsement", "realisation", "installation", "bills-of-entry", "application"} {
		if _, ok := requirement(a.EODC, key); !ok {
			t.Fatalf("no closure requirement %q", key)
		}
	}
	if r, _ := requirement(a.EODC, "endorsement"); r.State != ReqMissing {
		t.Fatalf("the unendorsed bill must show on the checklist, state = %q", r.State)
	}

	// The headline is the shortfall, not the exposure, while the period runs.
	if !strings.Contains(a.Headline.Label, "Still to export") {
		t.Fatalf("headline = %+v", a.Headline)
	}

	// Every assumption is written down.
	if len(a.Notes) < 8 {
		t.Fatalf("only %d notes; the assumptions must be stated", len(a.Notes))
	}
	joined := strings.Join(a.Notes, " ")
	for _, want := range []string{"INDICATIVE", "DGFT portal", "SIMPLE interest", "5.18"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the notes do not mention %q", want)
		}
	}
}

func TestAssessTheWorkedAdvanceAuthorisationExample(t *testing.T) {
	l := SampleAdvanceAuthorisation().Normalise()
	if err := l.Validate(); err != nil {
		t.Fatalf("the worked example must be valid: %v", err)
	}
	a := Assess(l, MustDate("2026-11-24"))

	if a.Period.PeriodWindow().String() != "20 April 2026 to 19 October 2027" {
		t.Fatalf("period = %s", a.Period.PeriodWindow())
	}
	if len(a.Blocks) != 0 {
		t.Fatalf("an Advance Authorisation has no block-wise milestone, got %d", len(a.Blocks))
	}
	if !a.Average.Applies && a.Average.Note == "" {
		t.Fatalf("it must say why no average applies")
	}

	// The obligation is a quantity as well as a value, and both are tracked.
	if !a.Quantity.Applies {
		t.Fatalf("quantity must apply")
	}
	if a.Quantity.Required != MustQuantity("180000") || a.Quantity.Counted != MustQuantity("103000") {
		t.Fatalf("quantity %s of %s", a.Quantity.Counted, a.Quantity.Required)
	}
	if a.Specific.Counted != MustMoney("78,50,000") {
		t.Fatalf("value counted = %s", a.Specific.Counted)
	}

	// Value addition is not measurable half way through, and says so rather
	// than reporting a failure.
	if !a.ValueAddition.Applies {
		t.Fatalf("value addition must apply")
	}
	if a.ValueAddition.Status != StatusUnknown {
		t.Fatalf("value addition status = %q part way through; want unknown", a.ValueAddition.Status)
	}
	if !strings.Contains(a.ValueAddition.Note, "not complete") {
		t.Fatalf("note = %q", a.ValueAddition.Note)
	}
	if a.ValueAddition.DenominatorB != MustMoney("92,00,000") {
		t.Fatalf("denominator = %s", a.ValueAddition.DenominatorB)
	}

	// Early fulfilment does not arise.
	if a.EarlyFulfilment.Applies {
		t.Fatalf("early fulfilment is an EPCG provision")
	}

	// The closure list names the right form and the six-month deadline.
	r, ok := requirement(a.EODC, "application")
	if !ok {
		t.Fatalf("no application requirement")
	}
	for _, want := range []string{"ANF 4F", "4.43(b)", "show cause"} {
		if !strings.Contains(r.Action, want) {
			t.Fatalf("the application action does not mention %q: %q", want, r.Action)
		}
	}
	if _, ok := requirement(a.EODC, "quantity-eo"); !ok {
		t.Fatalf("no quantity requirement on an Advance Authorisation")
	}
	if _, ok := requirement(a.EODC, "value-addition"); !ok {
		t.Fatalf("no value-addition requirement on an Advance Authorisation")
	}
	if _, ok := requirement(a.EODC, "installation"); ok {
		t.Fatalf("an installation certificate is an EPCG requirement, not an Advance Authorisation one")
	}

	joined := strings.Join(a.Notes, " ")
	if !strings.Contains(joined, "Appendix 4J") {
		t.Fatalf("an Advance Authorisation must be warned about the shorter Appendix 4J periods")
	}
}

func TestAssessEarlyFulfilment(t *testing.T) {
	// A period of 1 April 2024 to 31 March 2030 is 2191 days, so half of it
	// ends on 31 March 2027 — exactly three licensing years in.
	base := func(fob string) Licence {
		return Licence{
			Scheme:             SchemeEPCG,
			IssuedOn:           MustDate("2024-04-01"),
			Category:           CategoryAsEndorsed,
			SpecificEOEndorsed: MustMoney("6,00,00,000"),
			AverageEOExempt:    true,
			Imports: []Import{{BillOfEntryNo: "X", ClearedOn: MustDate("2024-05-01"),
				DutySaved: MustMoney("1,00,00,000")}},
			Shipments: []Shipment{ship("2024-06-01", fob, true)},
		}.Normalise()
	}

	t.Run("the half-way date and the threshold", func(t *testing.T) {
		a := Assess(base("1,00,00,000"), MustDate("2025-04-01"))
		if a.EarlyFulfilment.Window.To.String() != "2027-03-31" {
			t.Fatalf("half-way date = %s, want 2027-03-31", a.EarlyFulfilment.Window.To)
		}
		if a.EarlyFulfilment.Required != MustMoney("4,50,00,000") {
			t.Fatalf("needed by then = %s, want 75%% of six crore", a.EarlyFulfilment.Required)
		}
		if a.EarlyFulfilment.Status != StatusBehind {
			t.Fatalf("status = %q", a.EarlyFulfilment.Status)
		}
		if a.EarlyFulfilment.Shortfall != MustMoney("3,50,00,000") {
			t.Fatalf("shortfall = %s", a.EarlyFulfilment.Shortfall)
		}
		if _, ok := alertKeys(a)["early-fulfilment"]; !ok {
			t.Fatalf("a provision worth lakhs must be surfaced as an alert")
		}
	})

	t.Run("earned, before the half-way date", func(t *testing.T) {
		a := Assess(base("4,50,00,000"), MustDate("2025-04-01"))
		if !a.EarlyFulfilment.Eligible {
			t.Fatalf("75%% with the average maintained is eligible: %+v", a.EarlyFulfilment)
		}
		if a.EarlyFulfilment.Decided {
			t.Fatalf("the half-way date has not passed")
		}
		if a.EarlyFulfilment.Condonable != MustMoney("1,50,00,000") {
			t.Fatalf("condonable = %s, want the balance of 1,50,00,000", a.EarlyFulfilment.Condonable)
		}
		if al, ok := alertKeys(a)["early-fulfilment"]; !ok || al.Level != AlertGood {
			t.Fatalf("alert = %+v", al)
		}
	})

	t.Run("earned, and the half-way date has passed", func(t *testing.T) {
		a := Assess(base("4,50,00,000"), MustDate("2027-04-01"))
		if !a.EarlyFulfilment.Eligible || !a.EarlyFulfilment.Decided {
			t.Fatalf("%+v", a.EarlyFulfilment)
		}
		if !strings.Contains(a.EarlyFulfilment.Note, "condones the balance") {
			t.Fatalf("note = %q", a.EarlyFulfilment.Note)
		}
	})

	t.Run("missed", func(t *testing.T) {
		a := Assess(base("3,00,00,000"), MustDate("2027-04-01"))
		if a.EarlyFulfilment.Eligible {
			t.Fatalf("75%% was not reached")
		}
		if a.EarlyFulfilment.Status != StatusOverdue {
			t.Fatalf("status = %q", a.EarlyFulfilment.Status)
		}
		if !strings.Contains(a.EarlyFulfilment.Note, "no longer available") {
			t.Fatalf("note = %q", a.EarlyFulfilment.Note)
		}
	})

	t.Run("barred where another concession has been taken", func(t *testing.T) {
		// FTP 5.04(l): only one of the benefits in 5.04(d), 5.09, 5.10 and
		// 5.11 is admissible.
		for _, cat := range []Category{CategoryGreenTechnology, CategorySpecifiedArea, CategoryIndigenousSourcing} {
			l := base("4,50,00,000")
			l.Category = cat
			l.SpecificEOEndorsed = 0
			l = l.Normalise()
			a := Assess(l, MustDate("2025-04-01"))
			if a.EarlyFulfilment.Applies {
				t.Fatalf("%s: early fulfilment must not apply alongside another concession", cat)
			}
			if !a.EarlyFulfilment.Excluded {
				t.Fatalf("%s: it must say it is excluded", cat)
			}
			if !strings.Contains(a.EarlyFulfilment.Note, "5.04(l)") {
				t.Fatalf("%s: note = %q", cat, a.EarlyFulfilment.Note)
			}
		}
	})

	t.Run("the average must be maintained too", func(t *testing.T) {
		l := base("4,50,00,000")
		l.AverageEOExempt = false
		l.AverageEO = MustMoney("5,00,00,000") // impossible to maintain here
		l.Shipments[0].SameOrSimilar = true
		l = l.Normalise()
		a := Assess(l, MustDate("2027-04-01"))
		if a.EarlyFulfilment.Eligible {
			t.Fatalf("FTP 5.09 needs 100%% of the average as well as 75%% of the specific")
		}
		if a.EarlyFulfilment.AverageMaintained {
			t.Fatalf("the average was not maintained")
		}
	})
}

func TestAssessAverageOffsetOnAnOverallBasis(t *testing.T) {
	// HBP 5.18: excess in one year may offset a shortfall in another, provided
	// the average is maintained on an overall basis. ₹1.5 crore in one year and
	// ₹60 lakh in the next is ₹2.1 crore against two years of a ₹1 crore
	// average — short in the second year, and ahead overall.
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEO:          MustMoney("1,00,00,000"),
		Shipments: []Shipment{
			ship("2024-06-01", "1,50,00,000", true),
			ship("2025-06-01", "60,00,000", true),
		},
	}.Normalise()

	a := Assess(l, MustDate("2026-04-15"))

	if a.Average.AllYearsMet {
		t.Fatalf("2025-26 closed ₹40 lakh below the average")
	}
	if a.Average.YearsClosedShort != 1 {
		t.Fatalf("%d closed years short, want 1", a.Average.YearsClosedShort)
	}
	if a.Average.OverallAchieved != MustMoney("2,10,00,000") {
		t.Fatalf("overall achieved = %s", a.Average.OverallAchieved)
	}
	if !a.Average.OverallMet {
		t.Fatalf("overall, %s against %s is maintained", a.Average.OverallAchieved, a.Average.DueTotal)
	}
	if !a.Average.OffsetAvailable {
		t.Fatalf("this is exactly the case HBP 5.18 is for")
	}
	// The alert must be a warning rather than an urgent failure, and must say
	// what the provision is.
	al, ok := alertKeys(a)["average-closed-short"]
	if !ok {
		t.Fatalf("no alert about the short year")
	}
	if al.Level != AlertWarn {
		t.Fatalf("level = %q; a shortfall made up overall is not an outright failure", al.Level)
	}
	if !strings.Contains(al.Detail, "5.18") {
		t.Fatalf("detail = %q", al.Detail)
	}
	// And the closure checklist must point at the overall figure.
	r, _ := requirement(a.EODC, "average-eo")
	if !strings.Contains(r.Action, "5.18") {
		t.Fatalf("action = %q", r.Action)
	}

	// Specific-EO counting stays year-wise, which is the conservative reading:
	// only the ₹50 lakh excess from the first year counts.
	if a.Specific.Counted != MustMoney("50,00,000") {
		t.Fatalf("counted toward the obligation = %s, want 50,00,000 on the year-wise basis", a.Specific.Counted)
	}
}

func TestAssessAClosableLicence(t *testing.T) {
	l := Licence{
		Scheme:                SchemeEPCG,
		AuthorisationNo:       "CLOSE-1",
		IssuedOn:              MustDate("2020-04-01"),
		Category:              CategoryAsEndorsed,
		SpecificEOEndorsed:    MustMoney("6,00,00,000"),
		AverageEOExempt:       true,
		AverageEOExemptReason: "handicrafts sector, FTP 5.12(a)",
		Imports: []Import{{
			BillOfEntryNo: "7654321", ClearedOn: MustDate("2020-06-01"),
			CIFValue: MustMoney("4,00,00,000"), DutySaved: MustMoney("1,00,00,000"),
			InstalledOn: MustDate("2020-08-01"),
		}},
		Shipments: []Shipment{
			realisedShip("2021-06-01", "3,20,00,000"),
			realisedShip("2024-06-01", "3,00,00,000"),
		},
		Evidence: Evidence{
			CACertifiedStatement: true,
			RCMCHeld:             true,
			BondOrBGDetails:      "Bond 114/2020 with 15% bank guarantee",
		},
	}.Normalise()

	a := Assess(l, MustDate("2026-11-24"))

	if a.Specific.Status != StatusMet {
		t.Fatalf("the obligation is over-discharged: %+v", a.Specific)
	}
	if a.Exposure.Applies {
		t.Fatalf("nothing is payable on a discharged obligation")
	}
	if !a.Clock.Expired {
		t.Fatalf("the period ended 31 March 2026")
	}
	if !a.EODC.Ready {
		var blocking []string
		for _, r := range a.EODC.Requirements {
			if r.Blocking && !r.Satisfied() {
				blocking = append(blocking, r.Key+"="+string(r.State)+" ("+r.Detail+")")
			}
		}
		t.Fatalf("this licence should be closeable; blocking: %v", blocking)
	}
	if al, ok := alertKeys(a)["ready"]; !ok || al.Level != AlertGood {
		t.Fatalf("a closeable licence must say so: %+v", al)
	}
	if a.Headline.Level != AlertGood {
		t.Fatalf("headline = %+v", a.Headline)
	}
	// The application itself is not a precondition for being ready to apply.
	if r, _ := requirement(a.EODC, "application"); r.Blocking {
		t.Fatalf("filing the application cannot be a precondition of being ready to file it")
	}
	if r, _ := requirement(a.EODC, "average-eo"); r.State != ReqNotApplicable {
		t.Fatalf("an exempt licence has no average to maintain, state = %q", r.State)
	}
}

func TestAssessBlankAverageBlocksTheChecklist(t *testing.T) {
	// Silence is not an exemption: an average left blank makes the obligation
	// look more discharged than it is.
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2020-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("1,00,00,000"),
		Imports: []Import{{BillOfEntryNo: "1", ClearedOn: MustDate("2020-06-01"),
			DutySaved: MustMoney("20,00,000"), InstalledOn: MustDate("2020-07-01")}},
		Shipments: []Shipment{realisedShip("2021-06-01", "1,50,00,000")},
		Evidence:  Evidence{CACertifiedStatement: true, RCMCHeld: true, BondOrBGDetails: "Bond 1"},
	}.Normalise()

	a := Assess(l, MustDate("2026-11-24"))
	if a.Specific.Status != StatusMet {
		t.Fatalf("the value obligation is met")
	}
	r, ok := requirement(a.EODC, "average-eo")
	if !ok {
		t.Fatalf("no average requirement")
	}
	if r.State != ReqUnknown {
		t.Fatalf("a blank average must read as unknown, not as not-applicable; got %q", r.State)
	}
	if a.EODC.Ready {
		t.Fatalf("an unanswered average must block the checklist")
	}
	if !strings.Contains(strings.Join(a.Notes, " "), "OVERSTATES") {
		t.Fatalf("the notes must warn that a blank average overstates the discharge")
	}
}

func TestAssessAnExpiredPeriodWithAShortfall(t *testing.T) {
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2018-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("6,00,00,000"),
		AverageEOExempt:    true,
		Imports: []Import{{BillOfEntryNo: "9", ClearedOn: MustDate("2018-06-01"),
			DutySaved: MustMoney("1,00,00,000"), InstalledOn: MustDate("2018-08-01")}},
		Shipments: []Shipment{realisedShip("2019-06-01", "2,00,00,000")},
	}.Normalise()

	a := Assess(l, MustDate("2026-11-24"))

	if !a.Clock.Expired {
		t.Fatalf("the period ended 31 March 2024")
	}
	if a.Specific.Status != StatusOverdue {
		t.Fatalf("status = %q", a.Specific.Status)
	}
	if a.Exposure.Hypothetical {
		t.Fatalf("the period has ended short; this is not hypothetical")
	}
	keys := alertKeys(a)
	if al, ok := keys["period-expired"]; !ok || al.Level != AlertUrgent {
		t.Fatalf("an expired period with a shortfall is urgent: %+v", al)
	}
	// Both blocks closed short, and the payment window is named with its
	// customs authority.
	for _, no := range []string{"1", "2"} {
		al, ok := keys["block-"+no+"-overdue"]
		if !ok {
			t.Fatalf("block %s closed short and must be reported", no)
		}
		if !strings.Contains(al.Detail, "26/2023-Customs") {
			t.Fatalf("block %s detail must name the notification: %q", no, al.Detail)
		}
	}
	// The headline becomes the money.
	if !strings.Contains(a.Headline.Label, "Duty and interest") {
		t.Fatalf("headline = %+v", a.Headline)
	}
	if a.Headline.Level != AlertUrgent {
		t.Fatalf("headline level = %q", a.Headline.Level)
	}
}

func TestAssessInstallationCertificateHasTwoDeadlines(t *testing.T) {
	l := Licence{
		Scheme:             SchemeEPCG,
		IssuedOn:           MustDate("2024-04-01"),
		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("1,00,00,000"),
		AverageEOExempt:    true,
		Imports: []Import{{BillOfEntryNo: "1", ClearedOn: MustDate("2024-05-01"),
			DutySaved: MustMoney("20,00,000")}},
	}.Normalise()

	cases := []struct {
		name  string
		asOf  string
		alert string
	}{
		{"inside both deadlines", "2024-09-01", "installation-pending"},
		{"past the six-month customs deadline", "2024-12-01", "installation-customs-overdue"},
		{"past the three-year Regional Authority deadline", "2027-06-01", "installation-overdue"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Assess(l, MustDate(c.asOf))
			al, ok := alertKeys(a)[c.alert]
			if !ok {
				t.Fatalf("want alert %q, got %v", c.alert, alertKeys(a))
			}
			if al.Detail == "" {
				t.Fatalf("the alert must explain itself")
			}
		})
	}

	// Once the certificate is recorded, nothing is reported.
	done := l
	done.Imports[0].InstalledOn = MustDate("2024-06-01")
	done = done.Normalise()
	a := Assess(done, MustDate("2027-06-01"))
	for key := range alertKeys(a) {
		if strings.HasPrefix(key, "installation") {
			t.Fatalf("a recorded certificate must not raise %q", key)
		}
	}
	if r, _ := requirement(a.EODC, "installation"); r.State != ReqMet {
		t.Fatalf("state = %q", r.State)
	}
}

func TestAssessWithoutEnoughEntered(t *testing.T) {
	t.Run("no obligation figure", func(t *testing.T) {
		l := Licence{Scheme: SchemeEPCG, IssuedOn: MustDate("2024-04-01")}.Normalise()
		a := Assess(l, MustDate("2026-11-24"))
		if a.Specific.Status != StatusUnknown {
			t.Fatalf("status = %q", a.Specific.Status)
		}
		if a.Exposure.Applies {
			t.Fatalf("no obligation, no exposure")
		}
		if a.Headline.Value != "not entered" {
			t.Fatalf("headline = %+v", a.Headline)
		}
		if r, _ := requirement(a.EODC, "specific-eo"); r.State != ReqUnknown {
			t.Fatalf("state = %q", r.State)
		}
	})

	t.Run("an as-of date defaults to today", func(t *testing.T) {
		l := SampleEPCG().Normalise()
		a := Assess(l, Date{})
		if a.AsOf.Zero() {
			t.Fatalf("Assess must fall back to today")
		}
	})

	t.Run("assessment is deterministic", func(t *testing.T) {
		l := SampleEPCG().Normalise()
		first := Assess(l, MustDate("2026-11-24"))
		second := Assess(l, MustDate("2026-11-24"))
		if first.Specific.Counted != second.Specific.Counted ||
			first.Exposure.Total != second.Exposure.Total ||
			len(first.Alerts) != len(second.Alerts) {
			t.Fatalf("the same licence on the same day must give the same answer")
		}
	})
}

func TestReportIsCompleteAndSelfConsistent(t *testing.T) {
	for _, l := range []Licence{SampleEPCG().Normalise(), SampleAdvanceAuthorisation().Normalise()} {
		a := Assess(l, MustDate("2026-11-24"))
		body := Report(l, a)

		for _, want := range []string{
			"EXPORT OBLIGATION STATEMENT",
			"THE CLOCK",
			"THE OBLIGATION IN VALUE",
			"EXPORTS COUNTED",
			"IMPORTS AGAINST THIS AUTHORISATION",
			"IF THE OBLIGATION IS NOT DISCHARGED",
			"CLOSING THE LICENCE",
			"FIGURES APPLIED, AND WHERE THEY COME FROM",
			"ASSUMPTIONS AND LIMITS",
			a.Specific.Required.Indian(),
			a.Specific.Counted.Indian(),
			a.AsOf.Human(),
			a.Policy.Edition,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s: the report does not contain %q", l.Scheme, want)
			}
		}

		// Every citation appears, so no figure can be read without its source.
		for _, c := range a.Policy.Citations() {
			if !strings.Contains(body, c.Label) {
				t.Fatalf("%s: the report omits the figure %q", l.Scheme, c.Label)
			}
		}

		// Every closure requirement appears with its state.
		for _, r := range a.EODC.Requirements {
			if !strings.Contains(body, r.Label) {
				t.Fatalf("%s: the report omits the requirement %q", l.Scheme, r.Label)
			}
		}

		// Nothing in it claims a filing or a portal connection.
		for _, forbidden := range []string{"filed on your behalf", "submitted to DGFT", "uploaded to ICEGATE"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s: the report claims %q", l.Scheme, forbidden)
			}
		}
		if !strings.Contains(body, "files nothing") {
			t.Fatalf("%s: the report must say it files nothing", l.Scheme)
		}

		// No line runs away: a report is read in a terminal and printed.
		for i, line := range strings.Split(body, "\n") {
			if len([]rune(line)) > 100 {
				t.Fatalf("%s: line %d is %d characters long: %q", l.Scheme, i+1, len([]rune(line)), line)
			}
		}
	}
}

func TestReportSectionsByScheme(t *testing.T) {
	epcg := SampleEPCG().Normalise()
	aa := SampleAdvanceAuthorisation().Normalise()

	epcgBody := Report(epcg, Assess(epcg, MustDate("2026-11-24")))
	aaBody := Report(aa, Assess(aa, MustDate("2026-11-24")))

	for _, want := range []string{"BLOCK-WISE MILESTONES", "AVERAGE EXPORT OBLIGATION", "EARLY FULFILMENT"} {
		if !strings.Contains(epcgBody, want) {
			t.Fatalf("the EPCG report omits %q", want)
		}
		if strings.Contains(aaBody, want) {
			t.Fatalf("the Advance Authorisation report should not contain %q", want)
		}
	}
	for _, want := range []string{"THE OBLIGATION IN QUANTITY", "VALUE ADDITION"} {
		if !strings.Contains(aaBody, want) {
			t.Fatalf("the Advance Authorisation report omits %q", want)
		}
		if strings.Contains(epcgBody, want) {
			t.Fatalf("the EPCG report should not contain %q", want)
		}
	}
}

func TestWrapAndPad(t *testing.T) {
	if got := pad("abc", 6); got != "abc   " {
		t.Fatalf("pad = %q", got)
	}
	// A value longer than its column is never truncated: a figure cut short is
	// a wrong figure.
	if got := pad("abcdefgh", 4); got != "abcdefgh " {
		t.Fatalf("pad = %q", got)
	}
	if got := wrap("one two three four", 9); got != "one two\nthree\nfour" {
		t.Fatalf("wrap = %q", got)
	}
	if got := wrapIndent("one two three", 7, "..."); got != "one two\n...three" {
		t.Fatalf("wrapIndent = %q", got)
	}
	if got := wrap("", 10); got != "" {
		t.Fatalf("wrap of nothing = %q", got)
	}
	// A single word longer than the width is left whole rather than broken.
	if got := wrap("authorisation", 5); got != "authorisation" {
		t.Fatalf("wrap = %q", got)
	}
}

// realisedShip is an endorsed, realised, same-or-similar export.
func realisedShip(date, fob string) Shipment {
	s := ship(date, fob, true)
	s.RealisedOn = MustDate(date).AddMonths(3)
	s.RealisedValue = MustMoney(fob)
	s.BRCNo = "BRC-" + date
	return s
}
