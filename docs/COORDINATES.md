# Coordinate checks

Run an offline country-boundary audit:

```sh
npm run coordinates:check
node scripts/check-coordinates.mjs reports/2026/10/fiji-northern-1600-plants-raids.md
mkdir -p .cache
node scripts/check-coordinates.mjs --json > .cache/coordinate-audit.json
```

The checker reads explicit `[latitude, longitude]` coordinates from report front matter and uses
Natural Earth's 1:10m country land polygons, bundled in `world-atlas/countries-10m.json`.
It uses spherical containment, including polygons crossing ±180°, and checks every feature
sharing a country ISO id. Tiny reversed island rings are normalized before containment checks.
The larger dataset is used only by the offline scripts, never added to a website page.

Results:

- **inside**: the point falls inside the mapped country land. This does not verify the town,
  district, farm or report's stated precision.
- **outside**: review needed. The output identifies another mapped territory when one contains
  the point; otherwise the point may be offshore, on a simplified coastline or on an omitted island.
- **unmapped**: no polygon is available for the country id; no geographic conclusion is made.
- **invalid**: coordinates are not a numeric `[lat, lon]` pair within valid ranges.
- Reports without explicit coordinates are counted separately; fallback country centroids are not audited.

States/provinces are checked against their parent country's polygons only. These land shapes
are not territorial waters, authoritative legal borders or subnational boundaries. Border
crossings, contested areas, reclaimed land, airports, ports and small islands can legitimately
trigger a warning. An offshore result must be checked against the source; it is not proof of a
maritime event or a coordinate error.

The output also suggests a reversed longitude sign, latitude sign or swapped coordinate order
when that variant would fall in the expected country. These are clues, never automatic fixes.
Verify the named place and event source before editing. When exact farm coordinates are not
available, use a sourced locality/region reference point with the matching `precision`, rather
than inventing a more specific pin. Explain material uncertainty in `open_questions`.

Réunion is checked against its own island polygon extracted from France's atlas geometry;
mainland France does not count as Réunion.

The normal audit prints findings and exits successfully. `--strict` exits with status 1 for
any pending outside, invalid or unmapped finding. Documented geographic reference points and
offshore cases are stored in `scripts/data/coordinate-reviews.json`, with opened event and
geographic evidence URLs. They remain visible as **outside**, with their review attached;
they are not relabelled as inside or exact event locations. A review expires automatically if
the file's country, place, precision or coordinates change. Invalid and unmapped findings
cannot be exempted. The audit does not change reports or introduce a publication approval gate.

See [the initial review](COORDINATE-REVIEW-2026-10-30.md) for the corrected locations and
remaining geographic uncertainty. Do not copy an existing review to silence a new flag.

## Proposed checks within a country

Country containment alone cannot detect a pin in the wrong city. A second, separate review
pass should compare the report's pin with an independently resolved geographic reference.
This is a proposed extension, not a check the current command performs.

1. Resolve the source-named place using its country and administrative context. Cache candidate
   results and evidence, including stable geographic feature identifiers, boundaries or bounding
   boxes, and the date checked. Ambiguous names and missing matches need review; selecting the
   first geocoder result is not sufficient evidence. Reverse-geocoding a pin alone is also not
   independent confirmation of the source-named location.
2. Compare the pin against the expected district/state polygon, then the named town/site's
   footprint. Report the distance to the footprint, not just the distance to its centroid.
   Flag the wrong administrative area or a substantial distance outside the expected footprint.
   Allow a documented boundary tolerance and make distance thresholds configurable heuristics;
   a single worldwide radius would misclassify large rural districts and small urban sites.
3. Treat location certainty separately from computational success. An airport or town reference
   is not an exact inspection-room or farm location, even if its coordinates have many decimals.
   Record an explicit **map reference** separately from the event's **named place** when they
   differ. For example, an Ayritom event mapped to a Surxondaryo regional reference should be
   checked against that region and retain its unresolved exact-site note, rather than pass as
   a verified Ayritom pin. Multi-site reports need multiple references or an explicit broader
   reference, not a guessed single event point.
4. Add weaker triage signals: identical coordinates reused for unrelated localities, a locality
   pin matching a country/regional fallback, and a location described only as a broad area but
   labelled `exact`. These signals prioritize review and must not automatically move pins or
   reject publication. Repeated pins at a genuine shared port or airport are expected.

Suggested output categories are **administrative mismatch**, **far from named place**,
**ambiguous place**, **precision unsupported**, and **unverified reference**. Always show
the expected place, resolved feature, actual administrative area, distance and evidence so
a reviewer can assess the flag. A successful match verifies only the stated geographic level.
Use **not checked** when locality evidence is unavailable, never silently call it verified.

Start with cached references from the initial coordinate review and the most precise reports,
then expand coverage. Keep network geocoding out of page builds, cache and reuse results,
and respect the chosen provider's documented usage policy before adding any bulk lookup job.
As with the country audit, reviewed exceptions should expire when geographic fields or the
reference evidence change. Keep this review pass advisory and offline during normal builds.
