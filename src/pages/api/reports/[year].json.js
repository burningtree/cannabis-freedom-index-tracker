// One year's reports, newest first, in the same form as /api/reports.json.
import { reports, years, compact } from "../../../lib/reports.js";

export const getStaticPaths = () => years.map((y) => ({ params: { year: String(y) } }));

export function GET({ params, site }) {
  const list = reports.filter((r) => r.year === Number(params.year));
  return new Response(JSON.stringify({ version: 1, year: Number(params.year), count: list.length, reports: list.map((r) => compact(r, site)) }), { headers: { "Content-Type": "application/json" } });
}
