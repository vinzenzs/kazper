## Why

The web dashboard already renders the hard part — the intervals.icu-style analyst core (form/ACWR, load trend, fitness, race predictions, power thresholds, zones, recovery). But the Strava-shaped data the backend already mirrors from Garmin — **personal records, gear mileage, and achievements** — is exposed over REST yet never shown. Per-workout detail (splits, HR-zone time) is likewise fetchable but the dashboard only lists workouts. This is a surfacing gap, not a data gap: the endpoints exist, the SPA just doesn't read them. This is Phase 1 of blending the motivational Strava surface onto the existing analyst dashboard.

## What Changes

- Introduce **client-side routing** (`react-router-dom`) so the SPA is no longer a single scroll. The current dashboard moves to `/`; new routes are added. The server-side SPA fallback (`RegisterSPA` → `index.html` for non-API GETs, JSON 404 preserved under `/api/v1`) already supports client-side routes, so this is a **frontend-only** change with no backend work.
- New route **`/records`** — personal records (best efforts: fastest 5k/10k, longest ride, …) as a dense table, plus an achievements chip strip. Reads `GET /personal-records` and `GET /achievements`.
- New route **`/gear`** — gear inventory with mileage, retired gear dimmed. Reads `GET /gear`.
- New route **`/workouts/:id`** — per-activity detail: splits table + reused HR/power zone strip. Reads `GET /workouts/:id` (the single-get that returns `splits`, `sets`, `secs_in_zone_*` — data the list-shaped context payload omits). Workout rows in the existing `WorkoutList` become links into this route.
- **Header** gains nav links (Dashboard · Records · Gear) in the existing style.
- The visual treatment stays **analyst, not celebratory** — reuse `Panel` / `Stat` / `ZoneStrip`, muted Tailwind + visx idiom. No trophy cards, badge walls, or social/motivational styling. (Explicit non-goal.)
- **BREAKING (spec-level, not API):** the `coach-dashboard` v1 constraint "reads data exclusively from the `/context/*` payloads — no new fetch is introduced" is relaxed. The SPA may now read capability endpoints directly and fetch a single entity by id from a URL param.

Out of scope (deferred to later phases, captured in design notes): volume/totals rollups (weekly/monthly/YTD by sport) and the activity heatmap — need a new backend aggregation endpoint (**Phase 2**); power/pace curve — blocked on per-second stream ingestion the backend does not do today (**Phase 3**, gated).

## Capabilities

### New Capabilities
<!-- none — this extends the existing dashboard capability -->

### Modified Capabilities
- `coach-dashboard`: the dashboard becomes multi-route (adds `/records`, `/gear`, `/workouts/:id` alongside the existing training view at `/`); the SPA is permitted to read capability endpoints (`/personal-records`, `/gear`, `/achievements`) and a single workout by id (`/workouts/:id`), not just the `/context/*` bundles; new panels for personal records, achievements, gear mileage, and per-workout detail are added under the existing analyst aesthetic.

## Impact

- **Frontend only** (`apps/web/`): new dep `react-router-dom`; new route components (`RecordsView`, `GearView`, `WorkoutDetailView`) and panels (personal records table, achievements strip, gear mileage table, splits table); new React Query hooks (`usePersonalRecords`, `useGear`, `useAchievements`, `useWorkout(id)`); `WorkoutList` rows become `<Link>`s; `Header` gains nav; `App.tsx` becomes a router shell wrapping the existing dashboard.
- **No backend changes** — all four endpoints already exist and the SPA fallback already serves client-side routes. No new Go code, no migration, no `task swag`.
- **API surface consumed** (existing): `GET /personal-records`, `GET /achievements`, `GET /gear`, `GET /workouts/:id`.
- **Build/embed:** the new dep and routes ship through the existing pipeline SPA build; `apps/web/dist` is produced in CI (no committed-dist assumption).
