## 1. Bridge — run cadence extraction

- [x] 1.1 Extend `_extract_streams` in `apps/garmin-bridge/garmin_bridge/mapping.py`: fall back to the `directDoubleCadence` column (spm, posted as-is) when `directBikeCadence` is absent; bike wins when both exist; all-non-positive series still dropped
      _The fallback is gated on USABLE data, not mere presence: a bike column that exists but is all-zero (a ride with a dead sensor, or a run that reports an empty bike column) falls through to the run column rather than suppressing it. Covered by its own test._
- [x] 1.2 Add bridge tests in `apps/garmin-bridge/tests/test_effort_streams.py`: run double-cadence extracted as spm, bike-cadence preferred when both columns exist, missing both degrades to no cadence array
      _168 pass (was 163): +5 — the three listed, plus all-zero-double-cadence dropped and the dead-bike-column fallback. The run fixture deliberately carries NO power column, since the bug being guarded is that the fallback must fire on a payload shaped like a real run._

## 2. Stride analysis — pure computation

- [x] 2.1 Create `internal/activitystreams/stride.go`: pure function over paired speed+cadence samples — per-sample step length `speed/(spm/60)`, exclusion of non-positive (and sub-`min_speed_mps`) samples with excluded count, 0.25 m/s speed bins with seconds / mean cadence / mean step length, time-weighted log-log least-squares contribution split (cadence + step summing to 100), `null` split with `insufficient_speed_range` when qualifying bins span < 0.5 m/s, scatter thinned to ≤ 1000 triplets via the existing downsampling convention; full precision, no I/O
      _The range gate measures the span of BIN MIDPOINTS, not the nominal edges: two adjacent bins are 0.25 m/s apart by construction and comparing edges would fake a range the run doesn't have. The split is normalised by the slopes' actual sum — analytically they sum to 1, so this is a no-op that pins the reported pair at exactly 100% against float drift._
- [x] 2.2 Add `internal/activitystreams/stride_test.go`: synthetic run with known cadence/step composition recovers the expected split; steady-state run yields `null` split with reason; zeros excluded and counted; `min_speed_mps` filter excludes and counts; scatter capped at 1000
      _**The fixture caught itself being unphysical.** The first draft scaled cadence so a 5 m/s sample implied 97 spm and a 3 m step — the split math is scale-invariant so the tests passed, but the endpoint's plausibility assertion (rightly) failed. Re-scaled to a real runner (~170 spm / ~1.2 m at 3.5 m/s); a fixture nobody could run is a bad model to reason from. Also added: the opposite split, ragged arrays, single-bin, empty/all-excluded._

## 3. Endpoint

- [x] 3.1 Add types to `internal/activitystreams/types.go` and the service method: load speed+cadence streams, gate to sport run (`409 sport_unsupported` via sentinel error), map missing-data sentinels (`workout_not_found`, `streams_not_found`, `speed_stream_missing`, `cadence_stream_missing`)
      _The sport gate runs BEFORE the streams are touched: a ride with cadence would otherwise compute a plausible-looking number that means nothing._
- [x] 3.2 Add the `GET /workouts/{id}/stride` handler in `internal/activitystreams/handlers.go` with swag annotations: `min_speed_mps` validation (`400 min_speed_invalid`, bounds [0.5, 5.0], echoed when applied), `summary_only` omitting the scatter, rounding at the boundary (step length 2 dp, cadence/percentages 1 dp); register the route
- [x] 3.3 Add `internal/activitystreams/stride_endpoint_test.go`: happy path over stored streams, ride → 409, missing cadence → 404 sentinel, invalid `min_speed_mps` → 400, `summary_only=true` omits scatter, rounding asserted
      _Also: the plateau visible end-to-end, the steady-run 200-with-reason, `speed_stream_missing` as its own sentinel, the param bounds themselves valid, and a read-only check (streams byte-identical after, nothing derived persisted)._

## 4. Agent tool

- [x] 4.1 Register `stride_analysis` (read tier, `summary_only=true` always applied, args `workout_id` + optional `min_speed_mps`, description noting run-only and pace-variety preference) in `internal/agenttools/registry_activitystreams.go` with a test in `registry_activitystreams_test.go`
      _The description additionally tells the coach to sanity-check the numbers: an ~85 spm mean means the device reported single-foot cadence and the step length reads ~2× — say so rather than quoting a 2.4 m stride (the design's own risk, pushed to where the coach will actually meet it)._
- [x] 4.2 ~~Bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`~~
      _**Verified as a no-op — nothing to bump.** That list is DERIVED from the shared registry via `AnnouncedToolNames()` (the file's own comment: "new tools … are tracked automatically"), a property established when `unify-mcp-tool-registry` retired the hand-maintained drift guard. The hand-maintained artifact is the GOLDEN schema file, which did get its additive `stride_analysis` entry._

## 5. Dashboard

- [x] 5.1 Add the run workout-detail cadence-vs-stride view in `apps/web`: binned cadence/step-length series plus the contribution split; `insufficient_speed_range` renders the reason; non-run / missing streams / fetch failure → view absent, page unaffected
      _Both series are charted against speed on their own axes, because the finding IS the divergence (step length flat-lining while cadence climbs) — the split is a summary of that picture, not a substitute. The hook is gated on `sport === "run"` so the page never issues a request that can only 409. 84 web tests pass (+4); the sibling detail-view tests needed `useStride` added to their exhaustive hook mocks._

## 6. Verification

- [x] 6.1 `task swag` (handler request/response changed), `task vet`, `go test -count=1 ./internal/activitystreams/... ./internal/agenttools/...`, bridge test suite
      _Full Go suite green (one `TestQuadrantEndpoint_ParamValidation` blip was the documented testcontainers boot-contention flake — passes isolated, serially, and on re-run). Bridge 168, web 84, tsc clean._
- [ ] 6.2 End-to-end sanity: re-post a real run's streams through the updated bridge extraction (or a fixture), hit `/workouts/{id}/stride`, confirm the split and the run's mean cadence read plausibly (~160–180 spm)
      _(operator step — needs a real Garmin run + the deployed bridge. **Deploy order is free**: the backend ships first and simply returns `cadence_stream_missing` until the bridge update lands and a re-sync/backfill hydrates the streams. Historical runs stay `cadence_stream_missing` until re-posted.)_
