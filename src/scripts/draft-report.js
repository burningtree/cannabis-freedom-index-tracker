// The AI's answer for a submission, drawn with the same markup as a published report page
// (src/pages/report/[slug].astro): type chips, date, status, title, summary, place, figures, the
// text in paragraphs with numbered citations, the sources, and beside it where it happened. Used on
// the request page (compact: no side column) and on the draft preview page (/draft/?id=).
import { figureRows } from "../lib/figures.js";

const el = (tag, text, className) => { const n = document.createElement(tag); if (text !== undefined && text !== null) n.textContent = text; if (className) n.className = className; return n; };
const OUTCOMES = { new_event: "New report", existing_case: "Update to an existing report", not_relevant: "Outside scope", insufficient_evidence: "Unable to verify", routed_to_cfi: "Routed to the CFI", needs_review: "Needs review" };
const PRECISION_LABEL = { exact: "Exact spot", locality: "Village, town or city", district: "District", region: "Province or region", country: "Country only" };
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const DAY = /^\d{4}-\d{2}-\d{2}$/;
const fmtDate = (d) => new Date(`${d}T00:00:00Z`).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
const flagOf = (id = "") => { const cc = id.split("/")[0].toUpperCase(); return /^[A-Z]{2}$/.test(cc) ? String.fromCodePoint(...[...cc].map((ch) => 0x1f1a5 + ch.charCodeAt(0))) : ""; };
const languageName = (code) => { try { return new Intl.DisplayNames(["en"], { type: "language" }).of(code); } catch { return code; } };
const httpURL = (u) => { try { return ["http:", "https:"].includes(new URL(u).protocol) ? u : null; } catch { return null; } };

export function readResult(v) {
  const r = v?.result;
  return r && typeof r === "object" && (r.title || r.facts?.length) ? r : null;
}

// the facts as paragraphs: facts with the same paragraph number together; one without starts its own
function paragraphs(facts) {
  const out = [];
  let last = null;
  facts.forEach((f, i) => {
    if (i === 0 || !f.paragraph || f.paragraph !== last) out.push([]);
    last = f.paragraph;
    out.at(-1).push(f);
  });
  return out;
}

function typeChip(types, id) {
  const t = types[id];
  if (!t) return null;
  const s = el("span", t.label, "type"); s.style.setProperty("--c", t.color);
  return s;
}

// container: where to draw; v: the submission; types: { id: {label, color} }; countries: { id: name }
// Returns false when there is no answer to draw. Not compact, the side column holds a
// [data-draft-map] box that the preview page turns into a map.
export function renderDraft(container, v, { base = "/", types = {}, countries = {}, compact = false } = {}) {
  const r = readResult(v);
  container.replaceChildren();
  if (!r) return false;
  const layout = el("div", undefined, compact ? "draft-layout compact" : "report-layout draft-layout");
  const article = el("article");
  const country = countries[r.country] || r.country;

  const head = el("header", undefined, "report-head");
  const meta = el("div", undefined, "meta");
  for (const t of [r.type, ...(Array.isArray(r.also) ? r.also : [])]) { const chip = typeChip(types, t); if (chip) meta.append(chip); }
  if (DAY.test(r.incident_date || "")) { const d = el("time", fmtDate(r.incident_date)); d.dateTime = r.incident_date; meta.append(d); }
  const status = el("span", "draft", "status draft"); status.title = "Not reviewed or published"; meta.append(status);
  meta.append(el("span", OUTCOMES[r.outcome] || r.outcome, "draft-outcome"));
  head.append(meta, el(compact ? "h3" : "h1", r.title || "Untitled draft"));
  if (r.summary) head.append(el("p", r.summary, "lead"));
  if (r.country) head.append(el("p", `${flagOf(r.country)} ${[r.place, country].filter(Boolean).join(" · ")}`, "muted"));
  article.append(head);

  const rows = figureRows(r.figures && typeof r.figures === "object" ? r.figures : {});
  if (rows.length) {
    const dl = el("dl", undefined, "figures");
    for (const f of rows) { const d = el("div"); d.append(el("dt", f.label), el("dd", f.value)); dl.append(d); }
    article.append(dl);
  }

  // the text as the file will have it: paragraphs of sentences, each followed by its source's number
  const sources = Array.isArray(r.sources) ? r.sources : [];
  const facts = Array.isArray(r.facts) ? r.facts : [];
  if (facts.length) {
    const prose = el("div", undefined, "prose");
    for (const group of paragraphs(facts)) {
      const p = el("p");
      group.forEach((f, i) => {
        p.append(document.createTextNode(`${i ? " " : ""}${f.text}`));
        if (Number.isInteger(f.source) && f.source >= 1 && f.source <= sources.length) {
          const sup = el("sup", undefined, "cite"); const a = el("a", `[${f.source}]`); a.href = `#draft-ref-${f.source}`; a.setAttribute("aria-label", `Source ${f.source}`);
          if (f.quote) a.title = `“${f.quote}”`;
          sup.append(a); p.append(sup);
        }
      });
      prose.append(p);
    }
    article.append(prose);
  } else if (r.explanation) article.append(el("p", r.explanation, "draft-expl"));

  if (r.existing_report && SLUG.test(r.existing_report)) { const p = el("p", "Existing report: ", "muted"); const a = el("a", r.existing_report); a.href = `${base}report/${r.existing_report}/`; p.append(a); article.append(p); }

  if (sources.length) {
    article.append(el(compact ? "h4" : "h2", "Sources"));
    const ol = el("ol", undefined, "sources");
    sources.forEach((s, i) => {
      const li = el("li"); li.id = `draft-ref-${i + 1}`;
      const url = httpURL(s.url);
      if (url) { const a = el("a", s.title || s.url, "ext"); a.href = url; a.target = "_blank"; a.rel = "noopener noreferrer"; li.append(a); } else li.append(el("cite", s.title));
      li.append(document.createTextNode(`. ${s.publisher || ""}${DAY.test(s.date || "") ? `, ${fmtDate(s.date)}` : ""}.`));
      if (/^[a-z]{2}$/.test(s.language || "") && s.language !== "en") li.append(el("span", ` In ${languageName(s.language)}.`, "muted"));
      ol.append(li);
    });
    article.append(ol);
  }

  if (!compact && facts.some((f) => f.quote)) {
    // the passage each sentence rests on, as the service checked it in the source
    const box = el("details", undefined, "draft-quotes"); box.append(el("summary", "Supporting quotations"));
    const ol = el("ol");
    for (const f of facts) if (f.quote) { const li = el("li"); li.append(el("p", f.text), el("blockquote", f.quote), el("span", `Source ${f.source}`, "muted")); ol.append(li); }
    box.append(ol); article.append(box);
  }
  if (r.uncertainties?.length) {
    const box = el("div", undefined, "draft-uncertain"); box.append(el("strong", "Open questions the AI reported"));
    const ul = el("ul"); for (const u of r.uncertainties) ul.append(el("li", u)); box.append(ul); article.append(box);
  }
  layout.append(article);

  if (!compact) {
    // beside the text, as on a report page: where it happened, and the law there
    const side = el("aside", undefined, "report-side");
    const where = el("section"); where.append(el("h2", "Where"));
    const c = r.coordinates;
    if (Array.isArray(c) && c.length === 2 && c.every(Number.isFinite)) {
      const label = PRECISION_LABEL[r.precision];
      const pin = el("div", undefined, "pin-map");
      const map = el("div", undefined, "map"); map.dataset.draftMap = JSON.stringify({ lat: c[0], lon: c[1], precision: r.precision, type: r.type });
      pin.append(map);
      if (label) pin.append(el("span", label, `loc-chip ${r.precision}`));
      where.append(pin);
      const acc = el("p", undefined, "loc-accuracy");
      if (label) acc.append(el("b", `Location accuracy: ${label.toLowerCase()}.`), document.createTextNode(" "));
      if (r.place) acc.append(document.createTextNode(`Place: ${r.place}. `));
      acc.append(el("span", `${c[0].toFixed(2)}, ${c[1].toFixed(2)}`, "loc-coords"));
      where.append(acc);
    } else where.append(el("p", "No location given.", "muted"));
    side.append(where);
    if (r.country) {
      const law = el("section"); law.append(el("h2", "The law there"));
      const p = el("p"); const a = el("a", `${country}: the law and all reports →`); a.href = `${base}country/${r.country}/`; p.append(a); law.append(p);
      side.append(law);
    }
    layout.append(side);
  }
  container.append(layout);
  return true;
}
