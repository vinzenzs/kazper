// Package dataexport implements the logical JSON export/import of all user
// data (the data-export capability). It is the counterpart to the physical
// pg_dump backup (data-backup): portable, human-readable, and independent of
// the Postgres major version.
//
// The package is inventory-driven — every table in the database is explicitly
// classified as exported or excluded, and a drift guard fails the export loudly
// if the live schema ever contains a table on neither list. That converts a
// future migration's silent data leak (or loss) into a compile-a-decision
// checkpoint.
package dataexport

// FormatVersion is the export document's format version. Import refuses files
// whose manifest declares a version greater than this.
const FormatVersion = 1

// exportedTables lists every user-data table, in the fixed order they appear in
// the export document. Ordering is part of the format's determinism contract —
// do not reorder without bumping FormatVersion. Verified against migration head
// 064 (41 exported tables; workout_streams from 056 and garmin_detected_thresholds
// from 064 are excluded — see below).
// A future migration that adds a table forces a classification decision here via
// the drift guard (see Drift).
var exportedTables = []string{
	"products",
	"product_components",
	"meal_entries",
	"planned_meals",
	"shopping_items",
	"hydration_entries",
	"workouts",
	"workout_sets",
	"workout_splits",
	"workout_best_efforts",
	"workout_fuel_entries",
	"body_weight_entries",
	"nutrition_goals",
	"daily_goal_overrides",
	"goal_templates",
	"training_phases",
	"workout_templates",
	"multisport_templates",
	"training_plans",
	"plan_weeks",
	"plan_slots",
	"macrocycles",
	"races",
	"race_legs",
	"race_leg_pacing_overrides",
	"recovery_metrics",
	"fitness_metrics",
	"hydration_balance_metrics",
	"daily_summary",
	"health_vitals",
	"gear",
	"personal_records",
	"achievements",
	"devices",
	"athlete_config",
	"athlete_config_history",
	"chat_sessions",
	"chat_messages",
	"coach_memory",
	"wellness_entries",
	"supplement_entries",
}

// excludedTables lists tables deliberately kept out of the export: secrets,
// replay caches, and instance-bound transient state. They are recreated or
// re-provisioned on a fresh instance (Garmin re-login, companion re-registers
// its push token). Classified so the drift guard treats them as known, not as
// leaks.
var excludedTables = []string{
	"garmin_tokens",       // AES-256-GCM Garmin auth blob — secret, never exported
	"idempotency_records", // replay cache; embeds response bodies, stale on a new instance
	"sync_runs",           // Garmin-bridge invocation ops log; recreated on next sync
	"push_tokens",         // FCM device registrations; the companion re-registers
	"relogin_latch",       // single migration-seeded guard row
	"workout_streams",           // bulky raw 1 Hz sample arrays; re-derived on Garmin re-sync, and the compact derivations (workout_best_efforts + workouts.variability_index/efficiency_factor/decoupling_pct) are already exported
	"garmin_detected_thresholds", // latest-only advisory detection singleton; re-derived by the next sync (separate-garmin-threshold-detection). The confirmed athlete_config (incl. its garmin_sourced_fields policy column) is what's exported.
}

// exportedSet and excludedSet are lookup views of the two lists above.
var (
	exportedSet = toSet(exportedTables)
	excludedSet = toSet(excludedTables)
)

func toSet(xs []string) map[string]struct{} {
	s := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

// Manifest is the export document's self-describing header.
type Manifest struct {
	FormatVersion int            `json:"format_version"`
	ExportedAt    string         `json:"exported_at"`
	AppVersion    string         `json:"app_version"`
	MigrationHead int            `json:"migration_head"`
	Tables        map[string]int `json:"tables"`
}
