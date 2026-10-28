// One file per place of the Cannabis Freedom Index, with or without reports, so the CFI can ask
// for any country's file without a missing page: /api/country/de.json, /api/country/us/ca.json.
// A country's file also lists its states' reports.
import { reportsOf, compact, totals, countByType } from "../../../lib/reports.js";
import { countries, countryName } from "../../../lib/countries.js";

export function getStaticPaths() {
  return countries.map((c) => ({ params: { id: c.id } }));
}

export function GET({ params, site }) {
  const list = reportsOf(params.id);
  const body = {
    version: 1,
    id: params.id,
    name: countryName(params.id),
    url: list.length ? new URL(`country/${params.id}/`, site).href : null,
    count: list.length,
    totals: totals(list),
    by_type: countByType(list),
    latest: list[0] ? list[0].date.toISOString().slice(0, 10) : null,
    reports: list.map((r) => compact(r, site)),
  };
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } });
}
