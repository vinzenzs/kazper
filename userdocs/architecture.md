# System architecture

## The shape

One binary, three subcommands: `serve` (the REST API), `mcp` (the coach's tool server, a thin
HTTP client of the API), and `migrate`. Postgres is the only store. Every capability — meals,
workouts, hydration, streams, goals, races… — is its own package with the same internal shape
(types → repo → service → handlers), which is why the API feels uniform.

```
Garmin ──▶ garmin-bridge ──┐
                           │  bearer: garmin
Companion app ─────────────┤  bearer: mobile
Coach (MCP / chat) ────────┤  bearer: agent          ┌──────────────┐
Dashboard (browser) ───────┤  basic:  web       ────▶│  REST API    │──▶ Postgres
                           │                         │  /api/v1/*   │
Friends ─▶ public page ◀── CI (nightly build) ◀──────│ one secret-  │
                                                     │ gated feed   │
                                                     └──────────────┘
```

## Identities

Four clients authenticate with distinct credentials: **mobile**, **agent** (the coach),
**garmin** (the bridge), and **web** (the dashboard, HTTP Basic). A handful of routes are
identity-restricted (Garmin sync-status writes are garmin-only; push registration is
mobile-only). The public race feed sits outside authentication entirely, gated by its own
single-purpose secret and serving only `{race name, date, days remaining}`.

## The MCP surface

Every coach tool issues **exactly one HTTP call** and returns the response verbatim — the coach
sees the same data you do, no private computation. Read tools map to GETs; write tools carry
automatic idempotency keys; in chat, writes pause for your confirmation before dispatch.

## The Garmin pipeline

The bridge logs in once interactively (MFA), then syncs headlessly every day over a rolling
window: activities (with splits, sets, zone times), recovery signals, weight, fitness metrics,
gear, PRs, devices, health vitals — and per-activity **1 Hz streams** (power/speed/heart-rate),
which feed most of the power analytics in this guide. Structured workouts flow the other way:
plans built in Kazper compile to Garmin workout files and land on your watch calendar.

## Where numbers are computed

Almost everything derived is **compute-on-read**: the PMC, critical power, W′ balance,
durability, intensity distribution, compliance, energy availability are calculated from stored
rows at request time, never cached in tables. The exceptions are deliberate: per-workout best
efforts and execution metrics are derived once at stream ingest (and reproducible via a
recompute endpoint), and per-workout TSS is derived at write time so the PMC has a stable series.
