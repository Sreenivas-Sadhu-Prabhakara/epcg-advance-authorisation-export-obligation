package eo

// ExposureLine is the exposure attributable to one import consignment.
//
// The exposure is worked out consignment by consignment, and not on a single
// total, because interest runs from the date each consignment was CLEARED.
// Capital goods imported against one EPCG authorisation often arrive over two
// or three years; interest on the first container has been running a great deal
// longer than interest on the last, and a tool that used one date would
// understate the largest part of the liability.
type ExposureLine struct {
	Index         int    `json:"index"`
	Description   string `json:"description"`
	BillOfEntryNo string `json:"billOfEntryNo"`
	ClearedOn     Date   `json:"clearedOn"`

	DutySaved Money `json:"dutySaved"`

	// Proportionate is the part of the duty saved on this consignment that
	// corresponds to the unfulfilled part of the obligation.
	Proportionate Money `json:"proportionate"`

	Days     int   `json:"days"`
	Interest Money `json:"interest"`
	Total    Money `json:"total"`

	// Problem says why a line could not be computed in full — almost always a
	// missing clearance date, without which there is no period to run interest
	// over.
	Problem string `json:"problem"`
}

// ExposureView is what becomes payable if the obligation is not discharged:
// the duty saved proportionate to the shortfall, plus interest on it.
//
// This is the number the tool exists to put in front of somebody BEFORE the
// deadline rather than after. While the obligation period is still running it
// is hypothetical — it is what would be payable if the licence closed today on
// today's figures — and it says so. Once the period has ended with a shortfall,
// it is not hypothetical any more.
type ExposureView struct {
	Applies      bool `json:"applies"`
	Hypothetical bool `json:"hypothetical"`
	AsOf         Date `json:"asOf"`

	// ShortfallPct is the unfulfilled proportion of the obligation, which is
	// the proportion of the duty saved that becomes payable.
	ShortfallPct Bips  `json:"shortfallPct"`
	Shortfall    Money `json:"shortfall"`
	Required     Money `json:"required"`

	DutySavedTotal Money `json:"dutySavedTotal"`
	DutyPayable    Money `json:"dutyPayable"`
	Interest       Money `json:"interest"`
	Total          Money `json:"total"`

	RateApplied Bips `json:"rateApplied"`
	DayBasis    int  `json:"dayBasis"`

	Lines []ExposureLine `json:"lines"`

	// Understated is set when something missing means the figure is lower than
	// the real exposure — a consignment with no clearance date, or no duty
	// saved entered at all. It is phrased as a warning and not as a note,
	// because a liability reported too low is the dangerous direction.
	Understated string `json:"understated"`

	// Basis is the computation in words, for the report.
	Basis string `json:"basis"`
}

// Exposure computes what becomes payable on an unfulfilled obligation.
//
// The computation:
//
//   - the proportion of the obligation left unfulfilled is worked out;
//   - that proportion of the duty saved on each import consignment becomes
//     payable;
//   - simple interest runs on each such amount, at the rate in the policy
//     figures, in calendar days from the date that consignment was cleared to
//     the date this is computed for, on the day-count basis in the figures.
//
// The rate, the basis and the day count are all conventions or indicative
// figures, named in the policy block and overridable. A department may compute
// interest differently. What this produces is an order of magnitude and a
// reason to act, not a demand.
func Exposure(l Licence, p Policy, required, netShortfall Money, asOf Date, periodExpired bool) ExposureView {
	p = p.Normalise()
	v := ExposureView{
		AsOf:           asOf,
		Required:       required,
		Shortfall:      netShortfall,
		DutySavedTotal: l.DutySavedTotal(),
		RateApplied:    p.InterestRate(),
		DayBasis:       p.InterestDayBasis,
		Hypothetical:   !periodExpired,
	}

	if required <= 0 || netShortfall <= 0 {
		v.Basis = "Nothing is payable on the figures entered: the obligation is shown as discharged in full."
		if required <= 0 {
			v.Basis = "There is no obligation figure to measure a shortfall against, so no exposure can be computed. Enter the export obligation endorsed on the authorisation."
		}
		return v
	}

	v.Applies = true
	v.ShortfallPct = Proportion(netShortfall, required)

	if v.DutySavedTotal <= 0 {
		v.Understated = "No duty saved has been entered against this licence, so the duty that would become payable cannot be computed. The exposure shown is zero and the real exposure is not."
		v.Basis = "The obligation is short by " + netShortfall.Indian() + " of " + required.Indian() +
			" (" + v.ShortfallPct.Pct() + "), but no duty saved has been entered to apply that proportion to."
		return v
	}

	missingDates := 0
	for i, im := range l.Imports {
		if im.DutySaved <= 0 {
			continue
		}
		line := ExposureLine{
			Index:         i,
			Description:   im.Description,
			BillOfEntryNo: im.BillOfEntryNo,
			ClearedOn:     im.ClearedOn,
			DutySaved:     im.DutySaved,
		}
		line.Proportionate = im.DutySaved.MulDiv(int64(v.ShortfallPct), 10000)

		switch {
		case im.ClearedOn.Zero():
			missingDates++
			line.Problem = "no clearance date entered, so no interest is computed on this consignment"
		case asOf.Zero() || !asOf.After(im.ClearedOn):
			line.Problem = "cleared on or after the date this is computed for, so no interest has run yet"
		default:
			line.Days = DaysBetween(im.ClearedOn, asOf)
			line.Interest = line.Proportionate.MulDiv(
				int64(v.RateApplied)*int64(line.Days),
				10000*int64(v.DayBasis),
			)
		}
		line.Total = line.Proportionate + line.Interest

		v.Lines = append(v.Lines, line)
		v.DutyPayable += line.Proportionate
		v.Interest += line.Interest
	}
	v.Total = v.DutyPayable + v.Interest

	if missingDates > 0 {
		v.Understated = plural(missingDates, "import consignment", "import consignments") +
			" has no customs clearance date, so no interest has been computed on it. The real exposure is higher than the figure shown."
		if missingDates > 1 {
			v.Understated = plural(missingDates, "import consignment", "import consignments") +
				" have no customs clearance date, so no interest has been computed on them. The real exposure is higher than the figure shown."
		}
	}

	when := "if the licence were closed today on today's figures"
	if periodExpired {
		when = "the obligation period having ended with the obligation short"
	}
	v.Basis = "The obligation is short by " + netShortfall.Indian() + " of " + required.Indian() +
		" — " + v.ShortfallPct.Pct() + " unfulfilled. That proportion of the duty saved, " +
		v.DutyPayable.Indian() + " of " + v.DutySavedTotal.Indian() +
		", becomes payable " + when + ", with simple interest at " + v.RateApplied.Pct() +
		" a year from the date each consignment was cleared, on a " + itoa(v.DayBasis) + "-day year: " +
		v.Interest.Indian() + ". Total " + v.Total.Indian() + "."

	return v
}

// Daily is roughly what one more day of delay adds to the interest, so that the
// cost of waiting is a number and not a feeling. It is computed off the duty
// payable rather than by differencing two exposures, so it does not drift with
// rounding.
func (v ExposureView) Daily() Money {
	if !v.Applies || v.DayBasis <= 0 {
		return 0
	}
	return v.DutyPayable.MulDiv(int64(v.RateApplied), 10000*int64(v.DayBasis))
}
