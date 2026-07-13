## Why

The in-flight `add-data-backup` change covers the *physical* layer (`pg_dump -Fc` dumps + a Helm CronJob) and explicitly names JSON export a non-goal ("JSON export is portability, not backup"). That leaves no portable, human-readable, Postgres-version-independent copy of the data: nothing that survives a Postgres major-version jump, can be inspected or diffed by a human, migrated to a different instance, or handed over as "here is all my data" in one file. This change fills exactly that seam with a full logical JSON export and a guarded import.

## What Changes

- **New `kazper export` subcommand**: connects directly to the database (same standalone pattern as `kazper migrate`) and writes a single JSON document containing every user-data table plus a manifest (format version, export timestamp, app version, migration head, per-table row counts). Deterministic ordering — same data in, byte-identical output out (modulo the manifest timestamp).
- **New `kazper import` subcommand**: restores an export into an **empty** database at the matching migration head, in one transaction, in FK-dependency order, verifying row counts against the manifest. Refuses loudly on a non-empty target, a newer format version, or a migration-head mismatch. No merge/upsert, no destructive `--force` in v1.
- **New `internal/dataexport` package** with an explicit table inventory (export list + exclusion list) and a **schema-drift guard**: export fails if the live database contains a table that is on neither list, so a future migration can never silently leak a table out of the export.
- **Secrets and transient state are excluded by design**: `garmin_tokens` (encrypted token blob), `idempotency_records`, `sync_runs`, `push_tokens`, `relogin_latch` never appear in the export; the docs state that Garmin needs a re-login after importing into a new instance.
- No REST endpoint, no MCP tool, no migration, no response-shape change. Not breaking.

## Capabilities

### New Capabilities

- `data-export`: the logical export/import contract — full-fidelity JSON export of all user data with a manifest, the explicit-inventory drift guard, secret/transient exclusion, guarded empty-database import, and the round-trip fidelity guarantee. Kept separate from the sibling's `data-backup` capability on purpose: `data-backup` is an *operational* contract (scheduled physical dumps, retention, restore drill — Taskfile/Helm surface, no Go code) while this is an *application* contract (Go code, CLI subcommands, a defined file format with versioning). The sibling's own design declares JSON export out of scope, so folding this in would retroactively widen a capability that deliberately excluded it.

### Modified Capabilities

- `cli`: two new subcommands, `export` and `import`, following the existing subcommand + exit-code contract (added as new requirement blocks; the in-flight `harden-server-baseline` change touches this spec's `migrate` requirement, so this delta deliberately adds only new blocks and collides with nothing).

## Impact

- **`cmd/kazper/`**: new `export.go` and `import.go` (Cobra subcommands, config via the shared loader, `ValidateForMigrate`-style DB-only validation).
- **`internal/dataexport/`**: new package — table inventory, drift guard, canonical row serialization, export writer, import loader, plus testcontainers tests (round-trip, drift guard, guard refusals).
- **Docs**: `README.md` / `RUN_LOCAL.md` get a short export/import section positioned against `BACKUP.md` (physical vs logical); note about Garmin re-login after import into a new instance.
- **Not affected**: REST routes, MCP tools (the 1:1 MCP mirror convention applies to REST endpoints only — CLI subcommands are outside its scope), migrations, `docs/` (no `task swag` needed), Helm chart, Taskfile.
