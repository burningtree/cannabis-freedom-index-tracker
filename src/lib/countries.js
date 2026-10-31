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
// Use the CFI's explicit flags for territories; states inherit their country's flag.
export const flagOf = (id) => {
  const country = id.toLowerCase().split("/")[0];
  const flag = byId.get(id.toLowerCase())?.flag ?? byId.get(country)?.flag;
  if (flag) return flag;
  const cc = country.toUpperCase();
  return /^[A-Z]{2}$/.test(cc) ? String.fromCodePoint(...[...cc].map((ch) => 0x1f1a5 + ch.charCodeAt(0))) : "";
};
