# Garmin recovery metrics

The bridge mirrors Garmin's daily recovery and fitness signals so the coach can ground advice in
them:

| Signal | What it is |
|---|---|
| **HRV (overnight)** | autonomic recovery status — the strongest single readiness signal |
| **Sleep** (duration, score) | recovery input; trends matter more than single nights |
| **Resting HR** | elevated RHR often precedes illness/overreach |
| **Stress / Body Battery** | Garmin's all-day load-vs-recovery estimates |
| **Training readiness / status** | Garmin's own verdicts (productive, strained, …) |
| **VO₂max & race predictions** | Garmin's fitness estimates and predicted race times |
| **Acute/chronic load** | Garmin's load-ratio view (Kazper's own PMC is the primary) |

These are **mirrors** — Kazper stores them verbatim, never recomputes them — and they sit beside
Kazper's own derived metrics in the coach's context so disagreements are visible ("Garmin says
productive, your TSB says -25 and your diary says legs are dead").

## Sync freshness

Every sync is tracked; the API exposes `last_successful_at` and an `is_stale` flag (> 26 h).
If the bridge's Garmin login expires, the companion app gets a push prompting a re-login — until
then, recovery data quietly ages, which is exactly why staleness is surfaced.
