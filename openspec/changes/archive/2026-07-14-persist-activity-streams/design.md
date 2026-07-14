# Design — persist activity streams + execution metrics

## Context

The bridge resamples Garmin activity detail to contiguous 1 Hz arrays (gaps → `0.0`) and
posts them to `POST /workouts/{id}/streams`; `internal/effortanalytics` computes the
mean-maximal ladder in-request and stores only the compact `workout_best_efforts` rows
(migration `053`). The raw arrays die with the request. The bridge currently extracts
only `directPower` and `directSpeed` from `metricDescriptors`
(`apps/garmin-bridge/garmin_bridge/mapping.py:_extract_streams`); no HR series is sent
today. Workouts already carry ingest-time derived scalars (`normalized_power_w`,
`intensity_factor`) and per-split `avg_hr`, but no full HR time series exists anywhere.

## Goals / Non-Goals

**Goals**

- Persist the raw sample streams so best-efforts (and any future analysis) can be
  recomputed without re-syncing from Garmin.
- Add the heart-rate stream to the bridge payload — required for EF and decoupling.
- Derive and store Variability Index, Efficiency Factor, and aerobic decoupling per
  workout.
- Expose retrieval (with downsampling) for future dashboard graphs, and an explicit
  recompute path.

**Non-Goals**

- GPS/position, cadence, temperature, or altitude streams (extend `stream_type` later if
  needed — the table shape already admits it).
- Any nutrition/hydration/energy coupling — streams stay unit-isolated like the rest of
  effort-analytics.
- Retention/pruning policy (see Decisions).
- Frontend graph work; this change only provides the API.

## Decisions

### 1. Storage: one row per (workout, stream_type) with a native `REAL[]` column

New table `workout_streams`:

```
id             UUID PRIMARY KEY
workout_id     UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE
stream_type    TEXT NOT NULL CHECK (stream_type IN ('power','speed','heart_rate'))
samples        REAL[] NOT NULL
sample_rate_hz INTEGER NOT NULL DEFAULT 1
sample_count   INTEGER NOT NULL
created_at / updated_at TIMESTAMPTZ
UNIQUE (workout_id, stream_type)   -- replace-on-repost, mirrors workout_best_efforts
```

Rejected alternatives:

- **Per-sample rows** — ~11k rows/hour/stream for data that is only ever read and
  written as a whole array, for a single user. Pure overhead: index bloat, slow replace,
  no query we'd ever run needs row-level samples.
- **JSONB array** — text-encoded floats are 2–3× the size of `REAL[]` and slower to
  decode; we gain nothing since we never query inside the array server-side.
- **bytea with an app-level codec** — invents a private binary format for no benefit;
  `REAL[]` is 4 B/sample, TOASTed and transparently compressed by Postgres, and stays
  inspectable in plain SQL.

`REAL` (float4) is precision-sufficient: power is integral watts, speed is m/s at
centimeter-per-second resolution, HR is integral bpm. `sample_rate_hz` is fixed at 1
today but stored so a future higher-rate source doesn't need a schema change;
`sample_count` is written explicitly so size/coverage queries never deserialize arrays.

### 2. Which streams: power, speed, heart_rate — bridge gains the HR series

The bridge extracts only `directPower`/`directSpeed` today. Garmin's activity detail
`metricDescriptors` also carries `directHeartRate`; `_extract_streams` is extended to
pull it as `heart_rate` (bpm), same conventions: 1 Hz column, missing samples → `0.0`,
an entirely non-positive series dropped. Extending the bridge payload is **part of this
change** — without it EF and decoupling are uncomputable. Cadence and other descriptors
are deliberately left out (no consumer yet; the CHECK constraint is widened when one
appears).

### 3. HR gaps: zeros are dropout, not signal

For power and speed, `0` is meaningful (coasting) and stays in every mean — that is the
established mean-maximal convention. For HR, `0` means sensor dropout: **all HR-derived
computation excludes zero samples**, and when valid HR samples cover less than **80%**
of the stream, every HR-derived metric (EF, decoupling) is left NULL rather than
computed from junk. Workouts with no HR stream at all simply get NULL EF/decoupling —
"not measured" is a meaningful state, per repo convention. The raw stored array keeps
the zeros untouched (storage is faithful; interpretation happens at compute time).

### 4. Derived metrics live as columns on the workout row, written at ingest

Three new nullable `workouts` columns:

```
variability_index NUMERIC(4,2) NULL CHECK (variability_index IS NULL OR variability_index > 0)
efficiency_factor NUMERIC(6,3) NULL CHECK (efficiency_factor IS NULL OR efficiency_factor > 0)
decoupling_pct    NUMERIC(5,1) NULL CHECK (decoupling_pct IS NULL OR decoupling_pct BETWEEN -100 AND 100)
```

Chosen over a compute-on-read `GET /workouts/{id}/execution` endpoint because: (a) the
repo precedent is exactly this — `intensity_factor` and `normalized_power_w` are stored
on the workout at ingest; (b) the metrics are three cheap scalars that the coach wants
inline on workout reads and future list/context projections, and computing them on read
would mean deserializing ~40 KB arrays per workout in list paths; (c) staleness is
handled by the recompute endpoint. They are written only by the stream ingest/recompute
path — never accepted on POST/PATCH (`field_immutable`-adjacent: they are simply not in
the mutable field set).

### 5. Formulas

With `P[i]` the 1 Hz power series, `V[i]` speed (m/s), `H[i]` HR with zeros excluded:

- **NP (normalized power)** = `( mean( rolling_mean_30s(P)^4 ) )^(1/4)`, computed only
  when the power stream spans ≥ 20 min (NP is not meaningful shorter).
- **VI (Variability Index)** = `NP / mean(P)` — power stream only (bike); NULL otherwise.
- **EF (Efficiency Factor)** = `NP / mean(H)` when a qualifying power stream exists,
  else `mean(V) / mean(H)` when a speed stream exists (run); NULL without valid HR.
- **Aerobic decoupling** — split the stream into equal halves; per half compute
  `r = mean(output) / mean(H)` where output is power (preferred) or speed;
  `decoupling_pct = (r1 − r2) / r1 × 100`, rounded to 1 dp (positive = HR drifted up
  relative to output = aerobic fade). Requires ≥ 20 min total and valid HR coverage in
  both halves.

All three round via `numfmt` conventions at the response boundary; VI to 2 dp, EF to
3 dp, decoupling to 1 dp.

### 6. Recompute path: explicit endpoint, ingest unchanged

`POST /workouts/{id}/streams/recompute` (no body) loads the stored streams, replaces the
workout's best-effort ladder (delegating to the existing `effortanalytics` computation)
and rewrites the three execution columns. `404 workout_not_found` for an unknown id,
`404 streams_not_found` when the workout has no stored streams. Chosen over
recompute-on-read (hides cost, caches poorly) and over a bulk "recompute everything" job
(YAGNI for a single user; the agent or a shell loop can iterate ids when a formula
changes). Ingest (`POST .../streams`) keeps computing everything at post time exactly as
today — the daily sync stays a single round trip per activity.

### 7. Package boundary: new `internal/activitystreams`, POST route moves there

`effort-analytics` stays what it is — the compact best-effort/curve capability. Storage,
retrieval, recompute, and execution metrics form the new `activity-streams` capability in
`internal/activitystreams/` (types/repo/service/handlers, per the standard shape).
Import direction: `activitystreams → {workouts, effortanalytics}` — acyclic, since
`effortanalytics` imports only `workouts`. The `POST /workouts/{id}/streams` route
registration moves to `activitystreams`, whose service persists the arrays, delegates
the mean-maximal replace to the `effortanalytics` service, then computes/stores the
execution columns — one transaction-ish sequence, wired in `httpserver.Run()` like every
other cross-injection. Endpoint path, method, and semantics are preserved for the bridge.

### 8. MCP parity: recompute tool yes, raw streams no

- **`recompute_workout_streams`** is added (Tier write, one `POST`, tiny response) — a
  genuine agent action ("my FTP changed, re-derive"). Registered in
  `internal/agenttools/`, golden schemas regenerated with `-tags=goldengen`; the MCP
  integration test derives the announced-tools list from the registry automatically.
- **`GET /workouts/{id}/streams` gets no MCP tool** — a deliberate, spec'd exception to
  the 1:1 convention. Justification: the convention exists so the agent can reach every
  capability, and it already tolerates the streams POST having no tool (it is a
  bridge-only write path). A verbatim forward of 10k+ floats would flood the agent
  context for zero reasoning value; everything the agent can actually use — VI/EF/
  decoupling on the workout row, best-efforts via `power_curve` — is already exposed. A
  downsampled MCP read can be added later if an agent use-case materializes.

### 9. Retrieval + downsampling

`GET /workouts/{id}/streams?downsample=<points>` returns
`{workout_id, sample_rate_hz, duration_s, streams: {power: [...], speed: [...], heart_rate: [...]}}`
with absent stream types omitted. Optional `downsample` (bounded `[10, 5000]`) reduces
each series by equal-width bucket means to at most that many points and echoes
`downsample` in the response; omitted → full resolution (a 3 h ride is ~10.8k floats
per stream, ~200 KB of JSON — fine for a REST dashboard client). Bucket-mean over
min/max-preserving (LTTB) because the consumers are trend graphs, not peak-hunting —
peaks are exactly what best-efforts already store.

### 10. Size / retention

1 Hz × 3 h ≈ 10,800 samples × 4 B ≈ 43 KB per stream, ×3 streams ≈ 130 KB per
instrumented workout; ~500 instrumented workouts/year ≈ **65 MB/year** before TOAST
compression, single user. That is noise for Postgres — **no retention policy in v1**,
no pruning task. `ON DELETE CASCADE` already reclaims space with workout deletion; the
`sample_count` column makes a future audit query trivial if this ever needs revisiting.

## Risks / Trade-offs

- **[Risk] Table bloat over years** → 65 MB/year estimate (above); native array storage
  + TOAST compression; revisit with a pruning migration only if the DB volume ever
  matters. `sample_count` enables cheap monitoring.
- **[Risk] Ingest gets slower/heavier (persist + compute vs compute-only)** → one extra
  upsert per stream (~43 KB write) per activity, once a day, single user; the bridge
  already guards each stream post individually so a failure never sinks the sync day.
- **[Risk] Route-ownership move (`effortanalytics` → `activitystreams`) breaks the
  bridge contract silently** → path/method/status contract is preserved verbatim and the
  existing effortanalytics handler tests move/extend rather than disappear; the bridge's
  `test_effort_streams.py` runs against the same payload shape.
- **[Risk] HR zeros poison EF/decoupling** → zeros excluded from all HR math + 80%
  coverage floor, else NULL (Decision 3).
- **[Risk] Decoupling on interval sessions reads as noise (it is only meaningful for
  steady aerobic work)** → computed and stored regardless (cheap, honest raw value); the
  coach/agent interprets alongside VI — a high VI flags the session as intervals, which
  contextualizes the decoupling number. Documented in the tool/field descriptions.
- **[Risk] MCP parity exception sets precedent** → the exception is written into the
  spec delta with its reasoning, not silently skipped; the recompute tool preserves
  parity for the agent-actionable part of the surface.

## Migration Plan

1. Verify the current migration head (`ls internal/store/migrations/ | tail` — around
   `054` now; out-of-band work sometimes takes the next slot), then
   `task migrate:new NAME=add_workout_streams`.
2. Up: create `workout_streams` (shape in Decision 1) + `ALTER TABLE workouts ADD` the
   three execution-metric columns (NULL, no back-fill — existing rows have no stored
   streams yet). Down: drop the table and the three columns.
3. No data migration: historical workouts simply have no streams until the bridge
   re-syncs/backfills a day, at which point ingest persists and derives as normal.
4. Deploy order: backend first (accepts the new `heart_rate` key additively), bridge
   second — an old bridge posting two series to a new backend is fully valid.

## Open Questions

- Should the bridge trigger a one-off historical backfill (re-post streams for old
  activities) after deploy, or let the rolling sync window fill forward only? Leaning
  forward-only in v1; a manual `POST /sync {date}` loop covers targeted backfill.
- Cadence stream (`directRunCadence`/`directBikeCadence`) — worth persisting while we're
  touching the extractor, or wait for a consumer? Deferred (Non-Goal) unless review says
  otherwise.
- Does the mobile dashboard want a fixed default `downsample` (e.g. 500) instead of
  full-resolution-by-default? API supports both; default chosen as full resolution to
  keep the endpoint contract dumb.
