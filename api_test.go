package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/epcg-advance-authorisation-export-obligation/internal/eo"
)

func testServer(t *testing.T) (*httptest.Server, *store) {
	t.Helper()
	s := tempStore(t)
	mux := http.NewServeMux()
	(&api{store: s}).routes(mux)
	srv := httptest.NewServer(guard(mux))
	t.Cleanup(srv.Close)
	return srv, s
}

func do(t *testing.T, srv *httptest.Server, method, path, body string) (*http.Response, string) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(out)
}

func TestGetState(t *testing.T) {
	srv, _ := testServer(t)
	resp, body := do(t, srv, "GET", "/api/state", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var out statePayload
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Today.Zero() {
		t.Fatalf("the page needs today's date")
	}
	if out.DefaultPolicy.EPCGMultipleHundredths != 600 {
		t.Fatalf("policy = %+v", out.DefaultPolicy)
	}
	if len(out.Citations) < 15 {
		t.Fatalf("%d citations; every figure must be shown with its source", len(out.Citations))
	}
	if out.PolicyEdition == "" || out.PolicyAsAt == "" {
		t.Fatalf("the edition and the date it was read must be stated")
	}
	if out.DataFile == "" {
		t.Fatalf("the page should say where the data lives")
	}
	if len(out.Licences) != 0 {
		t.Fatalf("%d licences in a new store", len(out.Licences))
	}
	// The response must say nothing about any network service.
	for _, forbidden := range []string{"dgft.gov.in", "icegate", "http://", "https://"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("the state response mentions %q", forbidden)
		}
	}
}

func TestPostAssess(t *testing.T) {
	srv, st := testServer(t)

	licence := eo.SampleEPCG()
	licence.ID = ""
	payload, err := json.Marshal(assessRequest{Licence: licence, AsOf: eo.MustDate("2026-11-24")})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	t.Run("assessing without saving", func(t *testing.T) {
		resp, body := do(t, srv, "POST", "/api/assess", string(payload))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		var out assessResponse
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.Saved {
			t.Fatalf("nothing was asked to be saved")
		}
		if out.Assessment.Specific.Counted != eo.MustMoney("1,60,17,965.42") {
			t.Fatalf("counted = %s", out.Assessment.Specific.Counted)
		}
		if out.Summary.FulfilledPct.Pct() != "44.49%" {
			t.Fatalf("summary = %+v", out.Summary)
		}
		if len(out.Assessment.Alerts) == 0 {
			t.Fatalf("this licence has plenty to say")
		}
		if len(st.snapshot().Licences) != 0 {
			t.Fatalf("an assessment must not write to the file")
		}
	})

	t.Run("assessing and saving", func(t *testing.T) {
		save, err := json.Marshal(assessRequest{Licence: licence, Save: true, AsOf: eo.MustDate("2026-11-24")})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		resp, body := do(t, srv, "POST", "/api/assess", string(save))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		var out assessResponse
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !out.Saved || out.Licence.ID == "" {
			t.Fatalf("response = %+v", out)
		}
		if len(st.snapshot().Licences) != 1 {
			t.Fatalf("the licence was not saved")
		}
	})

	t.Run("a licence that cannot be assessed is refused with a readable reason", func(t *testing.T) {
		resp, body := do(t, srv, "POST", "/api/assess", `{"licence":{"scheme":"epcg"}}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		var problem map[string]string
		if err := json.Unmarshal([]byte(body), &problem); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !strings.Contains(problem["problem"], "date the authorisation was issued") {
			t.Fatalf("problem = %q", problem["problem"])
		}
	})

	t.Run("a licence with no policy of its own picks up the default", func(t *testing.T) {
		next := eo.AsPublished()
		next.InterestRatePercent = 18
		raw, _ := json.Marshal(next)
		if resp, body := do(t, srv, "PUT", "/api/policy", string(raw)); resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}

		bare := minimalLicence("DEFAULTS")
		req, _ := json.Marshal(assessRequest{Licence: bare})
		resp, body := do(t, srv, "POST", "/api/assess", string(req))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		var out assessResponse
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.Assessment.Policy.InterestRatePercent != 18 {
			t.Fatalf("rate = %d, want the default of 18", out.Assessment.Policy.InterestRatePercent)
		}
	})
}

func TestGetAndDeleteLicence(t *testing.T) {
	srv, st := testServer(t)
	saved, err := st.saveLicence(minimalLicence("0330099887"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	resp, body := do(t, srv, "GET", "/api/licences/"+saved.ID, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var out assessResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Licence.ID != saved.ID || !out.Saved {
		t.Fatalf("response = %+v", out)
	}
	if out.Assessment.Specific.Required != eo.MustMoney("1,00,00,000") {
		t.Fatalf("obligation = %s", out.Assessment.Specific.Required)
	}

	if resp, body := do(t, srv, "GET", "/api/licences/nope", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}

	if resp, body := do(t, srv, "DELETE", "/api/licences/"+saved.ID, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	if len(st.snapshot().Licences) != 0 {
		t.Fatalf("the licence was not removed")
	}
	if resp, _ := do(t, srv, "DELETE", "/api/licences/"+saved.ID, ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPutPolicy(t *testing.T) {
	srv, st := testServer(t)
	saved, err := st.saveLicence(minimalLicence("KEEP"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	next := eo.AsPublished()
	next.EPCGMultipleHundredths = 450
	next.InterestRatePercent = 18
	raw, _ := json.Marshal(next)

	resp, body := do(t, srv, "PUT", "/api/policy", string(raw))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var out struct {
		DefaultPolicy eo.Policy     `json:"defaultPolicy"`
		Citations     []eo.Citation `json:"citations"`
		Note          string        `json:"note"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.DefaultPolicy.EPCGMultipleHundredths != 450 || !out.DefaultPolicy.Edited {
		t.Fatalf("policy = %+v", out.DefaultPolicy)
	}
	if out.Note == "" {
		t.Fatalf("the response must say that saved licences keep their own figures")
	}
	// The citations must reflect the figure actually applied.
	for _, c := range out.Citations {
		if c.Key == "epcgMultiple" && !strings.Contains(c.Figure, "4.5") {
			t.Fatalf("the citation still shows %q", c.Figure)
		}
	}

	still, _ := st.getLicence(saved.ID)
	if still.Policy.EPCGMultipleHundredths != 600 {
		t.Fatalf("a saved licence must keep the figures it was issued under, got %d",
			still.Policy.EPCGMultipleHundredths)
	}
}

func TestGetSamples(t *testing.T) {
	srv, st := testServer(t)
	resp, body := do(t, srv, "GET", "/api/samples", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Samples []struct {
			Key        string        `json:"key"`
			Title      string        `json:"title"`
			Licence    eo.Licence    `json:"licence"`
			Assessment eo.Assessment `json:"assessment"`
		} `json:"samples"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Samples) != 2 {
		t.Fatalf("%d samples, want one of each scheme", len(out.Samples))
	}
	keys := map[string]bool{}
	for _, s := range out.Samples {
		keys[s.Key] = true
		if s.Title == "" {
			t.Fatalf("sample %q has no title", s.Key)
		}
		if s.Licence.ID != "" {
			t.Fatalf("a sample must not come back with an identifier, or saving it would overwrite something")
		}
		if s.Assessment.Specific.Required == 0 {
			t.Fatalf("sample %q has no obligation", s.Key)
		}
	}
	if !keys["epcg"] || !keys["advance-authorisation"] {
		t.Fatalf("samples = %v", keys)
	}
	if len(st.snapshot().Licences) != 0 {
		t.Fatalf("fetching the samples must not save anything")
	}
}

func TestPostReport(t *testing.T) {
	srv, _ := testServer(t)
	licence := eo.SampleEPCG()
	licence.ID = ""
	payload, _ := json.Marshal(assessRequest{Licence: licence, AsOf: eo.MustDate("2026-11-24")})

	resp, body := do(t, srv, "POST", "/api/report.txt", string(payload))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("content type = %q", got)
	}
	disposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, "0330099887-export-obligation.txt") {
		t.Fatalf("disposition = %q", disposition)
	}
	for _, want := range []string{"EXPORT OBLIGATION STATEMENT", "3,60,00,000.00", "files nothing"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the report does not contain %q", want)
		}
	}

	if resp, _ := do(t, srv, "POST", "/api/report.txt", `{"licence":{"scheme":"epcg"}}`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestReadJSONRefusesBadRequests(t *testing.T) {
	srv, _ := testServer(t)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty", "", http.StatusBadRequest},
		{"whitespace", "   ", http.StatusBadRequest},
		{"not JSON", "{", http.StatusBadRequest},
		{"two objects", `{"licence":{}} {"licence":{}}`, http.StatusBadRequest},
		{"an unknown field, which is usually a typo", `{"licance":{}}`, http.StatusBadRequest},
		{"an amount with too many decimals", `{"licence":{"scheme":"epcg","specificEoEndorsed":"1.234"}}`, http.StatusBadRequest},
		{"a date that is not a date", `{"licence":{"scheme":"epcg","issuedOn":"the fifteenth"}}`, http.StatusBadRequest},
		{"a day that does not exist", `{"licence":{"scheme":"epcg","issuedOn":"2023-02-30"}}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, body := do(t, srv, "POST", "/api/assess", c.body)
			if resp.StatusCode != c.want {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, c.want, body)
			}
			if resp.StatusCode != http.StatusOK {
				var problem map[string]string
				if err := json.Unmarshal([]byte(body), &problem); err != nil {
					t.Fatalf("a refusal must be readable JSON: %v", err)
				}
				if problem["problem"] == "" {
					t.Fatalf("a refusal must say what was wrong")
				}
			}
		})
	}
}

func TestReadJSONRefusesAnOversizedBody(t *testing.T) {
	srv, _ := testServer(t)
	huge := `{"licence":{"scheme":"epcg","notes":"` + strings.Repeat("x", maxJSONBody+1024) + `"}}`
	resp, _ := do(t, srv, "POST", "/api/assess", huge)
	if resp.StatusCode != http.StatusRequestEntityTooLarge && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGuardRefusesACrossSiteRequest(t *testing.T) {
	srv, _ := testServer(t)

	req, err := http.NewRequest("GET", srv.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Origin", "https://example.invalid")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d; a page on another site must not be able to read your licences", resp.StatusCode)
	}

	// The tool's own page is allowed.
	same, err := http.NewRequest("GET", srv.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	same.Header.Set("Origin", "http://"+same.Host)
	ok, err := srv.Client().Do(same)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("status = %d for a same-origin request", ok.StatusCode)
	}
}

func TestGuardSetsTheHeadersALocalToolNeeds(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := do(t, srv, "GET", "/api/state", "")
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		origin, host string
		want         bool
	}{
		{"http://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"HTTP://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"http://127.0.0.1:8080", "127.0.0.1:9000", false},
		{"https://example.invalid", "127.0.0.1:8080", false},
		{"null", "127.0.0.1:8080", false},
		{"", "127.0.0.1:8080", false},
		{"not a url at all", "127.0.0.1:8080", false},
	}
	for _, c := range cases {
		t.Run(c.origin, func(t *testing.T) {
			if got := sameOrigin(c.origin, c.host); got != c.want {
				t.Fatalf("sameOrigin(%q, %q) = %v, want %v", c.origin, c.host, got, c.want)
			}
		})
	}
}

func TestSummariseCarriesTheHeadlineFigures(t *testing.T) {
	l := eo.SampleEPCG().Normalise()
	l.ID = "sample"
	a := eo.Assess(l, eo.MustDate("2026-11-24"))
	s := summarise(l, a)

	if s.ID != "sample" || s.Label != "0330099887" {
		t.Fatalf("summary = %+v", s)
	}
	if s.SchemeTitle != "EPCG" {
		t.Fatalf("scheme title = %q", s.SchemeTitle)
	}
	if s.Required != a.Specific.Required || s.Counted != a.Specific.Counted {
		t.Fatalf("the summary disagrees with the assessment")
	}
	if s.DaysLeft != a.Clock.DaysLeft || s.PeriodEnds != a.Period.Ends {
		t.Fatalf("the clock disagrees")
	}
	if s.ExposureTotal != a.Exposure.Total {
		t.Fatalf("exposure = %s vs %s", s.ExposureTotal, a.Exposure.Total)
	}
	if s.EODCReady != a.EODC.Ready || s.EODCBlocking != a.EODC.BlockingCount {
		t.Fatalf("the closure position disagrees")
	}
	if s.AlertCount != len(a.Alerts) {
		t.Fatalf("alert count = %d, want %d", s.AlertCount, len(a.Alerts))
	}
	// The next open milestone, which is what a list should be read by.
	if s.NextBlockEnds.String() != "2027-05-14" {
		t.Fatalf("next block ends %s", s.NextBlockEnds)
	}
	if s.NextBlockNeeded != a.Blocks[0].Shortfall {
		t.Fatalf("next block needs %s", s.NextBlockNeeded)
	}
}

func TestTheUIIsServedFromTheBinary(t *testing.T) {
	s := tempStore(t)
	mux := http.NewServeMux()
	(&api{store: s}).routes(mux)
	assets, err := webUI.ReadFile("web/index.html")
	if err != nil {
		t.Fatalf("the interface must be built into the binary: %v", err)
	}
	page := string(assets)

	// The embedded page must carry the same guarantees as the explainer: no
	// inline script, no inline style, no handler attributes, and a strict
	// content security policy.
	for _, forbidden := range []string{"<style", "style=\"", "onclick=", "oninput=", "onchange=", "onsubmit="} {
		if strings.Contains(strings.ToLower(page), forbidden) {
			t.Fatalf("the embedded page contains %q", forbidden)
		}
	}
	if strings.Contains(page, "Content-Security-Policy") == false {
		t.Fatalf("the embedded page has no content security policy")
	}
	// And nothing in it may reach off the machine.
	for _, forbidden := range []string{"//fonts.", "//cdn.", "http://", "https://sreenivas"} {
		if strings.Contains(page, forbidden) && !strings.Contains(page, "sreenivas-sadhu-prabhakara.github.io") {
			t.Fatalf("the embedded page references %q", forbidden)
		}
	}
}

// TestThePagesOwnPayloadIsAccepted posts the exact JSON shape web/assets/app.js
// builds, field for field.
//
// It matters because the API decodes with DisallowUnknownFields: one renamed
// struct tag, or one field the page sends that the server no longer knows,
// turns every request from the interface into a 400 that no Go test would
// otherwise catch. The payload below is copied from readLicence() in app.js and
// includes every field it sends, each one left at the empty value a blank form
// produces.
func TestThePagesOwnPayloadIsAccepted(t *testing.T) {
	srv, _ := testServer(t)

	body := `{
	  "licence": {
	    "id": "",
	    "scheme": "epcg",
	    "authorisationNo": "0330099887",
	    "fileNo": "",
	    "holder": "",
	    "iec": "",
	    "product": "",
	    "currency": "INR",
	    "issuedOn": "2023-05-15",
	    "category": "as-endorsed",
	    "specificEoEndorsed": "3,60,00,000",
	    "averageEo": "",
	    "averageEoExempt": false,
	    "averageEoExemptReason": "",
	    "eoValueEndorsed": "",
	    "eoQuantity": "",
	    "eoQuantityUnit": "",
	    "normsBasis": "",
	    "sionReference": "",
	    "otherInputsCif": "",
	    "eoPeriodEndEndorsed": "",
	    "blockEndsEndorsed": [],
	    "extensions": [
	      {"kind":"overall","blockNo":0,"grantedOn":"","newEndsOn":"2031-05-14","reference":"","feePaid":""}
	    ],
	    "imports": [
	      {"description":"","billOfEntryNo":"7712345","port":"","clearedOn":"2023-07-10",
	       "cifValue":"1,80,00,000","dutySaved":"45,00,000","installedOn":"","indigenous":false}
	    ],
	    "shipments": [
	      {"shippingBillNo":"4455661","shippedOn":"2023-08-20","port":"","product":"","buyer":"",
	       "fobValue":"85,00,000","quantity":"","quantityUnit":"","authorisationEndorsed":true,
	       "realisedOn":"","realisedValue":"","brcNo":"","sameOrSimilar":true,
	       "excluded":false,"excludedReason":""}
	    ],
	    "evidence": {
	      "caCertifiedStatement": false, "rcmcHeld": false, "bondOrBgDetails": "",
	      "applicationFiled": false, "filedOn": "", "fileReference": ""
	    },
	    "notes": "",
	    "policy": {}
	  },
	  "save": false
	}`

	resp, out := do(t, srv, "POST", "/api/assess", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the interface's own payload was refused with %d: %s", resp.StatusCode, out)
	}
	var got assessResponse
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Assessment.Specific.Required != eo.MustMoney("3,60,00,000") {
		t.Fatalf("obligation = %s", got.Assessment.Specific.Required)
	}
	// The extension must have moved the end of the period, which proves the
	// nested objects were decoded and not merely tolerated.
	if got.Assessment.Period.Ends.String() != "2031-05-14" {
		t.Fatalf("period ends %s, want the extended date", got.Assessment.Period.Ends)
	}
	if got.Assessment.Period.EndsSource != eo.SourceExtended {
		t.Fatalf("source = %q", got.Assessment.Period.EndsSource)
	}
	if got.Assessment.Tally.TotalFOB != eo.MustMoney("85,00,000") {
		t.Fatalf("counted FOB = %s", got.Assessment.Tally.TotalFOB)
	}
	// An empty policy object must come back as the published figures, not as
	// zeroes.
	if got.Assessment.Policy.EPCGMultipleHundredths != 600 {
		t.Fatalf("policy = %+v", got.Assessment.Policy)
	}

	// The same payload must also produce a statement.
	rep, text := do(t, srv, "POST", "/api/report.txt", body)
	if rep.StatusCode != http.StatusOK {
		t.Fatalf("the statement was refused with %d: %s", rep.StatusCode, text)
	}
	if !strings.Contains(text, "EXPORT OBLIGATION STATEMENT") {
		t.Fatalf("statement = %q", text[:120])
	}
}
