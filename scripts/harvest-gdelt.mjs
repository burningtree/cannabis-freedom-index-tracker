#!/usr/bin/env node
// GDELT DOC 2.0 lead collector. Does not publish or verify reports.
// Run from repository root: node scripts/harvest-gdelt.mjs --days 2
import fs from 'node:fs';
import path from 'node:path';
import * as yaml from "js-yaml";

const args = process.argv.slice(2);
const opt = (name, fallback) => { const p = args.indexOf(name); return p < 0 ? fallback : args[p+1]; };
const days = Number(opt('--days', '2'));
const maxWindows = Number(opt('--max-windows', '150'));
const outPath = opt('--output', '.cache/gdelt-leads.json');
const minMs = 60 * 60 * 1000;
if (!Number.isFinite(days) || days <= 0 || days > 89 || !Number.isInteger(maxWindows) || maxWindows < 1 || maxWindows > 1000) {
  console.error('Usage: node scripts/harvest-gdelt.mjs [--days 1..89] [--max-windows 1..1000] [--output file]');
  process.exit(2);
}
const queries = {
  eradication: '(cannabis OR marijuana OR marihuana OR ganja OR hashish) (eradicated OR uprooted OR destroyed OR "cultivation destroyed")',
  seizure: '(cannabis OR marijuana OR marihuana OR ganja OR hashish) (seized OR confiscated OR intercepted)',
  raid: '(cannabis OR marijuana OR marihuana OR ganja OR hashish) (raid OR raided OR busted)',
  arrest: '(cannabis OR marijuana OR marihuana OR ganja OR hashish) (arrested OR detained OR jailed)',
  sentence: '(cannabis OR marijuana OR marihuana OR ganja OR hashish) (sentenced OR convicted OR imprisoned)'
};
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const fmt = ms => new Date(ms).toISOString().replace(/[-:T]/g, '').replace(/\.\d{3}Z$/, '');
function normalized(raw) {
  try {
    const u = new URL(raw);
    if (!['https:', 'http:'].includes(u.protocol)) return null;
    u.hash = '';
    for (const k of [...u.searchParams.keys()]) if (/^(utm_|fbclid$|gclid$|mc_)/i.test(k)) u.searchParams.delete(k);
    u.hostname = u.hostname.toLowerCase().replace(/^www\./, '');
    u.pathname = u.pathname.replace(/\/$/, '') || '/';
    return `${u.hostname}${u.pathname}${u.search}`;
  } catch { return null; }
}
function existingSources() {
  const seen = new Set();
  for (const f of fs.readdirSync('reports', { recursive: true }).filter(f => f.endsWith('.md'))) {
    try {
      const body = fs.readFileSync(path.join('reports', f), 'utf8');
      const front = body.match(/^---\r?\n([\s\S]*?)\r?\n---/);
      if (!front) continue;
      const data = yaml.load(front[1], { schema: yaml.JSON_SCHEMA });
      for (const src of data?.sources ?? []) { const n = normalized(src.url); if (n) seen.add(n); }
    } catch (e) { console.warn(`Could not inspect ${f}: ${e.message}`); }
  }
  return seen;
}
const cited = existingSources();
const now = Date.now();
const start = now - days * 86400000;
const records = new Map();
const failures = [];
const capped = [];
let windows = 0;
async function fetchArticleList(query, from, to) {
  const url = new URL('https://api.gdeltproject.org/api/v2/doc/doc');
  for (const [k,v] of Object.entries({ query, mode:'artlist', format:'json', sort:'datedesc', maxrecords:'250', startdatetime:fmt(from), enddatetime:fmt(to) })) url.searchParams.set(k,v);
  let lastError;
  for (let attempt=0;attempt<4;attempt++) {
    if (attempt) await pause(Math.min(30000, 2000 * 2 ** attempt));
    try {
      const res = await fetch(url, { headers: { 'user-agent':'CannabisEnforcementTracker-GDELT-Harvest/1.0' }, signal:AbortSignal.timeout(25000) });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = await res.text();
      if (!body.trim()) return [];
      const json = JSON.parse(body);
      if (!Array.isArray(json.articles)) throw new Error('Unexpected GDELT response');
      return json.articles;
    } catch (e) { lastError = e; }
  }
  throw lastError;
}
async function scan(type, from, to) {
  if (windows >= maxWindows) { capped.push({type,from:new Date(from).toISOString(),to:new Date(to).toISOString(),reason:'max-windows'});return; }
  windows++;
  await pause(1200);
  let articles;
  try { articles = await fetchArticleList(queries[type], from, to); }
  catch(e) { failures.push({type,from:new Date(from).toISOString(),to:new Date(to).toISOString(),error:String(e)}); return; }
  // A full page may be truncated; split the interval to recover more candidates.
  if (articles.length >= 250 && to - from > minMs) {
    const midpoint = from + Math.floor((to-from)/2);
    await scan(type,from,midpoint);
    await scan(type,midpoint,to);
    return;
  }
  if (articles.length >= 250) capped.push({type,from:new Date(from).toISOString(),to:new Date(to).toISOString(),reason:'250-result-cap'});
  for (const a of articles) {
    if (!a.url) continue;
    const key = normalized(a.url);
    if (!key) continue;
    const record = records.get(key) ?? {url:a.url,title:a.title??'',seendate:a.seendate??null,domain:a.domain??null,language:a.language??null,sourcecountry:a.sourcecountry??null,query_types:[],already_cited:cited.has(key)};
    if (!record.query_types.includes(type)) record.query_types.push(type);
    records.set(key,record);
  }
  console.error(`${type}: ${new Date(from).toISOString()} → ${new Date(to).toISOString()} : ${articles.length} hits, ${records.size} unique`);
}
for (const type of Object.keys(queries)) {
  for(let from=start;from<now;from+=86400000) await scan(type,from,Math.min(now,from+86400000));
}
const leads = [...records.values()].sort((a,b)=>String(b.seendate??'').localeCompare(String(a.seendate??'')));
const result = {generated_at:new Date().toISOString(),from:new Date(start).toISOString(),to:new Date(now).toISOString(),queries,windows_requested:windows,unique_urls:leads.length,already_cited:leads.filter(r=>r.already_cited).length,new_url_leads:leads.filter(r=>!r.already_cited).length,failures,capped_windows:capped,leads};
fs.mkdirSync(path.dirname(outPath),{recursive:true});
fs.writeFileSync(outPath,JSON.stringify(result,null,2)+'\n');
console.log(`GDELT scan: ${leads.length} unique URLs; ${result.already_cited} already cited; ${result.new_url_leads} new URL leads; ${failures.length} failed and ${capped.length} capped windows.\nSaved ${outPath}`);
if (failures.length || capped.length) { console.warn('WARNING: coverage incomplete; inspect failures and capped_windows in JSON.'); process.exitCode=1; }
