// Copies the places of the Cannabis Freedom Index into src/data/countries.json: their ids, so a
// report's `country` is exactly the CFI's (the country file's name, "de", or "<country>/<file>" for
// a state, "us/ca"), and what the CFI widget on a country page shows.
//
// From the CFI checkout (countries/*.yaml, segments.yaml): name, region, legal status, summary, the
// six segment scores and the date the profile was last updated. From the published CFI site
// (https://cannabisfreedom.fyi, or the checkout's dist/ with --local): the score and tier as the CFI
// shows them (search.json, which also covers states and national estimates) and the score year by
// year (map-series/freedom.json), from which the rank among countries is worked out.
//
//   node scripts/sync-countries.mjs [path to the CFI checkout] [--local]   (default ../cannabis-freedom-index)
import fs from "node:fs";
import path from "node:path";
import yaml from "js-yaml";

const args = process.argv.slice(2);
const LOCAL = args.includes("--local");
const cfi = path.resolve(args.find((a) => !a.startsWith("--")) || process.env.CFI_DIR || "../cannabis-freedom-index");
const SITE = "https://cannabisfreedom.fyi";
const dir = path.join(cfi, "countries");
if (!fs.existsSync(dir)) throw new Error(`no countries/ folder in ${cfi}`);

const read = (file) => yaml.load(fs.readFileSync(file, "utf8"));
const published = async (file) => LOCAL
  ? JSON.parse(fs.readFileSync(path.join(cfi, "dist", file), "utf8"))
  : (await fetch(`${SITE}/${file}`, { headers: { "User-Agent": "CannabisEnforcementTracker (+https://tracker.cannabisfreedom.fyi)" } })).json();

// the segments, in the CFI's order, with their names and weights
const SEGMENTS = read(path.join(cfi, "segments.yaml")).map((s) => ({ id: s.id, name: s.name, weight: s.weight }));

// the score and tier as published: "44/100 · Legal with limits", "United States · 50/100 · Legal with
// limits", "53/100 · Legal with limits · national estimate; provinces and territories 40–54"
const shown = new Map();
for (const row of await published("search.json")) {
  if (row.t !== "c") continue;
  const id = row.u.replace(/^country\//, "").replace(/\/$/, "");
  const m = row.s.match(/(\d+)\/100 · ([^·]+?)(?: · (.+))?$/);
  if (m) shown.set(id, { score: Number(m[1]), tier: m[2].trim(), note: m[3] ?? null });
}
// the score year by year (countries only, by upper-case id): [year, score, …segments]; keep the
// years it changed since 1900, and the last, for the trend line
const series = await published("map-series/freedom.json");
const trendOf = (id) => {
  const rows = series[id.toUpperCase()];
  if (!rows) return null;
  const pts = rows.filter((r) => r[0] >= 1900).map((r) => [r[0], Math.round(r[1] * 10) / 10]);
  return pts.filter(([, v], i) => i === 0 || i === pts.length - 1 || v !== pts[i - 1][1]);
};
// the rank among countries, by the precise latest score (equal scores share a rank)
const latest = Object.entries(series).map(([id, rows]) => [id.toLowerCase(), rows.at(-1)[1]]).sort((a, b) => b[1] - a[1]);
const rankOf = (id) => { const i = latest.findIndex(([x]) => x === id); return i < 0 ? null : latest.findIndex(([, v]) => v === latest[i][1]) + 1; };

const place = (id, d, extra = {}) => ({
  id, name: d.name, ...extra,
  recreational: d.recreational, medical: d.medical,
  cfi: {
    ...(shown.get(id) ?? { score: null, tier: null, note: null }),
    rank: extra.parent ? null : rankOf(id),
    segments: d.scores ? SEGMENTS.map((s) => d.scores[s.id] ?? null) : null,
    summary: d.summary?.trim().replace(/\s+/g, " ") ?? null,
    updated: d.updated ? String(d.updated).slice(0, 10) : null,
    trend: extra.parent ? null : trendOf(id),
  },
});

const out = [];
for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
  if (entry.isFile() && entry.name.endsWith(".yaml")) {
    const d = read(path.join(dir, entry.name));
    out.push(place(entry.name.slice(0, -5), d, { region: d.region, iso_numeric: d.iso_numeric ?? null }));
  } else if (entry.isDirectory()) {
    const d = read(path.join(dir, entry.name, "index.yaml"));
    out.push(place(entry.name, d, { region: d.region, iso_numeric: d.iso_numeric ?? null }));
    for (const f of fs.readdirSync(path.join(dir, entry.name)).filter((f) => f.endsWith(".yaml") && f !== "index.yaml")) {
      const s = read(path.join(dir, entry.name, f));
      out.push(place(`${entry.name}/${f.slice(0, -5)}`, s, { parent: entry.name, region: d.region, iso_numeric: null }));
    }
  }
}
out.sort((a, b) => a.id.localeCompare(b.id));
fs.writeFileSync("src/data/countries.json", JSON.stringify(out, null, 1) + "\n");
fs.writeFileSync("src/data/segments.json", JSON.stringify(SEGMENTS, null, 1) + "\n");
console.log(`src/data/countries.json: ${out.length} places (${out.filter((c) => c.cfi.score != null).length} with a published score, ${latest.length} ranked), from ${dir} and ${LOCAL ? "dist/" : SITE}`);
