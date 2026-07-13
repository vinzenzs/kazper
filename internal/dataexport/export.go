package dataexport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Drift checks the live database against the inventory. It returns an error
// naming the first offending table if any live table is unclassified (on
// neither the exported nor the excluded list), or if an exported table is
// missing from the database. A missing excluded table is not an error — those
// carry no data. Callers run this before every export so a schema change can
// never silently alter what the export contains.
func Drift(ctx context.Context, q queryer) error {
	live, err := liveBaseTables(ctx, q)
	if err != nil {
		return err
	}
	liveSet := toSet(live)

	// Every live table must be classified.
	for _, t := range live {
		_, exported := exportedSet[t]
		_, excluded := excludedSet[t]
		if !exported && !excluded {
			return fmt.Errorf("schema drift: table %q is on neither the export nor the exclusion list — classify it in internal/dataexport/inventory.go before exporting", t)
		}
	}

	// Every exported table must exist (deterministically report the first in
	// inventory order).
	for _, t := range exportedTables {
		if _, ok := liveSet[t]; !ok {
			return fmt.Errorf("schema drift: exported table %q does not exist in the database", t)
		}
	}
	return nil
}

// Export writes a full logical export of pool's database to w. appVersion is
// recorded in the manifest (build metadata from the caller). now is the value
// stamped into manifest.exported_at (RFC 3339 UTC).
//
// The whole export runs in one repeatable-read, read-only transaction pinned to
// UTC, so table counts and rows form a single consistent snapshot and
// timestamps render deterministically.
func Export(ctx context.Context, pool *pgxpool.Pool, appVersion string, now time.Time, w io.Writer) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("begin export snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // read-only; rollback releases the snapshot

	// Deterministic timestamp rendering for timestamptz columns.
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return fmt.Errorf("pin snapshot to UTC: %w", err)
	}

	if err := Drift(ctx, tx); err != nil {
		return err
	}

	head, err := migrationHead(ctx, tx)
	if err != nil {
		return err
	}

	// Pass 1: per-table counts for the manifest (same snapshot as the rows).
	counts := make(map[string]int, len(exportedTables))
	for _, t := range exportedTables {
		var n int
		q := fmt.Sprintf(`SELECT count(*) FROM %s`, pgx.Identifier{t}.Sanitize())
		if err := tx.QueryRow(ctx, q).Scan(&n); err != nil {
			return fmt.Errorf("count %s: %w", t, err)
		}
		counts[t] = n
	}

	manifest := Manifest{
		FormatVersion: FormatVersion,
		ExportedAt:    now.UTC().Format(time.RFC3339Nano),
		AppVersion:    appVersion,
		MigrationHead: head,
		Tables:        counts,
	}

	bw := bufio.NewWriter(w)
	if err := writeManifest(bw, manifest); err != nil {
		return err
	}

	// Pass 2: stream each table's rows, one compact JSON object per line.
	if _, err := bw.WriteString("  \"tables\": {\n"); err != nil {
		return err
	}
	for i, t := range exportedTables {
		if err := writeTableRows(ctx, tx, bw, t); err != nil {
			return err
		}
		if i < len(exportedTables)-1 {
			if _, err := bw.WriteString(",\n"); err != nil {
				return err
			}
		} else {
			if _, err := bw.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	if _, err := bw.WriteString("  }\n}\n"); err != nil {
		return err
	}
	return bw.Flush()
}

// writeManifest emits the opening brace and the manifest object, with the table
// counts in fixed inventory order (Go maps don't preserve order, so it's built
// by hand).
func writeManifest(bw *bufio.Writer, m Manifest) error {
	fmt.Fprintf(bw, "{\n  \"manifest\": {\n")
	fmt.Fprintf(bw, "    \"format_version\": %d,\n", m.FormatVersion)
	fmt.Fprintf(bw, "    \"exported_at\": %s,\n", jsonString(m.ExportedAt))
	fmt.Fprintf(bw, "    \"app_version\": %s,\n", jsonString(m.AppVersion))
	fmt.Fprintf(bw, "    \"migration_head\": %d,\n", m.MigrationHead)
	fmt.Fprintf(bw, "    \"tables\": {\n")
	for i, t := range exportedTables {
		sep := ","
		if i == len(exportedTables)-1 {
			sep = ""
		}
		fmt.Fprintf(bw, "      %s: %d%s\n", jsonString(t), m.Tables[t], sep)
	}
	fmt.Fprintf(bw, "    }\n  },\n")
	return nil
}

// writeTableRows emits `    "<table>": [ ... ]` with each row on its own line.
// Rows are ordered by primary key for determinism; subsequent rows are prefixed
// with a leading comma so adding a row is a single-line diff.
func writeTableRows(ctx context.Context, q queryer, bw *bufio.Writer, table string) error {
	pk, err := primaryKeyColumns(ctx, q, table)
	if err != nil {
		return err
	}
	orderBy := make([]string, len(pk))
	for i, c := range pk {
		orderBy[i] = pgx.Identifier{c}.Sanitize()
	}
	sql := fmt.Sprintf(`SELECT row_to_json(t)::text FROM %s t ORDER BY %s`,
		pgx.Identifier{table}.Sanitize(), strings.Join(orderBy, ","))

	rows, err := q.Query(ctx, sql)
	if err != nil {
		return fmt.Errorf("read %s rows: %w", table, err)
	}
	defer rows.Close()

	fmt.Fprintf(bw, "    %s: [", jsonString(table))
	any := false
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return fmt.Errorf("scan %s row: %w", table, err)
		}
		if any {
			bw.WriteString("\n,")
		} else {
			bw.WriteString("\n")
		}
		bw.WriteString(line)
		any = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s rows: %w", table, err)
	}
	if any {
		bw.WriteString("\n    ]")
	} else {
		bw.WriteString("]")
	}
	return nil
}

// jsonString renders s as a JSON string literal (with surrounding quotes and
// proper escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
