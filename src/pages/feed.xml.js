// An RSS feed of the 50 newest reports.
import { reports } from "../lib/reports.js";
import { rss } from "../lib/feed.js";

export function GET({ site }) {
  return rss({ title: "Cannabis Enforcement Tracker", link: String(site), description: "New reports of cannabis eradication, seizures, raids, arrests, prosecutions and sentences.", list: reports, site });
}
