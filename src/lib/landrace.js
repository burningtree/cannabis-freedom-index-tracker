// Which Landrace.wiki growing areas and regions a place lies in, from the map's snapshots in
// public/layers/ (CC BY-SA 4.0), worked out once at build time for the report pages: where cannabis
// has been grown for generations, next to where it was destroyed.
import fs from "node:fs";
import path from "node:path";

const read = (file) => JSON.parse(fs.readFileSync(path.join(process.cwd(), "public/layers", file), "utf8")).features;
let layers = null;
const load = () => (layers ??= { areas: read("landrace-areas.geojson"), regions: read("landrace-regions.geojson"), points: read("landrace.geojson") });

// even-odd ray casting over every ring of a polygon, so holes count as outside; the outlines are small
// enough for a flat test
function inRings(rings, [lon, lat]) {
  let inside = false;
  for (const ring of rings) {
    for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
      const [xi, yi] = ring[i], [xj, yj] = ring[j];
      if ((yi > lat) !== (yj > lat) && lon < ((xj - xi) * (lat - yi)) / (yj - yi) + xi) inside = !inside;
    }
  }
  return inside;
}
const contains = (geometry, point) => (geometry.type === "Polygon" ? inRings(geometry.coordinates, point)
  : geometry.type === "MultiPolygon" ? geometry.coordinates.some((rings) => inRings(rings, point)) : false);

const km = ([lon1, lat1], [lon2, lat2]) => {
  const rad = Math.PI / 180, a = Math.sin(((lat2 - lat1) * rad) / 2) ** 2 + Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(((lon2 - lon1) * rad) / 2) ** 2;
  return 12742 * Math.asin(Math.sqrt(a));
};
const brief = (f) => ({ name: f.properties.name, url: f.properties.url, status: f.properties.status ?? null });

// coordinates: [lat, lon]. Returns { areas, regions }, each [{ name, url, status }]. A growing area
// known only by a point (no outline) counts when it is within 15 km.
export function landraceAt(coordinates) {
  if (!coordinates) return { areas: [], regions: [] };
  const point = [coordinates[1], coordinates[0]];
  const { areas, regions, points } = load();
  const inAreas = areas.filter((f) => contains(f.geometry, point)).map(brief);
  for (const f of points) {
    if (f.properties.kind === "area" && !f.properties.outlined && km(f.geometry.coordinates, point) <= 15 && !inAreas.some((a) => a.name === f.properties.name)) inAreas.push(brief(f));
  }
  return { areas: inAreas, regions: regions.filter((f) => contains(f.geometry, point)).map(brief) };
}
