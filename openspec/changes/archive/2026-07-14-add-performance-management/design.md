# Design — Performance Management Chart (CTL / ATL / TSB)

## Context

Every input the PMC needs already exists: `workouts.tss` (NUMERIC, nullable,
validated `>= 0`), `workouts.status` (`planned`/`completed`), `started_at`, and
the established local-day bucketing convention (`tz` param defaulting to
`DEFAULT_USER_TZ`, start-day attribution). `fitness-metrics` mirrors Garmin's
own acute/chronic load, but that is a *stored snapshot of Garmin's opinion* —
it cannot be recomputed, back-filled, windowed arbitrarily, or trusted to use
TSS at all (Garmin load is its own metric). Computing the classic Coggan PMC
from our own TSS gives the coach a series that is consistent with the rest of
the API and improves automatically as TSS coverage improves.

Precedent read: `energy-availability` (pure computation over existing repos, no
tables, missing-data honesty flags), `race-prep` (stateless math), and
`workout-stats` / `effort-analytics` (read-side aggregation with date-window
params, `YYYY-MM-DD` + `tz`, year-scale cap, completed-only).

## Goals / Non-Goals

**Goals**

- Daily CTL / ATL / TSB series over an arbitrary date window, computed
  deterministically from stored completed-workout TSS.
- Ramp-rate as a first-class output (per-day 7-day CTL delta) with explicit
  weekly alerts when CTL climbs faster than a safe threshold.
- Missing-TSS never masquerades as rest: it is counted and surfaced.
- MCP tool + dashboard chart in the same change (the repo's REST↔MCP 1:1 rule
  and the analyst dashboard idiom).

**Non-Goals**

- No estimation of TSS for workouts that lack it (that is `add-per-sport-tss`).
- No persistence of computed values, no caching layer, no new tables.
- No per-sport PMC split (single combined series in v1).
- No configurable time constants or ramp threshold via query params in v1.
- Not touching the `fitness-metrics` acute/chronic mirror — the two coexist,
  unit-isolated, never merged into one response shape.

## Decisions

### D1. Compute-on-read, no persistence

Same rationale as `energy-availability`: every input lives in one table, the
computation is a linear pass over daily sums, and persisting derived values
would create a sync problem (every workout POST/PATCH/DELETE/backfill would
have to invalidate a cascade of days). A single-user database makes the full
recompute trivially cheap: one indexed aggregate query + one in-memory loop of
one iteration per day of training history (~365 iterations/year).

### D2. EWMA formulas and constants (classic Coggan / TrainingPeaks form)

```
ctl(d) = ctl(d−1) + (tss(d) − ctl(d−1)) / 42        // fitness, τ = 42 days
atl(d) = atl(d−1) + (tss(d) − atl(d−1)) / 7         // fatigue, τ = 7 days
tsb(d) = ctl(d−1) − atl(d−1)                        // form going INTO day d
```

`tss(d)` is the sum of `tss` over **completed** workouts whose `started_at`
falls on local day `d` in the requested `tz`; a day with no workouts (or only
NULL-TSS workouts) contributes `0`. Constants `ctlTimeConstantDays = 42` and
`atlTimeConstantDays = 7` are package constants, not parameters — they are the
published defaults and the whole point of the PMC is comparability. TSB uses
*yesterday's* CTL/ATL (the standard "form on the morning of" convention) so a
huge session today does not flatter today's form.

### D3. Seed / warm-up: start from the beginning of history, seed at zero

The series is computed from `seedDate = (local date of earliest completed
workout) − 1 day`, with `ctl = atl = 0` on the seed date, forward to `to`; only
`[from, to]` is returned. Rationale over a fixed warm-up window (e.g. `from −
90d`): the full-history pass costs the same single SQL aggregate and a slightly
longer loop, and it makes the returned values *exact given stored data* —
identical no matter what window the client asks for. A fixed window would make
`ctl` at `from` depend on the request, which is exactly the kind of quiet lie
the repo avoids. Zero-seeding is honest for this dataset: the Garmin backfill
means the earliest stored workout approximates the true start of (recorded)
training. The response carries `seed_date` so consumers can see how much
warm-up history backs the window. If `from` predates the seed, days before the
first workout carry zeros. No workouts at all → an all-zero series, `200 OK`
(empty history is a state, not an error), with `seed_date` null/omitted.

### D4. Which TSS counts — completed only, missing counted not imputed

Only `status = 'completed'` rows contribute (planned workouts have no earned
load; same exclusion as `workout-stats` and EA burn sums). A completed workout
with `tss IS NULL` contributes `0` to its day but is **counted**: each day
carries `missing_tss_count` (omitted when 0, mirroring the EA
`missing_burn_workout_ids` honesty pattern at lower resolution — ids would
bloat a 365-entry series) and the response carries a window-level
`missing_tss_workouts` total. The coach can therefore see "CTL says 45 but 12
sessions in this window carried no TSS" instead of trusting a deflated number.
`add-per-sport-tss` (in-flight sibling) will shrink these counters by filling
TSS for non-power sports; PMC needs no code change to benefit — accuracy
improves the moment those rows carry TSS.

### D5. Endpoint shape

`GET /performance/pmc?from=YYYY-MM-DD&to=YYYY-MM-DD&tz=<iana>` — date params in
the `workout-stats` style (a PMC is a calendar-day series; RFC 3339 instants
would be false precision), `from`/`to` inclusive, `tz` defaulting to
`DEFAULT_USER_TZ` and echoed. Window cap **400 days** (`range_too_large`,
`max_days: 400`) so year-to-date + a margin is first-class, matching
`/workouts/summary`. Error codes reuse the established vocabulary:
`range_required`, `date_invalid`, `range_invalid`, `range_too_large`,
`tz_invalid`. Response:

```
{
  "from": "2026-01-01", "to": "2026-07-01", "tz": "Europe/Berlin",
  "seed_date": "2024-03-14",
  "days": [
    {"date": "2026-01-01", "tss_total": 85.0, "ctl": 62.3, "atl": 71.9,
     "tsb": -8.4, "ramp_rate": 3.1, "missing_tss_count": 1},
    ...
  ],
  "ramp_alerts": [
    {"week_start": "2026-02-16", "ctl_start": 51.0, "ctl_end": 60.4, "ctl_delta": 9.4}
  ],
  "missing_tss_workouts": 3
}
```

Read-only: never writes, never consumes an `Idempotency-Key` (EA precedent).

### D6. Ramp rate and the safety flag

Per-day `ramp_rate = ctl(d) − ctl(d−7)` — CTL change per week, the standard
ramp measure (days before the seed evaluate as `ctl = 0`). Additionally the
service scans Monday-start calendar weeks (endurance convention, matching the
adherence weekly trend) whose last day falls inside `[from, to]` and emits a
`ramp_alerts` entry for each week where `ctl(week end) − ctl(day before week
start) > 8.0` — the published safe-ramp ceiling (`rampAlertThreshold = 8.0`
CTL/week, a package constant). `ramp_alerts` is always present (empty array
when none) so the coach never has to distinguish "no alerts" from "field
missing".

### D7. Package shape — own read-only repo, not a `workouts.Repo` scan

New package `internal/pmc/` (capability `performance-management`; the short
package name follows `effortanalytics`' pattern of read-side packages with
domain-standard names). It follows the `effortanalytics` shape rather than the
`energy` shape: `energy` reuses `workouts.Repo.List` because its window is ≤ 92
days, but PMC must aggregate *all history* since the seed — pulling every
workout row into Go to sum one column would be waste. So `pmc` gets its own
read-only `repo.go` with two queries against `store.Querier`: earliest
completed-workout local date, and per-local-day `SUM(tss)` +
`COUNT(*) FILTER (WHERE tss IS NULL)` over completed workouts (grouped via
`(started_at AT TIME ZONE $tz)::date`, start-day attribution exactly like EA —
a midnight-spanning workout belongs to its start day). `service.go` holds the
pure EWMA/ramp math (unit-testable without a database) plus validation
sentinels; `handlers.go` registers `GET /performance/pmc` with swag
annotations; wiring lands in `internal/httpserver/server.go`.

### D8. Rounding at the response boundary

`tss_total`, `ctl`, `atl`, `tsb`, `ramp_rate`, and `ctl_delta` are computed at
full float precision through the whole recurrence (rounding inside the EWMA
would compound drift over years of days) and rounded via `numfmt.Round1` only
at serialization — the repo-wide rule.

### D9. MCP tool via the shared registry

One read tool `pmc_series` in a new `internal/agenttools/registry_pmc.go`
(args: `from`, `to`, optional `tz`; builds one `GET /performance/pmc` call,
`TierRead`, no idempotency key). Because the MCP surface is generated from the
registry, the announced list updates itself; the frozen announced-schema golden
(`internal/mcpserver/testdata/announced_schemas.json`) must be regenerated with
`-tags=goldengen` to admit the new tool.

### D10. Dashboard chart on the `/stats` surface

The home route carries an explicit "no new fetch" constraint, and the PMC is a
stats-surface artifact anyway (the power/pace curve already lives there). The
`/stats` route gains a PMC panel: CTL line, ATL line, TSB rendered as an
area/bars around a zero baseline (positive = fresh, negative = fatigued), over
a selectable window (90 d / 180 d / 365 d), with ramp-alert weeks visually
flagged on the CTL trace. Built with the existing visx + `Panel`/`Stat` analyst
idiom, a `usePMC` hook in `apps/web/src/api/hooks.ts`, graceful empty-state,
and a committed `apps/web/dist` rebuild via `task web:build`.

## Risks / Trade-offs

- **[Risk] Missing TSS on non-power sports (runs without rTSS, swims, strength)
  systematically deflates CTL/ATL.** → Mitigation: never imputed silently;
  per-day `missing_tss_count` + window `missing_tss_workouts` make the gap
  visible, and the in-flight `add-per-sport-tss` sibling fills the gaps at the
  source — PMC benefits with zero code change. No structural dependency.
- **[Risk] Full-history recompute per request grows with years of data.** →
  Mitigation: the SQL is one indexed aggregate returning one row per *active
  day*; the Go loop is one iteration per calendar day since the seed (~3.7 k
  iterations for a decade). Single-user; measured cost is negligible. Revisit
  with a cached seed checkpoint only if it ever shows up in latency.
- **[Risk] Zero-seed underestimates CTL if real training predates stored
  history.** → Mitigation: `seed_date` is surfaced so the consumer can judge
  warm-up coverage; with the Garmin backfill the stored history *is* the
  athlete's history. A fixed initial-CTL parameter is deliberately deferred
  (Open Questions).
- **[Risk] Divergence from Garmin's stored acute/chronic load confuses the
  coach.** → Mitigation: different metrics by design (Coggan TSS-based vs
  Garmin's proprietary load); they stay in separate capabilities/response
  shapes and the tool description names the distinction. Never merged.
- **[Trade-off] Day-level `missing_tss_count` instead of EA-style workout-id
  lists.** Ids on a 365-entry series would bloat the payload; the count is
  enough to warn, and the agent can drill in via `list_workouts`.

## Migration Plan

None. No schema change, no data backfill. Purely additive endpoint + tool +
chart; nothing existing changes shape. Rollback = remove the route, registry
entry, and panel.

## Open Questions

- Should the ramp threshold (8 CTL/week) ever become a query param or athlete
  config? Constant for v1; revisit if the coach wants sport- or phase-specific
  ceilings.
- Optional `initial_ctl`/`initial_atl` seed overrides for athletes with known
  pre-history fitness? Deferred until the zero-seed provably misleads.
- Per-sport PMC split (run CTL vs bike CTL) — genuinely useful for triathletes
  but doubles the response shape; natural follow-up once `add-per-sport-tss`
  lands and per-sport TSS coverage is dense.
