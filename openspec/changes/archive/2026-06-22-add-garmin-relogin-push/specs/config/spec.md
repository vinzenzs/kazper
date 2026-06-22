## ADDED Requirements

### Requirement: FCM push configuration is opt-in, gated, and redacted

The loader SHALL recognize two optional push-notification environment variables:
`FCM_PROJECT_ID` (string) and `FCM_SERVICE_ACCOUNT_JSON` (string — inline service-account
JSON or a path to a JSON file). Push is enabled only when BOTH are set; when either is unset
the push surface is disabled (`503 push_disabled` on operations that require the sender, and
the relogin notification is a no-op). When `FCM_SERVICE_ACCOUNT_JSON` is set, the loader
SHALL validate that it resolves to parseable service-account JSON and return an error
naming the variable otherwise. The loader and any configuration-related logging SHALL NOT
emit the raw service-account JSON; diagnostic output SHALL indicate presence/absence only.

#### Scenario: Push disabled when keys are unset

- **WHEN** the loader is called without `FCM_PROJECT_ID` and `FCM_SERVICE_ACCOUNT_JSON`
- **THEN** config loads successfully with push reported as disabled

#### Scenario: Both keys enable push

- **WHEN** both `FCM_PROJECT_ID` and a valid `FCM_SERVICE_ACCOUNT_JSON` are set
- **THEN** the resolved config reports push as enabled

#### Scenario: Invalid service-account JSON is rejected

- **WHEN** `FCM_SERVICE_ACCOUNT_JSON` is set but does not resolve to parseable service-account JSON
- **THEN** the loader returns an error naming `FCM_SERVICE_ACCOUNT_JSON`

#### Scenario: Service-account JSON is redacted in diagnostics

- **WHEN** any config loader, startup, or error log line references push configuration
- **THEN** it contains no substring of the raw `FCM_SERVICE_ACCOUNT_JSON` value
