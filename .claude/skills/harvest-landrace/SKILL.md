---
name: harvest-landrace
description: Harvest cannabis enforcement news leads from landrace.wiki Portal:Current_Events — count absolutely every News item via the MediaWiki API, dedupe against the tracker, and show the scan percentage.
---

# Harvest Landrace.wiki Current Events

Turn `https://landrace.wiki/wiki/Portal:Current_Events` into a counted,
deduplicated lead list for the [find-reports](find-reports) flow. Never treat
the portal as a source itself: every lead must still be opened and verified as
its own article before it becomes a report.

## 1. Count absolutely everything (the denominator)

Do not count what the portal page renders — it shows only a recent window.
Enumerate every `News:` page through the MediaWiki API, following `continue`
paging until it is exhausted:

```sh
curl -s -A "Mozilla/5.0" --max-time 30 \
  "https://landrace.wiki/api.php?action=query&list=allpages&apprefix=News:&aplimit=500&format=json&formatversion=2"
```

Each page title has the form `News:<date>/<Type>/<slug>`, e.g.
`News:2026-10-26/Eradication/lebanese-army-destroys-...`. Record:

- the **absolute total** of `News:` pages (all continuations included);
- the per-month and per-`<Type>` breakdown (count the second path segment).

Known types include `Eradication`, `Trafficking`, `Enforcement`, `Policy`,
`Report`. Only `Eradication`, `Trafficking` and `Enforcement` are tracker
leads; `Policy`, `Report` and `Research` are law/statistics/science territory
(see find-reports §2b) — count them, but exclude them from the scan pool.

Use `api.php`, not the `Special:PrefixIndex` pages: those intermittently
return Cloudflare 403s, while `api.php` with a browser user-agent works.

## 2. Score each lead: scanned or pending

For every enforcement-type item, fetch its outgoing source links:

```sh
curl -s -A "Mozilla/5.0" --max-time 30 \
  "https://landrace.wiki/api.php?action=query&prop=extlinks&titles=<urlencoded-title>&ellimit=50&format=json&formatversion=2"
```

The first non-image `extlinks` URL is the primary source. Run each through
the duplicate check:

```sh
node scripts/coverage.mjs --url <source-url>
```

- **Scanned**: the source (or same event) is already a tracker report.
- **Pending**: not in the tracker — a lead still needing the find-reports
  treatment (open the article, confirm date/place/figures, second source).

## 3. Show the percentage

Print a table with, at minimum: absolute total `News:` pages, enforcement
pool (total minus Policy/Report), scanned count, pending count, and

`scanned % = 100 × scanned ÷ enforcement pool`

plus the per-type and per-month split of the pending items. Example:

| pool | scanned | pending | scanned % |
|------|---------|---------|-----------|
| 154  | 61      | 93      | 39.6%     |

Recompute from the live API on every run — never reuse an earlier count.

## 4. Hand off the pending leads

List each pending item as `date · Type · wiki title · primary source URL`,
newest first. Then continue with find-reports from step 2 (search/second
source) — the portal blurbs and wiki pages are leads only, never citations.
