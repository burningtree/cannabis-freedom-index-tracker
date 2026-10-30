// The figures and series the Statistics page draws, worked out at build time from the reports.
import { TYPES } from "./types.js";
import { weightParts } from "./figures.js";

const compactFormat = new Intl.NumberFormat("en-GB", { notation: "compact", maximumFractionDigits: 1 });
export const fmt = (n) => Math.round(n).toLocaleString("en-GB");
// a big number at a glance: in full up to 9,999, then 12.3K, 5.2M; the exact figure goes beside it
export const compact = (n) => (Math.abs(n) < 10000 ? fmt(n) : compactFormat.format(n).toUpperCase());

// the headline figures: total, how many reports state it, and the unit it is shown in
export function headlineFigures(list) {
  const stated = (k) => list.filter((r) => r.figures?.[k] != null);
  const sum = (k) => stated(k).reduce((s, r) => s + r.figures[k], 0);
  const kg = sum("kg_seized"), deaths = list.filter((r) => r.figures?.deaths != null || r.type === "execution");
  return [
    { id: "arrested", label: "People arrested", value: sum("arrested"), reports: stated("arrested").length },
    { id: "plants", label: "Plants destroyed", value: sum("plants_destroyed"), reports: stated("plants_destroyed").length },
    { id: "seized", label: "Cannabis seized", ...weightParts(kg), reports: stated("kg_seized").length },
    { id: "hectares", label: "Hectares destroyed", value: sum("hectares"), reports: stated("hectares").length },
    { id: "deaths", label: "Deaths", value: deaths.reduce((s, r) => s + (r.figures?.deaths ?? 1), 0), reports: deaths.length },
  ];
}

// reports per period with their types, for a column chart; every period between first and last
export function periodColumns(list, keyOf, periods) {
  const byKey = new Map(periods.map((p) => [p, { key: p, total: 0, types: {} }]));
  for (const r of list) {
    const c = byKey.get(keyOf(r));
    if (!c) continue;
    c.total++;
    c.types[r.type] = (c.types[r.type] ?? 0) + 1;
  }
  return [...byKey.values()];
}

// the tooltip of a column: its total and, in words, the types it is made of
export const typeRows = (types) => Object.keys(TYPES).filter((t) => types[t]).map((t) => [TYPES[t].color, TYPES[t].plural, types[t]]);

// the model that wrote a report, without its provider and reasoning setting:
// "spacexai/grok-4.7#medium" → "grok-4.7"; null when it was not recorded
export function modelName(id) {
  const v = String(id ?? "").trim();
  if (!v || v.startsWith("unknown")) return null;
  const slash = v.match(/^[\w.-]+\/(.+)$/);
  return (slash ? slash[1] : v).split("#")[0].trim();
}

// reports per model, most first, each with the exact identifiers (provider, setting) behind it
export function reportsByModel(list) {
  const models = new Map();
  let unrecorded = 0;
  for (const r of list) {
    const name = modelName(r.created_by_model);
    if (!name) { unrecorded++; continue; }
    const m = models.get(name) ?? { name, count: 0, variants: {} };
    m.count++;
    m.variants[r.created_by_model] = (m.variants[r.created_by_model] ?? 0) + 1;
    models.set(name, m);
  }
  const rows = [...models.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
  for (const m of rows) m.variants = Object.entries(m.variants).sort((a, b) => b[1] - a[1]);
  return { rows, unrecorded };
}

// the countries with the most of a figure: [{ id, value }], most first
export function topCountriesBy(list, figure, n = 8) {
  const sums = new Map();
  for (const r of list) {
    const v = r.figures?.[figure];
    if (v) sums.set(r.country, (sums.get(r.country) ?? 0) + v);
  }
  return [...sums].map(([id, value]) => ({ id, value })).sort((a, b) => b.value - a.value).slice(0, n);
}
