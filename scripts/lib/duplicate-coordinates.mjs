// Offline triage for identical / near-identical map pins reused across reports.
// Advisory only: does not move coordinates or gate publication.

const FACILITY_RE =
  /\b(airport|aeroport|a[eé]roport|international airport|terminal|harbour|harbor|seaport|jetty|docks?|customs?\s+warehouse|scanning station)\b|naia|klia|igi\b|csmia|bandaranaike|\bssria\b|ramgoolam|tanger\s*med|tin\s*can|shahjalal|faleolo|roland\s+garros|pasir\s+panjang/i;

const STOP = new Set([
  "district", "county", "village", "town", "city", "state", "province", "region",
  "and", "the", "near", "area", "forest", "river", "upper", "lower", "south", "north",
  "east", "west", "municipality", "tehsil", "barangay", "sitio", "poblacion", "parish",
  "locality", "rural", "farm", "farms", "farming", "road", "street", "highway", "bridge",
  "market", "police", "station", "port", "island", "islands", "camp", "hills", "hill",
]);

export function haversineMeters([lat1, lon1], [lat2, lon2]) {
  const R = 6371000;
  const toRad = (d) => (d * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(a));
}

export function normalizePlace(place) {
  return String(place ?? "")
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/\s+/g, " ");
}

export function placeTokens(place) {
  return new Set(
    normalizePlace(place)
      .split(" ")
      .filter((w) => w.length > 3 && !STOP.has(w) && !/^\d+$/.test(w))
  );
}

function sharedTokens(a, b) {
  const tb = placeTokens(b);
  return [...placeTokens(a)].filter((t) => tb.has(t));
}

export function placeRelation(placeA, placeB) {
  const a = normalizePlace(placeA);
  const b = normalizePlace(placeB);
  if (!a || !b) return "unknown";
  if (a === b) return "same_label";
  if (a.includes(b) || b.includes(a)) return "nested_label";
  // Proximity clustering already limits this to nearby pins; airport/port repeats are expected.
  if (FACILITY_RE.test(placeA ?? "") && FACILITY_RE.test(placeB ?? "")) return "shared_facility";
  const share = sharedTokens(placeA, placeB);
  if (share.length >= 1) return "overlapping_tokens";
  return "unrelated";
}

function groupKind(relations) {
  if (relations.every((r) => r === "same_label")) return "same_place";
  if (relations.every((r) => ["same_label", "nested_label", "shared_facility"].includes(r))) {
    return "shared_facility";
  }
  if (relations.every((r) => ["same_label", "nested_label", "shared_facility", "overlapping_tokens"].includes(r))) {
    return "related";
  }
  if (relations.some((r) => r === "unrelated")) return "review";
  return "related";
}

function clusterWithin(reports, meters) {
  const parent = reports.map((_, i) => i);
  const find = (i) => (parent[i] === i ? i : (parent[i] = find(parent[i])));
  const unite = (i, j) => {
    const a = find(i), b = find(j);
    if (a !== b) parent[b] = a;
  };
  for (let i = 0; i < reports.length; i++) {
    for (let j = i + 1; j < reports.length; j++) {
      if (haversineMeters(reports[i].coordinates, reports[j].coordinates) <= meters) unite(i, j);
    }
  }
  const buckets = new Map();
  for (let i = 0; i < reports.length; i++) {
    const root = find(i);
    if (!buckets.has(root)) buckets.set(root, []);
    buckets.get(root).push(reports[i]);
  }
  return [...buckets.values()].filter((g) => g.length > 1);
}

/** @param {Array<{file:string,country:string,place?:string|null,precision?:string|null,coordinates:[number,number]}>} reports */
export function findDuplicateCoordinateGroups(reports, { meters = 150 } = {}) {
  const withCoords = reports.filter(
    (r) => Array.isArray(r.coordinates) && r.coordinates.length === 2 &&
      Number.isFinite(r.coordinates[0]) && Number.isFinite(r.coordinates[1])
  );
  const byCountry = Map.groupBy(withCoords, (r) => r.country);
  const groups = [];
  for (const [country, list] of byCountry) {
    for (const items of clusterWithin(list, meters)) {
      const relations = [];
      for (let i = 0; i < items.length; i++) {
        for (let j = i + 1; j < items.length; j++) {
          relations.push(placeRelation(items[i].place, items[j].place));
        }
      }
      const kind = groupKind(relations);
      let maxGap = 0;
      for (let i = 0; i < items.length; i++) {
        for (let j = i + 1; j < items.length; j++) {
          maxGap = Math.max(maxGap, haversineMeters(items[i].coordinates, items[j].coordinates));
        }
      }
      const exact = items.every(
        (r) => r.coordinates[0] === items[0].coordinates[0] && r.coordinates[1] === items[0].coordinates[1]
      );
      groups.push({
        country,
        kind,
        exact,
        meters: Math.round(maxGap),
        coordinates: items[0].coordinates,
        places: [...new Set(items.map((r) => r.place ?? null))],
        relations: [...new Set(relations)],
        reports: items.map((r) => ({
          file: r.file,
          place: r.place ?? null,
          precision: r.precision ?? null,
          coordinates: r.coordinates,
        })),
      });
    }
  }
  const order = { review: 0, related: 1, shared_facility: 2, same_place: 3 };
  return groups.sort(
    (a, b) => (order[a.kind] - order[b.kind]) || b.reports.length - a.reports.length || a.country.localeCompare(b.country)
  );
}
