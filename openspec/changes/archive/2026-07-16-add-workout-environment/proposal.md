## Why

The enabler for the weather/heat arc (explore session 2026-07-16): heat logic must distinguish trainer rides from road rides — acclimatization only accrues outdoors, planned indoor sessions must not get heat-adjusted, and EF-vs-temperature analytics must not be polluted by indoor sessions. Nothing marks that today; the only signal is the incidental absence of weather fields on indoor Garmin activities.

## What Changes

- Migration (next free slot; head currently `064`): nullable `environment TEXT CHECK (environment IN ('indoor','outdoor'))` on `workouts` — null = unknown/unstated (no back-fill; the `training_focus` precedent).
- Settable on POST/bulk, **tri-state on PATCH** (`"indoor"`/`"outdoor"` set, `""` clears, omitted unchanged — the established empty-string-sentinel convention); returned `omitempty`; invalid → `400 environment_invalid`.
- garmin-bridge: derive it defensively from Garmin's activity type (`indoor_cycling`, `virtual_ride`, treadmill runs, pool vs open-water swims — pool counts indoor for weather purposes) and include it in the bulk items; unknown type → omitted, never a crash. History fills through the rolling re-sync window.
- No new MCP tool (rides the existing workout shapes).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `workouts`: 1 ADDED requirement — the environment field and its write semantics.
- `garmin-bridge`: 1 ADDED requirement — activity-type → environment mapping in the sync items.

## Impact

- **Code:** migration; `internal/workouts` types/validation/PATCH plumbing; bridge mapper + tests; `task swag`.
- **Known caveat (accepted, the `training_focus` precedent):** a Garmin re-sync's full-replace re-applies the derived value, clobbering a manual override on a synced workout — flip to COALESCE later if it bites.
- **Out of scope:** environment on templates/plan slots (planned workouts can be PATCHed directly; a template-level default is a later nicety), any heat logic (next changes).
