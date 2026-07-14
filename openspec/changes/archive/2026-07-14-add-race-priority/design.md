# Design — add-race-priority

## Context

The race calendar (`race-fueling-plan`, `internal/races/`) persists races
`{name, race_date, race_type?, location?, notes?}` with ordered legs. The
macrocycle capability separately anchors a season to its goal race via
`macrocycles.race_id` — "the A-race the season peaks for" — but that models only
one race per season and says nothing about the rest of the calendar.
TrainingPeaks-style coaching triages every race A/B/C, and the coach agent
currently has nowhere durable to record that. This change adds the smallest
possible field: an optional `priority` enum on the race row, advisory metadata
for the agent.

Relevant precedents in this repo:

- `workouts.training_focus` (migration `048`): an optional closed-set TEXT enum,
  nullable with no DEFAULT and no backfill ("an unclassified session is a valid
  state, not a data-quality defect"), `CHECK (… IN (…))` kept in sync with a
  service-layer validity switch, tri-state on PATCH via the empty-string
  sentinel (`""` → `ClearTrainingFocus: true`).
- `races` PATCH today: nil pointer = leave unchanged, applied via
  `CASE WHEN $set::boolean THEN $val ELSE col END` in `Repo.UpdateRace`. The
  existing optional scalars (`race_type`, `location`, `notes`) have **no clear
  path** — a nil pointer leaves them, and Go's JSON decoding collapses `null`
  and missing, so nothing can NULL them out.

## Goals / Non-Goals

**Goals**

- Persist `priority ∈ {A, B, C}` on a race; settable at create, settable and
  clearable on PATCH; returned on every read of the race row.
- Validate with the structured error shape (`400 race_priority_invalid`).
- `GET /races?priority=X` filter.
- MCP `create_race`/`update_race` schemas carry the field.

**Non-Goals**

- Taper automation or any plan-generation behavior driven by priority.
- Hard coupling to the macrocycle's A-race anchor (see D4).
- Retrofitting a clear path onto `race_type`/`location`/`notes` (pre-existing
  gap, orthogonal to this change).
- Priority in the public race feed, coach dashboard, or fueling-plan response.

## Decisions

### D1 — Nullable, no default, `omitempty`

`priority` is nullable with no DEFAULT and no backfill. Absence means "not
triaged", which is the honest state for every existing row — defaulting to `C`
would fabricate a triage decision nobody made. This mirrors `training_focus`
(migration `048`) exactly, down to the rationale. On the wire the field is
`*Priority` with `json:"priority,omitempty"`, so untriaged races serialize
byte-identically to today.

### D2 — Storage: TEXT + CHECK, strict uppercase `A|B|C`

`ALTER TABLE races ADD COLUMN priority TEXT CHECK (priority IN ('A','B','C'))`.
NULL passes the CHECK. The closed set lives twice — in the CHECK and in a typed
`Priority` enum with a `valid()` switch in `types.go` — the same dual-home
convention as `Discipline` and `TrainingFocus`. Values are strict (no
case-normalization): every closed set in this codebase (`swim|bike|run|…`,
`recovery|basic_endurance_1|…`) rejects rather than normalizes, and the MCP
jsonschema description spells out the exact values, so the agent won't guess
lowercase.

### D3 — PATCH tri-state via the empty-string sentinel

`{"priority":"A"}` sets, `{"priority":""}` clears, omitted leaves unchanged.
This follows the repo's canonical optional-enum PATCH convention
(`training_focus` on workouts; `race_id` on macrocycles; `workout_id` on
meals/hydration/workoutfuel) rather than the races package's own optional
scalars — those (`race_type` etc.) simply have no clear path, which is a gap,
not a convention worth extending. Plumbing: handler converts `""` into
`ClearPriority: true` on `UpdateInput` (nil + no clear = unchanged); the repo's
`UpdateRace` gains a set/clear-aware branch of the existing `CASE WHEN` pattern
(set flag true with a NULL value when clearing).

### D4 — Priority is advisory; no macrocycle consistency enforcement

The macrocycle may anchor race X while race X is marked `C`, `B`, or untriaged —
no error, no warning field, in either write direction. Rationale:

- **Single user + LLM coach.** Both signals feed the same agent, which reads
  the macrocycle and the race list together and can surface the disagreement
  conversationally ("your season anchors this race but it's marked C — which is
  it?"). A hard constraint buys nothing here.
- **PATCH ordering would get awkward.** Enforcing "anchored race must be A"
  means re-triaging a season requires a specific write order (clear the anchor,
  then downgrade the race, then anchor the new one) and makes
  `update_race`/`update_macrocycle` fail on states that are merely in
  transition. That's hostile to an agent doing sequential tool calls.
- **The anchor and the priority answer different questions.** `race_id` on the
  macrocycle is structural ("what this season peaks for"); `priority` is
  per-race triage across the whole calendar, including races outside any season.

**public-race-feed is unaffected** (confirmed): the in-flight change resolves
the goal race via `macrocycles.race_id` and projects only name, date, and a
countdown — it never reads or re-serializes the race row's optional metadata,
and this change does not touch the macrocycle surface it depends on.

### D5 — List filter is in scope: `GET /races?priority=A`

Included in v1. The list endpoint currently has no query params, so this sets
the style: optional param, omitted returns everything (unchanged behavior),
supplied value validated against the same closed set with the same
`400 race_priority_invalid`. It is cheap (single-user calendar, one WHERE
clause) and directly agent-useful. Filtering matches stored values exactly;
there is no `priority=none` selector for untriaged races in v1 (the agent can
list all and partition — add a selector only if that proves annoying).
`list_races` MCP args gain the optional `priority` filter to keep the 1:1
mirror.

### D6 — Error code and surface placement

New sentinel `ErrPriorityInvalid = errors.New("race_priority_invalid")` in
`service.go`, mapped in `respondServiceError`, emitted for create, PATCH, and
the list query param. Follows the `<resource>_<field>_invalid` naming of the
package's existing codes and the `http-error-shape` invariant (JSON body with a
stable `error` code).

### D7 — MCP: schema change via goldengen regeneration, no new tools

`CreateRaceArgs` and `UpdateRaceArgs` (and `ListRacesArgs` per D5) in
`internal/agenttools/registry_races.go` gain the field with a jsonschema
description spelling out `A|B|C` and, on update, the empty-string clear. The
tool *set* is unchanged, so the `mcp-server` spec requirement ("six tools, one
HTTP call each") needs no delta. The announced-schema golden
(`internal/mcpserver/testdata/announced_schemas.json`) will legitimately drift
and must be regenerated with the capture test
(`go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/`)
— post `unify-mcp-tool-registry` there is no hand-maintained expected-tools
list to bump (CLAUDE.md's "bump the expected-tools list" note predates the
registry port; the sibling `add-performance-management` tasks already follow
the goldengen path).

## Risks / Trade-offs

- **Golden regeneration discipline.** The goldengen file header warns against
  regenerating "after porting a tool" — that guarded the bespoke→registry port
  freeze. A deliberate schema change is exactly when regeneration is correct;
  the risk is regenerating with unrelated drift in the tree. Mitigation: the
  diff to `announced_schemas.json` must show only the three race tools' schemas
  changing.
- **Migration slot contention.** Head is `054` but `add-race-pacing-plan`,
  `persist-activity-streams`, and other in-flight changes each add migrations.
  Mitigation: tasks mandate checking the highest existing number at apply time
  (repo convention; out-of-band work has taken slots before).
- **Empty-string clear can surprise generic clients.** A client PATCHing
  `{"priority":""}` expecting to *store* an empty string gets NULL. Accepted:
  it is the established repo-wide sentinel, documented in swag and jsonschema
  descriptions, and an empty-string priority is meaningless anyway.
- **Advisory stance permits contradictory data** (macrocycle anchors a C-race).
  Accepted by design (D4); the coach agent is the reconciliation layer.

## Migration Plan

1. At apply time, list `internal/store/migrations/` and take the next free
   sequential slot (do not assume `055`).
2. `task migrate:new NAME=add_race_priority` to scaffold the pair.
3. Up: `ALTER TABLE races ADD COLUMN priority TEXT CHECK (priority IN
   ('A','B','C'));` with a comment carrying the D1 rationale (nullable = not
   triaged, no backfill, set kept in sync with the service-layer switch).
4. Down: `ALTER TABLE races DROP COLUMN IF EXISTS priority;`
5. Append-only, no data migration, no index (single-user table, trivially
   small).

## Open Questions

- Should anchoring a race via `POST/PATCH /macrocycles` opportunistically set
  that race's priority to `A` when untriaged? Deferred — it blurs D4's "advisory
  only" line and is a one-line agent behavior instead.
- A `priority=none` (untriaged) list selector — deferred until the agent
  actually wants it (D5).
