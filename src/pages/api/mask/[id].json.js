// The country outline for a country page's map, which greys out everything else around it and
// highlights the border (the grey world with the country cut out is made in the browser). One small file per country with reports, from Natural Earth's 1:50m borders
// (world-atlas, public domain), matched by ISO 3166-1 numeric code. States have none.
import { feature } from "topojson-client";
import atlas from "world-atlas/countries-50m.json" with { type: "json" };
import { countries } from "../../../lib/countries.js";
import { reportsOf } from "../../../lib/reports.js";

const shapes = new Map(feature(atlas, atlas.objects.countries).features.map((f) => [Number(f.id), f.geometry]));
// coordinates to 2 decimals (about 1 km), points closer than 0.05° to the last kept one dropped:
// plenty for a mask at country zoom, and small even for Canada or Russia
const round = (ring) => {
  const out = [];
  for (const [x, y] of ring) {
    const p = [Math.round(x * 100) / 100, Math.round(y * 100) / 100], last = out.at(-1);
    if (!last || Math.hypot(p[0] - last[0], p[1] - last[1]) >= 0.05) out.push(p);
  }
  if (out.length) out.push(out[0]);
  return out;
};
// a ring's area in square degrees: islands below about 20 km² are left out
const area = (ring) => Math.abs(ring.reduce((a, [x, y], i) => { const [x2, y2] = ring[(i + 1) % ring.length]; return a + x * y2 - x2 * y; }, 0)) / 2;
const keep = (ring) => ring.length >= 4 && area(ring) > 0.002;

export function getStaticPaths() {
  return countries.filter((c) => !c.parent && c.iso_numeric && shapes.has(c.iso_numeric) && reportsOf(c.id).length).map((c) => ({ params: { id: c.id }, props: { geometry: shapes.get(c.iso_numeric) } }));
}

export function GET({ props }) {
  const parts = props.geometry.type === "Polygon" ? [props.geometry.coordinates] : props.geometry.coordinates;
  const polygons = parts.map((p) => p.map(round).filter(keep)).filter((p) => p.length);
  const shape = { type: "Feature", properties: {}, geometry: { type: "MultiPolygon", coordinates: polygons } };
  return new Response(JSON.stringify(shape), { headers: { "Content-Type": "application/geo+json" } });
}
