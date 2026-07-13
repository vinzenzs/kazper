## ADDED Requirements

### Requirement: A secret-gated public race feed exposes the goal race outside the auth boundary

The system SHALL expose `GET /public/race-feed` on the root path, registered **outside** `auth.Middleware` (a sibling of `/healthz`), returning a curated, non-PII JSON projection of the athlete's goal race. The endpoint SHALL require an `X-Feed-Key` request header equal to the configured `FEED_SECRET`, compared in constant time; a missing or non-matching key SHALL be rejected with `401 feed_unauthorized` and no body data. The endpoint SHALL participate in no bearer-identity or role-based authorization — the secret gates this route only and grants access to nothing else. When `FEED_SECRET` is unset the endpoint SHALL be disabled and return `503 feed_disabled`. The response SHALL contain only non-PII fields (race name, race date, and a computed countdown) and SHALL NOT echo request input, ids, or any athlete health/nutrition data.

#### Scenario: A valid feed key returns the curated projection

- **WHEN** `GET /public/race-feed` is called with `X-Feed-Key` equal to `FEED_SECRET` and an active macrocycle anchored to a race exists
- **THEN** the response is `200 OK` with body shaped `{"race": {"name": "...", "race_date": "YYYY-MM-DD"}, "days_remaining": <int>}`
- **AND** the body contains no athlete health, nutrition, weight, or identity fields

#### Scenario: A missing or wrong feed key is rejected

- **WHEN** `GET /public/race-feed` is called without `X-Feed-Key`, or with a value that does not match `FEED_SECRET`
- **THEN** the response is `401 feed_unauthorized`
- **AND** no race data is returned

#### Scenario: The feed is disabled when no secret is configured

- **WHEN** `FEED_SECRET` is unset and `GET /public/race-feed` is called (with or without a key)
- **THEN** the response is `503 feed_disabled`

### Requirement: The feed resolves the goal race as the active macrocycle's A-race

The system SHALL resolve the feed's race as the A-race of the active macrocycle: the `macrocycle` whose `[start_date, end_date]` inclusive contains today (in the configured user timezone), breaking ties by the latest `start_date`, then its `race_id`. `days_remaining` SHALL be the whole-day difference from today to the race's `race_date` in the configured user timezone, floored at `0` on and after race day. When there is no active macrocycle, the active macrocycle has no `race_id`, or the referenced race does not exist, the endpoint SHALL return `200 OK` with `{"race": null, "days_remaining": null}` so a consuming page degrades gracefully rather than erroring.

#### Scenario: Resolves the active macrocycle's A-race with a countdown

- **WHEN** today falls within an active macrocycle whose `race_id` references a race dated in the future, and the feed is requested with a valid key
- **THEN** `race` carries that race's `name` and `race_date`
- **AND** `days_remaining` is the whole-day count from today to `race_date` in the user timezone

#### Scenario: On and after race day the countdown floors at zero

- **WHEN** the resolved race's `race_date` is today or in the past (still within the active macrocycle window)
- **THEN** `days_remaining` is `0`

#### Scenario: No active anchored race degrades to nulls

- **WHEN** no macrocycle contains today, or the active macrocycle has `race_id = null`, or the referenced race is missing, and the feed is requested with a valid key
- **THEN** the response is `200 OK` with `{"race": null, "days_remaining": null}`
