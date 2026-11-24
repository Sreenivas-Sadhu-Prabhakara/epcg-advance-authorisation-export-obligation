package eo

// YearTally is what happened in one measurement year, and the single most
// misunderstood piece of arithmetic in the EPCG scheme.
//
// An EPCG specific export obligation is to be discharged OVER AND ABOVE the
// average level of exports of the same and similar products. So an exporter who
// was already shipping ₹2 crore of the same product every year, and who now
// ships ₹2.5 crore, has not discharged ₹2.5 crore of the obligation. They have
// discharged ₹50 lakh of it — and they have only just maintained the average.
// Every year is measured on its own, which is why a year is the unit here and
// not the whole period.
type YearTally struct {
	Label string `json:"label"`

	// Window is the part of the measurement year that falls inside the window
	// being counted. FullWindow is the whole year it came from. They differ at
	// the two ends of an obligation period and at a block boundary.
	Window     Window `json:"window"`
	FullWindow Window `json:"fullWindow"`
	PartYear   bool   `json:"partYear"`

	DaysCounted int `json:"daysCounted"`
	DaysInYear  int `json:"daysInYear"`

	// SameOrSimilar is the FOB value of counted exports of the same or a
	// similar product as the one the average was fixed on. OtherProducts is
	// everything else counted against the licence.
	SameOrSimilar Money `json:"sameOrSimilar"`
	OtherProducts Money `json:"otherProducts"`

	Quantity  Quantity `json:"quantity"`
	Shipments int      `json:"shipments"`

	// AverageDue is the average obligation attributed to this year: the full
	// annual average for a whole year, pro-rated by days for a part year where
	// that convention is in force.
	AverageDue       Money `json:"averageDue"`
	AverageMet       bool  `json:"averageMet"`
	AverageShortfall Money `json:"averageShortfall"`

	// CountedToSpecific is what this year contributes to the specific
	// obligation: the excess of same-or-similar exports over the average, plus
	// the whole of any exports of other products.
	CountedToSpecific Money `json:"countedToSpecific"`
}

// Tally is the result of counting a licence's shipments over a window.
type Tally struct {
	// Window is what was counted: the obligation period up to the day the
	// assessment was made for, which is not the same as the obligation period.
	// Counting the whole period would attribute a full year's average to
	// licensing years that have not happened yet and report an exporter as
	// having already failed them.
	Window Window `json:"window"`
	// PeriodWindow is the whole obligation period, used to tell an export that
	// falls outside it from one that merely has not happened yet.
	PeriodWindow Window      `json:"periodWindow"`
	Years        []YearTally `json:"years"`

	// Totals inside the window.
	SameOrSimilar     Money    `json:"sameOrSimilar"`
	OtherProducts     Money    `json:"otherProducts"`
	TotalFOB          Money    `json:"totalFob"`
	CountedToSpecific Money    `json:"countedToSpecific"`
	Quantity          Quantity `json:"quantity"`
	Shipments         int      `json:"shipments"`

	// AverageDueTotal is the average obligation across every year in the
	// window, and AverageShortfallTotal how much of it was not maintained.
	AverageDueTotal       Money `json:"averageDueTotal"`
	AverageShortfallTotal Money `json:"averageShortfallTotal"`
	AverageApplies        bool  `json:"averageApplies"`

	// What was left out, and why. These are counts and index lists rather than
	// copies of the shipments, so a caller can show the user the actual rows.
	OutsidePeriod []int `json:"outsidePeriod"`
	NotEndorsed   []int `json:"notEndorsed"`
	Excluded      []int `json:"excluded"`
	Undated       []int `json:"undated"`
	// NotYet are exports dated inside the obligation period but after the day
	// the assessment was made for. They are not counted yet and they are not a
	// problem — a forward-dated shipping bill is an entry made early.
	NotYet []int `json:"notYet"`

	// Unrealised is counted FOB value for which no proof of realisation has
	// been recorded. It still counts toward the obligation here — the export
	// has been made — but it cannot go into a redemption file, so it is
	// reported on its own.
	Unrealised      Money `json:"unrealised"`
	UnrealisedCount int   `json:"unrealisedCount"`

	// RealisedTotal is the value actually realised against counted exports. It
	// is the A term of the Advance Authorisation value-addition formula, which
	// is expressed on realisation and not on the shipping bills.
	RealisedTotal Money `json:"realisedTotal"`

	// RealisedShortfall is where a bank realisation certificate came in for
	// less than the shipping bill's FOB value. It is reported rather than
	// netted off, because which figure a Regional Authority counts is its own
	// question and this tool will not decide it silently.
	RealisedShortfall Money `json:"realisedShortfall"`
}

// Count tallies a licence's shipments over a window, applying the average
// obligation year by year.
//
// It is a pure function of the licence, the policy figures and the window, and
// it is called more than once per assessment: once for the whole obligation
// period, and once per block-wise milestone over the window that ends with that
// block. That is what makes a block-wise figure correct rather than a
// proportion of the overall one — the average netting inside a block has to be
// done on the years inside the block.
func Count(l Licence, p Policy, w, period Window) Tally {
	p = p.Normalise()
	if !period.Valid() {
		period = w
	}
	t := Tally{Window: w, PeriodWindow: period}
	if !w.Valid() {
		return t
	}

	averageApplies := l.Scheme == SchemeEPCG && !l.AverageEOExempt && l.AverageEO > 0
	t.AverageApplies = averageApplies

	// Index the shipments that count, and record why the others do not.
	type counted struct {
		index int
		sh    Shipment
	}
	inWindow := make([]counted, 0, len(l.Shipments))
	for i, sh := range l.Shipments {
		switch {
		case sh.Excluded:
			t.Excluded = append(t.Excluded, i)
		case sh.ShippedOn.Zero():
			t.Undated = append(t.Undated, i)
		case !sh.AuthorisationEndorsed:
			t.NotEndorsed = append(t.NotEndorsed, i)
		case !period.Contains(sh.ShippedOn):
			t.OutsidePeriod = append(t.OutsidePeriod, i)
		case !w.Contains(sh.ShippedOn):
			t.NotYet = append(t.NotYet, i)
		default:
			inWindow = append(inWindow, counted{i, sh})
		}
	}

	years := measurementYears(l, p, w)
	t.Years = make([]YearTally, 0, len(years))

	for _, y := range years {
		yt := YearTally{
			Label:       y.label,
			FullWindow:  y.full,
			Window:      y.counted,
			PartYear:    y.counted.Days() < y.full.Days(),
			DaysCounted: y.counted.Days(),
			DaysInYear:  y.full.Days(),
		}

		for _, c := range inWindow {
			if !y.counted.Contains(c.sh.ShippedOn) {
				continue
			}
			yt.Shipments++
			yt.Quantity += c.sh.Quantity
			// Same-or-similar exports are the ones measured against the
			// average. Anything else exported against the licence counts
			// toward the specific obligation in full, because no average was
			// fixed for it to be over and above.
			if c.sh.SameOrSimilar {
				yt.SameOrSimilar += c.sh.FOBValue
			} else {
				yt.OtherProducts += c.sh.FOBValue
			}
		}

		if averageApplies {
			yt.AverageDue = averageDueFor(l.AverageEO, yt, p)
			yt.AverageMet = yt.SameOrSimilar >= yt.AverageDue
			if !yt.AverageMet {
				yt.AverageShortfall = yt.AverageDue - yt.SameOrSimilar
			}
			if excess := yt.SameOrSimilar - yt.AverageDue; excess > 0 {
				yt.CountedToSpecific = excess
			}
			yt.CountedToSpecific += yt.OtherProducts
		} else {
			yt.AverageMet = true
			yt.CountedToSpecific = yt.SameOrSimilar + yt.OtherProducts
		}

		t.Years = append(t.Years, yt)
		t.SameOrSimilar += yt.SameOrSimilar
		t.OtherProducts += yt.OtherProducts
		t.Quantity += yt.Quantity
		t.Shipments += yt.Shipments
		t.CountedToSpecific += yt.CountedToSpecific
		t.AverageDueTotal += yt.AverageDue
		t.AverageShortfallTotal += yt.AverageShortfall
	}

	t.TotalFOB = t.SameOrSimilar + t.OtherProducts

	for _, c := range inWindow {
		t.RealisedTotal += c.sh.RealisedValue
		if c.sh.RealisedOn.Zero() && c.sh.RealisedValue == 0 {
			t.Unrealised += c.sh.FOBValue
			t.UnrealisedCount++
			continue
		}
		if c.sh.RealisedValue > 0 && c.sh.RealisedValue < c.sh.FOBValue {
			t.RealisedShortfall += c.sh.FOBValue - c.sh.RealisedValue
		}
	}

	return t
}

// averageDueFor attributes the annual average obligation to a measurement year.
//
// A whole year carries the whole average. A PART year — the first and last
// licensing years of an obligation period, and the year a block boundary falls
// in — is pro-rated by days, which is a CONVENTION OF THIS TOOL and not a rule
// of the Policy. The alternative, charging a full year's average to a
// three-week sliver of a licensing year, is harsher than any reading of the
// Policy supports; charging nothing at all is more generous than it supports.
// Pro-rating sits between them, is stated wherever it is used, and can be
// turned off.
func averageDueFor(annual Money, yt YearTally, p Policy) Money {
	if annual <= 0 {
		return 0
	}
	if !yt.PartYear || yt.DaysInYear <= 0 || !p.ProRataPartYears() {
		return annual
	}
	return annual.MulDiv(int64(yt.DaysCounted), int64(yt.DaysInYear))
}

// measurementYear is one year to measure the average against, and the part of
// it that falls inside the window being counted.
type measurementYear struct {
	label   string
	full    Window
	counted Window
}

// measurementYears lists the years the average obligation is measured against,
// clipped to the window being counted.
//
// Under the default basis these are LICENSING years — 1 April to 31 March —
// which do not line up with the obligation period: a six-year period beginning
// in May touches seven of them, the first and last being part years. Under the
// alternative basis they are anniversary years of the authorisation, which line
// up with the period exactly.
func measurementYears(l Licence, p Policy, w Window) []measurementYear {
	if !w.Valid() {
		return nil
	}
	out := []measurementYear{}

	if p.AverageBasis == AverageByAuthorisationYear && !l.IssuedOn.Zero() {
		for n := 1; n <= 200; n++ {
			full := Window{
				From: l.IssuedOn.AddYears(n - 1),
				To:   l.IssuedOn.AddYears(n).AddDays(-1),
			}
			if full.From.After(w.To) {
				break
			}
			clipped, ok := full.Overlap(w)
			if !ok {
				continue
			}
			out = append(out, measurementYear{
				label:   "Year " + itoa(n),
				full:    full,
				counted: clipped,
			})
		}
		return out
	}

	for _, fy := range FinancialYearsOverlapping(w) {
		full := fy.Window()
		clipped, ok := full.Overlap(w)
		if !ok {
			continue
		}
		out = append(out, measurementYear{
			label:   fy.String(),
			full:    full,
			counted: clipped,
		})
	}
	return out
}
