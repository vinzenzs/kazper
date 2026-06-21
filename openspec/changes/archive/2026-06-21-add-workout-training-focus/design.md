## Context

Workouts (`internal/workouts/`) already carry three enum-valued string fields —
`source`, `sport`, `status` — each implemented identically: a typed `string` alias, a
const block of allowed values, a `Valid<X>(s string) bool` switch, a `Parse<X>` helper,
a DB CHECK constraint, and a `<x>_invalid` sentinel error mapped 1:1 to an API error
code. The table also has a well-worn precedent for *nullable, no-back-fill* fields
(`rpe`, `gi_distress_score`, and the five ingestion metrics) added by later migrations,
threaded through POST/bulk/PATCH with tri-state PATCH semantics and `omitempty`
serialization.

`training_focus` is the union of those two precedents: an enum field that is also
nullable. Nothing new architecturally — the work is following both patterns exactly.

## Goals / Non-Goals

**Goals:**
- Classify a workout's intensity band against the 7-zone Trainingsbereiche taxonomy.
- Optional everywhere: unclassified (NULL) is a first-class state, not a data defect.
- Settable on POST/bulk, patchable with tri-state (absent/value/null) semantics,
  returned with `omitempty`.
- Validated against the closed enum with a dedicated `training_focus_invalid` code.

**Non-Goals:**
- No sport-coupled validation (e.g. forbidding `strength_endurance` on `sport='run'`).
  The taxonomy crosses sports; coupling would be brittle and isn't needed.
- No derivation of `training_focus` from HR zones / power / TSS. It is an explicit
  human/coach annotation, not an inferred field.
- No new MCP tool and no change to the MCP expected-tools list.
- No mobile-companion change in this proposal (optional follow-up).
- No back-fill or default for existing rows — they stay NULL.

## Decisions

### 1. Enum values: snake_case canonical names, German codes documented not stored

Stored/validated values are the readable English snake_case names; the German
shorthand is documentation only:

| stored value           | German | gloss                         |
|------------------------|--------|-------------------------------|
| `recovery`             | REKOM  | regeneration / compensation   |
| `basic_endurance_1`    | GA1    | aerobic base                  |
| `basic_endurance_2`    | GA2    | extensive tempo               |
| `development`          | EB     | threshold / development        |
| `competition_specific` | WSA    | race-specific endurance       |
| `peak`                 | SB     | peak / sharpening (anaerobic) |
| `strength_endurance`   | KA     | strength endurance            |

Rationale: matches the existing convention (`run`, `basic`-style readable tokens), keeps
the wire self-describing, and keeps the CHECK constraint legible. The Go const
identifiers (`TrainingFocusBasicEndurance1`, …) carry the German code in a doc comment.

**Alternative considered:** store the German codes (`ga1`, `wsa`). Rejected — opaque to
anyone not steeped in the methodology, and inconsistent with the rest of the enum
vocabulary in this package.

### 2. Field name `training_focus` (not `intensity_class`, not `zone`)

`zone` is already overloaded — `secs_in_zone_1..5` are HR-time-in-zone columns.
`training_focus` reads naturally and collides with nothing. (Chosen by the user.)

### 3. Nullable, no back-fill, no default — follow the rpe/ingestion-metric precedent

Column is `TEXT NULL` with a CHECK that admits the 7 values *or NULL*. Existing rows
stay NULL; POST/bulk/PATCH default to NULL when omitted. This reuses the exact
migration shape of `018`/`019`, so the "nullable per session, no back-fill" scenarios
carry over verbatim.

### 4. Tri-state PATCH via the established mechanism

PATCH threads `training_focus` exactly like the ingestion fields: absent leaves
unchanged, a value sets it, explicit JSON `null` clears to NULL. The handler/service
already has the `*string` + `Clear<X>` plumbing for nullable fields to copy. The
empty-string sentinel used for `workout_id` link-clearing is *not* used here — this is a
direct-`null` clearable field like `tss`/`rpe`, not an FK link.

### 5. Validation lives in the service layer, mapped to one error code

`buildWorkout` and the patch path call `ValidTrainingFocus`; on failure the handler
returns `400 {"error":"training_focus_invalid"}`. No `range` object (unlike rpe) —
it's an enum, so the shape mirrors `sport_invalid`/`status_invalid`.

## Risks / Trade-offs

- **[Closed enum will need extension later]** → The Sport enum already documents the
  "extend the const + widen the CHECK when a value earns its surface" pattern; a future
  migration widens the CHECK additively, exactly as `074` widened sport for yoga/mobility.
- **[Taxonomy is opinionated / German-methodology-specific]** → Single-user project; the
  user picked the full 7-zone model deliberately. Documented mapping table keeps intent
  legible to future-self.
- **[Field could drift from HR-zone reality]** → Accepted: `training_focus` is declared
  intent, not measured. The two coexisting (intent vs. `secs_in_zone_*` actuals) is a
  feature for adherence review, not a contradiction.

## Migration Plan

1. `task migrate:new NAME=add_workout_training_focus` → scaffolds `048_*.{up,down}.sql`.
   Verify `047` is still the head before committing (out-of-band slots have happened).
2. `up.sql`: `ALTER TABLE workouts ADD COLUMN training_focus TEXT
   CHECK (training_focus IN (...7 values...));` (NULL-able, NULL passes the CHECK).
3. `down.sql`: `ALTER TABLE workouts DROP COLUMN training_focus;`
4. Thread the field through types/service/repo/handlers, add tests, `task swag`,
   `task test`, `task vet`.
5. Rollback = run the down migration; column drop is non-destructive to other fields.

## Open Questions

- None blocking. Mobile-companion surfacing of `training_focus` is deferred to a separate
  change if/when the Train screen wants to show or set it.
