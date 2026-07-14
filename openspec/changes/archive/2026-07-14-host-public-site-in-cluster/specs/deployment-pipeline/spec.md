# deployment-pipeline Specification (delta)

## ADDED Requirements

### Requirement: The chart optionally serves the public race site as an isolated static container

The Helm chart SHALL support hosting the public race site in-cluster behind a
`publicSite` values block, disabled by default. When `publicSite.enabled` is
`true`, the chart SHALL render a `Deployment` running the public-site nginx image
(`publicSite.image`), a `Service` fronting it, and a **dedicated Ingress** for
`publicSite.host` using the chart's existing ingress-nginx + cert-manager
assumptions (TLS). That Ingress SHALL route only to the public-site `Service` and
SHALL NOT expose the API `Service`, the Garmin-bridge, or any Kazper route — the
public workload is a static file server with no path to the application. The
public-site pod SHALL carry **no feed secret** in its environment or mounts (its
content is baked at image-build time). When `publicSite.enabled` is `false` (the
default), the chart SHALL render no public-site objects, leaving an existing
install unchanged. The API Deployment, Service, and Ingress SHALL be unaffected
by the `publicSite` block in either state.

#### Scenario: Disabled by default renders nothing

- **WHEN** the chart is rendered with default values (`publicSite.enabled` unset
  or `false`)
- **THEN** no public-site Deployment, Service, or Ingress appears in the output
- **AND** the API Deployment/Service/Ingress render exactly as before

#### Scenario: Enabled renders the isolated public workload

- **WHEN** the chart is rendered with `publicSite.enabled: true`, an image, and a
  `publicSite.host`
- **THEN** a public-site Deployment, Service, and an Ingress for that host are
  rendered, with TLS via the configured cluster issuer

#### Scenario: The public ingress has no route to the API

- **WHEN** the public-site Ingress is rendered
- **THEN** its only backend is the public-site Service
- **AND** no rule on it targets the API Service or exposes a Kazper API path

#### Scenario: The public-site pod holds no feed secret

- **WHEN** the public-site Deployment is rendered
- **THEN** its pod spec contains no `FEED_SECRET` (or feed-key) environment
  variable or secret mount — the site content was baked at image-build time
