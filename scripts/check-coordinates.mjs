// Offline review aid, not an assertion that a report's coordinates are wrong.
import fs from "node:fs";
import path from "node:path";
import { load, JSON_SCHEMA } from "js-yaml";
import { checkCoordinates } from "./lib/coordinate-check.mjs";
import { matchingReview } from "./lib/coordinate-reviews.mjs";
const reviews = JSON.parse(fs.readFileSync(new URL("./data/coordinate-reviews.json", import.meta.url), "utf8"));

const args = process.argv.slice(2);
const json = args.includes("--json"), strict = args.includes("--strict");
const files = args.filter(arg => !arg.startsWith("--"));
const unknown = args.filter(arg => arg.startsWith("--") && !["--json", "--strict"].includes(arg));
if (unknown.length) throw new Error(`Unknown option: ${unknown.join(", ")}`);
const paths = files.length ? files : fs.readdirSync("reports", { recursive: true }).filter(f => f.endsWith(".md")).map(f => path.join("reports", f));
const summary = { total: paths.length, checked: 0, inside: 0, outside: 0, invalid: 0, unmapped: 0, noCoordinates: 0, parentOnly: 0, reviewed: 0, pending: 0 };
const findings = [];
for (const file of paths) {
  const text = fs.readFileSync(file, "utf8");
  const frontmatter = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  if (!frontmatter) throw new Error(`${file}: missing front matter`);
  let data;
  try { data = load(frontmatter[1], { schema: JSON_SCHEMA }); }
  catch { throw new Error(`${file}: invalid YAML`); }
  if (data.coordinates == null) { summary.noCoordinates++; continue; }
  const result = checkCoordinates(data.country, data.coordinates);
  summary.checked++; summary[result.status]++; if (result.parentOnly) summary.parentOnly++;
  if (result.status !== "inside") {
    const review = result.status === "outside" ? matchingReview(reviews, path.relative(process.cwd(), path.resolve(file)), data) : null;
    if (review) summary.reviewed++; else summary.pending++;
    findings.push({ file, country: data.country, place: data.place ?? null, coordinates: data.coordinates, ...result, review });
  }
}
const notes = [
  "Natural Earth 1:10m country land polygons via world-atlas; not territorial waters or authoritative borders.",
  "Outside is a review flag, not proof of an error: offshore events, coastlines, omitted islands and disputed borders need source checks.",
  "Inside does not verify the named town, district or farm. States/provinces are checked only against their parent country.",
  "Coordinate variants are suggestions to investigate, never automatic corrections. Fallback country centroids are not audited.",
];
if (json) console.log(JSON.stringify({ dataset: "world-atlas/countries-10m", summary, notes, findings }, null, 2));
else {
  console.log(`Coordinates: ${summary.inside} inside mapped country; ${summary.outside} outside; ${summary.invalid} invalid; ${summary.unmapped} unmapped; ${summary.noCoordinates} without explicit coordinates.`);
  console.log(`Review flags: ${summary.reviewed} documented reference/offshore points; ${summary.pending} pending review.`);
  for (const note of notes) console.log(note);
  for (const finding of findings) {
    console.log(`\n${finding.file}: ${finding.status.toUpperCase()} · ${finding.country} · [${finding.coordinates}]${finding.place ? ` · ${finding.place}` : ""}`);
    if (finding.review) console.log(`  Reviewed ${finding.review.reviewed_on} (${finding.review.kind}): ${finding.review.reason}`);
    if (finding.otherTerritories?.length) console.log(`  Inside mapped territory: ${finding.otherTerritories.join(", ")}`);
    else if (finding.status === "outside") console.log("  Outside mapped country land: check offshore context, coast/small-island accuracy and source location.");
    for (const variant of finding.variants ?? []) console.log(`  Verify ${variant.change}: [${variant.coordinates}] would be inside the expected country.`);
  }
}
if (strict && summary.pending) process.exitCode = 1;
