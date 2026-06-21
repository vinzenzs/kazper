// Package coachmemory persists agent-authored coach memory as a kinded,
// athlete-scoped store (widen-coach-recs-to-memory; supersedes coachrecs). A
// recommendation is one `kind` of memory alongside dateless standing items
// (fact / preference / constraint / observation), each with a review/expire
// lifecycle. It is a storage primitive only: the coach authors the text; this
// package records and returns it verbatim, never generating, ranking, or
// interpreting memory, and never mutating an enforced target. It is the
// cross-surface channel — the in-app chat coach and the MCP agent share what
// they've learned here without sharing conversation transcripts.
package coachmemory

import (
	"time"

	"github.com/google/uuid"
)

// Kind discriminates a memory item. `recommendation` carries a required date
// (advice for a day); the others are dateless standing items. Kept in sync with
// the coach_memory.kind CHECK constraint.
type Kind string

const (
	KindFact           Kind = "fact"
	KindPreference     Kind = "preference"
	KindConstraint     Kind = "constraint"
	KindObservation    Kind = "observation"
	KindRecommendation Kind = "recommendation"
)

func ValidKind(s string) bool {
	switch Kind(s) {
	case KindFact, KindPreference, KindConstraint, KindObservation, KindRecommendation:
		return true
	default:
		return false
	}
}

// Status is the memory lifecycle. `archived` items are hidden from the default
// list and from grounding; the row survives so the "how long did we believe
// this" history is preserved. Kept in sync with the coach_memory.status CHECK.
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}

// Scope is the optional (was required) tag carried mainly by recommendations.
// Kept in sync with the coach_memory.scope CHECK constraint (which now admits
// NULL). The same five values as the prior coach-recommendations log.
type Scope string

const (
	ScopeFueling  Scope = "fueling"
	ScopeTraining Scope = "training"
	ScopeRecovery Scope = "recovery"
	ScopeRace     Scope = "race"
	ScopeGeneral  Scope = "general"
)

func ValidScope(s string) bool {
	switch Scope(s) {
	case ScopeFueling, ScopeTraining, ScopeRecovery, ScopeRace, ScopeGeneral:
		return true
	default:
		return false
	}
}

// Memory mirrors a coach_memory row. Date/ExpiresAt/ReviewAt serialize as
// YYYY-MM-DD strings (matching the training_plans.start_date convention);
// nullable ones use a pointer + omitempty. NeedsReview is a derived,
// non-persisted flag set when ReviewAt is on or before the grounding date.
type Memory struct {
	ID        uuid.UUID `json:"id"`
	Kind      Kind      `json:"kind"`
	Text      string    `json:"text"`
	Reason    *string   `json:"reason,omitempty"`
	Scope     *Scope    `json:"scope,omitempty"`
	Date      *string   `json:"date,omitempty"`       // YYYY-MM-DD; required when kind=recommendation
	ExpiresAt *string   `json:"expires_at,omitempty"` // YYYY-MM-DD; hard cutoff
	ReviewAt  *string   `json:"review_at,omitempty"`  // YYYY-MM-DD; soft "still true?"
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// NeedsReview is derived at grounding time (review_at <= as-of date), never
	// stored. Omitted from the plain CRUD responses; set on the context blocks.
	NeedsReview bool `json:"needs_review,omitempty"`
}
