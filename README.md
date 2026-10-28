# Cannabis Enforcement Tracker

An open-source, global record of cannabis **eradication, seizures, raids, arrests, prosecutions,
sentences, executions and deaths**. It documents what the prohibition of cannabis does in practice:
the human, economic and cultural cost that the text of a law does not show.

Every report is short, plain and sourced: each sentence cites a source that was opened and read,
Wikipedia-style (`[@1]`). The build refuses a report without sources.

It is a companion to the [Cannabis Freedom Index](https://cannabisfreedom.fyi/) (CFI), which scores
what each country's laws allow. Country ids are the CFI's, so every report links to the law behind
it, and the CFI can show the tracker's reports on its country profiles through the JSON API below.

Site: <https://tracker.cannabisfreedom.fyi> (static, built with [Astro](https://astro.build/)).

## Pages

| Address | What it shows |
|---|---|
| `/` | Map of all reports (dots coloured by type, zoom, hover box) and the list, filterable by type, country, year and free text; filters are kept in the address (`/?type=sentence&country=ge`) |
| `/report/<slug>/` | One report: figures, text with citations, sources, map pin, link to the CFI profile, related reports |
| `/country/<id>/` | A country's or state's reports, totals and map (`/country/ng/`, `/country/us/ca/`) |
| `/country/` | All countries with reports |
| `/stats/` | Totals, reports by type and region, timeline |
| `/about/` | What a report is, the types and statuses |
| `/api/…` | JSON, see below |

The map is [MapLibre GL JS](https://maplibre.org/) on [OpenFreeMap](https://openfreemap.org/)'s
dark vector style (OpenStreetMap data, no API key). The style URL is `MAP_STYLE` in
`src/lib/map.js`.

## Adding a report

1. Find the event in a reliable source and **open it**. Prefer the agency's own statement or court
   record, plus independent reporting. Use only facts the sources state.
2. Create `reports/<year>/<month>/<slug>.md` for the date of the event. The file name is the slug and
   the report's address: lower case, digits and hyphens, unique across all years
   (`ndlea-edo-igbanke-farms`).
3. Fill in the front matter and write the text (format below). Cite every sentence with `[@n]`, the
   number of the source in `sources`. Every source must be cited at least once.
4. Run `npm test` (a second) and `npm run build`.

The full guide is [docs/WRITING.md](docs/WRITING.md); to find new events, see
[.claude/skills/find-reports/SKILL.md](.claude/skills/find-reports/SKILL.md).

Writing: lead with what happened to people (who was arrested, jailed, lost their crop or land),
then the figures, then context. Use the source's numbers, and say whose numbers they are ("the
agency valued them at …"). No law-number lists, no adjectives the source does not support.

## Data format

```yaml
---
date: 2026-08-17                 # when it happened (a campaign: when it ended or was announced)
country: ng                      # a CFI country file: "de", "th"; a state: "us/ca", "au/act"
place: Igbanke forest, Orhionmwon, Edo State     # optional
coordinates: [6.38, 6.17]        # optional, [lat, lon]; without them the dot goes to the country's middle
precision: district              # needed with coordinates: exact | locality | district | region | country
type: eradication                # eradication | seizure | raid | arrest | prosecution | sentence | execution | death
also: [arrest]                   # optional: other types the event involved
title: NDLEA destroys two cannabis farms in Edo and makes eleven arrests   # ≤ 120 characters
summary: >-                      # ≤ 400 characters, used in lists, cards and the JSON
  Nigeria's drug agency destroyed 1.85 hectares of cannabis …
figures:                         # optional; only numbers a source states
  arrested: 11                   # people detained
  charged: 1                     # people charged or tried
  kg_seized: 12.4                # cannabis taken or destroyed, kg
  plants_destroyed: 48000
  hectares: 1.85                 # fields destroyed
  prison: 5 months 24 days (time served)   # a sentence, as stated
  fine: { amount: 500000, currency: GEL }
  value: { amount: 9600000, currency: PHP }   # the authorities' estimate
  deaths: 1
cover:                           # optional; a local picture with an open licence
  image: ../../../src/assets/reports/example.jpg
  alt: What the picture shows
  credit: Photographer / agency
  licence: CC BY-SA 4.0
  url: https://commons.wikimedia.org/wiki/File:…
media:                           # optional; photos or video of the event that can't be copied here
  - kind: photo                  # photo | video
    url: https://…               # the page they are on
    publisher: La Nación (Paraguay)
    caption: Agents pull up plants in a cleared forest plot
status: verified                 # verified (checked against opened sources) | reported | disputed
tags: [ndlea, farmers]           # optional, lower-case-with-hyphens
sources:                         # at least one
  - title: NDLEA Destroys Cannabis Farms Worth 4628kg, Arrests 10 Suspected Traffickers In Edo
    url: https://saharareporters.com/2026/08/17/…
    publisher: Sahara Reporters
    date: 2026-08-17             # optional: when the source was published
    archived: https://web.archive.org/web/…   # optional
    language: es                 # optional: two-letter code if not English
---

The Edo State command of Nigeria's National Drug Law Enforcement Agency (NDLEA) destroyed two
cannabis farms … [@1] A day later, … [@2]
```

The rules live in two places, both run by `npm run build`:

- `src/content.config.ts`, the zod schema: fields, types, the CFI country ids, at least one source,
  coordinates need a precision.
- `scripts/reports.test.mjs` (`npm test`): the file sits in the folder of its date, slugs are
  unique, every citation names a source and every source is cited.

Citations are turned into numbered links by a Markdown plugin in `astro.config.mjs` (the same one as
the CFI's); it also stops the build on a citation to a missing source.

### Country ids

`src/data/countries.json` is a copy of the CFI's places (id, name, region, ISO number, legal status).
Refresh it from a CFI checkout with:

```bash
npm run countries:sync -- ../cannabis-freedom-index
```

## Build

```bash
npm install
```

```bash
npm test
```

```bash
npm run build
```

`npm run build` runs the tests first and writes the site to `dist/`. Set `SITE` and `BASE_PATH` to
build for another address. `npm run dev` starts a local dev server.

## JSON API

All files are static and rebuilt with the site. Dates are `YYYY-MM-DD`, coordinates `[lat, lon]`.

### `/api/reports.json`

Every report, newest first.

```json
{
  "version": 1,
  "generated": "2026-10-29T00:00:00.000Z",
  "count": 8,
  "types": { "eradication": { "label": "Eradication", "color": "#5f9e57" }, "…": {} },
  "by_type": { "eradication": 5, "seizure": 1, "…": 0 },
  "reports": [ /* report objects, below */ ]
}
```

### `/api/country/<id>.json`

One file for **every** CFI place, with or without reports, so a CFI profile can always fetch its
own: `/api/country/ng.json`, `/api/country/us/ca.json`. A country's file includes its states'
reports (`us.json` lists `us/ca`'s).

```json
{
  "version": 1,
  "id": "ng",
  "name": "Nigeria",
  "url": "https://tracker.cannabisfreedom.fyi/country/ng/",   // null if no reports
  "count": 1,
  "totals": { "reports": 1, "arrested": 11, "plants_destroyed": 0, "hectares": 1.85, "kg_seized": 0, "deaths": 0 },
  "by_type": { "eradication": 1, "…": 0 },
  "latest": "2026-08-17",                                       // null if no reports
  "reports": [ /* report objects */ ]
}
```

### Report object

```json
{
  "id": "ndlea-edo-igbanke-farms",
  "url": "https://tracker.cannabisfreedom.fyi/report/ndlea-edo-igbanke-farms/",
  "date": "2026-08-17",
  "country": "ng",
  "country_name": "Nigeria",
  "place": "Igbanke forest, Orhionmwon, Edo State",
  "coordinates": [6.38, 6.17],
  "precision": "district",
  "type": "eradication",
  "also": ["arrest"],
  "title": "NDLEA destroys two cannabis farms in Edo and makes eleven arrests",
  "summary": "…",
  "status": "verified",
  "figures": { "hectares": 1.85, "arrested": 11 },
  "tags": ["ndlea", "farmers", "legalisation-debate"],
  "sources": [{ "title": "…", "url": "…", "publisher": "Sahara Reporters", "date": "2026-08-17", "archived": null }]
}
```

`coordinates` falls back to the middle of the country (`precision: "country"`) when the report gives
none. `figures` holds only the keys a report states. Fields may be added in later versions; existing
fields keep their meaning while `version` is 1.

## Evidence archive

`node scripts/archive-media.mjs [slug …]` saves every picture on each report's source and media pages
to `archive/<slug>/` with a manifest (page, image URL, date fetched, SHA-256), so the evidence
survives if the pages change or disappear. `archive/` is ignored by git and not part of the site,
because most of the pictures belong to their publishers; back it up privately.

## Planned

- AI-assisted collection: drafts from news and agency feeds, saved with `status: reported`, then
  checked by a person against the opened sources before they become `verified`.
- Archived copies of every source.
- Enforcement news on the CFI's country profiles, from `/api/country/<id>.json`.

## Licence

Code: MIT. Report texts and data: CC BY 4.0. Pictures keep their own licences, stated with each.
