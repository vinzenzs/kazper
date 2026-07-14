## Context

Kazper is a single-user, fully auth-gated backend: `auth.Middleware` guards every `/api/v1/*` route (bearer identities mobile/agent/garmin), web basic-auth guards the SPA at `/`, and only infra endpoints (`/healthz`, `/readyz`, `/swagger`) sit unauthenticated at the root. There is no public data surface.

The athlete wants a public "road to race" broadcast page (single hero number = a countdown to the goal race). It will be a **separate frontend + Strapi**, not the embedded dashboard, and it should reflect data that lives in Kazper (the race). The design constraint that drives everything: expose the public page's data **without opening Kazper's private surface**.

Relevant existing model:
- `races` (`race-fueling-plan`): `{name, race_date, race_type?, location?, notes?}`, read via `GET /races/{id}`.
- `macrocycles` (`macrocycle`): `{name, start_date, end_date, race_id → races(id) NULL, …}`. `race_id` anchors the season's A-race. There is no explicit "active" flag.
- Unauth routes register on the root engine `r` before `api := r.Group(APIBasePath); api.Use(auth.Middleware)`.

## Goals / Non-Goals

**Goals:**
- One curated, non-PII endpoint that the external Strapi shield pulls, so friends never touch Kazper and the token never reaches a browser.
- Preserve Kazper's "the exposed contract is small, explicit, and code-reviewed" property — curation lives in Kazper code, not in Strapi field-selection.
- Introduce the public surface with the *least* auth machinery possible — no RBAC.

**Non-Goals:**
- The Strapi instance and the public frontend (separate repos).
- A live training-derived `progress` number (deferred v2).
- Any bearer identity, role/permission matrix, or change to existing routes/auth.
- A write path or any mutation.

## Decisions

### D1 — Strapi is a shield; Kazper stays fully private
The topology is `friends → frontend → Strapi → (secret) → Kazper`. Strapi holds one credential, pulls the curated slice on a schedule, caches, and re-serves publicly. Kazper gains exactly one unauthenticated route and nothing else changes.

- **Why not frontend → Kazper directly?** That puts public traffic on the personal backend and risks the credential reaching the browser. Strapi-as-cache shields both.
- **Why not publish Kazper → Strapi (push)?** A scheduled pull keeps Kazper a passive reader-of-record with no outbound coupling; the countdown is static (race_date) so freshness is trivial.

### D2 — Access is a single shared secret, NOT a bearer identity (no RBAC)
`GET /public/race-feed` requires header `X-Feed-Key` to equal `FEED_SECRET`, compared with `crypto/subtle.ConstantTimeCompare`. The route registers on the root engine, outside `auth.Middleware`, so it does not participate in the bearer-identity system at all.

- **Why not a 5th bearer identity (`public-feed`)?** `auth.Middleware` validates a token and then most routes don't re-check identity, so a new identity would by default read everything lacking a guard. Confining it to one route means *denying it on every other route* — a route×identity matrix that grows into **RBAC** and is brittle (forget one deny → the feed token reads HRV). A single-purpose secret checked by one handler is structurally scoped: it unlocks nothing because nothing else checks it.
- **Why not fully public / no auth (B1)?** The projection is genuinely non-PII, so B1 is defensible — but a secret keeps it from being trivially scraped/indexed and is rotatable. Chosen for v1; downgrading to public later is a one-line change.
- Disabled when `FEED_SECRET` is unset → `503 feed_disabled`, mirroring the Garmin opt-in gating (an empty secret must never mean "no check").

### D3 — The goal race is the active macrocycle's A-race
Resolve the race by: the macrocycle whose `[start_date, end_date]` contains today (tie-break: latest `start_date`), then its `race_id` → the race. `days_remaining = race_date − today` in the configured user timezone (`DEFAULT_USER_TZ`), floored at 0 on/after race day.

- **Why not "next upcoming race by date"?** The race calendar holds B/C/tune-up races; "next upcoming" could broadcast the wrong one. The A-race is the athlete's declared goal.
- **Why not a new "featured race" pointer?** `macrocycles.race_id` already *is* that pointer, maintained as part of season planning — zero new machinery.
- **Graceful emptiness:** when there is no active macrocycle, it has no `race_id`, or the race is missing, return `200 {race: null, days_remaining: null}` so the broadcast page degrades to a fallback rather than erroring.

### D4 — Minimal projection now
v1 returns `{race: {name, race_date}, days_remaining}`. A `progress` object (plan-adherence "N of M sessions", training volume) is deferred — it adds a live-data read + freshness reasoning and isn't needed for a compelling countdown/OG card. The shape leaves room to add `progress` without breaking consumers.

## Risks / Trade-offs

- **First unauthenticated route → new attack surface** → Mitigation: registered in isolation, returns only a hardcoded non-PII projection (no query passthrough, no ids echoed), constant-time secret, disabled-by-default. Even a leaked secret exposes only race name/date/countdown.
- **Secret in Strapi config** → Mitigation: server-side only, never shipped to the browser; rotatable via `FEED_SECRET` + Strapi env; distinct from all bearer tokens.
- **"Active macrocycle" is a derived notion, not a stored flag** → the containing-today rule can be empty (between seasons) or ambiguous (overlapping macrocycles); the tie-break + graceful-null handling keep it deterministic and non-erroring.
- **Countdown staleness via Strapi cache** → a countdown only changes daily; any reasonable pull cadence (even hourly) is exact enough; the day math runs against Kazper's clock at pull time.

## Migration Plan

- Additive only: one new route + one config var + one read-only package. No migration, no schema change, no change to existing routes.
- Rollout: set `FEED_SECRET` to enable; leave unset to keep the endpoint disabled (503). Rollback = unset the var / revert the route registration.

## Open Questions

- `progress` v2: which training-derived number reads best for non-cyclist friends — "N of M sessions" (plan-adherence) vs hours this block vs a streak? (Deferred; decide when building v2.)
- Should the feed ever expose more than one race (e.g. a season arc of A/B races) or strictly the single A-race? (v1: single A-race.)
