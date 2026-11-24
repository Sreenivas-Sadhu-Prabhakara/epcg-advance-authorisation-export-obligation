package eo

import (
	"encoding/json"
	"testing"
)

func TestAsPublishedFigures(t *testing.T) {
	p := AsPublished()
	if p.EPCGMultipleHundredths != 600 {
		t.Fatalf("EPCG multiple = %d hundredths, want 600 (six times the duty saved)", p.EPCGMultipleHundredths)
	}
	if p.EPCGPeriodMonths != 72 {
		t.Fatalf("EPCG period = %d months, want 72", p.EPCGPeriodMonths)
	}
	if p.AAPeriodMonths != 18 {
		t.Fatalf("Advance Authorisation period = %d months, want 18", p.AAPeriodMonths)
	}
	if p.AAMinValueAdditionPercent != 15 {
		t.Fatalf("minimum value addition = %d%%, want 15", p.AAMinValueAdditionPercent)
	}
	if p.InterestRatePercent != 15 {
		t.Fatalf("interest = %d%%, want 15", p.InterestRatePercent)
	}
	if p.CondonationPercent != 0 {
		t.Fatalf("condonation defaults to %d%%, want 0 — this tool condones nothing unless told to", p.CondonationPercent)
	}
	// The reduced multiples. Indigenous sourcing is a QUARTER less, not a
	// tenth — FTP 2023 para 5.04(d) — which makes it the same 4.5 times as the
	// green technology reduction, by a different route.
	if p.EPCGGreenMultipleHundredths != 450 {
		t.Fatalf("green technology multiple = %d hundredths, want 450", p.EPCGGreenMultipleHundredths)
	}
	if p.EPCGSpecifiedAreaMultipleHundredths != 150 {
		t.Fatalf("specified area multiple = %d hundredths, want 150", p.EPCGSpecifiedAreaMultipleHundredths)
	}
	if p.EPCGIndigenousMultipleHundredths != 450 {
		t.Fatalf("indigenous sourcing multiple = %d hundredths, want 450 (a quarter less than six times)",
			p.EPCGIndigenousMultipleHundredths)
	}
	if p.EarlyFulfilmentPercent != 75 {
		t.Fatalf("early fulfilment threshold = %d%%, want 75", p.EarlyFulfilmentPercent)
	}
	// The two installation-certificate deadlines genuinely diverge, and the
	// customs one comes first.
	if p.InstallationCertificateMonths != 36 {
		t.Fatalf("Regional Authority installation deadline = %d months, want 36", p.InstallationCertificateMonths)
	}
	if p.InstallationCertificateCustomsMonths != 6 {
		t.Fatalf("customs installation deadline = %d months, want 6", p.InstallationCertificateCustomsMonths)
	}
	if p.InstallationCertificateCustomsMonths >= p.InstallationCertificateMonths {
		t.Fatalf("the customs deadline must be the earlier of the two")
	}
	if p.BlockDefaultPayMonths != 3 {
		t.Fatalf("time to pay after a block closes short = %d months, want 3", p.BlockDefaultPayMonths)
	}
	if p.AAEODCFilingMonths != 6 {
		t.Fatalf("Advance Authorisation redemption deadline = %d months, want 6", p.AAEODCFilingMonths)
	}
	if got := p.EODCForm(SchemeEPCG); got != "ANF 5B" {
		t.Fatalf("EPCG redemption form = %q, want ANF 5B", got)
	}
	if got := p.EODCForm(SchemeAdvanceAuthorisation); got != "ANF 4F" {
		t.Fatalf("Advance Authorisation redemption form = %q, want ANF 4F", got)
	}
	if len(p.Blocks) != 2 {
		t.Fatalf("want two blocks, got %d", len(p.Blocks))
	}
	if p.Blocks[0].EndsAfterMonths != 48 || p.Blocks[0].CumulativePercent != 50 {
		t.Fatalf("block 1 = %+v, want month 48 at 50%%", p.Blocks[0])
	}
	if p.Blocks[1].EndsAfterMonths != 72 || p.Blocks[1].CumulativePercent != 100 {
		t.Fatalf("block 2 = %+v, want month 72 at 100%%", p.Blocks[1])
	}
	if p.Edition == "" {
		t.Fatalf("the edition must be stated so no figure can be read without it")
	}
}

func TestPolicyNormaliseFillsBlanksAndRefusesNonsense(t *testing.T) {
	d := AsPublished()

	// An empty policy behaves exactly like the published figures.
	empty := Policy{}.Normalise()
	if empty.EPCGMultipleHundredths != d.EPCGMultipleHundredths ||
		empty.EPCGPeriodMonths != d.EPCGPeriodMonths ||
		empty.AAPeriodMonths != d.AAPeriodMonths ||
		empty.InterestRatePercent != d.InterestRatePercent ||
		empty.AAMinValueAdditionPercent != d.AAMinValueAdditionPercent ||
		empty.InstallationCertificateMonths != d.InstallationCertificateMonths ||
		empty.InterestDayBasis != d.InterestDayBasis {
		t.Fatalf("an empty policy should read as published, got %+v", empty)
	}
	if empty.Edited {
		t.Fatalf("an empty policy is not an edited one")
	}
	if empty.Edition == "" {
		t.Fatalf("Normalise must supply the edition")
	}

	// A zero interest rate is "not entered", never a deliberate nil: an
	// exposure with no interest on it is a liability reported too low.
	if got := (Policy{InterestRatePercent: 0}).Normalise().InterestRatePercent; got != 15 {
		t.Fatalf("a zero interest rate must fall back to the published figure, got %d", got)
	}
	if got := (Policy{AAMinValueAdditionPercent: 0}).Normalise().AAMinValueAdditionPercent; got != 15 {
		t.Fatalf("a zero minimum value addition must fall back, got %d", got)
	}

	// Negatives and impossibilities come back into range.
	for _, c := range []struct {
		name string
		in   Policy
		want func(Policy) bool
	}{
		{"negative multiple", Policy{EPCGMultipleHundredths: -5},
			func(p Policy) bool { return p.EPCGMultipleHundredths == 600 }},
		{"negative period", Policy{EPCGPeriodMonths: -1},
			func(p Policy) bool { return p.EPCGPeriodMonths == 72 }},
		{"zero day basis", Policy{InterestDayBasis: 0},
			func(p Policy) bool { return p.InterestDayBasis == 365 }},
		{"condonation over 100", Policy{CondonationPercent: 150},
			func(p Policy) bool { return p.CondonationPercent == 0 }},
		{"negative condonation", Policy{CondonationPercent: -1},
			func(p Policy) bool { return p.CondonationPercent == 0 }},
		{"an unknown average basis", Policy{AverageBasis: "whenever"},
			func(p Policy) bool { return p.AverageBasis == AverageByLicensingYear }},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Normalise(); !c.want(got) {
				t.Fatalf("Normalise(%+v) gave %+v", c.in, got)
			}
		})
	}

	// A real edit is reported as one.
	edited := Policy{EPCGMultipleHundredths: 450}.Normalise()
	if !edited.Edited {
		t.Fatalf("changing the multiple to 4.5 times is an edit")
	}
	if got, _ := edited.MultipleFor(CategoryOrdinary); got != Times(450) {
		t.Fatalf("edited multiple = %s", got.Multiple())
	}
}

func TestPolicyBlockNormalisation(t *testing.T) {
	cases := []struct {
		name   string
		in     Policy
		months []int
		pcts   []int
	}{
		{"published", Policy{}, []int{48, 72}, []int{50, 100}},
		{
			"out of order, with the last block forced to close the period",
			Policy{Blocks: []BlockRule{{EndsAfterMonths: 72, CumulativePercent: 100}, {EndsAfterMonths: 36, CumulativePercent: 25}}},
			[]int{36, 72}, []int{25, 100},
		},
		{
			"a block past the end of the period is dropped",
			Policy{EPCGPeriodMonths: 48, Blocks: []BlockRule{{EndsAfterMonths: 24, CumulativePercent: 40}, {EndsAfterMonths: 96, CumulativePercent: 100}}},
			[]int{24, 48}, []int{40, 100},
		},
		{
			"a decreasing cumulative percentage is pulled up, never down",
			Policy{Blocks: []BlockRule{{EndsAfterMonths: 24, CumulativePercent: 60}, {EndsAfterMonths: 48, CumulativePercent: 30}, {EndsAfterMonths: 72, CumulativePercent: 100}}},
			[]int{24, 48, 72}, []int{60, 60, 100},
		},
		{
			"duplicate end months collapse to the higher requirement",
			Policy{Blocks: []BlockRule{{EndsAfterMonths: 48, CumulativePercent: 40}, {EndsAfterMonths: 48, CumulativePercent: 55}, {EndsAfterMonths: 72, CumulativePercent: 100}}},
			[]int{48, 72}, []int{55, 100},
		},
		{
			"nonsense falls back to the published structure",
			Policy{Blocks: []BlockRule{{EndsAfterMonths: 0, CumulativePercent: 0}, {EndsAfterMonths: -4, CumulativePercent: 900}}},
			[]int{48, 72}, []int{50, 100},
		},
		{
			"one block only, closing the whole period",
			Policy{Blocks: []BlockRule{{EndsAfterMonths: 72, CumulativePercent: 100}}},
			[]int{72}, []int{100},
		},
		{
			"a shortened period with the published fallback keeps only the blocks inside it",
			Policy{EPCGPeriodMonths: 48},
			[]int{48}, []int{100},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalise().Blocks
			if len(got) != len(c.months) {
				t.Fatalf("got %d blocks %+v, want %d", len(got), got, len(c.months))
			}
			for i := range got {
				if got[i].No != i+1 {
					t.Fatalf("block %d is numbered %d", i, got[i].No)
				}
				if got[i].EndsAfterMonths != c.months[i] || got[i].CumulativePercent != c.pcts[i] {
					t.Fatalf("block %d = %+v, want month %d at %d%%", i+1, got[i], c.months[i], c.pcts[i])
				}
			}
			// Whatever the input, the last block must close the period on the
			// whole obligation: anything less would report a licence as closed
			// while duty was still payable on it.
			last := got[len(got)-1]
			if last.CumulativePercent != 100 {
				t.Fatalf("the last block requires %d%%, must require 100%%", last.CumulativePercent)
			}
			if last.EndsAfterMonths != c.in.Normalise().EPCGPeriodMonths {
				t.Fatalf("the last block ends at month %d, period is %d months",
					last.EndsAfterMonths, c.in.Normalise().EPCGPeriodMonths)
			}
		})
	}
}

func TestMultipleFor(t *testing.T) {
	p := AsPublished()
	cases := []struct {
		cat     Category
		want    Bips
		applies bool
	}{
		{CategoryOrdinary, Times(600), true},
		{CategoryGreenTechnology, Times(450), true},
		{CategorySpecifiedArea, Times(150), true},
		{CategoryIndigenousSourcing, Times(450), true},
		{CategoryAsEndorsed, 0, false},
		{"", Times(600), true}, // anything unrecognised is the ordinary multiple
	}
	for _, c := range cases {
		t.Run(string(c.cat), func(t *testing.T) {
			got, applies := p.MultipleFor(c.cat)
			if applies != c.applies {
				t.Fatalf("applies = %v, want %v", applies, c.applies)
			}
			if got != c.want {
				t.Fatalf("multiple = %s, want %s", got.Multiple(), c.want.Multiple())
			}
		})
	}
}

func TestPeriodMonthsByScheme(t *testing.T) {
	p := AsPublished()
	if got := p.PeriodMonths(SchemeEPCG); got != 72 {
		t.Fatalf("EPCG = %d months", got)
	}
	if got := p.PeriodMonths(SchemeAdvanceAuthorisation); got != 18 {
		t.Fatalf("Advance Authorisation = %d months", got)
	}
	if got := p.PeriodMonths("something else"); got != 72 {
		t.Fatalf("an unknown scheme falls back to the EPCG period, got %d", got)
	}
}

func TestPolicyConventionPointers(t *testing.T) {
	// false is a deliberate choice and must survive Normalise; absent is not
	// the same thing and must fall back.
	no := false
	p := Policy{EndsDayBeforeAnniversary: &no, PartYearProRata: &no}.Normalise()
	if p.EndsDayBefore() {
		t.Fatalf("an explicit false must be kept")
	}
	if p.ProRataPartYears() {
		t.Fatalf("an explicit false must be kept")
	}
	if !p.Edited {
		t.Fatalf("turning a convention off is an edit")
	}

	absent := Policy{}.Normalise()
	if !absent.EndsDayBefore() || !absent.ProRataPartYears() {
		t.Fatalf("an absent convention must fall back to the published default")
	}
	if absent.Edited {
		t.Fatalf("an absent convention is not an edit")
	}
}

func TestCitationsCoverEveryFigureAndStateConfidence(t *testing.T) {
	cites := AsPublished().Citations()
	if len(cites) < 15 {
		t.Fatalf("only %d citations; every figure the tool applies must appear", len(cites))
	}
	seen := map[string]bool{}
	for _, c := range cites {
		if seen[c.Key] {
			t.Fatalf("duplicate citation key %q", c.Key)
		}
		seen[c.Key] = true
		if c.Label == "" || c.Figure == "" {
			t.Fatalf("citation %q is missing its label or figure: %+v", c.Key, c)
		}
		if c.Source == "" {
			t.Fatalf("citation %q has no source", c.Key)
		}
		if c.Confidence == "" {
			t.Fatalf("citation %q does not say how much weight to put on it", c.Key)
		}
	}
	for _, key := range []string{
		"edition", "epcgMultiple", "epcgPeriod", "epcgBlocks", "epcgGreen", "epcgArea",
		"epcgIndigenous", "installationRA", "installationCustoms", "aaPeriod",
		"aaValueAddition", "aaEodc", "aaValueShortfall", "interest", "interestSimple",
		"interestBasis", "interestOnWhat", "condonation", "average", "averageOffset",
		"licensingYear", "averageBasis", "periodEnd", "partYearProRata",
		"earlyFulfilment", "mutuallyExclusive", "blockDefaultPay", "epcgEodcForm",
		"legacy",
	} {
		if !seen[key] {
			t.Fatalf("no citation for %q", key)
		}
	}

	// An edited figure must be reflected in the citation, so that a report can
	// never show a published figure while applying a different one.
	edited := Policy{InterestRatePercent: 18}.Normalise()
	found := false
	for _, c := range edited.Citations() {
		if c.Key == "interest" {
			found = true
			if c.Figure != "18% per annum, from the date the goods were cleared" {
				t.Fatalf("edited interest citation reads %q", c.Figure)
			}
		}
	}
	if !found {
		t.Fatalf("no interest citation after editing")
	}
}

func TestPolicyJSONRoundTrip(t *testing.T) {
	in := AsPublished()
	in.InterestRatePercent = 18
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Policy
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out = out.Normalise()
	if out.InterestRatePercent != 18 {
		t.Fatalf("rate did not survive the round trip: %d", out.InterestRatePercent)
	}
	if !out.Edited {
		t.Fatalf("the edited flag did not survive")
	}
	if len(out.Blocks) != 2 {
		t.Fatalf("blocks did not survive: %+v", out.Blocks)
	}
}

func TestItoa(t *testing.T) {
	for _, c := range []struct {
		in   int
		want string
	}{{0, "0"}, {7, "7"}, {48, "48"}, {100, "100"}, {-5, "-5"}, {1234567, "1234567"}} {
		if got := itoa(c.in); got != c.want {
			t.Fatalf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
