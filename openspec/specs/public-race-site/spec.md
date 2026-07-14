# public-race-site Specification

## Purpose
TBD - created by archiving change add-public-race-site. Update Purpose after archive.
## Requirements
### Requirement: The public site is built statically from the race feed with no runtime coupling to Kazper

The public "road to race" site SHALL live at `apps/public/` and SHALL be produced
as pure static files by a build that fetches `GET /public/race-feed` exactly once,
authenticating with the `X-Feed-Key` header sourced from build-provided
environment (`FEED_SECRET`, endpoint from `FEED_URL`). Those static files are
baked into a container image for serving. The shipped assets — **and every layer
of the container image built from them** — SHALL NOT contain the feed secret or
the Kazper origin, and the served page SHALL NOT make any runtime request to the
Kazper origin: the page renders only fields the feed provided at build time (plus
the client-side countdown derived from them), and the running container holds no
feed secret and makes no call to Kazper. A build whose feed fetch fails or returns
a non-200 status SHALL fail (leaving the previously deployed image serving) rather
than publish a page with missing data.

#### Scenario: Build renders the feed's race into the page

- **WHEN** the site is built while the feed returns `{"race": {"name": "...", "race_date": "YYYY-MM-DD"}, "days_remaining": N}`
- **THEN** the output contains a page with the race name, the race date, and a prerendered countdown of `N` days

#### Scenario: Shipped assets leak no secret and never call Kazper

- **WHEN** the built static output and every layer (and the build history) of the
  produced container image are searched for the configured secret value and for
  the Kazper origin URL
- **THEN** neither appears in any shipped asset or image layer
- **AND** loading the served page performs no network request to the Kazper origin

#### Scenario: A failed feed fetch fails the build

- **WHEN** the build runs and the feed request errors or returns a non-200 status
- **THEN** the build exits non-zero and no new image is published or rolled out

### Requirement: The countdown stays correct between rebuilds

The page SHALL embed the feed's `race_date` and compute the displayed days-remaining client-side in the viewer's local timezone as a whole-day calendar difference floored at `0`, with the build-time `days_remaining` prerendered as the no-JS fallback. A stale build therefore misstates at most which race is featured, never the number of days remaining.

#### Scenario: A stale build still shows today's countdown

- **WHEN** the page was built on an earlier day (prerendered countdown `N`) and is viewed today with JavaScript enabled
- **THEN** the displayed countdown reflects the calendar days from the viewer's current date to `race_date`, not `N`

#### Scenario: Without JavaScript the prerendered fallback shows

- **WHEN** the page is viewed with JavaScript disabled
- **THEN** the build-time `days_remaining` is visible as the countdown

#### Scenario: On and after race day the countdown floors at zero

- **WHEN** the viewer's current date is on or after `race_date`
- **THEN** the displayed countdown is `0`

### Requirement: The between-seasons null state renders a designed page

When the feed returns `{"race": null, "days_remaining": null}`, the build SHALL succeed and render a deliberate off-season state — same site shell, an explicit "between seasons" face instead of a countdown, and a neutral share card — rather than an error page or empty output.

#### Scenario: Null feed builds the off-season page

- **WHEN** the site is built while the feed returns `{"race": null, "days_remaining": null}`
- **THEN** the build succeeds and the page shows the off-season state with no countdown and no race name

### Requirement: The page carries a build-time-generated share card

The build SHALL generate an Open Graph card image containing the race name and the build-time countdown (or the neutral off-season variant when the race is null), and the page's head SHALL carry the corresponding `og:title`, `og:description`, and `og:image` metadata referencing that generated image, so a shared link previews meaningfully in chat/social contexts.

#### Scenario: Page head references the generated card

- **WHEN** the built page's HTML head is inspected after a build with an active race
- **THEN** it contains `og:image` metadata pointing at a build-generated image asset
- **AND** `og:title`/`og:description` reflect the race name and countdown

### Requirement: A scheduled pipeline rebuilds and deploys the site

A dedicated GitHub Actions workflow SHALL build the site into a container image,
push it to the container registry, and roll the in-cluster Deployment onto the
freshly-built image, triggered by: a nightly schedule (timed to land shortly
after midnight in the athlete's timezone, so the prerendered fallback and share
card roll over daily), manual dispatch, and pushes touching `apps/public/**`. The
feed secret SHALL be supplied only at **image-build time** (as a build secret,
kept out of every image layer) and never to the running pod. A failed workflow
run SHALL leave the previously-deployed image serving — the Deployment SHALL NOT
be rolled onto a build that did not succeed.

#### Scenario: Nightly rebuild refreshes the prerendered countdown

- **WHEN** the scheduled run executes after midnight in the athlete's timezone
- **THEN** the site is rebuilt against the live feed into a new image, the image
  is pushed, and the Deployment is rolled onto it, so the deployed prerendered
  countdown and share card reflect the new day

#### Scenario: Manual dispatch rebuilds on demand

- **WHEN** the workflow is manually dispatched (e.g. after the season's
  macrocycle/race anchoring changes)
- **THEN** the site is rebuilt, pushed, and rolled out without waiting for the
  nightly schedule

#### Scenario: A failed build leaves the running image serving

- **WHEN** the build step fails (feed non-200, network error, or a build error)
- **THEN** no new image is pushed and the Deployment is not rolled
- **AND** the previously-deployed image continues serving

