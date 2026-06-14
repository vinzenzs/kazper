# Tasks: unify-mcp-tool-registry

> Depends on `expand-chat-to-coach` phase 1 (agenttools exists) + task 1.5 (shared idempotency derivation). Ported group-by-group (DD7); keep `mcp_integration_test.go` green after every group.

## 1. agenttools: carry the full surface

- [ ] 1.1 Add a visibility marker to `agenttools.Spec` (DD1) — e.g. `ChatExposed bool` (or `Surfaces []Surface`); default existing 14 chat tools to chat-exposed. `internal/chat` filters to chat-exposed entries; `internal/mcpserver` ignores it.
- [ ] 1.2 Add an optional typed schema source to `Spec` (DD2): `SchemaType any` + a resolver that reflects it to a JSON-Schema string using the **same library the MCP SDK uses**; entries provide either `Schema` (string) or `SchemaType`. Keep the existing 14 string schemas working.
- [ ] 1.3 Golden test: for each tool with a `SchemaType`, the reflected JSON Schema equals the schema the MCP SDK announced before this change (capture a snapshot first). This is the safety gate before deleting any bespoke registration.
- [ ] 1.4 Confirm `agenttools.EffectiveIdempotencyKey`/`DeriveIdempotencyKey` exist (from `expand-chat-to-coach` 1.5); if not yet landed, land them here first.
- [ ] 1.5 Split the registry into per-domain files (`registry_meals.go`, `registry_workouts.go`, …) concatenated by `Registry()` (DD-risk: big file).

## 2. mcpserver: generic dispatch path

- [ ] 2.1 `apiClient` adapter: execute an `agenttools.HTTPCall` by switching on `Method` (`Get/Post/Patch/Put/Delete`), passing `Query`/`Body` and the idempotency key for write tiers.
- [ ] 2.2 `dispatchMCP(ctx, c, spec, raw)` (DD3): `spec.Build(raw)` → `HTTPCall` → adapter → existing `toToolResult`; attach `EffectiveIdempotencyKey(explicit-from-raw, name, raw)` for writes (DD4).
- [ ] 2.3 Generic registration loop over `agenttools.Registry()` using the SDK's raw-schema/untyped registration entry point; confirm the SDK exposes it, else add a one-line generated typed shim per tool (DD3 fallback).
- [ ] 2.4 Multipart escape hatch (DD5): register `log_meal_from_photo` outside the generic loop (documented single exception), still a registry entry for naming/discovery.

## 3. Port tool groups (DD7 — repeat per group, integration test green each time)

- [ ] 3.1 For each domain (meals, meal-plan, shopping, products, workouts, workout-fuel, hydration, weight, goals, overrides, training-phases, goal-templates, energy/summary, race/raceprep, recovery-metrics, fitness-metrics, devices/health-vitals/achievements, gear, personal-records, daily-context, garmin login/scheduling/reconcile/library/activity/backfill): move its typed arg structs into registry entries with `Build` funcs + correct tiers; delete its bespoke `registerXxxTools` + handlers; migrate its unit tests to registry-driven assertions (Build→HTTPCall shape, dispatch result).
- [ ] 3.2 After each group: `go test ./internal/mcpserver/... ./internal/agenttools/...` green; `mcp_integration_test.go` announced-surface + per-group smoke green.

## 4. Retire the drift machinery

- [ ] 4.1 Replace `AnnouncedToolNames` with a registry-derived function (or delete it) and update `mcp_integration_test.go` to assert announced surface == registry names (DD6).
- [ ] 4.2 Delete `mcpserver/drift_test.go` + the `chatBespokeTools` allowlist (now redundant — both surfaces are one registry).
- [ ] 4.3 Remove `mcpserver`'s `effectiveIdempotencyKey`/`deriveIdempotencyKey`/`canonicalJSON`/`stripIdempotencyKey` (superseded by `agenttools`).

## 5. Cross-cutting

- [ ] 5.1 Full `task test` green (`internal/mcpserver`, `internal/agenttools`, `internal/chat`).
- [ ] 5.2 `task vet`; `task swag` (no REST/handler change expected — verify the spec didn't drift).
- [ ] 5.3 Confirm `internal/chat`'s exposed surface is unchanged (curated subset still filtered correctly after DD1).
