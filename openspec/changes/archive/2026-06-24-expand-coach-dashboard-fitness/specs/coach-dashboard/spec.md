## MODIFIED Requirements

### Requirement: The dashboard presents a training-focused coach's-eye view (v1)

The dashboard SHALL be training-only and SHALL present, from `GET /api/v1/context/training`,
`GET /api/v1/context/recovery`, and `GET /api/v1/fitness-metrics`: a header (current phase,
season name, days-to-race when race-anchored, and a `training_status` badge); a form/ACWR
indicator from the derived acute:chronic ratio; an acute/chronic training-load trend; a
**fitness panel** (VO₂max running and cycling, training status, endurance score, hill score,
fitness age); a **race-predictions panel** (5k, 10k, half, full as formatted race times); a
**power & thresholds panel** (FTP, watts/kg, threshold HR, max HR, lactate-threshold HR,
threshold pace, threshold swim pace); **HR and power zone strips** (the five `*_zone_*_max`
bands rendered visually with labeled boundaries); a recovery snapshot (HRV, sleep, sleep score,
resting HR, average stress, body battery, training readiness); and recent + upcoming workout
lists (sport, status, load where present, with a by-sport breakdown of recent load). All of
these fields are already carried by the existing context payloads — no new endpoint or fetch is
introduced. Every metric is nullable; the dashboard SHALL degrade gracefully when a value is
absent (placeholder or empty-state) rather than erroring. Fueling, energy-availability, and
nutrition panels remain explicitly out of scope.

#### Scenario: The training composite renders

- **WHEN** the dashboard loads with a populated `GET /api/v1/context/training`
- **THEN** it shows the phase/season/days-to-race header with the training-status badge, the
  ACWR/form indicator, the acute/chronic load trend, the fitness, race-predictions, and
  power/threshold panels, the HR and power zone strips, and the recent + upcoming workout lists

#### Scenario: Recovery snapshot renders extended metrics

- **WHEN** `GET /api/v1/context/recovery` returns a latest snapshot
- **THEN** the dashboard shows the recovery panel including HRV, sleep, sleep score, resting HR,
  average stress, body battery, and training readiness where present

#### Scenario: Zones render as labeled bands

- **WHEN** the athlete config carries HR and/or power zone boundaries
- **THEN** the dashboard renders them as banded Z1–Z5 strips with each boundary labeled,
  showing only the bands whose boundaries are present

#### Scenario: Missing metrics degrade gracefully

- **WHEN** a context payload has null fields (e.g. no VO₂max, no zones, or no ACWR)
- **THEN** the corresponding panel, stat, or zone band shows an empty/placeholder state without
  erroring

#### Scenario: Fueling panels are absent

- **WHEN** the dashboard is loaded
- **THEN** no fueling / energy-availability / nutrition panels are shown
