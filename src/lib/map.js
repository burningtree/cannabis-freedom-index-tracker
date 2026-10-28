// Where a report goes on the map when it gives no coordinates of its own: the middle of its
// country, from world-atlas, as [lat, lon]. A state or province falls back to its country.
import { geoCentroid } from "d3-geo";
import { feature } from "topojson-client";
import detail from "world-atlas/countries-50m.json";
import { countries } from "./countries.js";

// the basemap: OpenFreeMap's dark vector style (OpenStreetMap data, no key needed)
export const MAP_STYLE = "https://tiles.openfreemap.org/styles/dark";

const centres = new Map(feature(detail, detail.objects.countries).features.map((f) => {
  const [lon, lat] = geoCentroid(f);
  return [+f.id, [+lat.toFixed(3), +lon.toFixed(3)]];
}));
export function centreOf(id) {
  const c = countries.find((x) => x.id === id);
  if (!c) return null;
  if (c.parent) return centreOf(c.parent);
  return c.iso_numeric != null ? centres.get(c.iso_numeric) ?? null : null;
}
