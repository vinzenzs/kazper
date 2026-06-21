## Context

`coach_recommendations` (migration `047`, `internal/coachrecs/`) is a thin
append-and-read primitive: `{id, date NOT NULL, scope NOT NULL (5-enum),
recommendation NOT NULL, reason NULL, timestamps}`, mirrored as four MCP tools
(`log_/list_/get_/delete_coach_recommendation(s)`), with corrections done by delete +
re-log (no PATCH). It is *not* folded into the `/context/daily` or `/context/training`
aggregators today — it has its own `list` read.

This change generalizes that primitive into **coach memory**: the cross-surface channel
through which the chat coach and the MCP agent share what they've learned about the
athlete, *without* sharing conversation transcripts. The guiding principle, confirmed in
exploration: **the database is the shared brain; conversations stay private to their
surface.**

## Goals / Non-Goals

**Goals:**
- One durable, athlete-scoped memory store both surfaces read at grounding time and write
  to explicitly.
- A recommendation is one *kind* of memory; standing facts/preferences/constraints/
  observations are the new dateless kinds.
- A review/expire lifecycle so standing facts age gracefully (confirm → push out, or
  expire → drop from grounding) instead of silently rotting.
- Memory rides every grounding call (folded into the context aggregators), so neither
  surface has to remember to ask.

**Non-Goals:**
- **No shared conversation transcript.** MCP will not see chat sessions and vice-versa.
  Memory is the *only* cross-surface channel — by design.
- **No autonomous journaling.** The coach never silently decides to remember; writes are
  user-initiated or user-confirmed.
- **No server-side synthesis.** Carried verbatim from #6F: the store records primitives,
  never generates/ranks/interprets, and writing memory never mutates an enforced target.
- No semantic search / embeddings retrieval — windowed + kind-filtered list is enough at
  single-user scale.

## Decisions

### 1. Widen-and-rename, one table with a `kind` discriminator

`coach_recommendations` → `coach_memory`:

| column        | change                                                                 |
|---------------|------------------------------------------------------------------------|
| `kind`        | **new** — `fact \| preference \| constraint \| observation \| recommendation` |
| `text`        | rename of `recommendation` (still `NOT NULL`, non-empty)               |
| `reason`      | unchanged (`NULL`)                                                     |
| `scope`       | `NOT NULL` → **nullable** optional tag (same 5-value set when present) |
| `date`        | `NOT NULL` → **nullable**; required only when `kind = 'recommendation'` |
| `expires_at`  | **new** `DATE NULL` — hard cutoff; filtered out of grounding after     |
| `review_at`   | **new** `DATE NULL` — soft; still surfaced, flagged `needs_review`     |
| `status`      | **new** — `active \| archived`, default `active`                       |

Existing rows back-fill to `kind = 'recommendation'`, `status = 'active'`. Validation
becomes kind-conditional: `date` required iff `kind = 'recommendation'`. This conditional
requiredness is a mild smell but is the honest cost of unifying advice-for-a-day with
standing facts.

**Alternative considered — keep `coach_recommendations`, add a sibling `coach_memory`.**
Rejected: two near-identical tables and two read surfaces, and the aggregator would merge
both anyway. The user chose the rename, accepting the blast radius.

### 2. Add PATCH for review/confirm; keep content edits as delete + re-log

#6F deliberately had no PATCH. The review mechanism forces a narrow reversal: confirming
"this constraint is still true" pushes `review_at` (and may set `status`) **in place**,
because delete-and-re-log resets `created_at` and you lose how long the fact has held —
which is the point of remembering it. So:

- `PATCH /coach/memory/{id}` accepts `review_at`, `status`, `expires_at` only (the
  lifecycle fields). It does **not** edit `text` / `kind` / `scope` / `date` — a content
  correction is still delete + re-log, preserving the #6F "no in-place rewrite of the
  authored note" stance.

### 3. Fold into the context aggregators (read once, ground everywhere)

`/context/daily` and `/context/training` gain a `memory` block:
- `status = 'active'` AND (`expires_at IS NULL OR expires_at >= today`),
- standing kinds (fact/preference/constraint/observation) always included,
- `recommendation` kind narrowed to the aggregator's date window,
- items with `review_at <= today` carry a `needs_review: true` flag so the coach can ask
  "is this still true?".

Because both surfaces already call these aggregators to ground, memory arrives for free —
no extra agent step, and the MCP agent picks up what the phone coach was told.

### 4. Explicit writes only

- **Chat:** a memory write is `TierWriteConfirm` — the coach proposes "want me to
  remember: knee pain since the 19th?" and the user confirms (the existing write-confirm
  flow). Never written from an inferred aside.
- **MCP:** no tier; "explicit" means the external agent calls the write tool because the
  user said "remember…". Trust model is the client's, same as every other MCP write.
- Net rule: **never autonomous** — user-initiated or user-confirmed, always.

### 5. Tool + endpoint rename (breaking, accepted)

`/coach/recommendations*` → `/coach/memory*`; MCP `log_/list_/get_/delete_coach_recommendation(s)`
→ a memory family (`remember` or `log_coach_memory` for write, `list_/get_/delete_coach_memory`,
plus `update_coach_memory` for the PATCH/confirm). Bumps the `mcp_integration_test`
expected-tools list. Single-user with both clients owned makes the hard rename cheaper
than parallel deprecation.

## Risks / Trade-offs

- **[Reshaping a 5-day-old capability]** → It hasn't accreted dependents; the rename +
  data back-fill is mechanical and the migration is reversible.
- **[Kind-conditional `date` validation is a smell]** → Accepted; it's the price of one
  table. The alternative (two tables) was explicitly rejected.
- **[Memory pollution / unbounded grounding block]** → Mitigated by explicit-write-only
  (no silent accumulation) + the expire/review lifecycle + `status='archived'` filtering.
  If the active set still grows large, cap/oder the folded block by recency or add a
  per-kind cap — revisit on real use.
- **[Breaking MCP rename]** → One-time; bump the expected-tools test and re-point the
  vault/desktop client. No external consumers.

## Migration Plan

1. `task migrate:new NAME=widen_coach_recs_to_memory` — verify head first (macrocycle
   holds an uncommitted `049`); this is `050`.
2. `up.sql`: `ALTER TABLE coach_recommendations RENAME TO coach_memory;`
   `RENAME COLUMN recommendation TO text;` add `kind`/`expires_at`/`review_at`/`status`;
   back-fill `kind='recommendation'`, `status='active'`; relax `date`/`scope` NOT NULL;
   refresh the CHECK constraints + index name.
3. `down.sql`: reverse (drop new columns, re-tighten NOT NULL after confirming no NULLs,
   rename back). Note: down is lossy for rows added as non-recommendation kinds — document
   it.
4. Rename `internal/coachrecs/` → `internal/coachmemory/`; add `kind` enum + conditional
   validation + the PATCH path; rename handlers/routes; `task swag`.
5. Rename the agenttools specs + add `update_coach_memory`; bump `mcp_integration_test`.
6. Add the `memory` block to the two context aggregators + their tools.

## Open Questions

- **Write-tool name:** `remember` (evocative, matches "remember that…") vs
  `log_coach_memory` (consistent with the `log_*` family). Lean `remember` for the agent's
  sake; confirm at apply.
- **Does `archived` need to be visible at all,** or is it just "soft delete so the review
  trail survives"? If never read back, a hard delete might do — but keeping it preserves
  the "how long did we believe this" history. Tentatively keep.
- **Capability-rename spec mechanics:** retire the `coach-recommendations` spec folder and
  author `coach-memory` — confirm the archive step handles the folder move cleanly or do
  it by hand.
