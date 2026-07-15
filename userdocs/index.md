# What is Kazper

Kazper is a **personal nutrition + endurance-training backend** with an in-app AI coach (also
named Kazper). It is built for one athlete, not a platform: a single Go + Postgres service that
stores everything you eat, drink, train, and weigh — and computes the numbers a coach needs to
reason about it.

## The one-sentence mental model

> Garmin and you feed raw data in; Kazper stores it honestly, derives training-science metrics
> on demand, and the coach — an LLM talking to the same API over MCP — turns those numbers into
> advice you confirm or reject.

## Surfaces

| Surface | What it is | Talks to |
|---|---|---|
| **REST API** | The single source of truth (`/api/v1/*`) | everything below |
| **Coach (MCP)** | An LLM agent with ~80 tools, each exactly one API call | the API, as the `agent` identity |
| **Chat** | Streaming coach conversation with write-confirmation pauses | the API, in-process |
| **Dashboard** | Browser SPA (training, stats, records, gear) served by the binary | the API, as the `web` identity |
| **Companion app** | Flutter mobile app (logging, Garmin connect, push) | the API, as the `mobile` identity |
| **garmin-bridge** | Headless daily sync pulling your Garmin data | the API, as the `garmin` identity |
| **Public race page** | A static "road to race" countdown for friends | built nightly from one curated feed |

## What gets calculated

Kazper's derived values fall into families, each with its own section in this guide:

- **Training load** — TSS per workout (from power, pace, or HR), and the PMC: CTL (fitness),
  ATL (fatigue), TSB (form), plus your planned-vs-actual CTL trajectory.
- **Power analytics** — from stored 1 Hz streams: NP/VI/EF/decoupling per ride, the
  mean-maximal power curve, critical power & W′, W′ balance inside a ride, the Coggan power
  profile, durability (power after kilojoules of work), detected intervals, quadrant analysis.
- **Plan execution** — adherence (did the sessions happen) and step compliance (were they
  executed as written).
- **Nutrition & fueling** — daily macro summaries against goals, hydration and in-session
  fueling, energy availability, race carb-loading, race fueling and pacing plans, sweat rate.
- **Recovery & wellness** — Garmin's objective signals (HRV, sleep, readiness) beside your own
  subjective diary, and the correlation between how you feel and your training load.

## Two design principles worth knowing

**Numbers are honest or absent.** Kazper never imputes: a workout without TSS stays NULL and is
*counted* as missing rather than guessed; an analytic that can't be computed says why
(`insufficient_points`, `no_tiered_data`, `weight_missing`) instead of returning something
plausible. Advisory estimates (like critical power) are labeled advisory and never silently
overwrite your configured values.

**Writes are deliberate.** Anything that changes your thresholds, goals, or plans goes through
an explicit confirmation — the coach proposes, you approve. Automated data flows (Garmin sync)
may *inform* those values but are being moved out of the business of writing them.
