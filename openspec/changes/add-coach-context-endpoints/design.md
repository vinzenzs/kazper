## Context

`internal/dailycontext` is the precedent: a wide-constructor Service that composes existing read-side repos in parallel (`errgroup`), returns one bundle, rounds numerics at the boundary (`numfmt`), and fails whole rather than partial. This change follows that shape for two new aggregate reads the coach needs. Source repos already exist: `workouts.Repo.List(from,to,group?,status?)`, `fitnessmetrics.Repo.List(from,to)` / `GetByDate`, `recoverymetrics.Repo.List(from,to)` / `GetByDate`, `trainingphases.PhasesRepo.PhaseFor(date)`. All list reads are ascending by date.

## Decisions

### D1 — One package, two endpoints

Both reads are cohesive "coach context" aggregators, so they live in one `internal/coachcontext` package with two handlers (`/context/training`, `/context/recovery`) over one Service, rather than two near-identical packages. `daily-context` stays separate (it predates this and is nutrition-anchored).

### D2 — Composition-only, parallel, no partial bundle

Mirror `dailycontext.BuildFor`: fetch each slice in an `errgroup`; any failure cancels and returns an error (the coach must not reason on a half-bundle). Numeric fields round through `numfmt.Round1` at the boundary. Absent snapshots are `null`, not errors (a quiet day is valid).

### D3 — Training context shape

`GET /context/training?date=&tz=&lookback_days=14&lookahead_days=7`:
- `phase`: the training phase covering `date` (or null).
- `fitness`: the latest fitness snapshot on/before `date` within the lookback window (or null) — VO2max, acute/chronic load, training status, race predictors.
- `acwr`: acute ÷ chronic load when both present (derived, rounded, never stored) — the single most useful load-balance signal.
- `recent_load`: a summary over completed workouts in `[date-lookback, date]` — count, total duration, total kcal, count by sport.
- `recent_workouts`: those completed workouts (lite shape).
- `upcoming_workouts`: planned workouts in `(date, date+lookahead]` (lite shape).

Lookback/lookahead are clamped to sane bounds (1–90 / 0–60) so an absurd query can't scan unboundedly.

### D4 — Recovery context shape

`GET /context/recovery?date=&days=7`:
- `latest`: the most recent recovery snapshot on/before `date` within the window (or null).
- `recent`: the snapshots over `[date-days, date]` ascending — the sleep/HRV/readiness trend the coach reads before advising on a hard session.

Recovery snapshots are date-keyed (no tz needed); `date` defaults to today in the server zone.

### D5 — Dual-surface naming (D11 from expand-chat-to-coach)

The MCP tools are named **`get_training_context` / `get_recovery_context`** — identical to the names `expand-chat-to-coach` will use in `agenttools` — and added to `AnnouncedToolNames`. Identical names across surfaces keep the chat drift-guard's subset invariant intact without growing `chatBespokeTools`. (The existing nutrition aggregate is `daily_context` in MCP vs `get_daily_context` in chat — a pre-existing bespoke exception we are deliberately not repeating.)

## Risks / Trade-offs

- **[Composition cost]** Each endpoint fans out several repo reads. Mitigation: parallel via errgroup; lookback bounds cap the row counts; these replace ~30 granular tool round-trips for the model, a net reduction.
- **[Shape churn]** The aggregate shapes may evolve as the coach prompt matures. Mitigation: they are additive JSON with `omitempty`; the chat side is added in a later change so we can adjust before it depends on them.

## Migration Plan

No DB migration. New package + handlers + MCP tools + `AnnouncedToolNames` entries. Independently shippable; `task swag` after handler structs land.
