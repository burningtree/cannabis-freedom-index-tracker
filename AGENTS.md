# Notes for agents

The Cannabis Enforcement Tracker: sourced reports of cannabis eradication, seizures, raids, arrests,
prosecutions, sentences, executions and deaths, built with Astro. Companion to the Cannabis Freedom
Index (country ids are the CFI's).

- **To add a report from a URL** the user gives, follow `.claude/skills/add-report/SKILL.md`.
- **To find and add reports**, follow `.claude/skills/find-reports/SKILL.md`. Start with
  `node scripts/coverage.mjs`.
- **What belongs here and what goes to the CFI history** (changes in the law, famous and landmark
  cases, statistics, executions, events before 2000): `docs/CFI.md`.
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

## Submission service

`backend/` is the persistent Go intake, Nostr, OpenCode research and publishing service; Astro stays static.
Read `docs/NOSTR-SUBMISSIONS.md` before changing it. Run `cd backend && go test -race ./...`
and `go vet ./...` for service changes, plus the site's checks for UI changes.
The user has requested automatic processing and publication without human approval gates.
The publisher opens normal PRs and merges the exact validated head only after the required
report validation and other commit checks succeed; it never bypasses repository protection.
`published` requires merge, successful Pages deployment and the confirmed live report URL.
Unresolved source conflicts and invalid evidence finish as unable to verify rather than awaiting approval.
OpenCode is the actual research harness: a sandboxed local CLI for macOS development, or a
read-only container for deployment. Give it only research-provider credentials, never Nostr
keys, GitHub credentials or the operational database. Keep automated report status `reported`.
Only deliberate public assistant/tool activity belongs in the live AI transcript; never expose
raw sources, private reasoning, tool arguments/output, credentials or host filesystem paths.
