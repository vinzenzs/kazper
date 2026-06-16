// Package coachrecs persists coach-authored recommendations as a thin dated log
// (priorities #6F). It is a storage primitive only: the agent synthesizes a
// recommendation and records it here; this package stores the authored text and
// returns it verbatim, never generating, ranking, or interpreting one, and never
// mutating an enforced target.
package coachrecs

import (
	"time"

	"github.com/google/uuid"
)

// Scope is the closed set of recommendation scopes. Kept in sync with the
// coach_recommendations.scope CHECK constraint.
type Scope string

const (
	ScopeFueling  Scope = "fueling"
	ScopeTraining Scope = "training"
	ScopeRecovery Scope = "recovery"
	ScopeRace     Scope = "race"
	ScopeGeneral  Scope = "general"
)

// ValidScope reports whether s is one of the allowed scopes.
func ValidScope(s string) bool {
	switch Scope(s) {
	case ScopeFueling, ScopeTraining, ScopeRecovery, ScopeRace, ScopeGeneral:
		return true
	default:
		return false
	}
}

// Recommendation mirrors a coach_recommendations row. Date is the local date the
// advice applies to, serialized as YYYY-MM-DD (matching training_plans.start_date).
type Recommendation struct {
	ID             uuid.UUID `json:"id"`
	Date           string    `json:"date"` // YYYY-MM-DD
	Scope          Scope     `json:"scope"`
	Recommendation string    `json:"recommendation"`
	Reason         *string   `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
