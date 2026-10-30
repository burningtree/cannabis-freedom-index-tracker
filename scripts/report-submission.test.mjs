import test from 'node:test';
import assert from 'node:assert/strict';
import { submissionFacts, shortSubmission } from '../src/lib/report-submission.js';

test('a report shows when and how its submission came, research time and cost, nothing more', () => {
  const v = { created_at: Date.UTC(2026, 9, 30, 12) / 1000, origin: 'nostr' };
  const ai = { activity: { runs: [{ started_at: 1000, finished_at: 1240, cost_usd: 0.0312 }, { started_at: 2000, finished_at: 2060, cost_usd: 0.01 }, { started_at: 3000 }] } };
  const facts = submissionFacts(v, ai);
  assert.equal(facts[0], 'submitted 30 Oct 2026 via Nostr');
  assert.match(facts[1], /^researched in .+ over 2 runs$/);
  assert.equal(facts[2], 'AI cost $0.0412');
  assert.deepEqual(submissionFacts({ origin: 'web' }, null), []);
  assert.equal(shortSubmission('0123456789abcdef0123456789abcdef'), '01234567');
});
