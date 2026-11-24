package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/epcg-advance-authorisation-export-obligation/internal/eo"
)

// api serves the JSON interface the page talks to. Every call is same-origin
// and nothing leaves the machine.
type api struct {
	store *store
}

// routes registers every endpoint.
func (a *api) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", a.getState)
	mux.HandleFunc("POST /api/assess", a.postAssess)
	mux.HandleFunc("GET /api/licences/{id}", a.getLicence)
	mux.HandleFunc("DELETE /api/licences/{id}", a.deleteLicence)
	mux.HandleFunc("PUT /api/policy", a.putPolicy)
	mux.HandleFunc("GET /api/samples", a.getSamples)
	mux.HandleFunc("POST /api/report.txt", a.postReport)
}

// licenceSummary is a saved licence as the list shows it, with the headline
// figures from a fresh assessment so that the list says which licences need
// attention without the page having to assess each one itself.
type licenceSummary struct {
	ID              string      `json:"id"`
	Label           string      `json:"label"`
	Scheme          eo.Scheme   `json:"scheme"`
	SchemeTitle     string      `json:"schemeTitle"`
	Holder          string      `json:"holder"`
	Product         string      `json:"product"`
	Currency        string      `json:"currency"`
	IssuedOn        eo.Date     `json:"issuedOn"`
	PeriodEnds      eo.Date     `json:"periodEnds"`
	DaysLeft        int         `json:"daysLeft"`
	Expired         bool        `json:"expired"`
	Urgency         eo.Urgency  `json:"urgency"`
	Required        eo.Money    `json:"required"`
	Counted         eo.Money    `json:"counted"`
	FulfilledPct    eo.Bips     `json:"fulfilledPct"`
	Shortfall       eo.Money    `json:"shortfall"`
	Status          eo.Status   `json:"status"`
	ExposureTotal   eo.Money    `json:"exposureTotal"`
	EODCReady       bool        `json:"eodcReady"`
	EODCBlocking    int         `json:"eodcBlocking"`
	Headline        eo.Headline `json:"headline"`
	AlertCount      int         `json:"alertCount"`
	UrgentAlerts    int         `json:"urgentAlerts"`
	NextBlockEnds   eo.Date     `json:"nextBlockEnds"`
	NextBlockNeeded eo.Money    `json:"nextBlockNeeded"`
	PolicyEdited    bool        `json:"policyEdited"`
}

func summarise(l eo.Licence, a eo.Assessment) licenceSummary {
	s := licenceSummary{
		ID:            l.ID,
		Label:         l.Label(),
		Scheme:        l.Scheme,
		SchemeTitle:   l.Scheme.Title(),
		Holder:        l.Holder,
		Product:       l.Product,
		Currency:      l.Currency,
		IssuedOn:      l.IssuedOn,
		PeriodEnds:    a.Period.Ends,
		DaysLeft:      a.Clock.DaysLeft,
		Expired:       a.Clock.Expired,
		Urgency:       a.Urgency,
		Required:      a.Specific.Required,
		Counted:       a.Specific.Counted,
		FulfilledPct:  a.Specific.FulfilledPct,
		Shortfall:     a.Specific.NetShortfall,
		Status:        a.Specific.Status,
		ExposureTotal: a.Exposure.Total,
		EODCReady:     a.EODC.Ready,
		EODCBlocking:  a.EODC.BlockingCount,
		Headline:      a.Headline,
		AlertCount:    len(a.Alerts),
		PolicyEdited:  a.PolicyEdited,
	}
	for _, al := range a.Alerts {
		if al.Level == eo.AlertUrgent {
			s.UrgentAlerts++
		}
	}
	// The next milestone that is still open, which is what a list of licences
	// should be sorted and read by.
	for _, b := range a.Blocks {
		if b.Closed || b.Shortfall == 0 {
			continue
		}
		s.NextBlockEnds, s.NextBlockNeeded = b.Window.To, b.Shortfall
		break
	}
	return s
}

// statePayload is everything the page needs to draw itself.
type statePayload struct {
	Today         eo.Date          `json:"today"`
	DefaultPolicy eo.Policy        `json:"defaultPolicy"`
	Citations     []eo.Citation    `json:"citations"`
	Licences      []licenceSummary `json:"licences"`
	DataFile      string           `json:"dataFile"`
	PolicyEdition string           `json:"policyEdition"`
	PolicyAsAt    string           `json:"policyAsAt"`
}

// getState answers with the figures in force, the saved licences and where the
// data file is.
func (a *api) getState(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	today := eo.Today()

	out := make([]licenceSummary, 0, len(data.Licences))
	for _, l := range data.Licences {
		out = append(out, summarise(l, eo.Assess(l, today)))
	}

	writeJSON(w, http.StatusOK, statePayload{
		Today:         today,
		DefaultPolicy: data.DefaultPolicy.Normalise(),
		Citations:     data.DefaultPolicy.Citations(),
		Licences:      out,
		DataFile:      a.store.path,
		PolicyEdition: eo.PolicyEdition,
		PolicyAsAt:    eo.PolicyFiguresCompiledOn,
	})
}

// assessRequest is a licence to assess, optionally saved at the same time, and
// optionally as at a day other than today.
type assessRequest struct {
	Licence eo.Licence `json:"licence"`
	Save    bool       `json:"save"`
	AsOf    eo.Date    `json:"asOf"`
}

// assessResponse is the licence as it stands and everything the package has to
// say about it.
type assessResponse struct {
	Licence    eo.Licence     `json:"licence"`
	Assessment eo.Assessment  `json:"assessment"`
	Summary    licenceSummary `json:"summary"`
	Saved      bool           `json:"saved"`
}

// postAssess assesses a licence. Nothing is written unless the request asks for
// it, so the page can recompute as the user types.
func (a *api) postAssess(w http.ResponseWriter, r *http.Request) {
	var in assessRequest
	if !readJSON(w, r, &in) {
		return
	}

	l := a.withPolicy(in.Licence)
	if err := l.Validate(); err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	if in.Save {
		saved, err := a.store.saveLicence(l)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, err)
			return
		}
		l = saved
	}

	asOf := in.AsOf
	if asOf.Zero() {
		asOf = eo.Today()
	}
	assessment := eo.Assess(l, asOf)
	writeJSON(w, http.StatusOK, assessResponse{
		Licence:    l,
		Assessment: assessment,
		Summary:    summarise(l, assessment),
		Saved:      in.Save,
	})
}

// withPolicy normalises an incoming licence, giving it the store's default
// figures if it arrived without any of its own.
//
// The test is made on the RAW licence, before normalising, because Normalise
// fills a blank policy in with the published figures — so a check made
// afterwards would never fire, and a user who had changed the defaults would
// silently get the published ones back.
func (a *api) withPolicy(in eo.Licence) eo.Licence {
	if in.Policy.Edition == "" {
		in.Policy = a.store.defaultPolicy()
	}
	return in.Normalise()
}

// getLicence answers with one saved licence and a fresh assessment of it.
func (a *api) getLicence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	saved, ok := a.store.getLicence(id)
	if !ok {
		writeProblem(w, http.StatusNotFound, fmt.Errorf("there is no licence %q saved", id))
		return
	}
	assessment := eo.Assess(saved, eo.Today())
	writeJSON(w, http.StatusOK, assessResponse{
		Licence:    saved,
		Assessment: assessment,
		Summary:    summarise(saved, assessment),
		Saved:      true,
	})
}

// deleteLicence removes a saved licence.
func (a *api) deleteLicence(w http.ResponseWriter, r *http.Request) {
	if err := a.store.deleteLicence(r.PathValue("id")); err != nil {
		writeProblem(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": r.PathValue("id")})
}

// putPolicy replaces the figures a NEW licence starts with. It deliberately
// does not touch a licence already saved: an authorisation goes on being
// governed by the figures that applied to it.
func (a *api) putPolicy(w http.ResponseWriter, r *http.Request) {
	var in eo.Policy
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putDefaultPolicy(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"defaultPolicy": saved,
		"citations":     saved.Citations(),
		"note":          "These figures apply to licences created from now on. A licence already saved keeps the figures it was assessed under, because an authorisation goes on being governed by the Policy that applied to it.",
	})
}

// getSamples answers with the worked examples, so that somebody opening the
// tool for the first time can see what it does before typing anything.
func (a *api) getSamples(w http.ResponseWriter, r *http.Request) {
	today := eo.Today()
	type sample struct {
		Key        string        `json:"key"`
		Title      string        `json:"title"`
		Licence    eo.Licence    `json:"licence"`
		Assessment eo.Assessment `json:"assessment"`
	}
	out := []sample{}
	for _, s := range []struct {
		key, title string
		licence    eo.Licence
	}{
		{"epcg", "Worked example — EPCG, a machining unit that was already exporting", eo.SampleEPCG()},
		{"advance-authorisation", "Worked example — Advance Authorisation, knitwear part way through", eo.SampleAdvanceAuthorisation()},
	} {
		l := s.licence.Normalise()
		l.ID = "" // not saved; the page saves it if the user wants it
		out = append(out, sample{s.key, s.title, l, eo.Assess(l, today)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"samples": out})
}

// postReport returns the written statement for a licence as plain text, which
// is what an exporter prints, files, or sends their consultant.
func (a *api) postReport(w http.ResponseWriter, r *http.Request) {
	var in assessRequest
	if !readJSON(w, r, &in) {
		return
	}
	l := a.withPolicy(in.Licence)
	if err := l.Validate(); err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	asOf := in.AsOf
	if asOf.Zero() {
		asOf = eo.Today()
	}
	body := eo.Report(l, eo.Assess(l, asOf))

	name := idSafe(l.Label())
	if name == "" {
		name = "export-obligation"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`-export-obligation.txt"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

// ---------- plumbing ----------

// readJSON reads a request body into v, capping its size and reporting a
// readable problem rather than a parser's complaint. It answers the request
// itself on failure and returns false.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge,
			errors.New("that is more than this tool reads in one request"))
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeProblem(w, http.StatusBadRequest, errors.New("the request was empty"))
		return false
	}

	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		writeProblem(w, http.StatusBadRequest, fmt.Errorf("the request could not be read: %w", err))
		return false
	}
	if decoder.More() {
		writeProblem(w, http.StatusBadRequest, errors.New("the request carried more than one object"))
		return false
	}
	return true
}

// writeJSON answers with a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(v)
}

// writeProblem answers with an error the page can show to the user as it
// stands.
func writeProblem(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"problem": err.Error()})
}
