## Context

Speed = cadence × step length. Kazper persists 1 Hz `speed` for every synced run, but run
cadence never reaches `workout_streams`: the cadence-quadrant change wired the `cadence`
stream type end to end and deliberately extracted only `directBikeCadence`, deferring
"run cadence (spm) uses". The per-workout compute-on-read analyses (quadrant, W′bal,
intervals) all live in `internal/activitystreams/` as pure functions over stored streams,
with rounding at the handler boundary, nothing persisted, downsampled scatters, and
`summary_only` for the MCP tool. This change follows that pattern exactly.

## Goals / Non-Goals

**Goals:**
- Run cadence (spm) flows bridge → ingest → storage with **zero schema change** (the
  `cadence` stream type and its REAL[] storage already exist).
- A per-workout `GET /workouts/{id}/stride` read decomposing speed into cadence vs step
  length across the run's observed speed range, with an honest contribution split and
  visible plateaus — numbers the coach can take apart, never a bare "you are
  stride-limited" label.
- A read-tier `stride_analysis` agent tool (summary only) and a run workout-detail
  dashboard view.

**Non-Goals:**
- Cross-workout stride trends over time (follow-up once this read exists).
- Grade-adjusted step length (no elevation stream is stored).
- Vertical oscillation / ground-contact time (different Garmin detail columns).
- Any persistence of derived values; any nutrition/energy coupling (streams stay
  unit-isolated).

## Decisions

**1. Home: `internal/activitystreams/`, not `effortanalytics`.**
This is a per-workout stream-derived read, the quadrant/W′bal/intervals family.
`effortanalytics` aggregates best-effort records across workouts; cadence deliberately
feeds no best-effort record, so there is nothing for it there.

**2. Bridge extraction: `directDoubleCadence` as fallback, one stream type, sport-native units.**
`_extract_streams` keeps preferring `directBikeCadence` (rides, rpm) and falls back to
`directDoubleCadence` (runs — Garmin's both-feet series, already in steps/min, the familiar
~170 number matching `averageRunningCadenceInStepsPerMinute`). No halving, no doubling, no
new stream type, no migration: the `cadence` row simply carries the sport-native unit, and
the spec wording changes from "(rpm)" to "sport-native (rpm rides / spm runs)".
*Alternative considered:* a separate `run_cadence` stream type — rejected; it needs a CHECK
migration and buys nothing, since sport disambiguates the unit and the two never mix in one
workout.

**3. `step_length_m`, not "stride length".**
Per-sample step length = `speed / (spm / 60)` — meters per single ground contact, Garmin's
"step length" (~1.0–1.3 m), which is what the colloquial German "Schrittlänge" means. Naming
it `step_length_m` avoids the biomechanics ambiguity where "stride" = two steps. The
response field names carry the unit, per house style.

**4. Analysis shape: speed bins + log-space contribution split.**
Samples with positive speed and cadence are bucketed into fixed-width speed bins
(0.25 m/s) over the observed range; each bin reports seconds, mean cadence (spm), and mean
step length (m). Because `ln(speed) = ln(cadence) + ln(step)`, the time-weighted
least-squares slopes of `ln(cadence)` and `ln(step)` against `ln(speed)` over the bin means
sum to 1 and split the speed gain into a `cadence_contribution_pct` /
`step_contribution_pct` pair. The per-bin table stays in the response so a plateau (step
length flat-lining above threshold pace, say) is visible rather than asserted.
*Alternative considered:* fitting per-sample instead of per-bin — rejected; thousands of
easy-pace samples would swamp the sparse fast tail, and bins weighted by seconds keep the
fit honest while making the response chartable.

**5. Honesty guard: `contribution: null` on insufficient range.**
When the observed speed spread across qualifying bins is under 0.5 m/s (a steady-state run
tells you nothing about the limiter), the bins still return but the contribution split is
`null` with `reason: "insufficient_speed_range"` — the raceprep/heat convention of refusing
to over-claim rather than emitting a confident nonsense number.

**6. Filters and params.**
Non-positive speed or cadence samples are excluded and counted (`excluded_s`, the quadrant
convention — standing, dropouts). Optional `min_speed_mps` (bounds [0.5, 5.0],
`400 min_speed_invalid`) lets the caller trim walk breaks; default off, because a fixed
walking cutoff would misread slow trail runs.

**7. Run gating: `409 sport_unsupported`.**
The math only means something for runs (a ride's cadence×"step" is nonsense), so a non-run
workout returns `409` with `{"error":"sport_unsupported"}` — the `workoutcompliance`
`multisport_unsupported` precedent (409 = resource state doesn't fit the analysis). Missing
data keeps the quadrant 404 sentinels: `workout_not_found`, `streams_not_found`,
`speed_stream_missing`, `cadence_stream_missing`.

**8. Response boundary conventions.**
Full precision internally; rounding only at the handler (step length 2 dp, cadence and
percentages 1 dp). Scatter (speed, cadence, step length triplets) downsampled to ≤ 1000
points via the existing systematic thinning; `summary_only=true` omits it. Nothing is
persisted. The `stride_analysis` MCP tool (read tier) always applies `summary_only=true` —
bins and split are reasoning data, the scatter stays chart data.

## Risks / Trade-offs

- **[Historical runs have no cadence]** → the `404 cadence_stream_missing` sentinel covers
  them honestly; a Garmin backfill re-post after the bridge deploy hydrates history.
- **[Old devices report single-foot run cadence (~85 spm)]** → step length would read ~2×.
  Mean bin cadence is in the response, so an ~87 spm run is visibly suspect rather than
  silently wrong; no auto-doubling heuristic (it would corrupt genuinely low-cadence data).
- **[Intervals/fartlek runs dominate the useful signal]** → steady runs return
  `insufficient_speed_range` by design; the tool description steers the coach toward runs
  with pace variety.
- **[Treadmill runs carry belt-speed artifacts]** → out of scope to detect; the workout's
  `environment` field is available to the coach for context.
- **[Bridge deploy ordering]** → backend ships first (endpoint 404s harmlessly on missing
  cadence); bridge deploy then starts the flow. No coordination hazard.

## Migration Plan

No migration. Backend and bridge deploy independently (backend first is harmless — see
risks). Golden/API docs regen via `task swag`; MCP expected-tools list bumped with the new
tool.

## Open Questions

_None._
