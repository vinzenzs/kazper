# public-race-site Specification (delta)

## MODIFIED Requirements

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

#### Scenario: Neither the static output nor the image layers leak the secret, and the page never calls Kazper

- **WHEN** the built static output and every layer (and the build history) of the
  produced container image are searched for the configured secret value and for
  the Kazper origin URL
- **THEN** neither appears in any shipped asset or image layer
- **AND** loading the served page performs no network request to the Kazper origin

#### Scenario: A failed feed fetch fails the build

- **WHEN** the build runs and the feed request errors or returns a non-200 status
- **THEN** the build exits non-zero and no new image is published or rolled out

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
