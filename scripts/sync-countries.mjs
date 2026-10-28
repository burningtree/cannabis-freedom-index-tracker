// Copies the list of places from a checkout of the Cannabis Freedom Index into
// src/data/countries.json, so a report's `country` uses exactly the CFI's ids:
// the country file's name ("de", "th") or, for a state or province, "<country>/<file>" ("us/ca").
//
//   node scripts/sync-countries.mjs [path to the CFI checkout]   (default ../cannabis-freedom-index)
import fs from "node:fs";
import path from "node:path";
import yaml from "js-yaml";

const cfi = path.resolve(process.argv[2] || process.env.CFI_DIR || "../cannabis-freedom-index");
const dir = path.join(cfi, "countries");
if (!fs.existsSync(dir)) throw new Error(`no countries/ folder in ${cfi}`);

const read = (file) => yaml.load(fs.readFileSync(file, "utf8"));
const out = [];
for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
  if (entry.isFile() && entry.name.endsWith(".yaml")) {
    const d = read(path.join(dir, entry.name));
    out.push({ id: entry.name.slice(0, -5), name: d.name, region: d.region, iso_numeric: d.iso_numeric ?? null, recreational: d.recreational, medical: d.medical });
  } else if (entry.isDirectory()) {
    const d = read(path.join(dir, entry.name, "index.yaml"));
    out.push({ id: entry.name, name: d.name, region: d.region, iso_numeric: d.iso_numeric ?? null, recreational: d.recreational, medical: d.medical });
    for (const f of fs.readdirSync(path.join(dir, entry.name)).filter((f) => f.endsWith(".yaml") && f !== "index.yaml")) {
      const s = read(path.join(dir, entry.name, f));
      out.push({ id: `${entry.name}/${f.slice(0, -5)}`, name: s.name, parent: entry.name, region: d.region, iso_numeric: null, recreational: s.recreational, medical: s.medical });
    }
  }
}
out.sort((a, b) => a.id.localeCompare(b.id));
fs.writeFileSync("src/data/countries.json", JSON.stringify(out, null, 1) + "\n");
console.log(`src/data/countries.json: ${out.length} places from ${dir}`);
