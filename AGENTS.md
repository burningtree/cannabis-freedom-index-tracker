# Notes for agents

The Cannabis Enforcement Tracker: sourced reports of cannabis eradication, seizures, raids, arrests,
prosecutions, sentences, executions and deaths, built with Astro. Companion to the Cannabis Freedom
Index (country ids are the CFI's).

- **To add a report from a URL** the user gives, follow `.claude/skills/add-report/SKILL.md`.
- **To find and add reports**, follow `.claude/skills/find-reports/SKILL.md`. Start with
  `node scripts/coverage.mjs`.
- **How a report is written** (one event, sources, figures, pictures): `docs/WRITING.md`.
  Data format and JSON API: `README.md`.
- Every sentence cites a source that was opened (`[@n]`); every source must be about the same event.
- Photos: copy one into `src/assets/reports/` only with an open licence or a no-copyright government
  work; link all others as `media`, and keep local evidence copies with `node scripts/archive-media.mjs`.
- The site must stay fast with thousands of reports: no page may embed all reports; use the
  indexes in `src/lib/reports.js` and `/api/index.json`. Check with `node scripts/load-test.mjs`.
- Check with `npm run build` (runs `npm test` first) and read the HTML in `dist/`. Don't start dev
  servers unless needed.
- Others may work here at the same time: check `git status` and commit only your own files.
