package eo

// EarlyFulfilmentView is the early-fulfilment condonation: the most valuable
// provision in the EPCG chapter and the one small exporters most often do not
// know exists.
//
// FTP 2023 paragraph 5.09 condones the BALANCE of the export obligation, and
// redeems the authorisation, where the holder has achieved 75% or more of the
// specific obligation AND 100% of the average obligation in HALF OR LESS than
// half the original obligation period. Mirrored in Notification 26/2023-Customs
// condition (7).
//
// Two things make it worth computing rather than mentioning. The first is that
// the deadline is three years into a six-year period, so by the time anybody
// thinks about redemption it has usually gone. The second is that the figure it
// turns on — 75% of the specific obligation, measured over and above the
// average — is not a number an exporter has anywhere: it has to be computed the
// same way the obligation itself is.
//
// It is reported as ELIGIBILITY, never as applied. The balance is condoned by a
// Regional Authority on an application, not by arithmetic, and FTP 5.04(l)
// makes it unavailable to a licence that has taken the indigenous-sourcing,
// green-technology or specified-area reduction.
type EarlyFulfilmentView struct {
	Applies bool `json:"applies"`

	// Excluded records that the concession is barred because another one has
	// been taken. FTP 2023 paragraph 5.04(l): only one of the benefits in paras
	// 5.04(d), 5.09, 5.10 and 5.11 is admissible.
	Excluded       bool   `json:"excluded"`
	ExcludedReason string `json:"excludedReason"`

	// Window is the first half of the ORIGINAL obligation period — the period
	// as first fixed, not as extended.
	Window     Window `json:"window"`
	Clock      Clock  `json:"clock"`
	HalfOfDays int    `json:"halfOfDays"`

	ThresholdPercent int   `json:"thresholdPercent"`
	Required         Money `json:"required"`
	Counted          Money `json:"counted"`
	Shortfall        Money `json:"shortfall"`
	FulfilledPct     Bips  `json:"fulfilledPct"`

	// AverageMaintained is whether the average obligation was maintained in
	// every measurement year inside the half period. The whole of it is
	// required, not a proportion.
	AverageMaintained bool  `json:"averageMaintained"`
	AverageShortfall  Money `json:"averageShortfall"`

	// Decided says the half period has passed, so eligibility is settled one
	// way or the other. Until then Eligible means "on these figures, so far".
	Decided  bool `json:"decided"`
	Eligible bool `json:"eligible"`

	// Condonable is what would be condoned if the concession were granted: the
	// whole balance of the obligation.
	Condonable Money `json:"condonable"`

	Status Status `json:"status"`
	Note   string `json:"note"`
}

// AssessEarlyFulfilment works out whether the early-fulfilment condonation is
// in reach, has been earned, or has been missed.
func AssessEarlyFulfilment(l Licence, p Policy, plan PeriodPlan, specific Money, asOf Date) EarlyFulfilmentView {
	p = p.Normalise()
	v := EarlyFulfilmentView{ThresholdPercent: p.EarlyFulfilmentPercent}

	if l.Scheme != SchemeEPCG {
		v.Note = "Early fulfilment condonation is a feature of the EPCG scheme. It does not arise on an Advance Authorisation."
		v.Status = StatusUnknown
		return v
	}

	switch l.Category {
	case CategoryIndigenousSourcing:
		v.Excluded = true
		v.ExcludedReason = "this authorisation takes the reduced obligation for capital goods sourced indigenously (FTP 5.04(d))"
	case CategoryGreenTechnology:
		v.Excluded = true
		v.ExcludedReason = "this authorisation takes the reduced obligation for green technology products (FTP 5.10)"
	case CategorySpecifiedArea:
		v.Excluded = true
		v.ExcludedReason = "this authorisation takes the reduced obligation for a unit in a specified area (FTP 5.11)"
	}
	if v.Excluded {
		v.Note = "Only one of the concessions in FTP 5.04(d), 5.09, 5.10 and 5.11 is admissible (FTP 5.04(l)), and " +
			v.ExcludedReason + ". Early fulfilment condonation is therefore not available on it."
		v.Status = StatusUnknown
		return v
	}

	// The half is measured on the ORIGINAL period: the one endorsed on the
	// authorisation, or derived from the policy figures, before any extension.
	original := plan.DerivedEnds
	if !plan.EndorsedEnds.Zero() {
		original = plan.EndorsedEnds
	}
	full := Window{From: plan.Start, To: original}
	if !full.Valid() || specific <= 0 {
		v.Note = "The date of issue and the export obligation are both needed before early fulfilment can be worked out."
		v.Status = StatusUnknown
		return v
	}

	v.Applies = true
	v.HalfOfDays = full.Days() / EPCGEarlyFulfilmentPeriodHalves
	v.Window = Window{From: full.From, To: full.From.AddDays(v.HalfOfDays - 1)}
	v.Clock = ClockFor(v.Window, asOf)
	v.Decided = v.Clock.Expired

	v.Required = specific.MulDiv(int64(p.EarlyFulfilmentPercent), 100)

	countTo := v.Window.To
	if !asOf.Zero() && asOf.Before(countTo) {
		countTo = asOf
	}
	tally := Count(l, p, Window{From: full.From, To: countTo}, full)
	v.Counted = tally.CountedToSpecific
	v.FulfilledPct = Proportion(v.Counted, v.Required)
	if v.Required > v.Counted {
		v.Shortfall = v.Required - v.Counted
	}

	v.AverageMaintained = true
	for _, y := range tally.Years {
		if !y.AverageMet {
			v.AverageMaintained = false
			v.AverageShortfall += y.AverageShortfall
		}
	}
	if !tally.AverageApplies {
		v.AverageMaintained = true
	}

	v.Eligible = v.Shortfall == 0 && v.AverageMaintained
	if v.Eligible {
		v.Condonable = specific - v.Counted
		if v.Condonable < 0 {
			v.Condonable = 0
		}
	}

	switch {
	case v.Eligible && v.Decided:
		v.Status = StatusMet
		v.Note = "By " + v.Window.To.Human() + " — half the original obligation period — " +
			v.Counted.Indian() + " had been counted, which is " + itoa(p.EarlyFulfilmentPercent) +
			"% or more of the obligation, and the average was maintained throughout. On these figures FTP 5.09 condones the balance of " +
			v.Condonable.Indian() + " and the authorisation may be redeemed. A Regional Authority grants that on an application; this tool does not."
	case v.Eligible:
		v.Status = StatusMet
		v.Note = "On the figures so far, this authorisation is on course for the FTP 5.09 condonation: " +
			v.Counted.Indian() + " counted of the " + v.Required.Indian() + " needed by " +
			v.Window.To.Human() + ", with the average maintained. Hold that to the half-way date and the balance of the obligation is condoned."
	case v.Decided:
		v.Status = StatusOverdue
		v.Note = "The half-way date, " + v.Window.To.Human() + ", has passed. " +
			v.Counted.Indian() + " was counted against the " + v.Required.Indian() +
			" that FTP 5.09 needed by then"
		if !v.AverageMaintained {
			v.Note += ", and the average was not maintained in every year of it"
		}
		v.Note += ". The condonation is no longer available; the obligation runs to the end of the period."
	default:
		v.Status = StatusBehind
		v.Note = "FTP 5.09 would condone the balance of the obligation if " + v.Required.Indian() +
			" were counted by " + v.Window.To.Human() + " — " + plural(v.Clock.DaysLeft, "day", "days") +
			" away — with the average maintained throughout. " + v.Shortfall.Indian() + " to go."
		if !v.AverageMaintained {
			v.Note += " The average has already fallen short in a year inside the half period, which on the face of FTP 5.09 puts the condonation out of reach."
		}
	}
	return v
}

// installationDue counts the import consignments with no installation
// certificate recorded, and how many of them are past each of the two
// deadlines — the customs one and the Regional Authority's.
//
// The two deadlines genuinely diverge: Notification 26/2023-Customs condition
// (8) allows six months from the completion of the import, HBP 2023 paragraph
// 5.04(a) allows the Regional Authority three years. Nothing reconciles them,
// so both are counted and both are reported, and the customs one — which comes
// first and is the one that produces a demand — is reported first.
func installationDue(l Licence, p Policy, asOf Date) (pending, pastCustoms, pastRA int) {
	p = p.Normalise()
	for _, im := range l.Imports {
		if !im.InstalledOn.Zero() {
			continue
		}
		if im.DutySaved <= 0 && im.CIFValue <= 0 {
			continue // an empty row, not a consignment
		}
		pending++
		if im.ClearedOn.Zero() || asOf.Zero() {
			continue
		}
		if asOf.After(im.ClearedOn.AddMonths(p.InstallationCertificateCustomsMonths)) {
			pastCustoms++
		}
		if asOf.After(im.ClearedOn.AddMonths(p.InstallationCertificateMonths)) {
			pastRA++
		}
	}
	return pending, pastCustoms, pastRA
}
