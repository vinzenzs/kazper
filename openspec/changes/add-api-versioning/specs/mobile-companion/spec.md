## MODIFIED Requirements

### Requirement: Pairing flow uses a QR code printed by the backend

The app's first-run experience SHALL be a pairing screen that scans a QR code. The QR payload SHALL be JSON of the shape `{"base_url": "<url>", "token": "<bearer>"}`, where `base_url` includes the `/api/v1` version prefix (e.g. `http://host:8080/api/v1`). The backend SHALL provide `task dev:pair` (and `task prod:pair`) helpers that print this QR (e.g. via `qrencode` to the terminal) using the value of `MOBILE_API_TOKEN`, the configured `HTTP_ADDR`, and the `/api/v1` base path. Devices paired before the version cutover MUST re-pair.

#### Scenario: First-run scan succeeds

- **WHEN** the user opens the app for the first time and scans the QR from `task dev:pair`
- **THEN** the JSON payload is parsed, `base_url` is validated as a syntactically valid URL that includes `/api/v1`, and `token` is non-empty
- **AND** both values are persisted (token to secure storage, base_url to preferences)
- **AND** subsequent API calls join their relative paths against the `/api/v1` base
- **AND** the app navigates to Today

#### Scenario: Malformed pairing payload is rejected

- **WHEN** the scanned QR contains malformed JSON or missing fields
- **THEN** the pairing screen shows an inline error
- **AND** does not persist anything
- **AND** prompts the user to try again
