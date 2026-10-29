// Takes a snapshot of Landrace.wiki's growing areas, growing regions and accessions (landrace cannabis
// populations and collected samples) for the map's "Landrace" layers:
//   public/layers/landrace.geojson         points: growing areas and accessions
//   public/layers/landrace-areas.geojson   outlines of growing areas  } from the wiki's Module:Boundaries
//   public/layers/landrace-regions.geojson outlines of growing regions } data pages, simplified
//
// The data is © Landrace.wiki contributors, CC BY-SA 4.0 (https://landrace.wiki/wiki/Landrace.Wiki:API);
// the file keeps that licence and its attribution. Coordinates are already offset by about 500 m by
// Landrace.wiki; never try to make them more precise. Etiquette from their API page: one request at a
// time, a few seconds apart, a User-Agent that says who we are, cache the result. Run it by hand now
// and then; email privacy@landrace.wiki before running it on a schedule.
//
//   node scripts/sync-landrace.mjs
import fs from "node:fs";

const API = "https://landrace.wiki/api.php";
const UA = "CannabisEnforcementTracker/0.1 (+https://tracker.cannabisfreedom.fyi; map layer snapshot)";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function ask(conditions, printouts) {
  const out = {};
  for (let offset = 0; offset != null;) {
    const query = `${conditions}|${printouts.map((p) => `?${p}`).join("|")}|limit=500|offset=${offset}`;
    const res = await fetch(`${API}?action=ask&format=json&query=${encodeURIComponent(query)}`, { headers: { "User-Agent": UA } });
    if (!res.ok) throw new Error(`${res.status} for ${conditions}`);
    const json = await res.json();
    Object.assign(out, json.query?.results ?? {});
    offset = json["query-continue-offset"] ?? null;
    console.log(`${conditions}: ${Object.keys(out).length}`);
    await sleep(3000);
  }
  return out;
}

// the source of a wiki page (the Lua data modules of the outlines)
async function source(title) {
  const res = await fetch(`${API}?action=query&prop=revisions&rvprop=content&rvslots=main&format=json&titles=${encodeURIComponent(title)}`, { headers: { "User-Agent": UA } });
  if (!res.ok) throw new Error(`${res.status} for ${title}`);
  const page = Object.values((await res.json()).query.pages)[0];
  await sleep(3000);
  return page.revisions?.[0]?.slots.main["*"] ?? "";
}

// the entries of a Module:Boundaries data page: { key: { polygon, style, source } }
function entries(lua) {
  const out = {};
  for (const m of lua.matchAll(/\["((?:[^"\\]|\\.)+)"\]\s*=\s*\{([^{}]*)\}/g)) {
    const field = (k) => m[2].match(new RegExp(`\\b${k}\\s*=\\s*"((?:[^"\\\\]|\\\\.)*)"`))?.[1];
    if (field("polygon")) out[m[1].replace(/\\"/g, '"')] = { polygon: field("polygon"), style: field("style") ?? null, source: field("source") ?? null };
  }
  return out;
}

// "lat,lon;lat,lon;…" to GeoJSON rings of [lon, lat], simplified. The parts of a multipolygon are
// separated by ":" (Module:Boundaries/doc, "Polygon string format"); each part is one ring.
// (Douglas–Peucker, in proportion to each shape's size) so the file stays small
function simplify(pts, tol) {
  if (pts.length < 4) return pts;
  const keep = new Uint8Array(pts.length); keep[0] = keep[pts.length - 1] = 1;
  const stack = [[0, pts.length - 1]];
  while (stack.length) {
    const [a, b] = stack.pop();
    const [x1, y1] = pts[a], [x2, y2] = pts[b];
    const dx = x2 - x1, dy = y2 - y1, len = Math.hypot(dx, dy) || 1e-12;
    let max = 0, idx = -1;
    for (let i = a + 1; i < b; i++) {
      const d = Math.abs(dy * pts[i][0] - dx * pts[i][1] + x2 * y1 - y2 * x1) / len;
      if (d > max) { max = d; idx = i; }
    }
    if (max > tol && idx > 0) { keep[idx] = 1; stack.push([a, idx], [idx, b]); }
  }
  return pts.filter((_, i) => keep[i]);
}
function rings(polygon) {
  return polygon.split(":").map((ring) => {
    const pts = ring.split(";").map((p) => p.split(",").map(Number)).filter((p) => p.length === 2 && p.every(Number.isFinite)).map(([lat, lon]) => [lon, lat]);
    // detail in proportion to the shape: about 1/500 of its extent, at least about 500 m
    const xs = pts.map((q) => q[0]), ys = pts.map((q) => q[1]);
    const tol = Math.max(0.005, Math.hypot(Math.max(...xs) - Math.min(...xs), Math.max(...ys) - Math.min(...ys)) / 500);
    // a closed ring has no line between its ends to measure from: simplify its two halves
    const mid = Math.floor(pts.length / 2);
    const s = [...simplify(pts.slice(0, mid + 1), tol), ...simplify(pts.slice(mid), tol).slice(1)].map(([x, y]) => [+x.toFixed(3), +y.toFixed(3)]);
    if (s.length && (s[0][0] !== s.at(-1)[0] || s[0][1] !== s.at(-1)[1])) s.push(s[0]);
    return s;
  }).filter((r) => r.length >= 4);
}

const first = (v) => v?.[0];
const name = (v) => first(v)?.fulltext ?? null;
const point = (r) => { const c = first(r.printouts["Has coordinates"]); return c ? [+c.lon.toFixed(3), +c.lat.toFixed(3)] : null; };

const areas = await ask("[[Category:Growing Areas]][[Has coordinates::+]]", ["Has coordinates", "Has country", "Has growing region"]);
const accessions = await ask("[[Category:Accessions]][[Has coordinates::+]]", ["Has coordinates", "Has country", "Has growing area", "Has descriptive name", "Has collection date", "Has coordinate uncertainty"]);

const features = [
  ...Object.entries(areas).map(([title, r]) => ({
    type: "Feature", geometry: { type: "Point", coordinates: point(r) },
    properties: { kind: "area", name: title, url: r.fullurl, country: name(r.printouts["Has country"]), region: name(r.printouts["Has growing region"]) },
  })),
  ...Object.entries(accessions).map(([title, r]) => {
    const date = first(r.printouts["Has collection date"])?.raw ?? null; // "1/1940/4/7": calendar/year/month/day
    const unc = first(r.printouts["Has coordinate uncertainty"]);
    return {
      type: "Feature", geometry: { type: "Point", coordinates: point(r) },
      properties: {
        kind: "accession", name: first(r.printouts["Has descriptive name"]) ?? title, id: title, url: r.fullurl,
        country: name(r.printouts["Has country"]), area: name(r.printouts["Has growing area"]),
        year: date ? Number(date.split("/")[1]) || null : null,
        uncertainty_m: unc ? Math.round(unc.unit === "km" ? unc.value * 1000 : unc.value) : null,
      },
    };
  }),
].filter((f) => f.geometry.coordinates);

const out = {
  type: "FeatureCollection",
  attribution: "Landrace.wiki contributors",
  licence: "CC BY-SA 4.0 (https://creativecommons.org/licenses/by-sa/4.0/)",
  source: `${API} (action=ask: [[Category:Growing Areas]] and [[Category:Accessions]] with coordinates)`,
  retrieved: new Date().toISOString().slice(0, 10),
  note: "Coordinates are offset by about 500 m by Landrace.wiki; some mark a settlement or district.",
  features,
};
// the outlines
const BANDS = { Areas: ["Americas", "West", "Central", "Central-East", "East", "East-100"], Regions: ["Americas", "West", "Central", "Central-East", "East", "East-95", "East-100"] };
const shapes = [];
for (const [tier, bands] of Object.entries(BANDS)) {
  for (const band of bands) {
    const found = entries(await source(`Module:Boundaries/${tier}/${band}`));
    console.log(`Module:Boundaries/${tier}/${band}: ${Object.keys(found).length}`);
    for (const [key, e] of Object.entries(found)) {
      const r = rings(e.polygon);
      if (!r.length) continue;
      shapes.push({
        type: "Feature",
        geometry: r.length === 1 ? { type: "Polygon", coordinates: r } : { type: "MultiPolygon", coordinates: r.map((x) => [x]) },
        properties: { kind: tier === "Areas" ? "area" : "region", name: key.replace(/ \(growing (area|region)\)$/, ""), url: `https://landrace.wiki/wiki/${encodeURIComponent(key.replace(/ /g, "_"))}`, style: e.style, source: e.source },
      });
    }
  }
}
// a growing area with an outline is drawn as its outline, not as a circle
const outlined = new Set(shapes.filter((s) => s.properties.kind === "area").map((s) => s.properties.name));
for (const f of features) if (f.properties.kind === "area") f.properties.outlined = outlined.has(f.properties.name);

fs.writeFileSync("public/layers/landrace.geojson", JSON.stringify(out) + "\n");
const meta = { ...out, source: `${API} (Module:Boundaries/Areas/* and Module:Boundaries/Regions/*), simplified to about 1/500 of each shape's extent`, note: "Outlines: style is editorial (drawn by hand), admin (an administrative unit) or derived." };
for (const kind of ["area", "region"]) {
  const list = shapes.filter((s) => s.properties.kind === kind);
  fs.writeFileSync(`public/layers/landrace-${kind}s.geojson`, JSON.stringify({ ...meta, features: list }) + "\n");
  console.log(`public/layers/landrace-${kind}s.geojson: ${list.length} outlines`);
}
console.log(`public/layers/landrace.geojson: ${features.filter((f) => f.properties.kind === "area").length} growing areas, ${features.filter((f) => f.properties.kind === "accession").length} accessions`);
