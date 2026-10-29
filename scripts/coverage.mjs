// What the tracker already has, to choose where to look next and to avoid duplicates.
//
//   node scripts/coverage.mjs                  countries with reports, by region, and regions with none
//   node scripts/coverage.mjs --country ng     that country's reports with dates and source URLs
//   node scripts/coverage.mjs --url <url>      whether a source URL is already cited, and where
import fs from "node:fs";
import path from "node:path";
import * as yaml from "js-yaml";

const countries = JSON.parse(fs.readFileSync("src/data/countries.json", "utf8"));
const byId = new Map(countries.map((c) => [c.id, c]));
const reports = fs.readdirSync("reports", { recursive: true }).filter((f) => f.endsWith(".md")).map((f) => {
  const text = fs.readFileSync(path.join("reports", f), "utf8");
  const data = yaml.load(text.match(/^---\n([\s\S]*?)\n---/)[1], { schema: yaml.JSON_SCHEMA });
  return { file: path.join("reports", f), slug: path.basename(f, ".md"), ...data };
}).sort((a, b) => String(b.date).localeCompare(String(a.date)));

const arg = (k) => { const i = process.argv.indexOf(k); return i > 0 ? process.argv[i + 1] : null; };
const norm = (u) => u.replace(/^https?:\/\/(www\.)?/, "").replace(/[?#].*$/, "").replace(/\/$/, "");

if (arg("--url")) {
  const u = norm(arg("--url"));
  const hits = reports.filter((r) => r.sources.some((s) => norm(s.url) === u) || r.media?.some((m) => norm(m.url) === u));
  console.log(hits.length ? hits.map((r) => `already in ${r.file}`).join("\n") : "not cited yet");
} else if (arg("--country")) {
  const id = arg("--country");
  const list = reports.filter((r) => r.country === id || byId.get(r.country)?.parent === id);
  console.log(`${byId.get(id)?.name ?? id}: ${list.length} reports`);
  for (const r of list) {
    console.log(`\n${r.date}  ${r.type.padEnd(12)} ${r.title}\n  ${r.file}`);
    for (const s of r.sources) console.log(`  - ${s.url}`);
  }
} else {
  const count = new Map();
  for (const r of reports) count.set(r.country, (count.get(r.country) ?? 0) + 1);
  const regions = [...new Set(countries.filter((c) => !c.parent).map((c) => c.region))].sort();
  console.log(`${reports.length} reports, ${count.size} countries and states\n`);
  for (const region of regions) {
    const inRegion = [...count].filter(([id]) => (byId.get(id)?.region) === region).sort((a, b) => b[1] - a[1]);
    console.log(`${region}: ${inRegion.length ? inRegion.map(([id, n]) => `${byId.get(id).name} ${n}`).join(", ") : "none yet"}`);
  }
  const types = {};
  for (const r of reports) types[r.type] = (types[r.type] ?? 0) + 1;
  console.log(`\nBy type: ${Object.entries(types).map(([t, n]) => `${t} ${n}`).join(", ")}`);
  console.log(`Latest: ${reports[0]?.date ?? "–"}`);
}
