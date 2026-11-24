package eo

// DateSource says where a date in an assessment came from, so that a derived
// date is never mistaken for one printed on the authorisation. Every date this
// package reports carries one.
type DateSource string

const (
	// SourceEndorsed — typed in from the authorisation. Authoritative.
	SourceEndorsed DateSource = "endorsed"
	// SourceDerived — computed from the obligation period in the policy
	// figures, using the period-end convention. Indicative.
	SourceDerived DateSource = "derived"
	// SourceExtended — moved by an extension that has actually been granted.
	SourceExtended DateSource = "extended"
)

// Title names the source for the screen.
func (s DateSource) Title() string {
	switch s {
	case SourceEndorsed:
		return "as endorsed on the authorisation"
	case SourceExtended:
		return "as extended"
	case SourceDerived:
		return "derived from the policy figures"
	default:
		return string(s)
	}
}

// Status is where something stands against its deadline.
type Status string

const (
	// StatusMet — the requirement is discharged in full.
	StatusMet Status = "met"
	// StatusOnTrack — not yet due, and the run rate so far would reach it.
	StatusOnTrack Status = "on-track"
	// StatusBehind — not yet due, but the run rate so far would not reach it.
	StatusBehind Status = "behind"
	// StatusOverdue — the deadline has passed and the requirement is short.
	StatusOverdue Status = "overdue"
	// StatusUnknown — there is not enough entered to say.
	StatusUnknown Status = "unknown"
)

// Title names the status for the screen.
func (s Status) Title() string {
	switch s {
	case StatusMet:
		return "Met"
	case StatusOnTrack:
		return "On track"
	case StatusBehind:
		return "Behind"
	case StatusOverdue:
		return "Overdue"
	default:
		return "Not enough entered"
	}
}

// DerivePeriodEnd turns "N months from a date" into a last day, applying the
// period-end convention. It is the only place that convention is applied.
//
// With the convention in force, a six-year period from 15 May 2023 ends on
// 14 May 2029; without it, on 15 May 2029. Month arithmetic clamps, so an
// 18-month period from 31 August 2026 ends on 28 February 2028 either way minus
// the convention's day.
func DerivePeriodEnd(start Date, months int, endsDayBefore bool) Date {
	if start.Zero() || months <= 0 {
		return Date{}
	}
	end := start.AddMonths(months)
	if endsDayBefore {
		end = end.AddDays(-1)
	}
	return end
}

// BlockWindow is one block-wise milestone with its dates settled: the window it
// covers, where its end date came from, and how much of the specific obligation
// must be discharged by the end of it.
type BlockWindow struct {
	No                int        `json:"no"`
	Window            Window     `json:"window"`
	CumulativePercent int        `json:"cumulativePercent"`
	EndsSource        DateSource `json:"endsSource"`

	// DerivedEnds and EndorsedEnds are kept beside the end actually used so
	// that a report can show what the policy figures would have given and what
	// the authorisation says, where they differ.
	DerivedEnds  Date `json:"derivedEnds"`
	EndorsedEnds Date `json:"endorsedEnds"`
	ExtendedTo   Date `json:"extendedTo"`
}

// PeriodPlan is the whole clock for a licence, worked out before any export is
// counted against it: when the obligation period runs from and to, where that
// end date came from, and the blocks inside it.
type PeriodPlan struct {
	Start      Date       `json:"start"`
	Ends       Date       `json:"ends"`
	EndsSource DateSource `json:"endsSource"`

	DerivedEnds  Date `json:"derivedEnds"`
	EndorsedEnds Date `json:"endorsedEnds"`
	ExtendedTo   Date `json:"extendedTo"`

	// PeriodMonths is the period the derived end was computed from.
	PeriodMonths int `json:"periodMonths"`

	Blocks []BlockWindow `json:"blocks"`

	// Disagreement is set when the endorsed end date and the derived one differ
	// by more than the one day the period-end convention accounts for. It is
	// not an error — the endorsed date governs — but it is worth saying,
	// because it usually means the period for this authorisation is not the
	// default one.
	Disagreement string `json:"disagreement"`
}

// Window is the obligation period as a closed interval.
func (pp PeriodPlan) PeriodWindow() Window { return Window{From: pp.Start, To: pp.Ends} }

// PlanPeriod works out the clock for a licence: the obligation period, the
// blocks inside it, and for each date whether it was endorsed, derived or
// extended.
//
// Order of precedence, which is the whole point of this function:
//
//  1. an extension actually granted, if it moves the date later;
//  2. the date endorsed on the authorisation;
//  3. a date derived from the obligation period in the policy figures.
//
// An extension that has been applied for but not granted does not appear here,
// and the tool has no field for one. An exporter planning on an extension that
// is not in hand is exactly the person this tool exists to warn.
func PlanPeriod(l Licence) PeriodPlan {
	l = l.Normalise()
	p := l.Policy.Normalise()

	months := p.PeriodMonths(l.Scheme)
	plan := PeriodPlan{
		Start:        l.IssuedOn,
		PeriodMonths: months,
		DerivedEnds:  DerivePeriodEnd(l.IssuedOn, months, p.EndsDayBefore()),
		EndorsedEnds: l.EOPeriodEndEndorsed,
	}

	plan.Ends, plan.EndsSource = plan.DerivedEnds, SourceDerived
	if !plan.EndorsedEnds.Zero() {
		plan.Ends, plan.EndsSource = plan.EndorsedEnds, SourceEndorsed
		if !plan.DerivedEnds.Zero() {
			if d := DaysBetween(plan.DerivedEnds, plan.EndorsedEnds); d > 1 || d < -1 {
				plan.Disagreement = "The obligation period endorsed on the authorisation ends " +
					plan.EndorsedEnds.Human() + ", and " + plural(months, "month", "months") +
					" from the date of issue would end " + plan.DerivedEnds.Human() +
					". The endorsed date is the one applied. Check whether a different period applies to this authorisation."
			}
		}
	}

	// An overall extension moves the end of the period, but only forward: an
	// extension cannot shorten an obligation period, and a date entered that
	// would do so is ignored rather than silently applied.
	for _, ex := range l.Extensions {
		if ex.Kind != ExtensionOverall {
			continue
		}
		if plan.Ends.Zero() || ex.NewEndsOn.After(plan.Ends) {
			plan.Ends, plan.EndsSource, plan.ExtendedTo = ex.NewEndsOn, SourceExtended, ex.NewEndsOn
		}
	}

	plan.Blocks = planBlocks(l, p, plan)
	return plan
}

// planBlocks settles the dates of the block-wise milestones. Blocks are an
// EPCG feature; an Advance Authorisation has one period and no milestone inside
// it, so this returns none for one.
func planBlocks(l Licence, p Policy, plan PeriodPlan) []BlockWindow {
	if l.Scheme != SchemeEPCG || plan.Start.Zero() {
		return nil
	}

	out := make([]BlockWindow, 0, len(p.Blocks))
	prevEnd := plan.Start.AddDays(-1) // so the first block begins on the start date
	for i, rule := range p.Blocks {
		b := BlockWindow{
			No:                rule.No,
			CumulativePercent: rule.CumulativePercent,
			DerivedEnds:       DerivePeriodEnd(plan.Start, rule.EndsAfterMonths, p.EndsDayBefore()),
		}
		if i < len(l.BlockEndsEndorsed) {
			b.EndorsedEnds = l.BlockEndsEndorsed[i]
		}

		end, source := b.DerivedEnds, SourceDerived
		if !b.EndorsedEnds.Zero() {
			end, source = b.EndorsedEnds, SourceEndorsed
		}

		// The last block closes the obligation period, so whatever moved the
		// period's end — an endorsement or an extension — moves it too.
		last := i == len(p.Blocks)-1
		if last && !plan.Ends.Zero() {
			end, source = plan.Ends, plan.EndsSource
		}

		for _, ex := range l.Extensions {
			if ex.Kind != ExtensionBlock || ex.BlockNo != rule.No {
				continue
			}
			if end.Zero() || ex.NewEndsOn.After(end) {
				end, source, b.ExtendedTo = ex.NewEndsOn, SourceExtended, ex.NewEndsOn
			}
		}

		// A block cannot end before the one before it, and no block but the
		// last may end after the period itself.
		if !prevEnd.Zero() && !end.Zero() && end.OnOrBefore(prevEnd) {
			end = prevEnd.AddDays(1)
		}
		if !last && !plan.Ends.Zero() && !end.Zero() && end.After(plan.Ends) {
			end = plan.Ends
		}

		b.Window = Window{From: prevEnd.AddDays(1), To: end}
		b.EndsSource = source
		out = append(out, b)
		prevEnd = end
	}
	return out
}

// Clock is how a deadline stands on a given day: how long the period is, how
// much of it has gone, and how much is left. Every figure is in whole calendar
// days, both end days inside the period.
type Clock struct {
	AsOf        Date   `json:"asOf"`
	Window      Window `json:"window"`
	DaysTotal   int    `json:"daysTotal"`
	DaysElapsed int    `json:"daysElapsed"`
	DaysLeft    int    `json:"daysLeft"`
	ElapsedPct  Bips   `json:"elapsedPct"`
	Expired     bool   `json:"expired"`
	NotStarted  bool   `json:"notStarted"`
}

// ClockFor measures a window against a day.
//
// DaysLeft counts the deadline day itself as a day you still have, because it
// is: a shipping bill filed on the last day of the period is filed inside the
// period. On the day after, DaysLeft is zero and Expired is true.
func ClockFor(w Window, asOf Date) Clock {
	c := Clock{AsOf: asOf, Window: w}
	if !w.Valid() || asOf.Zero() {
		return c
	}
	c.DaysTotal = w.Days()
	switch {
	case asOf.Before(w.From):
		c.NotStarted = true
		c.DaysLeft = c.DaysTotal + DaysBetween(asOf, w.From)
	case asOf.After(w.To):
		c.Expired = true
		c.DaysElapsed = c.DaysTotal
		c.ElapsedPct = 10000
	default:
		c.DaysElapsed = DaysBetween(w.From, asOf) + 1
		c.DaysLeft = DaysBetween(asOf, w.To) + 1
		c.ElapsedPct = ProportionOfCount(c.DaysElapsed, c.DaysTotal)
	}
	return c
}

// Urgency grades how pressing a deadline is, for ordering a list of licences
// and for colouring a figure on screen.
type Urgency string

const (
	UrgencyNone    Urgency = "none"
	UrgencySoon    Urgency = "soon"
	UrgencyUrgent  Urgency = "urgent"
	UrgencyExpired Urgency = "expired"
)

// UrgencyOf grades a clock against the two warning windows.
func UrgencyOf(c Clock) Urgency {
	switch {
	case !c.Window.Valid():
		return UrgencyNone
	case c.Expired:
		return UrgencyExpired
	case c.DaysLeft <= UrgentWithinDays:
		return UrgencyUrgent
	case c.DaysLeft <= WarnWithinDays:
		return UrgencySoon
	default:
		return UrgencyNone
	}
}
