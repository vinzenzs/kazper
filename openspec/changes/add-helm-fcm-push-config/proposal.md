## Why

The `add-garmin-relogin-push` change shipped opt-in FCM push in the binary (`FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON`, validated at startup, redacted in dumps), but the Helm chart was never updated to expose those keys. A chart-deployed pod therefore *always* runs with push disabled — the push surface returns `503 push_disabled` and the relogin notification is a silent no-op, with no value to turn it on. The feature is unreachable in the only environment it was built for (the personal Kubernetes cluster). This closes that gap so push can be enabled the same way Garmin is: opt-in via chart values.

## What Changes

- **ConfigMap gains `FCM_PROJECT_ID`.** Rendered from a new non-secret `config.fcmProjectId` value, conditionally (only emitted when set), mirroring the existing `garminBridgeUrl` pattern so an unset project id leaves the env var absent rather than empty.
- **Secret gains `FCM_SERVICE_ACCOUNT_JSON`.** Rendered from a new `secrets.fcmServiceAccountJson` value into the chart-managed Secret, conditionally (only when set), mirroring the opt-in `GARMIN_*` block. When `config.fcmProjectId` is set the value is **required** (the binary fails fast at startup otherwise) — `helm install` blocks with a clear message rather than letting the pod crash-loop on the same validation.
- **`existingSecret` documentation updated.** The doc comment listing the keys an externally-managed Secret must carry gains `FCM_SERVICE_ACCOUNT_JSON` (optional, push opt-in).
- **`values.yaml` defaults + comments.** Empty `config.fcmProjectId` and `secrets.fcmServiceAccountJson` with comments explaining the both-or-neither gating and that leaving both empty keeps push disabled (`503 push_disabled`).
- **Chart README.** A short "Push notifications (opt-in)" note documenting the two values, the both-required-together rule, and the disabled-by-default behavior.

No application code, migrations, or API surface change — this is purely the deployment value surface catching up to a shipped feature.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `deployment-pipeline`: the Helm chart's value surface gains opt-in FCM push configuration — `config.fcmProjectId` → `FCM_PROJECT_ID` in the ConfigMap and `secrets.fcmServiceAccountJson` → `FCM_SERVICE_ACCOUNT_JSON` in the Secret (or an externally-managed Secret), with `helm install` failing when a project id is set but the service-account credential is missing.

## Impact

- **Chart templates**: `deploy/helm/kazper/templates/configmap.yaml` (conditional `FCM_PROJECT_ID`), `deploy/helm/kazper/templates/secret.yaml` (conditional `FCM_SERVICE_ACCOUNT_JSON` with `required` guard).
- **Chart values/docs**: `deploy/helm/kazper/values.yaml` (`config.fcmProjectId`, `secrets.fcmServiceAccountJson`, `existingSecret` key-list comment), `deploy/helm/kazper/README.md` (push opt-in note).
- **No** Go code, migration, `docs/` (swag), or MCP changes. The config keys the chart now feeds already exist and are validated by `internal/config`.
- Single-user, opt-in posture preserved: with both values empty the chart renders exactly as today and push stays inert.
