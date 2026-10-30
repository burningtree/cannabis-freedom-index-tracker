import { atlasCountries } from "./atlas-countries.js";
import clipping from "polygon-clipping";
import atlas from "world-atlas/countries-50m.json" with { type: "json" };
import detailed from "world-atlas/countries-10m.json" with { type: "json" };
import { longitudeNear, pointBounds } from "./map-view.js";

const shapes = atlasCountries(atlas);
const fine = atlasCountries(detailed);
const cache = new Map();
const partsOf = g => g.type === "Polygon" ? [g.coordinates] : g.coordinates;
const area = ring => Math.abs(ring.reduce((sum, [x, y], i) => {
  const [xx, yy] = ring[(i + 1) % ring.length]; return sum + x * yy - xx * y;
}, 0)) / 2;
const biggest = parts => parts.reduce((a, b) => area(a[0]) >= area(b[0]) ? a : b);
const bounds = ring => [Math.min(...ring.map(p => p[0])), Math.min(...ring.map(p => p[1])), Math.max(...ring.map(p => p[0])), Math.max(...ring.map(p => p[1]))];

function unwrapped(parts) {
  const extent = pointBounds(parts.flatMap(p => p[0]));
  const anchor = (extent[0] + extent[2]) / 2;
  return parts.map(p => p.map(ring => ring.map(([x, y]) => [longitudeNear(x, anchor), y])));
}

export function countryMask(numericID) {
  if (cache.has(numericID)) return cache.get(numericID);
  let geometry = shapes.get(numericID) ?? fine.get(numericID);
  if (!geometry) return null;
  let parts = unwrapped(partsOf(geometry));
  let main = biggest(parts), [w, s, e, n] = bounds(main[0]);
  if (Math.max(e - w, n - s) < 3 && fine.has(numericID)) {
    const detailedParts = unwrapped(partsOf(fine.get(numericID)));
    // Some tiny states collapse to a line in the quantized 1:10m atlas.
    if (area(biggest(detailedParts)[0]) > 1e-10) parts = detailedParts;
  }
  main = biggest(parts); [w, s, e, n] = bounds(main[0]);
  const size = Math.max(e - w, n - s), tolerance = Math.min(.05, size / 150);
  const digits = tolerance < .01 ? 4 : 3, k = 10 ** digits;
  const simplify = ring => {
    const out = [];
    for (const [x, y] of ring.slice(0, -1)) {
      const p = [Math.round(x * k) / k, Math.round(y * k) / k];
      if (!out.length || Math.hypot(p[0] - out.at(-1)[0], p[1] - out.at(-1)[1]) >= tolerance) out.push(p);
    }
    if (out.length < 3) return [];
    out.push(out[0]); return out;
  };
  const minimum = area(main[0]) / 2000;
  const polygons = parts.flatMap(p => {
    // Never promote a lake/hole to the exterior if its outer ring was discarded.
    const shell = simplify(p[0]);
    if (shell.length < 4 || area(shell) < minimum) return [];
    return [[shell, ...p.slice(1).map(simplify).filter(r => r.length >= 4 && area(r) >= minimum)]];
  });
  // Joining the pieces at ±180 removes artificial date-line edges. A proper
  // polygon difference also preserves lakes and enclaves without overlapping holes.
  const united = clipping.union(polygons);
  const reach = Math.max(size * .6, 1);
  const near = united.filter(p => {
    const [a,b,c,d] = bounds(p[0]);
    return c >= w-reach && a <= e+reach && d >= s-reach && b <= n+reach;
  });
  const extent = pointBounds(near.flatMap(p => p[0]));
  const centre = (extent[0] + extent[2]) / 2;
  const world = [[[centre-180,-85.051129],[centre+180,-85.051129],[centre+180,85.051129],[centre-180,85.051129],[centre-180,-85.051129]]];
  const result = {
    type: "Feature", properties: { bbox: extent },
    geometry: { type: "MultiPolygon", coordinates: united },
    mask: { type: "MultiPolygon", coordinates: clipping.difference(world, united) },
  };
  cache.set(numericID, result); return result;
}
