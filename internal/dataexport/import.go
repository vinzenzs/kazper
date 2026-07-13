package dataexport

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// document is the parsed export file: the manifest plus each table's rows kept
// as raw JSON so the row arrays can be handed straight to Postgres'
// json_populate_recordset without a per-column round-trip through Go.
type document struct {
	Manifest Manifest                   `json:"manifest"`
	Tables   map[string]json.RawMessage `json:"tables"`
}

// Summary reports what an import committed.
type Summary struct {
	Tables int
	Rows   int
}

// Import loads an export document from r into pool's database. It enforces the
// data-export guards in order: valid document, supported format version, exact
// migration-head match, empty target (over exported-inventory tables only).
// The load runs in a single transaction inserting parents before children;
// after inserting, per-table counts are verified against the manifest. Any
// failure rolls back the whole import, leaving the database unmodified.
func Import(ctx context.Context, pool *pgxpool.Pool, data []byte) (Summary, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Summary{}, fmt.Errorf("invalid export document: %w", err)
	}
	if doc.Manifest.Tables == nil {
		return Summary{}, fmt.Errorf("invalid export document: missing or empty manifest")
	}

	// Format-version gate.
	if doc.Manifest.FormatVersion > FormatVersion {
		return Summary{}, fmt.Errorf("unsupported export: file format_version %d is newer than this binary supports (%d) — upgrade kazper to import it",
			doc.Manifest.FormatVersion, FormatVersion)
	}

	// Every table named in the file must be one we know how to load.
	for name := range doc.Tables {
		if _, ok := exportedSet[name]; !ok {
			return Summary{}, fmt.Errorf("invalid export document: contains unknown table %q", name)
		}
	}

	// Migration-head gate — the file's rows are shaped for a specific schema.
	head, err := migrationHead(ctx, pool)
	if err != nil {
		return Summary{}, err
	}
	if head != doc.Manifest.MigrationHead {
		if head < doc.Manifest.MigrationHead {
			return Summary{}, fmt.Errorf("migration-head mismatch: database is at %d but the export is at %d — run `kazper migrate` to bring the database up to %d, then retry",
				head, doc.Manifest.MigrationHead, doc.Manifest.MigrationHead)
		}
		return Summary{}, fmt.Errorf("migration-head mismatch: database is at %d but the export is at %d — import with the binary recorded in the manifest (app_version %q, whose head was %d), then upgrade and run `kazper migrate`",
			head, doc.Manifest.MigrationHead, doc.Manifest.AppVersion, doc.Manifest.MigrationHead)
	}

	// Empty-target gate: refuse if any exported-inventory table already holds
	// rows. Excluded tables (e.g. migration-seeded relogin_latch) don't count.
	nonEmpty, err := nonEmptyExportedTables(ctx, pool)
	if err != nil {
		return Summary{}, err
	}
	if len(nonEmpty) > 0 {
		return Summary{}, fmt.Errorf("target database is not empty: table(s) %s already contain rows — import requires an empty database (wipe and re-migrate first)",
			strings.Join(nonEmpty, ", "))
	}

	// Insert order: parents before children over the live FK graph.
	order, err := insertOrder(ctx, pool)
	if err != nil {
		return Summary{}, err
	}

	// Tables whose GENERATED ALWAYS identity columns must be written verbatim.
	identityAlways, err := identityAlwaysTables(ctx, pool)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{}
	err = store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		for _, table := range order {
			raw, ok := doc.Tables[table]
			if !ok || len(raw) == 0 {
				// Table absent from the file → nothing to insert; the manifest
				// count (if present) must then be zero, verified below.
				if want := doc.Manifest.Tables[table]; want != 0 {
					return fmt.Errorf("count mismatch for %s: manifest says %d but the file carries no rows", table, want)
				}
				continue
			}
			overriding := ""
			if _, ok := identityAlways[table]; ok {
				// Preserve the exported identity values (e.g. chat_messages.seq),
				// which Postgres would otherwise refuse for a GENERATED ALWAYS column.
				overriding = "OVERRIDING SYSTEM VALUE "
			}
			ident := pgx.Identifier{table}.Sanitize()
			tag, err := tx.Exec(ctx, fmt.Sprintf(
				`INSERT INTO %s %sSELECT * FROM json_populate_recordset(NULL::%s, $1::json)`,
				ident, overriding, ident), string(raw))
			if err != nil {
				return fmt.Errorf("insert into %s: %w", table, err)
			}
			inserted := int(tag.RowsAffected())
			if want := doc.Manifest.Tables[table]; inserted != want {
				return fmt.Errorf("count mismatch for %s: inserted %d rows but the manifest recorded %d", table, inserted, want)
			}
			summary.Rows += inserted
			if inserted > 0 {
				summary.Tables++
			}
		}
		return nil
	})
	if err != nil {
		return Summary{}, err
	}
	return summary, nil
}

// nonEmptyExportedTables returns the exported-inventory tables that already hold
// at least one row in the target database, in inventory order.
func nonEmptyExportedTables(ctx context.Context, q queryer) ([]string, error) {
	var nonEmpty []string
	for _, t := range exportedTables {
		var exists bool
		sql := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s)`, pgx.Identifier{t}.Sanitize())
		if err := q.QueryRow(ctx, sql).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check emptiness of %s: %w", t, err)
		}
		if exists {
			nonEmpty = append(nonEmpty, t)
		}
	}
	return nonEmpty, nil
}

// insertOrder topologically sorts the exported tables so every table's FK
// parents precede it. The FK graph is read live from pg_catalog, so it stays
// correct as the schema grows. Ties break alphabetically for a stable order.
func insertOrder(ctx context.Context, q queryer) ([]string, error) {
	edges, err := foreignKeyEdges(ctx, q)
	if err != nil {
		return nil, err
	}

	// Build the dependency graph restricted to exported tables.
	indegree := make(map[string]int, len(exportedTables))
	children := make(map[string][]string, len(exportedTables))
	for _, t := range exportedTables {
		indegree[t] = 0
	}
	for _, e := range edges {
		_, childExported := exportedSet[e.child]
		_, parentExported := exportedSet[e.parent]
		if !childExported || !parentExported {
			continue
		}
		children[e.parent] = append(children[e.parent], e.child)
		indegree[e.child]++
	}

	// Kahn's algorithm with an alphabetically-sorted ready set for determinism.
	var ready []string
	for _, t := range exportedTables {
		if indegree[t] == 0 {
			ready = append(ready, t)
		}
	}
	sort.Strings(ready)

	var order []string
	for len(ready) > 0 {
		t := ready[0]
		ready = ready[1:]
		order = append(order, t)

		next := children[t]
		sort.Strings(next)
		for _, c := range next {
			indegree[c]--
			if indegree[c] == 0 {
				ready = insertSorted(ready, c)
			}
		}
	}

	if len(order) != len(exportedTables) {
		return nil, fmt.Errorf("cannot compute insert order: foreign-key cycle among exported tables")
	}
	return order, nil
}

// insertSorted inserts s into the already-sorted slice, keeping it sorted.
func insertSorted(xs []string, s string) []string {
	i := sort.SearchStrings(xs, s)
	xs = append(xs, "")
	copy(xs[i+1:], xs[i:])
	xs[i] = s
	return xs
}
