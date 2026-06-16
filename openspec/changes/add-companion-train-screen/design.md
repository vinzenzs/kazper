## Context

The `mobile-companion` capability nails the app to **exactly four screens** (Today, Camera, Recent, Chat) plus Settings + shopping, framed as "a focused supplement to the agent, not a replacement." Its only training surface is Chat (the conversational coach, which already spans training + nutrition). Every structured/glanceable screen is nutrition. Meanwhile the backend exposes a complete training read surface — `GET /context/training` (phase, ACWR, recent + **upcoming** workouts), `GET /context/recovery`, `GET /workouts/{id}/program` (resolved zone/power/cadence targets, multisport segments), `workout-fuel` / `recommend-workout-fuel`, race fueling — none of which the app reads structurally. The app's client patterns are mature: stale-while-revalidate reads with no offline banner, and an offline-first outbox for every mutating call.

The user's two framing decisions set the shape: **(C)** training gets its own first-class screen (not folded into Today), and **(2)** its purpose is to *fuel* the training, not *track* it. Those two together would normally pull in opposite directions (own-screen → cockpit; fuel-not-track → companion); the resolution is a hard invariant that gates what may appear on the screen.

## Goals / Non-Goals

**Goals:**
- Give training a structured, glanceable home on the phone, scoped to its fueling consequences.
- v1 leads with the day's session and the fuel it demands (Band 1, the hero).
- Reuse the existing SWR + outbox client patterns; no new backend for v1.

**Non-Goals (v1):**
- Session scheduling, structured-workout editing, "mark done", lap/split review, power curves — execution and history live on the watch / Garmin / Strava.
- Band 2 (the causal "training moved your targets" delta) — wants a backend read; separate proposal.
- Band 3 (weekly load + adherence + EA flag + race countdown) — gated on `plan-adherence-analytics`.
- Any backend/API change.

## Decisions

### D1: A fifth screen, not a fold-into-Today (chosen: C)
`Train` joins the bottom nav as a peer (Today · Train · Camera · Recent · Chat — Material's 5-destination cap, exactly hit). Rejected folding training into Today (option B): the user wants training to have its own home, and Today stays meal-centric ("what/when do I eat") while Train is session-centric ("what's my body doing, and how do I fuel it"). This amends the spec's four-screen invariant — done consciously, as a MODIFIED requirement.

### D2: The fueling-consequence invariant is a hard requirement (chosen: yes)
**Every element on the Train screen MUST answer "so how do I feed this?"** The screen MUST NOT surface session scheduling, structured-set editing, completion toggles, or training-history analytics with no fueling consequence. This is the load-bearing constraint that keeps a training screen from becoming a Garmin clone — it is written as a normative requirement, not left to taste. The litmus test for any future addition: *if it doesn't change how the athlete fuels, it belongs on the watch.*

### D3: Band 1 is the hero (chosen: band 1)
v1's content is **today's session + its fuel**:
- the day's prescribed session(s) from `GET /context/training` (upcoming) — sport, duration, planned time;
- its resolved targets from `GET /workouts/{id}/program` (e.g. `230–268W (Z4)`, multisport segments);
- the **pre / during / post fueling** the session demands, from `recommend-workout-fuel` / `workout-fuel`.
Band 2 (causal delta) and Band 3 (weekly arc) are deferred. Band 1 is session-centric utility and needs only existing reads — the tightest shippable unit.

### D4: Reads SWR, writes fueling-only through the outbox
Training reads (today's session, its program, its fuel) follow the established stale-while-revalidate pattern (cached, shown stale, revalidated; no offline banner). The write surface is deliberately tiny and **fueling-only** — log workout fuel, log during-session hydration, optionally "accept" the recommended fuel to pre-seed planned meals/hydration — all through the existing offline outbox with client-minted idempotency keys. No scheduling write exists; adding one would violate D2.

### D5: No backend work for v1; one gap noted for Band 2
Every v1 read exists. The one thing the backend does not cleanly provide is Band 2's **causal delta** — "today's goal, with the training contribution broken out" — which today the client could only approximate by diffing base vs training-adjusted goals. That is captured as a future backend read, not built here.

## Risks / Trade-offs

- **Scope creep toward a cockpit** → D2's invariant is the explicit guard; every PR to the Train screen is checked against "does this change fueling?".
- **Two "my day" screens (Today + Train) confuse** → mitigated by a clean split: Today = meals (eat), Train = sessions-and-their-fuel. If they blur in practice, revisit a merge — but the user explicitly chose separate.
- **Bottom nav at the 5-destination max** → no room for a 6th without restructuring; acceptable and a useful forcing function.
- **Band 1 without Band 2** → the screen shows "this session needs X fuel" but not yet "…which is why today's targets changed." Still useful; Band 2 lands the emotional differentiator later.

## Migration Plan

Client-only, phased. v1 ships the Train screen with Band 1 behind the existing app build/release. No server migration, no API change. Rollback = remove the nav destination; nothing server-side to undo. Phase 2 (Band 2 causal delta) and Phase 3 (Band 3 adherence arc) are separate proposals, the latter sequenced after `plan-adherence-analytics`.

## Open Questions

- Does "accept the recommended fuel" (pre-seed planned meals/hydration around the session) belong in v1, or is it Band-1-adjacent scope for a fast-follow? It is the most actionable fueling write but also the most plumbing.
- Should a rest day render the Train screen empty, or show recovery/EA guidance (which is still fueling-relevant)? Leaning: a minimal "rest — fuel for recovery" state, not a blank screen.
- Nav order: Today·Train·Camera·Recent·Chat vs Train·Today·… — which is the athlete's primary morning glance?
