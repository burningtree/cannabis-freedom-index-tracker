// Offline review aid: identical / near-identical pins reused across reports.
import fs from "node:fs";
import path from "node:path";
import { load, JSON_SCHEMA } from "js-yaml";
import { findDuplicateCoordinateGroups } from "./lib/duplicate-coordinates.mjs";

const args = process.argv.slice(2);
const json = args.includes("--json");
const strict = args.includes("--strict");
const metersArg = args.find((a) => a.startsWith("--meters="));
const meters = metersArg ? Number(metersArg.slice("--meters=".length)) : 150;
const files = args.filter((arg) => !arg.startsWith("--"));
const unknown = args.filter(
  (arg) => arg.startsWith("--") && !["--json", "--strict"].includes(arg) && !arg.startsWith("--meters=")
);
if (unknown.length) throw new Error(`Unknown option: ${unknown.join(", ")}`);
if (!Number.isFinite(meters) || meters < 0) throw new Error(`Invalid --meters=${metersArg?.slice(9)}`);

const paths = files.length
  ? files
  : fs.readdirSync("reports", { recursive: true }).filter((f) => f.endsWith(".md")).map((f) => path.join("reports", f));

const reports = [];
for (const file of paths) {
  const text = fs.readFileSync(file, "utf8");
  const frontmatter = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  if (!frontmatter) throw new Error(`${file}: missing front matter`);
  let data;
  try {
    data = load(frontmatter[1], { schema: JSON_SCHEMA });
  } catch {
    throw new Error(`${file}: invalid YAML`);
  }
  if (data.coordinates == null) continue;
  reports.push({
    file,
    country: data.country,
    place: data.place ?? null,
    precision: data.precision ?? null,
    coordinates: data.coordinates,
  });
}

const groups = findDuplicateCoordinateGroups(reports, { meters });
const summary = {
  totalReports: paths.length,
  withCoordinates: reports.length,
  meters,
  groups: groups.length,
  review: groups.filter((g) => g.kind === "review").length,
  related: groups.filter((g) => g.kind === "related").length,
  shared_facility: groups.filter((g) => g.kind === "shared_facility").length,
  same_place: groups.filter((g) => g.kind === "same_place").length,
};

const notes = [
  "Clusters pins within --meters (default 150) in the same country. Does not move coordinates.",
  "review = unrelated place labels on the same pin: prioritize fixing. related/shared_facility/same_place are often expected.",
  "Airport, port and terminal repeats are labelled shared_facility. Multi-site reports that share a token with another pin are related.",
];

if (json) {
  console.log(JSON.stringify({ summary, notes, groups }, null, 2));
} else {
  console.log(
    `Duplicate coordinates: ${summary.groups} groups within ${meters} m ` +
      `(${summary.review} review, ${summary.related} related, ${summary.shared_facility} shared facility, ${summary.same_place} same place) ` +
      `among ${summary.withCoordinates} reports with explicit coordinates.`
  );
  for (const note of notes) console.log(note);
  for (const group of groups) {
    const tag = group.exact ? "exact" : `≤${group.meters} m`;
    console.log(`\n${group.kind.toUpperCase()} · ${group.country} · ${tag} · [${group.coordinates}]`);
    console.log(`  Places: ${group.places.map((p) => p ?? "(none)").join(" · ")}`);
    for (const r of group.reports) {
      console.log(`  ${r.file}`);
      console.log(`    ${r.precision ?? "?"} · ${r.place ?? "(no place)"} · [${r.coordinates}]`);
    }
  }
}

if (strict && summary.review) process.exitCode = 1;
