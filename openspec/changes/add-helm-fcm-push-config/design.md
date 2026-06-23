## Context

`add-garmin-relogin-push` (archived 2026-06-22) added an opt-in FCM HTTP v1 sender to the binary, gated on two config keys read by Viper: `FCM_PROJECT_ID` (non-secret) and `FCM_SERVICE_ACCOUNT_JSON` (secret — inline JSON or a path). `internal/config` validates the credential at startup whenever it is non-empty, and `Config.PushEnabled()` additionally requires the project id; with either unset the push surface is inert (`503 push_disabled`).

The Helm chart at `deploy/helm/kazper/` predates that change and never grew the keys. So the only deployment target for this single-user system can't turn push on. The chart already has the exact patterns this needs: a conditional non-secret env (`GARMIN_BRIDGE_URL`, emitted only when `config.garminBridgeUrl` is set) and an opt-in secret block with a `required` cross-field guard (`GARMIN_API_TOKEN` ⟹ `GARMIN_TOKEN_ENC_KEY`).

## Goals / Non-Goals

**Goals:**
- Expose `FCM_PROJECT_ID` and `FCM_SERVICE_ACCOUNT_JSON` through chart values, opt-in, matching the binary's both-or-neither gating.
- Fail `helm install` fast and clearly when a project id is set without a credential, mirroring the binary's startup validation.
- Keep the default render byte-identical to today (push stays off when unconfigured).
- Document the opt-in in `values.yaml` comments and the chart README, including the `existingSecret` path.

**Non-Goals:**
- No Go, migration, MCP, or `docs/` (swag) changes — the keys already exist and are validated by `internal/config`.
- No validation of the service-account JSON *shape* in Helm — that is the binary's job at startup. The chart only guards presence.
- Not exposing the path-vs-inline distinction for `FCM_SERVICE_ACCOUNT_JSON` specially; the value is passed through verbatim and the binary resolves inline-JSON-vs-path.

## Decisions

**Render `FCM_PROJECT_ID` in the ConfigMap, conditionally.** It is non-secret (a Firebase project id), so it belongs alongside `GARMIN_BRIDGE_URL` in `configmap.yaml`, wrapped in `{{- if .Values.config.fcmProjectId }}` so an unset value omits the key entirely rather than emitting an empty string. Emitting an empty `FCM_PROJECT_ID` would be harmless (the binary treats empty as disabled) but absent-when-unset keeps the rendered manifest clean and matches the bridge-url precedent.

**Render `FCM_SERVICE_ACCOUNT_JSON` in the Secret, conditionally, with a `required` guard.** The credential is secret, so it goes in `secret.yaml` inside the `{{- if not .Values.existingSecret -}}` block. The cross-field rule is the inverse anchor of the Garmin block: there the token gates the key; here the *project id* gates the credential. Implementation:

```gotemplate
{{- if .Values.config.fcmProjectId }}
  FCM_SERVICE_ACCOUNT_JSON: {{ required "secrets.fcmServiceAccountJson is required when config.fcmProjectId is set — inline service-account JSON or a path to a mounted credential" .Values.secrets.fcmServiceAccountJson | quote }}
{{- else if .Values.secrets.fcmServiceAccountJson }}
  FCM_SERVICE_ACCOUNT_JSON: {{ .Values.secrets.fcmServiceAccountJson | quote }}
{{- end }}
```

The `else if` arm renders the credential even if the project id is unset (so a credential-without-id config still surfaces the secret rather than silently dropping it); the binary then treats push as disabled because the id is empty, consistent with `PushEnabled()`. This keeps the chart from ever swallowing a value the operator supplied.

*Alternative considered:* gate only on `secrets.fcmServiceAccountJson` and skip the `required`. Rejected — it would let `config.fcmProjectId=x` install with no credential, producing a pod that crash-loops on the binary's startup validation. The whole point of the Garmin-style `required` is to convert that crash-loop into an install-time error.

**`existingSecret` path does not block.** When `existingSecret` is set the chart renders no Secret at all (existing behavior), so the `required` guard never fires — the operator owns the key set. The README documents that the external Secret must include `FCM_SERVICE_ACCOUNT_JSON` for push. This mirrors how the chart already treats `GARMIN_*` under `existingSecret`.

**Verification via `helm template`.** The PR workflow already runs `helm template deploy/helm/kazper/ --debug`. The new conditionals are covered by rendering the chart with and without the FCM values and asserting key presence/absence and the `required` failure — done locally with `helm template --set` during apply.

## Risks / Trade-offs

- **[Inline JSON in `--set` is awkward to quote]** → Service-account JSON is multi-line and brace-heavy; passing it via `--set-string` or `--set-file secrets.fcmServiceAccountJson=sa.json` is the ergonomic path. The README will show `--set-file`. The binary also accepts a *path*, so an operator can instead mount the credential and set `secrets.fcmServiceAccountJson` to its path — documented as the recommended production approach.
- **[Secret value visible in `helm get values`]** → Same exposure as the existing `garminTokenEncKey`/`databaseUrl`; the `existingSecret` escape hatch already exists for operators who want the chart to never hold the credential. No new posture.
- **[Spec drift if the binary's gating changes]** → The chart now encodes the both-or-neither rule in two places (binary + template). The new spec requirement and the matching scenarios pin the contract so a future change to gating is caught by the spec, not just code.
