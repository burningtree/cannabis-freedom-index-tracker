// The country outline for a country page's map, which greys out everything else around it,
// highlights the border and zooms to the country (the grey world with the country cut out is made in
// the browser). One small file per mapped country or territory, from Natural Earth's 1:50m borders
// (world-atlas, public domain; 1:10m for small countries), matched by ISO 3166-1 numeric code. States have none.
import { feature } from "topojson-client";
import atlas from "world-atlas/countries-50m.json" with { type: "json" };
import detailed from "world-atlas/countries-10m.json" with { type: "json" };
import { countries } from "../../../lib/countries.js";

const shapes = new Map(feature(atlas, atlas.objects.countries).features.map((f) => [Number(f.id), f.geometry]));
// small countries (Malta, Singapore, Luxembourg) are drawn from the 1:10m borders: at their zoom the
// 1:50m ones are a few straight lines
const fine = new Map(feature(detailed, detailed.objects.countries).features.map((f) => [Number(f.id), f.geometry]));
const area = (ring) => Math.abs(ring.reduce((a, [x, y], i) => { const [x2, y2] = ring[(i + 1) % ring.length]; return a + x * y2 - x2 * y; }, 0)) / 2;
const bboxOf = (rings) => {
  const b = [Infinity, Infinity, -Infinity, -Infinity];
  for (const ring of rings) for (const [x, y] of ring) { b[0] = Math.min(b[0], x); b[1] = Math.min(b[1], y); b[2] = Math.max(b[2], x); b[3] = Math.max(b[3], y); }
  return b;
};
// the bounds of a country that may cross the 180° meridian (Russia, Fiji): measured with western
// longitudes moved past 180 too, whichever is narrower; east can then be above 180, which the map takes
const wrappedBbox = (rings) => {
  const plain = bboxOf(rings), shifted = bboxOf(rings.map((r) => r.map(([x, y]) => [x < 0 ? x + 360 : x, y])));
  return shifted[2] - shifted[0] < plain[2] - plain[0] ? shifted : plain;
};

export function getStaticPaths() {
  return countries.filter((c) => !c.parent && c.iso_numeric && (shapes.has(c.iso_numeric) || fine.has(c.iso_numeric))).map((c) => ({ params: { id: c.id }, props: { geometry: shapes.get(c.iso_numeric) ?? fine.get(c.iso_numeric), id: c.iso_numeric } }));
}

const partsOf = (g) => (g.type === "Polygon" ? [g.coordinates] : g.coordinates);

export function GET({ props }) {
  let parts = partsOf(props.geometry);
  const big = parts.reduce((a, p) => (area(p[0]) > area(a[0]) ? p : a)), [x0, y0, x1, y1] = bboxOf([big[0]]);
  if (Math.max(x1 - x0, y1 - y0) < 3 && fine.has(props.id)) parts = partsOf(fine.get(props.id));
  // the largest part (the mainland) sets the scale: points closer than 1/150 of its size are dropped
  // (at most 0.05°, about 5 km), so Canada stays small and Malta keeps its shape; islands smaller than
  // 1/2000 of it are left out
  const main = parts.reduce((a, p) => (area(p[0]) > area(a[0]) ? p : a));
  const [w, s, e, n] = bboxOf([main[0]]), size = Math.max(e - w, n - s);
  const tolerance = Math.min(0.05, size / 150), decimals = tolerance < 0.01 ? 3 : 2, k = 10 ** decimals;
  const simplify = (ring) => {
    const out = [];
    for (const [x, y] of ring) {
      const p = [Math.round(x * k) / k, Math.round(y * k) / k], last = out.at(-1);
      if (!last || Math.hypot(p[0] - last[0], p[1] - last[1]) >= tolerance) out.push(p);
    }
    if (out.length) out.push(out[0]);
    return out;
  };
  const minArea = area(main[0]) / 2000;
  const polygons = parts.map((p) => p.map(simplify).filter((r) => r.length >= 4 && area(r) > minArea)).filter((p) => p.length);
  // where to zoom: the mainland and the parts near it, so France is not shown with French Guiana
  // or the United States with Guam
  const reach = Math.max(size * 0.6, 1), near = [w - reach, s - reach, e + reach, n + reach];
  const bbox = wrappedBbox(polygons.map((p) => p[0]).filter((r) => { const [a, b, c, d] = bboxOf([r]); return c >= near[0] && a <= near[2] && d >= near[1] && b <= near[3]; }));
  const shape = { type: "Feature", properties: { bbox: bbox.map((v) => Math.round(v * 100) / 100) }, geometry: { type: "MultiPolygon", coordinates: polygons } };
  return new Response(JSON.stringify(shape), { headers: { "Content-Type": "application/geo+json" } });
}
