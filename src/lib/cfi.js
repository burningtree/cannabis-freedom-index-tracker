// What the Cannabis Freedom Index says about a place, for the CFI widget: copied by
// scripts/sync-countries.mjs, and refreshed in the browser from the CFI's own /api/<id>.json.
import { countries, countryOf, CFI } from "./countries.js";
import segments from "../data/segments.json";

export { CFI };
export const SEGMENTS = segments;
// the CFI's score bands and its red-to-green scale (src/lib/tiers.js and the --t0…--t5 tokens there)
export const TIERS = [
  { min: 60, name: "Free", color: "#2e9e4f" },
  { min: 40, name: "Legal with limits", color: "#9ccc55" },
  { min: 25, name: "Tolerant", color: "#f0cf4a" },
  { min: 15, name: "Restrictive", color: "#f08c3a" },
  { min: 5, name: "Prohibition", color: "#dc4a2a" },
  { min: 0, name: "Severe prohibition", color: "#a3121f" },
];
export const tierOf = (score) => TIERS.find((t) => score >= t.min) ?? TIERS.at(-1);
export const REC = { legal: "Legal", partial: "Partly legal", decriminalised: "Decriminalised", illegal: "Illegal", death: "Illegal, death penalty for trafficking" };
export const MED = { legal: "Legal", limited: "Limited", none: "None" };
const RANKED = countries.filter((c) => c.cfi?.rank != null).length;
// the place's CFI data, or null; a state also gets its country's name
export function cfiOf(id) {
  const c = countryOf(id);
  if (!c?.cfi || c.cfi.score == null) return null;
  return { ...c.cfi, id, name: c.name, parent: c.parent ? countryOf(c.parent) : null, recreational: c.recreational, medical: c.medical, ranked: RANKED };
}
// the CFI's live JSON for a country (states have none)
export const cfiApi = (id) => (id.includes("/") ? null : `${CFI}/api/${id}.json`);
