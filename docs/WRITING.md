# Writing a report

A report tells one enforcement event, or one campaign, so that a reader understands in a minute what
happened to people, what it cost, and why it matters. The data format is in the README; this is about
what goes into a report and how it reads.

## 1. One report, one event

A report is about **one** raid, one campaign phase, one trial or one execution. Everything in it, and
every source it cites, must be about that event.

- **Sources about the same event only.** The agency's statement, the reports on it, the court's
  judgement, reactions *to this event*. An article about an earlier phase of the same operation, an
  older case or the general situation is not a source for this report, even if it adds good context.
  (The first version of the Paraguay report cited a 2025 article on phase 53 for a phase-57 report;
  that was wrong.)
- **A case that runs for months** (arrest → trial → sentence) is one report dated at its latest
  step, and may cite the earlier coverage of the same case. The LBC piece on Bella Culley's pre-trial
  hearing is a fair source for the report on her sentence.
- **Context that needs another event** gets its own report, and the two are found through the
  country page and "Related reports". Don't fold it into this one.
- **Campaign totals** (a quarter's seizures, a season's eradication) are one report when an
  authority publishes them as one result. Name the period in the title.

## 2. Open every source

Only facts from sources you have **opened and read** go in. A search-result snippet, an AI summary
or a headline is not a source.

- Prefer, in this order: the court's or agency's own statement; reporting that names its source;
  wire services; other outlets. Two independent sources are better than one for anything disputed.
- **Note who is speaking.** Most figures are the authorities' own: "the agency valued them at 9.6
  million pesos", not "plants worth 9.6 million pesos". Street values are official estimates and run
  high; mark them `value` and say whose they are.
- **When numbers differ**, use the final or official one and say that another was reported
  ("On 24 August, with the operation still under way, the count stood at 145 hectares"). If sources
  truly contradict each other, set `status: disputed` and give both.
- Check the article's **own date**, not the date in the page header. News sites show today's date
  everywhere; many "2026" pages are years old.
- A headline that disagrees with its own text (ten arrests in the title, eleven named in the body):
  use the text and count yourself.

## 3. How it reads

Plain English, short sentences, the same style as the Cannabis Freedom Index.

- **Lead with people.** Who was arrested, jailed, executed, lost a field, a home or a living. Then
  the numbers. Then the context.
- **Say what the authorities did, in their words where it matters**, and what they did not say
  ("No suspects were arrested." "The report names no arrests.").
- Explain local terms once: "Senad, Paraguay's anti-drug agency"; "a barangay, the smallest unit of
  local government".
- No adjectives the source doesn't support (no "massive", "brutal", "notorious") and no opinion of
  ours. Critics and the accused get a voice through quotes from the sources, briefly.
- No lists of law sections. Say what the law does: "importing more than 500 grams carries a
  mandatory death sentence".
- **Length:** 150–400 words. Two or three short sections (`##`) at most. The summary is two
  sentences that stand on their own in a list and in the JSON.
- **Title:** what happened, in under 90 characters, with the key number if there is one.
  "48,000 cannabis plants burned in a Kalinga communal forest", not "PDEA conducts operation".

## 4. Citations

Every sentence that states a fact ends with its source: `[@1]`, or `[@1,2]` for two. Every source
must be cited at least once; the build stops otherwise. Use the source's title and publisher as
published, with `language:` if it is not English.

## 5. Figures

Only numbers a source states. Convert units where the field asks for it (pounds → kg, acres →
hectares), keep the original in the text ("63,204 pounds (about 28.7 tonnes)"). A figure for people
(`arrested`, `charged`, `deaths`) counts the people the source names or counts for **this** event.
Leave a field out rather than guessing.

## 6. Place

Give `coordinates` for the most precise place the sources name, and say how precise with
`precision` (`locality`, `district`, `region` …). Coordinates of a town or district centre from
OpenStreetMap or Wikipedia are fine; don't guess a spot in a forest.

## 7. Pictures

A photo of the action makes a report far stronger: fields burning, agents in a plantation, a raid,
a courtroom. But most photos in news articles belong to the newspaper or the agency, and **may not be
copied** to this site.

- **`cover`: a copy on this site**, only when the licence allows it:
  - Wikimedia Commons files, Creative Commons licences (CC BY, CC BY-SA, CC0);
  - works with no copyright: US federal agencies (DEA, CBP, US Forest Service), Philippine
    government works (PDEA, PNP; RA 8293 §176), and other governments whose law says so;
  - photos released for reuse by the agency itself, with that release linked.
  Save it in `src/assets/reports/<slug>.jpg` (1200–1600 px wide), fill in `alt` (what is in the
  picture), `credit` (the photographer or agency, "via" the outlet), `licence` and `url` (where it
  and its licence can be checked).
- **`media`: links**, for every other good photo or video of the event: the page it is on, its
  publisher, and a caption saying what it shows. It is listed under the text as "Photos and video".
- Never publish a newspaper's photo, a stock photo, or a picture of a different event or of a building
  sign. A photo must show *this* event.
- **Archive everything.** Pages and pictures disappear, and they are the evidence. After writing a
  report, run `node scripts/archive-media.mjs <slug>`: it keeps a copy of every picture on the
  report's source and media pages in `archive/<slug>/`, with a manifest (page, image URL, date,
  SHA-256). `archive/` is not committed or published; back it up privately. Then run
  `node scripts/archive-sources.mjs <slug>` to add the Wayback Machine's copy of each source and
  media page as `archived:` (`--save` asks it to save pages it lacks).

## 8. Status

- `verified`: every fact was read in a source that was opened. The default for a finished report.
- `reported`: drafted from a source not yet checked in full (automated collection will use this).
- `disputed`: sources contradict each other, or the official account is contested.

## Checklist

- [ ] One event; every source is about it.
- [ ] Every source opened; its own publication date checked.
- [ ] Every factual sentence cited; every source cited.
- [ ] Figures only as stated, units converted, whose estimate said.
- [ ] Coordinates with precision.
- [ ] A cover only with an open licence; other photos as `media` links.
- [ ] Pictures archived (`node scripts/archive-media.mjs <slug>`); `archived` source copies where they exist.
- [ ] `npm test` and `npm run build` pass.
