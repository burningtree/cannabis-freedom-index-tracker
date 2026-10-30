// One RSS feed per CFI place, empty until its first report; its 50 newest otherwise:
// /feed/cz.xml, /feed/us/ca.xml. Linked from the country page.
import { reportsOf } from "../../lib/reports.js";
import { countries, countryName } from "../../lib/countries.js";
import { rss } from "../../lib/feed.js";

export function getStaticPaths() {
  return countries.map((c) => ({ params: { id: c.id } }));
}

export function GET({ params, site }) {
  const name = countryName(params.id);
  return rss({ title: `${name}: Cannabis Enforcement Tracker`, link: new URL(`country/${params.id}/`, site).href, description: `New reports of cannabis enforcement in ${name}.`, list: reportsOf(params.id), site });
}
