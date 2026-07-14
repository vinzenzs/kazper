# Tasks

## 1. Backend

- [x] 1.1 Pure Spearman (tie-ranked) + same-day pairing in `internal/wellness`; unit tests (hand-computed rho fixtures incl. ties, per-field gating at n=14, no-pair days dropped)
- [x] 1.2 Narrow `PMCSeries` interface wired in `httpserver.Run()`; `GET /wellness/correlation` handler (`metric_invalid`, PMC range/tz contract)
- [x] 1.3 Integration tests: correlated fixture, sparse-field gate, metric matrix, range 400s, read-only
- [x] 1.4 `task swag`

## 2. MCP

- [x] 2.1 `wellness_correlation` read tool (association-not-causation description); golden regen + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes); live read once ≥14 diary days exist (expected: mostly `insufficient_pairs` at first — correct behavior)
