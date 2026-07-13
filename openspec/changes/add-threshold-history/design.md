# Design — add-threshold-history

## Context

`athlete_config` is a fixed-key singleton (`internal/athleteconfig/`,
`PUT` full-replace mirroring `PUT /goals`): 16 nullable physiology columns —
`ftp_watts`, `threshold_hr`, `lactate_threshold_hr`, `max_hr`,
`threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m`, five
`hr_zone_*_max`, five `power_zone_*_max` — plus timestamps. Two properties
shape this design:

1. **The Garmin bridge re-issues the full `PUT` on every daily sync**
   (`apps/garmin-bridge/garmin_bridge/mapping.py` maps auto-detected FTP,
   LTHR, threshold run pace, max HR, HR zones, and FTP-derived power zones;
   the handler doc says "Garmin is source-of-truth: the daily sync re-issues
   this PUT"). So the write trigger sees a *mostly-unchanged* PUT once a day —
   naive append-on-PUT would produce a junk row per day. Conversely, this is
   also good news: Garmin-*detected* threshold changes already arrive through
   the only writer, so history captures them with zero new ingest machinery.
2. **Three in-flight siblings consume the singleton's CURRENT values** —
   `add-per-sport-tss` snapshots thresholds into TSS at write time (its design
   explicitly rejects read-time recompute), `add-step-compliance` resolves
   zones via the effective program, `add-race-pacing-plan` computes pacing on
   read against current thresholds. Two of them (`add-per-sport-tss`,
   `add-race-pacing-plan`) carry **MODIFIED** blocks on the same
   athlete-config requirement ("Config is the capture-only source of
   physiology; it consumes nothing in this change"). A third MODIFIED block on
   that requirement would guarantee a three-way merge conflict at archive
   time.

## Goals / Non-Goals

**Goals**

- Every real change to the athlete's physiology state is recorded with the
  date it took effect; the record is queryable (`GET /athlete-config/history`)
  and resolvable as of any date (`ConfigAsOf`).
- Zero behavior change to `GET`/`PUT /athlete-config` responses and error
  paths; the singleton remains the "current" read.
- History is never empty once a config exists (migration seed).
- Small blast radius: one package, one migration, one endpoint, one MCP tool,
  ADDED-only spec deltas.

**Non-Goals**

- **Rewiring existing/in-flight consumers to as-of resolution.** TSS
  write-time snapshots, zone resolution, and compute-on-read pacing keep
  reading current values; each rewiring is an explicit per-consumer follow-up
  once this lands. `ConfigAsOf` exists precisely so those follow-ups are a
  one-line data-source swap, not a schema project.
- Retroactive corrections of history rows (no mutation API on history).
- Auto-ingesting Garmin's *detection date* for a threshold change (we date by
  when the state reached Kazper — see Open Questions).
- Sub-day granularity (`effective_from` is a DATE, not a timestamp).

## Decisions

### D1. Full-row snapshots, not per-field change events

Each history row stores the **complete** post-PUT physiology state (all 16
columns, same types and CHECKs as `athlete_config`) with an `effective_from`
date.

- The config is small and fixed — a full row costs ~16 nullable columns, and
  the write path already *has* the full state in hand (PUT is full-replace, so
  the new state is the request; there is no partial-update path to diff).
- As-of resolution becomes one query: latest row with
  `effective_from <= $date`. Per-field events would require folding an event
  stream back into a state — reimplementing the snapshot at every read.
- "What changed" is still derivable by diffing adjacent snapshots client-side
  (the coach agent is good at exactly this); the reverse (state from events)
  is the expensive direction, so store the state.
- Precedent: `fitness_metrics` stores dated full snapshots, not deltas.

Alternative rejected: `(field, old, new, changed_at)` event rows — cheaper per
change, but every consumer pays reassembly, NULL-vs-absent gets ambiguous in
the event shape, and the "history is the sequence of states" read (the actual
coaching question) needs a window function instead of a `SELECT`.

### D2. Write trigger: inside `Service.Put`, with physiology-equality dedup

`Service.Put` already does validate → upsert → read-back. It gains: load the
prior state, upsert the singleton, and maintain history **iff** the new state
differs from the prior state on any of the 16 physiology fields (pointer-aware
equality; `created_at`/`updated_at` excluded). Singleton upsert and history
maintenance run atomically (single transaction — the repo layer already works
against `store.Querier`, usable with a `pgx.Tx`), so history can never
disagree with the singleton.

Dedup compares **all 16 fields**, not a curated "threshold-relevant" subset:
zone boundaries are threshold-derived physiology (the bridge derives power
zones from FTP), a zone-boundary change is coaching-relevant history in its
own right, and a subset rule would be a second list to keep in sync with the
schema. The practical requirement — the daily Garmin re-PUT of an unchanged
config appends nothing — is met either way.

First-ever PUT (no prior row): the new state trivially "differs from" nothing
and appends a snapshot (unless the seed row already equals it — the generic
dedup covers this).

### D3. `effective_from` is a DATE; same-day changes collapse to one row

`effective_from DATE PRIMARY KEY`. A threshold is a per-day quantity in every
consumer's mental model (workouts have dates; "FTP on race day"), and the PUT
date is the only honest timestamp we have.

Same-day double update: the second PUT **replaces** that date's snapshot
(upsert on `effective_from`) — the history records the state in effect *at
the end of* each changed day, not intra-day churn. Degenerate case: a same-day
revert (240 → 255 → 240) would leave a snapshot identical to the previous
day's; the maintenance step therefore dedups against the latest snapshot with
`effective_from` **strictly before** today — if the new state equals it, any
same-day row is deleted instead of upserted. Invariant: **no two consecutive
history rows are physiology-identical** — the history is canonical and
minimal.

`created_at`/`updated_at` on the row record when the snapshot was
written/last replaced, preserving audit fidelity beneath the date-grain
identity.

### D4. Seed row at sentinel epoch `1970-01-01`, inserted by the migration

The migration inserts one snapshot copied from the current singleton (if one
exists) with `effective_from = DATE '1970-01-01'`.

- **Why seed at all:** without it, history is empty until the next threshold
  change, and `ConfigAsOf(any date)` returns nothing despite a configured
  athlete — every future consumer would need an "empty history → fall back to
  singleton" branch forever.
- **Why the epoch sentinel, not the migration date:** the seed's honest claim
  is "this is the oldest state we know, assume it for everything earlier" —
  exactly how TrainingPeaks treats the initial threshold. Dating it at the
  migration date would make `ConfigAsOf(workout in 2025)` return nil even
  though we *do* have a best-known state, pushing the fallback branch onto
  every consumer anyway. The epoch makes `ConfigAsOf` total (non-nil for any
  plausible date) whenever history is non-empty. The row's `created_at` keeps
  the truth of *when* tracking began.
- Pure SQL `INSERT ... SELECT` from `athlete_config` — no Go logic in the
  migration, per the schema-plus-trivial-data-fixes convention. Fresh
  databases (no config row) seed nothing; the first PUT appends the first
  snapshot (D2).

### D5. Endpoint: `GET /athlete-config/history?from=&to=`

Registered in the existing `athleteconfig.Handlers.Register` (same router
group; no `httpserver/server.go` change). Response:

```
{"history": [
  {"effective_from": "1970-01-01", "ftp_watts": 240, ..., "created_at": ..., "updated_at": ...},
  {"effective_from": "2026-05-02", "ftp_watts": 255, ...}
]}
```

- Snapshots ascending by `effective_from`; each entry is the full config state
  with nulls omitted (`omitempty`), floats `numfmt.Round1` at the response
  boundary only — identical presentation rules to `GET /athlete-config`.
- `from`/`to` are optional inclusive DATE bounds on `effective_from`.
  Malformed date → `400 {"error":"date_invalid","field":"from"|"to"}`;
  `from > to` → `400 {"error":"range_invalid"}` (per `http-error-shape`, no
  new invariants needed).
- **No range cap and no pagination**, unlike `list_fitness_metrics`'s 92-day
  window: history grows only when physiology changes (a handful of rows per
  season for one athlete), so a bound would be ceremony. Empty result is
  `200 {"history":[]}` — including before any config exists (no 404; absence
  of history is a normal state, matching `GET /athlete-config`'s null-not-404
  posture).
- No `as_of` query param in v1 — the as-of contract is service-level (D6);
  the agent can derive "state on date X" from the history list. An HTTP
  `?as_of=` is a trivial later addition if a client wants it.

### D6. As-of lookup: `Service.ConfigAsOf(ctx, date)`

Contract: returns the snapshot (full physiology state + its `effective_from`)
from the latest history row with `effective_from <= date`; `(nil, nil)` when
history is empty (config never set) — mirroring `Repo.Get`'s nil-row signal.
One indexed query (`effective_from` is the PK; `ORDER BY effective_from DESC
LIMIT 1`).

Deliberately **unconsumed in this change**: no existing code path switches to
it. This keeps the change orthogonal to the consumption-gate requirement both
siblings are modifying (D7) and honors their already-reasonable semantics —
`add-per-sport-tss`'s "computed against the thresholds in effect at write
time, never retroactively recomputed" is exactly what history now makes
*auditable*, and its explicit recompute endpoint is where as-of resolution
would slot in as a follow-up.

### D7. Sibling-conflict avoidance: ADDED-only deltas

Both `add-per-sport-tss` and `add-race-pacing-plan` carry MODIFIED blocks on
the athlete-config requirement "Config is the capture-only source of
physiology; it consumes nothing in this change" (each appending itself to the
consumer list). This change's delta is **purely ADDED requirements** — four
new requirement blocks with new names, zero MODIFIED blocks — so it composes
with either, both, or neither sibling landing first, in any order. This works
because v1 adds *state and a read surface*, not a consumer: the gate
enumerates who consumes config values, and nothing here consumes anything —
history *is* the config, recorded over time. The one wording landmine (the
gate's "otherwise-unconsumed" clause) is avoided by making the as-of
requirement explicitly a provider contract with "no existing consumer is
rewired in this change".

### D8. MCP: one read tool `athlete_config_history_get`

Registered beside the existing `athlete_config_get` in the shared
`agenttools` registry (same file/domain), `TierRead`, optional `from`/`to`
args, building exactly one `GET /athlete-config/history` call and forwarding
the body verbatim. No idempotency key (read). The announced surface is
registry-derived (post `unify-mcp-tool-registry`) — no hand-maintained
expected-tools list; the schema golden is regenerated with
`go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/`.
No write tool: history has no direct write surface (the PUT hook is the only
writer).

## Risks / Trade-offs

- **[Risk] Triple-delta conflict on the athlete-config consumption-gate
  requirement.** → Mitigation: ADDED-only requirements (D7); this change
  never touches the gate block, so archive order relative to both siblings is
  irrelevant.
- **[Risk] The bridge's full-replace PUT clears fields it doesn't map** (swim
  CSS and `threshold_hr` are unmapped in `mapping.py`), so a manual CSS edit
  followed by a daily sync could yo-yo — and history will now record every
  flip as a dated change. → Mitigation: this is a pre-existing property of the
  full-replace singleton that history merely makes *visible* (arguably the
  point of an audit trail); recording it truthfully is correct. Fixing the
  bridge's clobbering is a separate bridge-side concern, noted as an open
  question.
- **[Risk] Date-grain `effective_from` misdates a threshold Garmin detected
  yesterday but synced today.** → Mitigation: accepted for v1 — off-by-one-day
  on a slowly-changing quantity is immaterial for progression analysis; the
  alternative (parsing Garmin detection timestamps) needs bridge changes that
  are out of scope (Open Questions).
- **[Risk] Epoch-dated seed row could read as "FTP has been 240 since 1970".**
  → Mitigation: sentinel semantics documented in the spec and the endpoint's
  swag description ("baseline: oldest known state"); `created_at` on the row
  carries the real recording time; the coach agent sees one obviously-sentinel
  date, not a fabricated plausible one.
- **[Risk] Dedup-on-all-fields means a zones-only change (no FTP/pace change)
  appends a row, which a naive "FTP history" reading might find noisy.** →
  Mitigation: intended (D2) — snapshots are state history, not an FTP feed;
  the agent filters by the field it cares about. Cheaper than maintaining a
  threshold-relevant field subset in code.
- **[Risk] Transactionality: if the history write were best-effort after the
  upsert, a failure would silently drop a change forever.** → Mitigation: D2
  runs both writes in one transaction; a history failure fails the PUT loudly
  (500), never silently diverges.

## Migration Plan

1. **Verify the migration head first**: `ls internal/store/migrations | tail`
   — head is `054_sync_run_summary_partial` at writing, but
   `add-per-sport-tss` and `add-race-pacing-plan` each claim a slot and
   out-of-band work has taken numbers before. Then
   `task migrate:new NAME=add_athlete_config_history`.
2. **Up**: create `athlete_config_history` — `effective_from DATE PRIMARY
   KEY`, the 16 physiology columns (same types + `> 0` CHECKs as
   `athlete_config`), `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT
   now()`; then the seed:
   `INSERT INTO athlete_config_history (effective_from, <cols>) SELECT DATE
   '1970-01-01', <cols> FROM athlete_config` (0 or 1 rows; pure SQL, no
   formulas).
3. **Down**: `DROP TABLE athlete_config_history` — nothing references it.
4. Purely additive: no existing table, endpoint, or response changes; rollback
   is the down migration.
5. Deploy order-independent with both siblings (no shared tables; only the
   migration *number* must be reconciled at apply time).

## Open Questions

- **Garmin-detected threshold auto-ingest fidelity.** The bridge already
  delivers detected FTP/LTHR/threshold-pace via the daily PUT, so changes are
  captured — but dated by sync arrival, not Garmin's detection date, and
  unmapped fields (CSS, `threshold_hr`) are clobbered by the full-replace.
  Should the bridge merge-instead-of-replace, and/or forward Garmin's
  detection timestamp for a truer `effective_from`? Deferred — bridge-side
  change, separate proposal.
- **Retroactive corrections.** "My FTP actually changed on May 2nd, not May
  9th when I got around to updating it" has no v1 answer (no history mutation
  API). A `PUT /athlete-config/history/{date}` correction surface is
  plausible later; deferred until the read surface proves its shape.
- **HTTP `?as_of=` param** (or `GET /athlete-config?as_of=`) once a client
  (companion app charts, coach dashboard) wants server-side resolution rather
  than deriving from the list. Trivial on top of D6; deferred.
- **Consumer rewiring order.** When the follow-ups land, which consumer goes
  first? Likely `add-per-sport-tss`'s recompute endpoint (as-of makes its
  historical recompute *more* honest, not less); step-compliance and race
  pacing arguably *should* stay current-value (you race on today's fitness).
  Decided per follow-up, not here.
