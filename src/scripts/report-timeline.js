// Count each report once by its primary category, within the map's date window.
export function reportTimeline(rows, { start = "", end = "", year = "", month = "", daily = false, yearly = false } = {}) {
  const dates = rows.map(r => r.d).sort();
  let first = start || (year ? `${year}-01-01` : dates[0]);
  let last = end || (year ? `${year}-12-31` : dates.at(-1));
  if (year) {
    first = first > `${year}-01-01` ? first : `${year}-01-01`;
    last = last < `${year}-12-31` ? last : `${year}-12-31`;
  }
  if (!first || !last || first > last) return [];
  const buckets = new Map();
  const date = new Date(`${yearly ? `${first.slice(0, 4)}-01-01` : daily ? first : `${first.slice(0, 7)}-01`}T00:00:00Z`);
  while (date.toISOString().slice(0, 10) <= last) {
    const key = date.toISOString().slice(0, yearly ? 4 : daily ? 10 : 7);
    if (yearly || !month || key.slice(5, 7) === month) buckets.set(key, { key, total: 0, categories: {} });
    if (yearly) date.setUTCFullYear(date.getUTCFullYear() + 1);
    else if (daily) date.setUTCDate(date.getUTCDate() + 1);
    else date.setUTCMonth(date.getUTCMonth() + 1);
  }
  for (const row of rows) {
    if (row.d < first || row.d > last || (month && row.d.slice(5, 7) !== month)) continue;
    const bucket = buckets.get(row.d.slice(0, yearly ? 4 : daily ? 10 : 7));
    if (!bucket) continue;
    bucket.total++;
    bucket.categories[row.t] = (bucket.categories[row.t] || 0) + 1;
  }
  return [...buckets.values()];
}
