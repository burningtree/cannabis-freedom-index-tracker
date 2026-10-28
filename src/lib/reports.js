// All reports, sorted newest first, with what the pages and the JSON files need worked out once:
// the country's name, where the dot goes on the map, the year, and the figures as short labels.
import { getCollection } from "astro:content";
import { TYPES } from "./types.js";
import { countryName, countryOf } from "./countries.js";
import { centreOf } from "./map.js";

const fmtNum = (n) => n.toLocaleString("en-GB", { maximumFractionDigits: 1 });
const fmtMoney = (m) => `${m.currency} ${fmtNum(m.amount)}`;
export const formatDate = (d) => d.toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
export const isoDate = (d) => d.toISOString().slice(0, 10);

// the figures a report gives, in reading order, as { key, label, value }
export function figureRows(f = {}) {
  const rows = [];
  if (f.arrested != null) rows.push({ key: "arrested", label: "Arrested", value: fmtNum(f.arrested) });
  if (f.charged != null) rows.push({ key: "charged", label: "Charged", value: fmtNum(f.charged) });
  if (f.plants_destroyed != null) rows.push({ key: "plants_destroyed", label: "Plants destroyed", value: fmtNum(f.plants_destroyed) });
  if (f.hectares != null) rows.push({ key: "hectares", label: "Hectares", value: fmtNum(f.hectares) });
  if (f.kg_seized != null) rows.push({ key: "kg_seized", label: "Cannabis", value: `${fmtNum(f.kg_seized)} kg` });
  if (f.prison) rows.push({ key: "prison", label: "Prison", value: f.prison });
  if (f.fine) rows.push({ key: "fine", label: "Fine", value: fmtMoney(f.fine) });
  if (f.deaths != null) rows.push({ key: "deaths", label: "Deaths", value: fmtNum(f.deaths) });
  if (f.value) rows.push({ key: "value", label: "Value (official estimate)", value: fmtMoney(f.value) });
  return rows;
}

const entries = await getCollection("reports");
export const reports = entries.map(({ id, data, body }) => {
  const coordinates = data.coordinates ?? centreOf(data.country);
  return {
    id, ...data, body,
    url: `report/${id}/`,
    countryName: countryName(data.country),
    parent: countryOf(data.country)?.parent ?? null,
    year: data.date.getUTCFullYear(),
    typeLabel: TYPES[data.type].label,
    color: TYPES[data.type].color,
    coordinates,
    precision: data.coordinates ? data.precision : coordinates ? "country" : null,
    figureRows: figureRows(data.figures),
  };
}).sort((a, b) => b.date - a.date || a.title.localeCompare(b.title));

// the reports about a country, its states included ("us" also lists "us/ca")
export const reportsOf = (id) => reports.filter((r) => r.country === id || r.parent === id);

// totals of the figures over a set of reports
export function totals(list) {
  const sum = (k) => list.reduce((s, r) => s + (r.figures?.[k] ?? 0), 0);
  return {
    reports: list.length,
    arrested: sum("arrested"),
    plants_destroyed: sum("plants_destroyed"),
    hectares: +sum("hectares").toFixed(2),
    kg_seized: +sum("kg_seized").toFixed(1),
    deaths: sum("deaths") + list.filter((r) => r.type === "execution" && r.figures?.deaths == null).length,
  };
}

export const countByType = (list) => Object.fromEntries(Object.keys(TYPES).map((t) => [t, list.filter((r) => r.type === t).length]));

// the countries that have reports, with their count, most first
export const countriesWithReports = () => {
  const m = new Map();
  for (const r of reports) m.set(r.country, (m.get(r.country) ?? 0) + 1);
  return [...m].map(([id, n]) => ({ id, name: countryName(id), count: n })).sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
};

// reports like this one: same country first, then same type, newest first
export function related(r, n = 4) {
  const score = (o) => (o.country === r.country || o.parent === r.country || r.parent === o.country ? 2 : 0) + (o.type === r.type ? 1 : 0);
  return reports.filter((o) => o.id !== r.id && score(o) > 0).sort((a, b) => score(b) - score(a) || b.date - a.date).slice(0, n);
}

// the compact form used by the JSON files (see README: "JSON API")
export function compact(r, site) {
  return {
    id: r.id,
    url: new URL(r.url, site).href,
    date: isoDate(r.date),
    country: r.country,
    country_name: r.countryName,
    place: r.place ?? null,
    coordinates: r.coordinates ?? null,
    precision: r.precision,
    type: r.type,
    also: r.also ?? [],
    title: r.title,
    summary: r.summary,
    status: r.status,
    figures: r.figures ?? {},
    tags: r.tags ?? [],
    sources: r.sources.map((s) => ({ title: s.title, url: s.url, publisher: s.publisher, date: s.date ? isoDate(s.date) : null, archived: s.archived ?? null })),
  };
}
