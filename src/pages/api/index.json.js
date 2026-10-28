// The shared index of all reports, newest first, that the map and the home page's filters load
// once (and the browser caches): one short row per report (indexRow in src/lib/reports.js).
// Not part of the public API; use /api/reports.json.
import { reports, indexRow } from "../../lib/reports.js";
import { countryName } from "../../lib/countries.js";

export function GET() {
  const countries = Object.fromEntries([...new Set(reports.map((r) => r.country))].map((id) => [id, countryName(id)]));
  return new Response(JSON.stringify({ v: 1, countries, reports: reports.map(indexRow) }), { headers: { "Content-Type": "application/json" } });
}
