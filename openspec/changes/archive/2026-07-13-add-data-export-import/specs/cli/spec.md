## ADDED Requirements

### Requirement: export subcommand

The binary SHALL provide an `export` subcommand that connects directly to the configured database (same standalone pattern as `migrate` — config via the shared loader, no HTTP server, no MCP server, no background goroutines) and writes a full logical JSON export per the `data-export` capability. It SHALL write the document to stdout by default, with an `--out <path>` flag to write to a file instead; all log output SHALL go to stderr so stdout stays pipeable. It SHALL exit 0 on success and non-zero on any failure (unreachable database, drift-guard violation, write error), logging the error.

#### Scenario: Export writes to stdout by default

- **WHEN** the user runs `kazper export` with a valid `DATABASE_URL`
- **THEN** the JSON export document is written to stdout, diagnostics go to stderr, and the process exits 0

#### Scenario: --out writes to a file

- **WHEN** the user runs `kazper export --out backup.json`
- **THEN** the export document is written to `backup.json` and the process exits 0

#### Scenario: Export fails on bad database URL

- **WHEN** the user runs `kazper export` with `DATABASE_URL` pointing to an unreachable host
- **THEN** the process logs the error and exits with a non-zero status

### Requirement: import subcommand

The binary SHALL provide an `import` subcommand taking the export file as a required positional argument (`kazper import <file>`, with `-` meaning stdin) that restores the export into the configured database per the `data-export` capability's guards (empty database, matching migration head, supported format version). It SHALL run standalone against the database like `migrate`. It SHALL exit 0 only when the full import committed, and non-zero — with a clear, named reason — on any refusal or failure, leaving the database unmodified in those cases. On success it SHALL print a summary including the number of tables and rows imported.

#### Scenario: Import succeeds against a fresh database

- **WHEN** the user runs `kazper import export.json` against a freshly migrated empty database at the matching migration head
- **THEN** the import commits, a summary of imported tables and row counts is printed, and the process exits 0

#### Scenario: Missing file argument is rejected

- **WHEN** the user runs `kazper import` with no file argument
- **THEN** the process prints usage naming the required argument and exits non-zero without touching the database

#### Scenario: Guard refusals exit non-zero without changes

- **WHEN** the user runs `kazper import <file>` and any import guard refuses (non-empty database, migration-head mismatch, unsupported format version, malformed file)
- **THEN** the process exits non-zero with an error naming the specific guard and the database is unchanged
