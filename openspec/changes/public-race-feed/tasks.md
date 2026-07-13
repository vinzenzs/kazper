## 1. Config

- [ ] 1.1 Add `FeedSecret string` with `mapstructure:"FEED_SECRET"` to `internal/config/config.go` (and its env allowlist); document it as the shared secret Strapi presents. Empty ⇒ feed disabled.

## 2. Package `internal/publicfeed`

- [ ] 2.1 `types.go` — the curated projection shape: `Race{Name, RaceDate}` and the response `{Race *Race, DaysRemaining *int}` (both nullable for the graceful-empty case). Non-PII only.
- [ ] 2.2 `repo.go` — read-only query resolving the active macrocycle (`[start_date,end_date]` contains today, tie-break latest `start_date`) joined to its `race_id` → race `name`/`race_date`; returns "no active anchored race" distinctly from an error.
- [ ] 2.3 `service.go` — compute `days_remaining` = whole days from today (configured user tz) to `race_date`, floored at 0; assemble the projection; map "no active race" → nulls.
- [ ] 2.4 `handlers.go` — `GET /public/race-feed`: constant-time `X-Feed-Key` vs `FEED_SECRET` check (`crypto/subtle`); `401 feed_unauthorized` on mismatch; `503 feed_disabled` when the secret is unset; swag annotations. `Register(r *gin.Engine)` on the root engine.

## 3. Wiring

- [ ] 3.1 In `internal/httpserver/server.go`, register the feed on the root engine **before** `auth.Middleware` (alongside `/healthz`/`/readyz`), passing the user timezone and `FEED_SECRET`; the handler self-gates to `503` when the secret is unset.

## 4. Tests

- [ ] 4.1 Handler/integration tests (testcontainers): valid key → 200 with `{race,days_remaining}` and NO PII fields; missing/wrong key → `401`; unset secret → `503`.
- [ ] 4.2 Resolution tests: active macrocycle's A-race resolved with correct countdown; race-day floors at 0; no active macrocycle / `race_id` null / missing race → `200 {race:null, days_remaining:null}`; overlapping macrocycles pick the latest `start_date`.

## 5. Docs

- [ ] 5.1 `task swag` to regenerate `docs/` for the new root route.
- [ ] 5.2 Document `FEED_SECRET` (README/RUN_LOCAL env table) and add a short note describing the external topology (Strapi shield + separate frontend pull the feed) so the seam's intent is discoverable — the Strapi/frontend themselves are out of this repo.

## 6. Verify

- [ ] 6.1 `task test` (or the `publicfeed` + `httpserver` packages) green.
- [ ] 6.2 `go vet ./...` and `task swag` clean; confirm the route is reachable WITHOUT a bearer token and rejected without the feed key.
