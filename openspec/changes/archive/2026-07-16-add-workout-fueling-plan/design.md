## Context

`internal/workoutfueling/` composes workouts + workout-fuel + hydration (the fueling summary, sweat rate). Planned workouts carry planned TSS/duration/IF-implying intensity; effective FTP is available through the post-threshold-split adapter. `race-fueling-plan` covers races with authored per-leg targets; training days have nothing — yet they're where fueling strategy gets rehearsed.

## Goals / Non-Goals

**Goals:** a pre-session read answering "how much will this burn and what should I take in", auditable inputs, degradations that keep partial answers useful.

**Non-Goals:** product-level scheduling (products lack serving sizes — modeling that is its own change); compliance scoring against the eventual workout-fuel log (needs this to exist first); metabolic profiling beyond the IF→CHO ladder; hydration/sodium prescription (sweat-rate and the race plan own those angles).

## Decisions

### D1 — Work from planned TSS × effective FTP
`TSS = 100 ≈ one hour at FTP ≈ FTP × 3.6 kJ`, so `kJ = planned_tss/100 × FTP × 3.6` — exact under the TSS definition, no stream needed, works for any planned session with a TSS estimate. Energy expenditure uses the standard cycling convention kJ_work ≈ kcal (gross efficiency ~24 % ≈ 1/4.184 nearly cancels). FTP comes from the **effective** config (garmin-sourced when the athlete chose so).

### D2 — CHO fraction ladder over planned IF
Planned IF = `sqrt(planned_tss/100 ÷ hours)` when not directly present. Fractions 45/55/70/80 % across the four IF bands bracket the substrate-utilization literature (crossover concept); constants v1, echoed in the response with the IF that selected them. Carbs at 4 kcal/g.

### D3 — Intake prescription is duration-gated guidance clamped by explicit capacity
The 0 / 30–60 / 60–90 g/hr duration ladder is standard sports-nutrition guidance; the optional `carbs_per_hr` param (validated > 0, ≤ 130) caps the upper bound at the athlete's tested gut capacity — explicit-params convention (the athlete's tolerance is theirs to assert, and rehearsal logs are where it gets discovered). Output: `per_hour_g` range, `session_total_g` range, and `projected_deficit_g` = burn − max intake, the number that tells a coach whether post-ride carbs need emphasis.

### D4 — Planned workouts only
The pre-session question is the product; a completed ride's retrospective ("what should I have taken") composes from existing reads and is deferred with compliance. Completed/other statuses → `409 workout_not_planned` (1:1 sentinel).

### D5 — Layered degradation
No planned TSS *and* no duration → `plan_data_missing` (nothing computable). Duration without TSS → intake guidance only (`reason: "tss_missing"`, burn omitted). TSS without FTP → same shape with `ftp_missing`. Every partial answer states what it lacks — the race-pacing per-leg-degradation posture.

### D6 — Home: `workout-fuel` capability, `internal/workoutfueling/` package
It is workout-anchored fueling math beside the summary and sweat rate; the spec delta rides the same capability. The athlete-config consumption gate gains the FTP read (MODIFIED delta, fail-open like every other consumer).

## Risks / Trade-offs

- **Planned TSS quality bounds burn accuracy** — inherent; the response echoes its inputs, and a template without a TSS estimate degrades visibly rather than guessing.
- **kJ≈kcal and the CHO ladder are conventions** — documented as such; this is a planning anchor, not a metabolic lab.
- **Overlap confusion with race-fueling-plan** — division stated in both tool descriptions: races carry *authored* plans; this computes *training-day* prescriptions.

## Migration Plan

None. Rollback = revert route/tool.

## Open Questions

- Product-slotted schedules once products model serving sizes (the "gel at 1:30" UX).
- Plan-vs-actual compliance + carb-capacity trending (needs accumulated rehearsal pairs).
