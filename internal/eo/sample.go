package eo

// SampleEPCG is a worked example of an EPCG authorisation, built so that
// somebody opening the tool for the first time sees the thing it exists to
// show: an exporter who has shipped ₹5.6 crore against a ₹3.6 crore obligation
// and is still less than half way through it, because the average export
// obligation nets most of it off.
//
// The figures are invented. The arrangement is not: a machining unit that was
// already exporting, imports two machines, and counts gross exports instead of
// the excess over its own average.
func SampleEPCG() Licence {
	yes := true
	return Licence{
		Scheme:          SchemeEPCG,
		AuthorisationNo: "0330099887",
		FileNo:          "03/21/000/00412/AM24",
		Holder:          "Nandi Precision Components",
		IEC:             "0788123456",
		Product:         "Machined aluminium housings and brackets",
		Currency:        "INR",
		IssuedOn:        MustDate("2023-05-15"),

		Category:           CategoryAsEndorsed,
		SpecificEOEndorsed: MustMoney("3,60,00,000"),
		AverageEO:          MustMoney("1,20,00,000"),

		Imports: []Import{
			{
				Description:   "Vertical machining centre, 4-axis",
				BillOfEntryNo: "7712345",
				Port:          "INBLR4",
				ClearedOn:     MustDate("2023-07-10"),
				CIFValue:      MustMoney("1,80,00,000"),
				DutySaved:     MustMoney("45,00,000"),
				InstalledOn:   MustDate("2023-10-02"),
			},
			{
				Description:   "CNC turning centre with bar feeder",
				BillOfEntryNo: "8823456",
				Port:          "INBLR4",
				ClearedOn:     MustDate("2024-01-22"),
				CIFValue:      MustMoney("60,00,000"),
				DutySaved:     MustMoney("15,00,000"),
			},
		},

		Shipments: []Shipment{
			{ShippingBillNo: "4455661", ShippedOn: MustDate("2023-08-20"), Port: "INBLR4",
				Product: "Machined aluminium housings", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("85,00,000"), Quantity: MustQuantity("12400"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2023-11-10"), RealisedValue: MustMoney("85,00,000"), BRCNo: "BRC/23/114502"},

			{ShippingBillNo: "4455662", ShippedOn: MustDate("2024-02-14"), Port: "INBLR4",
				Product: "Machined brackets", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("72,00,000"), Quantity: MustQuantity("10100"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2024-05-02"), RealisedValue: MustMoney("72,00,000"), BRCNo: "BRC/24/008811"},

			{ShippingBillNo: "5566771", ShippedOn: MustDate("2024-07-08"), Port: "INBLR4",
				Product: "Machined aluminium housings", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("96,00,000"), Quantity: MustQuantity("13900"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2024-10-01"), RealisedValue: MustMoney("96,00,000"), BRCNo: "BRC/24/041277"},

			{ShippingBillNo: "5566772", ShippedOn: MustDate("2024-11-26"), Port: "INBLR4",
				Product: "Machined brackets", Buyer: "Sandvik Coromant AB",
				FOBValue: MustMoney("88,00,000"), Quantity: MustQuantity("11700"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2025-02-18"), RealisedValue: MustMoney("88,00,000"), BRCNo: "BRC/25/003194"},

			{ShippingBillNo: "5566773", ShippedOn: MustDate("2025-03-19"), Port: "INBLR4",
				Product:  "Hydraulic manifold blocks — a new line, not same or similar",
				Buyer:    "Bosch Rexroth AG",
				FOBValue: MustMoney("24,00,000"), Quantity: MustQuantity("1800"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes,
				RealisedOn:            MustDate("2025-06-05"), RealisedValue: MustMoney("24,00,000"), BRCNo: "BRC/25/019006"},

			{ShippingBillNo: "6677881", ShippedOn: MustDate("2025-06-30"), Port: "INBLR4",
				Product: "Machined aluminium housings", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("64,00,000"), Quantity: MustQuantity("9200"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2025-09-22"), RealisedValue: MustMoney("64,00,000"), BRCNo: "BRC/25/031885"},

			{ShippingBillNo: "6677882", ShippedOn: MustDate("2026-01-15"), Port: "INBLR4",
				Product: "Machined brackets", Buyer: "Sandvik Coromant AB",
				FOBValue: MustMoney("46,00,000"), Quantity: MustQuantity("6100"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2026-04-03"), RealisedValue: MustMoney("46,00,000"), BRCNo: "BRC/26/007741"},

			{ShippingBillNo: "7788991", ShippedOn: MustDate("2026-05-12"), Port: "INBLR4",
				Product: "Machined aluminium housings", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("58,00,000"), Quantity: MustQuantity("8300"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes},

			{ShippingBillNo: "7788992", ShippedOn: MustDate("2026-09-08"), Port: "INBLR4",
				Product: "Machined brackets", Buyer: "Sandvik Coromant AB",
				FOBValue: MustMoney("41,00,000"), Quantity: MustQuantity("5400"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes, SameOrSimilar: yes,
				RealisedOn: MustDate("2026-11-05"), RealisedValue: MustMoney("40,50,000"), BRCNo: "BRC/26/052310"},

			// Shipped, realised, and worth nothing to this licence: the
			// authorisation number never went on the shipping bill.
			{ShippingBillNo: "7788993", ShippedOn: MustDate("2026-10-20"), Port: "INBLR4",
				Product: "Machined aluminium housings", Buyer: "Rheinmetall Zulieferer GmbH",
				FOBValue: MustMoney("32,00,000"), Quantity: MustQuantity("4600"), QuantityUnit: "pieces",
				SameOrSimilar: yes},
		},

		Evidence: Evidence{
			RCMCHeld:        true,
			BondOrBGDetails: "Bond for ₹60,00,000 with 15% bank guarantee, Bengaluru Air Cargo Commissionerate",
		},

		Notes: "Worked example. The figures are invented; the shape of the problem is not. " +
			"Gross exports against this licence come to more than the obligation, and less than half of it is discharged.",
	}
}

// SampleAdvanceAuthorisation is a worked example of an Advance Authorisation
// part way through its period: inputs all imported at the start, exports
// trickling out, a value-addition figure that is meaningless until they are
// finished, and an obligation that is a quantity as well as a value.
func SampleAdvanceAuthorisation() Licence {
	yes := true
	return Licence{
		Scheme:          SchemeAdvanceAuthorisation,
		AuthorisationNo: "0710045612",
		FileNo:          "07/04/000/01188/AM27",
		Holder:          "Tiruppur Knitwear Exports",
		IEC:             "0491234567",
		Product:         "Cotton knitted T-shirts",
		Currency:        "INR",
		IssuedOn:        MustDate("2026-04-20"),

		NormsBasis: NormsSION,

		EOValueEndorsed: MustMoney("1,38,00,000"),
		EOQuantity:      MustQuantity("180000"),
		EOQuantityUnit:  "pieces",

		Imports: []Import{
			{
				Description:   "Combed cotton yarn, 30s",
				BillOfEntryNo: "9912345",
				Port:          "INTUT6",
				ClearedOn:     MustDate("2026-05-18"),
				CIFValue:      MustMoney("92,00,000"),
				DutySaved:     MustMoney("13,80,000"),
			},
		},

		Shipments: []Shipment{
			{ShippingBillNo: "1122331", ShippedOn: MustDate("2026-07-14"), Port: "INTUT6",
				Product: "Cotton knitted T-shirts", Buyer: "Primark Stores Ltd",
				FOBValue: MustMoney("41,00,000"), Quantity: MustQuantity("54000"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes,
				RealisedOn:            MustDate("2026-10-02"), RealisedValue: MustMoney("41,00,000"), BRCNo: "BRC/26/044120"},

			{ShippingBillNo: "1122332", ShippedOn: MustDate("2026-10-30"), Port: "INTUT6",
				Product: "Cotton knitted T-shirts", Buyer: "Primark Stores Ltd",
				FOBValue: MustMoney("37,50,000"), Quantity: MustQuantity("49000"), QuantityUnit: "pieces",
				AuthorisationEndorsed: yes},
		},

		Evidence: Evidence{RCMCHeld: true},

		Notes: "Worked example. The figures are invented. Note that the obligation has to be " +
			"discharged in pieces as well as in rupees, and that value addition means nothing " +
			"until the exports are finished.",
	}
}
