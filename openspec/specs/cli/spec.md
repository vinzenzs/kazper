# cli Specification

## Purpose

Define the root command structure, subcommand contract, and exit-code conventions for the `kazper` binary so ops, API, and MCP entrypoints share a single discoverable CLI surface.
## Requirements
### Requirement: Root command and subcommand structure

The system SHALL ship a single binary named `kazper` that exposes its functionality as Cobra subcommands.

The binary SHALL provide the following subcommands at minimum: `serve`, `mcp`, `migrate`, `version`. Running the binary with no subcommand SHALL print usage help and exit with a non-zero status.

Every subcommand SHALL accept `--help` and print its description, flags, and example invocations.

#### Scenario: Help discovery

- **WHEN** a user runs `kazper --help`
- **THEN** the output lists the subcommands `serve`, `mcp`, `migrate`, `version` with one-line descriptions and exits 0

#### Scenario: Missing subcommand fails clearly

- **WHEN** a user runs `kazper` with no arguments
- **THEN** the process prints usage help to stderr and exits with status code 1

### Requirement: serve subcommand

The `serve` subcommand SHALL start the Gin HTTP server with the same behavior as the previous `cmd/api` binary: load config, validate auth tokens, run migrations if `MIGRATE_ON_START=true`, open the DB pool, register routes, listen on the configured address, and shut down gracefully on SIGINT/SIGTERM.

The subcommand SHALL accept an optional `--addr` flag that overrides the `HTTP_ADDR` environment variable.

#### Scenario: serve starts the HTTP server

- **WHEN** the user runs `kazper serve` with valid configuration
- **THEN** the process binds on the configured address, registers `/healthz`, `/readyz`, and the authenticated API routes, and logs `http listening`

#### Scenario: --addr overrides HTTP_ADDR

- **WHEN** the user runs `kazper serve --addr :9090` with `HTTP_ADDR=:8080` set
- **THEN** the server binds on `:9090`

#### Scenario: Graceful shutdown on SIGTERM

- **WHEN** the process receives SIGTERM while serving
- **THEN** it calls `srv.Shutdown` with a 10-second timeout and exits 0

### Requirement: mcp subcommand

The `mcp` subcommand SHALL start the MCP server with the same behavior as the previous `cmd/mcp` binary.

#### Scenario: mcp starts the MCP server

- **WHEN** the user runs `kazper mcp` with valid configuration
- **THEN** the MCP server starts and accepts protocol connections per the existing MCP server behavior

### Requirement: migrate subcommand

The `migrate` subcommand SHALL apply pending database migrations and exit. It SHALL NOT start the HTTP server, MCP server, or any background goroutines.

The subcommand SHALL exit with status 0 on success and a non-zero status on failure, logging the error.

#### Scenario: migrate applies pending migrations

- **WHEN** the user runs `kazper migrate` against a database with pending migrations
- **THEN** the process applies the migrations, logs success, and exits 0

#### Scenario: migrate fails on bad database URL

- **WHEN** the user runs `kazper migrate` with `DATABASE_URL` pointing to an unreachable host
- **THEN** the process logs the error and exits with a non-zero status

### Requirement: version subcommand

The `version` subcommand SHALL print build metadata (semantic version, git commit SHA, build date) to stdout and exit 0. These values SHALL be injected at build time via `-ldflags` and SHALL default to a placeholder (e.g., `dev`) when not injected.

#### Scenario: Version prints build metadata

- **WHEN** the user runs `kazper version`
- **THEN** stdout contains the version, commit, and build date in a stable, parseable format and the process exits 0

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

### Requirement: migrate detects and recovers a dirty migration state

The `migrate` subcommand SHALL accept a `--force <version>` flag wrapping golang-migrate's `Force` to clear a dirty migration state; a bare `--force` without a version SHALL be rejected. When any migration run fails — via the subcommand or via `MIGRATE_ON_START` at serve boot — and the migration state is dirty, the logged error SHALL name the dirty version and the exact recovery command (`kazper migrate --force <version>`) along with guidance to inspect the failed migration first, instead of only surfacing the raw driver error.

#### Scenario: Dirty state produces an actionable error

- **WHEN** a migration fails partway and the process is started again with `MIGRATE_ON_START=true`
- **THEN** startup fails with a message naming the dirty version and the `kazper migrate --force <version>` recovery command

#### Scenario: Force clears the dirty flag

- **WHEN** the operator runs `kazper migrate --force <version>` against a database with a dirty migration state
- **THEN** the dirty flag is cleared at that version, the process exits 0
- **AND** a subsequent `kazper migrate` applies pending migrations normally

#### Scenario: Bare force is rejected

- **WHEN** the operator runs `kazper migrate --force` without a version
- **THEN** the command exits non-zero without modifying migration state

