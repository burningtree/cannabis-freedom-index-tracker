# Automatic submissions, OpenCode and Nostr

The static Astro site stays on GitHub Pages. The Go service owns durable intake, SQLite/WAL,
OpenCode jobs, public activity, Nostr delivery and automatic report publication.

The user requested automatic processing without human approval gates. By default, `AUTO_PUBLISH=true`:
valid reports are researched, rendered, tested, proposed as normal PRs and merged automatically
when the exact validated commit passes the required report check and other commit checks.
Repository protections are respected. A failure or unresolved source contradiction finishes with
a reason, rather than waiting for a person to approve it. `AUTO_PUBLISH=false` retains an optional
legacy draft-PR/review mode.

## Local development

Run these in separate terminals from the repository root:

```sh
npm run backend-dev
npm run dev
```

`backend-dev` loads the ignored root `.env` and starts Go on `127.0.0.1:8080`. It defaults to
the locally installed OpenCode CLI, `opencode-go/deepseek-v4.1-flash`, automatic publication,
and private state under `backend/data/`. Astro proxies HTTP and WebSocket traffic through
`/__submissions`, so the frontend also works when port 4321 is occupied and Astro selects 4322.
Production builds require an explicit `PUBLIC_SUBMISSIONS_API_URL`.

Private local `.env` settings:

```dotenv
RESEARCH_RUNNER=local
RESEARCH_MODEL=opencode-go/deepseek-v4.1-flash
AUTO_PUBLISH=true
```

Keep `.env` mode 0600 and out of Git. Never prefix a secret with `PUBLIC_`.
The local runner uses OpenCode’s saved credentials. OpenCode 2 uses its native scoped
`auth export`/`auth import` commands to initialize a private standalone session; credential
contents never enter logs or `.env`. OpenCode 1 copies only the selected provider from its
legacy auth file (`OPENCODE_AUTH_FILE` can override that file). Local mode does not require
`RESEARCH_API_KEY`. Docker deployments can provide a dedicated provider key.
On every backend start, the service checks the binary/version and runs a one-step, no-tool
`opencode run` prompt asking the selected model to reply `OK`. Research starts only after that
test passes. The check is bounded to 60 seconds and appears live in the UI. A failed check leaves
intake/status endpoints running and jobs queued; authenticate the provider in OpenCode and restart.
You can run the same check without starting workers using `cd backend && go run ./cmd/tracker -check-ai`.
Restart `backend-dev` after changing environment settings or saved auth. `/v1/config` and the public request
register explicitly report the binary/model startup check and any unavailable research configuration.

The harness is **OpenCode**, not a replacement direct-LLM client. The local runner detects the
installed version and uses native OpenCode 2 configuration with `run --standalone`, or legacy
OpenCode 1 configuration with `run --pure`. Both select the researcher agent, configured model
and JSON stream in a disposable read-only evidence workspace. A functional startup prompt,
rather than an exact local version number, decides readiness. The optional Docker image pins
OpenCode 1.17.13 for reproducible deployment.
OpenCode's researcher agent and editorial skills drive source research and the final structured
artifact. The service independently checks source access, quotations, output schema and builds.

The local macOS runner uses `sandbox-exec`, a separate HOME/XDG/TMP directory, and filesystem rules
that deny host home directories, other temporary data and service data while allowing the job's
input and private session storage. It never inherits production/Nostr/GitHub environment variables.
Agent bash, edits, external directories and task delegation are denied. This local development
sandbox is a filesystem boundary, not a production network firewall. Use the Docker runner with
enforced egress controls for unattended deployment. Local mode on other platforms fails explicitly
rather than silently running without its sandbox.

Go 1.27.3 is pinned in `backend/go.mod`. Backend checks/build:

```sh
cd backend
go test -race ./...
go vet ./...
go build -o tracker ./cmd/tracker
```

## Public pages

- `/submit/`: accountless URL/public note intake, browser receipts, JSON export/import and removal.
- `/submission/?id=…`: public status, history, model/session information and live AI activity.
- `/submissions/`: all request origins and statuses in a 50-row table, filters, expandable details,
  live OpenCode chat/activity, and worker configuration status. Deliberately absent from the menu.

Both AI views offer **Show full log (JSON)** and **Download JSON**, containing every recorded
public activity event in chronological order and all run metadata, including retries. The JSON
view refreshes on live updates while open. `GET /v1/submissions/{id}/ai/log` returns the same
versioned export without the normal chat view's 100-message/10-run limits. Events omitted by
the public activity recorder (including raw tool output) are not reconstructed by this export.

`backend-dev` also enables `RAW_AI_LOGS=true`. In the JSON view, select **Raw output + emitted
reasoning** to inspect every captured OpenCode event (including reasoning, tool arguments,
results, final artifacts and errors) and stderr per run. The raw view refreshes every two seconds
while open, and its download includes the same data. `GET /v1/submissions/{id}/ai/raw` serves it.
Raw capture is written during execution; older completed runs have their original event archive
but may lack stderr. Reasoning is available only when emitted by OpenCode/the provider. Configured
credentials and credential fields are redacted. This diagnostic endpoint is off by default outside
`backend-dev`; enabling it makes raw logs accessible to anyone who can access that backend and
receipt, so keep the development backend on localhost.

The AI view shows the selected model/provider, OpenCode session ID, run status, reported token
usage and a bounded public transcript of request messages, assistant updates and tool activity.
It does not expose hidden reasoning, tool arguments/output, article bodies, raw final JSON,
credentials or private workspace paths. Full execution/evidence files stay private. OpenCode text
updates stream as the CLI produces them; the structured final artifact remains private and its
sanitized public explanation appears separately. Recent attempts remain visible after retries.

“Queued” means intake is committed, not that a model ran. Without working OpenCode authentication/model access, the worker keeps jobs durable and the
UI shows the startup test failure. Research can finish before
GitHub is configured: the validated publication job stays queued without consuming its retry
budget, and the UI identifies the missing publication configuration.

## Processing and publication

Progress is:

`queued → fetching → classifying → researching → drafted → publishing → merged → deploying → published`

Other outcomes: already covered (`duplicate`), outside scope (`not_relevant`), unable to verify
(`insufficient_evidence`), routed to the CFI (`routed_to_cfi`), and exhausted technical retries
(`failed`). Automatic mode converts legacy model `needs_review` results into unable to verify;
it does not bypass evidence checks or present uncertain claims as verified.

Exact normalized URLs share one case/job while each contributor keeps their own receipt.
The researcher can propose an existing-case update, preserving its stable slug, original prose,
source numbering and assets and moving the report to a later event date. Country changes,
earlier coverage that would replace a later step, ambiguous overlap and conflicting sources stop
automatic publication. Every factual claim must cite an opened source about the same event.
Pre-2000 incidents, executions, policy changes, precedents and other CFI-history material are
routed using `docs/CFI.md`. Automated Markdown remains `reported`, never automatically `verified`.

The isolated checkout runs `npm ci --ignore-scripts` and `npm run build`. The publisher accepts
only the validated report path and any old dated path removal. It persists a deterministic
`codex/submission-<case-id>` branch and commit SHA for idempotent retries. A separate durable
publication job prevents GitHub outages from buying another research run.

`.github/workflows/reports.yml` supplies the required **Validate report** check on report PRs.
Install the GitHub App only on this repository with Contents/Pull requests write and Checks,
Commit statuses and Deployments read permissions. Configure its App/installation IDs and private
RSA key using `backend/config.example.env`. The automatic publisher confirms the unchanged head,
normal PR state, the successful report check, all other reported checks and commit statuses, then
calls the ordinary merge endpoint with the exact validated SHA. It never bypasses branch
protection. Conflicts, failed checks and permission/rule failures produce explicit failures.
Ensure repository policy permits the App to merge reports without a human approval requirement.

The report becomes `published` only after PR merge, a successful `github-pages` deployment **at
the merge SHA**, and a reachable final report URL with the expected canonical link. Reconciliation
runs every ten seconds between jobs. A skipped merge SHA followed only by a newer descendant
currently remains `deploying`; the service conservatively does not infer deployment ancestry.
One long research job can delay reconciliation by its configured timeout.

GitHub configuration and the report-check workflow must be deployed before automatic publication
can complete. Code does not provision GitHub App secrets or change repository protections.

## API

| Method | Route | Behavior |
|---|---|---|
| POST | `/v1/submissions` | `{url,note?}` → durable `202 {id,status,status_url,nostr_event_id:null}` |
| GET | `/v1/submissions/{id}` | Public source, note, status, explanation, case/history/links |
| GET | `/v1/submissions/{id}/ai` | Processing configuration, 10 recent runs, latest 100 public activity messages |
| GET | `/v1/submissions/{id}/ai?after=n` | Next bounded activity batch after its numeric message ID |
| GET | `/v1/submissions?ids=a,b` | Up to 50 browser receipts, missing IDs omitted |
| GET | `/v1/submissions?cursor=n&status=…&origin=…` | All origins, 50 per page, total, next cursor, revision and processing status |
| WebSocket | `/v1/submissions/stream` | `{type:"submissions_changed",revision:"…"}` invalidations after public changes |
| GET | `/v1/activity?cursor=n` | Up to 25 terminal human-submission outcomes |
| GET | `/v1/config` | Public Nostr and AI processing configuration, never credentials |
| GET | `/healthz` | Database liveness |

IDs are 32-character random hex references; timestamps are Unix seconds. A receipt is public,
not a password or deletion credential. CORS accepts exact configured origins without cookies.
Public projections exclude private job provenance, evidence, model prompts and host paths.

Service-only endpoints:

- `POST /v1/ingest/candidate`: Bearer `INGEST_TOKEN`; JSON `{url,origin,provenance,idempotency_key}`.
- `GET /internal/metrics`: same token; bounded queue/running/dead counts.
- `POST /v1/webhooks/github`: verifies HMAC-SHA256 over the raw bounded body. Webhooks are hints,
  never proof of deployment; periodic authoritative reconciliation handles missed events.

## Live updates

Forward HTTP/1.1 WebSocket Upgrade/Connection headers for `/v1/submissions/stream`, with proxy idle
timeouts exceeding the 25-second heartbeat. HTTPS API URLs become WSS automatically. Connections
have exact origin checks, a 128-connection cap and 30-minute lifetime. Each connection checks one
SQLite version row per second. Transactional triggers cover intake, shared-case transitions,
Nostr links, model/run usage and public AI messages. A notification invalidates only the current
bounded table page and expanded details, not the entire corpus.

Clients reconnect with backoff, preserve the table on disconnection and pause while hidden.
The status page also subscribes so model messages appear without waiting for its fallback poll.

## Jobs, budgets and operations

SQLite uses WAL, foreign keys, busy timeout, atomic lease claims and fencing tokens. Research
heartbeats every 30 seconds on a two-minute lease. Technical failures back off and dead-letter
after eight attempts; configuration waits and daily budget waits do not consume attempts.
`RESEARCH_TIMEOUT_SECONDS` is 10–1800 (default 900). One research worker runs at a time.

`RESEARCH_RUN_CENTS` reserves a conservative amount for a run, and `RESEARCH_DAILY_CENTS` limits
starts per UTC day. These are run reservations, not a guarantee of metered model billing. Failed
runs count. OpenCode-reported token/cost telemetry is stored when available; enforce a provider
spending limit separately. Set filesystem quotas and retention for private evidence, session
storage, checkouts and build caches. The local runner has a wall-clock/output limit; Docker also
bounds CPU, memory, processes and scratch space.

The API caps bodies at 8 KiB, notes at 500 characters and outstanding jobs at 1,000. Source fetches
check HTTP(S) schemes, standard ports, redirects and DNS/IP destinations at connection time;
private/special addresses, oversized bodies and unsupported source formats are rejected. The
application's hardened fetcher does not make a local OpenCode tool a network sandbox: production
research needs an egress firewall/proxy denying private, host, loopback and metadata destinations.
Article content is untrusted data and may never grant tools or signing/publishing authority.

Short-lived rate limits cover IPs, authors and news hosts. Behind a proxy, configure only actual
`TRUSTED_PROXY_CIDRS` and correct forwarding headers. Use upstream connection limits and minimize
proxy access-log retention. No long-term IPs are stored by the service. Accountless does not mean
network anonymous. Public URLs/notes/outcomes may persist on third-party Nostr relays forever.
This is not a confidential tip line.

Repair commands, with workers stopped:

```sh
./tracker -retry RECEIPT_ID
./tracker -resolve RECEIPT_ID -outcome not_relevant -explanation 'This is general policy news rather than one incident.'
./tracker -backup /private/backups/tracker-2026-10-29.sqlite
```

Online backup uses `VACUUM INTO`, reserves a new mode-0600 destination and refuses overwrite.
Restore with workers stopped, then verify health, known receipts and recovered leases before
resuming. Tests cover backup/restore. Keep encrypted database and evidence backups off-host.
Operator resolution commands remain optional repair tools, not approval gates.

## Docker deployment

Set `RESEARCH_RUNNER=docker`, a digest-pinned `RESEARCH_IMAGE`, restricted `RESEARCH_NETWORK`,
model and provider key. Build `backend/research-image/` with OpenCode 1.17.13. The container gets
only the explicit read-only research snapshot and provider key, never `.git`, SQLite, Nostr or
GitHub credentials. Use a dedicated non-root host/service, persistent private disk, TLS proxy,
secret environment configuration, restart management and tested backups. The intake-only
`backend/Dockerfile` does not include Git, Node/npm or Docker orchestration tools.

A Docker network name alone is not a firewall. Provision enforced public-only research egress
before enabling unattended production research. No external OpenCode server is needed: the
service invokes the actual CLI harness per run and parses its event stream and final artifact.

## Nostr and discovery

Separate gateway and bot identities handle accountless web notes and native kind-1 submissions.
Generate protected keys with `./tracker -keygen /secure/path/nostr.env`, and configure at least
two distinct WSS relays. Publish the startup bot npub as `PUBLIC_NOSTR_BOT_NPUB`. Native notes
must p-tag the bot and contain one news URL; signatures/IDs/age/rate limits are verified and
bot/gateway loops ignored. NIP-10 root/reply threading, NIP-65 metadata/recipient read relays
and NIP-19 npub links are supported. Signed events, targets and ACKs persist before delivery;
retries reuse the same event ID. Relay ACKs do not guarantee client visibility.

Back up keys encrypted off-host. On compromise, stop delivery, rotate/announce the replacement
through trusted channels and inspect pending signed events before resuming. Old relay events
cannot be recalled globally. Metadata/cursors backfill a five-minute overlap with a seven-day
maximum intake age; longer outages need an explicit recovery run.

Optional RSS/Atom feeds and GDELT queries feed the same durable queue without posting every
candidate to Nostr. Provider/article keys deduplicate discovery. GDELT advances bounded one-hour
windows, pauses saturated 250-record partitions instead of silently skipping, and keeps cursors.
Public activity excludes machine discovery; the all-submissions register includes every origin.
