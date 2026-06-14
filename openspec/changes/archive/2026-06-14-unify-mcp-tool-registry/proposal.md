## Why

`expand-chat-to-coach` introduced `internal/agenttools` as the single source of truth for the agent tool surface, but only `internal/chat` consumes it; `internal/mcpserver` still hand-maintains its own 123 tool registrations (typed arg structs + bespoke handlers + 201 unit tests). The two surfaces are kept honest only by a name-level drift-guard test — the deferred "full unification" from that change's design (D4-amended). This change closes the loop: the MCP server's tool surface becomes **generated from the shared registry** rather than hand-written, so chat and the desktop coach are the same tools by construction, not by a guard that can only check names.

## What Changes

- **`internal/mcpserver` consumes `internal/agenttools`.** The ~123 bespoke `mcp.AddTool(...)` registrations + per-tool handlers collapse into one **generic handler** that decodes the SDK args to `json.RawMessage`, runs `Spec.Build` to get the single `HTTPCall`, executes it via `apiClient`, and returns the body verbatim — exactly what the per-tool handlers do today, once each.
- **`agenttools` grows to the full surface.** The registry expands from the ~14 curated chat tools to the full 123-tool union. `Spec` gains a surface/visibility marker so `internal/chat` continues to expose only its curated, tier-gated subset while `internal/mcpserver` exposes all of them (and ignores `Tier`, as today).
- **Schemas are reflected, not hand-rewritten.** To avoid hand-authoring 123 JSON-Schema strings (and to keep the existing `jsonschema:"…"` struct-tag ergonomics), `Spec` may carry a typed Go arg struct as its schema source; `agenttools` reflects it to the JSON Schema both surfaces need. The existing hand-written string schemas (the 14 chat tools) and reflected struct schemas coexist.
- **Idempotency derivation unifies (D12).** MCP write tools adopt `agenttools.EffectiveIdempotencyKey` (explicit agent-supplied key wins; otherwise the shared content-derived key), retiring `internal/mcpserver`'s private copy.
- **The drift-guard is superseded.** `mcpserver/drift_test.go` + the hand-maintained `AnnouncedToolNames` list become obsolete once the surface is generated from one registry; the announced surface and the integration test's expected-tools list derive from `agenttools.Registry()` instead.
- The MCP tool surface is **behaviorally unchanged** — same tool names, schemas, descriptions, idempotency semantics, and error mapping. This is an internal restructuring, not a surface change.

## Capabilities

### New Capabilities

_None — `agenttools` is an internal package, not a behavioral capability._

### Modified Capabilities

- `mcp-server`: one new requirement — the MCP tool surface SHALL be generated from the shared `agenttools` registry (single source of truth), remaining behaviorally identical (names, schemas, idempotency), with the announced surface derived from the registry rather than a hand-maintained list. No existing tool requirement changes behavior.

## Impact

- **`internal/agenttools`**: registry expands to the full 123-tool union; `Spec` gains a visibility marker and an optional typed schema source + reflection; `DeriveIdempotencyKey`/`EffectiveIdempotencyKey` already homed here by `expand-chat-to-coach` task 1.5 (hard dependency).
- **`internal/mcpserver`**: ~25 `tools_*.go` registration files collapse into a generic registration loop over the registry; the typed arg structs are retained as schema sources but their bespoke handlers are deleted; `effectiveIdempotencyKey`/`canonicalJSON` removed in favor of the `agenttools` versions; `toolnames.go` + `drift_test.go` retired/derived; `apiClient` gains a thin adapter to execute an `agenttools.HTTPCall`.
- **Tests**: the 201 mcpserver unit tests migrate from per-handler assertions to registry-driven assertions (build-the-HTTPCall + dispatch shape); `mcp_integration_test.go`'s expected-tools list derives from the registry. Net test count likely drops.
- **Edge — multipart**: `log_meal_from_photo` uses `PostMultipart`; `HTTPCall` is request/JSON-shaped. Either extend `HTTPCall` with a content-type/multipart marker or keep this one tool on a documented bespoke escape hatch (decided in design).
- **Sequencing**: depends on `expand-chat-to-coach` phase 1 (agenttools exists) + task 1.5 (shared idempotency). Ported incrementally one tool group at a time behind the generic handler, keeping `mcp_integration_test.go` green throughout.
