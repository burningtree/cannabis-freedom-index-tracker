import fs from "node:fs";
import { createRequire } from "node:module";
import { feature } from "topojson-client";
import { geoArea, geoBounds, geoContains } from "d3-geo";

const require = createRequire(import.meta.url);
const atlas = JSON.parse(fs.readFileSync(require.resolve("world-atlas/countries-10m.json"), "utf8"));
const countries = JSON.parse(fs.readFileSync(new URL("../../src/data/countries.json", import.meta.url), "utf8"));
const places = new Map(countries.map(c => [c.id, c]));
// A few tiny atlas islands have reversed/quantized rings. Normalize each
// polygon to D3's smaller spherical interior, or one island can contain the globe.
const shapes = feature(atlas, atlas.objects.countries).features.map(shape => {
  const polygons = shape.geometry.type === "MultiPolygon" ? shape.geometry.coordinates : [shape.geometry.coordinates];
  const normalized = polygons.map(rings => geoArea({ type: "Polygon", coordinates: rings }) > 2 * Math.PI ? rings.map(ring => [...ring].reverse()) : rings);
  shape = { ...shape, geometry: { type: "MultiPolygon", coordinates: normalized } };
  return { shape, bounds: geoBounds(shape) };
});
const byISO = new Map();
for (const shape of shapes) {
  if (shape.shape.id == null) continue;
  const id = Number(shape.shape.id);
  if (!byISO.has(id)) byISO.set(id, []);
  byISO.get(id).push(shape); // some ISO ids have several territory features
}
const inBounds = ([lon, lat], [[west, south], [east, north]]) => lat >= south && lat <= north && (west <= east ? lon >= west && lon <= east : lon >= west || lon <= east);
const contains = (shape, point) => inBounds(point, shape.bounds) && geoContains(shape.shape, point);

// Réunion is a separate tracker country, but Natural Earth includes its island
// in France's MultiPolygon. Select only that island, never all French territory.
const reunion = (byISO.get(250) ?? []).flatMap(({ shape }) =>
  shape.geometry.coordinates.map(coordinates => ({ type: "Polygon", coordinates }))
    .filter(polygon => geoContains(polygon, [55.53, -21.12]))
    .map(geometry => {
      const shape = { type: "Feature", properties: { name: "Réunion" }, geometry };
      return { shape, bounds: geoBounds(shape) };
    }));

export function checkCoordinates(country, coordinates) {
  let place = places.get(country);
  const parentOnly = !!place?.parent;
  while (place?.parent) place = places.get(place.parent);
  const checkedCountry = place?.id ?? null;
  if (!Array.isArray(coordinates) || coordinates.length !== 2 || !coordinates.every(Number.isFinite)
    || Math.abs(coordinates[0]) > 90 || Math.abs(coordinates[1]) > 180) {
    return { status: "invalid", checkedCountry, parentOnly };
  }
  const target = country === "re" && reunion.length ? reunion : place?.iso_numeric != null ? byISO.get(Number(place.iso_numeric)) : null;
  if (!target) return { status: "unmapped", checkedCountry, parentOnly };
  const [lat, lon] = coordinates;
  const inTarget = point => target.some(shape => contains(shape, point));
  if (inTarget([lon, lat])) return { status: "inside", checkedCountry, parentOnly };
  const otherTerritories = shapes.filter(s => !target.includes(s) && contains(s, [lon, lat])).map(s => s.shape.properties.name);
  const variants = [
    ["longitude sign", [lat, -lon]], ["latitude sign", [-lat, lon]], ["latitude/longitude order", [lon, lat]],
  ].filter(([, point]) => Math.abs(point[0]) <= 90 && Math.abs(point[1]) <= 180 && inTarget([point[1], point[0]]))
    .map(([change, coordinates]) => ({ change, coordinates }));
  return { status: "outside", checkedCountry, parentOnly, otherTerritories, variants };
}
