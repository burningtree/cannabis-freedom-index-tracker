export const rowTags = row => Array.isArray(row.g ?? row.tags) ? [...new Set(row.g ?? row.tags)] : [];
export const hasTag = (row, tag) => !tag || rowTags(row).includes(tag);

export function rankedTags(rows) {
  const counts = new Map();
  for (const row of rows) for (const tag of rowTags(row)) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  return [...counts].map(([tag, count]) => ({ tag, count })).sort((a,b) => b.count - a.count || a.tag.localeCompare(b.tag, "en"));
}

export function commonTags(ranked, selected, limit = 6) {
  const shown = ranked.slice(0, limit);
  if (selected && !shown.some(t => t.tag === selected)) shown.push(ranked.find(t => t.tag === selected) ?? { tag: selected, count: 0 });
  return shown;
}
