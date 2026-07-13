# Design — per-sport computed TSS with provenance

## Context

`workouts.tss` is a nullable NUMERIC(10,2) validated only as `>= 0`; it is filled
by the Garmin importer for sessions where Garmin computes a training load from
power, and stays NULL for most runs and all swims. The service already has the
exact precedent this change extends: `deriveIntensityFactor` (see the workouts
spec requirement "Bike intensity_factor is derived from FTP when missing")
cross-reads the `athlete_config` singleton at write time, fails open when the
config is unwired/unset, never overrides a caller-supplied value, and never
retroactively recomputes. The athlete-config singleton **already stores every
threshold this change needs**: `ftp_watts`, `threshold_pace_sec_per_km` (run),
`threshold_swim_pace_sec_per_100m` (CSS), `lactate_threshold_hr`, and
`threshold_hr` — so no config schema change is required, only a consumption-gate
widening in its spec.

## Goals / Non-Goals

**Goals**

- Every completed workout with sufficient inputs carries an honest TSS, so a
  future CTL/ATL/TSB read has a complete series.
- Provenance (`tss_source`) so downstream analytics can weigh or filter computed
  values, and so recompute can distinguish "measured" from "derived" rows.
- A re-runnable backfill for existing rows and for threshold changes.

**Non-Goals**

- CTL/ATL/TSB analytics themselves (a follow-up read over the now-complete series).
- Grade-adjusted pace (GAP) for rTSS — we store `elevation_gain_m` but no
  gradient stream; plain pace is the honest v1 (open question below).
- Moving-time handling — only the `started_at`/`ended_at` window is stored;
  duration is elapsed time.
- Deriving TSS for `planned` workouts — planned TSS is a caller-supplied target.
- Automatic recompute when athlete-config changes (explicit recompute call
  instead, matching the IF-derivation "computed against the FTP in effect at
  write time" rule).

## Decisions

### D1. Precedence: explicit > power > pace > hr > none, recorded in `tss_source`

`explicit caller-supplied tss` (source `garmin` → `tss_source='garmin'`, any
other source → `'manual'`) `>` `power` (bike, from IF) `>` `pace` (rTSS runs,
sTSS swims) `>` `hr` (any sport with `avg_hr`) `>` none (`tss` and `tss_source`
both NULL).

Rationale: a measured/watch value always beats a derivation (same rule as
caller-supplied `intensity_factor`); power is the gold standard TSS definition;
pace-based is sport-specific and better than HR (no cardiac drift/decoupling
noise); hrTSS is the last resort. Each method has a gate; a failed gate falls
through to the next method rather than erroring.

Alternative considered: a single `computed` provenance value. Rejected — a
future CTL read (and the recompute endpoint itself) needs to know *which* rows
are recomputable and how trustworthy they are; five values cost nothing.

### D2. Formulas (TrainingPeaks-style)

Let `duration_hr = (ended_at - started_at) / 1h`.

- **Power TSS** (bike): `TSS = duration_hr × IF² × 100`, where `IF` is the
  workout's `intensity_factor` — caller-supplied or derived from
  `normalized_power_w / ftp_watts` in the same write (the existing derivation
  runs first). Gate: `sport='bike'`, effective `IF > 0`. This is algebraically
  the canonical `(t_s × NP × IF) / (FTP × 3600) × 100`.
- **rTSS** (run): `pace_sec_per_km = duration_s / (distance_m / 1000)`;
  `IF = threshold_pace_sec_per_km / pace_sec_per_km` (faster than threshold ⇒
  smaller seconds ⇒ IF > 1); `TSS = duration_hr × IF² × 100`. Gate:
  `sport='run'`, `distance_m > 0`, `threshold_pace_sec_per_km` set and `> 0`.
  Plain pace, not grade-adjusted (Non-Goal).
- **sTSS** (swim): `pace_sec_per_100m = duration_s / (distance_m / 100)`;
  `IF = threshold_swim_pace_sec_per_100m / pace_sec_per_100m`;
  `TSS = duration_hr × IF³ × 100` — **cubic**, per the TrainingPeaks swim
  convention (drag makes swim power scale with velocity³). Gate: `sport='swim'`,
  `distance_m > 0`, CSS set and `> 0`.
- **hrTSS** (any sport, incl. strength/yoga/other): `IF = avg_hr / LTHR` where
  `LTHR = lactate_threshold_hr` when set, else `threshold_hr`;
  `TSS = duration_hr × IF² × 100`. Gate: `avg_hr > 0`, an LTHR field set and
  `> 0`.

Alternative for hrTSS: the full TRIMP-exponential regression TrainingPeaks
actually fits. Rejected — it needs resting HR and a gender coefficient (neither
in athlete-config), and for a single athlete the simple HR-ratio form is
monotone in effort and honest enough for trend analytics; the provenance tag
makes its lower fidelity explicit.

### D3. Ingest-time computation, `completed` only, never retroactive

Derivation runs where `deriveIntensityFactor` already runs: `POST /workouts`,
each `POST /workouts/bulk` item, and the `external_id` upsert-update path — in
`buildWorkout`, after IF derivation (power TSS consumes the derived IF).
It runs **only when `status='completed'`**: a planned workout's window is a
target, and any `tss` on it is a caller-supplied plan target
(`tss_source='manual'`/`'garmin'`), which keeps adherence's `planned_tss`
semantics untouched.

Computed values snapshot the thresholds in effect at write time and are never
recomputed when the config later changes — identical to the IF rule. Read-time
computation was rejected: it would make `tss` unstable across config edits
(dishonest for CTL, which needs a fixed historical series), recompute on every
list read, and break the "stored row is the truth" pattern every other derived
field follows. `PATCH` never derives (same as IF): patching `tss` to a value
sets `tss_source='manual'`; patching it to `null` clears both; patching
`distance_m`/`avg_hr` does not re-derive — the recompute endpoint covers that.

### D4. Backfill: one-off recompute endpoint, not migration-time

`POST /workouts/recompute-tss` re-runs the D1/D2 derivation over completed rows
whose `tss IS NULL` **or** whose `tss_source IN ('power','pace','hr')`; rows
with `tss_source IN ('garmin','manual')` are never touched. Response reports
`{examined, updated, by_source}` counts. Single-user table, bounded size — no
pagination or window parameter.

Why not a migration-time backfill:

1. The formulas live in Go next to their unit tests; duplicating them in SQL
   inside an append-only embedded migration is a drift trap.
2. A migration runs once — if thresholds are unset at migrate time (fresh
   environment, or CSS simply never configured) it silently backfills nothing
   and can never be re-run. The endpoint is re-invocable after configuring
   thresholds or changing FTP/paces.
3. Repo migrations are schema plus *trivial* data fixes only. The migration
   still does the trivial part: `UPDATE workouts SET tss_source = CASE WHEN
   source='garmin' THEN 'garmin' ELSE 'manual' END WHERE tss IS NOT NULL` —
   pure SQL, no formulas — which also satisfies the pairing CHECK (D5).

The endpoint is a mutating POST, so the idempotency middleware applies as
usual, and the MCP tool auto-derives a key like every other write tool.

### D5. `tss_source` is server-managed with a pairing invariant

`tss_source TEXT NULL`, CHECK in `('garmin','manual','power','pace','hr')`, plus
CHECK `(tss IS NULL) = (tss_source IS NULL)` — a TSS without provenance (or vice
versa) is a bug, and the migration's back-fill makes the constraint satisfiable
on existing data. The field is response-only (`omitempty`): it is not accepted
as an input on POST/PATCH/bulk (ignored if sent, matching the lenient-binding
pattern — no new rejection path), because provenance is derived from *how* the
value arrived, not asserted by the caller.

### D6. Rounding and sanity guard

Computed TSS is stored at the column's NUMERIC(10,2) precision (via
`numfmt.Round2`, mirroring the stored-IF precedent) and rounded with
`numfmt.Round1` at the response boundary like every other nutrient/measurement
float — storage stays maximally precise for future CTL sums, responses stay 1dp.

Guard: when a computed `IF > 2.5` (stale threshold, corrupt distance, or a
mis-tagged sport) the derivation **skips** — `tss` stays NULL rather than
storing an absurd load number. Fail-open, no error, same philosophy as every
other gate. Explicit caller-supplied TSS is never guarded (the existing
`tss >= 0` validation is unchanged).

### D7. Unset thresholds degrade, never error

Missing `threshold_pace_sec_per_km` on a run falls through to hrTSS (if
`avg_hr` + LTHR present) and then to none; an unwired athlete-config repo (unit
tests) disables all threshold-based methods. No new error codes: a workout
write never fails because physiology is unconfigured — identical to the
"Missing FTP leaves IF NULL" behavior.

## Risks / Trade-offs

- [Risk] Elapsed-time duration overstates TSS for sessions with long pauses
  (e.g. a swim with wall rest, a run with breaks). → Mitigation: accepted for
  v1 (moving time is not stored); provenance marks the value as computed;
  documented in the spec text. Revisit if moving time is ever ingested.
- [Risk] hrTSS on strength/yoga sessions is physiologically soft. → Mitigation:
  it is the *last* precedence tier, tagged `hr`, and TrainingPeaks applies the
  same convention; downstream analytics can filter by `tss_source`.
- [Risk] Recompute changes historical TSS after a threshold edit, shifting any
  future CTL series. → Mitigation: recompute is explicit and manual (never
  automatic), and only ever touches computed rows — measured `garmin`/`manual`
  values are immutable to it.
- [Risk] The pairing CHECK could fail the migration if an unexpected data state
  exists. → Mitigation: the back-fill UPDATE runs in the same migration before
  the constraint is added; `tss IS NOT NULL ⇒ source` is total (source is NOT
  NULL), so the CASE covers every row.
- [Risk] The upsert-update path re-derives on every Garmin re-sync; a re-synced
  activity that now carries an explicit `tss` must win over a previously
  computed one. → Mitigation: derivation is part of `buildWorkout` full-replace
  semantics — the incoming body is re-evaluated from scratch each time, so the
  precedence re-applies cleanly; covered by an integration test.

## Migration Plan

1. New migration pair (`task migrate:new NAME=add_workout_tss_source`; head is
   currently `054` — **verify the highest number before committing**, per
   convention):
   - up: `ALTER TABLE workouts ADD COLUMN tss_source TEXT NULL` + enum CHECK;
     back-fill UPDATE for rows with `tss IS NOT NULL`; then add the pairing
     CHECK `(tss IS NULL) = (tss_source IS NULL)`.
   - down: drop both constraints and the column.
2. Deploy; existing NULL-TSS rows are untouched (honest: nothing was measured).
3. Operator (or the coach agent) calls `POST /workouts/recompute-tss` once the
   thresholds are confirmed in athlete-config — fills historical rTSS/sTSS/hrTSS.
4. No client coordination needed: `tss_source` is additive `omitempty`.

## Open Questions

- **GAP for rTSS**: `elevation_gain_m`/`elevation_loss_m` exist per workout — a
  crude grade adjustment (e.g. Minetti-based equivalent flat pace) is possible
  without streams. Deferred; plain pace ships first, provenance stays `pace`.
- **IF sanity cap value (2.5)**: a guess; revisit if legitimate short all-out
  efforts get skipped.
- **Should recompute also run automatically after `PUT /athlete-config`?**
  Deliberately not in this change (explicit beats implicit for a
  history-rewriting operation); reconsider when CTL analytics land.
