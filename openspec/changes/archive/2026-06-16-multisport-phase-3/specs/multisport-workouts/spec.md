## ADDED Requirements

### Requirement: A multisport template exposes a derived total duration

A multisport template's read response (single GET and list) SHALL include a
derived `estimated_duration_sec`: the sum of every **sport** segment's
time-bounded step durations plus every **transition** segment's time duration.
The value SHALL be computed on read from the stored segments (not persisted), and
SHALL be **omitted/null** when the total is not fully determinable — that is, when
any sport segment contains a step that is not time-bounded
(`distance`/`lap_button`/`open`) or any transition segment's duration is not of
kind `time`. The presence or value of this field SHALL NOT change how the
template is validated, stored, scheduled, or compiled.

#### Scenario: A fully time-bounded template reports its summed duration

- **WHEN** a multisport template whose every sport segment uses time-bounded steps
  and whose transitions carry `time` durations is read
- **THEN** the response includes `estimated_duration_sec` equal to the sum of all
  segment step durations plus the transition durations

#### Scenario: A non-time-bounded segment yields a null duration

- **WHEN** a multisport template has a sport segment with a `distance`,
  `lap_button`, or `open` step (or a transition with a non-`time` duration) and is read
- **THEN** `estimated_duration_sec` is omitted/null in the response

#### Scenario: The derived duration does not affect validation or scheduling

- **WHEN** a template with a null derived duration is scheduled or referenced by a plan slot
- **THEN** it compiles/materializes exactly as before (the field is read-only metadata)
