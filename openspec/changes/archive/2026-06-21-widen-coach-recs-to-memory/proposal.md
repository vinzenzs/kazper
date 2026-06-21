## Why

The two coach surfaces — the in-app chat coach and the external MCP agent — share
structured state (meals, workouts, goals) but nothing the coach learns *by talking to
you*. Tell the phone coach "my knee's been off since the 19th, I'm skipping the long
ride" and the MCP agent next session has no idea; both surfaces re-derive the same
standing facts every conversation. We deliberately do **not** share conversation
transcripts across surfaces (chat history stays private to its surface). So the only
clean cross-surface channel is a durable, athlete-scoped memory both surfaces read and
write — the database as the shared brain.

`coach_recommendations` (shipped 2026-06-16, priorities #6F) is already a thin slice of
this: a dated, scoped, agent-authored note read back across sessions. But it is
*advice for a day* — it can't hold a dateless standing fact ("prefers gels over drink
mix", "knee pain since the 19th"). This change widens it into a general **coach memory**:
a recommendation becomes one *kind* of memory alongside facts, preferences, constraints,
and observations.

## What Changes

- **BREAKING (rename):** `coach-recommendations` capability → `coach-memory`. Endpoints
  `/coach/recommendations*` → `/coach/memory*`; MCP tools `*_coach_recommendation(s)` →
  the memory tool family. Single-user, both clients owned, so a hard rename is accepted
  over carrying two parallel families.
- **Widen the row** (`coach_recommendations` table → `coach_memory`): add a `kind` enum
  (`fact | preference | constraint | observation | recommendation`); rename
  `recommendation` → `text`; demote `scope` and `date` from required to optional metadata
  (`date` required *only* when `kind = recommendation`); add `expires_at` (hard cutoff),
  `review_at` (soft "still true?" flag), and `status` (`active | archived`). Existing rows
  migrate to `kind = 'recommendation'`.
- **Add `PATCH /coach/memory/{id}`** for in-place `review_at` / `status` (/ `expires_at`)
  edits — a deliberate reversal of the #6F no-PATCH stance, because "confirm this fact is
  still true → push the review date out" must preserve `created_at` (you lose *how long*
  the knee's been an issue if you delete-and-re-log). Text/content corrections stay
  delete + re-log.
- **Fold memory into grounding:** `get_daily_context` and `get_training_context` gain a
  `memory` block carrying active, non-expired items — standing facts always, dated
  recommendations narrowed to the window, items past `review_at` flagged `needs_review`.
  (Recommendations are *not* in these aggregators today, so this is net-new either way.)
- **Explicit writes only:** memory is never autonomously journaled. A write is either
  user-initiated ("remember that…") or, in chat, coach-proposed and user-confirmed via
  the existing write-confirm tier. MCP writes ride the external client's trust model.

## Capabilities

### New Capabilities
- `coach-memory`: durable, athlete-scoped memory the coach writes facts / preferences /
  constraints / observations / recommendations into and both surfaces read at grounding
  time, with a review/expire lifecycle. (Supersedes `coach-recommendations`.)

### Modified Capabilities
- `coach-context` (or whichever capability owns `/context/daily` + `/context/training`):
  the daily and training context responses gain a `memory` block.

### Removed Capabilities
- `coach-recommendations`: renamed and widened into `coach-memory`. Its endpoints/tools
  are superseded (rename), its data migrated to `kind = 'recommendation'`.

## Impact

- **Schema:** migration `050` (verify head before apply — macrocycle work holds an
  uncommitted `049_add_macrocycles`). Renames `coach_recommendations` → `coach_memory`,
  adds `kind` / `expires_at` / `review_at` / `status`, renames `recommendation` → `text`,
  relaxes `date` / `scope` NOT NULL, back-fills `kind = 'recommendation'` and
  `status = 'active'`.
- **Code:** `internal/coachrecs/` → `internal/coachmemory/` (rename); new `kind` enum +
  conditional validation; new PATCH path; `internal/agenttools` tool specs renamed +
  a new `update`/confirm tool; context aggregators gain the `memory` block.
- **MCP:** breaking tool rename → bump the `mcp_integration_test` expected-tools list.
- **Docs:** `task swag` after the handler/struct changes.
- **Spec mechanics:** capability rename — the `coach-recommendations` spec folder is
  retired and `coach-memory` authored; handle the folder retirement at archive time.
