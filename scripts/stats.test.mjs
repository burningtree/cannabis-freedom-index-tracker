import test from 'node:test';
import assert from 'node:assert/strict';
import { compact, headlineFigures, modelName, reportsByModel, topCountriesBy, periodColumns } from '../src/lib/stats.js';

test('big figures stay readable as they grow', () => {
  assert.equal(compact(9999), '9,999');
  assert.equal(compact(17437), '17.4K');
  assert.equal(compact(5170718), '5.2M');
});

test('headline figures say how many reports state them; seizures switch to tonnes', () => {
  const list = [{ type: 'seizure', figures: { kg_seized: 1500, arrested: 2 } }, { type: 'arrest', figures: { arrested: 3 } }, { type: 'execution', figures: {} }];
  const f = Object.fromEntries(headlineFigures(list).map((x) => [x.id, x]));
  assert.deepEqual([f.arrested.value, f.arrested.reports], [5, 2]);
  assert.deepEqual([f.seized.value, f.seized.unit], [1.5, 'tonnes']);
  assert.deepEqual([f.deaths.value, f.deaths.reports], [1, 1]);
});

test('models are grouped without provider and reasoning setting; unknown is counted apart', () => {
  assert.equal(modelName('spacexai/grok-4.7#medium'), 'grok-4.7');
  assert.equal(modelName('Meta Muse Spark (via Muse Code)'), 'Meta Muse Spark (via Muse Code)');
  assert.equal(modelName('unknown # Creator identifier was unavailable'), null);
  const { rows, unrecorded } = reportsByModel([{ created_by_model: 'xai/grok-4.7' }, { created_by_model: 'spacexai/grok-4.7#medium' }, { created_by_model: 'cursor/composer-2.5' }, { created_by_model: 'unknown' }]);
  assert.equal(unrecorded, 1);
  assert.deepEqual(rows.map((r) => [r.name, r.count]), [['grok-4.7', 2], ['composer-2.5', 1]]);
  assert.equal(rows[0].variants.length, 2);
});

test('top countries and period columns', () => {
  const list = [{ country: 'in', type: 'raid', year: 2025, figures: { arrested: 4 } }, { country: 'ke', type: 'arrest', year: 2026, figures: { arrested: 9 } }, { country: 'in', type: 'raid', year: 2026, figures: { arrested: 1 } }];
  assert.deepEqual(topCountriesBy(list, 'arrested'), [{ id: 'ke', value: 9 }, { id: 'in', value: 5 }]);
  const cols = periodColumns(list, (r) => r.year, [2024, 2025, 2026]);
  assert.deepEqual(cols.map((c) => c.total), [0, 1, 2]);
  assert.deepEqual(cols[2].types, { arrest: 1, raid: 1 });
});
