# Adherence vs compliance

Two different questions about your training plan:

> **Adherence** — did the planned sessions *happen*?
> **Compliance** — was a session executed *as written, step by step*?

## Adherence

Over a window (optionally scoped to one plan): counts of completed / missed / upcoming /
unplanned sessions, an **adherence rate** over sessions that were actually due (null when none
were — no fake 100 %), planned-vs-actual duration and TSS, per-sport breakdown, a compact
oldest-first **missed-sessions list** (capped, with an explicit truncation flag and a tunable
limit), and a **weekly trend** — aligned to plan weeks when a plan is given, calendar weeks
otherwise, with opt-in zero-filling for continuous charts.

Reconciliation runs both ways: a synced Garmin activity fulfills its matching planned session
(±1 day tolerance), and materializing a plan adopts an already-synced matching activity — so
adherence isn't fooled by ordering.

## Step compliance

For a completed workout linked to a structured template, Kazper expands the template (repeat
blocks flattened, "interval 3 of 5" provenance kept), matches watch laps to steps 1:1, resolves
zone-referenced targets to absolute numbers, and scores each step:

- **Target score:** 100 inside the prescribed band, falling linearly to 0 at 25 % outside it.
  The step's actual value is the right metric for its target kind (power, HR, pace, swim pace).
- **Duration score:** 100 within ±10 % of prescribed, linear to 0 at ±25 %.
- **Step score** = 0.7 · target + 0.3 · duration.
- **Overall** = planned-duration-weighted mean (a 10-minute step counts ten times a 1-minute
  one), with `steps_in_band` / `steps_scored` alongside.

Deviations are *signed*: "20 W under" is `delta: -20`, so the coach can distinguish
under-shooting from over-cooking.

**Honest refusals:** lap-count ≠ expanded-step-count returns `status: "unavailable"` with
`lap_count_mismatch` (a wrong alignment is worse than no score); unresolvable zones, missing
sensors, or RPE-only steps are marked unscorable with a reason rather than guessed; multisport
sessions and template-less workouts are explicit non-starts.
