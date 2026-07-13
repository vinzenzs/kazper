# Design — Intensity Distribution (Time in Zone)

## Context

Every input already exists on the `workouts` row: `secs_in_zone_1..5` (INTEGER NULL,
Garmin-synced HR-zone time), `status` (`planned`/`completed`), `started_at`, `sport`, and the
optional `training_focus` annotation (7-zone German classification — REKOM/GA1/GA2/EB/WSA/SB/KA
— explicitly *intent*, never derived from the zone actuals, per the workouts spec). What's
missing is the aggregate: per-window and per-week zone shares, the chart a coach reads to
verify a polarized base phase actually was polarized.

Precedent read: `workout-stats` (`GET /workouts/summary` — the read-side aggregation shape
this extends: `from`/`to`/`tz` params, 400-day cap, completed-only, present-only sums,
`numfmt.Round1` at the boundary), the archived `extend-plan-adherence-detail` (`weekly`
trend: Monday-start calendar weeks, buckets only for weeks with content, no zero-fill), and
the in-flight `add-performance-management` (missing-data honesty counters:
`missing_tss_count` per bucket + a window total, count not ids).

## Goals / Non-Goals

**Goals**

- Windowed time-in-zone totals and percentage shares per zone (1–5), computed
  deterministically from stored completed-workout zone seconds.
- A per-week trend (Monday-start, tz-local) and a by-sport breakdown, so "polarized overall"
  can be decomposed into "polarized on the bike, threshold-heavy on the run".
- A simple, documented, auditable distribution label (`polarized` / `pyramidal` /
  `threshold` / `mixed`) — deterministic enough for the LLM coach to trust and cheap enough
  to recompute per request.
- Missing zone data never masquerades as easy training: excluded workouts are counted and
  surfaced.
- A secondary session-count distribution over the stored `training_focus` axis.
- MCP tool + dashboard panel in the same change (REST↔MCP 1:1 rule, analyst dashboard idiom).

**Non-Goals**

- No derivation or imputation of zone time (no estimating zones from avg HR, pace, or power).
- No persistence of computed values, no caching, no new tables, no migration.
- No power-zone distribution (only HR `secs_in_zone_*` is stored today).
- No per-sport weekly trend or per-sport classification in v1 (window-level label only).
- No configurable classification thresholds via query params in v1 (fixed package constants).
- `training_focus` stays an annotation — this change never writes or infers it.

## Decisions

### D1. Capability boundary — extend `workout-stats`, not a new capability

Intensity distribution is exactly what `workout-stats` is for: a read-side aggregation of
completed workouts over a date window with a by-sport breakdown, unit-isolated from
nutrition/hydration/energy. The code lands in the existing `internal/workoutstats/` package:
it reuses the same dependency (`workouts.Repo.List`, which already returns `SecsInZone1..5`,
`TrainingFocus`, and `Sport` on every row), the same param validation, and the same 400-day
cap — a ≤400-day window of a single user's workouts is a trivially small in-memory pass, so
no dedicated repo query is needed (the `pmc` full-history argument doesn't apply here).
A new capability would duplicate the window/tz/error vocabulary for no boundary gain; the
alternative (a sibling package) was rejected because there is no import cycle to break —
`workoutfueling`'s reason to exist — and no divergent dependency set. The MCP-tool
requirement also lives in the `workout-stats` spec, mirroring the existing
`training_totals` requirement there (not in `mcp-server`, which does not enumerate this
capability's tools).

### D2. Endpoint shape and validation — the `workouts/summary` vocabulary verbatim

`GET /workouts/intensity-distribution?from=YYYY-MM-DD&to=YYYY-MM-DD&tz=<iana>` —
`from`/`to` inclusive calendar dates, `tz` defaulting to `DEFAULT_USER_TZ` and echoed,
window capped at the same **400 days** (`maxRangeDays` is already a package constant), and
the established error codes: `range_required`, `date_invalid`, `range_invalid`,
`range_too_large` (with `max_days`), `tz_invalid`. Read-only: never writes, never consumes
an `Idempotency-Key`. Response:

```
{
  "from": "2026-05-04", "to": "2026-06-28", "tz": "Europe/Berlin",
  "total": {
    "workouts_counted": 31,
    "total_zone_secs": 148920,
    "zones": [
      {"zone": 1, "secs": 61200, "share_pct": 41.1},
      {"zone": 2, "secs": 55800, "share_pct": 37.5},
      {"zone": 3, "secs": 17400, "share_pct": 11.7},
      {"zone": 4, "secs": 10320, "share_pct": 6.9},
      {"zone": 5, "secs": 4200,  "share_pct": 2.8}
    ],
    "bands": {"low_pct": 78.6, "moderate_pct": 11.7, "high_pct": 9.7},
    "classification": "pyramidal"
  },
  "by_sport": {
    "run":  {"workouts_counted": 14, "total_zone_secs": ..., "zones": [...]},
    "bike": {"workouts_counted": 12, "total_zone_secs": ..., "zones": [...]}
  },
  "weekly": [
    {"week_start": "2026-05-04", "workouts_counted": 5, "total_zone_secs": ...,
     "zones": [...], "missing_zone_data_count": 1},
    ...
  ],
  "by_training_focus": {"basic_endurance_1": 16, "development": 4, "recovery": 3},
  "unclassified_focus_count": 8,
  "missing_zone_data_count": 3
}
```

`zones` is always five entries ordered zone 1→5 (a stable shape for stacked bars and for the
agent), with `secs` an integer sum and `share_pct = secs / total_zone_secs × 100`. Zone-time
seconds and shares appear only in this response shape — never merged into nutrition,
hydration, energy, or volume totals (unit isolation, the capability's standing rule).

### D3. Zone-share math and who counts

A completed workout **has zone data** when at least one of `secs_in_zone_1..5` is non-null;
a null individual zone contributes `0` seconds (present-only summing, the capability's
existing convention). Workouts with all five NULL are **excluded from every zone sum** and
counted in `missing_zone_data_count` instead — so `workouts_counted + missing_zone_data_count`
equals the completed-workout count in the window. Planned workouts are excluded entirely
(no earned time in zone). Shares are computed at full float precision and rounded via
`numfmt.Round1` only at serialization (the repo-wide boundary rule). When the window has no
zone time at all (`total_zone_secs == 0`), each zone entry carries `secs: 0` and **omits**
`share_pct` (a 0% share would be a lie, not a measurement), `classification` is null, and
the response is still `200 OK` — empty history is a state, not an error (PMC precedent).
`by_sport` groups by `string(w.Sport)` with a multisport session contributing a single
`multisport` entry (no per-leg zone attribution — legs don't carry zone splits), exactly
like the volume summary's `by_sport`.

### D4. Week bucketing and timezone handling

Workouts bucket by the **local date of `started_at`** in the requested `tz` (start-day
attribution — a midnight-spanning workout belongs to the day it started; same rule as the
volume summary and EA). The weekly trend groups those local days into **Monday-start
calendar weeks** — the adherence weekly-trend and PMC ramp-week convention — each bucket
carrying `week_start` (the Monday's local date), the same `zones`/`total_zone_secs`/
`workouts_counted` shape as the window total, and an omitempty `missing_zone_data_count`.
A bucket is emitted only for a week containing at least one completed workout (counted *or*
missing-data, so per-week honesty counters can't vanish); empty weeks are **not**
zero-filled, matching the adherence spec's explicit no-zero-fill rule. Weeks at the window
edges aggregate only the in-window days (the window is authoritative; partial edge weeks are
not extended beyond `[from, to]`).

### D5. Classification heuristic — three bands, four labels, fixed documented constants

The five zones collapse into the standard three-band model: **low = Z1+Z2** (below aerobic
threshold), **moderate = Z3** (between thresholds), **high = Z4+Z5** (above anaerobic
threshold). Band shares are computed from full-precision zone sums, then classified with two
package constants — `thresholdBandPct = 20.0` and `lowBaseBandPct = 75.0` (the 80/20 rule
with tolerance, so a 78% base doesn't flap to `mixed`):

```
if total_zone_secs == 0        → null            (nothing to classify)
else if moderate ≥ 20.0        → "threshold"     (the big-Z3 middle coaches flag)
else if low ≥ 75.0 and high > moderate → "polarized"
else if low ≥ 75.0             → "pyramidal"     (moderate ≥ high remainder)
else                           → "mixed"
```

This is a total, deterministic partition — every window gets exactly one label (or null).
The `bands` object is always returned next to the label so the coach (human or LLM) can
audit it rather than trust it; the label is advisory, the shares are the data. The label is
included in v1 (it is deterministic, cheap, and exactly what the coaching agent needs to
answer "am I training polarized?" in one call) but computed **only at window level** — weekly
and per-sport labels are deferred (single-week samples are too noisy to name).

### D6. Missing-zone-data honesty (the `missing_tss_count` pattern)

Counts, not ids: window-level `missing_zone_data_count` (always present, `0` when full
coverage) plus per-week omitempty counters — the PMC `missing_tss_count` shape at week
resolution. Ids would bloat the payload; the agent can drill in via `list_workouts`. This
matters here more than most places: strength sessions and pool swims routinely carry no HR
zones, so an uncounted exclusion would silently bias the distribution toward outdoor sports.

### D7. TrainingFocus axis — in for v1, as session counts

Included: `by_training_focus` (map of stored `training_focus` value → completed-session
count over the window) plus `unclassified_focus_count` (completed workouts with null
`training_focus`). It costs one loop over rows already fetched, and it is the coach-relevant
counterpart axis: time-in-zone measures what the body did, training-focus counts what the
plan intended — divergence between the two is itself a coaching signal. Session counts, not
duration-weighted time, because `training_focus` is a per-session annotation of intent, not
a per-second measurement (the workouts spec is explicit that it is never derived from zone
actuals); weighting intent by duration would fake a precision it doesn't have. No
classification is derived from this axis.

### D8. Package mechanics

`internal/workoutstats/` gains: response types in `types.go` (`Distribution`,
`ZoneAggregate`, `ZoneShare`, `Bands`, `WeekBucket` + the classification constants), a
`DistributionFor(ctx, Params)` service method sharing the existing `Params`/`startOfDay`
helpers and the same single `repo.List(from, upper, nil, &completed)` read, pure helper
functions for band collapse + classification (unit-testable without a database), and a new
handler with swag annotations registered inside the package's existing
`Register(rg *gin.RouterGroup)` — **no `internal/httpserver/server.go` change**, since the
package is already wired.

### D9. MCP tool via the shared registry

One read tool `intensity_distribution` added to the existing
`internal/agenttools/registry_workoutstats.go` (args: `from`, `to`, optional `tz`; builds
one `GET /workouts/intensity-distribution` call; `TierRead`; no idempotency key), with a
description naming the zone-share/classification semantics and pointing at
`training_totals` for volume questions. The MCP surface is registry-derived post
`unify-mcp-tool-registry`: the announced list updates itself, and the frozen
announced-schema golden (`internal/mcpserver/testdata/announced_schemas.json`) is
regenerated with `-tags=goldengen` — no hand-maintained expected-tools list is edited.

### D10. Dashboard panel on the `/stats` surface — in scope

The `/stats` route (the analyst training-log surface, already home to the volume heatmap and
power/pace curve) gains an intensity panel: a window-total horizontal zone-share bar with
the classification badge and the band shares, plus a per-week stacked zone-share bar chart
from `weekly`, over the route's existing period selection. Missing-data counts render as a
muted honesty note. Built with the existing visx + `Panel`/`Stat` idiom, a
`useIntensityDistribution` hook in `apps/web/src/api/hooks.ts`, graceful empty-state for
zone-less windows, and a committed `apps/web/dist` rebuild via `task web:build`. The home
route is untouched (its "no new fetch" constraint holds).

## Risks / Trade-offs

- **[Risk] Zone semantics differ across sports** (Garmin HR zones can be configured
  per-sport; Z4 on the bike and Z4 on the run aren't physiologically identical), so the
  combined window distribution blurs sports. → Mitigation: `by_sport` returns the same zone
  aggregate per sport so the coach can decompose; the classification stays window-level and
  advisory, with its input `bands` always visible.
- **[Risk] Missing zone coverage (strength, swims, HR-strap-less sessions) skews the
  distribution toward outdoor sports.** → Mitigation: never hidden — excluded workouts are
  counted per window and per week (`missing_zone_data_count`), the D6 honesty pattern, and
  the dashboard surfaces the count next to the chart.
- **[Risk] The classification thresholds (20 / 75) are heuristics, not physiology** — a
  lab-tested athlete's polarization index would disagree at the margins. → Mitigation:
  constants are fixed, named, and documented in the spec; the raw shares and bands are the
  primary output and the label is derived, auditable, and cheap to re-derive if the
  constants ever change.
- **[Risk] Zone time ≠ workout duration** (Garmin only accrues zone seconds while HR is
  measured), so shares are of *zoned* time, not elapsed time. → Mitigation: shares are
  explicitly defined over `total_zone_secs`, which is returned; duration lives in the
  existing volume summary and is deliberately not mixed in.
- **[Trade-off] Reusing `workouts.Repo.List` pulls full rows for up to 400 days instead of a
  SQL `SUM`.** Accepted: single-user scale, one indexed range scan, and it keeps the package
  dependency-identical to the existing summary; revisit with a dedicated aggregate query
  only if latency ever shows it.
- **[Trade-off] Weekly buckets are calendar weeks, not plan weeks.** Plan-week alignment
  (the adherence `plan_id` mode) is deferred — intensity distribution is meaningful without
  a plan, and plan-aware bucketing would drag in the training-plan dependency for a v2-sized
  gain.

## Migration Plan

None. No schema change, no data backfill, no new tables — the endpoint reads existing
`workouts` columns. Purely additive endpoint + tool + panel; nothing existing changes shape.
Rollback = remove the route, the registry entry, and the panel.

## Open Questions

- Should classification thresholds (20/75) or the band mapping become athlete config once
  real windows have been eyeballed? Constants for v1.
- Power-zone distribution as a parallel axis if per-workout power-zone seconds are ever
  stored (only HR zone time exists today).
- Plan-week-aligned bucketing (`plan_id` param, adherence-style) once someone actually asks
  "was my *build block* polarized" rather than "was May polarized".
- Per-sport or per-week classification labels, if window-level proves too coarse — noisy on
  small samples, so deliberately left out of v1.
