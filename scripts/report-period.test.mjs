import test from 'node:test';
import assert from 'node:assert/strict';
import { readPeriod, changePeriod } from '../src/scripts/report-period.js';

test('calendar links override recent presets, including older shared links', () => {
  assert.deepEqual(readPeriod(new URLSearchParams('range=30d&year=2023&month=07'), '30d'), {
    mode: 'calendar', range: 'all', year: '2023', month: '07', date: '',
  });
  assert.deepEqual(readPeriod(new URLSearchParams('range=7d&date=2024-02-29'), '30d'), {
    mode: 'calendar', range: 'all', year: '2024', month: '02', date: '2024-02-29',
  });
});
test('recent and all-time modes clear every calendar restriction', () => {
  for (const mode of ['7d', '30d', '1y', 'all']) {
    assert.deepEqual(changePeriod(mode, { year: '2023', month: '07', date: '2023-07-01' }), {
      mode, range: mode, year: '', month: '', date: '',
    });
  }
  assert.deepEqual(changePeriod('calendar', {}, '2026'), { mode: 'calendar', range: 'all', year: '2026', month: '', date: '' });
});
test('page defaults and invalid date parameters never create hidden calendar filters', () => {
  assert.equal(readPeriod(new URLSearchParams(), '30d').range, '30d');
  assert.equal(readPeriod(new URLSearchParams(), 'all').range, 'all');
  assert.equal(readPeriod(new URLSearchParams('range=7d'), 'all').range, '7d');
  assert.deepEqual(readPeriod(new URLSearchParams('year=bad&month=99&date=2026-02-30'), '30d'), {
    mode: '30d', range: '30d', year: '', month: '', date: '',
  });
});

test('Reports page ignores relative windows while preserving calendar selections', () => {
  for (const range of ['7d', '30d', '1y']) {
    assert.equal(readPeriod(new URLSearchParams({ range }), 'all', { allowRelative: false }).mode, 'all');
  }
  assert.deepEqual(readPeriod(new URLSearchParams('range=7d&year=2023'), 'all', { allowRelative: false }), {
    mode: 'calendar', range: 'all', year: '2023', month: '', date: '',
  });
});
