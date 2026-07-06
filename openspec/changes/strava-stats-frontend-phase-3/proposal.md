## Why

The one Strava/intervals.icu statistic Phases 1–2 can't deliver is the **power (and pace) curve** — best mean power sustained for every duration from ~5s to ~60min, aggregated across a window ("mean-maximal power"). It's the single most-used analytical view for a cyclist and the backbone of FTP/threshold estimation. It's absent because the backend stores only per-lap `splits`, never the per-second time-series streams a curve requires. This is the gated, largest phase: it adds a real data-ingestion path (Garmin streams) plus storage and a curve rollup. Propose it now so the shape is captured; build it only when the curve is actually wanted.

## What Changes

- **Garmin bridge ingests per-activity streams.** `apps/garmin-bridge/` fetches `get_activity_details(activity_id)` alongside the splits/zones/sets it already pulls, extracts the per-sample **power** and **speed** (→ pace) series (plus timestamps/distance), and posts them to a new backend endpoint. The per-activity fetch stays guarded (an activity with no power stream — e.g. a run without a meter — simply omits it, exactly like weather today).
- **New backend capability `effort-analytics`** (`internal/effortanalytics/`, new package) with a schema migration.
  - **`POST /api/v1/workouts/{id}/streams`** — accepts the 1 Hz power/speed sample arrays for one workout; the **backend** computes that activity's best-mean-power and best-pace at a fixed set of durations (mean-maximal), stores the compact per-activity **best-effort records**, and (decision below) does **not** retain the raw streams long-term.
  - **`GET /api/v1/workouts/power-curve?from=&to=&sport=&tz=`** — returns the aggregated mean-maximal curve over the window: for each standard duration, the best value achieved and the workout it came from. Pace curve is the same shape for run/swim.
- **MCP mirror `power_curve`** — one read tool issuing the single GET, per the REST↔MCP 1:1 convention.
- **New frontend view: the power/pace curve chart** (visx log-x line, analyst idiom) on `/stats` (or a `/stats/curve` sub-route), with the sport + window selector. The per-workout detail page (Phase 1) MAY gain that activity's own curve overlaid on the all-time best.
- Unit isolation holds: power (W), pace (s/m), and the curve live only in this capability's shapes and feed no nutrition/energy total.

**Decision (see design):** store **precomputed per-activity best-efforts**, not raw streams. The curve is exactly a mean-maximal aggregation, best-efforts serve it directly and compactly, and a single-user Postgres shouldn't carry tens of thousands of raw samples per activity indefinitely. Trade-off: new analytics that need raw samples would require a re-pull. Raw streams are computed-over in-request and discarded (optionally: retain a downsampled 1 Hz stream only for the workout-detail chart — flagged as an open question, default off).

Out of scope: HR-curve and W′/critical-power modeling (the curve is the input to CP models, not the model itself); segment/lap leaderboards; live/real-time streaming. Backfilling historical activities' curves is a follow-on (the bridge's existing backfill path can re-drive it).

## Capabilities

### New Capabilities
- `effort-analytics`: per-activity mean-maximal best-effort records (power and pace at standard durations) ingested from Garmin activity streams, and a windowed aggregated power/pace curve exposed at `GET /workouts/power-curve` (+ ingest at `POST /workouts/{id}/streams`), mirrored as the `power_curve` MCP tool.

### Modified Capabilities
- `garmin-bridge`: the daily/backfill sync additionally fetches each activity's detail streams (power/speed samples) and posts them to `POST /workouts/{id}/streams`; the per-activity fetch stays individually guarded and idempotent (re-posting an activity's streams replaces its best-efforts, not duplicates).
- `coach-dashboard`: adds a power/pace curve chart (sport + window selector) to the stats surface under the existing analyst aesthetic.

## Impact

- **Bridge** (`apps/garmin-bridge/`): new `get_activity_details` fetch in `garmin_client.py`; stream extraction + POST in `sync.py`/`mapping.py`/`backend.py`; Python tests. The `garminconnect` upgrade already on `main` (0.3.6) exposes the details call.
- **New backend package** `internal/effortanalytics/` (`types.go`, `repo.go`, `service.go` with the mean-maximal algorithm, `handlers.go`, tests); a migration (**next after 052 → 053**) creating a `workout_best_efforts` table (workout_id FK, metric, duration_s, value, achieved_at); wired in `internal/httpserver/server.go`; **`task swag` required**.
- **New MCP tool** `power_curve` in `internal/agenttools/registry_effortanalytics.go`; bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`.
- **Frontend** (`apps/web/`): `usePowerCurve(from,to,sport)` hook + types; a visx log-x curve chart; sport/window selector; optional per-workout curve overlay on the detail page.
- **API surface added:** `POST /api/v1/workouts/{id}/streams`, `GET /api/v1/workouts/power-curve`. **New MCP tool:** `power_curve`.
- **Data volume:** best-efforts are ~N durations × metric per workout (tiny). Raw streams are large but transient (computed-over then discarded by default).
