# Design: add-chat-backend

## Context

Three clients now: mobile app (fast logging), desktop agent (full coaching over MCP), and this — a constrained in-app chat for meal planning. The chat agent becomes the third client of the REST API: it must not get a privileged in-process path to repos, or the "every agent action is an audited HTTP call" property quietly dies. The vision capability established the Anthropic client conventions (env-keyed, typed `ErrAPIKeyMissing`, 503 when unconfigured, versioned User-Agent).

## Goals / Non-Goals

**Goals:**
- "What should I eat?" → grounded options → selection lands as planned meals + shopping items, all within one chat.
- Self-populating recipe library: web search → Cookidoo import mid-conversation.
- Bounded blast radius (curated tools) and bounded cost (round/token caps).
- Streaming UX on mobile.

**Non-Goals:**
- Not the full coach: no goal editing, no training analysis, no deletes. Deep coaching stays on the desktop.
- No server-side conversation persistence in v1 (no tables; revisit if desktop-reads-mobile-chats becomes wanted — see priorities.md §6F for the rationale-persistence sibling).
- No voice, no images in chat v1.
- No multi-conversation management server-side.

## Decisions

### D1: Server-side loop with loopback HTTP tool dispatch

The loop runs in the Gin process; each custom tool execution issues a real HTTP request to `localhost` (own listen address) with the caller's bearer token forwarded. Preserves: auth, idempotency middleware (loop auto-derives keys for write tools, reusing the MCP server's `effectiveIdempotencyKey` approach), request logging as the audit trail, and the 1:1 tool→endpoint discipline. Alternatives rejected: in-process service calls (bypasses middleware, second wiring of every capability), app-side loop (API key on device, Dart re-implementation), MCP-over-HTTP + connector (exposes the server publicly; another moving part).

### D2: Stateless server, client-held transcript

Request body: `{messages: [{role, content}...]}` (full history) + the new user message. The server replays it into the Messages API. Costs grow with conversation length → mitigated by a max-history cap (server truncates to the most recent N messages) and the app's natural short-conversation usage. Alternative — `chat_conversations` tables — deferred: chat transcripts are not nutrition primitives, and the desktop agent can already see every *consequence* (plans, shopping items, imported recipes).

### D3: Tool allowlist (v1)

Reads: `get_daily_context`, `get_race_fueling`, `list_planned_meals`, `list_shopping_items`, `search_products`, `get_product`. Writes: `import_cookidoo_recipe`, `update_product` (nutriment follow-up after import), `create_planned_meal`, `update_planned_meal`, `mark_planned_meal_eaten`, `add_shopping_items`, `update_shopping_item`, `clear_checked_shopping_items`. Server tool: `web_search` with `allowed_domains` restricted to Cookidoo hosts. Excluded deliberately: meal/hydration logging (the app's native flows own that), goal/override writes, all deletes. Rationale: an unsupervised LLM reachable from a phone keyboard gets the smallest tool set that completes the planning loop.

**`update_product` needed a new endpoint.** Implementation revealed the products capability had no PATCH — create/lookup/list/get/search/delete/import only. Rather than drop the tool, this change adds `PATCH /products/{id}` (a partial merge update of name/serving_size_g/nutriments_per_100g) so `update_product` has a target. It is captured as a products delta spec in this change. The endpoint is generic but its motivating caller is the chat agent filling in nutriments after a serving-size-less Cookidoo import.

### D4: SSE over POST, four event types

`POST /chat` responds `text/event-stream` with events: `text` (assistant delta), `tool` (`{name, status: started|ok|error, summary}` — name and outcome only, never raw bodies), `done` (`{message, stop_reason, usage}` — the complete final assistant text so clients can drop deltas), `error` (terminal, typed code). SSE chosen over WebSocket: one-directional stream per request, works through the existing middleware stack, trivially consumable from Dart.

### D5: Caps and failure posture

`CHAT_MAX_TOOL_ROUNDS` (default 8): when exceeded, the loop forces a final text-only turn (tools withheld) so the user always gets an answer, flagged `stop_reason: "max_tool_rounds"`. Per-request timeout (default 120s) → `error` event, no partial writes beyond tools already completed (each tool call is independently idempotent). Missing `ANTHROPIC_API_KEY` → `503 chat_unavailable` before any stream starts. Anthropic 429/5xx mid-stream → typed `error` event (`upstream_unavailable`); the client retries the whole turn (server-side write tools replay safely via idempotency keys derived from conversation position).

### D6: System prompt assembled from config, grounding mandated

Template baked into the binary; config injects `CHAT_DIETARY_PREFERENCES` (default `"vegetarian"`) and the user timezone. The prompt: scope (meal planning + nutrition Q&A only; redirect anything else to the desktop coach), mandatory grounding (call `get_daily_context` — and race fueling when the race is near — before recommending), recommendation contract (2–3 options with macros + Cookidoo link when available; never invent nutriments — import or say so), selection contract (on user choice: planned meals for the agreed dates + one consolidated, merged shopping list), and per-serving→per-100g conversion guidance for imports. Dietary preference lives in config, not a new profile capability — cheapest correct home today, additive to move later.

## Risks / Trade-offs

- [Loopback HTTP adds latency per tool call] → single-user localhost round trips are sub-millisecond against LLM-turn latencies; the audit/middleware win dominates.
- [Stateless transcripts cost tokens on long chats] → history cap + planning conversations are naturally short; persistence is an additive later change.
- [Web search results beyond Cookidoo could leak into context] → `allowed_domains` restriction + prompt instruction to only import Cookidoo URLs; the import endpoint independently rejects non-Cookidoo hosts (defense in depth, already specced).
- [Cost runaway from agentic enthusiasm] → round cap, token cap, and usage echoed in the `done` event so the app can display per-conversation spend.
- [Prompt injection via web search snippets] → write tools are scoped to plan/shopping/import only; worst case is junk plan entries, all visible and reversible in-app.

## Migration Plan

No DB changes. Ship behind the existing key: unset `ANTHROPIC_API_KEY` keeps the endpoint returning 503. Rollback = binary swap.

## Open Questions

- ~~Whether `get_race_fueling` should be one composite read or reuse two existing endpoints~~ — **resolved:** one tool with an optional `race_id`. No `race_id` → `GET /races` (discover races + dates); with `race_id` → `GET /races/{id}/fueling-plan`. Exactly one HTTP call per invocation, and no separate `list_races` tool needed in the allowlist.
- Exact Cookidoo `allowed_domains` list (`.de` spike-verified; the rest assumed): de, com, at, ch, co.uk, fr, es, it, nl, be, com.au, pl.
