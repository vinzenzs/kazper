// Package hydrationbalance stores one daily water-balance snapshot per calendar
// date — Garmin's estimated daily sweat loss, fluid taken during activity, and
// the daily hydration goal. Sister to recoverymetrics / fitnessmetrics (same
// date-keyed upsert shape). DISTINCT from the `hydration` capability, which
// stores the user's per-entry logged intake: this is a device's daily estimate,
// a different grain and source. Unit-isolated (all ml; never merged into the
// hydration summary).
package hydrationbalance

import "time"

// Snapshot mirrors a hydration_balance_metrics row. Date is the identity (one
// row per calendar day), carried as a YYYY-MM-DD string. Every metric is a
// nullable pointer so absent stays distinct from a real zero (activity_intake_ml
// of 0 — sweated, drank nothing during the session — is meaningful).
type Snapshot struct {
	Date             string    `json:"date"`
	SweatLossML      *float64  `json:"sweat_loss_ml,omitempty"`
	ActivityIntakeML *float64  `json:"activity_intake_ml,omitempty"`
	GoalML           *float64  `json:"goal_ml,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
