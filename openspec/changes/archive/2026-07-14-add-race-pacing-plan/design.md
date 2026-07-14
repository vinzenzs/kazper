# Design — add-race-pacing-plan

## Context

`race-fueling-plan` owns a persistent race (`races` + ordered `race_legs`) and a
deterministic compute-on-read per-leg *fuelling* plan (`GET
/races/{id}/fueling-plan`). Pacing — per-leg intensity targets — has no
primitive: the agent re-derives "hold ~70–75% FTP on a full-distance bike, run
~4:5x/km off the bike" every conversation. All three thresholds the math needs
already exist on the `athlete_config` singleton: `ftp_watts`,
`threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m` (CSS). The
in-flight `add-per-sport-tss` change consumes the same thresholds for stored
TSS derivation and adds **no new fields** — so this change has only a soft
coordination point with it (see Open Questions), not a dependency.

Unlike fuelling (pure compute, no stored artifact), pacing plans get negotiated
— "I know my FTP says 207 W but I'm holding 195 on this course" — so this
change also introduces persisted per-leg manual overrides with compute-on-read
fallback.

## Goals / Non-Goals

**Goals:**

- A deterministic per-leg pacing baseline (bike power band, run pace band, swim
  pace band per 100 m) banded by leg duration, computed from athlete-config
  thresholds at read time — same posture as the fuelling plan.
- Per-leg IF and estimated TSS, plus a race-level `estimated_tss_total` usable
  for taper planning alongside the PMC (`add-performance-management`).
- Persisted per-leg overrides that survive race/leg edits, with the computed
  value as fallback and an explicit `source` marker.
- Honesty about inputs: unset thresholds degrade the affected legs only, loudly
  and machine-readably.
- Fit the existing package / registry / spec conventions exactly.

**Non-Goals:**

- Course-specific modelling (gradient, wind, aero, segment splits). This is
  explicitly not Best Bike Split; the baseline is duration-generic.
- HR-based targets, environment (heat/altitude) adjustment, race-morning
  execution cues — agent-side, layered on the baseline.
- Storing the computed plan (only overrides persist), writing targets into
  planned workouts, or results capture.
- New athlete-config fields.

## Decisions

### D1. New capability `race-pacing-plan`, new package `internal/racepacing/`

Fuelling and pacing answer different questions in different units and evolve
independently — the same reasoning that keeps `hydration`, `workout-fuel`, and
`summary` shapes separate keeps pacing out of the `race-fueling-plan` spec. The
race/leg tables stay owned by `race-fueling-plan`; this capability only *reads*
them.

Code-wise the computation needs three data sources: the race + legs (owned by
`internal/races`), the athlete-config thresholds (`internal/athleteconfig`),
and its own overrides table. Folding it into `internal/races` would grow that
package a second concern plus an `athleteconfig` import; instead a new
`internal/racepacing/` package depends on `races.Repo` and
`athleteconfig.Repo` — the exact cross-package pattern
`internal/workoutfueling/` established (aggregator package depending on
multiple repos, injected in `httpserver.Run()`). It registers its routes on the
same `/races/:id/...` subtree; Gin allows this because registration happens on
the shared router group.

**Alternative considered:** extend the `race-fueling-plan` spec/package.
Rejected — violates the one-capability-per-concern precedent and would couple
two independently-tuned models.

### D2. Endpoint shapes mirror the fuelling sibling

- `GET /races/{id}/pacing-plan` — compute-on-read, no query params (thresholds
  come from the athlete-config singleton, not the caller — unlike fuelling's
  `body_weight_kg`, which is a per-request athlete param; thresholds are
  durable config).
- `PUT /races/{id}/pacing-plan/overrides/{ordinal}` — full-replace of one
  leg's override, keyed by the leg's ordinal (natural key, like
  `PUT /goals/overrides/{date}`). Rejects `Idempotency-Key` with
  `400 idempotency_unsupported_for_put` via the existing middleware rule.
- `DELETE /races/{id}/pacing-plan/overrides/{ordinal}` — removes the override;
  the leg reverts to computed on the next read.

Response shape of the GET (per leg, discipline-dependent fields omitted when
inapplicable):

```
{
  "race_id": …, "race_name": …, "race_date": …, "total_duration_min": …,
  "legs": [{
    "ordinal": 2, "discipline": "bike", "expected_duration_min": 300,
    "source": "computed" | "override" | "none",
    "target_power_low_w": 180, "target_power_high_w": 207,        // bike only
    "target_pace_low_sec_per_km": …, "target_pace_high_sec_per_km": …,   // run only
    "target_pace_low_sec_per_100m": …, "target_pace_high_sec_per_100m": …, // swim only
    "intensity_factor": 0.73, "estimated_tss": 266.5,
    "missing_thresholds": ["ftp_watts"],                          // only when uncomputable
    "rationale": "…"
  }],
  "estimated_tss_total": …, "tss_complete": true,
  "missing_thresholds": []                                        // race-level union
}
```

### D3. The deterministic math (the core)

Bands key on the **leg's** `expected_duration_min` (a bike leg is paced by how
long *it* lasts), unlike fuelling's carbs which band on total race duration
(gut absorption is a whole-race budget). Sources: classic long-course guidance
(Coggan/Allen *Training and Racing with a Power Meter* IF-by-duration tables;
Friel / TrainingPeaks race-plan IF recommendations: full-distance bike IF
~0.68–0.78, 70.3 ~0.75–0.83, Olympic ~0.83–0.90, sprint near threshold).

**Bike — power band as % of `ftp_watts`, by leg duration:**

```
duration < 45 min        → 90–100 % FTP   (sprint-distance effort)
45 ≤ duration < 90       → 83–90 %        (olympic)
90 ≤ duration < 180      → 75–83 %        (70.3 / middle distance)
duration ≥ 180           → 68–78 %        (full distance)

target_power_low_w  = round(ftp_watts × band_low)     (integer watts)
target_power_high_w = round(ftp_watts × band_high)
IF                  = band midpoint (e.g. 0.73 for the ≥180 band)
estimated_tss       = duration_hr × IF² × 100          (Coggan TSS)
```

**Run — pace band as a multiple of `threshold_pace_sec_per_km` (higher
multiplier = slower = more seconds), by leg duration:**

```
duration < 30 min        → ×1.00–1.04
30 ≤ duration < 60       → ×1.04–1.10
60 ≤ duration < 150      → ×1.10–1.18   (70.3 run off the bike)
duration ≥ 150           → ×1.18–1.28   (full-distance marathon)

target_pace_low_sec_per_km  = threshold_pace × mult_low    (fast end)
target_pace_high_sec_per_km = threshold_pace × mult_high   (slow end)
IF                          = 1 / mult_midpoint
estimated_tss               = duration_hr × IF² × 100      (rTSS-style)
```

When the race contains a bike leg with a lower ordinal, the run leg's
`rationale` notes that the band already accounts for running off the bike
(multisport context) — the multipliers are multisport-calibrated, not
open-run PBs.

**Swim — pace band per 100 m as a multiple of
`threshold_swim_pace_sec_per_100m` (CSS), by leg duration:**

```
duration < 20 min        → ×1.00–1.05   (sprint 750 m)
20 ≤ duration < 45       → ×1.03–1.08   (olympic 1500 m)
duration ≥ 45            → ×1.06–1.12   (long-course 1.9–3.8 km)

target_pace_low/high_sec_per_100m = css × mult_low/high
IF            = 1 / mult_midpoint
estimated_tss = duration_hr × IF³ × 100    (sTSS convention: swim cost ∝ v³)
```

**Transition legs** carry no pacing target and contribute `estimated_tss = 0`
(rest), with a rationale saying so — the fuelling plan's zero-intake analogue.
**`other` legs** have no threshold model: no targets, no TSS, rationale notes
it, `source: "none"`. **Legs without `expected_duration_min`** cannot band:
computed targets and TSS are omitted with a "duration unknown" rationale
(mirrors fuelling); an override's absolute targets still apply, but TSS stays
null (no duration to integrate over).

**Race totals:** `estimated_tss_total` = sum of per-leg `estimated_tss` over
legs that produced one; `tss_complete` is `true` iff every swim/bike/run leg
produced an estimate (transitions don't count against it). IF is computed at
full precision and rounded for display; TSS derives from the full-precision IF.

### D4. Missing thresholds → partial `200` response, not a 422

Decision: a leg whose threshold is unset returns null targets plus
`missing_thresholds: ["ftp_watts"]` on the leg (and in a race-level union);
the request still succeeds. Justification:

1. **Per-leg independence.** A triathlete with FTP set but no CSS should still
   get bike and run targets; failing the whole plan over one missing threshold
   starves the agent of everything it *could* have.
2. **Overrides must work threshold-free.** Overrides are absolute numbers; an
   athlete who never configured FTP can still pin 195 W on the bike leg. A
   422 gate would make the override surface unreachable.
3. **Precedent.** The fuelling plan flags a defaulted sweat rate in `rationale`
   rather than erroring; "honest about inputs, loud in the response" is the
   established posture. `http-error-shape` still governs the real error paths:
   `404 race_not_found`, override-validation `400`s — all structured JSON with
   an `error` code.

The `missing_thresholds` field is machine-readable precisely so the coach can
say "set your FTP in athlete-config" instead of guessing.

### D5. Override persistence keyed `(race_id, ordinal)` — not `race_leg_id`

`PATCH /races/{id}` replaces legs **wholesale** (delete + reinsert), so an FK
to `race_legs.id ON DELETE CASCADE` would silently drop every override on any
leg edit. Keying by `(race_id, ordinal)` with `ON DELETE CASCADE` to `races`
survives leg replacement as long as the ordinal persists — the stable identity
the athlete actually thinks in ("leg 2 = the bike").

```
race_leg_pacing_overrides
  race_id                     UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE
  ordinal                     INT  NOT NULL
  target_power_low_w          INT NULL      -- bike family
  target_power_high_w         INT NULL
  target_pace_low_sec_per_km  NUMERIC NULL  -- run family
  target_pace_high_sec_per_km NUMERIC NULL
  target_pace_low_sec_per_100m  NUMERIC NULL  -- swim family
  target_pace_high_sec_per_100m NUMERIC NULL
  note                        TEXT NULL
  created_at, updated_at      TIMESTAMPTZ
  PRIMARY KEY (race_id, ordinal)
  CHECK: exactly one unit family fully populated (both low and high), low ≤ high, values > 0
```

Write-time validation (service sentinels → 1:1 error codes): race exists
(`404 race_not_found`), a leg with that ordinal exists (`404 leg_not_found`),
the populated unit family matches the leg's discipline
(`400 override_discipline_mismatch` — transitions and `other` legs accept no
override), exactly one family populated (`400 override_target_required` /
`400 override_unit_conflict`), `low ≤ high` and positive finite values
(`400 override_band_invalid`).

Read-time merge: an override whose unit family matches the leg's current
discipline replaces the computed band (`source: "override"`, rationale notes
manual override; IF/TSS re-derived from the override midpoint when the
relevant threshold is set, else omitted). If a later leg edit changed the
discipline under an override, the override is **ignored** with a rationale
note (never silently applied cross-unit) — it stays stored until explicitly
deleted or overwritten. Overrides whose ordinal no longer matches any leg are
simply not surfaced.

**Alternative considered:** override columns on `race_legs` itself. Rejected —
the columns are pacing-capability state and would be destroyed by the
wholesale leg replace; a separate table keeps ownership clean
(`race-fueling-plan` owns `race_legs`, this capability owns its overrides).

### D6. Rounding and unit isolation via `numfmt`

Applied only at the response boundary (storage full precision): power targets
integer watts; pace targets and `estimated_tss`/`estimated_tss_total`
`numfmt.Round1`; `intensity_factor` `numfmt.Round2` — matching the workouts
capability's 2dp `intensity_factor` precedent. Power lives only in `_w`
fields, run pace only in `_sec_per_km`, swim pace only in `_sec_per_100m`; no
shared target struct, and tests guard the isolation with `NotContains`
assertions (e.g. a bike leg's JSON never contains `sec_per_km`).

### D7. MCP tools on the shared registry

New `internal/agenttools/registry_racepacing.go` registering:

- `plan_race_pacing` (TierRead) → `GET /races/{id}/pacing-plan`
- `set_race_leg_pacing_override` (TierWriteConfirm, mirroring
  `set_daily_goal_override` — a training-target write) →
  `PUT /races/{id}/pacing-plan/overrides/{ordinal}`; the dispatcher already
  skips the idempotency header on PUT centrally.
- `clear_race_leg_pacing_override` (TierWriteConfirm, destructive) →
  `DELETE /races/{id}/pacing-plan/overrides/{ordinal}`

Each builds exactly one `HTTPCall`. The MCP announced surface derives from the
registry, so no manual expected-tools bump — the integration test's
registry-equality assertion covers it; the schema golden must be regenerated
(`-tags=goldengen`).

## Risks / Trade-offs

- **[Risk] Generic duration bands read as course-specific advice.** They are
  not Best Bike Split — no gradient/wind/aero modelling. → Mitigation: every
  leg `rationale` states it is a duration-banded baseline to adjust for
  course, weather, and fitness; course modelling is an explicit non-goal, and
  overrides are the escape hatch for a negotiated course-specific number.
- **[Risk] Band literature spread.** Published IF guidance varies by author
  and athlete level; any single band is contestable. → Mitigation: bands are
  named constants in a pure `compute.go` with the sources cited in comments;
  the response carries the band provenance in `rationale`; overrides let the
  athlete disagree without a code change.
- **[Risk] Ordinal-keyed overrides can attach to a *different* leg after a
  reorder** (athlete swaps legs 2 and 3; the override stays on ordinal 2). →
  Mitigation: the discipline-family check ignores the override when the unit
  no longer matches; when it does match (bike→bike reorder), the plan marks
  `source: "override"` loudly so the mismatch is visible, and the PUT/DELETE
  surface makes correction one call. Accepted as the price of surviving
  wholesale leg replacement.
- **[Risk] Estimated TSS diverges from stored per-workout TSS once
  `add-per-sport-tss` lands** (different formula families could disagree). →
  Mitigation: both use the Coggan IF²×hours×100 family (swim IF³ per sTSS
  convention); the estimate is labelled `estimated_` and never written to any
  workout row. Reconcile constants at apply time if the sibling lands first.
- **[Risk] Two in-flight changes modify the same athlete-config
  requirement** (`add-per-sport-tss` widens the same consumption gate). →
  Mitigation: both edits only append consumers; whichever archives second
  merges the union into the main spec — flagged in the proposal and below.

## Migration Plan

1. One append-only pair (`task migrate:new NAME=add_race_leg_pacing_overrides`)
   creating `race_leg_pacing_overrides` per D5. Verify the next free number —
   head is `054_sync_run_summary_partial` at writing, but out-of-band work has
   taken slots before.
2. Purely additive: no existing table or endpoint changes; the `races` schema
   is untouched. Rollback = down migration drops the one table; nothing else
   references it.
3. Wire the new package in `httpserver/server.go` (needs `racesRepo`,
   `athleteConfigRepo`, and the pool for the overrides repo); register routes;
   add registry entries; regenerate MCP schema golden; `task swag`.

## Open Questions

- **Sibling coordination (`add-per-sport-tss`).** Its proposal exists and adds
  no config fields, so this change needs nothing from it. If it archives
  first, re-base the athlete-config MODIFIED block on the merged text at
  archive time; align the run/swim TSS-estimate constants with whatever exact
  rTSS/sTSS formulas it implements.
- **Should `race_legs.intensity` (free-text annotation) nudge the band?** An
  athlete writing `easy` on a leg arguably wants the band's low half. Left out
  of v1 — the annotation is unvalidated free text; overrides are the precise
  mechanism. Revisit with usage.
- **HR fallback for legs with no power/pace threshold** (e.g. `other`
  discipline with `threshold_hr` set). Deferred — HR pacing bands are a
  different model with drift caveats; would be its own change.
- **Race-level pacing summary for the coach dashboard** (like the PMC chart in
  `add-performance-management`). Deferred; the REST/MCP surface is enough for
  the coach agent today.
