package eo

import (
	"strings"
)

// Report writes an assessment as plain text: the figures, the working behind
// them, the closure checklist and the assumptions, in an order somebody can put
// in a file or send to their consultant.
//
// Plain text and not a formatted document, deliberately. It prints from any
// machine, it opens in anything, it can be pasted into an email, and nothing
// about it can go stale or fail to render. Every figure in it comes from the
// assessment, so it cannot drift from what the screen says.
func Report(l Licence, a Assessment) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	rule := func() { w(strings.Repeat("-", 72)) }
	kv := func(k, v string) {
		if v == "" {
			return
		}
		w(pad(k, 30) + v)
	}

	w("EXPORT OBLIGATION STATEMENT")
	w(a.Scheme.Title() + " — " + a.Label)
	w("Position as at " + a.AsOf.Human())
	rule()

	kv("Scheme", a.Scheme.Title())
	kv("Authorisation number", l.AuthorisationNo)
	kv("File number", l.FileNo)
	kv("Holder", l.Holder)
	kv("Importer-exporter code", l.IEC)
	kv("Product", l.Product)
	kv("Currency of all figures", l.Currency)
	kv("Figures from", a.Policy.Edition)
	if a.PolicyEdited {
		kv("", "at least one figure has been CHANGED from the published default")
	}
	kv("Date of issue", a.Period.Start.Human())
	if l.Scheme == SchemeAdvanceAuthorisation {
		kv("Norms basis", l.NormsBasis.Title())
		kv("SION reference", l.SIONReference)
	}
	w("")

	w("THE CLOCK")
	rule()
	kv("Obligation period", a.Period.PeriodWindow().String())
	kv("End date taken from", a.Period.EndsSource.Title())
	if !a.Period.DerivedEnds.Zero() && a.Period.EndsSource != SourceDerived {
		kv("Derived end for comparison", a.Period.DerivedEnds.Human()+
			" ("+plural(a.Period.PeriodMonths, "month", "months")+" from issue)")
	}
	if !a.Period.ExtendedTo.Zero() {
		kv("Extended to", a.Period.ExtendedTo.Human())
	}
	switch {
	case a.Clock.Expired:
		kv("Status", "the period ENDED "+plural(-a.Clock.DaysLeft, "day", "days")+" ago")
	case a.Clock.NotStarted:
		kv("Status", "the period has not begun")
	default:
		kv("Status", plural(a.Clock.DaysLeft, "day", "days")+" left of "+
			plural(a.Clock.DaysTotal, "day", "days")+" — "+a.Clock.ElapsedPct.Pct()+" elapsed")
	}
	w("")

	w("THE OBLIGATION IN VALUE")
	rule()
	if a.Specific.MultipleApplies {
		kv("Duty saved", a.Specific.DutySaved.Indian())
		kv("Multiple applied", a.Specific.Multiple.Multiple()+" the duty saved")
		kv("Derived obligation", a.Specific.Derived.Indian())
	}
	kv("Obligation applied", a.Specific.Required.Indian()+" ("+string(a.Specific.Source)+")")
	kv("Counted toward it", a.Specific.Counted.Indian())
	kv("Discharged", a.Specific.FulfilledPct.Pct())
	if a.Specific.Shortfall > 0 {
		kv("Short by", a.Specific.Shortfall.Indian())
	}
	if a.Specific.Condoned > 0 {
		kv("Condoned", a.Specific.Condoned.Indian()+" (your condonation figure, not the Policy's)")
		kv("Net shortfall", a.Specific.NetShortfall.Indian())
	}
	kv("Standing", a.Specific.Status.Title())
	if a.NeededPerMonth > 0 {
		kv("Run rate that closes it", a.NeededPerMonth.Indian()+" a month for "+
			plural(a.MonthsLeft, "month", "months"))
	}
	if a.Specific.Disagreement != "" {
		w("")
		w(wrap(a.Specific.Disagreement, 72))
	}
	w("")

	if a.Quantity.Applies {
		w("THE OBLIGATION IN QUANTITY")
		rule()
		kv("Required", a.Quantity.Required.String()+" "+a.Quantity.Unit)
		kv("Exported", a.Quantity.Counted.String()+" "+a.Quantity.Unit)
		kv("Discharged", a.Quantity.FulfilledPct.Pct())
		if a.Quantity.Shortfall > 0 {
			kv("Short by", a.Quantity.Shortfall.String()+" "+a.Quantity.Unit)
		}
		kv("Standing", a.Quantity.Status.Title())
		w("")
	}

	if a.ValueAddition.Applies {
		w("VALUE ADDITION")
		rule()
		kv("Minimum required", a.ValueAddition.MinRequired.Pct())
		kv("CIF of authorisation inputs", a.ValueAddition.InputsCIF.Indian())
		if a.ValueAddition.OtherInputs > 0 {
			kv("Other imported inputs used", a.ValueAddition.OtherInputs.Indian())
		}
		kv("Denominator B", a.ValueAddition.DenominatorB.Indian())
		kv("Realised FOB (A)", a.ValueAddition.RealisedFOB.Indian())
		if a.ValueAddition.DenominatorB > 0 {
			kv("Value addition on realisation", a.ValueAddition.OnRealised.Pct())
			kv("Value addition on shipping bills", a.ValueAddition.OnShipped.Pct())
		}
		standing := a.ValueAddition.Status.Title()
		if a.ValueAddition.Status == StatusUnknown {
			standing = "Not yet measurable"
		}
		kv("Standing", standing)
		if a.ValueAddition.Note != "" {
			w("")
			w(wrap(a.ValueAddition.Note, 72))
		}
		w("")
	}

	if len(a.Blocks) > 0 {
		w("BLOCK-WISE MILESTONES")
		rule()
		w(pad("Block", 7) + pad("Ends", 20) + pad("Required", 24) + pad("Counted", 18) + "Standing")
		for _, bl := range a.Blocks {
			w(pad(itoa(bl.No), 7) +
				pad(bl.Window.To.Human(), 20) +
				pad(bl.Required.Indian()+" ("+itoa(bl.CumulativePercent)+"%)", 24) +
				pad(bl.Counted.Indian(), 18) +
				bl.Status.Title())
		}
		w("")
		for _, bl := range a.Blocks {
			if bl.Shortfall > 0 && !bl.Closed {
				w(wrap("Block "+itoa(bl.No)+": "+bl.Shortfall.Indian()+" still to count by "+
					bl.Window.To.Human()+", "+plural(bl.Clock.DaysLeft, "day", "days")+
					" away — about "+bl.NeededPerMonth.Indian()+" a month.", 72))
			}
			if bl.Shortfall > 0 && bl.Closed {
				w(wrap("Block "+itoa(bl.No)+" closed on "+bl.Window.To.Human()+" short by "+
					bl.Shortfall.Indian()+". A closed block that fell short is regularised separately.", 72))
			}
		}
		w("")
	}

	if a.EarlyFulfilment.Applies || a.EarlyFulfilment.Excluded {
		w("EARLY FULFILMENT — FTP 5.09")
		rule()
		if a.EarlyFulfilment.Applies {
			kv("Half the original period ends", a.EarlyFulfilment.Window.To.Human())
			kv("Needed by then", a.EarlyFulfilment.Required.Indian()+
				" ("+itoa(a.EarlyFulfilment.ThresholdPercent)+"% of the obligation)")
			kv("Counted by then", a.EarlyFulfilment.Counted.Indian())
			if a.EarlyFulfilment.Shortfall > 0 {
				kv("Short by", a.EarlyFulfilment.Shortfall.Indian())
			}
			kv("Average maintained throughout", map[bool]string{true: "yes", false: "no"}[a.EarlyFulfilment.AverageMaintained])
			if a.EarlyFulfilment.Condonable > 0 {
				kv("Balance that would be condoned", a.EarlyFulfilment.Condonable.Indian())
			}
			kv("Standing", a.EarlyFulfilment.Status.Title())
			w("")
		}
		w(wrap(a.EarlyFulfilment.Note, 72))
		w("")
	}

	if l.Scheme == SchemeEPCG {
		w("AVERAGE EXPORT OBLIGATION")
		rule()
		if !a.Average.Applies {
			w(wrap(a.Average.Note, 72))
		} else {
			kv("Average to maintain each year", a.Average.Annual.Indian())
			if a.Average.StillNeededThisYear > 0 {
				kv("Still needed this year", a.Average.StillNeededThisYear.Indian()+
					" by "+a.Average.CurrentYearCloses.Human())
			}
			kv("Measured against", a.Average.Basis.Title())
			kv("Part years", map[bool]string{true: "pro-rated by days (a convention of this tool)", false: "charged a full year's average"}[a.Average.ProRata])
			w("")
			w("Years are shown up to " + a.AsOf.Human() + " only: a year that has not happened")
			w("cannot have been failed.")
			w("")
			w(pad("Year", 12) + pad("Days", 7) + pad("Average due", 18) + pad("Same/similar", 18) + pad("To specific", 18) + "Average")
			for _, y := range a.Average.Years {
				mark := "met"
				if !y.AverageMet {
					mark = "SHORT by " + y.AverageShortfall.Indian()
				}
				w(pad(y.Label, 12) +
					pad(itoa(y.DaysCounted), 7) +
					pad(y.AverageDue.Indian(), 18) +
					pad(y.SameOrSimilar.Indian(), 18) +
					pad(y.CountedToSpecific.Indian(), 18) +
					mark)
			}
			w("")
			w("")
			kv("Average due, all years shown", a.Average.DueTotal.Indian())
			kv("Same/similar exports, all years", a.Average.OverallAchieved.Indian())
			kv("Maintained on an overall basis", map[bool]string{true: "yes", false: "no"}[a.Average.OverallMet])
			w("")
			w(wrap("Only the excess over the average counts toward the obligation in value. The year-by-year columns above are the conservative reading; HBP 5.18 allows excess in one year to offset a shortfall in another, provided the average is maintained on an overall basis within the block or the period, which is what the overall figures are for.", 72))
			if a.Average.OffsetAvailable {
				w("")
				w(wrap("A year here closed below the average, but the average HAS been maintained overall. That is the case HBP 5.18 is for. Put the overall figure in front of your Regional Authority; which reading it takes is its decision.", 72))
			}
			if a.Tally.OtherProducts > 0 {
				w("")
				w(wrap("Exports of other products, "+a.Tally.OtherProducts.Indian()+
					", are counted toward the obligation in full, because no average was fixed for them to be over and above.", 72))
			}
		}
		w("")
	}

	w("EXPORTS COUNTED")
	rule()
	kv("Shipping bills counted", itoa(a.Tally.Shipments))
	kv("FOB counted", a.Tally.TotalFOB.Indian())
	kv("Realised", a.Tally.RealisedTotal.Indian())
	if a.Tally.Unrealised > 0 {
		kv("Awaiting realisation", a.Tally.Unrealised.Indian()+" on "+
			plural(a.Tally.UnrealisedCount, "shipping bill", "shipping bills"))
	}
	if n := len(a.Tally.NotEndorsed); n > 0 {
		kv("NOT counted — unendorsed", plural(n, "shipping bill", "shipping bills")+
			" without the authorisation number on it")
	}
	if n := len(a.Tally.OutsidePeriod); n > 0 {
		kv("NOT counted — outside period", plural(n, "shipping bill", "shipping bills"))
	}
	if n := len(a.Tally.Undated); n > 0 {
		kv("NOT counted — undated", plural(n, "shipping bill", "shipping bills"))
	}
	if n := len(a.Tally.NotYet); n > 0 {
		kv("Not yet counted — future dated", plural(n, "shipping bill", "shipping bills"))
	}
	if n := len(a.Tally.Excluded); n > 0 {
		kv("Set aside by you", plural(n, "shipping bill", "shipping bills"))
	}
	w("")
	if len(l.Shipments) > 0 {
		w(pad("Shipped", 20) + pad("Shipping bill", 20) + pad("FOB", 16) + "Counted")
		for i, sh := range l.Shipments {
			why := "yes"
			switch {
			case contains(a.Tally.Excluded, i):
				why = "set aside"
				if sh.ExcludedReason != "" {
					why += " — " + sh.ExcludedReason
				}
			case contains(a.Tally.Undated, i):
				why = "no — undated"
			case contains(a.Tally.NotEndorsed, i):
				why = "no — not endorsed"
			case contains(a.Tally.OutsidePeriod, i):
				why = "no — outside the period"
			case contains(a.Tally.NotYet, i):
				why = "not yet — dated after " + a.AsOf.Human()
			}
			w(pad(sh.ShippedOn.Human(), 20) + pad(sh.ShippingBillNo, 20) +
				pad(sh.FOBValue.Indian(), 16) + why)
		}
		w("")
	}

	w("IMPORTS AGAINST THIS AUTHORISATION")
	rule()
	if len(l.Imports) == 0 {
		w("None entered.")
	} else {
		w(pad("Cleared", 20) + pad("Bill of entry", 18) + pad("CIF", 16) + pad("Duty saved", 16) + "Installed")
		for _, im := range l.Imports {
			installed := "not recorded"
			if !im.InstalledOn.Zero() {
				installed = im.InstalledOn.Human()
			}
			if im.Indigenous {
				installed += " (sourced indigenously)"
			}
			w(pad(im.ClearedOn.Human(), 20) + pad(im.BillOfEntryNo, 18) +
				pad(im.CIFValue.Indian(), 16) + pad(im.DutySaved.Indian(), 16) + installed)
		}
	}
	w("")

	w("IF THE OBLIGATION IS NOT DISCHARGED")
	rule()
	if !a.Exposure.Applies {
		w(wrap(a.Exposure.Basis, 72))
	} else {
		if a.Exposure.Hypothetical {
			w(wrap("The period is still running, so this is what would be payable if the licence closed today on today's figures. It is not a demand and nothing is payable yet.", 72))
			w("")
		}
		kv("Unfulfilled proportion", a.Exposure.ShortfallPct.Pct())
		kv("Duty that becomes payable", a.Exposure.DutyPayable.Indian())
		kv("Interest at "+a.Exposure.RateApplied.Pct()+" a year", a.Exposure.Interest.Indian())
		kv("Total", a.Exposure.Total.Indian())
		kv("Rising by", a.Exposure.Daily().Indian()+" a day")
		w("")
		w(pad("Cleared", 20) + pad("Bill of entry", 18) + pad("Duty payable", 16) + pad("Days", 7) + "Interest")
		for _, line := range a.Exposure.Lines {
			note := line.Interest.Indian()
			if line.Problem != "" {
				note = "— " + line.Problem
			}
			w(pad(line.ClearedOn.Human(), 20) + pad(line.BillOfEntryNo, 18) +
				pad(line.Proportionate.Indian(), 16) + pad(itoa(line.Days), 7) + note)
		}
		if a.Exposure.Understated != "" {
			w("")
			w(wrap("WARNING: "+a.Exposure.Understated, 72))
		}
		w("")
		w(wrap(a.Exposure.Basis, 72))
	}
	w("")

	w("CLOSING THE LICENCE — WHAT THE CERTIFICATE NEEDS")
	rule()
	w(wrap(a.EODC.Verdict, 72))
	w("")
	for _, r := range a.EODC.Requirements {
		mark := map[ReqState]string{
			ReqMet:           "[ok]      ",
			ReqMissing:       "[missing] ",
			ReqUnknown:       "[unknown] ",
			ReqNotApplicable: "[n/a]     ",
		}[r.State]
		w(mark + r.Label)
		if r.Detail != "" {
			w("          " + wrapIndent(r.Detail, 62, "          "))
		}
		if r.Action != "" && !r.Satisfied() {
			w("          -> " + wrapIndent(r.Action, 59, "             "))
		}
	}
	w("")

	if len(a.Alerts) > 0 {
		w("WHAT TO ACT ON")
		rule()
		for _, al := range a.Alerts {
			w(strings.ToUpper(string(al.Level)) + ": " + al.Title)
			w("  " + wrapIndent(al.Detail, 70, "  "))
			w("")
		}
	}

	w("FIGURES APPLIED, AND WHERE THEY COME FROM")
	rule()
	for _, c := range a.Policy.Citations() {
		w(wrap(c.Label+": "+c.Figure, 72))
		w("  " + wrapIndent(c.Source+" — "+c.Confidence, 70, "  "))
		if c.Note != "" {
			w("  " + wrapIndent(c.Note, 70, "  "))
		}
		w("")
	}

	w("ASSUMPTIONS AND LIMITS")
	rule()
	for _, n := range a.Notes {
		w("* " + wrapIndent(n, 70, "  "))
	}
	w("")
	if l.Notes != "" {
		w("YOUR NOTES")
		rule()
		w(wrap(l.Notes, 72))
		w("")
	}
	w(strings.Repeat("=", 72))
	w("Produced by an offline tool from figures entered by hand. Not advice, not")
	w("a filing, and not the Directorate General of Foreign Trade's reckoning.")
	w("Confirm every figure against the current Foreign Trade Policy, its")
	w("Handbook of Procedures, and the endorsements on your own authorisation.")

	return b.String()
}

// contains reports whether a sorted-or-not index list holds n.
func contains(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}

// pad left-aligns a string in a column, and never truncates: a figure cut short
// is a wrong figure, so a long value pushes the line out instead.
func pad(s string, n int) string {
	if len([]rune(s)) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

// wrap breaks text at word boundaries to a width.
func wrap(s string, width int) string { return wrapIndent(s, width, "") }

// wrapIndent breaks text at word boundaries, prefixing every line after the
// first with indent. The first line is left unprefixed so the caller can place
// it after a label.
func wrapIndent(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := words[0]
	for _, word := range words[1:] {
		if len([]rune(line))+1+len([]rune(word)) > width {
			b.WriteString(line + "\n" + indent)
			line = word
			continue
		}
		line += " " + word
	}
	b.WriteString(line)
	return b.String()
}
