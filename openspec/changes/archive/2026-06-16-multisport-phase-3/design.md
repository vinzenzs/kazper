## Context

Two read-time gaps remain from `multisport-phase-2`:

1. **No template duration.** `multisport.Template` (`internal/multisport/types.go`) is `{id, name, segments[], timestamps}` — no `estimated_duration_sec`, unlike single-sport `workouttemplates.Template`. The materializer already computes a multisport session length on the fly (`materializeDurationSec` → `multisportSessionDurationSec`/`multisportFallbackDurationSec`), proving the math exists and is well-defined; it's just never surfaced on the template itself. A multisport sport segment's length lives in its **steps** (transition segments carry an explicit `Duration`); `workouttemplates.SumTimedDurationSec(steps)` already sums time-bounded steps.

2. **Opaque `by_sport`.** `coachcontext.summarize` (`internal/coachcontext/service.go`) does `s.BySport[string(w.Sport)]++`. A `multisport` workout — which only ever exists as a *planned* row materialized from a multisport slot (completed bricks stay `session_group`-linked single-sport rows) — increments a single `multisport` key, so the per-discipline picture the coach grounds on loses the legs. `coachcontext` has `workoutsRepo`, fitness/recovery/phases/athlete-config/bodyweight repos, but not the multisport repo.

## Goals / Non-Goals

**Goals:**
- Surface a multisport template's total time the same way single-sport templates expose it.
- Make `by_sport` credit each leg of a brick.
- Read-time only, additive, no migration, no new tools.

**Non-Goals:**
- Changing the materialized window math (Phase 2 already derives it; this only *exposes* the same number on the template).
- Decomposing `count`/`total_duration_min`/`total_kcal` — a brick is still one session with one window and one burn.
- Touching completed-brick ingestion (`session_group` stays) or the push/bridge path.
- A stored `estimated_duration_sec` column on `multisport_templates` (derived-on-read keeps it honest as segments change).

## Decisions

### D1: Derive `estimated_duration_sec` at the response boundary
Add an `EstimatedDurationSec *int` (omitempty) to the multisport template response, populated by a pure helper `estimatedDurationSec(segments) (*int)`: for each segment, a transition contributes its `Duration.Seconds` (when `kind=time`), a sport segment contributes `workouttemplates.SumTimedDurationSec(steps)` **iff every** step is time-bounded. If any segment is non-time-bounded (a distance/open/lap-button step, or a transition that isn't `time`), return nil (omit). Computed in the service/handler read path, not stored — mirrors the `numfmt.Round1`-at-the-boundary convention. Chosen over a DB column (would go stale when a segment is edited and duplicate the materializer's logic in two places).

### D2: `by_sport` decomposition resolves segments from the template, with graceful fallback
`coachcontext` gets a cross-injected `multisportRepo` (nil-safe, like `trainingplan.SetMultisportRepo`). In `summarize`, when `w.Sport == "multisport"` and `w.MultisportTemplateID != nil`, load the template and increment `BySport[seg.Sport]` once per **non-transition** segment; on any miss (repo unset, template deleted, load error) fall back to `BySport["multisport"]++`. `count` still increments once per workout; duration/kcal unchanged. Because `summarize` is currently pure over a `[]*workouts.Workout`, it gains a resolver parameter (a `func(id) ([]string, ok)` returning segment sports) so it stays unit-testable without a live repo.

### D3: Decomposition is best-effort and never errors the bundle
The training-context bundle is built "in parallel with no partial result on error" — the multisport lookup MUST NOT break that. A failed/again-empty template resolution degrades to the opaque `multisport` bucket (D2 fallback). No new error paths in the aggregate read.

## Risks / Trade-offs

- **Extra template loads in the context path** (one per multisport workout in the window) → Negligible: planned multisport workouts in a 14-day lookback are few; loads are by-id and can be memoized within the call if needed. Mitigation: only load when a `multisport` row is actually present.
- **`by_sport` counts now exceed `count`** for a window containing a brick (one workout → three sport buckets) → Intended and documented; `count` remains the session count, `by_sport` is per-discipline exposure. Spec scenario makes this explicit.
- **Derived duration disagrees with a future author-supplied estimate** → Not a risk this phase (no author field exists); if one is ever added, derived stays the fallback.

## Migration Plan

Pure code change, no DB migration. Deploy: ship the helper + wiring; `task swag` for the template response. Rollback = revert; nothing persisted.

## Open Questions

- Should the derived `estimated_duration_sec` also appear per-segment (each segment's own duration) for a richer template view? Defer — the total is what the coach/UI asked for; per-segment can be added additively later.
