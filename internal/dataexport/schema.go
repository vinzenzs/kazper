package dataexport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// queryer is the subset of pgxpool.Pool / pgx.Tx the package needs, so the same
// code runs against a pool (drift guard, head read) or inside a transaction
// (export snapshot, import load).
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// liveBaseTables returns the names of all base tables in the public schema,
// excluding schema_migrations (which is migration bookkeeping, not data).
func liveBaseTables(ctx context.Context, q queryer) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		  AND table_name <> 'schema_migrations'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("list live tables: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan live table: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// migrationHead returns the version recorded in schema_migrations — the value
// import matches against and export records in the manifest. Errors if the
// database is unmigrated (no row).
func migrationHead(ctx context.Context, q queryer) (int, error) {
	var version int
	err := q.QueryRow(ctx, `SELECT version FROM schema_migrations`).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("database has no migration head — run `kazper migrate` first")
	}
	if err != nil {
		return 0, fmt.Errorf("read migration head: %w", err)
	}
	return version, nil
}

// primaryKeyColumns returns a table's primary-key columns in index order, used
// to give the export a deterministic per-table row order.
func primaryKeyColumns(ctx context.Context, q queryer, table string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)`, "public."+table)
	if err != nil {
		return nil, fmt.Errorf("read primary key for %s: %w", table, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan pk column for %s: %w", table, err)
		}
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %s has no primary key", table)
	}
	return cols, nil
}

// fkEdge is a child→parent foreign-key dependency between two tables.
type fkEdge struct {
	child  string
	parent string
}

// foreignKeyEdges returns every foreign-key dependency in the public schema,
// child→parent, so import can insert parents before children. Self-referential
// edges are dropped (they don't constrain table-level ordering; the schema has
// none today).
func foreignKeyEdges(ctx context.Context, q queryer) ([]fkEdge, error) {
	rows, err := q.Query(ctx, `
		SELECT c.conrelid::regclass::text, c.confrelid::regclass::text
		FROM pg_constraint c
		WHERE c.contype = 'f' AND c.connamespace = 'public'::regnamespace`)
	if err != nil {
		return nil, fmt.Errorf("read foreign keys: %w", err)
	}
	defer rows.Close()

	seen := make(map[fkEdge]struct{})
	var edges []fkEdge
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, fmt.Errorf("scan fk edge: %w", err)
		}
		e := fkEdge{child: unqualify(child), parent: unqualify(parent)}
		if e.child == e.parent {
			continue
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// unqualify strips a leading "public." schema qualifier from a regclass::text
// rendering (present only when the type is not on the search_path).
func unqualify(name string) string {
	return strings.TrimPrefix(name, "public.")
}

// identityAlwaysTables returns the set of tables that have at least one
// GENERATED ALWAYS AS IDENTITY column (e.g. chat_messages.seq). Their inserts
// need OVERRIDING SYSTEM VALUE to write the exported value verbatim; Postgres
// otherwise rejects a non-DEFAULT value for such a column. Read live so a
// future identity column is handled without code changes.
func identityAlwaysTables(ctx context.Context, q queryer) (map[string]struct{}, error) {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT c.relname
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r' AND a.attidentity = 'a'`)
	if err != nil {
		return nil, fmt.Errorf("read identity columns: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan identity table: %w", err)
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}
