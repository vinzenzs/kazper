## Context

`public-race-feed` shipped the data seam: `GET /public/race-feed` on the root engine, gated by `X-Feed-Key == FEED_SECRET` (constant-time), returning `{race: {name, race_date}, days_remaining}` or `{race: null, days_remaining: null}` between seasons. Its design D1 assumed the consumer would be a Strapi shield pulling on a schedule and re-serving publicly. This change builds the actual public page and **supersedes D1's topology**: the shield becomes a CI build, not a running CMS.

Repo precedent: `apps/` already hosts `web` (embedded React SPA), `companion` (Flutter), and `garmin-bridge` — separate surfaces with their own toolchains. The backend's deployment pipeline (image/Helm/Actions) is spec'd in `deployment-pipeline` and is untouched here.

## Goals / Non-Goals

**Goals:**
- A public "road to race" page friends can visit and share: hero countdown, race name/date, share card.
- Zero runtime infrastructure and zero public traffic on the personal cluster — pure static files on Pages/CDN.
- The feed secret never ships to a browser and exists only in the Actions secret store (and Kazper's own config).
- The page never shows a wrong countdown, even when a scheduled rebuild fails.

**Non-Goals:**
- Editorial content / CMS (the Strapi rationale — dropped for v1).
- The v2 training-derived `progress` number (feed-side and page-side both deferred).
- Custom domain, analytics, or any tracking (none — the page stays inert and private-by-construction).
- Any backend change: the feed endpoint, its secret gating, and its projection are consumed as-is.

## Decisions

### D1 — Static site replaces Strapi as the shield (supersedes public-race-feed D1)
Topology becomes: `GitHub Actions (cron) → X-Feed-Key → Kazper → SSG build → GitHub Pages → friends`. At serve time there is no server to patch, no secret to leak, and no path from public traffic to Kazper. Kazper is touched once per rebuild by CI.

- **Why not Strapi (the original D1)?** A full CMS + database + admin surface, maintained and patched forever, to cache one JSON object. Its only unique value — authoring editorial content — is a non-goal.
- **Why not an edge worker with a short-TTL cache?** Live-ish freshness is unnecessary: the only volatile value (days remaining) is handled client-side (D3). An edge function reintroduces a runtime secret and a serve-time dependency for no gain.

### D2 — Lives in this monorepo at `apps/public`
Follows the `apps/web` / `apps/garmin-bridge` precedent: the feed contract and its only consumer stay in one repo and one spec system. A separate repo would decouple deploys, but the site's CI is already independent (own workflow, path-filtered), which achieves the same isolation without splitting history.

### D3 — Countdown computed client-side from `race_date`; prerendered value is the no-JS fallback
The page embeds the feed's `race_date` and a few lines of vanilla JS compute whole-days-remaining in the **viewer's local timezone**, floored at 0; the build-time `days_remaining` is prerendered into the HTML as the fallback. Consequence: a stale build can only be wrong about *which race* (changes every few months), never *how many days* (changes nightly). The nightly cron becomes a safety net, not a load-bearing dependency.

- **Why viewer-local rather than the athlete's `DEFAULT_USER_TZ`?** A broadcast countdown reads as calendar days to the person looking at it; per-viewer local is the least surprising and avoids shipping tz data. Divergence from the feed's server-computed number is at most ±1 day near midnight boundaries — acceptable for this page (the OG card, which can't run JS, uses the build-time number and is refreshed nightly).

### D4 — Astro, static output
Astro is static-first, ships zero JS by default (the countdown script is a deliberate, tiny island), and has an established build-time OG-image path. The rejected minimal alternative — a bare Node script templating one HTML file — was competitive for v1 but loses on the OG card, the off-season state as a real component, and any future `progress`/editorial growth. No React needed; `.astro` components suffice.

### D5 — Build-time fetch; hard secret hygiene
The feed is fetched once at build via `FEED_URL` + `FEED_SECRET` env (Actions secrets/vars). Shipped assets must contain neither the secret nor any runtime reference to the Kazper origin. A `{race: null}` response builds the off-season page (D6); a **failed or non-200 fetch fails the build** — and a failed build leaves the previous Pages deploy serving, so the page degrades to *stale* (harmless per D3), never to *broken*.

### D6 — The off-season state is a designed face, not an error
`{race: null, days_remaining: null}` renders a deliberate "between seasons" page (same layout, no countdown, neutral OG card). The feed guaranteed graceful nulls precisely so consumers degrade; the page honors that.

### D7 — Pipeline: nightly cron + dispatch + path-filtered push, deploying to GitHub Pages
One workflow (`public-site.yml`): `schedule` (one nightly run, timed to land shortly after midnight in the athlete's tz so the prerendered fallback and OG card roll over), `workflow_dispatch` (rebuild on demand — e.g. after re-anchoring a macrocycle), and `push` filtered to `apps/public/**`. GitHub Pages hosting: zero config in-repo, free, and a custom domain can be layered on later without touching the build.

## Risks / Trade-offs

- **Cron needs a reachable production feed** → the Actions runner must reach the Kazper ingress. Mitigation: the endpoint is already public-internet-facing by design (that's its purpose); a failed run leaves the last deploy up and is visible in Actions.
- **OG card staleness** → the share-card image carries the build-time countdown; a missed cron shows yesterday's number in link previews (the page itself stays correct per D3). Accepted: previews are ephemeral and the cron is monitored by Actions' failure emails.
- **`race_date` is public by construction** → already accepted in `public-race-feed` (curated non-PII projection); the page adds no fields beyond the feed's.
- **GitHub Pages ties hosting to the repo host** → accepted; the build output is portable static files, so migrating hosts is a workflow-only change.

## Migration Plan

Purely additive: new `apps/public/` + one workflow + Pages enablement. No backend deploy required (the feed already ships). Enable by adding `FEED_SECRET`/`FEED_URL` to Actions and turning on Pages; roll back by disabling the workflow/Pages. The archived `public-race-feed` design is not edited (archives are history) — this design supersedes its D1 topology going forward.

## Open Questions

- Custom domain (and whether the OG card URL should assume it) — deferable until after first deploy.
- v2 `progress`: which training-derived number reads best to non-athletes ("N of M sessions this block" vs hours vs streak) — unchanged from the feed's deferral; D3/D5 leave room (nightly rebuild is fresh enough for any of them).
