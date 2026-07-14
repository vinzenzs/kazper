## Context

`internal/effortanalytics/` already stores the mean-maximal best-effort ladder (`{5,15,30,60,300,600,1200,1800,3600}` s) and serves `CurveFor` (windowed per-duration MAX) + the CP model. This change adds one more compute-on-read view over that same data: rank the athlete against Andrew Coggan's power-profile tables. Those tables (from *Training and Racing with a Power Meter*) list W/kg at four benchmark durations — **5 s** (neuromuscular power), **1 min** (anaerobic capacity), **5 min** (VO₂max/aerobic power), and **FT** (functional threshold) — across ~monotone category rows from "untrained" to "world class", separately for men and women.

Two of the four anchors are exact ladder durations (5 s, 60 s, 300 s). The FT column has no direct ladder point; the standard practical proxy is the 20-minute best (ladder `1200`), so the endpoint ranks the 20-min W/kg against the FT column and labels it a proxy (no 0.95 haircut — the ranking is advisory, transparency beats a hidden fudge).

The CP-model precedent sets the posture: advisory, compute-on-read, never touches athlete-config, degrades to a reasoned partial rather than erroring.

## Goals / Non-Goals

**Goals:**
- Turn the existing power curve into a "how good is this" answer: Coggan category + rider phenotype, in one read.
- Honest degradation: rank whichever of the four anchors have a best-effort in the window; name the missing ones; never fabricate.
- Reuse `CurveFor` and `parseWindow` — no new storage, no second data path.

**Non-Goals:**
- Run critical-speed / swim ranking (no comparable published table) — power only.
- A configurable or personal reference table — the Coggan tables are a fixed embedded reference; personalization is a different feature.
- Persisting a ranking or trend history — this is a stateless snapshot (a future "profile trend" would be its own change).
- Wiring the phenotype into `/context/training` or shipping the public-site v2 `progress` number — this endpoint is the building block, not those consumers.

## Decisions

### D1 — Four anchors, 20-min best as the FT proxy
Anchors map ladder→Coggan column: `5→neuromuscular`, `60→anaerobic`, `300→vo2max`, `1200→threshold (FT)`. The 1200-s best is ranked directly against the FT column as a documented 20-min proxy. Each anchor result carries `duration_s`, `watts`, `w_per_kg`, `category`, `percentile`, `workout_id`, `date`. An anchor with no best-effort in the window is omitted from `anchors` and listed in `missing_anchors` (the power-curve empty-window posture, per-duration).

- **Why not use CP as the FT anchor?** That would couple the profile to the CP model's own gates/degradation. The raw 20-min best is simpler, always available when the ride exists, and matches how the Coggan tool is used in practice.

### D2 — Category is primary, percentile is an interpolated estimate
The real Coggan output is a **category band** (e.g. "Cat 2 / Very good"). The change is titled "percentile ranking", so the response also carries a `percentile` (0–100) obtained by **linear interpolation between adjacent table rows**, mapping the table's full W/kg span onto a 0–100 scale (top row → ~100, bottom → ~0), clamped at the ends. It is labelled an estimate in the API description and the spec: the tables are category-calibrated against a loose reference population, not true percentiles, so the number is a smooth position-within-table, not a claim about a real distribution. Category (the honest output) and percentile (the requested framing) both ship; a consumer can trust the band and take the number as a gradient.

### D3 — Weight resolution mirrors the energy endpoint
`weight_kg` query param (>0) is the highest-trust source; absent, fall back to the **most-recent stored body-weight entry** (a `weightProvider` interface — `LatestWeight(ctx) (kg, error)` — injected into the service, wired to `bodyweight.Repo` in `httpserver.Run()`, the `workoutcompliance` narrow-interface pattern); absent both, `400 weight_data_missing`. The response echoes `weight_kg` and `weight_source` (`param` | `stored`) so the ranking's denominator is auditable. Latest (not in-window) weight is correct — the profile is a current-ability snapshot, and W/kg wants today's mass. `weight_kg ≤ 0` → `400 weight_kg_invalid`.

### D4 — `sex` selects the table, defaults male
`sex=male|female` picks the embedded table; omitted defaults to `male` (single-user pragmatism, documented); any other value → `400 sex_invalid`. Athlete-config has no sex field and this change does not add one (advisory posture; the param is the seam). The tables are embedded as a small Go literal (four columns × N category rows × two sexes), sourced from Coggan/Allen, each row `{category, w_per_kg_threshold...}`; a single `rankAnchor(wPerKg, column, sex) → (category, percentile)` does the lookup + interpolation, unit-tested against the published anchor values.

### D5 — Phenotype from relative anchor strength
Coggan's rider-type read: compare the four anchors' *relative* standing (their percentiles). Strong 5 s/1 min but weak 5 min/FT → `sprinter`; strong FT, weaker sprint → `time_trialist`; strong 5 min/FT at high W/kg, weaker sprint → `climber` (a.k.a. pursuiter/all-rounder per Coggan's quadrants); balanced → `all_rounder`. Computed by a pure `phenotype(anchors) → *string`, **null unless all four anchors are present** (the read needs the full profile to name a type). Fixed thresholds on the percentile spread, unit-tested; advisory, never persisted.

### D6 — Endpoint contract shared with the siblings
`GET /workouts/power-profile?from&to&tz` reuses `parseWindow` (same `range_required`/`date_invalid`/`range_invalid`/`range_too_large`+`max_days`/`tz_invalid`, ≥1yr, 400-day cap). Power metric only (no `sport` param). `numfmt` at the boundary (watts int, W/kg + percentile `Round1`). MCP `power_profile` (read tier, one GET, body verbatim; description states the FT-proxy + advisory framing). Compute-on-read; a GET mutates nothing.

## Risks / Trade-offs

- **"Percentile" overclaims precision** — mitigated by D2: category is the primary output, the number is labelled an interpolated estimate in the API/spec/tool description.
- **20-min-as-FT is optimistic** (20-min best > true 60-min FT) — accepted and documented; a rider who never does a 20-min max simply has that anchor missing rather than mis-ranked. No silent 0.95 correction.
- **Default `sex=male`** is a presumption — acceptable for a single-user tool with the param as the override; called out in the description.
- **Stale weight skews W/kg** — the latest stored entry can lag; the `weight_kg` param is the escape hatch and `weight_source` makes the denominator visible.
