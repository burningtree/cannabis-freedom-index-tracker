// The submissions list (/submissions/): one row per request, newest first, live over a WebSocket.
// A row says what is happening (a pulsing status while running, its stage and attempt) or how it
// ended and why, and links to the request's own page.
import { renderProcessingStatus } from "../lib/ai-activity.js";
import { ID_PATTERN } from "../lib/submission-receipts.js";
import { statusMeta, isRunning, STAGES, ORIGINS, relativeTime, fullTime, sourceParts } from "../lib/submission-status.js";

const root = document.querySelector("[data-submission-register]");
if (root) initialize(root);

function initialize(root) {
  const api = root.dataset.api.replace(/\/$/, "");
  if (!api) return;
  const base = root.dataset.base;
  const $ = (id) => document.getElementById(`register-${id}`);
  const el = (tag, text, className) => { const n = document.createElement(tag); if (text !== undefined) n.textContent = text; if (className) n.className = className; return n; };
  let cursors = [""], nextCursor = "", revision = "", rows = [], status = "";
  let socket, retryTimer, refreshTimer, tick, failures = 0, request, pending = false, disposed = false;

  function connection(text, state = "offline") { const c = $("connection"); c.dataset.state = state; c.querySelector("span").textContent = text; }
  function controls() {
    $("previous").disabled = cursors.length === 1; $("newest").disabled = cursors.length === 1; $("next").disabled = !nextCursor;
    $("page").textContent = `Page ${cursors.length}`;
    root.querySelector(".sx-pager").hidden = cursors.length === 1 && !nextCursor;
  }
  async function get(path, signal) {
    const res = await fetch(`${api}${path}`, { credentials: "omit", signal: AbortSignal.any([signal, AbortSignal.timeout(15000)]) });
    if (!res.ok) throw new Error("Submissions could not be refreshed.");
    return res.json();
  }

  // a status pill: a pulsing dot while running, a mark for how it ended
  function pill(s) {
    const meta = statusMeta(s), p = el("span", undefined, "sx-pill"); p.dataset.tone = meta.tone;
    p.append(el("i", undefined, "sx-dot"), document.createTextNode(meta.label)); return p;
  }
  function row(v) {
    const meta = statusMeta(v.status);
    const li = el("li", undefined, "sx-row"); li.dataset.tone = meta.tone;
    const a = el("a", undefined, "sx-row-link"); a.href = `${base}submission/?id=${v.id}`; a.title = v.explanation;
    const { host, path } = sourceParts(v.url);
    const src = el("span", undefined, "sx-row-source"); src.append(el("strong", host), el("span", path, "sx-path")); src.title = `${v.url}\n${v.explanation}`;
    const progress = el("span", undefined, "sx-row-progress");
    const lastStage = meta.stage === 5 && !["committed", "published", "duplicate", "routed_to_cfi"].includes(v.status) && v.last_stage ? statusMeta(v.last_stage).stage : meta.stage;
    progress.setAttribute("role", "progressbar"); progress.setAttribute("aria-label", "Processing stage");
    progress.setAttribute("aria-valuemin", "0"); progress.setAttribute("aria-valuemax", "5"); progress.setAttribute("aria-valuenow", String(lastStage));
    progress.setAttribute("aria-valuetext", meta.label); progress.title = `${STAGES[lastStage]} · ${meta.label}${v.resume_at ? ` · resumes ${fullTime(v.resume_at)}` : ""}`;
    for (let i = 0; i < STAGES.length; i++) { const part = el("i"); part.dataset.state = i < lastStage ? "done" : i === lastStage ? "current" : "todo"; progress.append(part); }
    const number = value => Number.isFinite(Number(value)) ? Number(value).toLocaleString() : "—";
    const input = el("span", number(v.input_tokens), "sx-row-number"); input.title = "Tokens read across all attempts (new input and cached)";
    const output = el("span", number(v.output_tokens), "sx-row-number"); output.title = "Tokens written across all attempts (answer and reasoning)";
    const runs = el("span", number(v.run_count), "sx-row-number"); runs.title = "AI runs across all attempts";
    const when = el("time", relativeTime(Math.max(v.created_at, v.updated_at || 0))); when.dateTime = new Date(Math.max(v.created_at, v.updated_at || 0) * 1000).toISOString(); when.title = `Updated ${fullTime(Math.max(v.created_at, v.updated_at || 0))}`;
    a.append(pill(v.status), src, progress, input, output, runs, el("span", ORIGINS[v.origin] || v.origin, "sx-origin"), when);
    li.append(a); return li;
  }
  function render() {
    const list = $("rows"); list.replaceChildren(); list.setAttribute("aria-busy", "false");
    if (!rows.length) list.append(el("li", status || $("origin").value ? "No submissions match this filter." : "No submissions yet.", "sx-empty"));
    for (const v of rows) list.append(row(v));
    controls();
  }
  async function refresh() {
    if (disposed || document.hidden) return;
    if (request) { pending = true; return; }
    const params = new URLSearchParams();
    if (cursors.at(-1)) params.set("cursor", cursors.at(-1));
    if (status) params.set("status", status);
    if ($("origin").value) params.set("origin", $("origin").value);
    request = new AbortController(); const active = request;
    try {
      const data = await get(`/v1/submissions?${params}`, active.signal);
      if (disposed || active.signal.aborted) return;
      renderProcessingStatus($("processing"), data.processing, { compact: true });
      const working = (data.working || []).filter(v => ID_PATTERN.test(v.id));
      $("working").replaceChildren(...working.map(row));
      if (!working.length) $("working").append(el("li", "No submissions are running right now.", "sx-empty"));
      $("working-count").textContent = Number.isFinite(data.summary?.running) ? `· ${data.summary.running.toLocaleString()}` : "";
      rows = data.submissions.filter((v) => ID_PATTERN.test(v.id));
      nextCursor = data.next_cursor; revision = data.revision; render();
      for (const key of ["total", "running", "queued", "paused"]) {
        $(key).textContent = Number.isFinite(data.summary?.[key]) ? data.summary[key].toLocaleString() : "—";
      }
      const running = rows.filter((v) => isRunning(v.status)).length;
      const paused = rows.filter(v => v.status === "budget_wait").length;
      $("count").textContent = `${Number(data.total).toLocaleString()} matching${running ? ` · ${running} in progress` : ""}${paused ? ` · ${paused} budget paused` : ""}`;
      $("message").textContent = cursors.length > 1 ? "Showing older submissions. Choose Newest to see new arrivals." : "";
    } catch {
      if (!active.signal.aborted) { $("message").textContent = "Submissions could not be refreshed; the last loaded list is kept."; if (!rows.length) $("rows").replaceChildren(el("li", "Submissions could not be loaded.", "sx-empty")); }
    } finally { request = null; if (pending) { pending = false; scheduleRefresh(); } }
  }
  function scheduleRefresh() { clearTimeout(refreshTimer); refreshTimer = setTimeout(refresh, 250); }
  function changePage(action) { request?.abort(); action(); pending = true; if (!request) { pending = false; refresh(); } }

  for (const chip of root.querySelectorAll("[data-filter]")) chip.addEventListener("click", () => {
    status = chip.dataset.filter;
    for (const c of root.querySelectorAll("[data-filter]")) c.setAttribute("aria-pressed", String(c === chip));
    changePage(() => { cursors = [""]; });
  });
  $("origin").addEventListener("change", () => changePage(() => { cursors = [""]; }));
  $("next").addEventListener("click", () => { if (nextCursor) changePage(() => cursors.push(nextCursor)); });
  $("previous").addEventListener("click", () => changePage(() => { if (cursors.length > 1) cursors.pop(); }));
  $("newest").addEventListener("click", () => changePage(() => { cursors = [""]; }));
  window.addEventListener("submission-created", () => {
    status = "";
    for (const chip of root.querySelectorAll("[data-filter]")) chip.setAttribute("aria-pressed", String(chip.dataset.filter === ""));
    changePage(() => { cursors = [""]; $("origin").value = ""; });
  });
  // "5 min ago" stays true without reloading
  tick = setInterval(() => {
    if (document.hidden) return;
    for (const t of root.querySelectorAll(".sx-list time")) t.textContent = relativeTime(Date.parse(t.dateTime) / 1000);
    // Refresh the UTC daily budget even when no WebSocket events occur overnight.
    refresh();
  }, 30000);

  function disconnect() { clearTimeout(retryTimer); const old = socket; socket = null; old?.close(); }
  function connect() {
    if (disposed || document.hidden || socket) return;
    if (!navigator.onLine) { connection("Offline"); return; }
    if (!window.WebSocket) { connection("No live updates"); return; }
    try {
      const url = new URL(`${api}/v1/submissions/stream`, location.href); url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
      connection("Connecting…"); let first = true; const current = new WebSocket(url); socket = current;
      current.onmessage = (event) => {
        if (socket !== current) return;
        try { const data = JSON.parse(event.data); if (data.type === "submissions_changed" && typeof data.revision === "string") { failures = 0; connection("Live", "live"); if (first || data.revision !== revision) scheduleRefresh(); first = false; } } catch { /* unknown message */ }
      };
      current.onclose = () => { if (socket !== current) return; socket = null; reconnect(); };
      current.onerror = () => { if (socket === current) connection("Reconnecting…"); };
    } catch { socket = null; reconnect(); }
  }
  function reconnect() { if (disposed || document.hidden) return; connection("Reconnecting…"); clearTimeout(retryTimer); retryTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** Math.min(failures++, 5)) + Math.random() * 500); }
  document.addEventListener("visibilitychange", () => { if (document.hidden) { disconnect(); clearTimeout(refreshTimer); connection("Paused"); } else { refresh(); connect(); } });
  window.addEventListener("online", () => { disconnect(); connect(); refresh(); });
  window.addEventListener("offline", () => { disconnect(); connection("Offline"); });
  window.addEventListener("pagehide", () => { disposed = true; disconnect(); request?.abort(); clearTimeout(refreshTimer); clearInterval(tick); });
  window.addEventListener("pageshow", (event) => { if (event.persisted) { disposed = false; connect(); refresh(); } });
  refresh(); connect();
}
