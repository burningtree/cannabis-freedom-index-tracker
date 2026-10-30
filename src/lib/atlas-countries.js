import { feature } from "topojson-client";

// Natural Earth can contain several features with the same ISO numeric code
// (Australia and Ashmore/Cartier, for example). Preserve all parts rather than
// letting the last tiny territory overwrite its mainland.
export function atlasCountries(atlas) {
  const countries = new Map();
  for (const f of feature(atlas, atlas.objects.countries).features) {
    if (!f.id || !/^\d+$/.test(String(f.id))) continue;
    const id = Number(f.id), parts = f.geometry.type === "Polygon" ? [f.geometry.coordinates] : f.geometry.coordinates;
    if (!countries.has(id)) countries.set(id, { type: "MultiPolygon", coordinates: [] });
    countries.get(id).coordinates.push(...parts);
  }
  return countries;
}
