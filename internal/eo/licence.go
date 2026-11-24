package eo

import (
	"fmt"
	"sort"
	"strings"
)

// Scheme is which of the two duty-exemption schemes an authorisation was issued
// under. The two carry different obligations, measured differently, over
// different periods, so almost every rule in this package branches on it.
type Scheme string

const (
	// SchemeEPCG is the Export Promotion Capital Goods scheme: capital goods
	// imported at nil or concessional duty against an export obligation
	// expressed as a MULTIPLE OF THE DUTY SAVED, to be fulfilled over a period
	// of years with a milestone part-way through.
	SchemeEPCG Scheme = "epcg"

	// SchemeAdvanceAuthorisation is Advance Authorisation: inputs imported
	// duty-free against an obligation to export the RESULTANT PRODUCT, in a
	// quantity and value worked out from an input-output norm, within a shorter
	// period, and with a minimum value addition to achieve.
	SchemeAdvanceAuthorisation Scheme = "advance-authorisation"
)

// Title names the scheme the way the authorisation does.
func (s Scheme) Title() string {
	switch s {
	case SchemeEPCG:
		return "EPCG"
	case SchemeAdvanceAuthorisation:
		return "Advance Authorisation"
	default:
		return string(s)
	}
}

// Valid reports whether the scheme is one this tool knows.
func (s Scheme) Valid() bool {
	return s == SchemeEPCG || s == SchemeAdvanceAuthorisation
}

// NormaliseScheme reads what a user is likely to type or paste.
func NormaliseScheme(s Scheme) Scheme {
	switch strings.ToLower(strings.Join(strings.Fields(string(s)), "-")) {
	case "epcg", "epcg-scheme", "export-promotion-capital-goods", "zero-duty-epcg":
		return SchemeEPCG
	case "advance-authorisation", "advance-authorization", "aa", "advance-licence",
		"advance-license", "advance-authorisation-scheme":
		return SchemeAdvanceAuthorisation
	default:
		return Scheme(strings.ToLower(strings.TrimSpace(string(s))))
	}
}

// Category is the concession, if any, that reduces the EPCG export obligation
// below the ordinary multiple of the duty saved. The Foreign Trade Policy
// provides several; which ones exist and what they reduce the obligation to
// changes with each amendment, so the multiple each category carries is a field
// of Policy rather than a fact of this type.
type Category string

const (
	// CategoryOrdinary is an EPCG authorisation with no concession: the full
	// multiple of the duty saved.
	CategoryOrdinary Category = "ordinary"

	// CategoryGreenTechnology is the reduced obligation the Policy provides for
	// authorisations covering green technology products.
	CategoryGreenTechnology Category = "green-technology"

	// CategorySpecifiedArea is the reduced obligation the Policy provides for
	// units in areas it specifies — the North Eastern States, Jammu & Kashmir
	// and others named in the Policy from time to time.
	CategorySpecifiedArea Category = "specified-area"

	// CategoryIndigenousSourcing is the reduced obligation where the capital
	// goods are sourced indigenously rather than imported.
	CategoryIndigenousSourcing Category = "indigenous-sourcing"

	// CategoryAsEndorsed means: do not derive the obligation at all, use the
	// figure endorsed on the authorisation. This is the right answer whenever
	// the authorisation states its own export obligation, which it normally
	// does, and it is what the tool recommends.
	CategoryAsEndorsed Category = "as-endorsed"
)

// Title names the category for the screen.
func (c Category) Title() string {
	switch c {
	case CategoryOrdinary:
		return "Ordinary EPCG authorisation"
	case CategoryGreenTechnology:
		return "Green technology product"
	case CategorySpecifiedArea:
		return "Unit in a specified area"
	case CategoryIndigenousSourcing:
		return "Capital goods sourced indigenously"
	case CategoryAsEndorsed:
		return "Use the obligation endorsed on the authorisation"
	default:
		return string(c)
	}
}

// NormsBasis is how an Advance Authorisation's input entitlement was worked
// out. It does not change any arithmetic here; it is recorded because the
// redemption file needs it and because a self-declared norm is redeemed on a
// different footing from a published one.
type NormsBasis string

const (
	// NormsSION — a Standard Input Output Norm published by the DGFT.
	NormsSION NormsBasis = "sion"
	// NormsSelfDeclared — no published norm, so the applicant declared the
	// input-output ratio and the Norms Committee is to fix it.
	NormsSelfDeclared NormsBasis = "self-declared"
	// NormsAdHoc — an ad hoc norm fixed for this authorisation.
	NormsAdHoc NormsBasis = "ad-hoc"
	// NormsNotStated — the user has not said.
	NormsNotStated NormsBasis = ""
)

// Title names the basis for the screen.
func (n NormsBasis) Title() string {
	switch n {
	case NormsSION:
		return "Standard Input Output Norms (SION)"
	case NormsSelfDeclared:
		return "Self-declared norms, to be fixed by the Norms Committee"
	case NormsAdHoc:
		return "Ad hoc norms fixed for this authorisation"
	default:
		return "Not stated"
	}
}

// Import is one consignment brought in against the authorisation.
//
// Each one is held separately rather than as a single total because interest on
// a duty that becomes payable runs from the date of CLEARANCE of the goods, and
// consignments cleared a year apart carry a year's difference in interest. A
// tool that kept one total and one date would understate the exposure on the
// earliest consignment, which is the largest part of it.
type Import struct {
	Description   string `json:"description"`
	BillOfEntryNo string `json:"billOfEntryNo"`
	Port          string `json:"port"`
	ClearedOn     Date   `json:"clearedOn"`

	// CIFValue is the assessable value of the consignment. For Advance
	// Authorisation it is the B term of the value-addition formula.
	CIFValue Money `json:"cifValue"`

	// DutySaved is the duty foregone on this consignment: the figure the
	// exposure is computed from. For an EPCG authorisation it is also what the
	// export obligation is a multiple of.
	DutySaved Money `json:"dutySaved"`

	// InstalledOn is the date of the installation certificate for the capital
	// goods in this consignment, where one has been obtained. EPCG only.
	InstalledOn Date `json:"installedOn"`

	// Indigenous marks capital goods sourced from a domestic manufacturer
	// rather than imported. There is no duty saved and no installation date to
	// chase on the import side, but the obligation still attaches.
	Indigenous bool `json:"indigenous"`
}

func (im Import) normalise() Import {
	im.Description = tidy(im.Description)
	im.BillOfEntryNo = tidyUpper(im.BillOfEntryNo)
	im.Port = tidy(im.Port)
	return im
}

func (im Import) validate(i int) error {
	where := fmt.Sprintf("import %d", i+1)
	if im.CIFValue < 0 {
		return fmt.Errorf("%s has a negative CIF value", where)
	}
	if im.DutySaved < 0 {
		return fmt.Errorf("%s has a negative duty saved", where)
	}
	if !im.ClearedOn.Zero() && !im.ClearedOn.Valid() {
		return fmt.Errorf("%s has a clearance date that is not a day that exists", where)
	}
	if !im.InstalledOn.Zero() && !im.InstalledOn.Valid() {
		return fmt.Errorf("%s has an installation date that is not a day that exists", where)
	}
	if !im.ClearedOn.Zero() && !im.InstalledOn.Zero() && im.InstalledOn.Before(im.ClearedOn) {
		return fmt.Errorf("%s is recorded as installed on %s, before it was cleared on %s",
			where, im.InstalledOn.Human(), im.ClearedOn.Human())
	}
	if len([]rune(im.Description)) > 200 {
		return fmt.Errorf("%s has a description longer than 200 characters", where)
	}
	return nil
}

// Shipment is one export counted — or offered to be counted — toward the
// obligation. It is a shipping bill, not an invoice: an export discharges an
// obligation when it is made under a shipping bill carrying the authorisation
// number, and realised.
type Shipment struct {
	ShippingBillNo string `json:"shippingBillNo"`
	ShippedOn      Date   `json:"shippedOn"`
	Port           string `json:"port"`
	Product        string `json:"product"`
	Buyer          string `json:"buyer"`

	// FOBValue is the free-on-board value of the export, in the authorisation's
	// currency. No currency conversion happens anywhere in this tool.
	FOBValue Money `json:"fobValue"`

	// Quantity of the resultant product exported, with its unit. It matters for
	// an Advance Authorisation, whose obligation is a quantity as well as a
	// value.
	Quantity     Quantity `json:"quantity"`
	QuantityUnit string   `json:"quantityUnit"`

	// AuthorisationEndorsed records whether the authorisation number is
	// endorsed on the shipping bill. An export that is not endorsed is not
	// counted toward the obligation — this is the commonest reason an exporter
	// who has shipped enough still cannot close the licence.
	AuthorisationEndorsed bool `json:"authorisationEndorsed"`

	// RealisedOn and RealisedValue record proof of realisation — the bank
	// realisation certificate, now the e-BRC. An unrealised export is still
	// counted toward the obligation here, and reported separately as evidence
	// not yet in hand, because the two states need different action from the
	// exporter.
	RealisedOn    Date   `json:"realisedOn"`
	RealisedValue Money  `json:"realisedValue"`
	BRCNo         string `json:"brcNo"`

	// SameOrSimilar records whether this export is of the same or a similar
	// product as the one the average export obligation was fixed on. Only such
	// exports are measured against the average. EPCG only.
	SameOrSimilar bool `json:"sameOrSimilar"`

	// Excluded keeps a shipment in the file without counting it — a shipment
	// already counted against another licence, or one the holder has decided
	// not to offer. The reason is kept with it.
	Excluded       bool   `json:"excluded"`
	ExcludedReason string `json:"excludedReason"`
}

func (sh Shipment) normalise() Shipment {
	sh.ShippingBillNo = tidyUpper(sh.ShippingBillNo)
	sh.Port = tidy(sh.Port)
	sh.Product = tidy(sh.Product)
	sh.Buyer = tidy(sh.Buyer)
	sh.QuantityUnit = tidy(sh.QuantityUnit)
	sh.BRCNo = tidyUpper(sh.BRCNo)
	sh.ExcludedReason = tidy(sh.ExcludedReason)
	if !sh.Excluded {
		sh.ExcludedReason = ""
	}
	return sh
}

func (sh Shipment) validate(i int) error {
	where := fmt.Sprintf("shipment %d", i+1)
	if sh.ShippingBillNo != "" && len([]rune(sh.ShippingBillNo)) > 40 {
		return fmt.Errorf("%s has a shipping bill number longer than 40 characters", where)
	}
	if sh.FOBValue < 0 {
		return fmt.Errorf("%s has a negative FOB value", where)
	}
	if sh.RealisedValue < 0 {
		return fmt.Errorf("%s has a negative realised value", where)
	}
	if sh.Quantity < 0 {
		return fmt.Errorf("%s has a negative quantity", where)
	}
	if !sh.ShippedOn.Zero() && !sh.ShippedOn.Valid() {
		return fmt.Errorf("%s has a shipment date that is not a day that exists", where)
	}
	if !sh.RealisedOn.Zero() && !sh.RealisedOn.Valid() {
		return fmt.Errorf("%s has a realisation date that is not a day that exists", where)
	}
	if !sh.ShippedOn.Zero() && !sh.RealisedOn.Zero() && sh.RealisedOn.Before(sh.ShippedOn) {
		return fmt.Errorf("%s is recorded as realised on %s, before it shipped on %s",
			where, sh.RealisedOn.Human(), sh.ShippedOn.Human())
	}
	if sh.ShippedOn.Zero() && !sh.Excluded && sh.FOBValue > 0 {
		return fmt.Errorf("%s has a value but no shipment date, so there is no way to say which block or licensing year it falls in", where)
	}
	return nil
}

// Counts reports whether a shipment is counted toward the obligation: it has to
// be inside the file, not excluded, dated, and endorsed with the authorisation
// number.
func (sh Shipment) Counts() bool {
	return !sh.Excluded && !sh.ShippedOn.Zero() && sh.AuthorisationEndorsed
}

// ExtensionKind is which clock an extension moves.
type ExtensionKind string

const (
	// ExtensionBlock moves the end of a block-wise milestone without moving the
	// overall period.
	ExtensionBlock ExtensionKind = "block"
	// ExtensionOverall moves the end of the whole export obligation period.
	ExtensionOverall ExtensionKind = "overall"
)

// Extension is an extension of time actually granted — not one the holder hopes
// for. The tool applies only extensions that have been granted, and shows the
// unextended dates alongside, so that nobody plans on an extension as though it
// were already in hand.
type Extension struct {
	Kind      ExtensionKind `json:"kind"`
	BlockNo   int           `json:"blockNo"` // for a block extension: which block
	GrantedOn Date          `json:"grantedOn"`
	NewEndsOn Date          `json:"newEndsOn"`
	Reference string        `json:"reference"`
	FeePaid   Money         `json:"feePaid"`
}

func (ex Extension) normalise() Extension {
	ex.Reference = tidy(ex.Reference)
	if ex.Kind != ExtensionBlock {
		ex.Kind = ExtensionOverall
		ex.BlockNo = 0
	}
	if ex.BlockNo < 0 {
		ex.BlockNo = 0
	}
	return ex
}

func (ex Extension) validate(i int) error {
	where := fmt.Sprintf("extension %d", i+1)
	if ex.NewEndsOn.Zero() {
		return fmt.Errorf("%s does not say what date it extends the period to", where)
	}
	if !ex.NewEndsOn.Valid() {
		return fmt.Errorf("%s extends the period to a day that does not exist", where)
	}
	if !ex.GrantedOn.Zero() && !ex.GrantedOn.Valid() {
		return fmt.Errorf("%s was granted on a day that does not exist", where)
	}
	if ex.Kind == ExtensionBlock && ex.BlockNo < 1 {
		return fmt.Errorf("%s extends a block but does not say which one", where)
	}
	if ex.FeePaid < 0 {
		return fmt.Errorf("%s records a negative fee", where)
	}
	return nil
}

// Evidence holds the few parts of the redemption file that cannot be worked out
// from the shipments and imports. Everything else on the checklist is derived
// from the data, so that ticking a box cannot make a licence look closeable
// when the figures say it is not.
type Evidence struct {
	// CACertifiedStatement — a statement of exports certified by a chartered
	// accountant, cost accountant or company secretary, which the redemption
	// application is normally accompanied by.
	CACertifiedStatement bool `json:"caCertifiedStatement"`
	// RCMCHeld — a valid registration-cum-membership certificate from the
	// relevant export promotion council.
	RCMCHeld bool `json:"rcmcHeld"`
	// BondOrBGDetails — the bond or bank guarantee executed with Customs, which
	// is cancelled on redemption.
	BondOrBGDetails string `json:"bondOrBgDetails"`
	// ApplicationFiled and FiledOn record that the redemption application has
	// gone in. This tool files nothing; the fields are here so the file is
	// complete.
	ApplicationFiled bool   `json:"applicationFiled"`
	FiledOn          Date   `json:"filedOn"`
	FileReference    string `json:"fileReference"`
}

func (ev Evidence) normalise() Evidence {
	ev.BondOrBGDetails = tidy(ev.BondOrBGDetails)
	ev.FileReference = tidy(ev.FileReference)
	if !ev.ApplicationFiled {
		ev.FiledOn = Date{}
		ev.FileReference = ""
	}
	return ev
}

// Licence is one authorisation and everything recorded against it.
//
// Where a figure is printed on the authorisation itself, the field for it is
// named "...Endorsed" and takes precedence over anything this tool would
// derive. That is deliberate: the obligation an exporter owes is the one the
// Directorate General of Foreign Trade endorsed on the licence, not the one a
// multiple in a policy document works out to. The derived figure exists to
// cross-check the endorsed one and to serve where it has not been entered.
type Licence struct {
	ID     string `json:"id"`
	Scheme Scheme `json:"scheme"`

	AuthorisationNo string `json:"authorisationNo"`
	FileNo          string `json:"fileNo"`
	Holder          string `json:"holder"`
	IEC             string `json:"iec"`
	Product         string `json:"product"`

	// Currency is a LABEL. No rate is applied and nothing is converted: an
	// authorisation states its obligation in one currency, and every figure
	// entered against it must be in that same currency. The tool refuses to
	// pretend otherwise, because the only honest exchange rate is the one on
	// the exporter's own documents.
	Currency string `json:"currency"`

	IssuedOn Date `json:"issuedOn"`

	// ---- EPCG ----

	// Category is the concession, if any, that reduces the obligation.
	Category Category `json:"category"`

	// SpecificEOEndorsed is the export obligation in value as endorsed on the
	// authorisation. Zero means "derive it".
	SpecificEOEndorsed Money `json:"specificEoEndorsed"`

	// AverageEO is the average export obligation to be maintained in each
	// licensing year — the average of exports of the same and similar products
	// in the preceding licensing years, as endorsed on the authorisation.
	AverageEO Money `json:"averageEo"`

	// AverageEOExempt records that the holder is not required to maintain an
	// average, with the reason endorsed. An exemption is a matter of the Policy
	// and of what the authorisation says; this tool records it, it does not
	// decide it.
	AverageEOExempt       bool   `json:"averageEoExempt"`
	AverageEOExemptReason string `json:"averageEoExemptReason"`

	// ---- Advance Authorisation ----

	// EOValueEndorsed is the FOB value of the export obligation as endorsed.
	EOValueEndorsed Money `json:"eoValueEndorsed"`
	// EOQuantity is the quantity of the resultant product to be exported.
	EOQuantity     Quantity `json:"eoQuantity"`
	EOQuantityUnit string   `json:"eoQuantityUnit"`

	NormsBasis    NormsBasis `json:"normsBasis"`
	SIONReference string     `json:"sionReference"`

	// OtherInputsCIF is the value of imported materials used in the resultant
	// product that are not covered by this authorisation. It belongs in the B
	// term of the value-addition formula, and leaving it out overstates the
	// value addition achieved.
	OtherInputsCIF Money `json:"otherInputsCif"`

	// ---- the clock ----

	// EOPeriodEndEndorsed is the last day of the export obligation period as
	// endorsed on the authorisation. Zero means "derive it", and a derived date
	// is always labelled as derived.
	EOPeriodEndEndorsed Date `json:"eoPeriodEndEndorsed"`

	// BlockEndsEndorsed are the block end dates as endorsed, where the
	// authorisation prints them. Zero length means "derive them".
	BlockEndsEndorsed []Date `json:"blockEndsEndorsed"`

	Extensions []Extension `json:"extensions"`

	// ---- what has happened ----

	Imports   []Import   `json:"imports"`
	Shipments []Shipment `json:"shipments"`
	Evidence  Evidence   `json:"evidence"`

	Notes string `json:"notes"`

	// Policy is the set of figures to apply to THIS licence. A licence issued
	// under one edition of the Policy keeps being governed by it after the
	// Policy is amended, so the figures belong to the licence and not to the
	// tool. Zero fields fall back to the figures as published.
	Policy Policy `json:"policy"`
}

// Normalise tidies a licence and fills in what can be filled in without
// inventing anything.
func (l Licence) Normalise() Licence {
	l.ID = tidy(l.ID)
	l.Scheme = NormaliseScheme(l.Scheme)
	l.AuthorisationNo = tidyUpper(l.AuthorisationNo)
	l.FileNo = tidyUpper(l.FileNo)
	l.Holder = tidy(l.Holder)
	l.IEC = tidyUpper(l.IEC)
	l.Product = tidy(l.Product)
	l.SIONReference = tidyUpper(l.SIONReference)
	l.AverageEOExemptReason = tidy(l.AverageEOExemptReason)
	l.Notes = strings.TrimSpace(l.Notes)

	l.Currency = tidyUpper(l.Currency)
	if l.Currency == "" {
		l.Currency = DefaultCurrency
	}

	if l.Category == "" {
		if l.SpecificEOEndorsed > 0 {
			l.Category = CategoryAsEndorsed
		} else {
			l.Category = CategoryOrdinary
		}
	}
	if !l.AverageEOExempt {
		l.AverageEOExemptReason = ""
	}

	for i := range l.Imports {
		l.Imports[i] = l.Imports[i].normalise()
	}
	for i := range l.Shipments {
		l.Shipments[i] = l.Shipments[i].normalise()
	}
	for i := range l.Extensions {
		l.Extensions[i] = l.Extensions[i].normalise()
	}
	l.Evidence = l.Evidence.normalise()
	l.Policy = l.Policy.Normalise()

	// A stable order: shipments and imports by date, then by document number,
	// so the file, the screen and the report always read the same way.
	sort.SliceStable(l.Shipments, func(i, j int) bool {
		a, b := l.Shipments[i], l.Shipments[j]
		if !a.ShippedOn.Equal(b.ShippedOn) {
			return a.ShippedOn.Before(b.ShippedOn)
		}
		return a.ShippingBillNo < b.ShippingBillNo
	})
	sort.SliceStable(l.Imports, func(i, j int) bool {
		a, b := l.Imports[i], l.Imports[j]
		if !a.ClearedOn.Equal(b.ClearedOn) {
			return a.ClearedOn.Before(b.ClearedOn)
		}
		return a.BillOfEntryNo < b.BillOfEntryNo
	})
	sort.SliceStable(l.Extensions, func(i, j int) bool {
		return l.Extensions[i].NewEndsOn.Before(l.Extensions[j].NewEndsOn)
	})

	ends := make([]Date, 0, len(l.BlockEndsEndorsed))
	for _, d := range l.BlockEndsEndorsed {
		if !d.Zero() {
			ends = append(ends, d)
		}
	}
	sort.SliceStable(ends, func(i, j int) bool { return ends[i].Before(ends[j]) })
	l.BlockEndsEndorsed = ends

	return l
}

// Validate refuses a licence that cannot be assessed, with a message that says
// what to do about it. It is deliberately strict about dates and amounts and
// deliberately lax about descriptive text.
func (l Licence) Validate() error {
	if !l.Scheme.Valid() {
		return fmt.Errorf("choose a scheme: EPCG or Advance Authorisation")
	}
	if l.IssuedOn.Zero() {
		return fmt.Errorf("enter the date the authorisation was issued — every period in both schemes is counted from it")
	}
	if !l.IssuedOn.Valid() {
		return fmt.Errorf("the date of issue is not a day that exists")
	}
	if !l.EOPeriodEndEndorsed.Zero() {
		if !l.EOPeriodEndEndorsed.Valid() {
			return fmt.Errorf("the endorsed end of the export obligation period is not a day that exists")
		}
		if l.EOPeriodEndEndorsed.Before(l.IssuedOn) {
			return fmt.Errorf("the endorsed end of the obligation period, %s, falls before the authorisation was issued on %s",
				l.EOPeriodEndEndorsed.Human(), l.IssuedOn.Human())
		}
	}
	if l.SpecificEOEndorsed < 0 || l.AverageEO < 0 || l.EOValueEndorsed < 0 || l.OtherInputsCIF < 0 {
		return fmt.Errorf("an obligation or a value cannot be negative")
	}
	if l.EOQuantity < 0 {
		return fmt.Errorf("an obligation quantity cannot be negative")
	}
	if len([]rune(l.Notes)) > 4000 {
		return fmt.Errorf("the notes on a licence are longer than 4000 characters")
	}
	if len(l.Imports) > MaxImportsPerLicence {
		return fmt.Errorf("a licence in this tool holds up to %d import consignments; this one has %d",
			MaxImportsPerLicence, len(l.Imports))
	}
	if len(l.Shipments) > MaxShipmentsPerLicence {
		return fmt.Errorf("a licence in this tool holds up to %d shipping bills; this one has %d",
			MaxShipmentsPerLicence, len(l.Shipments))
	}
	if len(l.Extensions) > MaxExtensionsPerLicence {
		return fmt.Errorf("a licence in this tool holds up to %d extensions; this one has %d",
			MaxExtensionsPerLicence, len(l.Extensions))
	}
	for i, im := range l.Imports {
		if err := im.validate(i); err != nil {
			return err
		}
		if !im.ClearedOn.Zero() && im.ClearedOn.Before(l.IssuedOn) {
			return fmt.Errorf("import %d was cleared on %s, before the authorisation was issued on %s",
				i+1, im.ClearedOn.Human(), l.IssuedOn.Human())
		}
	}
	for i, sh := range l.Shipments {
		if err := sh.validate(i); err != nil {
			return err
		}
	}
	for i, ex := range l.Extensions {
		if err := ex.validate(i); err != nil {
			return err
		}
		if ex.NewEndsOn.Before(l.IssuedOn) {
			return fmt.Errorf("extension %d extends the period to %s, before the authorisation was issued",
				i+1, ex.NewEndsOn.Human())
		}
	}
	if l.Scheme == SchemeEPCG && l.Category == CategoryAsEndorsed && l.SpecificEOEndorsed == 0 {
		return fmt.Errorf("you have asked to use the obligation endorsed on the authorisation, but have not entered it")
	}
	return nil
}

// DutySavedTotal is the duty foregone across every consignment: for EPCG, the
// figure the obligation is a multiple of, and for both schemes the principal of
// the exposure.
func (l Licence) DutySavedTotal() Money {
	var total Money
	for _, im := range l.Imports {
		total += im.DutySaved
	}
	return total
}

// CIFTotal is the CIF value of everything imported against the authorisation.
func (l Licence) CIFTotal() Money {
	var total Money
	for _, im := range l.Imports {
		total += im.CIFValue
	}
	return total
}

// Label names the licence for a list: its authorisation number if it has one,
// otherwise its product, otherwise the scheme.
func (l Licence) Label() string {
	switch {
	case l.AuthorisationNo != "":
		return l.AuthorisationNo
	case l.Product != "":
		return l.Product
	default:
		return l.Scheme.Title() + " authorisation"
	}
}

// tidy collapses runs of whitespace and trims the ends.
func tidy(s string) string { return strings.Join(strings.Fields(s), " ") }

// tidyUpper does the same and upper-cases, for the reference numbers that are
// always printed in capitals.
func tidyUpper(s string) string { return strings.ToUpper(tidy(s)) }
