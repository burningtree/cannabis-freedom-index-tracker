// Takes a snapshot of Landrace.wiki's growing areas and accessions (landrace cannabis populations and
// collected samples) for the map's "Landrace" layers: public/layers/landrace.geojson.
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
fs.writeFileSync("public/layers/landrace.geojson", JSON.stringify(out) + "\n");
console.log(`public/layers/landrace.geojson: ${features.filter((f) => f.properties.kind === "area").length} growing areas, ${features.filter((f) => f.properties.kind === "accession").length} accessions`);
