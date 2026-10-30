import test from 'node:test';
import assert from 'node:assert/strict';
import { checkCoordinates } from './lib/coordinate-check.mjs';
import { matchingReview } from './lib/coordinate-reviews.mjs';

test('checks expected country and identifies another mapped territory', () => {
  assert.equal(checkCoordinates('gb', [51.5074, -0.1278]).status, 'inside');
  const paris = checkCoordinates('gb', [48.8566, 2.3522]);
  assert.equal(paris.status, 'outside');
  assert.ok(paris.otherTerritories.includes('France'));
});
test('handles both sides of the antimeridian and flags the old offshore Fiji point', () => {
  assert.equal(checkCoordinates('fj', [-16.5, 179.5]).status, 'inside');
  assert.equal(checkCoordinates('fj', [-18.2, -178.8]).status, 'inside');
  const offshore = checkCoordinates('fj', [-16.5162, -179.394]);
  assert.equal(offshore.status, 'outside');
  assert.deepEqual(offshore.otherTerritories, []);
  assert.ok(offshore.variants.some(v => v.change === 'longitude sign'));
});
test('small island winding never treats the rest of the globe as Maldives', () => {
  assert.equal(checkCoordinates('mv', [4.1755, 73.5093]).status, 'inside');
  const atlantic = checkCoordinates('mv', [0, 0]);
  assert.equal(atlantic.status, 'outside');
  assert.deepEqual(atlantic.otherTerritories, []);
});
test('states use parent land polygons, including ISO ids with multiple features', () => {
  const canberra = checkCoordinates('au/act', [-35.28, 149.13]);
  assert.equal(canberra.status, 'inside');
  assert.equal(canberra.checkedCountry, 'au');
  assert.equal(canberra.parentOnly, true);
  // This is deliberately not a state-boundary validator.
  assert.equal(checkCoordinates('us/ny', [34.05, -118.24]).status, 'inside');
});
test('missing shapes and invalid coordinate ranges remain distinct from outside land', () => {
  assert.equal(checkCoordinates('not-a-country', [0, 0]).status, 'unmapped');
  for (const point of [[91, 0], [0, 181], [NaN, 0], [0], ['1', 1]]) assert.equal(checkCoordinates('fj', point).status, 'invalid');
});

test('Réunion uses only its island from France, never mainland France', () => {
  assert.equal(checkCoordinates('re', [-20.88, 55.45]).status, 'inside');
  assert.equal(checkCoordinates('re', [48.8566, 2.3522]).status, 'outside');
  assert.equal(checkCoordinates('re', [-20.890223, 55.517318]).status, 'inside');
});

test('a review expires when any geographic field changes and needs evidence', () => {
  const data = { country: 'fj', coordinates: [-18.125929, 178.428397], place: 'Narain Jetty', precision: 'locality' };
  const review = { file: 'reports/example.md', ...data, reason: 'Verified coastal locality reference', evidence: ['https://example.org/source'] };
  assert.equal(matchingReview([review], './reports/example.md', data), review);
  for (const change of [{ coordinates: [-18, 178] }, { country: 'au' }, { place: 'Other jetty' }, { precision: 'exact' }]) {
    assert.equal(matchingReview([review], review.file, { ...data, ...change }), null);
  }
  assert.equal(matchingReview([{ ...review, evidence: [] }], review.file, data), null);
  assert.equal(matchingReview([review], 'reports/other.md', data), null);
});
