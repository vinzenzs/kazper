## Context

The web dashboard (`apps/web/`, Vite + React + TS + Tailwind, charts via visx, data via `@tanstack/react-query`) currently renders a single-column, single-route training composite (`App.tsx`) reading only `GET /context/training`, `GET /context/recovery`, and `GET /fitness-metrics`. The `coach-dashboard` v1 spec deliberately constrained it to "reads exclusively from context payloads — no new fetch."

Meanwhile the backend already mirrors Strava-shaped data from Garmin and exposes it over REST that the SPA does not read: `GET /personal-records`, `GET /achievements`, `GET /gear`, and `GET /workouts/:id` (single-get with `splits`, `sets`, `secs_in_zone_*`). Server-side, `RegisterSPA` already serves `index.html` for any non-API GET and preserves the JSON 404 contract under `/api/v1`, so client-side routing has no backend prerequisite.

This is Phase 1 of a three-phase arc surfaced in explore mode: (1) surface existing data + add routing (this change), (2) volume/totals rollups needing a new backend aggregation endpoint, (3) power/pace curve gated on per-second stream ingestion the backend does not do today.

## Goals / Non-Goals

**Goals:**
- Introduce client-side routing so the SPA is multi-view without a backend change.
- Surface personal records, achievements, and gear mileage — data already fetchable, never shown.
- Add a per-workout detail drill-down (splits + zone strip) reachable from the workout lists.
- Keep everything inside the existing analyst visual language.

**Non-Goals:**
- No backend code, migration, endpoint, or `task swag` — Phase 1 is frontend-only.
- No volume/totals rollups or activity heatmap (Phase 2 — needs a `/workouts/summary` aggregation endpoint).
- No power/pace curve (Phase 3 — blocked; the backend stores per-lap splits, not per-second streams, so a real best-power-per-duration curve cannot be computed from current data).
- No celebratory/social visual treatment (trophy cards, badge walls). Analyst aesthetic only.
- No auth/routing-guard changes — HTTP Basic is browser-native as today.

## Decisions

### Routing: `react-router-dom` (BrowserRouter)
Add `react-router-dom` as the one new dependency. `App.tsx` becomes a router shell: `<BrowserRouter>` wrapping a `<Routes>` with the current dashboard extracted into a `DashboardView` at `/`, plus `/records`, `/gear`, `/workouts/:id`. `Header` moves into a persistent layout with nav `<Link>`s.

- **Why BrowserRouter over HashRouter:** the server already does SPA fallback to `index.html` for non-API GETs, so clean paths deep-link and reload correctly. Hash routing would be a workaround for a problem we don't have.
- **Why a library over hand-rolled state routing:** a URL param (`/workouts/:id`) plus deep-linking is exactly what a router is for; hand-rolling `history` handling is more code and less testable. `react-router-dom` is the ecosystem default and small.
- **Alternative considered — keep single-scroll, no router:** rejected because the workout detail view needs an id in the URL for deep-linking/back-button, which forces routing anyway.

### Data access: extend the existing `apiGet` + React Query hook pattern verbatim
New hooks in `api/hooks.ts` mirror the existing ones: `usePersonalRecords`, `useAchievements`, `useGear` (list queries, `SLOW_INTERVAL_MS` backstop like the others), and `useWorkout(id)` (keyed `["workout", id]`, `enabled: !!id`). New response types go in `api/types.ts` mirroring the Go JSON shapes (`PersonalRecord`, `Achievement`, `Gear`, `Workout` with `splits`/`sets`/`secs_in_zone_*`).

- **Why not a new fetch abstraction:** `apiGet` already handles same-origin Basic credentials and the `ApiError` shape. Consistency > novelty.
- **Note:** this is the spec-level change — the SPA now reads capability endpoints and a single entity by id, relaxing the v1 "no new fetch" constraint (scoped in the delta to apply only to the home route).

### Reuse primitives; no new visual language
Records = dense table (reuse `Panel` + table styling); achievements = compact chip strip; gear = table with a thin muted mileage bar (a bare Tailwind div width %, or visx if a shared scale helps) and dimmed retired rows; workout detail reuses `ZoneStrip`/`Zones` for zone time and `Stat`/`Panel` for summary metrics. Every view degrades gracefully on empty/null (matching the existing `isLoading` / `isError` / empty-hint pattern already in `WorkoutList`).

- **Why:** the locked decision is analyst-not-celebratory. Reusing primitives guarantees visual coherence and minimizes new code.

### Workout detail sources from single-get, not the context payload
The home route's `recent_workouts` are list-shaped (no splits/zones). `/workouts/:id` fetches `GET /workouts/:id` on demand, which returns the nested detail. `WorkoutList` rows become `<Link to={`/workouts/${w.id}`}>`.

## Risks / Trade-offs

- **[Spec relaxation — SPA now fans out to capability endpoints]** → The v1 "single bundle" simplicity is intentionally traded for surfacing existing data. Mitigated by scoping the relaxation in the delta spec to non-home routes and keeping the home route's composite unchanged.
- **[`react-router-dom` bundle size / new dep in an embedded SPA]** → Small, tree-shaken, ships through the existing pipeline build into `apps/web/dist`; no runtime/server impact since serving is static.
- **[Zone/PR/gear JSON shapes drift from Go types]** → Hand-written TS types could fall out of sync. Mitigated by keeping them minimal (only fields rendered) and covering render + empty-state with vitest component tests, matching the existing `__tests__` pattern.
- **[PR `activity_id` may not resolve to a known workout]** → Garmin PR `activity_id` is an external id string, not necessarily a Kazper workout id. Mitigated by linking to detail only when it resolves and otherwise rendering the row plainly (no dead link).
- **[Deep-link 404 for a bad workout id]** → `useWorkout` surfaces the API 404 as a not-found state in the detail view rather than erroring the shell.

## Migration Plan

Purely additive frontend change; no data migration, no API change, no rollback coordination.
1. Add `react-router-dom`; refactor `App.tsx` into a router shell with a `Layout` (Header + nav) and extract `DashboardView`.
2. Add types + hooks; build `RecordsView`, `GearView`, `WorkoutDetailView` and their panels.
3. Link `WorkoutList` rows to detail.
4. Component tests for each new view (render + empty-state). Build the SPA through the existing pipeline; the `webembed` real-embed test still passes since serving/fallback is unchanged.
- **Rollback:** revert the frontend commit; the backend is untouched.

## Open Questions

- Does the Garmin-mirrored PR `activity_id` reliably map to a Kazper `workouts.id`, or only to a Garmin external id? Determines whether the PR→workout link is usually live or usually absent. (Resolvable during apply by inspecting real rows; the design tolerates either.)
- Should `/records` and `/gear` share one "Stats" parent route/tab group, or stay flat top-level routes? Leaning flat for Phase 1; revisit if Phase 2 adds a totals view that wants to co-locate.
