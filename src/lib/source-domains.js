// Aggregate at build time; the statistics page receives counts, never report bodies.
export function sourcesByDomain(reports) {
  const domains = new Map();
  for (const report of reports) {
    const cited = new Set();
    for (const source of report.sources ?? []) {
      let url;
      try { url = new URL(source.url); } catch { continue; }
      if (!["https:", "http:"].includes(url.protocol)) continue;
      const domain = url.hostname.replace(/^www\./, "");
      url.hostname = domain;
      url.hash = "";
      let entry = domains.get(domain);
      if (!entry) domains.set(domain, entry = { domain, urls: new Set(), reports: 0 });
      entry.urls.add(url.href);
      cited.add(domain);
    }
    for (const domain of cited) domains.get(domain).reports++;
  }
  return [...domains.values()]
    .map(({ domain, urls, reports }) => ({ domain, sources: urls.size, reports }))
    .sort((a, b) => b.sources - a.sources || b.reports - a.reports || a.domain.localeCompare(b.domain));
}
