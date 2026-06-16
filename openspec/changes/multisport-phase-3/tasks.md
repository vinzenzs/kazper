## 1. Derived multisport template duration

- [x] 1.1 Add a pure helper `estimatedDurationSec(segments []Segment) *int` in `internal/multisport/`: sum each transition segment's `Duration.Seconds` (kind `time`) + each sport segment's `workouttemplates.SumTimedDurationSec(steps)`; return nil if any sport segment has a non-time-bounded step or any transition is not kind `time`.
- [x] 1.2 Add `EstimatedDurationSec *int json:"estimated_duration_sec,omitempty"` to the multisport template response and populate it (service/handler read path) on single GET and list — computed, not stored.
- [x] 1.3 Unit-test the helper: fully-time-bounded → summed total; a distance/open/lap_button sport step → nil; a non-time transition → nil; transitions-only edge.

## 2. by_sport decomposition in training context

- [x] 2.1 Cross-inject a nil-safe `multisportRepo` (`GetByID`) into `coachcontext.Service` (`SetMultisportRepo`), wired in `internal/httpserver/server.go`.
- [x] 2.2 Refactor `summarize` to accept a segment-sport resolver (`func(id string) ([]string, bool)`); for a `multisport` workout with a `multisport_template_id`, increment `BySport` once per non-transition segment sport; on unresolved/error, fall back to one `multisport` entry. Keep `count`/`total_duration_min`/`total_kcal` one-per-workout.
- [x] 2.3 Build the resolver in the service from the injected repo (load template by id → list of non-transition segment sports), best-effort and memoized within the call; never error the bundle.

## 3. Tests

- [x] 3.1 Unit-test `summarize`: a multisport workout decomposes into its segment sports in `by_sport` while `count` increments by one; an unresolvable multisport workout falls back to the `multisport` bucket; single-sport unaffected.
- [x] 3.2 Integration-test `GET /context/training`: a window containing a materialized multisport planned workout shows `swim`/`bike`/`run` in `by_sport` (not `multisport`), with `count` reflecting one session.
- [x] 3.3 Integration-test `GET /multisport-templates/{id}`: a fully time-bounded template returns the summed `estimated_duration_sec`; a template with an open/lap-button step omits it.

## 4. Docs & verification

- [x] 4.1 Run `task swag` to regenerate `docs/` for the multisport-template response shape (new `estimated_duration_sec`).
- [x] 4.2 Run `task test` and `task vet`; confirm the MCP integration expected-tools list still passes (no new tools).
