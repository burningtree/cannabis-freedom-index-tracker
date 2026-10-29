// The request page (/submission/?id=): the outcome and why, the pipeline stage, what came out of it,
// the AI run with its live chat, the history grouped by attempt, and the details of the request.
// Called by submissions.js on every refresh; the parts are redrawn in place except the AI panel,
// which updates itself (src/lib/ai-activity.js) so its chat keeps its place.
import { restartControl } from "../lib/submission-restart.js";
import { renderAIActivity } from "../lib/ai-activity.js";
import { renderDraft, readResult } from "./draft-report.js";
import { safeHTTP } from "../lib/submission-receipts.js";
import { STAGES, statusMeta, isRunning, OUTCOME_HELP, ORIGINS, attempts, relativeTime, fullTime, duration, sourceParts } from "../lib/submission-status.js";

const el = (tag, text, className) => { const n = document.createElement(tag); if (text !== undefined && text !== null) n.textContent = text; if (className) n.className = className; return n; };
const link = (text, href, className) => { const a = el("a", text, className); a.href = href; if (/^https?:/.test(href)) { a.target = "_blank"; a.rel = "noopener noreferrer"; } return a; };
const time = (seconds, text) => { const t = el("time", text ?? relativeTime(seconds)); t.dateTime = new Date(seconds * 1000).toISOString(); t.title = fullTime(seconds); return t; };
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const PR = /^https:\/\/github\.com\/[^/]+\/[^/]+\/pull\/\d+$/;

function pill(status) {
  const meta = statusMeta(status), p = el("span", undefined, "sx-pill"); p.dataset.tone = meta.tone;
  p.append(el("i", undefined, "sx-dot"), document.createTextNode(meta.label)); return p;
}

// the top: status, a headline for the outcome, the worker's explanation, what it means, the source
function hero(box, v, run) {
  const meta = statusMeta(v.status), running = isRunning(v.status);
  const tries = attempts(v.history).length;
  box.replaceChildren(); box.dataset.tone = meta.tone; box.hidden = false;
  const top = el("div", undefined, "sx-hero-top");
  top.append(pill(v.status));
  if (running && tries > 1) top.append(el("span", `attempt ${tries}`, "sx-tag"));
  if (running && run?.started_at && !run.finished_at) top.append(el("span", `running for ${duration(Date.now() / 1000 - run.started_at)}`, "sx-tag"));
  box.append(top);
  const { host, path } = sourceParts(v.url), url = safeHTTP(v.url);
  const h1 = el("h1", undefined, "sx-hero-title");
  h1.append(el("span", host, "sx-host"), el("span", path, "sx-path"));
  box.append(h1);
  if (url) box.append(link(`${v.url} ↗`, url, "sx-hero-url"));
  const why = el("div", undefined, "sx-why-box");
  why.append(el("p", v.explanation, "sx-why-main"));
  if (!running && OUTCOME_HELP[v.status]) why.append(el("p", OUTCOME_HELP[v.status], "sx-why-help"));
  if (v.status === "budget_wait" && v.resume_at) why.append(el("p", `Scheduled to resume ${fullTime(v.resume_at)}. The budget counts run reservations, not the lower estimated cost shown below.`, "sx-why-help"));
  box.append(why);
  if (v.note) box.append(el("blockquote", v.note, "sx-quote"));
}

// the pipeline: done stages, the current one (pulsing while running), the outcome at the end
function stages(box, v) {
  const meta = statusMeta(v.status);
  box.replaceChildren(); box.hidden = false; box.dataset.tone = meta.tone;
  STAGES.forEach((name, i) => {
    const li = el("li", undefined, "sx-stage");
    li.dataset.state = i < meta.stage ? "done" : i === meta.stage ? (meta.tone === "running" ? "current" : "end") : "todo";
    li.append(el("i", undefined, "sx-stage-dot"), el("span", i === STAGES.length - 1 && meta.stage === i ? meta.label : name));
    box.append(li);
  });
}

// what came out of it: the draft drawn as a report (with a link to its preview), the published
// report, the proposal, the Nostr thread, or why there is nothing
function output(box, v) {
  const root = document.querySelector("[data-submission-status]");
  const base = root?.dataset.base ?? "/";
  box.replaceChildren(el("h2", "Outcome"));
  const items = el("div", undefined, "sx-out-items");
  if (SLUG.test(v.report_slug || "") && ["committed", "published", "duplicate"].includes(v.status)) items.append(link(v.status === "committed" ? "Read the committed report →" : v.status === "published" ? "Read the published report →" : "Read the existing report →", `${base}report/${v.report_slug}/`, "sx-btn sx-btn-primary"));
  if (readResult(v)) items.append(link("Preview as a report →", `${base}draft/?id=${v.id}`, `sx-btn${items.childElementCount ? "" : " sx-btn-primary"}`));
  if (PR.test(v.pr_url || "")) items.append(link("Proposal on GitHub ↗", v.pr_url, "sx-btn"));
  if (/^[a-f0-9]{64}$/.test(v.reply_event_id || "")) items.append(link("Reply on Nostr ↗", `https://njump.me/${v.reply_event_id}`, "sx-btn"));
  const draft = el("article", undefined, "sx-draft");
  const drew = renderDraft(draft, v, { base, types: JSON.parse(root?.dataset.types || "{}"), countries: JSON.parse(root?.dataset.countries || "{}"), compact: true });
  if (drew) box.append(el("p", isRunning(v.status) ? "The AI's draft, still being checked:" : ["published", "committed"].includes(v.status) ? "The report as drafted:" : "The AI's answer:", "sx-muted sx-draft-label"), draft);
  if (drew) box.append(validation(v));
  else if (isRunning(v.status)) box.append(el("p", "Nothing yet: the request is still being worked on. This page updates by itself.", "sx-muted"));
  else if (!items.childElementCount) box.append(el("p", OUTCOME_HELP[v.status] ?? v.explanation, "sx-muted"));
  if (items.childElementCount) box.append(items);
}

// how the AI's answer was checked: every source opened again by the service and each fact's
// quotation looked for in it; blocked sources are kept as the AI read them, missing quotations
// left out, and only a really bad answer refused
function validation(v) {
  const c = v.validation, wrap = el("div", undefined, "sx-validation");
  wrap.append(el("h3", "Check of the answer"));
  if (!c) {
    wrap.append(el("p", v.status === "verifying" ? "Opening the sources again to look for every quotation…" : isRunning(v.status) ? "The answer will be checked against its sources once the AI is done." : "This answer was not checked against its sources.", "sx-muted"));
    return wrap;
  }
  const n = (k, one, many) => `${k} ${k === 1 ? one : many}`;
  const summary = c.passed
    ? `${c.confirmed} of ${n(c.checked, "quotation", "quotations")} found in the sources${c.facts > c.checked ? `; ${n(c.facts - c.checked, "fact rests", "facts rest")} on a source that could not be opened again` : ""}.`
    : `Not passed: ${c.reason}.`;
  const head = el("p", summary, "sx-validation-summary"); head.dataset.ok = String(Boolean(c.passed));
  wrap.append(head);
  const list = el("ul", undefined, "sx-checks");
  for (const s of c.sources || []) {
    // ✓ a source that opened and confirmed something (any missing quotations are left out below),
    // ! one that could not be opened again, ✕ one that opened but confirmed nothing
    const li = el("li"); li.dataset.ok = !s.opened ? "warn" : String(s.confirmed > 0 || !s.facts);
    const text = el("span"), why = (s.error || "").replace(/\.$/, "");
    text.append(safeHTTP(s.url) ? link(`${s.publisher || s.title} ↗`, s.url) : el("span", s.publisher || s.title), document.createTextNode(s.opened
      ? `: opened again, ${s.confirmed} of ${n(s.facts, "quotation", "quotations")} found`
      : `: could not be opened again${why ? ` (${why})` : ""}; its ${n(s.facts, "fact is", "facts are")} kept as the AI read ${s.facts === 1 ? "it" : "them"}`));
    li.append(text);
    list.append(li);
  }
  wrap.append(list);
  if (c.left_out?.length) {
    const box = el("details", undefined, "sx-left-out"); box.append(el("summary", c.left_out.length === 1 ? "1 fact left out, its quotation not found" : `${c.left_out.length} facts left out, their quotations not found`));
    const ul = el("ul"); for (const t of c.left_out) ul.append(el("li", t)); box.append(ul); wrap.append(box);
  }
  return wrap;
}

// what the service found on the submitted page before any AI research
function sourceCheck(box, v, { api, local }) {
  const previousScreenshot = box.querySelector(".sx-source-screenshot");
  const c = v.source_check;
  const failed = v.status === "source_unavailable";
  box.hidden = !c && !failed && !["fetching"].includes(v.status);
  if (box.hidden) return;
  box.replaceChildren(el("h2", "Source check"));
  const list = el("ul", undefined, "sx-checks");
  const row = (ok, text) => { const li = el("li", text); li.dataset.ok = ok === null ? "wait" : String(ok); list.append(li); };
  if (v.status === "fetching" && !c) row(null, "Opening the submitted page…");
  else if (failed && !c) row(false, v.explanation.replace(/ Send a link.*$/, ""));
  if (c) {
    row(c.chars > 0, `Loaded from ${c.host}${c.title ? `: “${c.title}”` : ""}`);
    row(c.chars >= 200, `${Number(c.chars).toLocaleString()} characters of readable text`);
    if (c.chars >= 200) row(Boolean(c.mention), c.mention ? `Mentions cannabis (${c.mention})` : "Does not mention cannabis");
  }
  box.append(list);
  if (c) {
    box.append(el("p", c.rendered ? "Loaded with Playwright · JavaScript rendered" : "Loaded with HTTP", "sx-muted"));
    if (c.browser_warning) box.append(el("p", c.browser_warning, "sx-muted"));
    if (local && c.screenshot) {
      const captureKey = c.capture_id || `${v.id}:${c.title}:${c.chars}`;
      const figure = previousScreenshot?.dataset.capture === captureKey ? previousScreenshot : el("figure", undefined, "sx-source-screenshot");
      if (!figure.childElementCount) {
      figure.dataset.capture = captureKey;
      const screenshot = link("", `${api}/v1/submissions/${v.id}/source/screenshot${c.capture_id ? `?capture=${encodeURIComponent(c.capture_id)}` : ""}`);
      screenshot.target = "_blank"; screenshot.rel = "noopener noreferrer";
      const image = el("img"); image.src = screenshot.href; image.alt = `Captured source page from ${c.host}`;
      image.loading = "lazy"; image.decoding = "async";
      screenshot.append(image); figure.append(screenshot, el("figcaption", "Captured source page · click to open the full-size screenshot"));
      }
      box.append(figure);
    }
    const m = c.metadata;
    if (m && Object.values(m).some(value => Array.isArray(value) ? value.length : Boolean(value))) {
      const details = el("section", undefined, "sx-source-metadata"); details.append(el("h3", "Page metadata"));
      const dl = el("dl", undefined, "sx-facts");
      for (const [label, value] of [["Description", m.description], ["Publisher", m.site_name], ["Canonical", m.canonical], ["Language", m.language], ["Author", m.authors?.join(", ")], ["Published", m.published_at], ["Updated", m.modified_at]]) {
        if (!value) continue;
        const dd = el("dd"); const url = label === "Canonical" && safeHTTP(value);
        dd.append(url ? link(value, url) : document.createTextNode(value)); dl.append(el("dt", label), dd);
      }
      details.append(dl); box.append(details);
    }
  }
}

// the history, newest attempt first and open; earlier attempts folded, each with its result
function history(box, v) {
  box.replaceChildren(el("h2", "History"));
  const list = attempts(v.history);
  if (!list.length) { box.append(el("p", "No history yet.", "sx-muted")); return; }
  list.slice().reverse().forEach((a, k) => {
    const n = list.length - k;
    const group = el("details", undefined, "sx-attempt"); group.open = k === 0;
    const sum = el("summary");
    // an earlier attempt that stopped while still running was cut short and retried
    const retried = k > 0 && isRunning(a.status);
    const badge = retried ? el("span", undefined, "sx-pill") : pill(a.status);
    if (retried) { badge.dataset.tone = "attention"; badge.append(el("i", undefined, "sx-dot"), document.createTextNode("Retried")); }
    sum.append(el("strong", list.length > 1 ? `Attempt ${n}` : "Steps"), badge, el("span", a.end > a.start ? duration(a.end - a.start) : "", "sx-muted"));
    group.append(sum);
    const ol = el("ol", undefined, "sx-steps");
    for (const h of a.entries) {
      const li = el("li"); li.dataset.tone = statusMeta(h.status).tone;
      const head = el("div", undefined, "sx-step-head"); head.append(el("strong", statusMeta(h.status).label), time(h.created_at, new Date(h.created_at * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })));
      li.append(head, el("p", h.explanation)); ol.append(li);
    }
    group.append(ol); box.append(group);
  });
}

// the request itself: when, from where, its ids
function facts(box, v) {
  box.replaceChildren(el("h2", "Request"));
  const dl = el("dl", undefined, "sx-facts");
  const row = (k, val) => { const dd = el("dd"); dd.append(val instanceof Node ? val : document.createTextNode(val || "—")); dl.append(el("dt", k), dd); };
  row("Received", time(v.created_at, fullTime(v.created_at)));
  row("From", ORIGINS[v.origin] || v.origin);
  if (/^[a-f0-9]{64}$/.test(v.nostr_event_id || "")) row("Nostr post", link(`${v.nostr_event_id.slice(0, 12)}… ↗`, `https://njump.me/${v.nostr_event_id}`));
  const id = el("button", `${v.id.slice(0, 12)}…`, "sx-copy"); id.type = "button"; id.title = "Copy the receipt id";
  id.addEventListener("click", () => { navigator.clipboard?.writeText(v.id); id.textContent = "Copied"; setTimeout(() => { id.textContent = `${v.id.slice(0, 12)}…`; }, 1200); });
  row("Receipt", id);
  row("Case", el("code", v.case_id));
  box.append(dl);
}

export function renderSubmissionDetail(v, ai, { api, id }) {
  const run = ai?.activity?.runs?.[0];
  const attempt = attempts(v.history).at(-1);
  const previousAttempt = Boolean(run && attempt && (run.created_at < attempt.start));
  document.title = `${statusMeta(v.status).label}: ${sourceParts(v.url).host} · Submissions`;
  document.getElementById("crumb-id").textContent = sourceParts(v.url).host;
  hero(document.getElementById("submission-detail"), v, run);
  const restart = restartControl(v, { api, enabled: ai?.processing?.restart_available, onRestart: () => window.dispatchEvent(new Event("submission-restarted")) });
  if (restart) document.getElementById("submission-detail").append(restart);
  stages(document.getElementById("submission-stages"), v);
  document.getElementById("submission-body").hidden = false;
  output(document.getElementById("submission-output"), v);
  sourceCheck(document.getElementById("submission-source"), v, { api, local: ai?.processing?.restart_available });
  history(document.getElementById("submission-history"), v);
  facts(document.getElementById("submission-facts"), v);
  renderAIActivity(document.getElementById("submission-ai"), ai, { api, id, requestRunning: isRunning(v.status), previousAttempt });
}
