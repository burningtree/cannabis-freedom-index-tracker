// Select one continuous world copy. A negative longitude is not automatically
// an eastern-Pacific island: Argentina and Antigua must stay west of Greenwich.
export const longitudeNear = (lon, anchor) => lon + 360 * Math.round((anchor - lon) / 360);

export function pointBounds(points) {
  if (!points.length) return null;
  const xs = points.map(([x]) => ((x % 360) + 360) % 360).sort((a, b) => a - b);
  let gap = -1, start = xs[0];
  for (let i = 0; i < xs.length; i++) {
    const next = xs[(i + 1) % xs.length] + (i === xs.length - 1 ? 360 : 0);
    if (next - xs[i] > gap) { gap = next - xs[i]; start = next % 360; }
  }
  const end = start + 360 - gap;
  const shift = 360 * Math.floor(((start + end) / 2 + 180) / 360);
  return [start - shift, Math.min(...points.map(p => p[1])), end - shift, Math.max(...points.map(p => p[1]))];
}
