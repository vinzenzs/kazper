## Why

The backend has become a full endurance-training engine — training plans, multisport sessions, Garmin scheduling, resolved zone/power targets, recovery/readiness context, race fueling — but the Flutter companion surfaces **none of it structurally**. Training reaches the app only through the conversational Chat coach; everything you can *glance at* (Today, Camera, Recent) is nutrition. A triathlete's day is two halves, **train** and **fuel**, and the app only shows the fuel half. The unique value Kazper has over a Garmin + MyFitnessPal stack is that those halves are **one engine** — today's hard session literally moves today's fueling — yet that coupling is invisible on the phone. This change gives training a first-class home, framed strictly as a **fueling lens**: a screen whose every element answers "so how do I feed this?"

## What Changes

- **Add a fifth screen, `Train`**, to the companion's bottom navigation (Today · **Train** · Camera · Recent · Chat). This amends the foundational "exactly four screens" invariant; the app stays a focused companion, but training now has a structured surface — admitted **only because it serves fueling**.
- **Bake in the guardrail as a hard requirement**: every element on the Train screen MUST have a fueling consequence. The screen MUST NOT include session scheduling, structured-workout editing, or "mark done" — execution lives on the watch/Garmin. This invariant is what keeps the 5th screen from drifting into a Garmin-clone cockpit.
- **v1 leads with Band 1 (the hero): today's session and its fuel.** The Train screen shows today's prescribed session(s) — sport, duration, resolved targets — and the **pre / during / post fueling that session demands**, with one-tap fueling actions (log workout fuel / hydration) through the existing offline outbox.
- **Reads are stale-while-revalidate**, reusing the app's established caching + outbox patterns; the Train screen is read-mostly with a small fueling-only write surface.
- **Deferred to later phases (out of scope here):**
  - *Band 2 — the causal delta* ("today's targets moved 2200→2600 kcal because of this session"). The differentiator, but it wants a clean backend read that breaks out the training contribution to the day's goal; a separate proposal.
  - *Band 3 — the weekly load + adherence arc* (sessions done/planned, TSS, energy-availability flag, race countdown). Gated on `plan-adherence-analytics`.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `mobile-companion`: the four-screen invariant becomes a **five-screen** invariant adding `Train`; new requirements define the Train screen as a fueling-lens-on-training (the fueling-consequence guardrail, the Band-1 session+fuel content, and its fueling-only outbox write surface).

## Impact

- **Code (client only)**: `apps/companion/lib/ui/train/` (new screen), `apps/companion/lib/data/` (repos/DAOs for the training reads — `GET /context/training`, `GET /workouts/{id}/program`, the per-session fuel recommendation), bottom-nav wiring, and fueling writes through the existing `data/sync` outbox.
- **Backend**: none for v1 — every read already exists (`coach-context`, `workouts/{id}/program`, `workout-fuel`/`recommend-workout-fuel`). The Band-2 causal-delta read is a separate future proposal.
- **Docs**: none server-side (no API change). The companion app has its own build.
- **Coupling**: complements `plan-adherence-analytics` (which feeds the deferred Band 3) and builds on the planned↔completed linkage and resolved-target work already shipped.
