# Proposal: add-coach-context-endpoints

## Why

The in-app coach (`expand-chat-to-coach`, phase 3) needs to **ground training and recovery advice** before giving it — the same discipline `get_daily_context` already enforces for nutrition. But there is no aggregate read for training or recovery: the data lives across ~30 granular Garmin-mirror tools (workouts, fitness_metrics, recovery_metrics, training phases). Putting all of those in front of the chat model on every round is costly and degrades tool selection (design D10).

This change adds the **two aggregate context reads** the coach surface wants, following the `daily_context` precedent: composition-only bundles over existing read-side repos, one call each. Per D11 they are **dual-surface** — a REST endpoint, an MCP tool (so the desktop coach gains them too), and the shared name lands in `AnnouncedToolNames` — so when `expand-chat-to-coach` adds the matching `agenttools` entries, the drift-guard's subset invariant holds and `chatBespokeTools` does not grow.

## What Changes

- **`GET /context/training`** — a new capability `coach-context`: current training phase, latest fitness snapshot (VO2max, acute/chronic load + derived ACWR, training status), a recent-load summary + recent completed workouts (lookback, default 14d), and upcoming planned workouts (lookahead, default 7d).
- **`GET /context/recovery`** — the latest recovery snapshot (sleep, HRV, resting HR, body battery, training readiness) plus the recent trend (default 7d).
- **MCP tools `get_training_context` / `get_recovery_context`** in `internal/mcpserver`, each one loopback call to the new endpoints, added to `AnnouncedToolNames`.
- Both are **read-only, composition-only** over existing repos (no new tables, no migration), mirroring `internal/dailycontext`'s parallel-fetch / no-partial-bundle shape.

## Capabilities

### New Capabilities

- `coach-context`: aggregate training and recovery context reads for grounding coaching advice.

### Modified Capabilities

_None — `daily-context` is untouched; the MCP tool surface gains two announced names._

## Impact

- **Backend**: a new `internal/coachcontext/` package (types/service/handlers/tests) composing `workouts`, `fitnessmetrics`, `recoverymetrics`, `trainingphases` repos; wiring in `internal/httpserver/server.go`.
- **MCP**: `internal/mcpserver/tools_coachcontext.go` + registration + two new entries in `AnnouncedToolNames`.
- **No DB migration.** Pure read composition.
- **Consumed later**: `expand-chat-to-coach` phase 3 adds the matching `agenttools` registry entries + coach prompt; this change is the backend it grounds on, and is independently shippable (the desktop coach benefits immediately).

## Non-goals

- No new stored metrics or derived totals beyond ACWR (acute/chronic), which is computed at the response boundary, never stored.
- No change to the granular Garmin/fitness/recovery tools — they remain for deep dives.
- No chat-surface or prompt changes here (that is `expand-chat-to-coach` phase 3).
