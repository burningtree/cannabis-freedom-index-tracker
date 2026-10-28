// The places of the Cannabis Freedom Index (copied by scripts/sync-countries.mjs), so a report's
// `country` is the same id the CFI uses, and its page can link to the CFI profile.
import list from "../data/countries.json";

export const CFI = "https://cannabisfreedom.fyi";
export const countries = list;
const byId = new Map(list.map((c) => [c.id, c]));
export const countryOf = (id) => byId.get(id);
export const countryName = (id) => {
  const c = byId.get(id);
  if (!c) return id;
  return c.parent ? `${c.name}, ${byId.get(c.parent)?.name ?? c.parent}` : c.name;
};
// the CFI profile of a country or state
export const cfiUrl = (id) => `${CFI}/country/${id}/`;
// a flag emoji from the two-letter id (a state gets its country's flag)
export const flagOf = (id) => {
  const cc = id.split("/")[0].toUpperCase();
  return /^[A-Z]{2}$/.test(cc) ? String.fromCodePoint(...[...cc].map((ch) => 0x1f1a5 + ch.charCodeAt(0))) : "";
};
