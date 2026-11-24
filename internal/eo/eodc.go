package eo

// ReqState is where one requirement for closing a licence stands.
type ReqState string

const (
	// ReqMet — satisfied on the figures and the documents recorded.
	ReqMet ReqState = "met"
	// ReqMissing — not satisfied, and the tool can say why.
	ReqMissing ReqState = "missing"
	// ReqUnknown — not enough has been entered to say. Treated as not
	// satisfied, because a requirement nobody has checked is not a requirement
	// that has been met.
	ReqUnknown ReqState = "unknown"
	// ReqNotApplicable — the requirement does not arise on this licence.
	ReqNotApplicable ReqState = "not-applicable"
)

// Requirement is one item of the file that has to be in order before a
// redemption application can go in.
type Requirement struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	State    ReqState `json:"state"`
	Detail   string   `json:"detail"`
	Action   string   `json:"action"`
	Blocking bool     `json:"blocking"`
}

// Satisfied reports whether a requirement is out of the way — met, or not
// arising at all.
func (r Requirement) Satisfied() bool {
	return r.State == ReqMet || r.State == ReqNotApplicable
}

// EODCView is the closure step: an Export Obligation Discharge Certificate is
// what a Regional Authority issues when it accepts that an authorisation's
// obligation has been discharged, and it is what releases the bond and the bank
// guarantee given to Customs. Until it is in hand the licence is open, however
// much has been exported.
//
// This tool assembles the file and says whether it hangs together. It does NOT
// file anything: it has no connection to the DGFT portal, it cannot submit an
// application, and it does not assert the number of the form to file, because
// the Handbook's annexures are renumbered between editions and an invented form
// number would send somebody to the wrong place.
type EODCView struct {
	Scheme       Scheme        `json:"scheme"`
	Requirements []Requirement `json:"requirements"`

	MetCount      int  `json:"metCount"`
	BlockingCount int  `json:"blockingCount"`
	TotalCount    int  `json:"totalCount"`
	Ready         bool `json:"ready"`

	Verdict string `json:"verdict"`

	Filed         bool   `json:"filed"`
	FiledOn       Date   `json:"filedOn"`
	FileReference string `json:"fileReference"`
}

// AssessEODC builds the closure checklist.
//
// Almost every item is DERIVED from the figures and the documents entered,
// rather than being a box the user ticks. That is deliberate: a checklist of
// tick boxes tells you what somebody remembered to tick, and the thing that
// stops a redemption is usually something nobody remembered at all — a shipping
// bill filed without the authorisation number endorsed on it, which cannot be
// counted however much was shipped under it.
func AssessEODC(l Licence, a *Assessment) EODCView {
	v := EODCView{
		Scheme:        l.Scheme,
		Filed:         l.Evidence.ApplicationFiled,
		FiledOn:       l.Evidence.FiledOn,
		FileReference: l.Evidence.FileReference,
	}

	add := func(r Requirement) { v.Requirements = append(v.Requirements, r) }

	// ---- the obligation itself ----

	switch {
	case a.Specific.Required <= 0:
		add(Requirement{"specific-eo", "Export obligation in value discharged in full", ReqUnknown,
			"No export obligation figure has been entered for this authorisation.",
			"Enter the export obligation endorsed on the authorisation. Nothing else on this list can be judged without it.", true})
	case a.Specific.Status == StatusMet:
		add(Requirement{"specific-eo", "Export obligation in value discharged in full", ReqMet,
			a.Specific.Counted.Indian() + " counted against an obligation of " + a.Specific.Required.Indian() +
				" — " + a.Specific.FulfilledPct.Pct() + ".", "", true})
	default:
		add(Requirement{"specific-eo", "Export obligation in value discharged in full", ReqMissing,
			a.Specific.Counted.Indian() + " counted of " + a.Specific.Required.Indian() +
				" — " + a.Specific.FulfilledPct.Pct() + ". Short by " + a.Specific.NetShortfall.Indian() + ".",
			"Export and endorse a further " + a.Specific.NetShortfall.Indian() +
				" against this authorisation, or regularise the shortfall by paying the proportionate duty with interest.", true})
	}

	if l.Scheme == SchemeEPCG {
		switch {
		case l.AverageEOExempt:
			add(Requirement{"average-eo", "Average export obligation maintained in every licensing year", ReqNotApplicable,
				a.Average.Note, "", true})
		case l.AverageEO == 0:
			// Silence is not an exemption. An average left blank makes the
			// obligation look more discharged than it is, so it blocks the
			// checklist until the user either enters it or says it does not
			// apply.
			add(Requirement{"average-eo", "Average export obligation maintained in every licensing year", ReqUnknown,
				"No average export obligation has been entered, and the licence is not marked exempt from maintaining one.",
				"Enter the average endorsed on the authorisation, or mark the licence exempt. Leaving it blank OVERSTATES how much of the obligation has been discharged, because the specific obligation is to be met over and above the average.", true})
		case a.Average.AllYearsMet && a.Average.StillNeededThisYear == 0:
			detail := "An average of " + l.AverageEO.Indian() + " a year has been maintained in each of " +
				plural(a.Average.YearsClosed, "closed licensing year", "closed licensing years") + " so far."
			state := ReqMet
			if a.Average.StillNeededThisYear > 0 {
				state = ReqMissing
				detail += " The current year is " + a.Average.StillNeededThisYear.Indian() +
					" short of it with " + a.Average.CurrentYearCloses.Human() + " to go."
			}
			add(Requirement{"average-eo", "Average export obligation maintained in every licensing year", state,
				detail,
				"The average has to be maintained in every year of the period, including the ones still to come.", true})
		default:
			detail := plural(a.Average.YearsClosedShort, "closed licensing year", "closed licensing years") +
				" fell below the average of " + l.AverageEO.Indian() + ", by " +
				a.Average.ClosedShortfall.Indian() + " in all."
			action := "Take the shortfall to your Regional Authority. It is dealt with separately from the specific obligation."
			state := ReqMissing
			if a.Average.OffsetAvailable {
				detail += " On an overall basis the average HAS been maintained: " +
					a.Average.OverallAchieved.Indian() + " against " + a.Average.DueTotal.Indian() + "."
				action = "HBP 5.18 allows excess exports in one year to offset a shortfall in another, provided the average is maintained on an overall basis within the block or the obligation period. Put the overall figure in front of your Regional Authority; which reading it takes is its decision, not this tool's."
			}
			add(Requirement{"average-eo", "Average export obligation maintained in every licensing year", state,
				detail, action, true})
		}

		short := 0
		for _, b := range a.Blocks {
			if b.Status == StatusOverdue {
				short++
			}
		}
		switch {
		case len(a.Blocks) == 0:
			add(Requirement{"blocks", "Block-wise obligation met for every block", ReqUnknown,
				"The blocks could not be worked out, because the date of issue or the obligation period is missing.",
				"Enter the date of issue of the authorisation.", true})
		case short == 0:
			add(Requirement{"blocks", "Block-wise obligation met for every block", ReqMet,
				"No block has closed short.", "", true})
		default:
			add(Requirement{"blocks", "Block-wise obligation met for every block", ReqMissing,
				plural(short, "block", "blocks") + " closed with the obligation short.",
				"A closed block that fell short is regularised separately, whatever happens in the remaining block. Ask your Regional Authority what it needs.", true})
		}
	}

	if a.Quantity.Applies {
		if a.Quantity.Status == StatusMet {
			add(Requirement{"quantity-eo", "Export obligation in quantity discharged in full", ReqMet,
				a.Quantity.Counted.String() + " " + a.Quantity.Unit + " exported against " +
					a.Quantity.Required.String() + " " + a.Quantity.Unit + ".", "", true})
		} else {
			add(Requirement{"quantity-eo", "Export obligation in quantity discharged in full", ReqMissing,
				a.Quantity.Counted.String() + " of " + a.Quantity.Required.String() + " " + a.Quantity.Unit +
					" — short by " + a.Quantity.Shortfall.String() + " " + a.Quantity.Unit + ".",
				"An Advance Authorisation has to be discharged in quantity as well as in value. Exporting more value does not close a quantity shortfall.", true})
		}
	} else if l.Scheme == SchemeAdvanceAuthorisation {
		add(Requirement{"quantity-eo", "Export obligation in quantity discharged in full", ReqUnknown,
			"No obligation quantity has been entered.",
			"Enter the quantity of the resultant product the authorisation requires, with its unit. An Advance Authorisation is redeemed on quantity as well as value.", true})
	}

	if a.ValueAddition.Applies {
		switch a.ValueAddition.Status {
		case StatusMet:
			add(Requirement{"value-addition", "Minimum value addition achieved", ReqMet,
				"Value addition on realisation is " + a.ValueAddition.OnRealised.Pct() +
					" against a minimum of " + a.ValueAddition.MinRequired.Pct() + ".", "", true})
		case StatusUnknown:
			add(Requirement{"value-addition", "Minimum value addition achieved", ReqUnknown,
				a.ValueAddition.Note,
				"Value addition is computed on realisation, not on the shipping bills, and only means something once the exports are made and the proceeds are in. Record the CIF value of the inputs and the realised value against each shipping bill.", true})
		default:
			add(Requirement{"value-addition", "Minimum value addition achieved", ReqMissing,
				a.ValueAddition.Note,
				"Value addition below the minimum is a ground for refusing redemption on its own, whatever the export value comes to.", true})
		}
	}

	// ---- the clock ----

	switch {
	case !a.Clock.Window.Valid():
		add(Requirement{"period", "Obligation period not expired with the obligation short", ReqUnknown,
			"The obligation period could not be worked out.",
			"Enter the date of issue of the authorisation, and the end of the obligation period if it is endorsed on it.", true})
	case !a.Clock.Expired:
		add(Requirement{"period", "Obligation period not expired with the obligation short", ReqMet,
			plural(a.Clock.DaysLeft, "day", "days") + " left of the period, which ends " + a.Period.Ends.Human() +
				" (" + string(a.Period.EndsSource) + ").", "", true})
	case a.Specific.Status == StatusMet:
		add(Requirement{"period", "Obligation period not expired with the obligation short", ReqMet,
			"The period ended " + a.Period.Ends.Human() + " with the obligation discharged.", "", true})
	default:
		add(Requirement{"period", "Obligation period not expired with the obligation short", ReqMissing,
			"The period ended " + a.Period.Ends.Human() + ", " + plural(-a.Clock.DaysLeft, "day", "days") +
				" ago on this reckoning, with the obligation short.",
			"An extension cannot be granted after the event in the ordinary way. The duty saved proportionate to the shortfall, with interest, is what is in question now.", true})
	}

	// ---- the evidence ----

	if n := len(a.Tally.NotEndorsed); n > 0 {
		add(Requirement{"endorsement", "Authorisation number endorsed on every shipping bill offered", ReqMissing,
			plural(n, "shipping bill", "shipping bills") + " recorded here is not endorsed with the authorisation number, and is not counted.",
			"An export made without the authorisation number on the shipping bill is the commonest reason an exporter who has shipped enough still cannot close the licence. Ask Customs about amending the bill; it is not always possible.", true})
	} else if a.Tally.Shipments > 0 {
		add(Requirement{"endorsement", "Authorisation number endorsed on every shipping bill offered", ReqMet,
			"All " + itoa(a.Tally.Shipments) + " counted shipping bills are endorsed.", "", true})
	} else {
		add(Requirement{"endorsement", "Authorisation number endorsed on every shipping bill offered", ReqUnknown,
			"No shipping bills have been entered inside the obligation period.",
			"Enter the shipping bills you intend to offer against this authorisation.", true})
	}

	switch {
	case a.Tally.Shipments == 0:
		add(Requirement{"realisation", "Proof of realisation on file for every export offered", ReqUnknown,
			"No exports have been entered.", "Enter the shipping bills and the realisation against each.", true})
	case a.Tally.UnrealisedCount == 0:
		add(Requirement{"realisation", "Proof of realisation on file for every export offered", ReqMet,
			"Realisation recorded against all " + itoa(a.Tally.Shipments) + " counted exports.", "", true})
	default:
		add(Requirement{"realisation", "Proof of realisation on file for every export offered", ReqMissing,
			plural(a.Tally.UnrealisedCount, "counted export", "counted exports") + " worth " +
				a.Tally.Unrealised.Indian() + " has no realisation recorded.",
			"Collect the bank realisation certificates. An export counted toward the obligation but not realised will not go into a redemption file.", true})
	}

	if l.Scheme == SchemeEPCG {
		pending, pastCustoms, pastRA := installationDue(l, a.Policy, a.AsOf)
		switch {
		case len(l.Imports) == 0:
			add(Requirement{"installation", "Installation certificate held for every consignment", ReqUnknown,
				"No import consignments have been entered.",
				"Enter each consignment with its bill of entry, the duty saved on it and the date it was cleared.", true})
		case pending == 0:
			add(Requirement{"installation", "Installation certificate held for every consignment", ReqMet,
				"An installation certificate is recorded for every consignment.", "", true})
		default:
			detail := plural(pending, "consignment", "consignments") + " has no installation certificate recorded."
			if pending > 1 {
				detail = plural(pending, "consignment", "consignments") + " have no installation certificate recorded."
			}
			if pastCustoms > 0 {
				detail += " On " + itoa(pastCustoms) + " of them the " +
					plural(a.Policy.InstallationCertificateCustomsMonths, "month", "months") +
					" that Notification 26/2023-Customs cond. (8) allows has passed."
			}
			if pastRA > 0 {
				detail += " On " + itoa(pastRA) + " the " +
					plural(a.Policy.InstallationCertificateMonths, "month", "months") +
					" that HBP 5.04(a) allows the Regional Authority has passed too."
			}
			add(Requirement{"installation", "Installation certificate held for every consignment", ReqMissing,
				detail,
				"The certificate may come from jurisdictional Customs or from an independent chartered engineer, at your option (HBP 5.04(a)). Record its date. An extension is applied for, not assumed.", true})
		}
	}

	missingBoE := 0
	for _, im := range l.Imports {
		if im.Indigenous {
			continue
		}
		if im.BillOfEntryNo == "" {
			missingBoE++
		}
	}
	switch {
	case len(l.Imports) == 0:
		add(Requirement{"bills-of-entry", "Bill of entry recorded for every import", ReqUnknown,
			"No import consignments have been entered.", "Enter each consignment with its bill of entry.", true})
	case missingBoE == 0:
		add(Requirement{"bills-of-entry", "Bill of entry recorded for every import", ReqMet,
			"A bill of entry is recorded for every imported consignment.", "", true})
	default:
		add(Requirement{"bills-of-entry", "Bill of entry recorded for every import", ReqMissing,
			plural(missingBoE, "imported consignment", "imported consignments") + " has no bill of entry number.",
			"The redemption file is checked against the bills of entry. Find them before you apply.", true})
	}

	// ---- the few things the tool cannot work out ----

	if l.Evidence.CACertifiedStatement {
		add(Requirement{"ca-statement", "Statement of exports certified by a chartered accountant", ReqMet,
			"Recorded as held.", "", true})
	} else {
		add(Requirement{"ca-statement", "Statement of exports certified by a chartered accountant", ReqMissing,
			"Not recorded as held.",
			"A redemption application is normally accompanied by a statement of exports certified by a chartered accountant, cost accountant or company secretary. This tool cannot produce one.", true})
	}

	if l.Evidence.RCMCHeld {
		add(Requirement{"rcmc", "Valid registration-cum-membership certificate", ReqMet,
			"Recorded as held.", "", false})
	} else {
		add(Requirement{"rcmc", "Valid registration-cum-membership certificate", ReqMissing,
			"Not recorded as held.",
			"Check that the certificate from your export promotion council is valid and covers the product. Expiry of it holds up more applications than it should.", false})
	}

	if l.Evidence.BondOrBGDetails != "" {
		add(Requirement{"bond", "Bond or bank guarantee details recorded for cancellation", ReqMet,
			l.Evidence.BondOrBGDetails, "", false})
	} else {
		add(Requirement{"bond", "Bond or bank guarantee details recorded for cancellation", ReqMissing,
			"Not recorded.",
			"The certificate is what releases the bond and the bank guarantee you gave Customs. Record their particulars so the release can be chased.", false})
	}

	if l.Evidence.ApplicationFiled {
		detail := "Filed"
		if !l.Evidence.FiledOn.Zero() {
			detail += " on " + l.Evidence.FiledOn.Human()
		}
		if l.Evidence.FileReference != "" {
			detail += ", reference " + l.Evidence.FileReference
		}
		add(Requirement{"application", "Redemption application filed", ReqMet, detail + ".", "", false})
	} else {
		action := "Apply online in " + a.Policy.EODCForm(l.Scheme) + " (HBP 5.20(a)). " +
			"HBP 5.20 sets no deadline for applying, but Notification 26/2023-Customs cond. (6) wants evidence of block-wise fulfilment within thirty days of each block expiring. " +
			"The certificate that accompanies it is Appendix 5C, not Appendix 5B. " +
			"Confirm the form number against the current Handbook: its annexures are renumbered between editions. This tool files nothing — there is no portal connection here."
		if l.Scheme == SchemeAdvanceAuthorisation {
			due := ""
			if !a.Clock.Window.To.Zero() {
				due = " — by " + a.Clock.Window.To.AddMonths(a.Policy.AAEODCFilingMonths).Human()
			}
			action = "Apply online in " + a.Policy.EODCForm(l.Scheme) + " (HBP 4.46, 4.47(b)(i)) within " +
				plural(a.Policy.AAEODCFilingMonths, "month", "months") +
				" of the obligation period expiring" + due +
				", failing which HBP 4.43(b) warns of a show cause notice under the Foreign Trade (Development and Regulation) Act. " +
				"Notification 21/2023-Customs cond. (xi) separately wants evidence of discharge with Customs within sixty days. " +
				"Confirm the form number against the current Handbook. This tool files nothing — there is no portal connection here."
		}
		add(Requirement{"application", "Redemption application filed", ReqMissing, "Not filed.", action, false})
	}

	// ---- the verdict ----

	for _, r := range v.Requirements {
		v.TotalCount++
		if r.Satisfied() {
			v.MetCount++
			continue
		}
		if r.Blocking {
			v.BlockingCount++
		}
	}
	v.Ready = v.BlockingCount == 0

	switch {
	case v.Ready && v.Filed:
		v.Verdict = "The file hangs together on the figures entered, and the application is recorded as filed."
	case v.Ready:
		v.Verdict = "On the figures and documents entered, nothing on this list stands in the way of applying for the certificate. What a Regional Authority accepts is its own decision."
	case v.BlockingCount == 1:
		v.Verdict = "One thing on this list stands in the way of applying for the certificate."
	default:
		v.Verdict = itoa(v.BlockingCount) + " things on this list stand in the way of applying for the certificate."
	}
	return v
}
