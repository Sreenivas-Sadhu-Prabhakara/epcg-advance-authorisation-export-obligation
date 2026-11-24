package eo

// ObligationView is a value obligation and how it stands: what is owed, where
// that figure came from, what has been counted toward it, and what is short.
type ObligationView struct {
	// Required is the obligation in value. Source says whether it was taken
	// from the authorisation or derived from the policy figures.
	Required Money      `json:"required"`
	Source   DateSource `json:"source"`

	// Multiple and DutySaved are the EPCG derivation. MultipleApplies is false
	// for an Advance Authorisation, whose obligation does not come from a
	// multiple of anything, and for an EPCG licence whose endorsed obligation
	// is being used.
	Multiple        Bips  `json:"multiple"`
	MultipleApplies bool  `json:"multipleApplies"`
	DutySaved       Money `json:"dutySaved"`

	// Derived is what the policy figures would make the obligation, kept
	// beside the endorsed figure so the two can be compared. Disagreement says
	// so in words when they differ.
	Derived      Money  `json:"derived"`
	Disagreement string `json:"disagreement"`

	Counted      Money `json:"counted"`
	FulfilledPct Bips  `json:"fulfilledPct"`
	Shortfall    Money `json:"shortfall"`

	// Condoned is the part of the shortfall the condonation figure would
	// forgive, and NetShortfall what is left after it. The default condonation
	// is zero, so by default these are zero and the whole shortfall.
	Condoned     Money `json:"condoned"`
	NetShortfall Money `json:"netShortfall"`

	Status Status `json:"status"`
}

// SpecificObligation works out the value obligation on a licence.
//
// The order of precedence is the point of this function. The obligation an
// exporter owes is the one endorsed on the authorisation. A multiple in a
// policy document is how that endorsement was arrived at, not a substitute for
// it: the Policy is amended, concessions apply that this tool does not know
// about, and a Regional Authority's endorsement governs. So an endorsed figure
// is used whenever there is one, the derived figure is computed anyway, and the
// two are compared out loud.
func SpecificObligation(l Licence, p Policy) ObligationView {
	p = p.Normalise()
	v := ObligationView{DutySaved: l.DutySavedTotal()}

	if l.Scheme == SchemeAdvanceAuthorisation {
		v.Required, v.Source = l.EOValueEndorsed, SourceEndorsed
		return v
	}

	if m, applies := p.MultipleFor(l.Category); applies {
		v.Multiple, v.MultipleApplies = m, true
		v.Derived = m.Of(v.DutySaved)
	}

	switch {
	case l.SpecificEOEndorsed > 0:
		v.Required, v.Source = l.SpecificEOEndorsed, SourceEndorsed
		if v.MultipleApplies && v.Derived > 0 && v.Derived != v.Required {
			v.Disagreement = "The obligation endorsed on the authorisation is " + v.Required.Indian() +
				", and " + v.Multiple.Multiple() + " the duty saved of " + v.DutySaved.Indian() +
				" would be " + v.Derived.Indian() +
				". The endorsed figure is the one applied. A difference usually means the duty saved entered here is not the figure the obligation was fixed on, or that a concession applies."
		}
	case v.MultipleApplies:
		v.Required, v.Source = v.Derived, SourceDerived
	}
	return v
}

// fill completes an obligation view once the counted figure is known.
func (v ObligationView) fill(counted Money, condonation int, deadlinePassed bool) ObligationView {
	v.Counted = counted
	v.FulfilledPct = Proportion(counted, v.Required)
	if v.Required > counted {
		v.Shortfall = v.Required - counted
	}
	if condonation > 0 && v.Required > 0 {
		allowance := v.Required.MulDiv(int64(condonation), 100)
		v.Condoned = v.Shortfall
		if v.Condoned > allowance {
			v.Condoned = allowance
		}
	}
	v.NetShortfall = v.Shortfall - v.Condoned
	switch {
	case v.Required <= 0:
		v.Status = StatusUnknown
	case v.NetShortfall == 0:
		v.Status = StatusMet
	case deadlinePassed:
		v.Status = StatusOverdue
	default:
		v.Status = StatusBehind // refined by the pace projection in Assess
	}
	return v
}

// QuantityView is the quantity side of an Advance Authorisation's obligation.
// An Advance Authorisation is discharged by exporting the resultant product in
// BOTH the value and the quantity the authorisation states, and the binding one
// is whichever is further from being met. A tool that tracked only value would
// report a licence as closeable while the quantity was short.
type QuantityView struct {
	Applies      bool     `json:"applies"`
	Required     Quantity `json:"required"`
	Unit         string   `json:"unit"`
	Counted      Quantity `json:"counted"`
	FulfilledPct Bips     `json:"fulfilledPct"`
	Shortfall    Quantity `json:"shortfall"`
	Status       Status   `json:"status"`
}

// QuantityObligation works out the quantity side of an obligation. The clock is
// passed so that a quantity part way through a period is judged on the same
// footing as a value part way through one: against the pace that would reach
// it, not against the whole requirement.
func QuantityObligation(l Licence, counted Quantity, c Clock) QuantityView {
	deadlinePassed := c.Expired
	v := QuantityView{
		Required: l.EOQuantity,
		Unit:     l.EOQuantityUnit,
		Counted:  counted,
		Applies:  l.Scheme == SchemeAdvanceAuthorisation && l.EOQuantity > 0,
	}
	if !v.Applies {
		v.Status = StatusUnknown
		return v
	}
	v.FulfilledPct = ProportionOfQuantity(counted, v.Required)
	if v.Required > counted {
		v.Shortfall = v.Required - counted
	}
	switch {
	case v.Shortfall == 0:
		v.Status = StatusMet
	case deadlinePassed:
		v.Status = StatusOverdue
	case paceReachesQuantity(counted, v.Required, c):
		v.Status = StatusOnTrack
	default:
		v.Status = StatusBehind
	}
	return v
}

// paceReachesQuantity is paceReaches for a quantity rather than an amount.
func paceReachesQuantity(counted, required Quantity, c Clock) bool {
	if counted >= required {
		return true
	}
	if c.Expired || c.DaysElapsed <= 0 || c.DaysLeft <= 0 {
		return false
	}
	projected := Money(counted).MulDiv(int64(c.DaysLeft), int64(c.DaysElapsed))
	return Quantity(projected) >= required-counted
}

// ValueAdditionView is the minimum value addition an Advance Authorisation must
// achieve, on the Policy's own formula.
//
// VA = ((A − B) ÷ B) × 100
//
// A is the FOB value of the export REALISED. B is the CIF value of the inputs
// covered by the authorisation, plus any other imported materials used in the
// resultant product on which a benefit has been taken. Leaving those other
// materials out of B overstates the value addition, which is why this tool has
// a field for them and puts them in the denominator.
//
// Two figures are reported. The operative one is computed on realisation,
// because that is what the formula says. The other is computed on the shipping
// bills, because most of an exporter's shipments are unrealised while they are
// still deciding what to do, and a projection is more use than a blank.
type ValueAdditionView struct {
	Applies      bool  `json:"applies"`
	MinRequired  Bips  `json:"minRequired"`
	InputsCIF    Money `json:"inputsCif"`
	OtherInputs  Money `json:"otherInputs"`
	DenominatorB Money `json:"denominatorB"`

	RealisedFOB Money `json:"realisedFob"`
	ShippedFOB  Money `json:"shippedFob"`

	OnRealised Bips `json:"onRealised"`
	OnShipped  Bips `json:"onShipped"`

	// StillNeeded is how much more realisation would reach the minimum. It
	// exists because the displayed percentage is rounded and the comparison is
	// not: a value addition of 14.9999% prints as 15.00% and is still short, so
	// the figure a reader can act on has to be a rupee amount, not a
	// percentage.
	StillNeeded Money `json:"stillNeeded"`

	Met    bool   `json:"met"`
	Status Status `json:"status"`
	Note   string `json:"note"`
}

// ValueAddition computes the value addition achieved.
//
// measurable says whether the exports are complete enough for the figure to
// mean anything: either the obligation has been discharged or the period has
// ended. Until then value addition is NECESSARILY short — the inputs are all
// imported at the start and the exports trickle out over a year and a half — so
// reporting it as a failure half way through would be alarming and wrong. It is
// reported as a projection instead, with the figure shown.
func ValueAddition(l Licence, p Policy, realised, shipped Money, measurable bool) ValueAdditionView {
	p = p.Normalise()
	v := ValueAdditionView{
		MinRequired: p.MinValueAddition(),
		InputsCIF:   l.CIFTotal(),
		OtherInputs: l.OtherInputsCIF,
		RealisedFOB: realised,
		ShippedFOB:  shipped,
		Applies:     l.Scheme == SchemeAdvanceAuthorisation,
	}
	v.DenominatorB = v.InputsCIF + v.OtherInputs

	if !v.Applies {
		v.Status = StatusUnknown
		return v
	}
	if v.DenominatorB <= 0 {
		v.Status = StatusUnknown
		v.Note = "Value addition cannot be computed until the CIF value of the imported inputs is entered — it is the denominator of the formula."
		return v
	}

	v.OnRealised = Proportion(v.RealisedFOB-v.DenominatorB, v.DenominatorB)
	v.OnShipped = Proportion(v.ShippedFOB-v.DenominatorB, v.DenominatorB)
	// Compared EXACTLY, not on the rounded percentage above: 14.9999% rounds to
	// 15.00% for display and is still below a 15% minimum, and reporting that
	// requirement as met would understate a liability.
	v.Met = MeetsRate(v.RealisedFOB-v.DenominatorB, v.DenominatorB, v.MinRequired)
	if !v.Met {
		needed := v.DenominatorB + v.MinRequired.Of(v.DenominatorB)
		if needed > v.RealisedFOB {
			v.StillNeeded = needed - v.RealisedFOB
		} else {
			v.StillNeeded = 1 // a paisa: short, but less than the rounding shows
		}
	}

	switch {
	case v.Met:
		v.Status = StatusMet
	case !measurable:
		v.Status = StatusUnknown
		v.Note = "Value addition is measured on the completed exports, and these are not complete. On what has been realised so far it stands at " +
			v.OnRealised.Pct() + ", and on the shipping bills at " + v.OnShipped.Pct() +
			", against a minimum of " + v.MinRequired.Pct() + ". Both figures rise as the exports are made and realised."
	case v.RealisedFOB == 0:
		v.Status = StatusUnknown
		v.Note = "No realisation has been recorded, so value addition on the Policy's formula cannot be computed. On the shipping bills alone it would be " +
			v.OnShipped.Pct() + " against a minimum of " + v.MinRequired.Pct() + "."
	default:
		v.Status = StatusBehind
		v.Note = "Value addition on realisation is " + v.OnRealised.Pct() + " against a minimum of " +
			v.MinRequired.Pct() + " — a further " + v.StillNeeded.Indian() +
			" of realisation would reach it. (The percentage shown is rounded; the comparison is not.)"
	}
	return v
}

// BlockAssessment is one block-wise milestone measured against the exports
// counted up to the end of it.
type BlockAssessment struct {
	BlockWindow

	// Clock measures the CUMULATIVE window — from the start of the obligation
	// period to the end of this block — and not the block's own window. The
	// requirement is cumulative, so the time available to meet it is too: the
	// second block of an EPCG authorisation does not get two years to do the
	// whole obligation in, it gets six.
	Clock   Clock   `json:"clock"`
	Urgency Urgency `json:"urgency"`

	// Required is the cumulative part of the specific obligation that must be
	// discharged by the end of this block.
	Required Money `json:"required"`

	// Counted is what has been counted toward the specific obligation up to
	// the end of this block, or up to today where the block has not closed —
	// recounted over its own window rather than taken as a share of the
	// overall figure, because the average netting has to be done year by year
	// inside the block.
	Counted      Money `json:"counted"`
	FulfilledPct Bips  `json:"fulfilledPct"`
	Shortfall    Money `json:"shortfall"`

	// NeededPerMonth is what remains divided by the whole months left in the
	// block: the run rate that would close it. Zero once the block is met or
	// closed.
	NeededPerMonth Money `json:"neededPerMonth"`
	MonthsLeft     int   `json:"monthsLeft"`

	Status Status `json:"status"`

	// Closed says the block's end date has passed.
	Closed bool `json:"closed"`
	// Current says today falls inside this block.
	Current bool `json:"current"`
}

// AssessBlocks measures every block-wise milestone on a licence.
//
// Each block is counted over the window from the start of the obligation period
// to the end of that block, so the figure is cumulative — which is the question
// an exporter is actually asking: not "how much did I do in years five and six"
// but "by this date, have I done enough".
func AssessBlocks(l Licence, p Policy, plan PeriodPlan, specific Money, asOf Date) []BlockAssessment {
	p = p.Normalise()
	if len(plan.Blocks) == 0 || plan.Start.Zero() {
		return nil
	}

	out := make([]BlockAssessment, 0, len(plan.Blocks))
	for _, b := range plan.Blocks {
		ba := BlockAssessment{BlockWindow: b}
		if !b.Window.Valid() {
			ba.Status = StatusUnknown
			out = append(out, ba)
			continue
		}

		cumulative := Window{From: plan.Start, To: b.Window.To}
		ba.Clock = ClockFor(cumulative, asOf)
		ba.Urgency = UrgencyOf(ba.Clock)
		ba.Closed = !asOf.Zero() && asOf.After(b.Window.To)
		ba.Current = b.Window.Contains(asOf)
		ba.Required = specific.MulDiv(int64(b.CumulativePercent), 100)

		// Count from the start of the period to the end of the block, or to
		// today where the block is still open: an exporter needs to know where
		// they stand now, not where they will stand if nothing more ships.
		countTo := b.Window.To
		if !asOf.Zero() && asOf.Before(countTo) {
			countTo = asOf
		}
		ba.Counted = Count(l, p, Window{From: plan.Start, To: countTo}, plan.PeriodWindow()).CountedToSpecific
		ba.FulfilledPct = Proportion(ba.Counted, ba.Required)
		if ba.Required > ba.Counted {
			ba.Shortfall = ba.Required - ba.Counted
		}

		if !asOf.Zero() && asOf.OnOrBefore(b.Window.To) {
			ba.MonthsLeft = MonthsBetween(asOf, b.Window.To)
			if ba.Shortfall > 0 {
				months := ba.MonthsLeft
				if months < 1 {
					months = 1 // less than a month left: it is all due now
				}
				ba.NeededPerMonth = ba.Shortfall.MulDiv(1, int64(months))
			}
		}

		switch {
		case ba.Required <= 0:
			ba.Status = StatusUnknown
		case ba.Shortfall == 0:
			ba.Status = StatusMet
		case ba.Closed:
			ba.Status = StatusOverdue
		case paceReaches(ba.Counted, ba.Required, ba.Clock):
			ba.Status = StatusOnTrack
		default:
			ba.Status = StatusBehind
		}
		out = append(out, ba)
	}
	return out
}

// paceReaches projects the rate achieved so far over the days remaining and
// reports whether it would reach the requirement.
//
// This is a PROJECTION and nothing in the Policy turns on it. It exists because
// "40% done with two years to go" is not an answer on its own: 40% in the first
// four years of a six-year obligation is behind, and 40% in the first year is
// ahead. A projection says which.
func paceReaches(counted, required Money, c Clock) bool {
	if counted >= required {
		return true
	}
	if c.Expired || c.DaysElapsed <= 0 || c.DaysLeft <= 0 {
		return false
	}
	// counted / elapsed × remaining ≥ required − counted, rearranged to avoid
	// a division: counted × remaining ≥ (required − counted) × elapsed.
	projected := counted.MulDiv(int64(c.DaysLeft), int64(c.DaysElapsed))
	return projected >= required-counted
}
