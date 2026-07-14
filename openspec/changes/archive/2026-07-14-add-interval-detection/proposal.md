## Why

Double-voted in the gap analyses (GoldenCheetah's effort finder; Intervals.icu's namesake feature): Kazper can score *planned* structure (step compliance vs a template) but is blind inside unstructured rides — a self-directed "5×4 min hard" session shows only Garmin laps if the athlete pressed the button. Detecting sustained efforts from the stored 1 Hz power stream lets the coach see and discuss what was actually done ("you did 5 efforts averaging 4:10 at 305 W") without requiring the ride to have been planned or lap-buttoned.

## What Changes

- `GET /api/v1/workouts/{id}/intervals` — detects work intervals in the stored power stream: 30 s-smoothed power, an Otsu (bimodal) work/rest threshold derived **from the ride itself** (no parameters), gap-merge ≤ 30 s, minimum effort 60 s; returns the derived `threshold_w`, per-interval `{n, start_s, end_s, duration_s, avg_w, max_w, kj}`, rest gaps between them, and a summary (count, total work time, mean effort duration/power).
- A ride whose power distribution isn't meaningfully bimodal (steady endurance) returns `200` with `intervals: []` and `reason: "no_distinct_efforts"` — absence of intervals is a finding, not an error.
- New `detect_intervals` MCP tool (read tier, one GET, verbatim — the interval list is compact reasoning data, unlike raw series).
- Workout-detail dashboard page gains a detected-intervals table for power-streamed rides, absent when none detected.
- Compute-on-read, persists nothing, no migration; lands in `internal/activitystreams/` (per-workout stream computation, the W′bal home).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `activity-streams`: 2 ADDED requirements — the detection endpoint and the MCP tool.
- `coach-dashboard`: 1 ADDED requirement — the workout-detail detected-intervals table.

## Impact

- **Code:** `internal/activitystreams/intervals.go` (pure: smoothing, Otsu split, span assembly) + handler; `apps/web` detail-page table.
- **API/MCP:** one GET, one read tool, golden regen additive, `task swag`.
- **Out of scope (deferred):** run/swim pace-based detection, comparing detected vs planned structure (step-compliance owns planned), persisting detections, auto-classifying workout type from detections, tunable detection parameters (constants v1).
