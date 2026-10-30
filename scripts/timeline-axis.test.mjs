import test from 'node:test';
import assert from 'node:assert/strict';
import { axisTicks } from '../src/scripts/report-timeline.js';

const years = Array.from({ length: 27 }, (_, i) => String(2000 + i));
const months = Array.from({ length: 24 }, (_, i) => new Date(Date.UTC(2024, 10 + i, 1)).toISOString().slice(0, 7));

test('a wide chart labels every year; a narrow one aggregates to round steps', () => {
  assert.equal(axisTicks(years, 'year', 1600).length, 27);
  assert.deepEqual(axisTicks(years, 'year', 900).map((t) => t.label), ['2000', '2002', '2004', '2006', '2008', '2010', '2012', '2014', '2016', '2018', '2020', '2022', '2024', '2026']);
  assert.deepEqual(axisTicks(years, 'year', 340).map((t) => t.label), ['2000', '2005', '2010', '2015', '2020', '2025']);
});

test('months show the year at January and month names between, fewer on small screens', () => {
  const wide = axisTicks(months, 'month', 1400).map((t) => t.label);
  assert.equal(wide.length, 24);
  assert.ok(wide.includes('2025') && wide.includes('Feb'));
  assert.deepEqual(axisTicks(months, 'month', 360).map((t) => t.label), ['2025', 'Jul', '2026', 'Jul']);
});

test('days are labelled by week; a short range gets its ends', () => {
  const days = Array.from({ length: 30 }, (_, i) => new Date(Date.UTC(2026, 9, 1 + i)).toISOString().slice(0, 10));
  assert.deepEqual(axisTicks(days, 'day', 400).map((t) => t.label), ['1 Oct', '8', '15', '22']);
  assert.deepEqual(axisTicks(['2025', '2026'], 'year', 40).map((t) => t.label), ['2025', '2026']);
});
