## ADDED Requirements

### Requirement: Helm chart exposes opt-in FCM push configuration

The chart SHALL expose Android push (FCM HTTP v1) configuration as opt-in chart values, gated the same way the binary gates it: push is enabled only when BOTH the project id and the service-account credential are provided. The non-secret `FCM_PROJECT_ID` SHALL be rendered into the `ConfigMap` from `config.fcmProjectId`, and only when that value is non-empty (so an unset project id leaves the env var absent, not empty). The secret `FCM_SERVICE_ACCOUNT_JSON` SHALL be rendered into the chart-managed `Secret` from `secrets.fcmServiceAccountJson`, and only when that value is non-empty. When `config.fcmProjectId` is set but `secrets.fcmServiceAccountJson` is empty (and no `existingSecret` is referenced), `helm install` SHALL fail with a clear error naming the missing value, because the binary requires the credential at startup whenever the project id is set.

#### Scenario: Push disabled by default

- **WHEN** the chart is installed with default values (neither `config.fcmProjectId` nor `secrets.fcmServiceAccountJson` set)
- **THEN** the rendered `ConfigMap` contains no `FCM_PROJECT_ID` key
- **AND** the rendered `Secret` contains no `FCM_SERVICE_ACCOUNT_JSON` key
- **AND** the pod runs with push inert, which the existing push code path handles by returning `503 push_disabled`

#### Scenario: Both values enable push

- **WHEN** the chart is installed with `config.fcmProjectId=my-proj` and `secrets.fcmServiceAccountJson` set to valid service-account JSON
- **THEN** the rendered `ConfigMap` has `FCM_PROJECT_ID: my-proj`
- **AND** the rendered `Secret` has `FCM_SERVICE_ACCOUNT_JSON` carrying the credential

#### Scenario: Project id without credential blocks install

- **WHEN** the chart's Secret is being rendered (no `existingSecret`) with `config.fcmProjectId` set but `secrets.fcmServiceAccountJson` empty
- **THEN** `helm install` fails with a clear error naming `secrets.fcmServiceAccountJson` as required when `config.fcmProjectId` is set

#### Scenario: Externally managed Secret can carry the credential

- **WHEN** the user installs the chart with `existingSecret=my-tokens` and sets `config.fcmProjectId`
- **THEN** the chart does NOT render a `Secret` of its own and does NOT block on `secrets.fcmServiceAccountJson`
- **AND** the chart README documents that the external Secret must carry `FCM_SERVICE_ACCOUNT_JSON` for push to work
