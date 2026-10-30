// Fills in the report page's "From submission" line from the submissions API, once, after the page
// has loaded; without the API the line keeps just its link.
import { submissionFacts, SUBMISSION_ID } from "../lib/report-submission.js";

const line = document.querySelector("[data-report-submission]");
const api = (line?.dataset.api || "").replace(/\/$/, ""), id = line?.dataset.reportSubmission || "";
if (line && api && SUBMISSION_ID.test(id)) {
  const get = (path) => fetch(`${api}/v1/submissions/${id}${path}`, { credentials: "omit", signal: AbortSignal.timeout(15000) }).then((r) => (r.ok ? r.json() : null));
  Promise.all([get(""), get("/ai").catch(() => null)]).then(([v, ai]) => {
    const facts = v ? submissionFacts(v, ai) : [];
    if (facts.length) line.querySelector("[data-facts]").textContent = ` · ${facts.join(" · ")}`;
  }).catch(() => {});
}
