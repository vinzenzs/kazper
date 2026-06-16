## MODIFIED Requirements

### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly five screens (Today, Train,
Camera, Recent, Chat) plus a Settings sheet and a shopping list screen reachable
from the Today header. The Chat screen is the in-app coach backed by the server's
`nutrition-chat` capability — it spans nutrition planning and endurance-training
coaching, including the full write surface, with consequential (training/goal/
destructive) writes gated behind an in-app confirmation. The **Train** screen is
the structured, glanceable view of training **as a fueling lens** (see the Train
screen requirements). The app SHALL NOT include an in-app recipe builder or a
generalized product search experience.

#### Scenario: Five screens in the bottom navigation

- **WHEN** the user opens the app
- **THEN** the bottom navigation surfaces Today, Train, Camera, Recent, and Chat
  as the five primary destinations
- **AND** Settings and the shopping list are reachable from the Today screen's top-right
- **AND** no recipe builder or all-products search screen exists

#### Scenario: Chat is the coach

- **WHEN** the user asks the Chat screen a training, recovery, or fueling question
- **THEN** the app renders the assistant's grounded coaching reply as ordinary chat content
- **AND** the assistant is not limited to nutrition planning (no redirect-to-desktop-coach behavior is expected)

## ADDED Requirements

### Requirement: The Train screen is a fueling lens on training, not a training tracker

Every element rendered on the Train screen SHALL have a fueling consequence — it
SHALL exist to inform or capture how the athlete fuels around training. The Train
screen SHALL NOT provide session scheduling, structured-workout authoring/editing,
completion ("mark done") toggles, or training-history analytics (laps, splits,
power curves) that carry no fueling consequence; session execution and history
remain the responsibility of the watch / Garmin / Strava. This invariant SHALL
gate what may be added to the screen.

#### Scenario: A training element without a fueling consequence is out of scope

- **WHEN** a candidate Train-screen element conveys training information but does
  not inform or capture fueling (e.g. a lap-split table or a "reschedule session"
  control)
- **THEN** it is not part of the Train screen (it belongs to the watch/Garmin surface)

#### Scenario: The Train screen does not execute or re-log sessions

- **WHEN** the user views a prescribed session on the Train screen
- **THEN** the screen offers no control to mark it done, edit its steps, or move it
- **AND** completion still arrives via the normal Garmin sync / reconciliation path

### Requirement: The Train screen leads with the day's session and its fuel

The Train screen SHALL lead with today's prescribed session(s) and the fueling
that session demands. For each of today's sessions it SHALL show the sport,
duration, planned time, and the session's resolved targets (read from the
effective-program endpoint, e.g. an absolute power/HR range with its origin zone,
or per-segment targets for a multisport session), together with the
**pre / during / post** fueling the session requires (read from the workout-fuel
recommendation). Reads SHALL follow the app's stale-while-revalidate pattern (no
offline banner). When today has no session, the screen SHALL render a minimal
rest-day state oriented to recovery fueling rather than a blank screen.

#### Scenario: Today's session shows its targets and its fuel

- **WHEN** the user opens the Train screen on a day with a prescribed endurance ride
- **THEN** the screen shows the ride's sport, duration, planned time, and resolved
  target (e.g. `230–268W (Z4)`)
- **AND** it shows the pre / during / post fueling that session demands

#### Scenario: A multisport session shows per-segment targets

- **WHEN** the prescribed session is a multisport (brick/triathlon) workout
- **THEN** the screen shows its ordered segments with each segment's resolved target

#### Scenario: A rest day shows a recovery-fueling state, not a blank screen

- **WHEN** today has no prescribed session
- **THEN** the Train screen shows a minimal rest-day state oriented to recovery fueling

### Requirement: The Train screen is read-only in v1; any future write is fueling-only through the outbox

The Train screen SHALL be read-only in v1 — it surfaces the session and its
recommended fueling but initiates no writes. Should a write ever be added, it
SHALL be a fueling write (e.g. logging workout fuel or during-session hydration)
flowing through the app's offline-first outbox with a client-minted idempotency
key, exactly as other companion writes. The Train screen SHALL NOT initiate
training-state writes (scheduling, status changes, structured-set edits) in any
version.

#### Scenario: The v1 Train screen offers no write

- **WHEN** the user interacts with a session on the Train screen in v1
- **THEN** no control mutates any state — the screen only reads and displays

#### Scenario: No training-state write is offered

- **WHEN** any future fueling action is added to the Train screen
- **THEN** it goes through the outbox, and no control mutates the session's
  schedule, status, or structure
