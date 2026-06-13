// Package achievements mirrors Garmin earned badges and ad-hoc challenges as
// slowly-changing, upsert-by-external-id rows. Coaching/reference context; never
// feeds nutrition computation. Personal records are NOT here — they live in the
// separate personal-records capability.
package achievements

import (
	"time"

	"github.com/google/uuid"
)

// Kind discriminates a badge from an (ad-hoc) challenge.
type Kind string

const (
	KindBadge     Kind = "badge"
	KindChallenge Kind = "challenge"
)

func ValidKind(s string) bool {
	switch Kind(s) {
	case KindBadge, KindChallenge:
		return true
	}
	return false
}

// Achievement mirrors an achievements row. Identity is the backend `id`;
// `external_id` is the stable Garmin badge/challenge id the upsert dedups on.
type Achievement struct {
	ID          uuid.UUID  `json:"id"`
	ExternalID  string     `json:"external_id"`
	Kind        Kind       `json:"kind"`
	Name        string     `json:"name"`
	EarnedAt    *time.Time `json:"earned_at,omitempty"`
	ProgressPct *float64   `json:"progress_pct,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
