## MODIFIED Requirements

### Requirement: An opaque garmin token blob is stored encrypted at rest

The system SHALL store a single garmin authentication token blob, supplied
verbatim by the garmin-bridge, encrypted at rest with a configured key
(`GARMIN_TOKEN_ENC_KEY`, AES-256-GCM). The blob is opaque: the system MUST NOT
parse, refresh, validate, or otherwise interpret its contents, and MUST return
it byte-identical on read. There SHALL be at most one stored blob (single user).
The plaintext blob MUST NOT appear in any log line.

Storing a token (`POST/PUT /garmin/token`) SHALL clear the Garmin relogin latch
(see the `push-notifications` capability), so that re-authenticating after an
outage stops the relogin reminder and a future lapse can notify again. Clearing
the latch SHALL NOT affect the stored-verbatim guarantee and SHALL NOT fail the
token store if the latch update errors.

#### Scenario: Store then read returns the blob verbatim

- **WHEN** the garmin client `PUT`s `/garmin/token` with a blob
- **THEN** the blob is encrypted and persisted (replacing any prior value)
- **AND** a subsequent `GET /garmin/token` returns the exact same bytes

#### Scenario: Storing a token clears the relogin latch

- **WHEN** the garmin client `PUT`s `/garmin/token` with a fresh blob
- **THEN** the Garmin relogin latch is cleared so the next distinct outage can notify

#### Scenario: Reading when nothing is stored

- **WHEN** `GET /garmin/token` is called and no blob has been stored
- **THEN** the response is `404 garmin_token_not_found`

#### Scenario: Clearing forces re-login

- **WHEN** the garmin client `DELETE`s `/garmin/token`
- **THEN** the stored blob is removed
- **AND** a subsequent `GET` returns `404 garmin_token_not_found`

#### Scenario: At rest the blob is ciphertext, not plaintext

- **WHEN** the `garmin_tokens` row is inspected directly in the database
- **THEN** the stored bytes are the ciphertext, not the supplied blob
- **AND** decryption requires `GARMIN_TOKEN_ENC_KEY` (absent from the database)
