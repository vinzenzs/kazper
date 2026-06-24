## 1. Chart templates

- [x] 1.1 Add conditional `FCM_PROJECT_ID` to `deploy/helm/kazper/templates/configmap.yaml`, rendered from `.Values.config.fcmProjectId` only when non-empty (mirror the `GARMIN_BRIDGE_URL` block).
- [x] 1.2 Add conditional `FCM_SERVICE_ACCOUNT_JSON` to `deploy/helm/kazper/templates/secret.yaml` inside the `if not .Values.existingSecret` block: when `config.fcmProjectId` is set, render via `required` (clear message naming `secrets.fcmServiceAccountJson`); else if `secrets.fcmServiceAccountJson` is set, render it; else omit.

## 2. Chart values & docs

- [x] 2.1 Add `config.fcmProjectId: ""` to `deploy/helm/kazper/values.yaml` with a comment explaining it pairs with `secrets.fcmServiceAccountJson` and that both empty keeps push disabled (`503 push_disabled`).
- [x] 2.2 Add `secrets.fcmServiceAccountJson: ""` to `values.yaml` with a comment: required when `config.fcmProjectId` is set; inline service-account JSON or a path to a mounted credential (`--set-file` recommended).
- [x] 2.3 Update the `existingSecret` doc comment in `values.yaml` to list `FCM_SERVICE_ACCOUNT_JSON` as an optional key the external Secret must carry for push.
- [x] 2.4 Add a "Push notifications (opt-in)" note to `deploy/helm/kazper/README.md`: the two values, the both-required-together rule, the `existingSecret` path, and disabled-by-default behavior.

## 3. Verify

- [x] 3.1 `helm template deploy/helm/kazper/` with default values → no `FCM_PROJECT_ID` in ConfigMap, no `FCM_SERVICE_ACCOUNT_JSON` in Secret.
- [x] 3.2 `helm template` with `config.fcmProjectId` set and `secrets.fcmServiceAccountJson` empty (no `existingSecret`) → fails with the `required` message naming `secrets.fcmServiceAccountJson`.
- [x] 3.3 `helm template` with both set → `FCM_PROJECT_ID` present in ConfigMap and `FCM_SERVICE_ACCOUNT_JSON` present in Secret; with `existingSecret` set + project id → renders no Secret and does not block.
- [x] 3.4 `helm lint deploy/helm/kazper/` passes.
