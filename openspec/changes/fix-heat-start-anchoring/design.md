## Context

`Forecast(ctx, lat, lon, Window{From: w.StartedAt, To: w.EndedAt})` — correct for workouts with real times, wrong for the scheduling path: Garmin's calendar is date-only, so materialized/scheduled planned workouts land at local midnight and the heat window covers 00:00 + duration. On a fast-warming day the read under-scores by several °C-equivalent (live-verified). The athlete's habitual start is early morning; specific sessions vary.

## Goals / Non-Goals

**Goals:** score the hours actually likely to be ridden; make the assumption visible whenever one is made; make "what if I start later" a first-class question; fix history retroactively without touching stored rows.

**Non-Goals:** write-side time stamping at materialize/schedule (a read-side sentinel is retroactive and total; stamping adds write-path churn for no additional correctness — revisit only if the visible assumption annoys); plan-slot preferred-time modeling; sub-hour forecast precision theatrics.

## Decisions

### D1 — Exact-local-midnight as the "unknown time" sentinel
Nobody starts a session at exactly 00:00:00; every scheduling-path row does. Treating that one instant as "unset" needs no migration, no new column, and heals all existing rows at read time. A genuinely intended midnight start (ultra events) would mis-assume — accepted and visible (`assumed_start` in the response); a real time one minute off midnight anchors exactly.

### D2 — `DEFAULT_TRAINING_START` config, `06:00` default
The habitual-start hour is athlete infrastructure, like `DEFAULT_USER_TZ` (and it interprets in that zone). Validated `HH:MM` at boot. Chosen over inferring from history (average completed-workout start hour) — inference drifts with season and surprises; a config states intent. The echo makes a wrong default self-evident.

### D3 — `start=HH:MM` param wins over everything
Param > workout time > assumed default; `start_source` names which applied, and the effective `window: {from, to}` is echoed beside the forecast values. Two calls give the coach the early-vs-late comparison that motivated this fix.

### D4 — Context inherits the assumption silently-but-visibly
The daily-context heat block computes with the same precedence (no params possible there) and carries `assumed_start` when it applied — the morning check-in shows "at your usual 06:00" framing for free.

## Risks / Trade-offs

- **The default lies when habits change** (winter indoor→outdoor shifts, holidays) — bounded by the echo + the `start` param + PATCHing real times onto specific sessions.
- **Window mean over a warming morning still averages** — a 06:00–08:30 window mean is the honest number for that ride; the param answers sensitivity questions.

## Migration Plan

None. Rollback = revert the anchoring (and re-inherit the midnight bug).

## Open Questions

- Stamp `DEFAULT_TRAINING_START` into `started_at` at materialize time later? Only if the visible assumption proves noisy — read-side semantics would remain as the fallback either way.
