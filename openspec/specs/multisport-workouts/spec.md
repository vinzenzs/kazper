# multisport-workouts Specification

## Purpose

Define multisport workout templates — a single continuous session composed of an ordered set of per-sport segments (e.g. swim → T1 → bike → T2 → run) plus transition segments — so a triathlon or brick can be authored once and pushed to the watch as one structured Garmin workout that auto-advances through its segments. This is distinct from single-sport `workout-templates`: each segment carries its own sport and step program, validated under that segment's sport rules, and the whole template compiles to one Garmin multisport workout object. The capability exposes authenticated REST CRUD (mirrored 1:1 by MCP tools) and a compile-and-schedule action.
## Requirements
### Requirement: A multisport template is an ordered set of per-sport segments

The system SHALL store a multisport template as a name plus an ordered, non-empty
array of **segments**. Each segment SHALL carry a `sport` and, for non-transition
segments, a step program in the existing workout-templates step model (single
steps and repeat groups). A segment whose `sport` is `transition` SHALL carry a
single `duration` (`time`/`lap_button`/`open`) and no steps or targets. A
multisport template SHALL contain at least two non-transition segments. Each
segment's steps SHALL be validated by the existing step validator **under that
segment's sport**, so per-sport target rules apply (e.g. a swim segment accepts
`swim_pace`, a bike segment may carry a `secondary_target`). The system SHALL
reject malformed templates with a sentinel error mapped to a 1:1 API error code.

#### Scenario: A triathlon template is accepted

- **WHEN** a multisport template is created with segments
  `[swim {steps…}, transition T1 lap_button, bike {steps…}, transition T2 lap_button, run {steps…}]`
- **THEN** the template persists and is returned with a generated id and the
  segments echoed verbatim

#### Scenario: Fewer than two sport segments is rejected

- **WHEN** a multisport template is created with a single non-transition segment
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: A segment's steps are validated under its sport

- **WHEN** a swim segment supplies a `pace` (`/km`) target, or a run segment
  supplies a `secondary_target`
- **THEN** the response is a validation error (per-sport step validation) and
  nothing is persisted

#### Scenario: A transition segment carries only a duration

- **WHEN** a `transition` segment supplies steps or a target
- **THEN** the response is a validation error and nothing is persisted

### Requirement: REST and MCP surface for multisport templates

The system SHALL expose authenticated REST CRUD for multisport templates and
mirror it 1:1 in the MCP server (one HTTP call per tool). The MCP integration
expected-tools list SHALL include the new multisport tools.

#### Scenario: Create and read a multisport template over REST

- **WHEN** a client `POST`s a valid multisport template and then `GET`s it by id
- **THEN** the stored segments are returned verbatim

#### Scenario: MCP tools mirror the REST surface

- **WHEN** the agent calls the multisport create tool
- **THEN** the MCP server issues exactly one corresponding HTTP request and
  forwards the response verbatim

### Requirement: A multisport template compiles and schedules to Garmin as one workout

The system SHALL expose an action that compiles a multisport template into a
single Garmin multisport workout (via the bridge) and schedules it on a calendar
date, reusing the existing Garmin library/calendar mechanics. The pushed workout
SHALL be one Garmin workout object containing all segments in order, not multiple
separate workouts.

#### Scenario: Scheduling pushes a single multisport workout

- **WHEN** a client schedules a multisport template to a date
- **THEN** the bridge creates one multisport Garmin workout and it is placed on
  that calendar date
- **AND** the response carries the Garmin workout id

### Requirement: A multisport template can be scheduled through a training-plan slot

The system SHALL allow a training-plan slot to reference a multisport template
(via `plan_slots.multisport_template_id`), so a triathlon/brick can sit on a plan
day like any single-sport session. When such a slot is materialized, the plan
SHALL emit a `multisport` planned workout (per the training-plan and workouts
requirements), and that planned workout SHALL reach the watch through the same
effective-program → Garmin push path as single-sport planned workouts — compiling
to one multi-segment Garmin workout via the bridge's multisport payload. This is
in addition to the existing direct compile-and-schedule action; both paths
produce a single multisport Garmin workout from the same template. Deleting a
multisport template referenced by any plan slot SHALL be prevented (the slot
reference is `ON DELETE RESTRICT`).

#### Scenario: A planned multisport slot pushes through the plan path

- **WHEN** a plan slot references a multisport template, the plan is materialized,
  and the resulting planned workout is pushed to Garmin
- **THEN** the bridge receives the multisport form (segments in order) and creates
  one multisport Garmin workout placed on the planned date

#### Scenario: A referenced multisport template cannot be deleted

- **WHEN** a delete is attempted on a multisport template that a plan slot
  references
- **THEN** the delete is rejected (ON DELETE RESTRICT) and the template persists

#### Scenario: The direct schedule action still works independently

- **WHEN** a multisport template is scheduled via the direct
  `POST /garmin/schedule/multisport` action (no plan slot)
- **THEN** it compiles and schedules as in Phase 1, unaffected by the plan path

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

### Requirement: Template reads carry per-segment estimated durations

Multisport template read responses (single and list, over REST and the MCP tools that forward
them) SHALL carry on each **sport segment** a derived, response-only `estimated_duration_sec` —
the sum of that segment's time-bound step durations with repeat blocks expanded, `null` when the
segment contains any non-time-bound step — using the same derivation rule as the existing
template-level value, which SHALL remain unchanged. The field SHALL NOT be writable or stored;
transition durations keep their existing explicit representation.

#### Scenario: Fully time-bounded segments report their estimates

- **WHEN** a swim/bike/run template's segments are each fully time-bounded
- **THEN** each segment carries its own `estimated_duration_sec` and the template-level value
  equals their sum plus no contribution from transitions beyond existing semantics

#### Scenario: An unbounded segment is null without hiding its siblings

- **WHEN** the bike segment contains a distance-bound step while swim and run are time-bounded
- **THEN** the bike segment's `estimated_duration_sec` is null, swim and run report values, and
  the template-level value is null as today

#### Scenario: The field is response-only

- **WHEN** a template write supplies `estimated_duration_sec` on a segment
- **THEN** it is ignored (never stored), and reads continue to derive it

