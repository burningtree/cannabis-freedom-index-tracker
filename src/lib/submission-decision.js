import { safeHTTP } from './submission-receipts.js';
import { statusMeta } from './submission-status.js';

const outcomes = new Set(['insufficient_evidence', 'not_relevant', 'needs_review', 'source_unavailable', 'failed', 'routed_to_cfi', 'duplicate']);
const slug = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
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
  for (const source of Array.isArray(result.sources) ? result.sources : []) {
    const url = safeHTTP(source?.url);
    if (url && !links.some(link => link.url === url)) links.push({ label: source.title || source.publisher || 'Source article', url });
  }
  return { title, details: reason || model, modelAssessment: model && !modelDecision ? model : null, links: links.slice(0, 10) };
}
