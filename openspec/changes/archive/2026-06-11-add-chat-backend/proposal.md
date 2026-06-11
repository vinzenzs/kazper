# Proposal: add-chat-backend

## Why

The companion app needs a nutrition-focused chat ("what should I eat today / the next 3 days?") but the coaching agent today runs on the desktop over stdio MCP — unreachable from a phone. The backend already holds `ANTHROPIC_API_KEY` (vision precedent) and persists everything the desktop agent decides (goals, overrides, phases, race fueling plan), so a server-side agent loop can ground itself from the same database with no agent-to-agent channel. A spike confirmed the missing external piece works: Anthropic server-side web search finds Cookidoo recipes and anonymous fetches yield full recipe JSON-LD.

## What Changes

- New `nutrition-chat` capability: `POST /chat` runs a server-side Anthropic Messages API agent loop and streams the result to the client via SSE (text deltas, tool-activity events, final message, errors).
- The server is stateless per conversation: the client sends the transcript each turn; no chat tables.
- The loop gets a curated tool set — reads (daily context, race/plan/goals data, product search) and scoped writes (Cookidoo import, planned meals, shopping items). No goal edits, no deletes, no generic meal logging. Each custom tool dispatches as exactly one loopback HTTP call against the server's own REST API reusing the caller's bearer token, preserving auth and idempotency middleware. Anthropic's `web_search` server tool is enabled, restricted to Cookidoo domains.
- A system prompt scopes the assistant to nutrition planning, encodes dietary preferences from config (vegetarian), and instructs grounding via daily context + race data before recommending.
- Hard caps: max tool rounds per message, max output tokens, request timeout. Missing API key → `503 chat_unavailable` (mirrors `vision_unavailable`).

## Capabilities

### New Capabilities

- `nutrition-chat`: the streaming chat endpoint, its agent loop, tool allowlist, system-prompt contract, and configuration.

### Modified Capabilities

- `products`: adds `PATCH /products/{id}` (partial merge update of name/serving_size_g/nutriments_per_100g). Discovered during implementation — the `update_product` allowlisted tool had no endpoint to call, so this change supplies one. No DB schema change (uses existing columns).

Other consumed capabilities (`meal-plan`, `shopping-list`, `cookidoo-importer`, `daily-context`) are used as-is via their REST surfaces. The MCP server is untouched.

## Impact

- **DB**: none (stateless; the products PATCH reuses existing columns — no migration).
- **Code**: new `internal/chat` package (agent loop, Anthropic streaming client, tool registry + loopback dispatcher, SSE writer); `internal/products` gains a PATCH handler/service/repo method; `internal/config` additions (`CHAT_MODEL` default `claude-sonnet-4-6`, `CHAT_MAX_TOOL_ROUNDS`, `CHAT_MAX_HISTORY_MESSAGES`, `CHAT_REQUEST_TIMEOUT_SECONDS`, `CHAT_DIETARY_PREFERENCES`, reuses `ANTHROPIC_API_KEY`); `internal/httpserver` wiring.
- **Docs**: `task swag` (SSE endpoint annotated as text/event-stream).
- **Cost**: each message is a metered Anthropic call (plus web search); caps bound the worst case.
- **Sequencing**: depends on `add-recipe-ingredients`, `add-meal-plan`, `add-shopping-list` being implemented first; consumed by `add-companion-chat`.
