## ADDED Requirements

### Requirement: Full logical export as a single JSON document with a manifest

The system SHALL produce a full logical export of all user data as a single JSON document containing a manifest object and a per-table map of row arrays. The manifest SHALL include: a `format_version` integer, the export timestamp (`exported_at`, RFC 3339 UTC), the application version (`app_version`, from build metadata), the database migration head (`migration_head`), and per-table row counts. Every table classified as user data in the export inventory SHALL appear in the document (empty tables as empty arrays), with UUID primary and foreign keys preserved verbatim so that all referential links (e.g. `meal_entries.workout_id` → `workouts.id`, `race_legs.race_id` → `races.id`, `product_components.product_id` → `products.id`) remain intact.

#### Scenario: Export contains all user-data tables and a complete manifest

- **WHEN** the user runs `kazper export` against a populated database
- **THEN** the output is one JSON document whose `manifest` carries `format_version`, `exported_at`, `app_version`, `migration_head`, and per-table row counts
- **AND** every table on the export inventory appears under `tables` with row counts matching the manifest
- **AND** all `id` and foreign-key UUID values appear verbatim as stored in the database

#### Scenario: Empty tables are represented, not omitted

- **WHEN** an inventoried user-data table has zero rows at export time
- **THEN** the export contains that table as an empty array and the manifest records a count of 0

### Requirement: Export output is deterministic and diffable

The export SHALL be deterministic: tables serialized in a fixed inventory order, rows ordered by primary key, and row keys in a stable column order, with each row serialized as one compact JSON object per line. Two exports of an unchanged database SHALL differ only in the manifest's `exported_at` value.

#### Scenario: Repeated export of unchanged data is stable

- **WHEN** the user runs `kazper export` twice against a database with no writes in between
- **THEN** the two documents are byte-identical after normalizing `manifest.exported_at`

#### Scenario: Row changes produce line-scoped diffs

- **WHEN** exactly one row is modified between two exports
- **THEN** a line diff of the two documents (ignoring the manifest) touches only the line(s) for that row

### Requirement: Export fails loudly on unclassified tables (schema-drift guard)

The exporter SHALL maintain an explicit inventory classifying every database table as either exported or excluded. At export time it SHALL enumerate the live database's base tables and MUST fail with a non-zero exit — naming the offending table and performing no partial export — if any table exists that is on neither list, or if an inventoried table is missing from the database. A future migration therefore cannot silently add a table that leaks out of (or into) the export without an explicit classification.

#### Scenario: Unknown table aborts the export

- **WHEN** a table exists in the database that is neither on the export inventory nor on the exclusion list
- **THEN** `kazper export` exits non-zero with an error naming that table and writes no export document

#### Scenario: Missing inventoried table aborts the export

- **WHEN** a table on the export inventory does not exist in the database
- **THEN** `kazper export` exits non-zero with an error naming that table

### Requirement: Secrets and transient state are never exported

The export MUST NOT contain the `garmin_tokens` table (encrypted Garmin auth blob) or any other credential/token material, and SHALL exclude instance-bound transient tables: `idempotency_records`, `sync_runs`, `push_tokens`, and `relogin_latch`. There SHALL be no flag that includes secrets in the export. Documentation SHALL state that after importing into a new instance, Garmin requires a re-login and the mobile companion re-registers its push token.

#### Scenario: Garmin token blob is absent from the export

- **WHEN** the database holds a stored Garmin token and the user runs `kazper export`
- **THEN** the export document contains no `garmin_tokens` entry and no ciphertext, nonce, or token material anywhere in the output

#### Scenario: Transient tables are absent from the export

- **WHEN** the database holds rows in `idempotency_records`, `sync_runs`, `push_tokens`, and `relogin_latch`
- **THEN** none of those tables appear in the export document

### Requirement: Import restores into an empty database only

The import SHALL refuse to run — exiting non-zero with an error listing the non-empty tables and modifying nothing — if any table on the export inventory contains rows in the target database. Tables on the exclusion list (including migration-seeded rows such as `relogin_latch`) SHALL NOT count toward this check. The import SHALL provide no merge, upsert, or overwrite mode.

#### Scenario: Non-empty target is refused

- **WHEN** the user runs `kazper import <file>` against a database where at least one inventoried table has rows
- **THEN** the process exits non-zero, names the non-empty table(s), and leaves the database unchanged

#### Scenario: Migration-seeded excluded rows do not block import

- **WHEN** the target database is freshly migrated (so `relogin_latch` holds its seeded row) and all inventoried tables are empty
- **THEN** the empty-database check passes and the import proceeds

### Requirement: Import is atomic and preserves referential integrity

The import SHALL run in a single database transaction, inserting tables in an order that satisfies the live foreign-key graph, preserving all UUIDs verbatim. After inserting, it SHALL verify per-table row counts against the manifest within the same transaction. Any foreign-key violation, insert error, or count mismatch SHALL roll back the entire import, leaving the database empty of imported rows.

#### Scenario: FK-linked rows survive import

- **WHEN** an export containing meals, hydration, and workout-fuel entries linked to workouts via `workout_id` is imported into an empty database
- **THEN** the import succeeds and every `workout_id` reference resolves to the same workout row it referenced at export time

#### Scenario: A failed import leaves no partial state

- **WHEN** an import fails partway (e.g. a row violates a constraint)
- **THEN** the process exits non-zero and the target database contains no rows imported by that run

#### Scenario: Count mismatch rolls back

- **WHEN** the number of rows inserted for a table differs from the manifest's recorded count
- **THEN** the import rolls back entirely and exits non-zero naming the table

### Requirement: Import refuses incompatible export files

The import SHALL read and validate the manifest before touching data. It MUST refuse files whose `format_version` is greater than the version the binary supports, and MUST refuse when the target database's migration head does not exactly equal `manifest.migration_head`; the refusal error SHALL state the remedial step (run `kazper migrate` if the database is behind, or use the binary version recorded in `manifest.app_version` if the file predates the current schema). The import SHALL NOT run migrations itself, and SHALL also refuse files that are not valid export documents.

#### Scenario: Newer format version is refused

- **WHEN** the user runs `kazper import` on a file whose `manifest.format_version` exceeds the binary's supported version
- **THEN** the process exits non-zero naming both versions and modifies nothing

#### Scenario: Migration-head mismatch is refused with a remedy

- **WHEN** the target database's migration head differs from `manifest.migration_head`
- **THEN** the process exits non-zero, names both heads, states the remedial command, and modifies nothing

#### Scenario: Malformed file is refused

- **WHEN** the input file is not a valid export document (missing manifest, invalid JSON)
- **THEN** the process exits non-zero with a parse/validation error and modifies nothing

### Requirement: Export and import round-trip with full fidelity

Exporting a database, importing that export into a freshly migrated empty database, and exporting again SHALL yield a document byte-identical to the first export after normalizing `manifest.exported_at`. This round-trip guarantee SHALL be verified by an integration test against a real Postgres instance.

#### Scenario: Round trip yields identical output

- **WHEN** an export of a populated database is imported into a fresh empty database at the same migration head and a second export is taken
- **THEN** the two export documents are byte-identical after normalizing `manifest.exported_at`
