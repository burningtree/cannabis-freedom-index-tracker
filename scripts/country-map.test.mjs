import { test } from "node:test";
import assert from "node:assert/strict";
import { countryMask } from "../src/lib/country-mask.js";
import { pointBounds, longitudeNear } from "../src/lib/map-view.js";
import { atlasCountries } from "../src/lib/atlas-countries.js";
import atlas from "world-atlas/countries-50m.json" with { type: "json" };

const contains = (bbox, [x,y]) => x >= bbox[0] && x <= bbox[2] && y >= bbox[1] && y <= bbox[3];
test("Australia keeps its mainland and Tasmania when the atlas repeats its ISO code", () => {
  const mask = countryMask(36), b = mask.properties.bbox;
  assert(contains(b, [115.86,-31.95])); // Perth
  assert(contains(b, [151.21,-33.87])); // Sydney
  assert(contains(b, [147.33,-42.88])); // Hobart
  assert(b[2] - b[0] > 39 && b[2] - b[0] < 42);
  assert(atlasCountries(atlas).get(36).coordinates.length > 2);
});

test("western countries and tiny islands stay in the correct world copy", () => {
  for (const [id,points] of [[28,[[-61.8,17.1],[-61.8,17.6]]], [32,[[-58.38,-34.6],[-68.3,-54.8]]]]) {
    const b = countryMask(id).properties.bbox;
    assert(b[0] > -90 && b[2] < 0);
    for (const p of points) assert(contains(b,p));
  }
  assert(countryMask(28).geometry.coordinates.length >= 2, "both Antigua and Barbuda retained");
});

test("Russia and Fiji have continuous date-line geometry without world-spanning edges", () => {
  for (const id of [643,242]) {
    const f = countryMask(id), b = f.properties.bbox;
    assert(b[2] > 180 && b[2]-b[0] < 180);
    for (const polygon of f.geometry.coordinates) for (const ring of polygon) {
      assert.deepEqual(ring[0],ring.at(-1));
      for (let i=1;i<ring.length;i++) assert(Math.abs(ring[i][0]-ring[i-1][0]) < 180);
    }
    assert(f.mask.coordinates.length > 0);
  }
});

test("all mapped countries produce finite bounds and valid closed shells", () => {
  for (const id of atlasCountries(atlas).keys()) {
    const f=countryMask(id);
    assert(f.properties.bbox.every(Number.isFinite), `bounds ${id}`);
    assert(f.geometry.coordinates.length > 0, `geometry ${id}`);
    for(const polygon of f.geometry.coordinates) for(const ring of polygon) {
      assert(ring.length >= 4, `ring ${id}`);
      assert.deepEqual(ring[0],ring.at(-1));
    }
  }
});

test("point framing chooses the narrow arc without shifting Argentina east", () => {
  assert.deepEqual(pointBounds([[-70,-50],[-55,-20]]),[-70,-50,-55,-20]);
  assert.deepEqual(pointBounds([[179,-18],[-179,-16]]),[-181,-18,-179,-16]);
  assert.equal(longitudeNear(-170,100),190);
  assert.equal(pointBounds([]),null);
});
