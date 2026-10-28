// The 100 newest reports, in the same form as /api/reports.json: for a quick look or a widget.
import { reports, compact } from "../../lib/reports.js";

export function GET({ site }) {
  const list = reports.slice(0, 100);
  return new Response(JSON.stringify({ version: 1, count: list.length, total: reports.length, reports: list.map((r) => compact(r, site)) }), { headers: { "Content-Type": "application/json" } });
}
