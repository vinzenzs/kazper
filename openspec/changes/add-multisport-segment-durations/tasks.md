# Tasks

## 1. Backend

- [x] 1.1 Surface the per-segment sum in the multisport template serialization (single + list), null on non-time-bound segments, repeat expansion consistent with the template-level derivation
- [x] 1.2 Tests: fully-bounded tri template (per-segment values + total consistency), one-unbounded-segment null isolation, write-ignored regression
- [x] 1.3 `task swag`

## 2. Verification

- [x] 2.1 `task vet` + multisport package green; MCP golden unchanged expected (response-only field flows through verbatim tools — confirm no schema regen needed)
