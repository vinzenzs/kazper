## Context

The capability template (types/repo/service/handlers, sentinel errors 1:1) fits directly; `workout-fuel` is the nearest sibling — a capture-only, unit-isolated event log with a required free-text `name`. Supplements differ in having no measurable-intake requirement (the name alone is the record: "vitamin D") and no workout anchoring.

## Goals / Non-Goals

**Goals:** a queryable dated record of what was taken; conversational capture via the coach; zero coupling into nutrition totals.

**Non-Goals:** a supplement catalog, protocol/compliance tracking (revisit if a real protocol needs enforcement), dose normalization across units, feeding any summary total.

## Decisions

### D1 — Timestamped events, not per-date singletons
Unlike wellness (one state per day), supplements are discrete events (iron at breakfast, magnesium at night) — `logged_at TIMESTAMPTZ` rows, multiple per day, the workout-fuel shape. Window queries bucket by the athlete's local date at read time.

### D2 — `name` required; `dose`/`dose_unit` paired or absent; no macro fields
A bare name is a valid record. A dose without a unit (or vice versa) is rejected (`dose_pair_required`) — a number with no unit is noise later. Deliberately no kcal/macro columns: anything with meaningful macros is a meal or workout-fuel; the unit-isolation line holds.

### D3 — No PATCH
Delete + re-log (coach-memory/coach-recommendations precedent) — corrections to a two-field event don't warrant tri-state machinery.

### D4 — Errors and window
`400 name_required`, `400 dose_pair_required`, `400 dose_invalid` (≤ 0), `404 not_found`; window: ascending, `200 {"entries":[]}` empty, 92-day cap with the shared range vocabulary (wellness precedent — recent-block-shaped questions).

### D5 — `/context/daily` carries today's entries verbatim
An array (events, not a singleton), omitted when empty — sibling of the wellness object, same snapshot posture; history stays behind `list_supplements`.

## Risks / Trade-offs

- **Free-text names drift** ("vit D" / "vitamin D3") — accepted for v1; the coach normalizes conversationally, and a catalog is the deferred fix if querying suffers.

## Migration Plan

Migration on the next free slot (verify head — sibling proposals also carry migrations). Down drops the table. Export-included classification lands in the same change (drift guard).

## Open Questions

- Protocol layer (declared regimen + adherence read) — only if a real protocol shows up.
