import { runCost, costEstimated } from "./model-cost.js";

export const MODEL_LABELS = { "opencode-go/deepseek-v4.1-flash": "DeepSeek V4.1 Flash · OpenCode Go" };
export function modelLabel(model) { return MODEL_LABELS[model] || model || "Not configured"; }
const node = (tag, text, className) => { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (className) el.className = className; return el; };

export function renderProcessingStatus(container, processing, { compact = false } = {}) {
  container.replaceChildren();
  if (!processing) { container.dataset.state = "blocked"; container.append(node("p", "AI worker status is unavailable. Restart backend-dev after backend changes.")); return; }
  container.dataset.state = processing.enabled ? "ready" : "blocked";
  if (compact) {
    // one line for the submissions list: is the worker running, which model, anything holding it up
    const line = node("div", undefined, "ai-worker-line");
    line.append(node("i", undefined, "ai-worker-dot"), node("strong", processing.enabled ? (processing.mode === "automatic" ? "AI worker running" : processing.mode === "demo" ? "Demonstration mode" : "Review mode") : "AI worker paused"), node("span", modelLabel(processing.model), "ai-model"));
    if (processing.startup?.version) line.append(node("span", `OpenCode ${processing.startup.version}`, "ai-muted"));
    container.append(line);
    const notes = [];
    if (!processing.enabled && processing.reason) notes.push(processing.reason);
    if (processing.automatic_publication && !processing.publication_configured) notes.push("Research runs; publishing waits for the GitHub App to be configured.");
    for (const n of notes) container.append(node("p", n, "ai-worker-note"));
    return;
  }
  container.append(node("strong", processing.mode === "automatic" ? "Automatic processing" : processing.mode === "demo" ? "Demonstration mode" : "Review mode"));
  container.append(node("span", modelLabel(processing.model), "ai-model"), node("p", processing.reason));
  if (processing.startup) { const check = processing.startup; container.append(node("p", `Startup test: ${check.status}${check.version ? ` · OpenCode ${check.version}` : ""}`, "ai-muted")); }
  if (processing.automatic_publication && !processing.publication_configured) container.append(node("p", "Research can run independently. Publication is waiting for GitHub App configuration.", "ai-muted"));
}

// The AI panel is built once per container and then updated in place: the status and run details
// are redrawn, and only chat messages not shown yet (by id) are added. Rebuilding everything on each
// refresh made the page jump and threw the chat back to its top, as a scroll box loses its position
// when it is taken out of the page (the submissions table does that on every refresh). The chat keeps
// its own scroll state, so it follows new messages while the reader is at the bottom and stays put,
// with a "New messages" button, while they read further up.
const ROLES = { user: "Request", assistant: "OpenCode", tool: "Tool", system: "Worker" };
const timeOf = (event) => { const date = new Date(event.created_at * 1000), time = node("time", date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })); time.dateTime = date.toISOString(); return time; };
function eventNode(event) {
  // a finished model step is a thin divider between the messages of one step and the next
  if (event.kind === "step") { const line = node("div", undefined, "ai-step"); line.dataset.id = event.id; line.append(node("span", event.text), timeOf(event)); return line; }
  const bubble = node("article", undefined, "ai-message"); bubble.dataset.role = event.role; bubble.dataset.kind = event.kind || ""; bubble.dataset.id = event.id;
  const label = event.kind === "reasoning" ? "Thinking" : event.kind === "answer" ? "Final answer" : ROLES[event.role] || "Worker";
  const header = node("div", undefined, "ai-message-meta"); header.append(node("strong", label), timeOf(event));
  bubble.append(header);
  if (event.kind === "answer") {
    // the structured answer, folded and formatted
    let text = event.text; try { text = JSON.stringify(JSON.parse(text.replace(/^```(?:json)?\s*|```$/g, "")), null, 2); } catch { /* not complete JSON: as it came */ }
    const box = node("details"); box.append(node("summary", "Show the JSON"), node("pre", text)); bubble.append(box);
  } else if (event.kind === "reasoning" && event.text.length > 320) {
    const box = node("details"); box.append(node("summary", `${event.text.slice(0, 220).trim()}…`), node("p", event.text)); bubble.append(box);
  } else bubble.append(node("p", event.text));
  return bubble;
}

function buildPanel(container, api, id) {
  container.replaceChildren(); container.classList.add("ai-panel");
  const heading = node("div", undefined, "ai-heading");
  const title = node("div", undefined, "ai-title"); const runPill = node("span", undefined, "ai-run-pill");
  title.append(node("h2", "AI research"), runPill);
  const model = node("span", "", "ai-harness");
  heading.append(title, model);
  const processing = node("div", undefined, "ai-processing-status");
  const details = node("dl", undefined, "ai-stats");
  const runs = node("div", undefined, "ai-runs");
  const wrap = node("div", undefined, "ai-chat-wrap");
  const log = node("div", undefined, "ai-chat"); log.dataset.aiChat = ""; log.setAttribute("role", "log"); log.setAttribute("aria-label", "Public OpenCode activity"); log.setAttribute("aria-live", "polite");
  const empty = node("p", "", "ai-muted ai-empty");
  const more = node("button", "New messages ↓", "ai-new"); more.type = "button"; more.hidden = true;
  // while a run works: animated dots, what it is doing and how long since its last update
  const working = node("div", undefined, "ai-working"); working.hidden = true; working.setAttribute("aria-live", "off");
  const dots = node("span", undefined, "ai-dots"); dots.append(node("i"), node("i"), node("i"));
  const doing = node("span", "", "ai-doing"), since = node("span", "", "ai-since");
  working.append(dots, doing, since);
  log.append(empty, working); wrap.append(log, more);
  const ui = { log, empty, more, processing, details, runPill, model, runList: runs, working, doing, since, seen: new Set(), atBottom: true, top: 0, lastAt: 0 };
  ui.timer = setInterval(() => {
    if (!container.isConnected) { clearInterval(ui.timer); return; }
    if (!working.hidden && ui.lastAt) since.textContent = `${span(Date.now() / 1000 - ui.lastAt)} since the last update`;
  }, 1000);
  // where the reader is in the chat, kept on the panel so a refresh can put them back
  log.addEventListener("scroll", () => { ui.top = log.scrollTop; ui.atBottom = log.scrollHeight - log.scrollTop - log.clientHeight < 40; if (ui.atBottom) more.hidden = true; }, { passive: true });
  more.addEventListener("click", () => { log.scrollTo({ top: log.scrollHeight, behavior: "smooth" }); more.hidden = true; });
  const chatHead = node("div", undefined, "ai-chat-head"); chatHead.append(node("h3", "Live activity"), node("span", "Everything OpenCode does as it works: its reasoning, the sources it opens and searches, and its answer. Credentials are hidden.", "ai-muted"));
  container.append(heading, processing, details, runs, chatHead, wrap);
  if (api && id) { ui.json = createJSONLog(api, id); container.append(ui.json); }
  container._ai = ui;
  return ui;
}

const RUN_LABELS = { running: "Running", queued: "Waiting", completed: "Finished", failed: "Failed", cancelled: "Cancelled" };
const OUTCOME_LABELS = { new_event: "new report", duplicate: "already covered", routed_to_cfi: "routed to the CFI", not_relevant: "outside scope", insufficient_evidence: "unable to verify", needs_review: "needs review" };
const runTone = (run) => (run.status === "running" || run.status === "queued" ? "running" : run.status === "failed" ? "failed" : run.outcome === "new_event" ? "done" : "covered");
const runText = (run) => `${RUN_LABELS[run.status] || run.status}${run.outcome ? ` · ${OUTCOME_LABELS[run.outcome] || run.outcome.replace(/_/g, " ")}` : ""}`;
const span = (s) => { s = Math.max(0, Math.round(s)); return s < 60 ? `${s} s` : s < 3600 ? `${Math.floor(s / 60)} min ${s % 60 ? `${s % 60} s` : ""}`.trim() : `${Math.floor(s / 3600)} h ${Math.round((s % 3600) / 60)} min`; };

export function renderAIActivity(container, payload, { api, id, requestRunning = false } = {}) {
  let ui = container._ai;
  if (!ui || ui.id !== id || !container.contains(ui.log)) { clearInterval(ui?.timer); ui = buildPanel(container, api, id); ui.id = id; }
  const { log } = ui;
  const processing = payload?.processing;
  // the worker's state shows here only when something holds it up
  ui.processing.hidden = Boolean(processing?.enabled);
  if (!processing?.enabled) renderProcessingStatus(ui.processing, processing);
  const activity = payload?.activity;
  const run = activity?.runs?.[0];
  ui.model.textContent = modelLabel(run?.model || processing?.model);
  ui.runPill.replaceChildren(); ui.runPill.hidden = !run;
  if (run) {
    // a failed run of a request that is still going will be retried
    const retrying = run.status === "failed" && requestRunning;
    ui.runPill.dataset.tone = retrying ? "running" : runTone(run);
    ui.runPill.append(node("i"), document.createTextNode(retrying ? "Failed · retrying" : runText(run)));
  }
  ui.details.replaceChildren(); ui.details.hidden = !run;
  if (run) {
    const steps = (activity.events || []).filter((e) => e.kind === "step" && e.run_id === run.id && !e.text.startsWith("OpenCode started")).length;
    const end = run.finished_at || (run.status === "running" ? Date.now() / 1000 : run.started_at);
    // this run, and under it the sum over every run of this request (retries included)
    const all = activity.runs, many = all.length > 1;
    const sum = (key) => all.reduce((s, r) => s + Number(r[key] || 0), 0);
    const time = (r) => (r.started_at ? (r.finished_at || (r.status === "running" ? Date.now() / 1000 : r.started_at)) - r.started_at : 0);
    const money = (v) => (v ? `$${v.toFixed(v < 1 ? 4 : 2)}` : "—");
    // OpenCode reports no cost for these runs, so it is worked out from the tokens at the price for
    // the hour the run started (src/lib/model-cost.js)
    const cost = runCost(run), totalCost = all.reduce((s, r) => s + runCost(r), 0);
    const stats = [
      ["Time", run.started_at ? span(end - run.started_at) : "—", many && span(all.reduce((s, r) => s + time(r), 0))],
      ["Steps", steps ? String(steps) : "—", many && String((activity.events || []).filter((e) => e.kind === "step" && !e.text.startsWith("OpenCode started")).length)],
      ["Tokens read", Number(run.input_tokens).toLocaleString(), many && sum("input_tokens").toLocaleString()],
      ["Tokens written", Number(run.output_tokens).toLocaleString(), many && sum("output_tokens").toLocaleString()],
      ["Cost", money(cost), many && totalCost > 0 && money(totalCost)],
      ["Session", run.session_id || "starting…", many && `${all.length} runs`],
    ];
    for (const [label, value, total] of stats) {
      const item = node("div"), dd = node("dd", value);
      dd.title = label === "Cost" && costEstimated(run) ? `${value} estimated from the tokens used, at OpenCode Go prices for the hour the run started` : value;
      item.append(node("dt", label), dd);
      if (total) item.append(node("span", label === "Session" ? total : `${total} in all runs`, "ai-total"));
      ui.details.append(item);
    }
  }
  // earlier runs of this request (retries), newest first
  const earlier = (activity?.runs || []).slice(1);
  ui.runList.replaceChildren(); ui.runList.hidden = !earlier.length;
  if (earlier.length) {
    const box = node("details"); box.append(node("summary", `${earlier.length} earlier ${earlier.length === 1 ? "run" : "runs"}`));
    const ol = node("ol");
    for (const r of earlier) {
      const li = node("li"); li.dataset.tone = runTone(r);
      li.append(node("i"), node("span", runText(r)), node("span", r.started_at && r.finished_at ? span(r.finished_at - r.started_at) : "", "ai-muted"), node("span", `${Number(r.input_tokens + r.output_tokens).toLocaleString()} tokens`, "ai-muted"));
      ol.append(li);
    }
    box.append(ol); ui.runList.append(box);
  }
  // only the messages not shown yet; the box was maybe taken out of the page, so its position comes
  // from what was recorded, not from the box itself
  const follow = ui.atBottom;
  let added = 0;
  for (const event of activity?.events || []) {
    if (ui.seen.has(event.id)) continue;
    if (event.kind === "step" && event.text.startsWith("OpenCode started")) continue; // stored by earlier versions, says nothing
    ui.seen.add(event.id); log.insertBefore(eventNode(event), ui.working); added++;
  }
  ui.empty.hidden = ui.seen.size > 0;
  // what the run is doing now, from its latest message
  const busy = Boolean(run) && ["running", "starting", "queued"].includes(run.status);
  ui.working.hidden = !busy;
  if (busy) {
    const mine = (activity.events || []).filter((e) => e.run_id === run.id);
    const last = mine.at(-1);
    ui.lastAt = last?.created_at || run.started_at || run.created_at;
    const steps = mine.filter((e) => e.kind === "step" && !e.text.startsWith("OpenCode started")).length;
    let doing = "Starting OpenCode";
    if (last) {
      if (last.kind === "tool" && last.text.endsWith("…")) doing = last.text.replace(/…$/, "");
      else if (last.kind === "answer") doing = "Checking the final answer";
      else if (last.kind === "reasoning") doing = "Thinking";
      else doing = steps ? `Thinking about step ${steps + 1}` : "Thinking";
    }
    ui.doing.textContent = doing;
    ui.since.textContent = `${span(Date.now() / 1000 - ui.lastAt)} since the last update`;
  }
  ui.empty.textContent = run ? "Waiting for OpenCode session updates…" : "No AI session has started for this request yet.";
  if (follow) { log.scrollTop = log.scrollHeight; ui.top = log.scrollTop; }
  else { log.scrollTop = ui.top; if (added) ui.more.hidden = false; }
  if (ui.json) { ui.json.setRawAvailable(Boolean(payload?.processing?.raw_logs_available)); if (ui.json.open) ui.json.refreshLog(); }
}

function createJSONLog(api, id) {
  const details = node("details", undefined, "ai-json-log"); details.dataset.aiJson = "";
  details.append(node("summary", "Show full log (JSON)"));
  const actions = node("div", undefined, "ai-json-actions");
  const mode = node("select"); mode.setAttribute("aria-label", "AI log content");
  const publicOption = node("option", "Public activity"); publicOption.value = "public";
  const rawOption = node("option", "Raw output + emitted reasoning"); rawOption.value = "raw";
  mode.append(publicOption, rawOption);
  let rawInitialized = false;
  details.setRawAvailable = available => {
    rawOption.disabled = !available; rawOption.hidden = !available;
    if (available && !rawInitialized) { mode.value = "raw"; rawInitialized = true; }
  };
  const refresh = node("button", "Refresh JSON"), download = node("button", "Download JSON");
  refresh.type = download.type = "button"; download.disabled = true;
  const status = node("p", "", "ai-muted"); status.setAttribute("role", "status");
  const pre = node("pre"); pre.tabIndex = 0; pre.setAttribute("aria-label", "Full AI activity JSON");
  actions.append(mode, refresh, download); details.append(actions, status, pre);
  let loading = false, json = "", timer;
  async function load() {
    if (loading) return;
    loading = true; refresh.disabled = true;
    const requestedMode = mode.value;
    if (!json) status.textContent = "Loading full log…";
    try {
      const response = await fetch(`${api}/v1/submissions/${encodeURIComponent(id)}/ai/${requestedMode === "raw" ? "raw" : "log"}`, { credentials: "omit", signal: AbortSignal.timeout(30000) });
      if (!response.ok) throw new Error("AI log unavailable");
      const data = await response.json();
      if (mode.value !== requestedMode) return;
      const updated = JSON.stringify(data, null, 2);
      if (updated !== json) { json = updated; pre.textContent = json; }
      download.disabled = false;
      status.textContent = requestedMode === "raw"
        ? `${data.raw_runs?.reduce((sum, run) => sum + run.events.length, 0) || 0} raw events across ${data.raw_runs?.length || 0} runs. Includes tool output and reasoning emitted by OpenCode. Refreshes every 2 seconds. Configured credentials are redacted.`
        : `${data.activity?.events?.length || 0} recorded messages across ${data.activity?.runs?.length || 0} runs. Updates while connected. Raw tool output and private reasoning are excluded.`;
    } catch { status.textContent = "Could not load the full AI log. Try Refresh JSON. Restart backend-dev if it has not been updated."; }
    finally { loading = false; refresh.disabled = false; if (mode.value !== requestedMode && details.open) load(); }
  }
  details.refreshLog = load;
  function trackRaw() {
    clearInterval(timer);
    if (details.open && mode.value === "raw") timer = setInterval(() => {
      if (!details.isConnected || !details.open) { clearInterval(timer); return; }
      if (!document.hidden) load();
    }, 2000);
  }
  details.addEventListener("toggle", () => { if (details.open) load(); trackRaw(); });
  mode.addEventListener("change", () => { json = ""; pre.textContent = ""; download.disabled = true; load(); trackRaw(); });
  refresh.addEventListener("click", load);
  download.addEventListener("click", () => {
    if (!json) return;
    const url = URL.createObjectURL(new Blob([json + "\n"], { type: "application/json" }));
    const link = node("a"); link.href = url; link.download = `submission-${id}-ai-${mode.value}-log.json`;
    link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  });
  return details;
}

export function watchSubmissionUpdates(api, refresh) {
  let socket, timer, closed = false, failures = 0;
  function connect() {
    if (closed || document.hidden || !navigator.onLine || !window.WebSocket) return;
    try { const url = new URL(`${api}/v1/submissions/stream`, location.href); url.protocol = url.protocol === "https:" ? "wss:" : "ws:"; const current = new WebSocket(url); socket = current;
      current.onmessage = () => { failures = 0; refresh(); };
      current.onclose = () => { if (socket !== current || closed || document.hidden) return; timer = setTimeout(connect, Math.min(30000, 1000 * 2 ** Math.min(failures++, 5))); };
      current.onerror = () => current.close();
    } catch { timer = setTimeout(connect, 5000); }
  }
  function stop() { clearTimeout(timer); const previous = socket; socket = null; previous?.close(); }
  const visibility = () => { stop(); if (!document.hidden) { refresh(); connect(); } };
  document.addEventListener("visibilitychange", visibility);
  window.addEventListener("online", visibility);
  window.addEventListener("pagehide", () => { closed = true; stop(); document.removeEventListener("visibilitychange", visibility); window.removeEventListener("online", visibility); }, { once: true });
  connect(); return stop;
}
