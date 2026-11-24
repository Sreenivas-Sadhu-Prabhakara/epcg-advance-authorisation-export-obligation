# Export obligation and EODC tracker — EPCG and Advance Authorisation

A free, offline, single-binary tool for Indian manufacturer-exporters holding an **EPCG** or **Advance Authorisation** licence. It tracks the export obligation attached to the licence: the duty saved, the obligation, the exports that actually count toward it, the block-wise and overall fulfilment, the days left, and the **duty plus interest** that becomes payable if the licence closes short.

It serves a small web interface on the machine it runs on and keeps your licences in one JSON file beside the binary. **It makes no outbound network calls of any kind.**

Explainer site: <https://sreenivas-sadhu-prabhakara.github.io/epcg-advance-authorisation-export-obligation/>

## What it does

You imported a machine at nil duty and owe six times the duty saved in exports over six years. You have shipped more than that, and you are still less than half way — because an EPCG obligation is discharged **over and above** the average level of your own exports of the same and similar products in the preceding three licensing years, and that average has to be maintained in every financial year. Only the *excess* counts, year by year. That is the most expensive misunderstanding in the scheme, and it is what this tool exists to show.

It computes:

- **What is actually discharged** — the exports that count, netted against the average year by year, with the unendorsed shipping bills and the ones outside the period reported separately rather than quietly dropped.
- **Both clocks, which do not line up** — the obligation period and its blocks run from the date of issue, so they turn over on an anniversary; the average is measured against the licensing year, 1 April to 31 March. A six-year period beginning in May touches seven licensing years, the first and last part years.
- **The block-wise milestone** — half the obligation in the first four years, the balance in the fifth and sixth, cumulative, with the run rate in rupees a month that would close it.
- **The exposure, before the deadline** — the duty saved proportionate to the shortfall plus simple interest at 15% a year, worked out consignment by consignment from the date each one was cleared, and what one more day of delay adds.
- **Early fulfilment** — the figure and the date that would get the whole balance condoned under FTP 5.09, a provision whose deadline falls three years into a six-year licence and which almost nobody claims in time.
- **The closure file** — a twelve-item checklist for the Export Obligation Discharge Certificate, almost all of it derived from your own data rather than ticked.

For an Advance Authorisation it also tracks the **quantity** obligation beside the value one, and the **minimum value addition** on the Policy's own formula.

## Who it's for

Small and medium manufacturer-exporters in India with an EPCG or Advance Authorisation licence and no in-house trade-compliance desk: engineering and machining units, textile and garment units, food processors, pharmaceutical formulators, auto-component makers. Anybody whose obligation lives in a spreadsheet that one person maintains, and which is usually discovered to be wrong in the sixth year.

## The regulation

The **Foreign Trade Policy 2023** and its **Handbook of Procedures 2023**, notified by the Directorate General of Foreign Trade and in force from 1 April 2023, together with the customs notifications that give the schemes effect: **Notification No. 26/2023-Customs** (EPCG) and **Notification No. 21/2023-Customs** (Advance Authorisation), both dated 1 April 2023.

The provisions computed on include FTP 5.01(b) (six times the duty saved, in six years), FTP 5.04(c) (the average export obligation), HBP 5.13(a) (the block-wise milestone), HBP 5.18 (offsetting a year's shortfall against another year's excess), FTP 5.04(d), 5.09, 5.10, 5.11 and 5.04(l) (the reduced obligations, the early-fulfilment condonation, and the fact that only one may be taken), HBP 5.04(a) and Notification 26/2023-Customs condition (8) (the installation certificate, on two divergent deadlines), HBP 5.20(a) (redemption in ANF 5B), HBP 4.40(a) (the eighteen-month Advance Authorisation period), FTP 4.08 and 4.09(i) (value addition and its 15% minimum), and HBP 4.43(b), 4.46 and 4.47(b) (redemption in ANF 4F within six months).

**The Policy and its Handbook are amended often.** Every figure the tool applies is a named constant in one file (`internal/eo/policy.go`), is shown in the tool with the paragraph it comes from *and how much weight to put on it*, and is **editable** — because an authorisation goes on being governed by the figures that applied to it when it was issued. The tool states the Policy edition and the date the figures were last read on every screen and on every statement it produces. The defaults in this build were read on **24 November 2026**.

## Install &amp; run

```sh
# build it
git clone https://github.com/sreenivas-sadhu-prabhakara/epcg-advance-authorisation-export-obligation.git
cd epcg-advance-authorisation-export-obligation
go build -o epcg-advance-authorisation-export-obligation .

# run it, then open http://127.0.0.1:8080 in a browser
./epcg-advance-authorisation-export-obligation

# or run it straight from the source
go run .

# another port, and your licences somewhere of your choosing
./epcg-advance-authorisation-export-obligation -addr 127.0.0.1:9000 -data ~/exports/licences.json
```

Go 1.25, the standard library and nothing else. No database, no packages to fetch, no build step for the interface — it is embedded in the binary.

Two worked examples are built in. Press **Load the EPCG example** to see the arithmetic before typing anything of your own.

## Where your data is stored

One JSON file on your own machine, `./epcg-advance-authorisation-export-obligation-data.json` unless `-data` points elsewhere.

- **No outbound network calls.** Not for fonts, not for analytics, not for updates, not for exchange rates.
- **No DGFT or Customs integration.** No connection to the DGFT portal, to ICEGATE, or to any trade service.
- **No currency conversion.** A licence is in one currency and every figure entered against it must be in that currency; the tool applies no rate.
- **Written atomically** — a temporary file renamed over the old one, so an interrupted save leaves the previous file intact. Readable only by you.
- **Plain, indented JSON**, with amounts as decimal strings and dates as `yyyy-mm-dd`. Money is held internally as whole paise, never as a floating-point number.

The interface binds to `127.0.0.1` by default — the machine it runs on, not the network — and refuses any request a page on another site sends to it.

## Scope and accuracy

- **It has no DGFT portal integration and it does not file an EODC.** It cannot submit an application, register a shipping bill against an authorisation, fetch an e-BRC, or obtain a discharge certificate. It assembles the file and says whether it hangs together; you file it.
- **It is not the Regional Authority's reckoning.** On at least two points the published texts do not agree with each other — how a year's shortfall interacts with another year's excess (HBP 5.18 against the year-by-year rule in FTP 5.04(c)), and which of the two installation-certificate deadlines governs. The tool reports both readings rather than choosing one and hiding it.
- **Three things in the interest computation are decisions of this tool, not sourced figures,** and are labelled as such wherever they appear: that the interest is **simple** (nothing found for this build says whether it is simple or compound); that the rate is applied to the **whole** duty-saved figure entered (there is authority that it cannot be levied on the additional and special additional duty components); and the **end date and day count** (365 days, to the day computed for).
- **Two calendar conventions**, both overridable and both labelled: the last day of a derived period is taken as the day *before* the anniversary, because being wrong in the direction of urgency is the safe direction; and the average obligation in a part licensing year is pro-rated by days.
- **It condones nothing on its own.** The condonation field defaults to zero. FTP 5.09 early fulfilment is computed and reported as *eligibility*, never as applied, because a Regional Authority grants it.
- **It is only as right as what you type.** No scanning, no optical character recognition. A field left blank is reported as "not enough entered", never silently as satisfied — and a blank average export obligation blocks the closure checklist, because silence is not an exemption.
- **Not covered:** DFIA, RoDTEP, duty drawback, deemed exports, the gem and jewellery provisions of FTP Chapter 4, Export Oriented Units, Special Economic Zones, clubbing of authorisations, composition and late fees, and any authorisation issued before 1 April 2023 under an earlier Policy and a different customs notification.
- **It is not advice.** Confirm every figure against the current Foreign Trade Policy, its Handbook of Procedures and the notification quoted on your own bill of entry. Where a figure here differs from the one in force, that one governs and this tool is wrong.

## Development

```sh
gofmt -l .              # must print nothing
go vet ./...
go test ./... -count=1
go build -o /tmp/epcg-advance-authorisation-export-obligation-bin .
```

The rules live in `internal/eo` as pure functions with table-driven tests: `policy.go` (every statutory figure, in one place, with its source), `day.go` (calendar arithmetic — leap days, month-end clamping, licensing years), `money.go` (integer minor units and exact rate comparisons), `count.go` (the average-obligation netting), `obligation.go`, `early.go`, `exposure.go`, `eodc.go`, `assess.go` and `report.go`. Nothing in that package reads a file, opens a socket or looks at the clock except through `Today()`.

MIT licensed.
