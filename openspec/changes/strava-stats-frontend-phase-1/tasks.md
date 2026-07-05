## 1. Routing foundation

- [x] 1.1 Add `react-router-dom` to `apps/web/package.json` and install
- [x] 1.2 Extract the current `App.tsx` dashboard body into a `DashboardView` component (`/` route), keeping its hooks and layout unchanged
- [x] 1.3 Add a `Layout` component that renders `Header` + persistent nav and an `<Outlet />`; move `Header` into it
- [x] 1.4 Rewrite `App.tsx` as a `<BrowserRouter>` + `<Routes>` shell wiring `/` → `DashboardView`, `/records` → `RecordsView`, `/gear` → `GearView`, `/workouts/:id` → `WorkoutDetailView`
- [x] 1.5 Add header nav `<Link>`s (Dashboard · Records · Gear) styled in the existing idiom
- [x] 1.6 Verify deep-link + reload on each route resolves via the existing SPA fallback (no backend change)

## 2. Data layer (types + hooks)

- [x] 2.1 Add TS response types in `api/types.ts`: `PersonalRecord`, `Achievement`, `Gear`, and a detail `Workout` (with `splits`, `sets`, `secs_in_zone_*`) mirroring the Go JSON shapes (only fields rendered)
- [x] 2.2 Add `usePersonalRecords`, `useAchievements`, `useGear` list hooks in `api/hooks.ts` following the existing `apiGet` + `SLOW_INTERVAL_MS` pattern
- [x] 2.3 Add `useWorkout(id)` hook keyed `["workout", id]` with `enabled: !!id`

## 3. Records route

- [x] 3.1 Build a personal-records table panel (PR type · value+unit · achieved-at date), reusing `Panel`
- [x] 3.2 Build an achievements chip strip
- [x] 3.3 Link a PR row to `/workouts/:id` only when its `activity_id` resolves to a known workout; otherwise render the row plainly
- [x] 3.4 Assemble `RecordsView` with loading / error / empty-state handling (match `WorkoutList` pattern)

## 4. Gear route

- [x] 4.1 Build a gear table panel: type · name · accumulated distance with a thin muted mileage bar
- [x] 4.2 Dim (de-emphasize) retired gear rather than hiding them
- [x] 4.3 Assemble `GearView` with loading / error / empty-state handling

## 5. Workout detail route

- [x] 5.1 Build `WorkoutDetailView` reading `useWorkout(id)`: summary metrics via `Stat`/`Panel`
- [x] 5.2 Render a splits table (per-lap distance, duration, pace/speed, HR, power where present); omit gracefully when no splits
- [x] 5.3 Render HR/power zone time reusing `ZoneStrip`/`Zones`
- [x] 5.4 Handle unknown-id (API 404) as a not-found state, not a shell error
- [x] 5.5 Make `WorkoutList` rows `<Link>` to `/workouts/:id`

## 6. Tests & build

- [x] 6.1 Add vitest component tests for each new view: populated render + empty-state (following the existing `__tests__` pattern)
- [x] 6.2 Add a routing test: nav link navigates and a deep-linked route renders its view
- [x] 6.3 Run `apps/web` lint + typecheck + `vitest`; build the SPA and confirm the `webembed` serving/fallback tests still pass
