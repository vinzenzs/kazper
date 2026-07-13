package dataexport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/dataexport"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

var exportedAtLine = regexp.MustCompile(`(?m)^.*"exported_at".*$`)

// stripExportedAt removes the manifest's exported_at line so two exports taken
// at different wall-clock times can be compared for byte-identity of everything
// else.
func stripExportedAt(s string) string {
	return exportedAtLine.ReplaceAllString(s, "")
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(ctx, sql, args...)
	require.NoError(t, err, sql)
}

// seedPopulated writes FK-linked rows across the surface the round-trip must
// cover: workouts + linked meals/hydration/fuel, products + components, races +
// legs, chat sessions + messages (identity seq), coach memory, body weight.
func seedPopulated(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	const (
		banana  = "11111111-0000-0000-0000-000000000001"
		recipe  = "11111111-0000-0000-0000-000000000002"
		workout = "22222222-0000-0000-0000-000000000001"
		race    = "33333333-0000-0000-0000-000000000001"
		chatSes = "44444444-0000-0000-0000-000000000001"
	)
	mustExec(t, ctx, pool, `INSERT INTO products (id,name,source) VALUES ($1::uuid,'Banana','manual'),($2::uuid,'Recipe Bowl','recipe')`, banana, recipe)
	mustExec(t, ctx, pool, `INSERT INTO product_components (id,product_id,component_product_id,quantity_g) VALUES (gen_random_uuid(),$1::uuid,$2::uuid,100)`, recipe, banana)
	mustExec(t, ctx, pool, `INSERT INTO workouts (id,source,sport,started_at,ended_at) VALUES ($1::uuid,'manual','bike','2026-06-01T08:00:00Z','2026-06-01T09:30:00Z')`, workout)
	mustExec(t, ctx, pool, `INSERT INTO meal_entries (id,logged_at,quantity_g,product_id,workout_id) VALUES (gen_random_uuid(),'2026-06-01T08:30:00Z',120,$1::uuid,$2::uuid)`, banana, workout)
	mustExec(t, ctx, pool, `INSERT INTO hydration_entries (id,logged_at,quantity_ml,workout_id) VALUES (gen_random_uuid(),'2026-06-01T08:45:00Z',500,$1::uuid)`, workout)
	mustExec(t, ctx, pool, `INSERT INTO workout_fuel_entries (id,logged_at,name,workout_id) VALUES (gen_random_uuid(),'2026-06-01T08:50:00Z','Gel',$1::uuid)`, workout)
	mustExec(t, ctx, pool, `INSERT INTO races (id,name,race_date) VALUES ($1::uuid,'Ironman','2026-09-01')`, race)
	mustExec(t, ctx, pool, `INSERT INTO race_legs (id,race_id,ordinal,discipline) VALUES (gen_random_uuid(),$1::uuid,1,'swim'),(gen_random_uuid(),$1::uuid,2,'bike')`, race)
	mustExec(t, ctx, pool, `INSERT INTO chat_sessions (id) VALUES ($1::uuid)`, chatSes)
	mustExec(t, ctx, pool, `INSERT INTO chat_messages (id,session_id,role,content) VALUES (gen_random_uuid(),$1::uuid,'user','{"m":"hi"}'::jsonb),(gen_random_uuid(),$1::uuid,'assistant','{"m":"hello"}'::jsonb)`, chatSes)
	mustExec(t, ctx, pool, `INSERT INTO coach_memory (text,kind) VALUES ('Prefers morning sessions','preference')`)
	mustExec(t, ctx, pool, `INSERT INTO body_weight_entries (id,logged_at,weight_kg) VALUES (gen_random_uuid(),'2026-06-01T07:00:00Z',72.50)`)
}

// TestRoundTrip: export a populated DB, import into a fresh DB, export again,
// assert byte-identical after normalizing exported_at (task 3.1).
func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := storetest.NewPool(t)
	seedPopulated(t, ctx, src)

	var first bytes.Buffer
	require.NoError(t, dataexport.Export(ctx, src, "test", time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC), &first))

	dst := storetest.NewPool(t)
	summary, err := dataexport.Import(ctx, dst, first.Bytes())
	require.NoError(t, err)
	require.Greater(t, summary.Rows, 0)
	require.Greater(t, summary.Tables, 0)

	var second bytes.Buffer
	require.NoError(t, dataexport.Export(ctx, dst, "test", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), &second))

	require.Equal(t, stripExportedAt(first.String()), stripExportedAt(second.String()),
		"round-trip export must be byte-identical modulo exported_at")
}

// TestDriftGuard: an unclassified table aborts the export naming it, writing no
// document (task 3.2).
func TestDriftGuard(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	mustExec(t, ctx, pool, `CREATE TABLE zzz_orphan (id int PRIMARY KEY)`)

	var buf bytes.Buffer
	err := dataexport.Export(ctx, pool, "test", time.Now(), &buf)
	require.Error(t, err)
	require.Contains(t, err.Error(), "zzz_orphan")
	require.Zero(t, buf.Len(), "no export document should be written when drift is detected")
}

// TestSecretsExcluded: rows in every excluded table (and a Garmin token blob)
// never appear in the export (task 3.4).
func TestSecretsExcluded(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)

	mustExec(t, ctx, pool, `INSERT INTO garmin_tokens (ciphertext, nonce) VALUES ($1,$2)`, []byte("CIPHERTEXTMARKER"), []byte("NONCEMARKER"))
	mustExec(t, ctx, pool, `INSERT INTO push_tokens (token) VALUES ('PUSHTOKENMARKER')`)
	mustExec(t, ctx, pool, `INSERT INTO idempotency_records (client_id,method,path,idempotency_key,status,response_body,request_body_hash) VALUES ('mobile','POST','/x','K',200,$1,'H')`, []byte("BODYMARKER"))
	mustExec(t, ctx, pool, `INSERT INTO sync_runs DEFAULT VALUES`)
	// relogin_latch already holds its migration-seeded row.

	var buf bytes.Buffer
	require.NoError(t, dataexport.Export(ctx, pool, "test", time.Now(), &buf))
	out := buf.String()

	for _, table := range []string{"garmin_tokens", "idempotency_records", "sync_runs", "push_tokens", "relogin_latch"} {
		require.NotContainsf(t, out, `"`+table+`"`, "excluded table %s must not appear", table)
	}
	for _, marker := range []string{"CIPHERTEXTMARKER", "NONCEMARKER", "PUSHTOKENMARKER", "BODYMARKER"} {
		require.NotContainsf(t, out, marker, "secret material %s must not appear", marker)
	}
}

// TestDeterministic: two consecutive exports of unchanged data are byte-identical
// modulo exported_at (task 3.5).
func TestDeterministic(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewPool(t)
	seedPopulated(t, ctx, pool)

	var a, b bytes.Buffer
	require.NoError(t, dataexport.Export(ctx, pool, "test", time.Now(), &a))
	require.NoError(t, dataexport.Export(ctx, pool, "test", time.Now().Add(time.Hour), &b))
	require.Equal(t, stripExportedAt(a.String()), stripExportedAt(b.String()))
}

// tweak parses an export, applies fn to its manifest, and re-marshals. Formatting
// changes are irrelevant to import (it only needs valid JSON).
func tweak(t *testing.T, raw []byte, fn func(manifest map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	fn(doc["manifest"].(map[string]any))
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

// TestImportGuards exercises every import refusal against one seeded export and
// a fresh target, then a real import, then the non-empty guard (task 3.3).
func TestImportGuards(t *testing.T) {
	ctx := context.Background()
	src := storetest.NewPool(t)
	seedPopulated(t, ctx, src)

	var buf bytes.Buffer
	require.NoError(t, dataexport.Export(ctx, src, "v0.42.0", time.Now(), &buf))
	raw := buf.Bytes()

	dst := storetest.NewPool(t) // fresh: all inventory tables empty, relogin_latch seeded

	// Malformed file.
	_, err := dataexport.Import(ctx, dst, []byte("not a valid export"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid export document")

	// Newer format version.
	_, err = dataexport.Import(ctx, dst, tweak(t, raw, func(m map[string]any) { m["format_version"] = 999 }))
	require.Error(t, err)
	require.Contains(t, err.Error(), "format_version")

	// Migration-head mismatch, with a remedy in the message.
	_, err = dataexport.Import(ctx, dst, tweak(t, raw, func(m map[string]any) { m["migration_head"] = 53 }))
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration-head mismatch")

	// Count mismatch → rollback, target left empty.
	_, err = dataexport.Import(ctx, dst, tweak(t, raw, func(m map[string]any) {
		tables := m["tables"].(map[string]any)
		tables["body_weight_entries"] = tables["body_weight_entries"].(float64) + 1
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "count mismatch")

	// All refusals above left dst untouched, so a real import now succeeds —
	// which also proves the migration-seeded relogin_latch row does not block it.
	summary, err := dataexport.Import(ctx, dst, raw)
	require.NoError(t, err)
	require.Greater(t, summary.Rows, 0)

	// Now the target is non-empty: import must refuse and name a table.
	_, err = dataexport.Import(ctx, dst, raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not empty")
	require.Contains(t, err.Error(), "body_weight_entries")
}
