// Every report in compact form, newest first: the feed the Cannabis Freedom Index reads.
// The format is described in README.md ("JSON API").
import { reports, compact, countByType } from "../../lib/reports.js";
import { TYPES } from "../../lib/types.js";

export function GET({ site }) {
  const body = {
    version: 1,
    generated: new Date().toISOString(),
    count: reports.length,
    types: Object.fromEntries(Object.entries(TYPES).map(([k, v]) => [k, { label: v.label, color: v.color }])),
    by_type: countByType(reports),
    reports: reports.map((r) => compact(r, site)),
  };
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } });
}
