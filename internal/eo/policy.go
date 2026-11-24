// Package eo computes export obligations under India's EPCG and Advance
// Authorisation schemes: what is owed, what has been counted toward it, what is
// still short, how long is left, and what becomes payable if the obligation is
// not discharged.
//
// Everything in this package is a pure function of its arguments. Nothing here
// reads a file, opens a socket, or looks at the clock except through Today(),
// so every rule can be tested on its own and is.
//
// # What this package is not
//
// It is not the Directorate General of Foreign Trade and it is not Customs. It
// does not connect to the DGFT portal, it does not file a redemption
// application, and it cannot tell you what a Regional Authority will accept. It
// reads figures an exporter typed in from their own authorisation and shipping
// bills, and does arithmetic on them that is otherwise done on a spreadsheet.
//
// # Where the figures come from, and why that matters
//
// The multiples, the periods, the block structure and the interest rate are
// fixed by the Foreign Trade Policy and its Handbook of Procedures, which are
// AMENDED OFTEN — by notification, by public notice, and wholesale when a new
// Policy is notified. Every such figure is a named constant in this one file,
// carries the paragraph it comes from, and is a field of Policy so that the
// user can override it. Each is labelled INDICATIVE. Where a figure could not
// be sourced with confidence for this build, it says so in its own comment
// rather than stating a number as fact.
//
// A wrong multiple here understates a liability by lakhs. Confirm every figure
// against the current Policy and Handbook, or against the endorsement on your
// own authorisation, before relying on any number this package produces.
package eo

const (
	// PolicyEdition is the Policy these defaults are drawn from.
	//
	// The Foreign Trade Policy 2023 came into force on 1 April 2023 and has no
	// sunset date: FTP 2023 paragraph 1.01 provides that it "shall continue to
	// be in operation unless otherwise specified or amended". It has not been
	// replaced.
	PolicyEdition = "Foreign Trade Policy 2023 and Handbook of Procedures 2023"

	// PolicyEditionInForceFrom is the date that Policy came into force.
	PolicyEditionInForceFrom = "1 April 2023"

	// PolicyFiguresCompiledOn is the date the figures in this file were last
	// read against the Policy and Handbook texts the DGFT publishes, and
	// against the customs notifications that give the two schemes effect. It is
	// NOT a statement that they were in force on that date; it is the date
	// after which no amendment has been checked. Read it as the age of the
	// defaults.
	PolicyFiguresCompiledOn = "24 November 2026"
)

// ---------------------------------------------------------------------------
// EPCG — Export Promotion Capital Goods
// ---------------------------------------------------------------------------
//
// INDICATIVE. The paragraph references in this block were read against the
// consolidated FTP 2023 Chapter 5 and HBP 2023 Chapter 5 as the DGFT publishes
// them, and against Customs Notification No. 26/2023-Customs dated 1 April
// 2023, which gives the scheme effect. Read them against the current text
// before relying on them.
const (
	// EPCGMultipleHundredths is the specific export obligation as a multiple of
	// the duty saved, in hundredths: 600 is six times.
	//
	// FTP 2023 paragraph 5.01(b): import under the EPCG Scheme is subject to an
	// export obligation "equivalent to 6 times of duties, taxes and cess saved
	// on capital goods, to be fulfilled in 6 years reckoned from date of issue
	// of Authorisation". The same multiple and the same period appear in
	// Notification 26/2023-Customs, condition (4).
	//
	// Note what the multiple is OF: duties, taxes AND cess saved. FTP 5.01(e)
	// excludes IGST and Compensation Cess paid in cash from the net duty saved
	// where input tax credit has not been availed, and FTP 5.08 reckons
	// domestic sourcing on the NOTIONAL customs duty on the FOR value in the
	// invalidation letter. This tool takes the duty saved as a figure you
	// enter; it does not compute it.
	EPCGMultipleHundredths = 600

	// EPCGPeriodMonths is the period within which the specific export
	// obligation is to be fulfilled, counted from the date of issue of the
	// authorisation. Six years — FTP 2023 paragraph 5.01(b).
	EPCGPeriodMonths = 72

	// EPCGBlock1Months and EPCGBlock1CumulativePercent are the block-wise
	// milestone inside the overall period.
	//
	// HBP 2023 paragraph 5.13(a): of the period reckoned from the date of issue
	// of the authorisation, the block of the 1st to 4th year carries a minimum
	// export obligation of 50%, and the block of the 5th and 6th year the
	// balance. The same table appears in Notification 26/2023-Customs,
	// condition (4).
	//
	// HBP 5.13(b) requires the holder to intimate the Regional Authority within
	// three months of the completion of a block.
	EPCGBlock1Months            = 48
	EPCGBlock1CumulativePercent = 50

	// EPCGBlock2Months and EPCGBlock2CumulativePercent close the period: the
	// whole obligation by the end of the sixth year.
	EPCGBlock2Months            = 72
	EPCGBlock2CumulativePercent = 100

	// EPCGGreenMultipleHundredths is the reduced specific obligation for
	// authorisations covering green technology products: 75% of the obligation
	// in FTP 5.01(b), so four and a half times the duty saved.
	//
	// FTP 2023 paragraph 5.10. The list of what counts as a green technology
	// product is at HBP 2023 paragraph 5.26 — solar and wind generating systems
	// and their parts, LED lights, vapour absorption chillers, waste heat
	// boilers and recovery units, unfired heat recovery steam generators, water
	// treatment plants, battery electric vehicles, vertical farming equipment,
	// wastewater treatment and recycling, rainwater harvesting systems and
	// filters, and green hydrogen. This tool does not hold that list and will
	// not decide whether your product is on it.
	//
	// FTP 5.10 expressly leaves the AVERAGE obligation unchanged.
	EPCGGreenMultipleHundredths = 450

	// EPCGSpecifiedAreaMultipleHundredths is the reduced specific obligation
	// for units in the areas the Policy names: 25% of the obligation in FTP
	// 5.01(b), so one and a half times the duty saved.
	//
	// FTP 2023 paragraph 5.11, for units in Arunachal Pradesh, Assam, Manipur,
	// Meghalaya, Mizoram, Nagaland, Sikkim, Tripura, Jammu & Kashmir and
	// Ladakh. FTP 5.11 also leaves the average obligation unchanged.
	EPCGSpecifiedAreaMultipleHundredths = 150

	// EPCGIndigenousMultipleHundredths is the reduced specific obligation where
	// the capital goods are sourced from a domestic manufacturer: 25% LESS than
	// the obligation in FTP 5.01, so four and a half times the duty saved.
	//
	// FTP 2023 paragraph 5.04(d). The reduction is a quarter, not a tenth. FTP
	// 5.04(d) leaves the average obligation unchanged.
	EPCGIndigenousMultipleHundredths = 450

	// EPCGEarlyFulfilmentPercent and EPCGEarlyFulfilmentPeriodHalves are the
	// early-fulfilment condonation, which is worth more to a small exporter
	// than anything else in the chapter and is the least known part of it.
	//
	// FTP 2023 paragraph 5.09: where the authorisation holder has fulfilled 75%
	// or more of the specific export obligation AND 100% of the average export
	// obligation, in HALF OR LESS than half the original export obligation
	// period, the balance of the export obligation is condoned and the
	// authorisation is redeemed. Mirrored in Notification 26/2023-Customs,
	// condition (7).
	//
	// "Original" matters: the half is measured on the period as first fixed,
	// not on an extended one.
	EPCGEarlyFulfilmentPercent      = 75
	EPCGEarlyFulfilmentPeriodHalves = 2

	// EPCGInstallationCertificateMonths is the time allowed to produce a
	// certificate that the capital goods have been installed, to the REGIONAL
	// AUTHORITY: three years from the date of completion of the import.
	//
	// HBP 2023 paragraph 5.04(a). The certificate may come from jurisdictional
	// Customs or from an independent chartered engineer, at the holder's
	// option. The Regional Authority may extend the time up to the valid export
	// obligation period on payment of a composition fee.
	EPCGInstallationCertificateMonths = 36

	// EPCGInstallationCertificateCustomsMonths is the SHORTER deadline the
	// customs notification sets for the same certificate: six months from the
	// date of completion of imports, and three years for spares.
	//
	// Notification 26/2023-Customs, condition (8). Where the Regional Authority
	// grants an extension beyond six months, Customs is to extend
	// correspondingly.
	//
	// The two deadlines genuinely diverge and no instrument reconciles them, so
	// this tool reports BOTH rather than choosing. The Customs one comes first
	// and is the one that produces a demand.
	EPCGInstallationCertificateCustomsMonths = 6

	// EPCGBlockDefaultPayMonths is how long there is, after a block closes
	// short, to pay the proportionate duty.
	//
	// Notification 26/2023-Customs, condition (6): within THREE months from the
	// expiry of the block, the importer shall pay the proportionate duty
	// together with interest; condition (6) also requires evidence of
	// fulfilment to be produced within thirty days of the expiry of each block.
	//
	// HBP 2023 paragraph 5.13(c) gives SIX months for the same payment. The two
	// diverge. This tool applies the shorter, customs-side window, because that
	// is the instrument under which a demand is raised, and reports the longer
	// one as a note.
	EPCGBlockDefaultPayMonths = 3

	// EPCGBlockDefaultPayHBPMonths is the longer figure, reported alongside.
	// HBP 2023 paragraph 5.13(c).
	EPCGBlockDefaultPayHBPMonths = 6

	// EPCGEODCForm is the application for an Export Obligation Discharge
	// Certificate under the EPCG scheme. HBP 2023 paragraph 5.20(a): the holder
	// applies online in ANF 5B with the documents prescribed therein.
	//
	// Note the trap: ANF 5B is the EODC APPLICATION. Appendix 5B is a different
	// document — the chartered accountant's certificate for the ISSUE of an
	// authorisation. The certificate that goes with an EODC application is
	// Appendix 5C.
	EPCGEODCForm = "ANF 5B"
)

// ---------------------------------------------------------------------------
// Advance Authorisation
// ---------------------------------------------------------------------------
//
// INDICATIVE, on the same terms as the EPCG block above. Read against the
// consolidated FTP 2023 Chapter 4 and HBP 2023 Chapter 4, and against Customs
// Notification No. 21/2023-Customs dated 1 April 2023.
const (
	// AAPeriodMonths is the export obligation period for an Advance
	// Authorisation: eighteen months from the date of issue.
	//
	// HBP 2023 paragraph 4.40(a). The period commences from the date of issue
	// of the authorisation unless otherwise specified.
	//
	// SHORTER AND LONGER PERIODS APPLY TO CASES THE POLICY NAMES, and this tool
	// does not hold that table:
	//
	//   - HBP 4.40(b) — supplies to projects in India or abroad: co-terminus
	//     with the contracted duration, or eighteen months, whichever is more.
	//   - HBP 4.40(c) — defence, military store, aerospace and nuclear energy:
	//     twenty-four months, or co-terminus with the export order, whichever
	//     is more.
	//   - Appendix 4J — much shorter periods for particular inputs, counted
	//     FROM THE DATE OF CLEARANCE OF EACH CONSIGNMENT rather than from the
	//     date of issue: four months for precious metals in the gem and
	//     jewellery sector, three months for maize, six months for tea,
	//     coconut oil, raw sugar, wheat and most spices, twelve months for silk
	//     and for drugs from an unregistered source.
	//
	// Enter the period endorsed on your own authorisation rather than relying
	// on this default, and read Appendix 4J if your input is on it: a tool that
	// told a jeweller they had eighteen months would be wrong by fourteen.
	AAPeriodMonths = 18

	// AAPeriodDefenceMonths is the longer period for defence, military store,
	// aerospace and nuclear energy. HBP 2023 paragraph 4.40(c).
	AAPeriodDefenceMonths = 24

	// AAMinValueAdditionPercent is the minimum value addition an Advance
	// Authorisation must achieve: fifteen percent.
	//
	// FTP 2023 paragraph 4.09(i). The formula is FTP 2023 paragraph 4.08:
	//
	//	VA = ((A − B) ÷ B) × 100
	//
	// where A is the FOB value of the export realised (or the FOR value of the
	// supply received) and B is the CIF value of the inputs covered by the
	// authorisation PLUS the value of any other input used on which drawback is
	// claimed or intended to be claimed.
	//
	// DIFFERENT MINIMA APPLY to cases the Policy names, and this tool applies
	// one figure:
	//
	//   - FTP 4.09(iv) — tea: fifty percent.
	//   - FTP 4.09(v) — spices: twenty-five percent.
	//   - FTP 4.09(ii) and Appendix 4D — eight percent for copper from
	//     concentrate and for certain petroleum products.
	//   - FTP 4.09(iii) and HBP 4.60 — gems and jewellery, from 3.5% for plain
	//     gold jewellery to 15% for studded religious idols.
	//
	// If your product is one of those, change the figure. The tool will say it
	// is your figure.
	AAMinValueAdditionPercent = 15

	// AAEODCFilingMonths is the deadline to apply for redemption of an Advance
	// Authorisation: six months from the date of expiry of the export
	// obligation period.
	//
	// HBP 2023 paragraph 4.43(b): within six months of the expiry of the export
	// obligation period the holder shall file the application online by linking
	// the shipping bills against the authorisation, "failing which Regional
	// Authority may initiate action as per FT (D&R) Act by way of issuing Show
	// Cause Notice". Notification 21/2023-Customs, condition (xi), separately
	// requires evidence of discharge to be produced to Customs within sixty
	// days of the expiry of the period allowed.
	//
	// There is no equivalent deadline for EPCG redemption: HBP 5.20 prescribes
	// none.
	AAEODCFilingMonths = 6

	// AAEODCForm is the application for redemption of an Advance Authorisation.
	// HBP 2023 paragraphs 4.46 and 4.47(b)(i): the holder files online in
	// ANF 4F and uploads the prescribed documents.
	//
	// Note the trap: the APPLICATION for an Advance Authorisation is ANF 4A in
	// every case, whether on notified norms, ad hoc norms or self-declaration
	// (HBP 4.04). ANF 4B is the application to the Norms Committee for fixation
	// of norms (HBP 4.06, 4.22), not a self-declaration form.
	AAEODCForm = "ANF 4F"

	// AAValueShortfallFeePercent is what is payable where the obligation was
	// met in quantity but fell short in value AND the minimum value addition
	// was not achieved: one percent of the shortfall in FOB value, in rupees.
	//
	// HBP 2023 paragraph 4.49(b). Where the minimum value addition WAS
	// achieved, 4.49(b) imposes no penalty for a value shortfall at all.
	//
	// This tool does not compute it: it reports that the provision exists and
	// what it is measured on. The quantity-and-value interaction in HBP 4.49 is
	// pro-rata arithmetic on figures a Regional Authority verifies against
	// Appendix 4H, and guessing at it would be worse than naming it.
	AAValueShortfallFeePercent = 1
)

// ---------------------------------------------------------------------------
// What becomes payable when an obligation is not discharged
// ---------------------------------------------------------------------------
const (
	// DefaultInterestRatePercent is the rate of interest payable, per annum, on
	// the duty that becomes payable when an export obligation is not
	// discharged: fifteen percent.
	//
	// The rate is written into the customs notifications that give the schemes
	// effect, and runs FROM THE DATE OF CLEARANCE OF THE GOODS:
	//
	//   - EPCG — Notification No. 26/2023-Customs dated 1 April 2023,
	//     condition (6): the importer shall pay duties of customs bearing the
	//     same proportion to the duties leviable but for the exemption as the
	//     unfulfilled portion of the export obligation bears to the total,
	//     "together with interest at the rate of fifteen per cent. per annum
	//     from the date of clearance of the goods".
	//   - Advance Authorisation — Notification No. 21/2023-Customs dated
	//     1 April 2023, condition (iv), in the same terms, from the date of
	//     clearance of the materials.
	//
	// The statutory rate under section 28AA of the Customs Act 1962 is
	// separately fixed at fifteen percent by Notification No. 33/2016-Customs
	// (N.T.) dated 1 March 2016, which is the rate that applies where the
	// department proceeds by show cause notice instead. The two coincide today
	// and are legally independent, so a change to one would not change the
	// other.
	//
	// The rate that applies to YOUR goods depends on the notification quoted on
	// your own bill of entry. An authorisation issued before 1 April 2023 is
	// governed by a different notification, whose conditions this build has not
	// verified. Check the bill of entry.
	DefaultInterestRatePercent = 15

	// DefaultInterestDayBasis is the number of days in a year used to pro-rate
	// interest, and the day count is calendar days.
	//
	// A CONVENTION OF THIS TOOL. The notifications fix the rate and the start
	// date and say nothing about the arithmetic. See InterestIsSimple for the
	// larger gap.
	DefaultInterestDayBasis = 365

	// InterestIsSimple is how this tool computes the interest, and it is a
	// DECISION, not a sourced figure. It is named here so that it is as visible
	// as the rate it applies to.
	//
	// Neither notification, nor section 28AA, nor Notification 33/2016-Customs
	// (N.T.), nor any Central Board of Indirect Taxes and Customs circular or
	// reported decision that could be found for this build states in terms
	// whether this interest is simple or compound. None of those texts contains
	// any compounding language — no rests, no capitalisation, no periodic
	// conversion — and simple interest is what practitioners describe. That is
	// a negative finding, not an authority.
	//
	// This tool therefore computes SIMPLE interest and says so wherever it
	// shows a figure. If the department computes it otherwise, the figure here
	// is low. Treat the exposure as an order of magnitude and a deadline, never
	// as the demand.
	InterestIsSimple = true

	// MinorShortfallCondonationPercent is a shortfall this tool will treat as
	// condoned.
	//
	// The default is ZERO: this tool condones nothing on its own.
	//
	// The condonation the Policy does provide is not a percentage of the
	// shortfall — it is FTP 2023 paragraph 5.09, which condones the WHOLE
	// balance where 75% of the specific obligation and 100% of the average
	// obligation were achieved in half the original period. That is computed
	// separately and reported as eligibility, because a Regional Authority
	// grants it; see EPCGEarlyFulfilmentPercent.
	//
	// This field exists for a figure your own Regional Authority has told you
	// applies. Whatever you set it to is labelled as your figure.
	MinorShortfallCondonationPercent = 0
)

// ---------------------------------------------------------------------------
// Conventions of this tool, named here so that they are as visible as the
// cited figures and are never mistaken for them
// ---------------------------------------------------------------------------
const (
	// FinancialYearStartMonth and FinancialYearStartDay are the beginning of
	// the licensing year: 1 April.
	//
	// This one is not a convention. A licensing year under the Foreign Trade
	// (Development and Regulation) framework runs from 1 April to the following
	// 31 March, and FTP 2023 paragraph 5.04(c) requires the average export
	// obligation to be fulfilled in every financial year until the export
	// obligation is completed. The obligation period and its blocks, by
	// contrast, are counted from the date of issue of the authorisation, so the
	// two clocks do not turn over together.
	FinancialYearStartMonth = 4
	FinancialYearStartDay   = 1

	// PeriodEndsDayBeforeAnniversary decides how "six years from the date of
	// issue" is turned into a last day.
	//
	// Two readings are in circulation. On one, a six-year period from 15 May
	// 2023 runs to 15 May 2029 — the anniversary itself. On the other it runs
	// to 14 May 2029 — the day before, which makes the period exactly six years
	// long. Neither the Policy nor the Handbook resolves it, and the
	// authorisation prints the date.
	//
	// This tool defaults to the EARLIER reading, for one reason: being wrong by
	// a day in the direction of urgency makes an exporter ship a day early, and
	// being wrong the other way makes them miss a deadline. It is a CONVENTION,
	// it is labelled as derived wherever it is used, and it is overridden the
	// moment you enter the end date endorsed on your own authorisation — which
	// you should.
	PeriodEndsDayBeforeAnniversary = true

	// AveragePartYearProRata decides how the annual average export obligation
	// is attributed to a PART year — the first and last licensing years of an
	// obligation period, and the year a block boundary falls in.
	//
	// A CONVENTION OF THIS TOOL. FTP 5.04(c) requires the average to be
	// fulfilled in every financial year and does not say what happens to a
	// financial year only part of which falls inside the period. With this
	// convention in force, a licensing year of which 73 days fall inside the
	// period carries 73/365 of the annual average. Without it, that sliver
	// carries a full year's average, which is harsher than any reading of the
	// Policy supports; attributing nothing at all would be more generous than
	// it supports. Whichever is in force is stated wherever an average figure
	// is shown.
	AveragePartYearProRata = true

	// DefaultCurrency is the currency a licence is assumed to be denominated in
	// when none is given. It is a LABEL only: this tool applies no exchange
	// rate and converts nothing, because the only honest rate is the one on the
	// exporter's own documents.
	DefaultCurrency = "INR"

	// WarnWithinDays is how far ahead a deadline starts being reported as
	// approaching, and UrgentWithinDays how close it has to be to be reported
	// as urgent. Conventions, chosen so that a quarter's notice is given on a
	// block that takes years to fill.
	WarnWithinDays   = 180
	UrgentWithinDays = 60

	// MaxImportsPerLicence, MaxShipmentsPerLicence and MaxExtensionsPerLicence
	// cap what one licence holds, so that a mistaken paste cannot exhaust
	// memory. They are limits of this tool, not of the schemes.
	MaxImportsPerLicence    = 500
	MaxShipmentsPerLicence  = 5000
	MaxExtensionsPerLicence = 20
)

// BlockRule is one block-wise milestone: the end of the block, as a number of
// months from the date of issue of the authorisation, and the cumulative
// percentage of the specific export obligation that must be discharged by then.
//
// Cumulative, not incremental, because that is the question an exporter needs
// answered — "how much must be done by this date" — and because it makes a
// policy with a different number of blocks a change of data rather than a
// change of code.
type BlockRule struct {
	No                int `json:"no"`
	EndsAfterMonths   int `json:"endsAfterMonths"`
	CumulativePercent int `json:"cumulativePercent"`
}

// AverageYearBasis is which clock the average export obligation is measured
// against.
type AverageYearBasis string

const (
	// AverageByLicensingYear measures the average against the licensing year —
	// 1 April to 31 March. This is the default and is what FTP 2023 paragraph
	// 5.04(c) requires: the average is fixed on the preceding three LICENSING
	// years and is to be fulfilled in every FINANCIAL year until the obligation
	// is completed.
	AverageByLicensingYear AverageYearBasis = "licensing-year"

	// AverageByAuthorisationYear measures it against anniversary years of the
	// authorisation instead. It is offered for an exporter who has been told
	// their licence is computed that way; it is not what the Policy says.
	AverageByAuthorisationYear AverageYearBasis = "authorisation-year"
)

// Title names the basis for the screen.
func (b AverageYearBasis) Title() string {
	switch b {
	case AverageByAuthorisationYear:
		return "Anniversary years of the authorisation"
	default:
		return "Licensing years (1 April to 31 March)"
	}
}

// Policy is every figure above in a form the user can change.
//
// It belongs to a LICENCE and not to the tool. An authorisation issued under
// one edition of the Policy continues to be governed by the figures that applied
// to it after the Policy is amended, so storing the figures on the licence is
// the only arrangement that stays correct over time. A zero field means "use the
// figure as published", so an empty Policy behaves exactly like the constants.
type Policy struct {
	// Edition is a free-text note of which Policy these figures are from. It is
	// printed on the report so that a figure can never be read without its
	// edition.
	Edition string `json:"edition"`

	// EPCG
	EPCGMultipleHundredths              int         `json:"epcgMultipleHundredths"`
	EPCGGreenMultipleHundredths         int         `json:"epcgGreenMultipleHundredths"`
	EPCGSpecifiedAreaMultipleHundredths int         `json:"epcgSpecifiedAreaMultipleHundredths"`
	EPCGIndigenousMultipleHundredths    int         `json:"epcgIndigenousMultipleHundredths"`
	EPCGPeriodMonths                    int         `json:"epcgPeriodMonths"`
	Blocks                              []BlockRule `json:"blocks"`

	// EarlyFulfilmentPercent is the proportion of the specific obligation that,
	// together with the whole average obligation, earns condonation of the
	// balance when achieved in half the original period (FTP 5.09).
	EarlyFulfilmentPercent int `json:"earlyFulfilmentPercent"`

	// InstallationCertificateMonths is the Regional Authority's deadline and
	// InstallationCertificateCustomsMonths the customs one. They diverge; both
	// are reported.
	InstallationCertificateMonths        int `json:"installationCertificateMonths"`
	InstallationCertificateCustomsMonths int `json:"installationCertificateCustomsMonths"`

	// BlockDefaultPayMonths is how long there is to pay after a block closes
	// short.
	BlockDefaultPayMonths int `json:"blockDefaultPayMonths"`

	// Advance Authorisation
	AAPeriodMonths            int `json:"aaPeriodMonths"`
	AAMinValueAdditionPercent int `json:"aaMinValueAdditionPercent"`
	AAEODCFilingMonths        int `json:"aaEodcFilingMonths"`

	// Default
	InterestRatePercent int `json:"interestRatePercent"`
	InterestDayBasis    int `json:"interestDayBasis"`

	// CondonationPercent is a shortfall to treat as condoned. It defaults to
	// zero: this tool condones nothing unless told to.
	CondonationPercent int `json:"condonationPercent"`

	// AverageBasis is which year the average obligation is measured against.
	AverageBasis AverageYearBasis `json:"averageBasis"`

	// EndsDayBeforeAnniversary is the period-end convention, and
	// PartYearProRata the part-year convention for the average obligation.
	// Both are pointers because false is a deliberate choice and an absent
	// field is not, and the two must not be confused.
	EndsDayBeforeAnniversary *bool `json:"endsDayBeforeAnniversary"`
	PartYearProRata          *bool `json:"partYearProRata"`

	// Edited records that at least one figure has been moved off the published
	// one, so that a report can say so plainly.
	Edited bool `json:"edited"`
}

// AsPublished returns the figures exactly as the constants above state them.
func AsPublished() Policy {
	ends := PeriodEndsDayBeforeAnniversary
	prorata := AveragePartYearProRata
	return Policy{
		Edition:                             PolicyEdition,
		EPCGMultipleHundredths:              EPCGMultipleHundredths,
		EPCGGreenMultipleHundredths:         EPCGGreenMultipleHundredths,
		EPCGSpecifiedAreaMultipleHundredths: EPCGSpecifiedAreaMultipleHundredths,
		EPCGIndigenousMultipleHundredths:    EPCGIndigenousMultipleHundredths,
		EPCGPeriodMonths:                    EPCGPeriodMonths,
		Blocks: []BlockRule{
			{No: 1, EndsAfterMonths: EPCGBlock1Months, CumulativePercent: EPCGBlock1CumulativePercent},
			{No: 2, EndsAfterMonths: EPCGBlock2Months, CumulativePercent: EPCGBlock2CumulativePercent},
		},
		EarlyFulfilmentPercent:               EPCGEarlyFulfilmentPercent,
		InstallationCertificateMonths:        EPCGInstallationCertificateMonths,
		InstallationCertificateCustomsMonths: EPCGInstallationCertificateCustomsMonths,
		BlockDefaultPayMonths:                EPCGBlockDefaultPayMonths,
		AAPeriodMonths:                       AAPeriodMonths,
		AAMinValueAdditionPercent:            AAMinValueAdditionPercent,
		AAEODCFilingMonths:                   AAEODCFilingMonths,
		InterestRatePercent:                  DefaultInterestRatePercent,
		InterestDayBasis:                     DefaultInterestDayBasis,
		CondonationPercent:                   MinorShortfallCondonationPercent,
		AverageBasis:                         AverageByLicensingYear,
		EndsDayBeforeAnniversary:             &ends,
		PartYearProRata:                      &prorata,
	}
}

// Normalise fills in anything left blank with the figure as published and
// brings anything impossible back into range. It never silently keeps a
// negative rate or a zero obligation period.
//
// Zero means "not entered" for every rate and period here, not "nil". A zero
// interest rate would report an exposure of duty with no interest on it, and a
// zero minimum value addition would report a requirement as met that is not —
// and a liability reported too low is the dangerous direction to be wrong in.
// None of these figures is ever zero under either scheme.
func (p Policy) Normalise() Policy {
	d := AsPublished()

	if p.Edition == "" {
		p.Edition = d.Edition
	}
	if p.EPCGMultipleHundredths <= 0 {
		p.EPCGMultipleHundredths = d.EPCGMultipleHundredths
	}
	if p.EPCGGreenMultipleHundredths <= 0 {
		p.EPCGGreenMultipleHundredths = d.EPCGGreenMultipleHundredths
	}
	if p.EPCGSpecifiedAreaMultipleHundredths <= 0 {
		p.EPCGSpecifiedAreaMultipleHundredths = d.EPCGSpecifiedAreaMultipleHundredths
	}
	if p.EPCGIndigenousMultipleHundredths <= 0 {
		p.EPCGIndigenousMultipleHundredths = d.EPCGIndigenousMultipleHundredths
	}
	if p.EPCGPeriodMonths <= 0 {
		p.EPCGPeriodMonths = d.EPCGPeriodMonths
	}
	if p.EarlyFulfilmentPercent <= 0 || p.EarlyFulfilmentPercent > 100 {
		p.EarlyFulfilmentPercent = d.EarlyFulfilmentPercent
	}
	if p.InstallationCertificateMonths <= 0 {
		p.InstallationCertificateMonths = d.InstallationCertificateMonths
	}
	if p.InstallationCertificateCustomsMonths <= 0 {
		p.InstallationCertificateCustomsMonths = d.InstallationCertificateCustomsMonths
	}
	if p.BlockDefaultPayMonths <= 0 {
		p.BlockDefaultPayMonths = d.BlockDefaultPayMonths
	}
	if p.AAPeriodMonths <= 0 {
		p.AAPeriodMonths = d.AAPeriodMonths
	}
	if p.AAMinValueAdditionPercent <= 0 {
		p.AAMinValueAdditionPercent = d.AAMinValueAdditionPercent
	}
	if p.AAEODCFilingMonths <= 0 {
		p.AAEODCFilingMonths = d.AAEODCFilingMonths
	}
	if p.InterestRatePercent <= 0 {
		p.InterestRatePercent = d.InterestRatePercent
	}
	if p.InterestDayBasis <= 0 {
		p.InterestDayBasis = d.InterestDayBasis
	}
	if p.CondonationPercent < 0 || p.CondonationPercent > 100 {
		p.CondonationPercent = d.CondonationPercent
	}
	if p.AverageBasis != AverageByAuthorisationYear {
		p.AverageBasis = AverageByLicensingYear
	}
	if p.EndsDayBeforeAnniversary == nil {
		p.EndsDayBeforeAnniversary = d.EndsDayBeforeAnniversary
	}
	if p.PartYearProRata == nil {
		p.PartYearProRata = d.PartYearProRata
	}
	p.Blocks = normaliseBlocks(p.Blocks, p.EPCGPeriodMonths, d.Blocks)
	p.Edited = p.differsFromPublished()
	return p
}

// normaliseBlocks keeps the blocks that make sense, in order, and falls back to
// the published structure when nothing usable is left.
func normaliseBlocks(in []BlockRule, periodMonths int, fallback []BlockRule) []BlockRule {
	out := make([]BlockRule, 0, len(in))
	for _, b := range in {
		if b.EndsAfterMonths <= 0 || b.CumulativePercent <= 0 || b.CumulativePercent > 100 {
			continue
		}
		if b.EndsAfterMonths > periodMonths {
			continue
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		out = append(out, fallback...)
		// The fallback is written for the published period; if the user has
		// shortened the period, drop the blocks that now fall outside it.
		trimmed := out[:0]
		for _, b := range out {
			if b.EndsAfterMonths <= periodMonths {
				trimmed = append(trimmed, b)
			}
		}
		out = trimmed
	}

	// In order of time, de-duplicated on the end month, cumulative percentages
	// non-decreasing.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].EndsAfterMonths < out[j-1].EndsAfterMonths; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	deduped := make([]BlockRule, 0, len(out))
	for _, b := range out {
		if n := len(deduped); n > 0 && deduped[n-1].EndsAfterMonths == b.EndsAfterMonths {
			if b.CumulativePercent > deduped[n-1].CumulativePercent {
				deduped[n-1].CumulativePercent = b.CumulativePercent
			}
			continue
		}
		deduped = append(deduped, b)
	}
	out = deduped
	for i := 1; i < len(out); i++ {
		if out[i].CumulativePercent < out[i-1].CumulativePercent {
			out[i].CumulativePercent = out[i-1].CumulativePercent
		}
	}

	// The period must close on the whole obligation. If the last block already
	// ends with the period, it is the closing one; otherwise a closing block is
	// ADDED rather than the last milestone being moved, which would quietly
	// delete a milestone the user asked for.
	if n := len(out); n > 0 && out[n-1].EndsAfterMonths == periodMonths {
		out[n-1].CumulativePercent = 100
	} else {
		out = append(out, BlockRule{EndsAfterMonths: periodMonths, CumulativePercent: 100})
	}
	for i := range out {
		out[i].No = i + 1
	}
	return out
}

// differsFromPublished reports whether any figure has been moved.
func (p Policy) differsFromPublished() bool {
	d := AsPublished()
	if p.EPCGMultipleHundredths != d.EPCGMultipleHundredths ||
		p.EPCGGreenMultipleHundredths != d.EPCGGreenMultipleHundredths ||
		p.EPCGSpecifiedAreaMultipleHundredths != d.EPCGSpecifiedAreaMultipleHundredths ||
		p.EPCGIndigenousMultipleHundredths != d.EPCGIndigenousMultipleHundredths ||
		p.EPCGPeriodMonths != d.EPCGPeriodMonths ||
		p.EarlyFulfilmentPercent != d.EarlyFulfilmentPercent ||
		p.InstallationCertificateMonths != d.InstallationCertificateMonths ||
		p.InstallationCertificateCustomsMonths != d.InstallationCertificateCustomsMonths ||
		p.BlockDefaultPayMonths != d.BlockDefaultPayMonths ||
		p.AAPeriodMonths != d.AAPeriodMonths ||
		p.AAMinValueAdditionPercent != d.AAMinValueAdditionPercent ||
		p.AAEODCFilingMonths != d.AAEODCFilingMonths ||
		p.InterestRatePercent != d.InterestRatePercent ||
		p.InterestDayBasis != d.InterestDayBasis ||
		p.CondonationPercent != d.CondonationPercent ||
		p.AverageBasis != d.AverageBasis {
		return true
	}
	if (p.EndsDayBeforeAnniversary == nil) != (d.EndsDayBeforeAnniversary == nil) {
		return true
	}
	if p.EndsDayBeforeAnniversary != nil && d.EndsDayBeforeAnniversary != nil &&
		*p.EndsDayBeforeAnniversary != *d.EndsDayBeforeAnniversary {
		return true
	}
	if (p.PartYearProRata == nil) != (d.PartYearProRata == nil) {
		return true
	}
	if p.PartYearProRata != nil && d.PartYearProRata != nil &&
		*p.PartYearProRata != *d.PartYearProRata {
		return true
	}
	if len(p.Blocks) != len(d.Blocks) {
		return true
	}
	for i := range p.Blocks {
		if p.Blocks[i].EndsAfterMonths != d.Blocks[i].EndsAfterMonths ||
			p.Blocks[i].CumulativePercent != d.Blocks[i].CumulativePercent {
			return true
		}
	}
	return false
}

// EndsDayBefore reports the period-end convention in force.
func (p Policy) EndsDayBefore() bool {
	if p.EndsDayBeforeAnniversary == nil {
		return PeriodEndsDayBeforeAnniversary
	}
	return *p.EndsDayBeforeAnniversary
}

// ProRataPartYears reports the part-year convention in force for the average
// export obligation.
func (p Policy) ProRataPartYears() bool {
	if p.PartYearProRata == nil {
		return AveragePartYearProRata
	}
	return *p.PartYearProRata
}

// InterestRate is the rate of interest on a duty that becomes payable.
func (p Policy) InterestRate() Bips { return Percent(int64(p.InterestRatePercent), 0) }

// MinValueAddition is the minimum value addition for an Advance Authorisation.
func (p Policy) MinValueAddition() Bips { return Percent(int64(p.AAMinValueAdditionPercent), 0) }

// EODCForm names the redemption application for a scheme.
func (p Policy) EODCForm(s Scheme) string {
	if s == SchemeAdvanceAuthorisation {
		return AAEODCForm
	}
	return EPCGEODCForm
}

// MultipleFor returns the obligation multiple that applies to an EPCG
// authorisation of a given category, and whether a multiple applies at all —
// it does not where the obligation is taken from the endorsement.
func (p Policy) MultipleFor(c Category) (Bips, bool) {
	p = p.Normalise()
	switch c {
	case CategoryGreenTechnology:
		return Times(int64(p.EPCGGreenMultipleHundredths)), true
	case CategorySpecifiedArea:
		return Times(int64(p.EPCGSpecifiedAreaMultipleHundredths)), true
	case CategoryIndigenousSourcing:
		return Times(int64(p.EPCGIndigenousMultipleHundredths)), true
	case CategoryAsEndorsed:
		return 0, false
	default:
		return Times(int64(p.EPCGMultipleHundredths)), true
	}
}

// PeriodMonths returns the obligation period for a scheme.
func (p Policy) PeriodMonths(s Scheme) int {
	p = p.Normalise()
	if s == SchemeAdvanceAuthorisation {
		return p.AAPeriodMonths
	}
	return p.EPCGPeriodMonths
}

// Citation records where a figure comes from and how much weight to put on it.
// Confidence is stated because some of these figures were verifiable against
// the published texts for this build and some were not, and a reader deserves
// to know which is which.
type Citation struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Figure     string `json:"figure"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
	Note       string `json:"note"`
	Editable   bool   `json:"editable"`
}

// Confidence levels used below.
const (
	confidenceSourced   = "indicative — read against the published text"
	confidenceUnsourced = "indicative — not verified for this build"
	confidenceDecision  = "a decision of this tool, not a sourced figure"
	confidenceTool      = "a convention of this tool, not the Policy"
	confidenceCertain   = "fixed by the framework, not edition-specific"
)

// Citations lists every figure the tool applies, with its attribution, in the
// order a reader wants them. The interface and the written report both read
// this list rather than restating a figure anywhere else, so there is exactly
// one place a number lives.
func (p Policy) Citations() []Citation {
	p = p.Normalise()
	blocks := ""
	for i, b := range p.Blocks {
		if i > 0 {
			blocks += "; "
		}
		blocks += "block " + itoa(b.No) + " to month " + itoa(b.EndsAfterMonths) +
			": " + itoa(b.CumulativePercent) + "% cumulative"
	}
	return []Citation{
		{"edition", "Policy edition the figures are from", p.Edition,
			"In force from " + PolicyEditionInForceFrom + "; figures last read on " + PolicyFiguresCompiledOn,
			confidenceTool,
			"FTP 2023 para 1.01 gives the Policy no sunset date — it runs until amended or replaced — and it is amended often, by notification and by public notice. A licence is governed by the figures that applied to it, which is why the edition is kept on the licence and not on the tool.", true},

		{"epcgMultiple", "EPCG specific export obligation", Times(int64(p.EPCGMultipleHundredths)).Multiple() + " the duty saved",
			"FTP 2023 para 5.01(b); Notification 26/2023-Customs cond. (4)", confidenceSourced,
			"Six times the duties, TAXES AND CESS saved. FTP 5.01(e) excludes IGST and Compensation Cess paid in cash where input tax credit was not availed, and FTP 5.08 reckons domestic sourcing on notional duty. This tool takes the duty saved as a figure you enter; the obligation endorsed on the authorisation governs over any multiple.", true},

		{"epcgPeriod", "EPCG obligation period", itoa(p.EPCGPeriodMonths) + " months from the date of issue",
			"FTP 2023 para 5.01(b)", confidenceSourced,
			"Reckoned from the date of issue of the authorisation, not from the date of import. Separately, FTP 5.01(d) gives 24 months to import and permits no revalidation.", true},

		{"epcgBlocks", "EPCG block-wise milestones", blocks,
			"HBP 2023 para 5.13(a); Notification 26/2023-Customs cond. (4)", confidenceSourced,
			"50% of the specific obligation in the block of the 1st to 4th year, the balance in the block of the 5th and 6th. HBP 5.13(b) requires the Regional Authority to be told within three months of a block completing.", true},

		{"blockDefaultPay", "Time to pay after a block closes short", plural(p.BlockDefaultPayMonths, "month", "months") + " from the expiry of the block",
			"Notification 26/2023-Customs cond. (6)", confidenceSourced,
			"The customs notification gives three months to pay the proportionate duty with interest, and thirty days to produce evidence of fulfilment. HBP 5.13(c) gives six months for the same payment. The two diverge and nothing reconciles them; the shorter, customs-side window is applied here because that is the instrument a demand comes under.", true},

		{"epcgGreen", "EPCG obligation, green technology product", Times(int64(p.EPCGGreenMultipleHundredths)).Multiple() + " the duty saved",
			"FTP 2023 para 5.10; list at HBP 2023 para 5.26", confidenceSourced,
			"75% of the ordinary obligation. The list at HBP 5.26 runs to thirteen items — solar and wind systems, LED lights, vapour absorption chillers, waste heat recovery, water and wastewater treatment, battery electric vehicles, vertical farming, rainwater harvesting and green hydrogen. This tool does not hold the list. The AVERAGE obligation is unchanged by this reduction.", true},

		{"epcgArea", "EPCG obligation, unit in a specified area", Times(int64(p.EPCGSpecifiedAreaMultipleHundredths)).Multiple() + " the duty saved",
			"FTP 2023 para 5.11", confidenceSourced,
			"25% of the ordinary obligation, for units in Arunachal Pradesh, Assam, Manipur, Meghalaya, Mizoram, Nagaland, Sikkim, Tripura, Jammu & Kashmir and Ladakh. The average obligation is unchanged by this reduction.", true},

		{"epcgIndigenous", "EPCG obligation, capital goods sourced indigenously", Times(int64(p.EPCGIndigenousMultipleHundredths)).Multiple() + " the duty saved",
			"FTP 2023 para 5.04(d)", confidenceSourced,
			"25% LESS than the ordinary obligation — a quarter, not a tenth. The average obligation is unchanged by this reduction.", true},

		{"mutuallyExclusive", "Only one concession may be taken", "the reductions in paras 5.04(d), 5.09, 5.10 and 5.11 are mutually exclusive",
			"FTP 2023 para 5.04(l); Notification 26/2023-Customs proviso to cond. (7)", confidenceSourced,
			"Indigenous sourcing, early-fulfilment condonation, the green technology reduction and the specified-area reduction cannot be combined. This tool applies one category at a time and will not stack them.", false},

		{"earlyFulfilment", "Early fulfilment condones the balance", itoa(p.EarlyFulfilmentPercent) + "% of the specific obligation and 100% of the average, in half the original period",
			"FTP 2023 para 5.09; Notification 26/2023-Customs cond. (7)", confidenceSourced,
			"The most valuable provision in the chapter and the least used. Achieve it and the balance of the obligation is condoned and the authorisation redeemed. The half is measured on the ORIGINAL period, not an extended one, and it cannot be combined with the reductions in 5.04(d), 5.10 or 5.11.", true},

		{"installationRA", "Installation certificate — Regional Authority", "within " + plural(p.InstallationCertificateMonths, "month", "months") + " of completing the import",
			"HBP 2023 para 5.04(a)", confidenceSourced,
			"Three years, from jurisdictional Customs or an independent chartered engineer at your option, extendable by the Regional Authority up to the valid obligation period on a composition fee.", true},

		{"installationCustoms", "Installation certificate — Customs", "within " + plural(p.InstallationCertificateCustomsMonths, "month", "months") + " of completing the import",
			"Notification 26/2023-Customs cond. (8)", confidenceSourced,
			"Six months, and three years for spares. This is SHORTER than the Regional Authority's three years and nothing reconciles the two, so both are shown. Where the Regional Authority extends beyond six months, Customs is to extend correspondingly.", true},

		{"epcgEodcForm", "EPCG redemption application", EPCGEODCForm + ", online, with no deadline prescribed",
			"HBP 2023 para 5.20(a)", confidenceSourced,
			"HBP 5.20 sets no deadline for applying, unlike the Advance Authorisation. What is time-bound is the evidence: thirty days from the expiry of each block under Notification 26/2023-Customs cond. (6), and the online fulfilment report under HBP 5.14. Do not confuse ANF 5B, the application, with Appendix 5B, a certificate for the ISSUE of an authorisation; the certificate for redemption is Appendix 5C.", false},

		{"aaPeriod", "Advance Authorisation obligation period", plural(p.AAPeriodMonths, "month", "months") + " from the date of issue",
			"HBP 2023 para 4.40(a)", confidenceSourced,
			"MUCH SHORTER PERIODS apply to the inputs in Appendix 4J, and they run from the date each consignment is CLEARED, not from the date of issue: four months for precious metals in gem and jewellery, three for maize, six for tea, coconut oil, raw sugar, wheat and most spices. Defence, military store, aerospace and nuclear energy get 24 months under HBP 4.40(c). Enter the period endorsed on your own authorisation.", true},

		{"aaValueAddition", "Advance Authorisation minimum value addition", itoa(p.AAMinValueAdditionPercent) + "%",
			"FTP 2023 para 4.09(i); formula at FTP 2023 para 4.08", confidenceSourced,
			"VA = ((A − B) ÷ B) × 100, A being the FOB value of the export REALISED and B the CIF value of the inputs plus any other imported input on which drawback is claimed. Different minima apply: tea 50% (4.09(iv)), spices 25% (4.09(v)), Appendix 4D items 8%, gems and jewellery 3.5% to 15% (HBP 4.60). Change the figure if your product is one of those.", true},

		{"aaEodc", "Advance Authorisation redemption application", AAEODCForm + ", within " + plural(p.AAEODCFilingMonths, "month", "months") + " of the obligation period expiring",
			"HBP 2023 paras 4.46, 4.47(b)(i) and 4.43(b)", confidenceSourced,
			"HBP 4.43(b) warns that failure to file within six months may draw a show cause notice under the Foreign Trade (Development and Regulation) Act. Notification 21/2023-Customs cond. (xi) separately requires evidence of discharge to Customs within sixty days. The APPLICATION for an authorisation is ANF 4A in every case; ANF 4B is the norms-fixation application, not a self-declaration form.", true},

		{"aaValueShortfall", "Advance Authorisation value shortfall", itoa(AAValueShortfallFeePercent) + "% of the shortfall in FOB value",
			"HBP 2023 para 4.49(b)", confidenceSourced,
			"Payable only where the minimum value addition was NOT achieved; where it was, HBP 4.49(b) imposes nothing for a value shortfall. HBP 4.49(a) separately requires duty with interest on unutilised material, plus 10% of its CIF value where the input was restricted. This tool reports that these provisions exist; it does not compute them, because the pro-rata arithmetic in HBP 4.49 runs on figures a Regional Authority verifies against Appendix 4H.", false},

		{"interest", "Interest on duty that becomes payable", itoa(p.InterestRatePercent) + "% per annum, from the date the goods were cleared",
			"Notification 26/2023-Customs cond. (6) for EPCG; Notification 21/2023-Customs cond. (iv) for Advance Authorisation", confidenceSourced,
			"The rate is written into the notification itself, and runs from the DATE OF CLEARANCE — later than the date of import, far earlier than the date of payment. Section 28AA of the Customs Act is separately fixed at 15% by Notification 33/2016-Customs (N.T.), which applies where the department proceeds by show cause notice. An authorisation issued before 1 April 2023 is governed by a different notification, which this build has not read: check the one quoted on your own bill of entry.", true},

		{"interestSimple", "Interest computed as", map[bool]string{true: "simple interest", false: "compound interest"}[InterestIsSimple],
			"Not stated in either notification, in section 28AA, or in any circular found", confidenceDecision,
			"NOTHING FOUND FOR THIS BUILD SAYS WHETHER THIS INTEREST IS SIMPLE OR COMPOUND. None of the texts contains compounding language, and simple interest is what practitioners describe — a negative finding, not an authority. This tool computes simple interest. If the department computes it otherwise, the figure here is low.", false},

		{"interestBasis", "Interest day-count basis", itoa(p.InterestDayBasis) + " days a year, in calendar days from clearance to the day computed for",
			"Not from the Policy", confidenceTool,
			"The notifications fix the rate and the start date and say nothing about the arithmetic or the end date. HBP 4.50(e) implies the rate applicable on the date of payment, so in practice interest runs to payment. Treat the figure as an order of magnitude and a deadline, not as the demand.", true},

		{"interestOnWhat", "What the interest is applied to", "the whole duty saved you enter",
			"Unresolved — see the note", confidenceDecision,
			"THIS MAY OVERSTATE THE INTEREST. There is a developed line of authority that interest cannot be levied on the additional and special additional customs duty components absent an express charging provision, and CBIC Circular 11/2023-Customs said no interest was payable on those parts under the 2023 settlement scheme. This tool applies the rate to whatever duty-saved figure you enter. If part of yours is additional duty, special additional duty or IGST, take advice before treating the figure as the liability.", false},

		{"condonation", "Shortfall treated as condoned", itoa(p.CondonationPercent) + "%",
			"Nothing in the Policy condones a percentage of a shortfall", confidenceTool,
			"The default is zero: this tool condones nothing on its own. The condonation the Policy does provide is FTP 5.09, which condones the WHOLE balance on early fulfilment and is computed separately. Set this only to a figure your own Regional Authority has told you applies; whatever you set is labelled as your figure.", true},

		{"average", "Average export obligation", "maintained over and above, in every financial year",
			"FTP 2023 para 5.04(c)", confidenceSourced,
			"The arithmetic mean of exports of the SAME AND SIMILAR products in the preceding three licensing years, to be maintained in every financial year until the obligation is completed. Only exports over and above it count toward the specific obligation. Thirteen sectors are exempt under FTP 5.12(a) — handicrafts, handlooms, KVIC industries, agriculture, aquaculture, animal husbandry and dairying, floriculture and horticulture, poultry, viticulture, sericulture, carpets, coir and jute.", false},

		{"averageOffset", "A shortfall in one year", "may be offset by excess in another, on an overall basis",
			"HBP 2023 para 5.18", confidenceSourced,
			"Excess exports in one year may offset a shortfall in the average in another, PROVIDED the average is maintained on an overall basis within the block or the obligation period. HBP 5.17 separately reduces the average where the whole sector's exports fell by more than 5%, and HBP 5.19 suspends it for the duration of an export ban. This tool reports both the year-by-year position and the overall one, and does not decide which a Regional Authority will take.", false},

		{"licensingYear", "Licensing year", "1 April to 31 March",
			"The Foreign Trade (Development and Regulation) framework; FTP 2023 para 5.04(c)", confidenceCertain,
			"The average obligation is measured against this year. The obligation period and its blocks are counted from the date of issue instead, so the two clocks do not turn over together.", false},

		{"averageBasis", "Average obligation measured against", p.AverageBasis.Title(),
			"FTP 2023 para 5.04(c) for the licensing-year basis", confidenceSourced,
			"Licensing years is the default and is what the Policy requires. Anniversary years are offered only for an exporter who has been told their licence is computed that way.", true},

		{"partYearProRata", "Average obligation in a part year", map[bool]string{true: "pro-rated by days", false: "a full year's average"}[p.ProRataPartYears()],
			"Not from the Policy", confidenceTool,
			"FTP 5.04(c) requires the average in every financial year and does not say what happens to a financial year only part of which falls inside the period. Pro-rating is the default because the alternative charges a full year's average to a few weeks.", true},

		{"periodEnd", "Last day of a derived period", map[bool]string{true: "the day before the anniversary", false: "the anniversary itself"}[p.EndsDayBefore()],
			"Not from the Policy", confidenceTool,
			"Practice is not uniform and the authorisation prints the date. The default is the earlier reading, because being wrong in the direction of urgency is the safe direction. Enter the endorsed end date and this convention is not used at all.", true},

		{"legacy", "Authorisations issued before 1 April 2023", "governed by a different notification, not read for this build",
			"FTP 2023 para 5.13; HBP 2023 paras 5.16(a) and 4.48", confidenceUnsourced,
			"Notification 26/2023-Customs defines the Policy as FTP 2023, so it governs FTP 2023 authorisations only. An older authorisation runs under the notification in force when its goods were cleared, whose rate and conditions this build has NOT verified. Read the notification quoted on your own bill of entry and set the figures here to match it.", false},
	}
}

// itoa is strconv.Itoa under a shorter name, used only for building the
// citation strings above.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
