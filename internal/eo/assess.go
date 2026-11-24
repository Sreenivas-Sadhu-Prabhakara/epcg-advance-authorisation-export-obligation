package eo

// AverageView is the average export obligation and whether it was maintained.
//
// It is reported year by year and never as a single total, because a licensing
// year that closed below the average cannot be made up by exporting more in a
// later one. The average is a floor under each year, not a total over the
// period, and that is the distinction that costs exporters money.
type AverageView struct {
	Applies bool             `json:"applies"`
	Annual  Money            `json:"annual"`
	Basis   AverageYearBasis `json:"basis"`
	ProRata bool             `json:"proRata"`

	Years []YearTally `json:"years"`

	// AllYearsMet looks only at years that have CLOSED. An open year is not a
	// year that has been failed.
	AllYearsMet      bool `json:"allYearsMet"`
	YearsClosed      int  `json:"yearsClosed"`
	YearsClosedShort int  `json:"yearsClosedShort"`
	YearsShort       int  `json:"yearsShort"`

	// ShortfallTotal and ClosedShortfall are the shortfall across every year
	// shown and across the closed years only. The second is the one that has
	// actually gone wrong; the first includes a year still being worked on.
	ShortfallTotal  Money `json:"shortfallTotal"`
	ClosedShortfall Money `json:"closedShortfall"`
	DueTotal        Money `json:"dueTotal"`

	// OverallAchieved and OverallMet are the same-or-similar exports and the
	// average obligation added up across every year shown, which is the OTHER
	// way the requirement can be read.
	//
	// HBP 2023 para 5.18 allows excess exports in one year to offset a
	// shortfall in another, provided the average is maintained on an overall
	// basis within the block or the obligation period. So a year that closed
	// short is not necessarily a year that has gone wrong. The year-by-year
	// position and the overall one are both reported, and this tool does not
	// decide which a Regional Authority will take.
	OverallAchieved Money `json:"overallAchieved"`
	OverallMet      bool  `json:"overallMet"`
	// OffsetAvailable is the case HBP 5.18 is for: a year short, but the
	// average maintained overall.
	OffsetAvailable bool `json:"offsetAvailable"`

	// CurrentYear is the measurement year today falls in, where it falls in
	// one. CurrentYearShort is how far behind the PRO-RATED average it is, and
	// StillNeededThisYear how much more of the same or a similar product has to
	// be exported before the year closes to maintain the FULL annual average.
	// The second is the number an exporter can act on.
	CurrentYear         *YearTally `json:"currentYear"`
	CurrentYearShort    Money      `json:"currentYearShort"`
	StillNeededThisYear Money      `json:"stillNeededThisYear"`
	CurrentYearCloses   Date       `json:"currentYearCloses"`

	Status Status `json:"status"`
	Note   string `json:"note"`
}

// Alert is something worth saying out loud, in the order it should be read.
type Alert struct {
	Level  AlertLevel `json:"level"`
	Key    string     `json:"key"`
	Title  string     `json:"title"`
	Detail string     `json:"detail"`
}

// AlertLevel grades an alert.
type AlertLevel string

const (
	AlertUrgent AlertLevel = "urgent"
	AlertWarn   AlertLevel = "warn"
	AlertInfo   AlertLevel = "info"
	AlertGood   AlertLevel = "good"
)

// Headline is the one figure to put at the top of the screen, with the words
// that make it mean something.
type Headline struct {
	Label string     `json:"label"`
	Value string     `json:"value"`
	Sub   string     `json:"sub"`
	Level AlertLevel `json:"level"`
}

// Assessment is everything this package has to say about one licence on one
// day.
type Assessment struct {
	AsOf   Date   `json:"asOf"`
	Scheme Scheme `json:"scheme"`

	AuthorisationNo string `json:"authorisationNo"`
	Holder          string `json:"holder"`
	Product         string `json:"product"`
	Currency        string `json:"currency"`
	Label           string `json:"label"`

	Policy       Policy `json:"policy"`
	PolicyEdited bool   `json:"policyEdited"`

	Period  PeriodPlan `json:"period"`
	Clock   Clock      `json:"clock"`
	Urgency Urgency    `json:"urgency"`

	Tally Tally `json:"tally"`

	Specific        ObligationView      `json:"specific"`
	Average         AverageView         `json:"average"`
	Blocks          []BlockAssessment   `json:"blocks"`
	EarlyFulfilment EarlyFulfilmentView `json:"earlyFulfilment"`
	Quantity        QuantityView        `json:"quantity"`
	ValueAddition   ValueAdditionView   `json:"valueAddition"`
	Exposure        ExposureView        `json:"exposure"`
	EODC            EODCView            `json:"eodc"`

	// NeededPerMonth is the run rate that would close the whole obligation in
	// the days left of the period.
	NeededPerMonth Money `json:"neededPerMonth"`
	MonthsLeft     int   `json:"monthsLeft"`

	Alerts   []Alert  `json:"alerts"`
	Headline Headline `json:"headline"`
	Notes    []string `json:"notes"`
}

// Assess measures a licence on a given day. It is the one entry point the rest
// of the program uses, and it is a pure function: the same licence and the same
// day always give the same assessment.
func Assess(l Licence, asOf Date) Assessment {
	l = l.Normalise()
	p := l.Policy.Normalise()
	if asOf.Zero() {
		asOf = Today()
	}

	a := Assessment{
		AsOf:            asOf,
		Scheme:          l.Scheme,
		AuthorisationNo: l.AuthorisationNo,
		Holder:          l.Holder,
		Product:         l.Product,
		Currency:        l.Currency,
		Label:           l.Label(),
		Policy:          p,
		PolicyEdited:    p.Edited,
	}

	a.Period = PlanPeriod(l)
	window := a.Period.PeriodWindow()
	a.Clock = ClockFor(window, asOf)
	a.Urgency = UrgencyOf(a.Clock)

	// Count the obligation period UP TO TODAY, not the whole of it. Counting
	// the whole period would attribute a full year's average export obligation
	// to licensing years that have not happened yet, and report an exporter as
	// having already failed them — which is both wrong and the kind of wrong
	// that makes somebody stop trusting the tool.
	countTo := window.To
	if window.Valid() && asOf.Before(countTo) {
		countTo = asOf
	}
	a.Tally = Count(l, p, Window{From: window.From, To: countTo}, window)

	a.Specific = SpecificObligation(l, p).
		fill(a.Tally.CountedToSpecific, p.CondonationPercent, a.Clock.Expired)
	if a.Specific.Status == StatusBehind && paceReaches(a.Specific.Counted, a.Specific.Required, a.Clock) {
		a.Specific.Status = StatusOnTrack
	}

	a.Average = averageView(l, p, a.Tally, asOf)
	a.Blocks = AssessBlocks(l, p, a.Period, a.Specific.Required, asOf)
	a.EarlyFulfilment = AssessEarlyFulfilment(l, p, a.Period, a.Specific.Required, asOf)
	a.Quantity = QuantityObligation(l, a.Tally.Quantity, a.Clock)
	a.ValueAddition = ValueAddition(l, p, a.Tally.RealisedTotal, a.Tally.TotalFOB,
		a.Clock.Expired || a.Specific.Status == StatusMet)
	a.Exposure = Exposure(l, p, a.Specific.Required, a.Specific.NetShortfall, asOf, a.Clock.Expired)

	if a.Specific.NetShortfall > 0 && !a.Clock.Expired && window.Valid() {
		a.MonthsLeft = MonthsBetween(asOf, window.To)
		months := a.MonthsLeft
		if months < 1 {
			months = 1
		}
		a.NeededPerMonth = a.Specific.NetShortfall.MulDiv(1, int64(months))
	}

	a.EODC = AssessEODC(l, &a)
	a.Notes = notes(l, p, a)
	a.Alerts = alerts(l, p, a)
	a.Headline = headline(a)
	return a
}

// averageView turns the per-year tally into the average obligation's own view.
func averageView(l Licence, p Policy, t Tally, asOf Date) AverageView {
	v := AverageView{
		Applies:        t.AverageApplies,
		Annual:         l.AverageEO,
		Basis:          p.AverageBasis,
		ProRata:        p.ProRataPartYears(),
		Years:          t.Years,
		DueTotal:       t.AverageDueTotal,
		ShortfallTotal: t.AverageShortfallTotal,
	}

	switch {
	case l.Scheme != SchemeEPCG:
		v.Note = "An average export obligation is a feature of the EPCG scheme. It does not arise on an Advance Authorisation."
	case l.AverageEOExempt:
		v.Note = "This authorisation is recorded as exempt from maintaining an average export obligation."
		if l.AverageEOExemptReason != "" {
			v.Note += " Reason recorded: " + l.AverageEOExemptReason + "."
		}
		v.Note += " Whether an exemption applies is a matter of the Policy and of the endorsement on the authorisation; this tool records what you tell it."
	case l.AverageEO == 0:
		v.Note = "No average export obligation has been entered. If the authorisation endorses one, enter it: the specific obligation is to be discharged over and above the average, so leaving it out OVERSTATES how much of the obligation has been discharged."
	}

	if !v.Applies {
		v.AllYearsMet = true
		v.Status = StatusUnknown
		if l.Scheme != SchemeEPCG || l.AverageEOExempt {
			v.Status = StatusMet
		}
		return v
	}

	v.AllYearsMet = true
	for i := range v.Years {
		y := &v.Years[i]
		// A year has CLOSED when its own full year is over — not when the
		// counting window ran out. The last year in the list is clipped to
		// today, so its clipped end is today, and today is not a year end.
		closed := !asOf.Zero() && asOf.After(y.FullWindow.To)
		if closed {
			v.YearsClosed++
		}
		if !y.AverageMet {
			v.YearsShort++
			if closed {
				v.AllYearsMet = false
				v.YearsClosedShort++
				v.ClosedShortfall += y.AverageShortfall
			}
		}
		if !asOf.Zero() && y.FullWindow.Contains(asOf) {
			current := *y
			v.CurrentYear = &current
			v.CurrentYearShort = y.AverageShortfall
			v.CurrentYearCloses = y.FullWindow.To
			if l.AverageEO > y.SameOrSimilar {
				v.StillNeededThisYear = l.AverageEO - y.SameOrSimilar
			}
		}
	}

	v.OverallAchieved = t.SameOrSimilar
	v.OverallMet = v.OverallAchieved >= v.DueTotal
	v.OffsetAvailable = !v.AllYearsMet && v.OverallMet

	switch {
	case !v.AllYearsMet && !v.OverallMet:
		v.Status = StatusOverdue
	case !v.AllYearsMet:
		v.Status = StatusBehind // short in a year, but made up overall
	case v.StillNeededThisYear > 0:
		v.Status = StatusBehind
	default:
		v.Status = StatusMet
	}
	return v
}

// notes are the assumptions and interpretations behind the figures, written out
// so that a reader can disagree with a specific one rather than with the whole
// result.
func notes(l Licence, p Policy, a Assessment) []string {
	out := []string{}

	out = append(out, "Every figure in the policy block is INDICATIVE and user-overridable. The Foreign Trade Policy and its Handbook of Procedures are amended often. These defaults were compiled on "+
		PolicyFiguresCompiledOn+" from "+p.Edition+". Confirm them against the current Policy, or against the endorsement on your own authorisation, which governs.")

	if p.Edited {
		out = append(out, "At least one policy figure on this licence has been changed from the published default. The report shows the figures actually applied.")
	}

	if a.Period.EndsSource == SourceDerived {
		out = append(out, "The end of the obligation period shown here is DERIVED: "+plural(a.Period.PeriodMonths, "month", "months")+
			" from the date of issue, ending "+map[bool]string{true: "the day before the anniversary", false: "on the anniversary"}[p.EndsDayBefore()]+
			". Enter the end date endorsed on the authorisation and this convention is not used.")
	}
	if a.Period.Disagreement != "" {
		out = append(out, a.Period.Disagreement)
	}
	if a.Specific.Disagreement != "" {
		out = append(out, a.Specific.Disagreement)
	}

	if l.Scheme == SchemeEPCG && !a.Average.Applies && a.Average.Note != "" {
		out = append(out, a.Average.Note)
	}
	if a.Average.Applies {
		out = append(out, "The specific obligation is counted OVER AND ABOVE the average (FTP 2023 para 5.04(c)): in each "+
			map[AverageYearBasis]string{AverageByAuthorisationYear: "anniversary year", AverageByLicensingYear: "licensing year"}[p.AverageBasis]+
			", only the excess of same-or-similar exports over the average counts toward it.")
		out = append(out, "The year-by-year figures are the conservative reading. HBP 2023 para 5.18 allows excess exports in one year to offset a shortfall in the average in another, provided the average is maintained on an OVERALL basis within the block or the obligation period, so both positions are reported. This tool nets the average year by year when counting toward the specific obligation, which is the harsher of the two readings, and does not decide which one a Regional Authority will apply.")
		out = append(out, "Two further reliefs are not computed here because they depend on facts outside the licence: HBP 5.17 reduces the average where the whole sector's or product group's exports fell by more than 5% against the previous year, and HBP 5.19 suspends it for the duration of a ban or restriction on the export product, with no composition fee.")
		if a.Average.ProRata {
			hasPart := false
			for _, y := range a.Average.Years {
				if y.PartYear {
					hasPart = true
					break
				}
			}
			if hasPart {
				out = append(out, "A part year — at either end of the obligation period, or where a block boundary falls — carries a share of the annual average in proportion to its days. That is a convention of this tool, not a rule of the Policy, and it can be turned off.")
			}
		}
		if a.Tally.OtherProducts > 0 {
			out = append(out, "Exports of products other than the one the average was fixed on, "+a.Tally.OtherProducts.Indian()+
				" here, are counted toward the specific obligation IN FULL, with nothing netted off, because no average was fixed for them to be over and above. That is an interpretation; check it with your Regional Authority if the figure matters.")
		}
	}

	if l.Currency != DefaultCurrency {
		out = append(out, "Figures on this licence are in "+l.Currency+". No exchange rate is applied anywhere in this tool and nothing is converted: every amount entered must be in the same currency as the obligation endorsed on the authorisation.")
	}

	if a.Exposure.Applies {
		out = append(out, "Interest is computed as SIMPLE interest at "+a.Exposure.RateApplied.Pct()+
			" a year, in calendar days from the date each consignment was cleared, on a "+itoa(a.Exposure.DayBasis)+
			"-day year. The rate and the start date come from the customs notification that gives the scheme effect. "+
			"WHETHER THE INTEREST IS SIMPLE OR COMPOUND IS NOT STATED in that notification, in section 28AA of the Customs Act, or in any circular found for this build. Simple interest is this tool's decision, not a sourced figure, and if the department compounds it the figure here is low.")
		out = append(out, "The rate is applied to the WHOLE duty-saved figure you enter. That may overstate it: there is a developed line of authority that interest cannot be levied on the additional and special additional customs duty components without an express charging provision, and CBIC Circular 11/2023-Customs took that view under the 2023 settlement scheme. If part of your duty saved is additional duty, special additional duty or IGST, take advice before treating this as the liability.")
		out = append(out, "The end date is also unstated. HBP 4.50(e) implies the rate applicable on the date of PAYMENT, so in practice interest runs to payment and not to the date of a demand. This figure is computed to the day it is produced for.")
	}

	if l.Scheme == SchemeAdvanceAuthorisation {
		out = append(out, "The "+plural(p.AAPeriodMonths, "month", "months")+" period applied here is the general one in HBP 2023 para 4.40(a). MUCH SHORTER PERIODS, counted from the date each consignment is CLEARED rather than from the date of issue, apply to the inputs listed in Appendix 4J: four months for precious metals in the gem and jewellery sector, three for maize, six for tea, coconut oil, raw sugar, wheat and most spices, twelve for silk. Defence, military store, aerospace and nuclear energy get 24 months under HBP 4.40(c). If your input is on Appendix 4J, enter the endorsed period: the default here would be wrong by a year.")
		out = append(out, "Where the obligation is met in quantity but short in value, HBP 2023 para 4.49(b) imposes nothing if the minimum value addition was achieved, and "+itoa(AAValueShortfallFeePercent)+"% of the shortfall in FOB value if it was not. Where it is met in value but short in quantity, HBP 4.49(a) requires duty with interest on the unutilised material, plus 10% of its CIF value where the input was restricted. This tool reports the headline exposure; it does not compute those, because the pro-rata arithmetic runs on figures a Regional Authority verifies against Appendix 4H.")
	}
	if l.Scheme == SchemeEPCG {
		out = append(out, "Only ONE of the concessions in FTP 2023 paras 5.04(d), 5.09, 5.10 and 5.11 is admissible (FTP 5.04(l)): indigenous sourcing, early-fulfilment condonation, the green technology reduction and the specified-area reduction cannot be stacked. This tool applies one category at a time.")
	}
	out = append(out, "An authorisation issued BEFORE 1 April 2023 runs under a different customs notification, whose rate and conditions this build has not read. Notification 26/2023-Customs defines the Policy as FTP 2023 and governs FTP 2023 authorisations only. If yours is older, read the notification quoted on your own bill of entry and set the figures here to match it.")
	out = append(out, "This tool has no connection to the DGFT portal, to ICEGATE or to Customs. It files nothing, it submits nothing, and it cannot obtain an Export Obligation Discharge Certificate. It does arithmetic on what you typed in.")
	return out
}

// alerts are the things to act on, urgent first.
func alerts(l Licence, p Policy, a Assessment) []Alert {
	out := []Alert{}

	if a.Clock.Expired && a.Specific.NetShortfall > 0 {
		out = append(out, Alert{AlertUrgent, "period-expired",
			"The obligation period has ended with the obligation short",
			"It ended " + a.Period.Ends.Human() + ". " + a.Specific.NetShortfall.Indian() +
				" of the obligation was not discharged, and " + a.Exposure.Total.Indian() +
				" of duty and interest is in question — rising by about " + a.Exposure.Daily().Indian() + " a day."})
	}

	for _, b := range a.Blocks {
		if b.Status != StatusOverdue {
			continue
		}
		payBy := b.Window.To.AddMonths(p.BlockDefaultPayMonths)
		detail := "It closed " + b.Window.To.Human() + " needing " + b.Required.Indian() +
			"; " + b.Counted.Indian() + " was counted, short by " + b.Shortfall.Indian() + ". " +
			"Notification 26/2023-Customs cond. (6) gives " +
			plural(p.BlockDefaultPayMonths, "month", "months") + " from the expiry of the block — to " +
			payBy.Human() + " — to pay the proportionate duty with interest, and thirty days to produce evidence of fulfilment. " +
			"HBP 5.13(c) gives " + plural(EPCGBlockDefaultPayHBPMonths, "month", "months") +
			" for the same payment; the shorter window is the one shown."
		if !a.AsOf.Zero() && a.AsOf.After(payBy) {
			detail += " That window has itself passed."
		}
		out = append(out, Alert{AlertUrgent, "block-" + itoa(b.No) + "-overdue",
			"Block " + itoa(b.No) + " closed with the obligation short", detail})
	}
	for _, b := range a.Blocks {
		if b.Status == StatusMet || b.Closed || b.Shortfall == 0 {
			continue
		}
		switch b.Urgency {
		case UrgencyUrgent:
			out = append(out, Alert{AlertUrgent, "block-" + itoa(b.No) + "-due",
				"Block " + itoa(b.No) + " closes in " + plural(b.Clock.DaysLeft, "day", "days"),
				"It needs " + b.Required.Indian() + " by " + b.Window.To.Human() + " and " +
					b.Counted.Indian() + " has been counted. " + b.Shortfall.Indian() + " to go."})
		case UrgencySoon:
			out = append(out, Alert{AlertWarn, "block-" + itoa(b.No) + "-soon",
				"Block " + itoa(b.No) + " closes in " + plural(b.Clock.DaysLeft, "day", "days"),
				b.Shortfall.Indian() + " still to export and endorse, about " +
					b.NeededPerMonth.Indian() + " a month."})
		default:
			if b.Status == StatusBehind && b.Current {
				out = append(out, Alert{AlertWarn, "block-" + itoa(b.No) + "-behind",
					"Block " + itoa(b.No) + " is behind the pace that would close it",
					b.Counted.Indian() + " of " + b.Required.Indian() + " counted with " +
						plural(b.Clock.DaysLeft, "day", "days") + " left. About " +
						b.NeededPerMonth.Indian() + " a month from here closes it."})
			}
		}
	}

	if !a.Clock.Expired && a.Specific.NetShortfall > 0 {
		switch a.Urgency {
		case UrgencyUrgent:
			out = append(out, Alert{AlertUrgent, "period-due",
				"The obligation period ends in " + plural(a.Clock.DaysLeft, "day", "days"),
				a.Specific.NetShortfall.Indian() + " is still to be discharged. If the period ends short, " +
					a.Exposure.Total.Indian() + " of duty and interest comes into question on today's figures."})
		case UrgencySoon:
			out = append(out, Alert{AlertWarn, "period-soon",
				"The obligation period ends in " + plural(a.Clock.DaysLeft, "day", "days"),
				a.Specific.NetShortfall.Indian() + " to go, about " + a.NeededPerMonth.Indian() + " a month."})
		}
	}

	if a.Average.Applies && !a.Average.AllYearsMet {
		level, detail := AlertUrgent,
			plural(a.Average.YearsClosedShort, "closed licensing year", "closed licensing years")+
				" fell below the average export obligation, by "+a.Average.ClosedShortfall.Indian()+
				" in all, and the average has not been made up across the years either ("+
				a.Average.OverallAchieved.Indian()+" of "+a.Average.DueTotal.Indian()+" overall)."
		if a.Average.OffsetAvailable {
			level = AlertWarn
			detail = plural(a.Average.YearsClosedShort, "closed licensing year", "closed licensing years") +
				" fell below the average export obligation, by " + a.Average.ClosedShortfall.Indian() +
				" in all. On an OVERALL basis the average has been maintained — " +
				a.Average.OverallAchieved.Indian() + " against " + a.Average.DueTotal.Indian() +
				" — and HBP 5.18 allows excess in one year to offset a shortfall in another on that basis. " +
				"Which reading your Regional Authority takes is its own decision; put the overall figure in front of it."
		}
		out = append(out, Alert{level, "average-closed-short",
			plural(a.Average.YearsClosedShort, "closed year", "closed years") + " fell below the average export obligation",
			detail})
	}
	if a.Average.Applies && a.Average.StillNeededThisYear > 0 && a.Average.CurrentYear != nil {
		out = append(out, Alert{AlertWarn, "average-current-year",
			"The current year has not yet reached the average export obligation",
			a.Average.CurrentYear.Label + " has " + a.Average.CurrentYear.SameOrSimilar.Indian() +
				" of same-or-similar exports against an annual average of " + a.Average.Annual.Indian() +
				". A further " + a.Average.StillNeededThisYear.Indian() + " is needed by " +
				a.Average.CurrentYearCloses.Human() +
				", and nothing beyond the average counts toward the obligation in value until it is cleared."})
	}

	if a.EarlyFulfilment.Applies && a.Specific.Status != StatusMet {
		switch a.EarlyFulfilment.Status {
		case StatusMet:
			out = append(out, Alert{AlertGood, "early-fulfilment",
				"The balance of the obligation may be condoned under FTP 5.09",
				a.EarlyFulfilment.Note})
		case StatusBehind:
			level := AlertInfo
			if a.EarlyFulfilment.Clock.DaysLeft <= WarnWithinDays {
				level = AlertWarn
			}
			out = append(out, Alert{level, "early-fulfilment",
				"Early fulfilment would condone the balance — " +
					plural(a.EarlyFulfilment.Clock.DaysLeft, "day", "days") + " left to earn it",
				a.EarlyFulfilment.Note})
		}
	}

	if n := len(a.Tally.NotEndorsed); n > 0 {
		var value Money
		for _, i := range a.Tally.NotEndorsed {
			value += l.Shipments[i].FOBValue
		}
		out = append(out, Alert{AlertWarn, "not-endorsed",
			plural(n, "shipping bill", "shipping bills") + " is not endorsed with the authorisation number",
			value.Indian() + " of exports cannot be counted toward the obligation as recorded. This is the commonest reason an exporter who has shipped enough still cannot close the licence."})
	}

	if n := len(a.Tally.OutsidePeriod); n > 0 {
		var value Money
		for _, i := range a.Tally.OutsidePeriod {
			value += l.Shipments[i].FOBValue
		}
		out = append(out, Alert{AlertInfo, "outside-period",
			plural(n, "export", "exports") + " falls outside the obligation period",
			value.Indian() + " shipped before " + a.Period.Start.Human() + " or after " +
				a.Period.Ends.Human() + ", and not counted."})
	}

	if l.Scheme == SchemeEPCG {
		pending, pastCustoms, pastRA := installationDue(l, p, a.AsOf)
		switch {
		case pastRA > 0:
			out = append(out, Alert{AlertWarn, "installation-overdue",
				"An installation certificate is overdue on " + plural(pastRA, "consignment", "consignments"),
				"HBP 5.04(a) allows " + plural(p.InstallationCertificateMonths, "month", "months") +
					" from completing the import, extendable by the Regional Authority up to the obligation period on a composition fee. " +
					"Notification 26/2023-Customs cond. (8) allows only " +
					plural(p.InstallationCertificateCustomsMonths, "month", "months") +
					", so Customs' own deadline went first. An extension is applied for, not assumed."})
		case pastCustoms > 0:
			out = append(out, Alert{AlertWarn, "installation-customs-overdue",
				"Customs' installation-certificate deadline has passed on " + plural(pastCustoms, "consignment", "consignments"),
				"Notification 26/2023-Customs cond. (8) allows " +
					plural(p.InstallationCertificateCustomsMonths, "month", "months") +
					" from completing the import (three years for spares), while HBP 5.04(a) allows the Regional Authority " +
					plural(p.InstallationCertificateMonths, "month", "months") + ". " +
					"The two deadlines diverge and nothing reconciles them, so both are reported. " +
					"Where the Regional Authority extends beyond the customs period, Customs is to extend correspondingly."})
		case pending > 0:
			out = append(out, Alert{AlertInfo, "installation-pending",
				"An installation certificate is not yet recorded on " + plural(pending, "consignment", "consignments"),
				"Record the date once you have it; the redemption file is checked for it. " +
					"It may come from jurisdictional Customs or from an independent chartered engineer, at your option (HBP 5.04(a))."})
		}
	}

	if a.Tally.UnrealisedCount > 0 {
		out = append(out, Alert{AlertInfo, "unrealised",
			plural(a.Tally.UnrealisedCount, "counted export", "counted exports") + " has no proof of realisation",
			a.Tally.Unrealised.Indian() + " counted toward the obligation but not shown as realised. It counts here; it will not go into a redemption file."})
	}
	if a.Tally.RealisedShortfall > 0 {
		out = append(out, Alert{AlertWarn, "realised-short",
			"Realisation came in below the shipping bill value",
			a.Tally.RealisedShortfall.Indian() + " less was realised than the shipping bills show. This tool counts the shipping bill value and reports the difference rather than choosing between them."})
	}

	if a.Quantity.Applies && a.Quantity.Status != StatusMet {
		level := AlertWarn
		if a.Clock.Expired {
			level = AlertUrgent
		}
		out = append(out, Alert{level, "quantity-short",
			"The obligation quantity is short",
			a.Quantity.Counted.String() + " of " + a.Quantity.Required.String() + " " + a.Quantity.Unit +
				" exported. An Advance Authorisation is discharged in quantity as well as value; exporting more value does not close it."})
	}
	if a.ValueAddition.Applies && a.ValueAddition.Status == StatusBehind {
		out = append(out, Alert{AlertWarn, "value-addition",
			"Value addition is below the minimum", a.ValueAddition.Note +
				" Value addition below the minimum is a ground for refusing redemption on its own."})
	}

	if a.Specific.Status == StatusMet && a.EODC.Ready {
		out = append(out, Alert{AlertGood, "ready",
			"Nothing on the closure list stands in the way of applying",
			a.EODC.Verdict})
	} else if a.Specific.Status == StatusMet {
		out = append(out, Alert{AlertInfo, "obligation-met",
			"The obligation is discharged, but the file is not complete",
			a.EODC.Verdict})
	}

	return out
}

// headline picks the one figure to show large.
func headline(a Assessment) Headline {
	switch {
	case a.Specific.Required <= 0:
		return Headline{"Export obligation", "not entered",
			"Enter the obligation endorsed on the authorisation, or the duty saved for it to be derived from.", AlertInfo}

	case a.Clock.Expired && a.Specific.NetShortfall > 0:
		return Headline{"Duty and interest in question", a.Exposure.Total.Indian(),
			"The period ended " + a.Period.Ends.Human() + " short by " + a.Specific.NetShortfall.Indian() +
				" — " + a.Specific.FulfilledPct.Pct() + " of the obligation discharged. Rising by about " +
				a.Exposure.Daily().Indian() + " a day.", AlertUrgent}

	case a.Specific.NetShortfall > 0:
		level := AlertInfo
		switch a.Urgency {
		case UrgencyUrgent:
			level = AlertUrgent
		case UrgencySoon:
			level = AlertWarn
		}
		return Headline{"Still to export and endorse", a.Specific.NetShortfall.Indian(),
			a.Specific.FulfilledPct.Pct() + " of " + a.Specific.Required.Indian() + " discharged, " +
				plural(a.Clock.DaysLeft, "day", "days") + " left — about " + a.NeededPerMonth.Indian() + " a month.", level}

	case a.EODC.Ready:
		return Headline{"Obligation discharged", a.Specific.FulfilledPct.Pct(),
			a.Specific.Counted.Indian() + " counted against " + a.Specific.Required.Indian() +
				". Nothing on the closure list stands in the way of applying for the certificate.", AlertGood}

	default:
		return Headline{"Obligation discharged, file incomplete", a.Specific.FulfilledPct.Pct(),
			a.EODC.Verdict, AlertWarn}
	}
}
