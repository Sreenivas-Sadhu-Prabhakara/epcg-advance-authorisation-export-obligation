/* The interface for the export obligation tracker.

   Plain DOM, no framework, no build step, no network beyond this machine. Every
   figure shown here comes from the server's assessment; nothing is recomputed
   in the browser, so the screen and the written statement can never disagree.

   Every piece of text that comes from the user or from the server is put on the
   page with textContent, never as markup. */
(function () {
  "use strict";

  /* ---------- small helpers ---------- */

  var $ = function (id) { return document.getElementById(id); };

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) { n.className = cls; }
    if (text !== undefined && text !== null && text !== "") { n.textContent = String(text); }
    return n;
  }

  function clear(node) { while (node.firstChild) { node.removeChild(node.firstChild); } }

  function val(id) { var n = $(id); return n ? n.value.trim() : ""; }
  function setVal(id, v) { var n = $(id); if (n) { n.value = v === undefined || v === null ? "" : String(v); } }
  function checked(id) { var n = $(id); return !!(n && n.checked); }
  function setChecked(id, v) { var n = $(id); if (n) { n.checked = !!v; } }

  function banner(message, kind) {
    var b = $("banner");
    b.className = "note" + (kind ? " " + kind : "");
    b.textContent = message || "";
    b.hidden = !message;
  }

  /* An amount as the server sends it: a decimal string in major units. Shown in
     the Indian grouping, with the licence's currency in front. */
  function money(text) {
    if (text === undefined || text === null || text === "") { return "—"; }
    var s = String(text);
    var neg = s.charAt(0) === "-";
    if (neg) { s = s.slice(1); }
    var dot = s.indexOf(".");
    var whole = dot < 0 ? s : s.slice(0, dot);
    var frac = dot < 0 ? ".00" : s.slice(dot);
    var out;
    if (whole.length <= 3) {
      out = whole;
    } else {
      var head = whole.slice(0, whole.length - 3);
      var tail = whole.slice(whole.length - 3);
      var parts = [];
      while (head.length > 2) { parts.unshift(head.slice(-2)); head = head.slice(0, -2); }
      if (head) { parts.unshift(head); }
      out = parts.join(",") + "," + tail;
    }
    return (neg ? "-" : "") + state.currency + " " + out + frac;
  }

  function pct(bips) {
    if (bips === undefined || bips === null) { return "—"; }
    var v = Number(bips) / 100;
    var s = v.toFixed(2);
    if (s.indexOf(".") >= 0) { s = s.replace(/0+$/, "").replace(/\.$/, ""); }
    return s + "%";
  }

  function day(iso) {
    if (!iso) { return "—"; }
    var parts = String(iso).split("-");
    if (parts.length !== 3) { return String(iso); }
    var months = ["January", "February", "March", "April", "May", "June",
      "July", "August", "September", "October", "November", "December"];
    var m = months[Number(parts[1]) - 1];
    if (!m) { return String(iso); }
    return String(Number(parts[2])) + " " + m + " " + parts[0];
  }

  function statusClass(s) {
    switch (s) {
      case "met": return "met";
      case "on-track": return "ontrack";
      case "behind": return "behind";
      case "overdue": return "overdue";
      default: return "";
    }
  }

  function statusWord(s) {
    switch (s) {
      case "met": return "met";
      case "on-track": return "on track";
      case "behind": return "behind";
      case "overdue": return "overdue";
      default: return "not enough entered";
    }
  }

  /* A table of key/value rows, which is most of what the results column is. */
  function table(rows) {
    var wrapper = el("div", "table-scroll");
    var t = el("table", "breakdown");
    var body = el("tbody");
    rows.forEach(function (r) {
      if (!r) { return; }
      var tr = el("tr", r[2] || "");
      tr.appendChild(el("td", null, r[0]));
      tr.appendChild(el("td", "num", r[1]));
      body.appendChild(tr);
    });
    t.appendChild(body);
    wrapper.appendChild(t);
    return wrapper;
  }

  /* ---------- state ---------- */

  var state = {
    today: "",
    currency: "INR",
    policy: null,
    citations: [],
    licences: [],
    licenceId: "",
    assessment: null,
    licence: null
  };

  /* ---------- reading the form into a licence ---------- */

  function rowValues(container) {
    var out = [];
    var rows = container.querySelectorAll("[data-row]");
    for (var i = 0; i < rows.length; i++) {
      var row = rows[i];
      var fields = row.querySelectorAll("[data-f]");
      var obj = {};
      var any = false;
      for (var j = 0; j < fields.length; j++) {
        var f = fields[j];
        var name = f.getAttribute("data-f");
        if (f.type === "checkbox") {
          obj[name] = f.checked;
          if (f.checked) { any = true; }
        } else {
          obj[name] = f.value.trim();
          if (obj[name] !== "") { any = true; }
        }
      }
      if (any) { out.push(obj); }
    }
    return out;
  }

  function readLicence() {
    var scheme = val("l-scheme");
    var l = {
      id: state.licenceId,
      scheme: scheme,
      authorisationNo: val("l-authorisationNo"),
      fileNo: val("l-fileNo"),
      holder: val("l-holder"),
      iec: val("l-iec"),
      product: val("l-product"),
      currency: val("l-currency") || "INR",
      issuedOn: val("l-issuedOn"),
      category: val("l-category"),
      specificEoEndorsed: val("l-specificEoEndorsed"),
      averageEo: val("l-averageEo"),
      averageEoExempt: checked("l-averageEoExempt"),
      averageEoExemptReason: val("l-averageEoExemptReason"),
      eoValueEndorsed: val("l-eoValueEndorsed"),
      eoQuantity: val("l-eoQuantity"),
      eoQuantityUnit: val("l-eoQuantityUnit"),
      normsBasis: val("l-normsBasis"),
      sionReference: val("l-sionReference"),
      otherInputsCif: val("l-otherInputsCif"),
      eoPeriodEndEndorsed: val("l-eoPeriodEndEndorsed"),
      blockEndsEndorsed: val("l-block1End") ? [val("l-block1End")] : [],
      extensions: rowValues($("extensions")).map(function (e) {
        return {
          kind: e.kind || "overall",
          blockNo: Number(e.blockNo || 0) || 0,
          grantedOn: e.grantedOn || "",
          newEndsOn: e.newEndsOn || "",
          reference: e.reference || "",
          feePaid: e.feePaid || ""
        };
      }),
      imports: rowValues($("imports")).map(function (im) {
        return {
          description: im.description || "",
          billOfEntryNo: im.billOfEntryNo || "",
          port: "",
          clearedOn: im.clearedOn || "",
          cifValue: im.cifValue || "",
          dutySaved: im.dutySaved || "",
          installedOn: im.installedOn || "",
          indigenous: !!im.indigenous
        };
      }),
      shipments: rowValues($("shipments")).map(function (sh) {
        return {
          shippingBillNo: sh.shippingBillNo || "",
          shippedOn: sh.shippedOn || "",
          port: "",
          product: sh.product || "",
          buyer: "",
          fobValue: sh.fobValue || "",
          quantity: sh.quantity || "",
          quantityUnit: sh.quantityUnit || "",
          authorisationEndorsed: !!sh.authorisationEndorsed,
          realisedOn: sh.realisedOn || "",
          realisedValue: sh.realisedValue || "",
          brcNo: "",
          sameOrSimilar: !!sh.sameOrSimilar,
          excluded: !!sh.excluded,
          excludedReason: ""
        };
      }),
      evidence: {
        caCertifiedStatement: checked("l-caCertifiedStatement"),
        rcmcHeld: checked("l-rcmcHeld"),
        bondOrBgDetails: val("l-bondOrBgDetails"),
        applicationFiled: checked("l-applicationFiled"),
        filedOn: val("l-filedOn"),
        fileReference: val("l-fileReference")
      },
      notes: ($("l-notes") ? $("l-notes").value : ""),
      policy: state.policy || {}
    };
    state.currency = l.currency;
    return l;
  }

  /* ---------- writing a licence into the form ---------- */

  function addRow(which, values) {
    var tpl = $("tpl-" + which);
    var container = $(which === "import" ? "imports" : which === "shipment" ? "shipments" : "extensions");
    var node = tpl.content.firstElementChild.cloneNode(true);
    var fields = node.querySelectorAll("[data-f]");
    for (var i = 0; i < fields.length; i++) {
      var f = fields[i];
      var name = f.getAttribute("data-f");
      var v = values ? values[name] : undefined;
      if (f.type === "checkbox") {
        f.checked = !!v;
      } else if (v !== undefined && v !== null) {
        f.value = String(v);
      }
    }
    var remove = node.querySelector("[data-remove]");
    if (remove) {
      remove.addEventListener("click", function () {
        node.parentNode.removeChild(node);
        recompute();
      });
    }
    container.appendChild(node);
    return node;
  }

  function writeLicence(l) {
    state.licenceId = l.id || "";
    state.currency = l.currency || "INR";

    setVal("l-scheme", l.scheme || "epcg");
    setVal("l-authorisationNo", l.authorisationNo);
    setVal("l-fileNo", l.fileNo);
    setVal("l-holder", l.holder);
    setVal("l-iec", l.iec);
    setVal("l-product", l.product);
    setVal("l-currency", l.currency || "INR");
    setVal("l-issuedOn", l.issuedOn);
    setVal("l-category", l.category || "as-endorsed");
    setVal("l-specificEoEndorsed", zeroBlank(l.specificEoEndorsed));
    setVal("l-averageEo", zeroBlank(l.averageEo));
    setChecked("l-averageEoExempt", l.averageEoExempt);
    setVal("l-averageEoExemptReason", l.averageEoExemptReason);
    setVal("l-eoValueEndorsed", zeroBlank(l.eoValueEndorsed));
    setVal("l-eoQuantity", zeroBlank(l.eoQuantity));
    setVal("l-eoQuantityUnit", l.eoQuantityUnit);
    setVal("l-normsBasis", l.normsBasis || "");
    setVal("l-sionReference", l.sionReference);
    setVal("l-otherInputsCif", zeroBlank(l.otherInputsCif));
    setVal("l-eoPeriodEndEndorsed", l.eoPeriodEndEndorsed);
    setVal("l-block1End", (l.blockEndsEndorsed && l.blockEndsEndorsed[0]) || "");

    clear($("imports"));
    clear($("shipments"));
    clear($("extensions"));
    (l.imports || []).forEach(function (im) {
      addRow("import", {
        billOfEntryNo: im.billOfEntryNo, clearedOn: im.clearedOn,
        cifValue: zeroBlank(im.cifValue), dutySaved: zeroBlank(im.dutySaved),
        description: im.description, installedOn: im.installedOn,
        indigenous: im.indigenous
      });
    });
    (l.shipments || []).forEach(function (sh) {
      addRow("shipment", {
        shippingBillNo: sh.shippingBillNo, shippedOn: sh.shippedOn,
        fobValue: zeroBlank(sh.fobValue), product: sh.product,
        quantity: zeroBlank(sh.quantity), quantityUnit: sh.quantityUnit,
        realisedOn: sh.realisedOn, realisedValue: zeroBlank(sh.realisedValue),
        authorisationEndorsed: sh.authorisationEndorsed,
        sameOrSimilar: sh.sameOrSimilar, excluded: sh.excluded
      });
    });
    (l.extensions || []).forEach(function (e) {
      addRow("extension", {
        kind: e.kind, blockNo: e.blockNo || "", newEndsOn: e.newEndsOn,
        grantedOn: e.grantedOn, reference: e.reference, feePaid: zeroBlank(e.feePaid)
      });
    });
    if (!(l.imports || []).length) { addRow("import", null); }
    if (!(l.shipments || []).length) { addRow("shipment", { authorisationEndorsed: true, sameOrSimilar: true }); }

    var ev = l.evidence || {};
    setChecked("l-caCertifiedStatement", ev.caCertifiedStatement);
    setChecked("l-rcmcHeld", ev.rcmcHeld);
    setVal("l-bondOrBgDetails", ev.bondOrBgDetails);
    setChecked("l-applicationFiled", ev.applicationFiled);
    setVal("l-filedOn", ev.filedOn);
    setVal("l-fileReference", ev.fileReference);
    if ($("l-notes")) { $("l-notes").value = l.notes || ""; }
    if (l.policy && l.policy.edition) { state.policy = l.policy; }

    schemeFields();
  }

  /* An amount of zero is almost always "not entered" on a form, and showing
     "0.00" in every blank field makes a licence look filled in when it is not. */
  function zeroBlank(v) {
    if (v === undefined || v === null) { return ""; }
    var s = String(v);
    if (s === "0" || s === "0.00" || s === "0.000") { return ""; }
    return s;
  }

  /* Only the fields that belong to the chosen scheme are shown. */
  function schemeFields() {
    var aa = val("l-scheme") === "advance-authorisation";
    var epcg = document.querySelectorAll("[data-epcg]");
    var advance = document.querySelectorAll("[data-aa]");
    for (var i = 0; i < epcg.length; i++) { epcg[i].hidden = aa; }
    for (var j = 0; j < advance.length; j++) { advance[j].hidden = !aa; }
  }

  /* ---------- talking to the server ---------- */

  function post(path, body) {
    return fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }).then(readResponse);
  }

  function put(path, body) {
    return fetch(path, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }).then(readResponse);
  }

  function get(path) { return fetch(path).then(readResponse); }

  function del(path) { return fetch(path, { method: "DELETE" }).then(readResponse); }

  function readResponse(response) {
    return response.json().then(function (body) {
      if (!response.ok) {
        throw new Error(body && body.problem ? body.problem : "the request failed");
      }
      return body;
    }, function () {
      throw new Error("the reply could not be read");
    });
  }

  /* ---------- the results column ---------- */

  function render(assessment, licence) {
    state.assessment = assessment;
    state.licence = licence;
    var card = $("results");
    clear(card);
    card.appendChild(el("h2", null, "Where it stands"));

    var a = assessment;

    /* The headline. */
    var head = el("div", "result-primary");
    head.appendChild(el("div", "k", a.headline.label));
    head.appendChild(el("div", "v", a.headline.value));
    head.appendChild(el("div", "sub", a.headline.sub));
    card.appendChild(head);

    /* What to act on. */
    if (a.alerts && a.alerts.length) {
      var alerts = el("div", "block");
      alerts.appendChild(el("h3", null, "What to act on"));
      a.alerts.forEach(function (al) {
        var box = el("div", "alert " + al.level);
        box.appendChild(el("h4", null, al.title));
        box.appendChild(el("p", null, al.detail));
        alerts.appendChild(box);
      });
      card.appendChild(alerts);
    }

    /* The clock. */
    var clock = el("div", "block");
    clock.appendChild(el("h3", null, "The clock"));
    clock.appendChild(table([
      ["Obligation period", day(a.period.start) + " to " + day(a.period.ends)],
      ["End date taken from", sourceWord(a.period.endsSource)],
      a.period.endsSource !== "derived" && a.period.derivedEnds
        ? ["Derived end, for comparison", day(a.period.derivedEnds)] : null,
      a.clock.expired
        ? ["Status", "the period ENDED " + Math.abs(a.clock.daysLeft) + " days ago"]
        : ["Days left", String(a.clock.daysLeft) + " of " + String(a.clock.daysTotal) +
            " (" + pct(a.clock.elapsedPct) + " elapsed)"]
    ]));
    if (a.period.disagreement) { clock.appendChild(el("p", null, a.period.disagreement)); }
    card.appendChild(clock);

    /* The obligation in value. */
    var ob = el("div", "block");
    var obHead = el("h3", null, "The obligation in value ");
    obHead.appendChild(standing(a.specific.status));
    ob.appendChild(obHead);
    ob.appendChild(table([
      a.specific.multipleApplies ? ["Duty, taxes and cess saved", money(a.specific.dutySaved)] : null,
      a.specific.multipleApplies ? ["Multiple applied", pct(a.specific.multiple) + " of the duty saved"] : null,
      ["Obligation", money(a.specific.required) + " (" + sourceWord(a.specific.source) + ")"],
      ["Counted toward it", money(a.specific.counted)],
      ["Discharged", pct(a.specific.fulfilledPct)],
      a.specific.shortfall !== "0.00" ? ["Short by", money(a.specific.shortfall)] : null,
      a.specific.condoned !== "0.00" ? ["Condoned (your figure)", money(a.specific.condoned)] : null,
      a.neededPerMonth !== "0.00"
        ? ["Run rate that closes it", money(a.neededPerMonth) + " a month for " + a.monthsLeft + " months"] : null
    ]));
    if (a.specific.disagreement) { ob.appendChild(el("p", null, a.specific.disagreement)); }
    card.appendChild(ob);

    /* The obligation in quantity, for an Advance Authorisation. */
    if (a.quantity && a.quantity.applies) {
      var q = el("div", "block");
      var qHead = el("h3", null, "The obligation in quantity ");
      qHead.appendChild(standing(a.quantity.status));
      q.appendChild(qHead);
      q.appendChild(table([
        ["Required", a.quantity.required + " " + (a.quantity.unit || "")],
        ["Exported", a.quantity.counted + " " + (a.quantity.unit || "")],
        ["Discharged", pct(a.quantity.fulfilledPct)],
        a.quantity.shortfall !== "0" ? ["Short by", a.quantity.shortfall + " " + (a.quantity.unit || "")] : null
      ]));
      q.appendChild(el("p", null,
        "An Advance Authorisation is discharged in quantity as well as in value. Exporting more value does not close a quantity shortfall."));
      card.appendChild(q);
    }

    /* Value addition, for an Advance Authorisation. */
    if (a.valueAddition && a.valueAddition.applies) {
      var va = el("div", "block");
      var vaHead = el("h3", null, "Value addition ");
      vaHead.appendChild(standing(a.valueAddition.status));
      va.appendChild(vaHead);
      va.appendChild(table([
        ["Minimum required", pct(a.valueAddition.minRequired)],
        ["CIF of the inputs (B)", money(a.valueAddition.denominatorB)],
        ["Realised FOB (A)", money(a.valueAddition.realisedFob)],
        ["On realisation", pct(a.valueAddition.onRealised)],
        ["On the shipping bills", pct(a.valueAddition.onShipped)],
        a.valueAddition.stillNeeded && a.valueAddition.stillNeeded !== "0.00"
          ? ["Further realisation to reach it", money(a.valueAddition.stillNeeded)] : null
      ]));
      if (a.valueAddition.note) { va.appendChild(el("p", null, a.valueAddition.note)); }
      card.appendChild(va);
    }

    /* Block-wise milestones. */
    if (a.blocks && a.blocks.length) {
      var bl = el("div", "block");
      bl.appendChild(el("h3", null, "Block-wise milestones"));
      var wrapper = el("div", "table-scroll");
      var t = el("table", "breakdown");
      var head2 = el("thead");
      var hr = el("tr");
      ["Block", "Ends", "Required", "Counted", "Standing"].forEach(function (h, i) {
        hr.appendChild(el("th", i >= 2 && i <= 3 ? "num" : null, h));
      });
      head2.appendChild(hr);
      t.appendChild(head2);
      var body = el("tbody");
      a.blocks.forEach(function (b) {
        var tr = el("tr");
        tr.appendChild(el("td", null, String(b.no)));
        tr.appendChild(el("td", null, day(b.window.to)));
        tr.appendChild(el("td", "num", money(b.required) + " (" + b.cumulativePercent + "%)"));
        tr.appendChild(el("td", "num", money(b.counted)));
        var td = el("td");
        td.appendChild(standing(b.status));
        tr.appendChild(td);
        body.appendChild(tr);
      });
      t.appendChild(body);
      wrapper.appendChild(t);
      bl.appendChild(wrapper);
      a.blocks.forEach(function (b) {
        if (b.shortfall === "0.00") { return; }
        if (b.closed) {
          bl.appendChild(el("p", null, "Block " + b.no + " closed on " + day(b.window.to) +
            " short by " + money(b.shortfall) + "."));
        } else {
          bl.appendChild(el("p", null, "Block " + b.no + ": " + money(b.shortfall) +
            " still to count by " + day(b.window.to) + ", " + b.clock.daysLeft +
            " days away — about " + money(b.neededPerMonth) + " a month."));
        }
      });
      card.appendChild(bl);
    }

    /* Early fulfilment. */
    if (a.earlyFulfilment && (a.earlyFulfilment.applies || a.earlyFulfilment.excluded)) {
      var ef = el("div", "block");
      var efHead = el("h3", null, "Early fulfilment ");
      if (a.earlyFulfilment.applies) { efHead.appendChild(standing(a.earlyFulfilment.status)); }
      ef.appendChild(efHead);
      if (a.earlyFulfilment.applies) {
        ef.appendChild(table([
          ["Half the original period ends", day(a.earlyFulfilment.window.to)],
          ["Needed by then", money(a.earlyFulfilment.required) +
            " (" + a.earlyFulfilment.thresholdPercent + "% of the obligation)"],
          ["Counted by then", money(a.earlyFulfilment.counted)],
          a.earlyFulfilment.shortfall !== "0.00" ? ["Short by", money(a.earlyFulfilment.shortfall)] : null,
          ["Average maintained throughout", a.earlyFulfilment.averageMaintained ? "yes" : "no"],
          a.earlyFulfilment.condonable !== "0.00"
            ? ["Balance that would be condoned", money(a.earlyFulfilment.condonable)] : null
        ]));
      }
      ef.appendChild(el("p", null, a.earlyFulfilment.note));
      card.appendChild(ef);
    }

    /* The average export obligation, year by year. */
    if (a.average && a.average.applies) {
      var av = el("div", "block");
      var avHead = el("h3", null, "Average export obligation ");
      avHead.appendChild(standing(a.average.status));
      av.appendChild(avHead);
      var aw = el("div", "table-scroll");
      var at = el("table", "breakdown");
      var ah = el("thead");
      var ahr = el("tr");
      ["Year", "Average due", "Same/similar", "To the obligation", "Average"].forEach(function (h, i) {
        ahr.appendChild(el("th", i >= 1 && i <= 3 ? "num" : null, h));
      });
      ah.appendChild(ahr);
      at.appendChild(ah);
      var ab = el("tbody");
      a.average.years.forEach(function (y) {
        var tr = el("tr");
        tr.appendChild(el("td", null, y.label + (y.partYear ? " (part)" : "")));
        tr.appendChild(el("td", "num", money(y.averageDue)));
        tr.appendChild(el("td", "num", money(y.sameOrSimilar)));
        tr.appendChild(el("td", "num", money(y.countedToSpecific)));
        tr.appendChild(el("td", null, y.averageMet ? "met" : "short by " + money(y.averageShortfall)));
        ab.appendChild(tr);
      });
      at.appendChild(ab);
      aw.appendChild(at);
      av.appendChild(aw);
      av.appendChild(table([
        ["Average due, all years shown", money(a.average.dueTotal)],
        ["Same/similar exports, all years", money(a.average.overallAchieved)],
        ["Maintained on an overall basis", a.average.overallMet ? "yes" : "no"],
        a.average.stillNeededThisYear !== "0.00"
          ? ["Still needed this year", money(a.average.stillNeededThisYear) +
              " by " + day(a.average.currentYearCloses)] : null
      ]));
      av.appendChild(el("p", null, "Years are shown up to " + day(a.asOf) +
        " only: a year that has not happened cannot have been failed. Only the excess over the average counts toward the obligation in value."));
      card.appendChild(av);
    } else if (a.average && a.average.note) {
      var avn = el("div", "block");
      avn.appendChild(el("h3", null, "Average export obligation"));
      avn.appendChild(el("p", null, a.average.note));
      card.appendChild(avn);
    }

    /* Exports counted and left out. */
    var ex = el("div", "block");
    ex.appendChild(el("h3", null, "Exports counted"));
    ex.appendChild(table([
      ["Shipping bills counted", String(a.tally.shipments)],
      ["FOB counted", money(a.tally.totalFob)],
      ["Realised", money(a.tally.realisedTotal)],
      a.tally.unrealised !== "0.00"
        ? ["Awaiting realisation", money(a.tally.unrealised) + " on " + a.tally.unrealisedCount + " bills"] : null,
      a.tally.notEndorsed && a.tally.notEndorsed.length
        ? ["NOT counted — unendorsed", String(a.tally.notEndorsed.length) + " bills"] : null,
      a.tally.outsidePeriod && a.tally.outsidePeriod.length
        ? ["NOT counted — outside the period", String(a.tally.outsidePeriod.length) + " bills"] : null,
      a.tally.notYet && a.tally.notYet.length
        ? ["Not yet counted — future dated", String(a.tally.notYet.length) + " bills"] : null,
      a.tally.undated && a.tally.undated.length
        ? ["NOT counted — undated", String(a.tally.undated.length) + " bills"] : null,
      a.tally.excluded && a.tally.excluded.length
        ? ["Set aside by you", String(a.tally.excluded.length) + " bills"] : null
    ]));
    card.appendChild(ex);

    /* The exposure. */
    var exp = el("div", "block");
    exp.appendChild(el("h3", null, "If the obligation is not discharged"));
    if (!a.exposure.applies) {
      exp.appendChild(el("p", null, a.exposure.basis));
    } else {
      if (a.exposure.hypothetical) {
        exp.appendChild(el("p", null,
          "The period is still running, so this is what would be payable if the licence closed today on today's figures. Nothing is payable yet."));
      }
      exp.appendChild(table([
        ["Unfulfilled proportion", pct(a.exposure.shortfallPct)],
        ["Duty that becomes payable", money(a.exposure.dutyPayable)],
        ["Interest at " + pct(a.exposure.rateApplied) + " a year", money(a.exposure.interest)],
        ["Total", money(a.exposure.total), "total"]
      ]));
      if (a.exposure.understated) {
        var warn = el("p", null, a.exposure.understated);
        exp.appendChild(warn);
      }
      exp.appendChild(el("p", null, a.exposure.basis));
    }
    card.appendChild(exp);

    /* The assumptions, folded away. */
    var det = el("details", "assumptions");
    det.appendChild(el("summary", null, "Assumptions, conventions and limits"));
    var ul = el("ul");
    (a.notes || []).forEach(function (n) { ul.appendChild(el("li", null, n)); });
    det.appendChild(ul);
    card.appendChild(det);

    renderEODC(a);
  }

  function standing(status) {
    return el("span", "standing " + statusClass(status), statusWord(status));
  }

  function sourceWord(s) {
    switch (s) {
      case "endorsed": return "endorsed on the authorisation";
      case "extended": return "as extended";
      case "derived": return "derived from the policy figures";
      default: return s || "—";
    }
  }

  /* ---------- the closure checklist ---------- */

  function renderEODC(a) {
    var list = $("eodc-list");
    clear(list);
    if (!a || !a.eodc || !a.eodc.requirements) {
      list.appendChild(el("p", "empty", "Work out a licence first and its closure list appears here."));
      return;
    }
    var verdict = el("div", "note" + (a.eodc.ready ? " good" : " bad"), a.eodc.verdict);
    list.appendChild(verdict);
    list.appendChild(table([
      ["Scheme", a.scheme === "epcg" ? "EPCG" : "Advance Authorisation"],
      ["Items satisfied", String(a.eodc.metCount) + " of " + String(a.eodc.totalCount)],
      ["Standing in the way", String(a.eodc.blockingCount)]
    ]));
    a.eodc.requirements.forEach(function (r) {
      var row = el("div", "req");
      row.appendChild(el("span", "mark " + r.state, r.state === "not-applicable" ? "n/a" : r.state));
      var body = el("div");
      body.appendChild(el("h4", null, r.label));
      if (r.detail) { body.appendChild(el("p", null, r.detail)); }
      if (r.action && r.state !== "met" && r.state !== "not-applicable") {
        body.appendChild(el("p", "action", r.action));
      }
      row.appendChild(body);
      list.appendChild(row);
    });
  }

  /* ---------- the saved-licence list ---------- */

  function renderLicences() {
    var list = $("licence-list");
    clear(list);
    if (!state.licences.length) {
      list.appendChild(el("p", "empty",
        "Nothing saved yet. Fill in a licence on the first tab and press Save, or load one of the worked examples."));
      return;
    }
    state.licences.forEach(function (s) {
      var row = el("div", "lic");
      row.appendChild(el("h3", null, s.label + " — " + s.schemeTitle));
      var meta = [s.holder, s.product, "issued " + day(s.issuedOn)].filter(Boolean).join(" · ");
      row.appendChild(el("p", "meta", meta));
      var figs = el("div", "figs");
      function fig(label, value) {
        var f = el("span", null, label + " ");
        f.appendChild(el("b", null, value));
        figs.appendChild(f);
      }
      fig("discharged", pct(s.fulfilledPct));
      fig("short by", s.currency + " " + s.shortfall);
      fig(s.expired ? "period ended" : "days left", s.expired ? day(s.periodEnds) : String(s.daysLeft));
      if (s.exposureTotal !== "0.00") { fig("duty + interest", s.currency + " " + s.exposureTotal); }
      fig("closure", s.eodcReady ? "ready to apply" : s.eodcBlocking + " in the way");
      row.appendChild(figs);

      var buttons = el("div", "btnrow");
      var open = el("button", "btn primary", "Open");
      open.type = "button";
      open.addEventListener("click", function () { openLicence(s.id); });
      buttons.appendChild(open);
      var remove = el("button", "btn ghost", "Remove");
      remove.type = "button";
      remove.addEventListener("click", function () { removeLicence(s.id, s.label); });
      buttons.appendChild(remove);
      row.appendChild(buttons);
      list.appendChild(row);
    });
  }

  /* ---------- the figures page ---------- */

  function renderFigures() {
    var list = $("figures-list");
    clear(list);
    var editable = {
      epcgMultiple: ["epcgMultipleHundredths", "hundredths of a multiple — 600 is six times"],
      epcgPeriod: ["epcgPeriodMonths", "months"],
      epcgGreen: ["epcgGreenMultipleHundredths", "hundredths of a multiple"],
      epcgArea: ["epcgSpecifiedAreaMultipleHundredths", "hundredths of a multiple"],
      epcgIndigenous: ["epcgIndigenousMultipleHundredths", "hundredths of a multiple"],
      earlyFulfilment: ["earlyFulfilmentPercent", "percent"],
      installationRA: ["installationCertificateMonths", "months"],
      installationCustoms: ["installationCertificateCustomsMonths", "months"],
      blockDefaultPay: ["blockDefaultPayMonths", "months"],
      aaPeriod: ["aaPeriodMonths", "months"],
      aaValueAddition: ["aaMinValueAdditionPercent", "percent"],
      aaEodc: ["aaEodcFilingMonths", "months"],
      interest: ["interestRatePercent", "percent a year"],
      interestBasis: ["interestDayBasis", "days in a year"],
      condonation: ["condonationPercent", "percent"]
    };

    state.citations.forEach(function (c) {
      var box = el("div", "fig");
      box.appendChild(el("h4", null, c.label));
      box.appendChild(el("div", "val", c.figure));
      box.appendChild(el("div", "src", c.source));
      box.appendChild(el("div", "cf", c.confidence));
      if (c.note) { box.appendChild(el("p", null, c.note)); }

      var field = editable[c.key];
      if (field && state.policy) {
        var wrapper = el("div", "field");
        var id = "p-" + field[0];
        var label = el("label", null, "Change it (" + field[1] + ")");
        label.setAttribute("for", id);
        wrapper.appendChild(label);
        var inp = el("div", "inp");
        var input = document.createElement("input");
        input.type = "text";
        input.inputMode = "numeric";
        input.id = id;
        input.value = String(state.policy[field[0]]);
        input.setAttribute("data-policy", field[0]);
        inp.appendChild(input);
        wrapper.appendChild(inp);
        box.appendChild(wrapper);
      }
      list.appendChild(box);
    });
  }

  function readPolicy() {
    var next = {};
    var keys = Object.keys(state.policy || {});
    keys.forEach(function (k) { next[k] = state.policy[k]; });
    var inputs = document.querySelectorAll("[data-policy]");
    for (var i = 0; i < inputs.length; i++) {
      var name = inputs[i].getAttribute("data-policy");
      var n = parseInt(inputs[i].value, 10);
      if (!isNaN(n)) { next[name] = n; }
    }
    return next;
  }

  /* ---------- actions ---------- */

  function recompute() {
    var licence = readLicence();
    if (!licence.issuedOn) {
      banner("Enter the date the authorisation was issued — every period in both schemes is counted from it.", "bad");
      return Promise.resolve();
    }
    banner("");
    return post("/api/assess", { licence: licence, save: false })
      .then(function (body) {
        render(body.assessment, body.licence);
      })
      .catch(function (err) { banner(err.message, "bad"); });
  }

  function save() {
    var licence = readLicence();
    banner("");
    return post("/api/assess", { licence: licence, save: true })
      .then(function (body) {
        writeLicence(body.licence);
        render(body.assessment, body.licence);
        banner("Saved as " + body.licence.id + ".", "good");
        return refreshState();
      })
      .catch(function (err) { banner(err.message, "bad"); });
  }

  function download() {
    var licence = readLicence();
    banner("");
    return fetch("/api/report.txt", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ licence: licence, save: false })
    }).then(function (response) {
      if (!response.ok) {
        return response.json().then(function (body) {
          throw new Error(body && body.problem ? body.problem : "the statement could not be produced");
        });
      }
      return response.blob();
    }).then(function (blob) {
      var url = URL.createObjectURL(blob);
      var link = document.createElement("a");
      link.href = url;
      link.download = (licence.authorisationNo || "export-obligation") + "-statement.txt";
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    }).catch(function (err) { banner(err.message, "bad"); });
  }

  function loadSample(key) {
    banner("");
    return get("/api/samples").then(function (body) {
      var found = null;
      (body.samples || []).forEach(function (s) { if (s.key === key) { found = s; } });
      if (!found) { throw new Error("that example is not available"); }
      writeLicence(found.licence);
      state.licenceId = "";
      render(found.assessment, found.licence);
      banner(found.title + ". The figures are invented; the shape of the problem is not.", null);
      showPanel("licence");
    }).catch(function (err) { banner(err.message, "bad"); });
  }

  function openLicence(id) {
    banner("");
    return get("/api/licences/" + encodeURIComponent(id)).then(function (body) {
      writeLicence(body.licence);
      render(body.assessment, body.licence);
      showPanel("licence");
    }).catch(function (err) { banner(err.message, "bad"); });
  }

  function removeLicence(id, label) {
    if (!window.confirm("Remove " + label + " from this file? The licence itself is unaffected.")) { return; }
    banner("");
    del("/api/licences/" + encodeURIComponent(id)).then(function () {
      if (state.licenceId === id) { state.licenceId = ""; }
      banner("Removed " + label + ".", null);
      return refreshState();
    }).catch(function (err) { banner(err.message, "bad"); });
  }

  function startNew() {
    state.licenceId = "";
    writeLicence({ scheme: val("l-scheme") || "epcg", currency: "INR", category: "as-endorsed" });
    var card = $("results");
    clear(card);
    card.appendChild(el("h2", null, "Where it stands"));
    card.appendChild(el("p", "hint",
      "Fill in the scheme, the date of issue and the obligation, then press Work it out."));
    renderEODC(null);
    banner("");
  }

  function refreshState() {
    return get("/api/state").then(function (body) {
      state.today = body.today;
      state.policy = body.defaultPolicy;
      state.citations = body.citations;
      state.licences = body.licences || [];
      renderLicences();
      renderFigures();
    });
  }

  function showPanel(name) {
    var buttons = $("tabs").querySelectorAll("button");
    for (var i = 0; i < buttons.length; i++) {
      var on = buttons[i].getAttribute("data-panel") === name;
      buttons[i].setAttribute("aria-pressed", on ? "true" : "false");
    }
    ["licence", "licences", "eodc", "figures"].forEach(function (p) {
      $("panel-" + p).hidden = p !== name;
    });
  }

  /* ---------- wiring ---------- */

  function wire() {
    $("tabs").addEventListener("click", function (event) {
      var button = event.target.closest("button[data-panel]");
      if (button) { showPanel(button.getAttribute("data-panel")); }
    });

    $("l-scheme").addEventListener("change", function () { schemeFields(); recompute(); });
    $("recompute").addEventListener("click", recompute);
    $("save").addEventListener("click", save);
    $("download").addEventListener("click", download);
    $("load-epcg").addEventListener("click", function () { loadSample("epcg"); });
    $("load-aa").addEventListener("click", function () { loadSample("advance-authorisation"); });
    $("clear").addEventListener("click", startNew);
    $("add-import").addEventListener("click", function () { addRow("import", null); });
    $("add-shipment").addEventListener("click", function () {
      addRow("shipment", { authorisationEndorsed: true, sameOrSimilar: true });
    });
    $("add-extension").addEventListener("click", function () { addRow("extension", null); });

    $("save-policy").addEventListener("click", function () {
      var note = $("policy-note");
      put("/api/policy", readPolicy()).then(function (body) {
        state.policy = body.defaultPolicy;
        state.citations = body.citations;
        renderFigures();
        note.textContent = body.note;
        note.hidden = false;
      }).catch(function (err) {
        note.textContent = err.message;
        note.hidden = false;
      });
    });

    $("reset-policy").addEventListener("click", function () {
      var note = $("policy-note");
      put("/api/policy", {}).then(function (body) {
        state.policy = body.defaultPolicy;
        state.citations = body.citations;
        renderFigures();
        note.textContent = "Back to the figures as published. " + body.note;
        note.hidden = false;
      }).catch(function (err) {
        note.textContent = err.message;
        note.hidden = false;
      });
    });
  }

  /* ---------- start ---------- */

  wire();
  startNew();
  refreshState().catch(function (err) {
    banner("The tool could not be reached: " + err.message, "bad");
  });
})();
