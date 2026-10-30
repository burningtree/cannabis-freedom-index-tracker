// An RSS 2.0 feed of reports: the site's newest reports (/feed.xml) or one country's (/feed/<id>.xml).
import { isoDate } from "./reports.js";

const esc = (s = "") => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

export function rss({ title, link, description, list, site }) {
  const items = list.slice(0, 50).map((r) => {
    const url = new URL(r.url, site).href;
    return `<item><title>${esc(r.title)}</title><link>${url}</link><guid>${url}</guid><pubDate>${r.date.toUTCString()}</pubDate><category>${esc(r.typeLabel)}</category><description>${esc(`${r.countryName}, ${isoDate(r.date)}. ${r.summary}`)}</description></item>`;
  }).join("");
  const xml = `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>${esc(title)}</title><link>${esc(link)}</link><description>${esc(description)}</description><language>en</language>${items}</channel></rss>`;
  return new Response(xml, { headers: { "Content-Type": "application/rss+xml" } });
}
