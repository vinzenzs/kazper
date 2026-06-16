## Context

The `athlete-config` singleton stores `ftp_watts` (cycling functional threshold power) and the `workouts` table stores `normalized_power_w` + `intensity_factor` (NUMERIC(4,2)). Today both inputs are captured but IF is only ever populated when a client/watch supplies it directly; the `athlete-config` spec deferred deriving it ("Storing FTP does not back-fill workout intensity_factor"). The sibling deferrals from that same spec — cadence, secondary, swim-pace, and multisport target resolution — have all shipped, leaving IF-from-FTP as the last named follow-up.

The codebase already has the exact wiring precedent: `trainingplan` reads the athlete-config singleton via a cross-injected repo (`trainingPlanSvc.SetAthleteConfigRepo(athleteConfigRepo)`) and gates power-zone resolution to bike sports (`resolve.go` D7). `coachcontext` already derives a value from `ftp_watts` (`wattsPerKg`) on read. This change applies the same patterns to `workouts`.

## Goals / Non-Goals

**Goals:**
- Populate `intensity_factor` for bike workouts that have `normalized_power_w` but no client-supplied IF, using the configured FTP.
- Never override a watch-/client-supplied `intensity_factor`.
- Keep the workout write path correct and error-free when athlete-config or FTP is absent.

**Non-Goals:**
- No bulk back-fill migration over historical rows (they fill on next re-sync/patch).
- No FTP history table; IF is point-in-time against the FTP at write.
- No running/swim power-based IF (FTP is cycling-specific).
- No new endpoints, request fields, or MCP tools.
- No consumption of FTP by race-fueling/raceprep (still deferred).

## Decisions

**Derive at write time and store, rather than derive on read.** IF is point-in-time: it should reflect the FTP in effect when the workout happened. With a singleton (history-less) athlete-config, derive-on-read would silently rewrite *every* historical workout's IF whenever the athlete bumps their FTP — wrong. Derive-on-write captures the correct FTP at the moment of ingestion and never mutates afterward. It also makes the stored column authoritative for any future reader (fueling math, summaries) without each re-deriving. _Alternative considered:_ derive-on-read in handlers/coachcontext (like `wattsPerKg`) — rejected for the retroactive-rewrite problem and because IF is a real stored column, unlike the purely-derived `wattsPerKg`.

**Cross-inject the athlete-config repo into the workouts service via an optional setter.** Add a `SetAthleteConfigRepo(*athleteconfig.Repo)` setter and a nilable field on `workouts.Service`, wired in `httpserver/server.go` right after `athleteConfigRepo` is constructed — identical to `trainingPlanSvc.SetAthleteConfigRepo` and `mealsSvc.SetWorkoutsRepo`. When the field is nil (e.g. unit tests that don't wire it), the gate fails closed: no derivation, no error. _Alternative considered:_ constructor param — rejected to match the established optional-setter convention and avoid touching every `NewService` caller. No import cycle: `athleteconfig` does not import `workouts`.

**Gate = bike AND np>0 AND ftp>0 AND no supplied IF.** Mirrors the `resolve.go` D7 bike-gate (power is FTP/bike-derived). The "no supplied IF" arm requires distinguishing "caller omitted IF" from "caller sent IF"; the service input already carries `IntensityFactor *float64`, so `nil` = derive-eligible, non-nil = verbatim. Garmin ingestion that already provides IF therefore passes through untouched; the derivation only fills the genuine gap.

**Round to 2dp at derivation, consistent with the column.** `intensity_factor` is `NUMERIC(4,2)` and the workouts service already documents storing IF at 2dp. Use the same rounding helper family already in the service (the create path comment at `service.go:806` notes 2dp for intensity_factor). Reads continue to round at the response boundary.

**Apply on both create and update (full-replace).** The update path is full-replace (PUT-style); a workout re-synced from Garmin without IF but with NP should fill in. Same gate, same code path — factor the derivation into one helper called from both build sites.

## Risks / Trade-offs

- **[Stale IF after an FTP change]** A workout written under an old FTP keeps its old IF even after the athlete updates FTP. → Intended: IF is point-in-time. If a true recompute is ever wanted, it is a separate explicit back-fill task, not this change.
- **[Re-sync overwrites a manually-set IF with a derived one]** Only if the re-sync request omits `intensity_factor`. → The gate already protects any IF present *in the request*; a full-replace that drops a previously-stored manual IF is the caller's intent (PUT semantics), consistent with how every other field behaves on replace.
- **[Service gains a new dependency]** `workouts` now reads athlete-config on every qualifying write. → Single indexed singleton lookup; only performed when the bike+NP+no-IF pre-gate already passes, so most writes skip it entirely. Nil-repo path keeps existing tests green.
- **[Spec delta drift]** The `athlete-config` MODIFIED requirement is full-replace; its header text must match the live spec exactly so the archive merge targets the right requirement (per the continuity pattern note). → Header copied verbatim; only body + the one FTP scenario edited, sibling scenarios preserved.
