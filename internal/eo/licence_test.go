package eo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormaliseScheme(t *testing.T) {
	cases := []struct {
		in   Scheme
		want Scheme
	}{
		{"epcg", SchemeEPCG},
		{"EPCG", SchemeEPCG},
		{"  EPCG Scheme  ", SchemeEPCG},
		{"zero duty epcg", SchemeEPCG},
		{"export promotion capital goods", SchemeEPCG},
		{"advance-authorisation", SchemeAdvanceAuthorisation},
		{"Advance Authorization", SchemeAdvanceAuthorisation},
		{"AA", SchemeAdvanceAuthorisation},
		{"advance licence", SchemeAdvanceAuthorisation},
		{"DFIA", "dfia"}, // a different scheme; passed through, and refused by Validate
		{"", ""},
	}
	for _, c := range cases {
		t.Run(string(c.in), func(t *testing.T) {
			if got := NormaliseScheme(c.in); got != c.want {
				t.Fatalf("NormaliseScheme(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
	if !SchemeEPCG.Valid() || !SchemeAdvanceAuthorisation.Valid() {
		t.Fatalf("both schemes must be valid")
	}
	if Scheme("dfia").Valid() {
		t.Fatalf("this tool covers two schemes and must not pretend to cover a third")
	}
	if SchemeEPCG.Title() != "EPCG" || SchemeAdvanceAuthorisation.Title() != "Advance Authorisation" {
		t.Fatalf("titles = %q and %q", SchemeEPCG.Title(), SchemeAdvanceAuthorisation.Title())
	}
	if got := Scheme("dfia").Title(); got != "dfia" {
		t.Fatalf("got %q", got)
	}
}

func TestLicenceNormalise(t *testing.T) {
	l := Licence{
		Scheme:          "EPCG",
		AuthorisationNo: "  0330099887  ",
		Holder:          "Nandi   Precision    Components",
		IEC:             "0788123456",
		Product:         "  Machined   housings ",
		Currency:        "inr",
		IssuedOn:        MustDate("2023-05-15"),
		Shipments: []Shipment{
			ship("2024-06-01", "10,00,000", true),
			ship("2023-08-01", "20,00,000", true),
			ship("2024-06-01", "30,00,000", true),
		},
		Imports: []Import{
			{BillOfEntryNo: "zzz", ClearedOn: MustDate("2024-01-01")},
			{BillOfEntryNo: "aaa", ClearedOn: MustDate("2023-07-01")},
		},
		BlockEndsEndorsed: []Date{{}, MustDate("2027-05-15"), {}},
	}.Normalise()

	if l.Scheme != SchemeEPCG {
		t.Fatalf("scheme = %q", l.Scheme)
	}
	if l.AuthorisationNo != "0330099887" {
		t.Fatalf("authorisation number = %q", l.AuthorisationNo)
	}
	if l.Holder != "Nandi Precision Components" {
		t.Fatalf("holder = %q", l.Holder)
	}
	if l.Product != "Machined housings" {
		t.Fatalf("product = %q", l.Product)
	}
	if l.Currency != "INR" {
		t.Fatalf("currency = %q", l.Currency)
	}
	// Shipments in date order, then by bill number, so the file, the screen and
	// the report always read the same way.
	if l.Shipments[0].ShippedOn.String() != "2023-08-01" {
		t.Fatalf("shipments are not in date order: %s first", l.Shipments[0].ShippedOn)
	}
	if l.Shipments[1].ShippingBillNo > l.Shipments[2].ShippingBillNo {
		t.Fatalf("same-day shipments are not ordered by bill number")
	}
	if l.Imports[0].BillOfEntryNo != "AAA" {
		t.Fatalf("imports are not in clearance order, or not upper-cased: %q", l.Imports[0].BillOfEntryNo)
	}
	// Empty endorsed block dates are dropped, not kept as holes that would
	// shift the remaining ones onto the wrong block.
	if len(l.BlockEndsEndorsed) != 1 || l.BlockEndsEndorsed[0].String() != "2027-05-15" {
		t.Fatalf("endorsed block ends = %v", l.BlockEndsEndorsed)
	}
	// An empty policy normalises to the published figures.
	if l.Policy.EPCGMultipleHundredths != 600 {
		t.Fatalf("policy was not normalised: %+v", l.Policy)
	}

	t.Run("a currency defaults to rupees", func(t *testing.T) {
		if got := (Licence{Scheme: SchemeEPCG}).Normalise().Currency; got != "INR" {
			t.Fatalf("currency = %q", got)
		}
	})

	t.Run("a category is inferred from whether an obligation is endorsed", func(t *testing.T) {
		withEndorsed := Licence{Scheme: SchemeEPCG, SpecificEOEndorsed: MustMoney("1,00,000")}.Normalise()
		if withEndorsed.Category != CategoryAsEndorsed {
			t.Fatalf("category = %q, want as-endorsed", withEndorsed.Category)
		}
		without := Licence{Scheme: SchemeEPCG}.Normalise()
		if without.Category != CategoryOrdinary {
			t.Fatalf("category = %q, want ordinary", without.Category)
		}
	})

	t.Run("an exemption reason is dropped when the exemption is not claimed", func(t *testing.T) {
		got := Licence{Scheme: SchemeEPCG, AverageEOExemptReason: "handicrafts"}.Normalise()
		if got.AverageEOExemptReason != "" {
			t.Fatalf("reason = %q", got.AverageEOExemptReason)
		}
	})

	t.Run("an exclusion reason is dropped when the shipment is not excluded", func(t *testing.T) {
		s := ship("2024-06-01", "1,00,000", true)
		s.ExcludedReason = "already counted"
		got := Licence{Scheme: SchemeEPCG, Shipments: []Shipment{s}}.Normalise()
		if got.Shipments[0].ExcludedReason != "" {
			t.Fatalf("reason = %q", got.Shipments[0].ExcludedReason)
		}
	})

	t.Run("a file reference is dropped when nothing has been filed", func(t *testing.T) {
		got := Licence{Scheme: SchemeEPCG, Evidence: Evidence{
			FiledOn: MustDate("2026-01-01"), FileReference: "X/1"}}.Normalise()
		if !got.Evidence.FiledOn.Zero() || got.Evidence.FileReference != "" {
			t.Fatalf("evidence = %+v", got.Evidence)
		}
	})
}

func TestLicenceValidate(t *testing.T) {
	good := func() Licence {
		return Licence{
			Scheme:             SchemeEPCG,
			IssuedOn:           MustDate("2023-05-15"),
			Category:           CategoryAsEndorsed,
			SpecificEOEndorsed: MustMoney("3,60,00,000"),
		}
	}

	cases := []struct {
		name string
		mut  func(*Licence)
		want string // a fragment of the message, or "" for no error
	}{
		{"a complete licence", nil, ""},
		{"no scheme", func(l *Licence) { l.Scheme = "" }, "choose a scheme"},
		{"a scheme this tool does not cover", func(l *Licence) { l.Scheme = "dfia" }, "choose a scheme"},
		{"no date of issue", func(l *Licence) { l.IssuedOn = Date{} }, "date the authorisation was issued"},
		{"a date of issue that does not exist", func(l *Licence) { l.IssuedOn = Date{2023, 2, 30} }, "not a day that exists"},
		{
			"an endorsed period end before the date of issue",
			func(l *Licence) { l.EOPeriodEndEndorsed = MustDate("2022-01-01") },
			"before the authorisation was issued",
		},
		{
			"an endorsed period end that does not exist",
			func(l *Licence) { l.EOPeriodEndEndorsed = Date{2029, 13, 1} },
			"not a day that exists",
		},
		{"a negative obligation", func(l *Licence) { l.SpecificEOEndorsed = -1 }, "cannot be negative"},
		{"a negative average", func(l *Licence) { l.AverageEO = -1 }, "cannot be negative"},
		{"a negative obligation quantity", func(l *Licence) { l.EOQuantity = -1 }, "quantity cannot be negative"},
		{
			"asked to use the endorsed obligation but none entered",
			func(l *Licence) { l.SpecificEOEndorsed = 0 },
			"have not entered it",
		},
		{
			"an import cleared before the authorisation was issued",
			func(l *Licence) {
				l.Imports = []Import{{BillOfEntryNo: "1", ClearedOn: MustDate("2023-01-01"), DutySaved: MustMoney("1,00,000")}}
			},
			"before the authorisation was issued",
		},
		{
			"an import installed before it was cleared",
			func(l *Licence) {
				l.Imports = []Import{{BillOfEntryNo: "1", ClearedOn: MustDate("2023-07-01"),
					InstalledOn: MustDate("2023-06-01")}}
			},
			"before it was cleared",
		},
		{
			"a negative duty saved",
			func(l *Licence) { l.Imports = []Import{{BillOfEntryNo: "1", DutySaved: -1}} },
			"negative duty saved",
		},
		{
			"a shipment realised before it shipped",
			func(l *Licence) {
				s := ship("2024-06-01", "1,00,000", true)
				s.RealisedOn = MustDate("2024-05-01")
				l.Shipments = []Shipment{s}
			},
			"before it shipped",
		},
		{
			"a shipment with a value but no date",
			func(l *Licence) {
				l.Shipments = []Shipment{{ShippingBillNo: "1", FOBValue: MustMoney("1,00,000"), AuthorisationEndorsed: true}}
			},
			"no shipment date",
		},
		{
			"a negative FOB value",
			func(l *Licence) {
				l.Shipments = []Shipment{{ShippingBillNo: "1", ShippedOn: MustDate("2024-06-01"), FOBValue: -1}}
			},
			"negative FOB value",
		},
		{
			"an extension that does not say what to",
			func(l *Licence) { l.Extensions = []Extension{{Kind: ExtensionOverall}} },
			"does not say what date",
		},
		{
			"a block extension that does not say which block",
			func(l *Licence) {
				l.Extensions = []Extension{{Kind: ExtensionBlock, NewEndsOn: MustDate("2028-01-01")}}
			},
			"does not say which one",
		},
		{
			"an extension to a date before the authorisation was issued",
			func(l *Licence) {
				l.Extensions = []Extension{{Kind: ExtensionOverall, NewEndsOn: MustDate("2020-01-01")}}
			},
			"before the authorisation was issued",
		},
		{
			"notes longer than the limit",
			func(l *Licence) { l.Notes = strings.Repeat("x", 4001) },
			"longer than 4000",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := good()
			if c.mut != nil {
				c.mut(&l)
			}
			l = l.Normalise()
			err := l.Validate()
			if c.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not mention %q", err, c.want)
			}
		})
	}
}

func TestLicenceValidateRefusesTooMuch(t *testing.T) {
	base := Licence{Scheme: SchemeEPCG, IssuedOn: MustDate("2024-04-01"),
		Category: CategoryAsEndorsed, SpecificEOEndorsed: MustMoney("1,00,000")}

	t.Run("too many shipments", func(t *testing.T) {
		l := base
		l.Shipments = make([]Shipment, MaxShipmentsPerLicence+1)
		for i := range l.Shipments {
			l.Shipments[i] = Shipment{ShippingBillNo: "x", ShippedOn: MustDate("2024-06-01"), AuthorisationEndorsed: true}
		}
		if err := l.Normalise().Validate(); err == nil {
			t.Fatalf("expected a refusal")
		}
	})

	t.Run("too many imports", func(t *testing.T) {
		l := base
		l.Imports = make([]Import, MaxImportsPerLicence+1)
		if err := l.Normalise().Validate(); err == nil {
			t.Fatalf("expected a refusal")
		}
	})

	t.Run("too many extensions", func(t *testing.T) {
		l := base
		l.Extensions = make([]Extension, MaxExtensionsPerLicence+1)
		for i := range l.Extensions {
			l.Extensions[i] = Extension{Kind: ExtensionOverall, NewEndsOn: MustDate("2031-01-01")}
		}
		if err := l.Normalise().Validate(); err == nil {
			t.Fatalf("expected a refusal")
		}
	})
}

func TestShipmentCounts(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Shipment)
		want bool
	}{
		{"endorsed and dated", nil, true},
		{"not endorsed", func(s *Shipment) { s.AuthorisationEndorsed = false }, false},
		{"undated", func(s *Shipment) { s.ShippedOn = Date{} }, false},
		{"excluded by the holder", func(s *Shipment) { s.Excluded = true }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := ship("2024-06-01", "1,00,000", true)
			if c.mut != nil {
				c.mut(&s)
			}
			if got := s.Counts(); got != c.want {
				t.Fatalf("Counts() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestLicenceTotalsAndLabel(t *testing.T) {
	l := Licence{
		Scheme: SchemeEPCG,
		Imports: []Import{
			{DutySaved: MustMoney("45,00,000"), CIFValue: MustMoney("1,80,00,000")},
			{DutySaved: MustMoney("15,00,000"), CIFValue: MustMoney("60,00,000")},
		},
	}.Normalise()
	if l.DutySavedTotal() != MustMoney("60,00,000") {
		t.Fatalf("duty saved = %s", l.DutySavedTotal())
	}
	if l.CIFTotal() != MustMoney("2,40,00,000") {
		t.Fatalf("CIF = %s", l.CIFTotal())
	}

	cases := []struct {
		name string
		l    Licence
		want string
	}{
		{"by authorisation number", Licence{Scheme: SchemeEPCG, AuthorisationNo: "0330099887", Product: "Housings"}, "0330099887"},
		{"by product when there is no number", Licence{Scheme: SchemeEPCG, Product: "Housings"}, "Housings"},
		{"by scheme when there is neither", Licence{Scheme: SchemeEPCG}, "EPCG authorisation"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.l.Normalise().Label(); got != c.want {
				t.Fatalf("Label = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLicenceJSONRoundTrip(t *testing.T) {
	in := SampleEPCG().Normalise()
	raw, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The file must be readable: amounts as decimal strings, dates as
	// yyyy-mm-dd.
	text := string(raw)
	for _, want := range []string{`"36000000.00"`, `"2023-05-15"`, `"epcg"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("the saved form does not contain %s", want)
		}
	}

	var out Licence
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out = out.Normalise()

	if out.SpecificEOEndorsed != in.SpecificEOEndorsed ||
		out.AverageEO != in.AverageEO ||
		!out.IssuedOn.Equal(in.IssuedOn) ||
		len(out.Shipments) != len(in.Shipments) ||
		len(out.Imports) != len(in.Imports) {
		t.Fatalf("the licence did not survive the round trip")
	}

	// And the assessment is identical, which is the property that matters: a
	// licence reloaded from the file must produce the same figures.
	before := Assess(in, MustDate("2026-11-24"))
	after := Assess(out, MustDate("2026-11-24"))
	if before.Specific.Counted != after.Specific.Counted ||
		before.Exposure.Total != after.Exposure.Total ||
		before.EODC.BlockingCount != after.EODC.BlockingCount {
		t.Fatalf("a reloaded licence gives different figures")
	}
}

func TestTitlesAreAllPopulated(t *testing.T) {
	for _, c := range []Category{CategoryOrdinary, CategoryGreenTechnology, CategorySpecifiedArea,
		CategoryIndigenousSourcing, CategoryAsEndorsed} {
		if c.Title() == "" {
			t.Fatalf("category %q has no title", c)
		}
	}
	if got := Category("other").Title(); got != "other" {
		t.Fatalf("got %q", got)
	}
	for _, n := range []NormsBasis{NormsSION, NormsSelfDeclared, NormsAdHoc, NormsNotStated} {
		if n.Title() == "" {
			t.Fatalf("norms basis %q has no title", n)
		}
	}
	for _, b := range []AverageYearBasis{AverageByLicensingYear, AverageByAuthorisationYear, ""} {
		if b.Title() == "" {
			t.Fatalf("average basis %q has no title", b)
		}
	}
	for _, r := range []ReqState{ReqMet, ReqMissing, ReqUnknown, ReqNotApplicable} {
		req := Requirement{State: r}
		if r == ReqMet || r == ReqNotApplicable {
			if !req.Satisfied() {
				t.Fatalf("%q must count as satisfied", r)
			}
			continue
		}
		if req.Satisfied() {
			t.Fatalf("%q must not count as satisfied — a requirement nobody has checked is not one that has been met", r)
		}
	}
}
