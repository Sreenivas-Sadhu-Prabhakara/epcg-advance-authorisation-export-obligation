package eo

import (
	"encoding/json"
	"testing"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Money // minor units
		err  bool
	}{
		{"plain rupees", "1250000", 125000000, false},
		{"with paise", "1250000.50", 125000050, false},
		{"one decimal place is read as tenths", "10.5", 1050, false},
		{"indian grouping", "12,50,000.00", 125000000, false},
		{"western grouping", "1,250,000.00", 125000000, false},
		{"rupee symbol", "₹1,25,00,000", 1250000000, false},
		{"currency code beside the figure", "INR 42,00,000.75", 420000075, false},
		{"Rs prefix", "Rs. 5,000", 500000, false},
		{"leading dot", ".25", 25, false},
		{"zero", "0", 0, false},
		{"empty is absent, not an error", "", 0, false},
		{"spaces only", "   ", 0, false},
		{"negative", "-1500.25", -150025, false},
		{"three decimals refused rather than rounded", "100.456", 0, true},
		{"two dots", "1.2.3", 0, true},
		{"two minus signs", "--5", 0, true},
		{"not a number", "about four lakh", 0, true},
		{"a stray symbol", "100#", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseMoney(c.in)
			if c.err {
				if err == nil {
					t.Fatalf("ParseMoney(%q) should have failed, got %d", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMoney(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseMoney(%q) = %d minor units, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestMoneyRendering(t *testing.T) {
	cases := []struct {
		name   string
		m      Money
		plain  string
		indian string
		words  string
	}{
		{"zero", 0, "0.00", "0.00", "0.00"},
		{"a few rupees", 12345, "123.45", "123.45", "123.45"},
		{"a thousand", 100000, "1000.00", "1,000.00", "1,000.00"},
		{"a lakh", 10000000, "100000.00", "1,00,000.00", "1 lakh"},
		{"eighteen and a half lakh", 185000000, "1850000.00", "18,50,000.00", "18.5 lakh"},
		{"a crore", 1000000000, "10000000.00", "1,00,00,000.00", "1 crore"},
		{"one point two four crore", 1240000000, "12400000.00", "1,24,00,000.00", "1.24 crore"},
		{"a negative balance", -185000000, "-1850000.00", "-18,50,000.00", "-18.5 lakh"},
		{"a small negative", -12345, "-123.45", "-123.45", "-123.45"},
		{"paise survive", 1, "0.01", "0.01", "0.01"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.m.String(); got != c.plain {
				t.Fatalf("String = %q, want %q", got, c.plain)
			}
			if got := c.m.Indian(); got != c.indian {
				t.Fatalf("Indian = %q, want %q", got, c.indian)
			}
			if got := c.m.Words(); got != c.words {
				t.Fatalf("Words = %q, want %q", got, c.words)
			}
		})
	}
	if got := Money(125000000).Major(); got != 1250000 {
		t.Fatalf("Major = %d, want 1250000", got)
	}
}

func TestMoneyJSONRoundTrip(t *testing.T) {
	type holder struct {
		Duty Money `json:"duty"`
	}

	var h holder
	if err := json.Unmarshal([]byte(`{"duty":"12,50,000.50"}`), &h); err != nil {
		t.Fatalf("unmarshal a quoted grouped amount: %v", err)
	}
	if h.Duty != 125000050 {
		t.Fatalf("duty = %d, want 125000050", h.Duty)
	}

	out, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `{"duty":"1250000.50"}` {
		t.Fatalf("marshal gave %s", out)
	}

	// A bare JSON number is accepted and read through the same parser.
	var bare holder
	if err := json.Unmarshal([]byte(`{"duty":1250000.5}`), &bare); err != nil {
		t.Fatalf("unmarshal a bare number: %v", err)
	}
	if bare.Duty != 125000050 {
		t.Fatalf("bare duty = %d, want 125000050", bare.Duty)
	}

	// null is absent.
	var none holder
	if err := json.Unmarshal([]byte(`{"duty":null}`), &none); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if none.Duty != 0 {
		t.Fatalf("null duty = %d, want 0", none.Duty)
	}

	// Three decimal places are refused through JSON as well as through the
	// parser, so a data file cannot smuggle in a rounded figure.
	var bad holder
	if err := json.Unmarshal([]byte(`{"duty":"100.456"}`), &bad); err == nil {
		t.Fatalf("three decimal places should be refused")
	}
	if err := json.Unmarshal([]byte(`{"duty":true}`), &bad); err == nil {
		t.Fatalf("a boolean is not an amount")
	}
}

func TestMoneyMulDivIsExactAndRoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		name     string
		m        Money
		num, den int64
		want     Money
	}{
		{"six times duty saved", 1250000000, 6, 1, 7500000000},
		{"four and a half times, expressed in hundredths", 1250000000, 450, 100, 5625000000},
		{"half of an odd number of paise rounds up", 3, 1, 2, 2},
		{"half of a negative rounds away from zero", -3, 1, 2, -2},
		{"a third", 100, 1, 3, 33},
		{"two thirds", 100, 2, 3, 67},
		{"a zero denominator gives nothing rather than panicking", 100, 1, 0, 0},
		{"zero times anything", 0, 6, 1, 0},
		{"a figure big enough to overflow a naive 64-bit multiply",
			// 50 crore rupees in paise, times 15% times 3650 days: the
			// intermediate product is beyond what int64 would hold if it were
			// formed before the division.
			5000000000000, 1500 * 3650, 10000 * 365, 7500000000000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.m.MulDiv(c.num, c.den); got != c.want {
				t.Fatalf("%d.MulDiv(%d,%d) = %d, want %d", c.m, c.num, c.den, got, c.want)
			}
		})
	}
}

func TestBips(t *testing.T) {
	cases := []struct {
		name     string
		b        Bips
		pct      string
		multiple string
		of       Money
		want     Money
	}{
		{"fifteen percent interest", Percent(15, 0), "15%", "0.15 times", 100000000, 15000000},
		{"half a percent composition fee", Percent(0, 50), "0.5%", "0.01 times", 100000000, 500000},
		{"six times", Times(600), "600%", "6 times", 1250000000, 7500000000},
		{"four and a half times", Times(450), "450%", "4.5 times", 1250000000, 5625000000},
		{"one and a half times", Times(150), "150%", "1.5 times", 1250000000, 1875000000},
		{"five point four times", Times(540), "540%", "5.4 times", 1250000000, 6750000000},
		{"nothing", 0, "0%", "0 times", 1250000000, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.b.Pct(); got != c.pct {
				t.Fatalf("Pct = %q, want %q", got, c.pct)
			}
			if got := c.b.Multiple(); got != c.multiple {
				t.Fatalf("Multiple = %q, want %q", got, c.multiple)
			}
			if got := c.b.Of(c.of); got != c.want {
				t.Fatalf("%s of %s = %s, want %s", c.pct, c.of, got, c.want)
			}
		})
	}
}

func TestProportion(t *testing.T) {
	cases := []struct {
		name        string
		part, whole Money
		want        Bips
		pct         string
	}{
		{"half", 50, 100, 5000, "50%"},
		{"nothing of something", 0, 100, 0, "0%"},
		{"all of it", 100, 100, 10000, "100%"},
		{"more than all of it", 150, 100, 15000, "150%"},
		{"a third", 100, 300, 3333, "33.33%"},
		{"two thirds", 200, 300, 6667, "66.67%"},
		{"nothing to divide by reports nothing, not everything", 100, 0, 0, "0%"},
		{"a realistic shortfall: 5.6 crore of a 7.5 crore obligation", 560000000, 750000000, 7467, "74.67%"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Proportion(c.part, c.whole)
			if got != c.want {
				t.Fatalf("Proportion(%d,%d) = %d bips, want %d", c.part, c.whole, got, c.want)
			}
			if got.Pct() != c.pct {
				t.Fatalf("rendered %q, want %q", got.Pct(), c.pct)
			}
		})
	}

	if got := ProportionOfCount(3, 4); got != 7500 {
		t.Fatalf("ProportionOfCount(3,4) = %d, want 7500", got)
	}
	if got := ProportionOfCount(1, 0); got != 0 {
		t.Fatalf("ProportionOfCount(1,0) = %d, want 0", got)
	}
}

func TestParseQuantity(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Quantity
		err  bool
	}{
		{"whole units", "1200", 1200000, false},
		{"three decimals", "1.234", 1234, false},
		{"two decimals", "12.50", 12500, false},
		{"grouped", "1,200.500", 1200500, false},
		{"empty is absent", "", 0, false},
		{"zero", "0", 0, false},
		{"five grams of a kilogram", "0.005", 5, false},
		{"negative", "-3.5", -3500, false},
		{"four decimals refused", "1.2345", 0, true},
		{"not a quantity", "a dozen", 0, true},
		{"two dots", "1.2.3", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseQuantity(c.in)
			if c.err {
				if err == nil {
					t.Fatalf("ParseQuantity(%q) should have failed, got %d", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseQuantity(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseQuantity(%q) = %d thousandths, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestQuantityRenderingAndJSON(t *testing.T) {
	cases := []struct {
		q    Quantity
		want string
	}{
		{0, "0"},
		{1234, "1.234"},
		{1200000, "1200"},
		{12500, "12.5"},
		{5, "0.005"},
		{-3500, "-3.5"},
	}
	for _, c := range cases {
		if got := c.q.String(); got != c.want {
			t.Fatalf("Quantity(%d).String() = %q, want %q", c.q, got, c.want)
		}
	}

	type holder struct {
		Q Quantity `json:"q"`
	}
	var h holder
	if err := json.Unmarshal([]byte(`{"q":"1,200.500"}`), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.Q != 1200500 {
		t.Fatalf("q = %d, want 1200500", h.Q)
	}
	out, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `{"q":"1200.5"}` {
		t.Fatalf("marshal gave %s", out)
	}
	var bare holder
	if err := json.Unmarshal([]byte(`{"q":1200.5}`), &bare); err != nil {
		t.Fatalf("unmarshal a bare number: %v", err)
	}
	if bare.Q != 1200500 {
		t.Fatalf("bare q = %d, want 1200500", bare.Q)
	}
	var none holder
	if err := json.Unmarshal([]byte(`{"q":null}`), &none); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if none.Q != 0 {
		t.Fatalf("null q = %d", none.Q)
	}
	var bad holder
	if err := json.Unmarshal([]byte(`{"q":"1.2345"}`), &bad); err == nil {
		t.Fatalf("four decimal places should be refused")
	}
}

func TestProportionOfQuantity(t *testing.T) {
	cases := []struct {
		name        string
		part, whole Quantity
		want        Bips
	}{
		{"three quarters of a tonnage obligation", 900000, 1200000, 7500},
		{"exactly met", 1200000, 1200000, 10000},
		{"nothing shipped", 0, 1200000, 0},
		{"no obligation to measure against", 900000, 0, 0},
		{"a hair under, which must not round to 100%", 1199000, 1200000, 9992},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ProportionOfQuantity(c.part, c.whole); got != c.want {
				t.Fatalf("ProportionOfQuantity(%d,%d) = %d, want %d", c.part, c.whole, got, c.want)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	if got := plural(1, "day", "days"); got != "1 day" {
		t.Fatalf("got %q", got)
	}
	if got := plural(0, "day", "days"); got != "0 days" {
		t.Fatalf("got %q", got)
	}
	if got := plural(2, "day", "days"); got != "2 days" {
		t.Fatalf("got %q", got)
	}
}
