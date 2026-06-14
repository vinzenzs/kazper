## Context

`internal/agenttools` (from `expand-chat-to-coach`) is the source of truth for the agent tool surface: `Spec{Name, Description, Schema (JSON-Schema string), Tier, Build(json.RawMessage) -> HTTPCall}`. Today only `internal/chat` consumes it (rendering Anthropic tool defs + dispatching via loopback). `internal/mcpserver` still defines its 123 tools by hand: each is a `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler(ctx, req, TypedArgs))` where the MCP SDK **reflects the input schema from the `TypedArgs` Go struct** (`jsonschema:"…"` tags), and the handler decodes typed args, calls `apiClient.{Get,Post,Patch,Delete}(path, query, body, idemKey)`, and maps the response via `toToolResult`. A name-level drift-guard (`mcpserver/drift_test.go` vs `AnnouncedToolNames`, minus a 4-entry chat-bespoke allowlist) is the only thing keeping the two surfaces aligned.

This change makes `internal/mcpserver` consume `agenttools`, so the surface is generated from one registry. The decisions referenced from `expand-chat-to-coach` design: **D4-amended** (this is the deferred full port), **D11** (aggregates are dual-surface), **D12** (idempotency derivation homed in `agenttools`).

Codebase constraints:
- `agenttools.HTTPCall{Method, Path, Query, Body}` maps cleanly onto `apiClient`'s verb methods; the only outlier is `log_meal_from_photo` (`PostMultipart`).
- The MCP tool surface is a hard external contract for the desktop coach — names, schemas, descriptions, idempotency, and error mapping MUST be byte-for-byte stable across this refactor.
- `agenttools` is pure (no transport/auth/IO); execution and headers stay in each consumer.

## Goals / Non-Goals

**Goals:**
- `internal/mcpserver` registers its entire tool surface by iterating `agenttools.Registry()`, via one generic handler — no per-tool handler functions.
- `agenttools` holds the full 123-tool union; `internal/chat` still exposes only its curated, tier-gated subset.
- Schemas are reflected from the existing typed arg structs (no 123 hand-rewritten JSON-Schema strings); reflected and string schemas coexist.
- Idempotency uses `agenttools.EffectiveIdempotencyKey` (D12); `mcpserver`'s private copy is removed.
- The drift-guard + hand-maintained `AnnouncedToolNames` are retired in favor of a registry-derived surface.
- Zero behavioral change to the announced MCP surface, proven by the integration test.

**Non-Goals:**
- No change to tool names, schemas, descriptions, or behavior (pure restructuring).
- No change to `internal/chat`'s curated surface or the confirmation protocol (that's `expand-chat-to-coach`).
- No new REST endpoints (the dual-surface aggregates of D11 land with `expand-chat-to-coach` phase 3, not here).
- No move of `agenttools` off "pure registry" — transport/auth/headers stay in consumers.

## Decisions

### DD1 — `agenttools` becomes the full union; a visibility marker selects the chat subset
`Registry()` grows from ~14 to the full 123 tools. `Spec` gains a visibility marker (e.g. `ChatExposed bool` or `Surfaces []Surface{Chat, MCP}`). `internal/mcpserver` iterates the whole registry (ignoring `Tier`, as today); `internal/chat` filters to `ChatExposed` entries and applies `Tier`. This keeps chat's surface deliberately small (D10) while the registry is the union. The 4 chat-bespoke aggregate tools (`get_daily_context`, etc.) are entries with `ChatExposed=true`; under D11 their MCP equivalents also become registry entries, shrinking the bespoke gap.

### DD2 — Schemas are reflected from typed arg structs (the crux)
Hand-authoring 123 JSON-Schema strings is the highest-risk part of the port (a schema that differs from the SDK's reflected one breaks tool-calling). Avoid it: `Spec` carries an **optional typed schema source** (`SchemaType any` — a zero value of the arg struct) alongside the existing `Schema string`. `agenttools` reflects `SchemaType` to a JSON-Schema string using the **same reflection library the MCP SDK uses** (so the output is identical to today's announced schema). Entries provide *either* a hand-written `Schema` (the original chat tools) *or* a `SchemaType` (the ported mcp tools); a helper resolves both to the JSON string each consumer needs. This preserves the `jsonschema:"…"` tag ergonomics and guarantees schema parity. The reflection lib choice + a golden test (reflected schema == previously-announced schema, per tool) de-risk it.

### DD3 — One generic MCP handler over the registry
`registerXxxTools(server, c)` functions are replaced by a single loop:
```
for _, s := range agenttools.Registry() {
    mcp.AddRawTool(server, s.Name, s.Description, resolveSchema(s), func(ctx, raw json.RawMessage) (*mcp.CallToolResult, error) {
        return dispatchMCP(ctx, c, s, raw)   // Build → apiClient → toToolResult
    })
}
```
`dispatchMCP` runs `s.Build(raw)` → `HTTPCall`, executes it via an `apiClient` adapter that switches on `Method` (`Get/Post/Patch/Put/Delete`), attaches the idempotency key for write tiers (DD4), and maps the result through the existing `toToolResult`. This is exactly what each bespoke handler did, written once. Requires the SDK to support **raw-schema tool registration** (registering with a JSON-Schema string + a `json.RawMessage` handler) rather than only reflection-typed `AddTool`; confirm the SDK exposes this (most do via a lower-level `Tool.InputSchema` + untyped handler). If it does not, the fallback is a tiny generated shim per tool — still mechanical, no logic.

### DD4 — Idempotency via the shared `agenttools` derivation
Write tools call `agenttools.EffectiveIdempotencyKey(explicitKey, name, raw)` (D12): an agent-supplied `idempotency_key` wins; otherwise the shared content-derived key (strips `idempotency_key`, `UseNumber`). `mcpserver`'s `effectiveIdempotencyKey`/`canonicalJSON`/`stripIdempotencyKey` are deleted. The explicit-key passthrough (today read off the typed args' `IdempotencyKey` field) is read generically from the raw input's `idempotency_key` field. Hard dependency: `expand-chat-to-coach` task 1.5 must have landed these functions in `agenttools`.

### DD5 — Multipart escape hatch for `log_meal_from_photo`
`HTTPCall` is JSON/request-shaped; `log_meal_from_photo` posts multipart. Keep it on a documented bespoke registration outside the generic loop (one tool), OR extend `HTTPCall` with an optional `ContentType`/`Multipart` marker the adapter honors. Lean toward the **escape hatch** initially (one explicitly-listed exception) to keep `HTTPCall` simple; revisit if more multipart tools appear. Either way it stays a registry entry for discovery/naming; only its execution adapter differs.

### DD6 — Retire the drift-guard; derive the announced surface
Once both surfaces come from `agenttools.Registry()`, `drift_test.go`'s name-subset check is meaningless (they're the same list) and `AnnouncedToolNames` is redundant. Replace: `AnnouncedToolNames` becomes a function over the registry (or is deleted, with `mcp_integration_test.go` asserting the announced surface == registry names). The chat-bespoke allowlist disappears. `expand-chat-to-coach`'s `nutrition-chat` "one registry, no drift" requirement is now satisfied by construction rather than by a guard.

### DD7 — Incremental, group-by-group rollout
Port one tool group at a time (meals, workouts, garmin, …): move its typed structs into registry entries with `Build` funcs, delete its bespoke handlers, migrate its unit tests to registry-driven assertions, keep `mcp_integration_test.go` green after each group. The generic handler and the bespoke handlers coexist during the migration (the loop skips already-ported names, or the registry is split until complete). This keeps each PR reviewable and the desktop coach working throughout.

## Risks / Trade-offs

- **[Schema drift breaks tool-calling]** A reflected schema that differs from today's announced schema (even whitespace/ordering the agent tolerates, but `required`/types it does not) breaks the desktop coach. Mitigation: DD2 reuses the SDK's own reflection lib; a per-tool golden test asserts reflected-schema == previously-announced-schema before deleting the bespoke registration.
- **[Idempotency key changes for some inputs]** Adopting the shared derivation (`UseNumber`, strip) can change derived keys vs `mcpserver`'s old algorithm for inputs like `45.0`. Mitigation: pre-release, keys matter only within a replay window; document it; the explicit-key path is unchanged. Acceptable.
- **[SDK lacks raw-schema registration]** DD3 assumes an untyped/raw registration entry point. Mitigation: if absent, generate a one-line typed shim per tool (mechanical, no logic) — the registry still owns name/schema/build; only the SDK glue is per-tool.
- **[201-test churn]** Migrating the tests is the bulk of the work and risk of regressions. Mitigation: DD7 group-by-group; the integration test (announced surface + a smoke call per group) is the safety net held green throughout.
- **[Big registry file]** `agenttools.Registry()` at 123 entries is large. Mitigation: split into per-domain files (`registry_meals.go`, …) returning slices a top-level `Registry()` concatenates — mirrors mcpserver's current file split.
- **[Multipart special-case]** DD5 leaves one tool outside the uniform path. Mitigation: explicitly listed and tested; a clear extension point if multipart spreads.

## Migration Plan

1. Land `agenttools` additions: visibility marker (DD1), typed schema source + reflection + golden test (DD2), confirm/raw-schema registration entry point (DD3). Depends on `expand-chat-to-coach` 1.5 for idempotency (DD4).
2. Add the generic `dispatchMCP` + `apiClient` `HTTPCall` adapter; wire the registry loop alongside existing registrations.
3. Port tool groups one at a time (DD7): entries + `Build` + test migration; delete bespoke handlers as each group goes green.
4. Retire `drift_test.go` + `AnnouncedToolNames` → registry-derived (DD6); update `mcp_integration_test.go`.
5. Remove `mcpserver`'s idempotency helpers; final `task test` + `task vet` + `task swag` (no handler/REST change expected, but verify).
