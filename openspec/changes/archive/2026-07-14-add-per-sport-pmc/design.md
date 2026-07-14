## Context

`internal/pmc` computes the Coggan EWMA over per-local-day summed completed TSS, warmed from the earliest workout. The repo query already restricts to completed workouts; `workouts.sport` is the established vocabulary (multisport is its own value). Per-sport TSS honesty shipped in `add-per-sport-tss`.

## Goals / Non-Goals

**Goals:** the same PMC semantics over a sport-filtered TSS series; combined behavior byte-identical when the param is absent.

**Non-Goals:** splitting multisport TSS across segments (no per-segment TSS exists); multiple sports per response; per-sport ramp-threshold tuning (the 8/week constant applies per series).

## Decisions

### D1 — A repo-level predicate, not a service change
The repo's per-day SUM/earliest-date queries gain an optional sport filter; the pure EWMA service is untouched — warm-up and `seed_date` are computed within the filtered series (a rider who started running last year gets a run `seed_date` of last year, correctly).

### D2 — Sport vocabulary is the workouts enum, multisport verbatim
No aliasing, no segment attribution: `sport=multisport` means "load from brick sessions", matching `workout-stats.by_sport`. Splitting a brick's TSS across disciplines would require per-segment TSS — a different change. `sport_invalid` on unknown values.

### D3 — Ramp alerts and missing-TSS counters stay per-series
Within a filtered read they describe that sport's series — a run-only ramp alert is meaningful on its own. The response echoes `sport` so a stored/quoted payload is self-describing.

### D4 — Dashboard selector defaults to All
The PMC panel's existing view is the default; Bike/Run/Swim re-fetch with the param. The target-trajectory overlay (if `add-planned-vs-actual-ctl` ships) renders only on All — targets are combined-load declarations.

## Risks / Trade-offs

- **Sparse sports produce jumpy small-N series** — inherent; the missing-TSS counters and low absolute CTL make it readable.
- **Combined ≠ sum of filtered** (EWMA is nonlinear) — expected math, noted in the tool description to preempt agent confusion.

## Migration Plan

Additive param; no migration. Rollback = revert param handling.

## Open Questions

- Per-sport target trajectories once both changes coexist (deferred; composes naturally).
