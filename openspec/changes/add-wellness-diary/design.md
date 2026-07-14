## Context

Kazper's recovery picture is entirely objective and Garmin-fed (`recovery-metrics`, `health-vitals`: HRV, sleep, RHR, readiness, body battery), surfaced through `/context/daily` and the dashboard. Subjective state exists only as per-workout `rpe` and unstructured `coach-memory` text. Intervals.icu's wellness log is the model: a dated record combining device vitals with self-reported scores. Kazper only needs the self-reported half — the objective half is already better than what a manual log would capture.

The repo's capability template fits exactly: one domain shape, `internal/<name>/` with types/repo/service/handlers, sentinel errors 1:1 with API codes, per-date singleton semantics precedented by `goals` overrides (`PUT /goals/overrides/{date}`).

## Goals / Non-Goals

**Goals:**
- A queryable daily series of subjective scores the coach can write in one tool call and read over a window.
- Semantics simple enough to answer in ten seconds of conversation ("legs? mood? sleep quality is already tracked").
- Subjective and objective side by side in `/context/daily` — correlation happens in the coach's reasoning, not in SQL, for v1.

**Non-Goals:**
- Correlation/analytics endpoints (wellness × TSB/PMC) — revisit once there's data.
- Dashboard panel and companion-app UI — chat is the collection surface; read surfaces can follow demand.
- Structured injury tracking (site/severity/status lifecycle) — a freeform `note` mention suffices until it doesn't.
- Importing subjective scores from Garmin (its "self-evaluation" feature) — the coach conversation is deliberately the source.

## Decisions

### D1 — One row per date, five optional 1–5 scores + note; at least one field required
`wellness_entries`: `entry_date DATE PRIMARY KEY`, `fatigue`/`soreness`/`stress`/`mood`/`motivation` as nullable `SMALLINT` with `CHECK BETWEEN 1 AND 5`, `note TEXT` (service-capped at 2000 chars), timestamps. Directions: symptom-like fields read 1 = none → 5 = severe; state-like fields read 1 = low → 5 = high — each field keeps its natural reading rather than forcing a uniform "higher is better" that nobody speaks. An entry with every field null is rejected (`wellness_empty`): partial answers are the norm ("just log soreness 4"), empty ones are noise.

- **Why 1–5 ints, not enums or 1–10?** Five levels is what a human distinguishes conversationally; ints sort/average trivially when correlation analytics come later.
- **Why no `injury` column?** Premature structure — the note carries it; a real injury lifecycle would be its own capability.

### D2 — Per-date singleton with PUT full-replace upsert
`PUT /wellness/{date}` creates or fully replaces that date's entry — the `goals` per-date-override pattern. Corrections are a re-PUT, so there's no PATCH tri-state to design; `DELETE /wellness/{date}` removes. Per `harden-write-paths`, the PUT rejects an `Idempotency-Key` header (`400 idempotency_unsupported_for_put`). Dates are naive calendar dates (the athlete's day, like goals overrides); no timezone parameter.

- **Why not POST + PATCH?** A day's subjective state is one thing, not an append log; full-replace matches how it's collected ("this morning: fatigue 3, mood 4") and re-collection replaces.

### D3 — Window read is ascending with the shared range vocabulary
`GET /wellness?from=&to=` returns ascending entries, `200 {"entries":[]}` when empty (no 404 — the athlete-config-history precedent), shared errors (`date_invalid` + `field`, `range_invalid`); a 92-day cap (the nutrition-summary tier, not the 400-day workout tier — wellness questions are recent-block-shaped). `GET /wellness/{date}` returns `404 not_found` for an absent day.

### D4 — MCP: `log_wellness` (write) + `list_wellness` (read)
`log_wellness` wraps the PUT (full-replace semantics stated in the description, no idempotency key — the `athlete_config_update` precedent); `list_wellness` wraps the window GET. Both registry-derived, golden-gated. The write is a normal MCP write tool (chat surfaces its usual confirm flow); the tool description encourages partial entries over skipped ones.

### D5 — `/context/daily` carries today's entry only
The daily context payload gains `wellness` — today's entry verbatim, omitted entirely when absent (never an empty object). History stays behind `list_wellness`; the context endpoint is a snapshot, not a series (its existing posture). No `/context/training` change — training context is load-shaped, and the coach pulls the window explicitly when it wants trend.

### D6 — Export-included
User-authored, small, not re-derivable — the `athlete_config_history` classification, added to `internal/dataexport/inventory.go` in the same change so the drift guard never fires.

## Risks / Trade-offs

- **Sparse data risk** (diaries die) — mitigated by design: the coach *asks* (collection is conversational, not form discipline), partial entries are first-class, and absence is simply omitted from context rather than nagged about.
- **Subjective scores drift in personal meaning over time** — accepted; the scores are conversation anchors for the coach, not clinical instruments.
- **A second recovery-ish block in `/context/daily`** grows the payload — one small object per day; acceptable against the value of subjective-next-to-objective.

## Migration Plan

Additive: migration `060` (check the on-disk head first — the standing convention), new routes/package, one context field, two MCP tools. Rollback = revert registration + down-migration. No back-fill (history starts when logging starts).

## Open Questions

- Should `/context/daily` also carry yesterday's entry when today's is absent (morning check-in reads yesterday evening's log)? (v1: today only; the coach can `list_wellness` for yesterday.)
- Correlation surface (wellness vs TSB/ramp) — deferred until enough entries exist to be honest about.
