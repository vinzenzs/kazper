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
