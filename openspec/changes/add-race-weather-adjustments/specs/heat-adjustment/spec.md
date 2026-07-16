## ADDED Requirements

### Requirement: Heat analytics quantify performance degradation across history

The system SHALL expose `GET /api/v1/workouts/heat-analytics?from=&to=&tz=` over outdoor
completed workouts carrying temperature data: sessions bucketed by session heat index
(< 20 / 20–25 / 25–30 / > 30 °C), each bucket reporting session count, mean duration, mean EF,
mean decoupling, and mean output (power or pace) relative to the window baseline; plus Spearman
correlations (EF vs heat index, decoupling vs heat index) gated at 10 pairs
(`insufficient_pairs` per metric below it — the wellness-correlation posture). Workouts with a
null environment SHALL count with an `assumed_outdoor` tally; indoor SHALL be excluded. The
read SHALL be compute-on-read, persist nothing, use the shared range vocabulary (400-day cap),
and exists explicitly as the evidence stream for refining the heat-adjustment constants.

#### Scenario: A season shows the degradation gradient

- **WHEN** the window holds outdoor sessions across all four buckets
- **THEN** each bucket reports its means and the correlations carry rho with their pair counts

#### Scenario: Thin heat exposure gates the correlation

- **WHEN** only 6 sessions exceed 25 °C heat index
- **THEN** bucket means still report and the correlations return `insufficient_pairs` where
  pairs fall short

### Requirement: Heat analytics are readable over MCP

The system SHALL expose a `heat_analytics` MCP tool (read tier, one GET, verbatim). The
description SHALL note the duration confound (hot sessions skew long) and that findings inform
proposed constant refinements, not automatic ones.

#### Scenario: The agent reads the heat evidence in one call

- **WHEN** the agent invokes `heat_analytics` over the season
- **THEN** one GET is issued and the buckets and correlations return verbatim
