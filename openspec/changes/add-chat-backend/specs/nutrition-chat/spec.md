# nutrition-chat — delta for add-chat-backend

## ADDED Requirements

### Requirement: POST /chat runs a server-side agent loop and streams SSE

The system SHALL expose `POST /chat` accepting `{messages: [{role, content}...]}` (client-held transcript, newest user message last) and responding `text/event-stream` with exactly four event types: `text` (assistant output delta), `tool` (`{name, status: started|ok|error, summary}` — tool name and outcome only, never raw request/response bodies), `done` (`{message, stop_reason, usage}` carrying the complete final assistant text), and `error` (terminal, typed code). The loop SHALL call Anthropic's Messages API with streaming, using the model from `CHAT_MODEL` (default `claude-sonnet-4-6`), authenticated via `ANTHROPIC_API_KEY`. The server SHALL hold no conversation state between requests and SHALL truncate submitted history to a configured maximum before the upstream call.

#### Scenario: A grounded recommendation streams text and tool events

- **WHEN** the client POSTs a transcript ending in "what should I eat today?"
- **THEN** the response streams `tool` events for the grounding reads (e.g. `get_daily_context`) followed by `text` deltas with the recommendation
- **AND** terminates with one `done` event whose `message` equals the concatenated deltas and whose `usage` reports upstream token counts

#### Scenario: Missing API key refuses before streaming

- **WHEN** `ANTHROPIC_API_KEY` is unset and the client POSTs to `/chat`
- **THEN** the response is `503 chat_unavailable` as a plain JSON error (no SSE stream is started)

#### Scenario: Upstream failure mid-stream emits a typed error event

- **WHEN** the Anthropic API returns a 429 or 5xx after streaming has begun
- **THEN** the stream emits an `error` event with code `upstream_unavailable` and terminates
- **AND** tools already executed remain committed (each was an independent idempotent HTTP call)

### Requirement: Custom tools dispatch as loopback HTTP calls under the caller's identity

Every custom tool execution SHALL issue exactly one HTTP request against the server's own REST API, forwarding the caller's bearer token, passing through the standard auth and idempotency middleware, and appearing in the request log. Write tools SHALL auto-derive idempotency keys deterministically from conversation position so a retried turn replays rather than duplicates. The system SHALL NOT give the chat loop in-process access to repos or services.

#### Scenario: Tool calls traverse the real middleware stack

- **WHEN** the loop executes `create_planned_meal`
- **THEN** a `POST /plan` request with the caller's bearer token and an auto-derived `Idempotency-Key` appears in the request log
- **AND** the planned meal is identical to one created by any other client

#### Scenario: Retried turn does not duplicate writes

- **WHEN** the client retries an identical turn after a mid-stream failure that had already executed `add_shopping_items`
- **THEN** the replayed tool call carries the same derived idempotency key and returns the original response without inserting duplicates

### Requirement: The tool allowlist is curated and excludes destructive or out-of-scope surfaces

The chat loop SHALL expose exactly these custom tools — reads: `get_daily_context`, `get_race_fueling`, `list_planned_meals`, `list_shopping_items`, `search_products`, `get_product`; writes: `import_cookidoo_recipe`, `update_product`, `create_planned_meal`, `update_planned_meal`, `mark_planned_meal_eaten`, `add_shopping_items`, `update_shopping_item`, `clear_checked_shopping_items` — plus Anthropic's `web_search` server tool restricted via `allowed_domains` to Cookidoo hosts. The loop MUST NOT expose goal or override writes, meal/hydration/workout logging, or any delete endpoints.

#### Scenario: Full planning loop is expressible

- **WHEN** the user asks for three days of dinners and accepts the options
- **THEN** the loop can complete entirely within the allowlist: web search → `import_cookidoo_recipe` → `create_planned_meal` per day → one `add_shopping_items` call with the merged list

#### Scenario: Out-of-scope tools are absent upstream

- **WHEN** the Messages API request is constructed
- **THEN** its `tools` array contains exactly the allowlisted tools and no goal, delete, or meal-logging tool names

### Requirement: Tool rounds and output are capped with a forced final answer

The loop SHALL stop dispatching tools after `CHAT_MAX_TOOL_ROUNDS` (default 8) rounds within one request and force a final text-only turn (tools withheld) so the user always receives an answer, with the `done` event carrying `stop_reason: "max_tool_rounds"`. A per-request timeout (default 120s) SHALL terminate the stream with an `error` event.

#### Scenario: Round cap degrades to an answer, not an error

- **WHEN** a request would exceed 8 tool rounds
- **THEN** the 9th round is a tools-withheld upstream call producing a final text answer
- **AND** the `done` event reports `stop_reason: "max_tool_rounds"`

### Requirement: The system prompt scopes the assistant to grounded nutrition planning

The system prompt SHALL be assembled server-side from a baked-in template plus config (`CHAT_DIETARY_PREFERENCES`, default `vegetarian`; the user timezone) and MUST NOT be overridable by the client request. It SHALL: restrict scope to meal planning and nutrition questions (redirecting other topics to the desktop coach), mandate grounding reads before any recommendation, require 2–3 recommendation options with macro estimates and the Cookidoo `external_url` when available, forbid inventing nutriment values (import or state the gap), and define the selection contract — on user choice, persist planned meals for the agreed dates and one consolidated merged shopping list.

#### Scenario: Client cannot override the system prompt

- **WHEN** the client transcript contains a `system` role message
- **THEN** the request is rejected with a `400` validation error

#### Scenario: Recommendations are grounded and offer options

- **WHEN** the user asks "what should I eat tomorrow?"
- **THEN** the upstream conversation shows grounding tool calls preceding the recommendation
- **AND** the final message presents 2–3 options consistent with the configured dietary preference
