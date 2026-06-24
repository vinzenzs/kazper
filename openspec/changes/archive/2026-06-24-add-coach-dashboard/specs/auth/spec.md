## MODIFIED Requirements

### Requirement: Bearer token authentication with two static tokens

The system SHALL authenticate every request by an `Authorization` header. It SHALL accept the
`Bearer <token>` scheme, where `<token>` matches one of the env-configured static tokens, and
SHALL reject non-matching requests with `401 Unauthorized`. There are two required Bearer
identities — `MOBILE_API_TOKEN` (`client_id = "mobile"`) and `AGENT_API_TOKEN`
(`client_id = "agent"`) — and one OPTIONAL Bearer identity, `GARMIN_API_TOKEN`
(`client_id = "garmin"`), recognized only when configured.

The system SHALL additionally accept the `Basic <base64(user:pass)>` scheme for an OPTIONAL
`web` identity, used by the browser dashboard. When `WEB_USER` and `WEB_PASSWORD` are both set,
a Basic credential matching them (constant-time comparison) SHALL resolve to
`client_id = "web"` with full access (the same authorization as the `mobile` identity in v1;
no read-only restriction). The `web` identity SHALL be recognized only when both env vars are
configured. Basic auth transmits credentials as base64 (not encryption) on every request, so
the dashboard is intended to be reached over an encrypted transport (TLS or Tailscale); this is
an operational expectation, documented rather than enforced in code.

#### Scenario: Mobile token is accepted

- **WHEN** a request includes `Authorization: Bearer <value-of-MOBILE_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "mobile"`

#### Scenario: Agent token is accepted

- **WHEN** a request includes `Authorization: Bearer <value-of-AGENT_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "agent"`

#### Scenario: Garmin token is accepted when configured

- **WHEN** `GARMIN_API_TOKEN` is set and a request includes `Authorization: Bearer <value-of-GARMIN_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "garmin"`

#### Scenario: Web Basic credential is accepted when configured

- **WHEN** `WEB_USER` and `WEB_PASSWORD` are set and a request includes `Authorization: Basic <base64(WEB_USER:WEB_PASSWORD)>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "web"`
- **AND** the request has full access (no method restriction in v1)

#### Scenario: Web identity is not recognized when unset

- **WHEN** `WEB_USER`/`WEB_PASSWORD` are unset and a request presents some Basic credential
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_invalid"}` (no web identity exists)

#### Scenario: Garmin token is not recognized when unset

- **WHEN** `GARMIN_API_TOKEN` is unset and a request presents some bearer value as the garmin token
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_invalid"}` (no garmin identity exists)

#### Scenario: Missing Authorization header is rejected

- **WHEN** a request has no `Authorization` header
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_required"}`
