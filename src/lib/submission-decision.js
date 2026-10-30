import { safeHTTP } from './submission-receipts.js';
import { statusMeta } from './submission-status.js';

const outcomes = new Set(['insufficient_evidence', 'not_relevant', 'needs_review', 'source_unavailable', 'failed', 'routed_to_cfi', 'duplicate']);
const slug = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
// what the AI had concluded before the service stopped the run, in one line: the event itself is
// on the draft preview, so the decision box only says why it stopped
const concluded = {
  new_event: 'The AI found a supported new event and wrote a draft; the service stopped it afterwards, so this is not a rejection of the event.',
  existing_case: 'The AI matched it to an existing report and drafted an update; the service stopped it afterwards, so this is not a rejection of the event.',
  additional_source: 'The AI found another account of an event an existing report records, to be added as a source; the service stopped it afterwards.',
};
export function submissionDecision(v, base = '/') {
  if (!outcomes.has(v.status)) return null;
  const result = v.result && typeof v.result === 'object' ? v.result : {};
  const overlap = v.validation?.source_overlap;
  const model = typeof result.explanation === 'string' ? result.explanation.trim() : '';
  const reason = typeof v.explanation === 'string' ? v.explanation.trim() : '';
  const modelDecision = model && model === reason;
  const title = overlap || reason.startsWith('Source overlap with another case') ? 'Source already used by another case'
    : modelDecision && typeof result.decision_title === 'string' && result.decision_title.trim() ? result.decision_title.trim() : statusMeta(v.status).label;
  const links = [];
  if (overlap?.submission_id && /^[0-9a-f]{32}$/.test(overlap.submission_id)) links.push({ label: 'Related submission', url: `${base}submission/?id=${overlap.submission_id}` });
  const report = overlap?.report_slug || result.existing_report;
  if (slug.test(report || '')) links.push({ label: 'Related report', url: `${base}report/${report}/` });
  if (safeHTTP(overlap?.url)) links.push({ label: 'Shared source article', url: safeHTTP(overlap.url) });
  // the AI's sources back its own decision; when the service stopped the run they are beside the point
  for (const source of (modelDecision || !model) && Array.isArray(result.sources) ? result.sources : []) {
    const url = safeHTTP(source?.url);
    if (url && !links.some(link => link.url === url)) links.push({ label: source.title || source.publisher || 'Source article', url });
  }
  const modelAssessment = model && !modelDecision ? concluded[result.outcome] || null : null;
  return { title, details: reason || model, modelAssessment, links: links.slice(0, 10) };
}

// the existing reports a run adds sources to or corrects, besides a new report it writes
export function changedReports(v) {
  const result = v.result && typeof v.result === 'object' ? v.result : {};
  const out = [];
  if (result.outcome === 'additional_source' && slug.test(result.existing_report || '')) out.push({ slug: result.existing_report, change: 'source added' });
  if (result.outcome === 'existing_case' && slug.test(result.existing_report || '')) out.push({ slug: result.existing_report, change: 'updated' });
  for (const u of Array.isArray(result.report_updates) ? result.report_updates : []) {
    if (!slug.test(u?.report || '') || out.some(r => r.slug === u.report)) continue;
    const change = [Array.isArray(u.facts) && u.facts.length ? 'source added' : '', u.title || u.summary ? 'corrected' : '', Array.isArray(u.uncertainties) && u.uncertainties.length ? 'open questions' : ''].filter(Boolean).join(', ');
    out.push({ slug: u.report, change: change || 'updated' });
  }
  return out;
}
