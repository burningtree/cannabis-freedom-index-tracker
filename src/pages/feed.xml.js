// An RSS feed of the 50 newest reports.
import { reports, isoDate } from "../lib/reports.js";

const esc = (s = "") => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

export function GET({ site }) {
  const items = reports.slice(0, 50).map((r) => {
    const url = new URL(r.url, site).href;
    return `<item><title>${esc(r.title)}</title><link>${url}</link><guid>${url}</guid><pubDate>${r.date.toUTCString()}</pubDate><category>${esc(r.typeLabel)}</category><description>${esc(`${r.countryName}, ${isoDate(r.date)}. ${r.summary}`)}</description></item>`;
  }).join("");
  const xml = `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Cannabis Enforcement Tracker</title><link>${site}</link><description>New reports of cannabis eradication, seizures, raids, arrests, prosecutions and sentences.</description><language>en</language>${items}</channel></rss>`;
  return new Response(xml, { headers: { "Content-Type": "application/rss+xml" } });
}
