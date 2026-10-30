// Where a report goes on the map when it gives no coordinates of its own: the middle of its
// country, from world-atlas, as [lat, lon]. A state or province falls back to its country.
import { geoCentroid } from "d3-geo";
import { atlasCountries } from "./atlas-countries.js";
import detail from "world-atlas/countries-50m.json";
import { countries } from "./countries.js";

// the basemaps: OpenFreeMap's vector styles (OpenStreetMap data, no key needed), one per theme
export const MAP_STYLES = { dark: "https://tiles.openfreemap.org/styles/dark", light: "https://tiles.openfreemap.org/styles/positron" };

const centres = new Map([...atlasCountries(detail)].map(([id, geometry]) => {
  const [lon, lat] = geoCentroid(geometry);
  return [id, [+lat.toFixed(3), +lon.toFixed(3)]];
}));
export function centreOf(id) {
  const c = countries.find((x) => x.id === id);
  if (!c) return null;
  if (c.parent) return centreOf(c.parent);
  return c.iso_numeric != null ? centres.get(c.iso_numeric) ?? null : null;
}
