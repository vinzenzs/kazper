## Context

The coach (LLM agent over MCP, and the in-app Kazper persona) synthesizes dated, scoped advice every session — carb targets, session-fueling notes, recovery nudges — but that output is never stored, so cross-session continuity ("what did I advise, did it hold?") is impossible. Priorities #6F proposes a tiny `coach_logs(date, recommendation, reason, scope)` primitive and explicitly flags the tension: recommendations *are* synthesis, and the project's discipline is "the agent synthesizes, the API records primitives." The resolution this design commits to: a recommendation, once written, is a **primitive record** (immutable text the agent authored) — storing it is recording, not synthesizing. The capability follows the established per-package shape (`types`/`repo`/`service`/`handlers` + tests, registered in `httpserver`), mirrors the `meals`/`bodyweight` precedent for a dated log with windowed reads and the standard `tz` handling, and rides the existing idempotency middleware on its POST.

## Goals / Non-Goals

**Goals:**
- Persist agent-authored recommendations as a dated, scoped log; read them back over a window for continuity.
- Stay a thin primitive: store/retrieve/delete only; zero server-side generation, ranking, or interpretation.
- Mirror 1:1 to MCP so the coach writes-as-it-advises and grounds on history next session.

**Non-Goals:**
- No server-side recommendation engine, scoring, or "best recommendation" logic.
- No mutation of nutrition goals/overrides — a recommendation is a note, not an enforced target; if the agent wants the number enforced it still calls the goals/override endpoints separately.
- No edit/PATCH of a recorded recommendation (a log is append-only; corrections are delete + re-log).
- No threading/links to specific meals/workouts in v1 (a `scope` enum is enough; entity links are a later follow-up if usage demands).

## Decisions

### D1: A new capability + table, not notes on goal-overrides
A standalone `coach_recommendations` table (own `internal/coachrecs/` package) rather than stuffing rationale into a `nutrition_goals_override.notes` field. Recommendations are cross-cutting (fueling *and* training *and* recovery), not all date-overrides exist for every recommendation, and overloading overrides would couple a free-text log to an enforced numeric target — exactly the synthesis/primitive blur to avoid. A dedicated log keeps the primitive clean and independently queryable.

### D2: Schema — a minimal dated log
`coach_recommendations(id uuid pk, date DATE NOT NULL, scope TEXT NOT NULL, recommendation TEXT NOT NULL, reason TEXT NULL, created_at, updated_at)`. `date` is the day the advice applies to (the agent supplies it; defaults are not server-invented). `recommendation` is required and length-bounded; `reason` optional. No FK to goals/workouts (D-non-goal). One additive migration; verify the on-disk head before numbering (currently `046`, so likely `047`).

### D3: `scope` is a small validated set
`fueling | training | recovery | race | general`, validated in the service with a sentinel error mapping 1:1 to an API code (`scope_invalid`), matching the `sport_invalid`/`status_invalid` convention. A closed set keeps the list/filter useful and the agent honest; "general" is the catch-all so nothing is unclassifiable. Stored as TEXT with a CHECK constraint mirroring the Go validation.

### D4: CRUD surface = create / list-window / get / delete
`POST /coach/recommendations`, `GET /coach/recommendations?from=&to=&tz=&scope=`, `GET /coach/recommendations/{id}`, `DELETE /coach/recommendations/{id}`. List is windowed on `date` (inclusive local dates, standard `tz` resolution like `bodyweight`/`summary`), optional `scope` filter, ordered `date DESC, created_at DESC` (newest advice first). No PATCH (D-non-goal: append-only log). POST honors the idempotency middleware; PUT is not used so the PUT-rejects-idempotency rule is irrelevant.

### D5: MCP mirror, four tools
`log_coach_recommendation` (TierWriteAuto, auto-derived idempotency key), `list_coach_recommendations` / `get_coach_recommendation` (TierRead), `delete_coach_recommendation` (TierWriteAuto), each issuing exactly one HTTP call and forwarding the body verbatim. Bump the `mcp_integration_test.go` expected-tools list by four and regenerate the announced-schema golden (the documented `goldengen` step, as with prior new tools).

### D6: Rounding / response boundary
No numeric fields beyond ids/dates, so no `numfmt` rounding applies; responses serialize the stored text + ISO date directly. `date` is returned as `YYYY-MM-DD`.

## Risks / Trade-offs

- **[Principle blur — "is storing synthesis still synthesis?"]** The #6F caveat. → Mitigation: the API only ever round-trips agent-authored text; it computes nothing. This is the same shape as `meals` (stores a user-authored intake) — recording an output is not producing it. Documented in the requirement so the boundary stays explicit.
- **[Free-text recommendation could drift into a structured-target backdoor]** An agent might encode "220g carbs" as text and expect enforcement. → Mitigation: explicitly non-enforcing; the design and tool descriptions state a recommendation is a note, and the agent must still write a goal override to change an enforced number. No code reads `recommendation` to alter behavior.
- **[Scope enum too coarse]** Five buckets may not fit every advice. → `general` absorbs the long tail; widening the enum later is an additive change. Starting closed beats starting open for list/filter utility.
- **[Unbounded growth]** A daily log accumulates. → It's single-user and tiny (text rows); windowed reads keep payloads bounded. No retention policy in v1; revisit if volume ever matters.

## Migration Plan

One additive migration `NNN_add_coach_recommendations` (`.up`/`.down`), embedded in the binary; verify the head (`046` on disk now → likely `047`, but re-check per the append-only rule). No backfill — the log starts empty. Deploy ships endpoints + tools; `task swag` regenerates docs, `goldengen` regenerates the MCP baseline. Rollback = down-migration drops the table; nothing else references it.

## Open Questions

- Should `list` also support a `date`-less "most recent N" mode for the agent's quick "what did I last advise on fueling?" grounding, or is the window enough? Lean: window is enough for v1; add `limit`/`latest` only if the agent's grounding calls prove awkward.
- Optional `entity` links (a recommendation tied to a specific workout or planned meal) — deferred; the `scope` enum covers the coarse case and links add join complexity better justified by real usage.
