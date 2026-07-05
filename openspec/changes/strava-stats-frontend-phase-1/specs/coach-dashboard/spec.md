## MODIFIED Requirements

### Requirement: The dashboard presents a training-focused coach's-eye view (v1)

The dashboard's **home route (`/`)** SHALL present, from `GET /api/v1/context/training`,
`GET /api/v1/context/recovery`, and `GET /api/v1/fitness-metrics`: a header (current phase,
season name, days-to-race when race-anchored, and a `training_status` badge); a form/ACWR
indicator from the derived acute:chronic ratio; an acute/chronic training-load trend; a
**fitness panel** (VO₂max running and cycling, training status, endurance score, hill score,
fitness age); a **race-predictions panel** (5k, 10k, half, full as formatted race times); a
**power & thresholds panel** (FTP, watts/kg, threshold HR, max HR, lactate-threshold HR,
threshold pace, threshold swim pace); **HR and power zone strips** (the five `*_zone_*_max`
bands rendered visually with labeled boundaries); a recovery snapshot (HRV, sleep, sleep score,
resting HR, average stress, body battery, training readiness); and recent + upcoming workout
lists (sport, status, load where present, with a by-sport breakdown of recent load). Every
metric is nullable; the home route SHALL degrade gracefully when a value is absent (placeholder
or empty-state) rather than erroring. Fueling, energy-availability, and nutrition panels remain
explicitly out of scope on the home route. Additional routes MAY read other REST endpoints (see
the routing and route-specific requirements below); the "no new fetch" constraint applies only
to the home route's training composite.

#### Scenario: The training composite renders

- **WHEN** the dashboard home route (`/`) loads with a populated `GET /api/v1/context/training`
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

- **WHEN** the dashboard home route is loaded
- **THEN** no fueling / energy-availability / nutrition panels are shown

## ADDED Requirements

### Requirement: The dashboard is a multi-route SPA with client-side navigation

The dashboard SHALL use client-side routing so that distinct views live at distinct URL paths:
the training composite at `/`, personal records at `/records`, gear at `/gear`, and per-activity
detail at `/workouts/:id`. Navigation between the top-level views SHALL be available from the
header. Because the server already serves `index.html` for any non-API GET path and returns the
JSON 404 contract under `/api/v1`, client-side routing SHALL require no backend change: a
deep-linked or reloaded route SHALL resolve to the correct view. New views SHALL reuse the
existing analyst visual language (the `Panel` / `Stat` / zone-strip primitives and the muted
Tailwind + visx idiom); celebratory or social treatments (trophy cards, badge walls) SHALL NOT
be introduced.

#### Scenario: Header navigates between top-level routes

- **WHEN** the user clicks a header nav link (Dashboard, Records, or Gear)
- **THEN** the SPA navigates client-side to the corresponding route without a full page reload

#### Scenario: Deep-linking a client-side route resolves to its view

- **WHEN** the browser loads `/records`, `/gear`, or `/workouts/:id` directly (or reloads on it)
- **THEN** the server returns the SPA shell and the client router renders the matching view

#### Scenario: New views keep the analyst aesthetic

- **WHEN** any Phase-1 view renders
- **THEN** it uses the existing `Panel` / `Stat` / zone-strip primitives and muted idiom, with
  no trophy cards, badge walls, or other celebratory/social styling

### Requirement: The records route surfaces personal records and achievements

The `/records` route SHALL read `GET /api/v1/personal-records` and `GET /api/v1/achievements`
and present personal records (best efforts such as fastest 5k/10k, longest ride) as a dense
table showing PR type, value with its unit, and achieved-at date, plus a compact achievements
strip. A personal record whose row carries an `activity_id` SHALL link to the corresponding
workout detail route where the id resolves to a known workout. Empty datasets SHALL render an
empty-state rather than erroring.

#### Scenario: Records and achievements render

- **WHEN** `GET /api/v1/personal-records` and `GET /api/v1/achievements` return rows
- **THEN** the records route shows the PR table (type · value+unit · date) and the achievements
  strip

#### Scenario: No records yet

- **WHEN** `GET /api/v1/personal-records` returns an empty list
- **THEN** the records route shows an empty-state without erroring

### Requirement: The gear route surfaces gear inventory and mileage

The `/gear` route SHALL read `GET /api/v1/gear` and present each gear item with its type
(shoes / bike / other), name, and accumulated distance rendered as a labeled value with a
muted mileage bar. Retired gear SHALL be visually de-emphasized (dimmed) rather than hidden.
An empty inventory SHALL render an empty-state rather than erroring.

#### Scenario: Gear mileage renders

- **WHEN** `GET /api/v1/gear` returns gear items with distance
- **THEN** the gear route lists each item with its type, name, and mileage

#### Scenario: Retired gear is dimmed

- **WHEN** a gear item is marked retired
- **THEN** it is rendered de-emphasized (dimmed) rather than omitted

### Requirement: The workout detail route surfaces per-activity splits and zones

The `/workouts/:id` route SHALL read `GET /api/v1/workouts/:id` (the single-get that returns
`splits`, `sets`, and `secs_in_zone_*` — detail the list-shaped context payloads omit) and
present a per-activity view: the workout's summary metrics, a splits table (per-lap distance,
duration, pace/speed, HR, power where present), and an HR/power zone strip reusing the existing
zone-strip primitive. Rows in the home route's workout lists SHALL link into this route by
workout id. A workout with no splits SHALL still render its summary and omit the splits table
gracefully; an unknown id SHALL render a not-found state rather than erroring.

#### Scenario: Workout detail renders splits and zones

- **WHEN** the user opens `/workouts/:id` for a workout that has splits and zone time
- **THEN** the detail route shows the summary metrics, the splits table, and the zone strip

#### Scenario: Workout list rows link to detail

- **WHEN** the user clicks a workout row in the home route's recent or upcoming list
- **THEN** the SPA navigates to that workout's `/workouts/:id` detail route

#### Scenario: Workout without splits degrades gracefully

- **WHEN** `GET /api/v1/workouts/:id` returns a workout with no splits
- **THEN** the detail route renders the summary and omits the splits table without erroring
