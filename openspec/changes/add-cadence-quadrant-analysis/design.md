## Context

`workout_streams` (migration 056) stores 1 Hz REAL[] series with `stream_type` CHECK-constrained to power/speed/heart_rate; the bridge defensively extracts `directPower`/`directSpeed`/`directHeartRate` from Garmin activity details. Quadrant analysis (Coggan) plots each pedaling sample as AEPF (average effective pedal force, N) vs CPV (circumferential pedal velocity, m/s):

- `CPV = cadence × crank_length × 2π / 60`
- `AEPF = power / CPV`

split into four quadrants at a reference force/velocity point, conventionally the athlete's threshold power at self-selected cadence.

## Goals / Non-Goals

**Goals:**
- Cadence as a first-class stored stream with exactly the existing streams' semantics (replace-on-repost, cascade, downsample, gap-zeros).
- A per-workout quadrant read that is a pure function of (streams, explicit params) — the W′bal posture.
- Bridge extraction that cannot break sync when Garmin's shape surprises.

**Non-Goals:**
- Run cadence consumers (stored fine if Garmin sends it; no spm analysis yet).
- Windowed/cross-workout quadrant aggregation; persisted quadrant results.
- Torque effectiveness / pedal smoothness (Garmin-computed pedal metrics, not derivable from these streams).
- Crank length in `athlete-config` (a per-request param with the 172.5 mm default until it proves annoying).

## Decisions

### D1 — Widen the existing CHECK, not a new table/shape
Cadence is another 1 Hz series; everything about `workout_streams` applies verbatim. The migration swaps the CHECK to `('power','speed','heart_rate','cadence')`. Ingest/retrieval treat it exactly like heart_rate: optional, gap-zeros stored, all-non-positive dropped, omitted when absent — and it feeds **no** best-effort, execution metric, or energy total (the unit-isolation line extends).

### D2 — Quadrant math on paired samples, zeros excluded
Per sample where both power > 0 and cadence > 0: compute CPV/AEPF; classify against the reference point (`aepf_ref` from `cp_watts` at `cadence_rpm`; `cpv_ref` from `cadence_rpm`); accumulate time per quadrant (I: high-force/high-velocity … IV: low-force/high-velocity). Samples with either value ≤ 0 (coasting, dropouts) are excluded and counted (`excluded_s`) — coasting is not pedaling and must not dilute shares.

### D3 — Explicit params, W′bal convention
`cp_watts` and `cadence_rpm` required (`cp_invalid` / `cadence_invalid`), `crank_mm` optional defaulting 172.5 (`crank_invalid` outside [100, 220]). Same reasoning as W′bal D1: no hidden fit, no config coupling, reproducible responses; the coach composes with `cp_model` and their known cadence.

### D4 — Response + `summary_only`
`{params: {...}, summary: {q1_pct, q2_pct, q3_pct, q4_pct, pedaling_s, excluded_s, aepf_ref_n, cpv_ref_mps}, scatter: [...]}` — shares `Round1`, refs `Round2`; scatter always downsampled to ≤ 1000 paired points (systematic sampling — a scatter needs shape, not fidelity), omitted under `summary_only=true`. Sentinels: `workout_not_found`/`streams_not_found`/`power_stream_missing`/`cadence_stream_missing`.

### D5 — MCP `quadrant_analysis` hardcodes `summary_only=true`
Shares and refs are reasoning data; the scatter is chart data (the established MCP line). Description points at `cp_model` for the CP value.

### D6 — Bridge: defensive extraction, post-with-existing
`_extract_streams` additionally pulls `directBikeCadence` (gaps → 0, all-non-positive dropped) and includes it in the same stream POST — no new bridge request, no new failure mode (missing/malformed descriptor → no cadence array, sync unaffected). Descriptor key confirmed on first real sync, the `directPower` precedent.

### D7 — Dashboard: detail-page scatter beside the W′bal strip
Rendered only when power+cadence streams and a fitted cp-model exist (params: model CP, pivot cadence constant 90 in the UI); quadrant shares as a compact legend. Absent otherwise — supplementary detail degrades to absence.

## Risks / Trade-offs

- **1 Hz recording vs Garmin "smart recording"** — smart-recorded rides have sparse cadence; gap-zeros then inflate `excluded_s`, which is the honest signal that the data is thin. Surfaced, not hidden.
- **Crank default wrong for the athlete** — a 165 mm rider's AEPF shifts ~4%; param exists, and the response echoes what was used.
- **Pre-change history has no cadence** — `cadence_stream_missing` names it; a stream re-post after a bridge-side backfill would fill it (operational, out of scope here).

## Migration Plan

Migration on the next free slot (verify head — sibling proposals also carry migrations): up widens the CHECK; down deletes `cadence` rows then narrows it back. Bridge deploy sequenced after the backend (old bridge + new backend is fine; new bridge + old backend gets its cadence array rejected by the CHECK — deploy backend first).

## Open Questions

- UI pivot cadence: constant 90 rpm vs the ride's own mean cadence? (v1: 90 — stable across rides; the endpoint takes whatever the caller prefers.)
- Should `excluded_s` distinguish coasting (power 0, cadence > 0 impossible; both 0) from dropout patterns? (v1: one bucket.)
