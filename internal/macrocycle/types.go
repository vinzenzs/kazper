// Package macrocycle owns the macrocycles table: the season-level
// periodization container that orders existing training-phases into a yearly
// progression toward a goal race. A macrocycle is planning/visualization +
// coach-context only — it never enters the goals resolver or plan
// materialization. Membership lives on the phase (training_phases.macrocycle_id);
// this package never duplicates the per-period date ranges, only the season
// envelope. See openspec/changes/add-macrocycle-planning.
package macrocycle

import (
	"time"

	"github.com/google/uuid"
)

// Macrocycle mirrors a macrocycles row plus two resolved conveniences:
// RaceName (the joined name of the anchored race, null when unanchored) and
// Phases (the ordered member phases, populated only by the by-id read).
type Macrocycle struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	StartDate time.Time  `json:"start_date"`
	EndDate   time.Time  `json:"end_date"`
	RaceID    *uuid.UUID `json:"race_id"`
	// RaceName is the resolved name of the anchored race, null when RaceID is
	// unset. A convenience sibling so the coach can name the A-race without a
	// follow-up get_race call.
	RaceName *string `json:"race_name"`
	// Methodology is curated, cited Markdown "why this whole arc" prose, stored
	// verbatim; distinct from the operational Notes (mirrors a phase's
	// methodology). Null when unset.
	Methodology *string `json:"methodology"`
	Notes       *string `json:"notes"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Phases is the ordered set of member phases (the periods). The by-id read
	// always sets it (an empty, non-nil slice serializes as `[]`); list entries
	// leave it nil, serializing as `null` (no omitempty — an empty member set on
	// the by-id read must show `[]`, which omitempty would drop).
	Phases []*MemberPhase `json:"phases"`
}

// MemberPhase is the lite projection of a training_phases row that belongs to a
// macrocycle, ordered by macrocycle_ordinal (nulls last) then start_date. It
// carries the per-period progression targets so the whole yearly load
// progression presents in one read. Target fields are rounded at the response
// boundary; all three optional fields serialize as null when unset.
type MemberPhase struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	MacrocycleOrdinal *int      `json:"macrocycle_ordinal"`
	TargetWeeklyTSS   *float64  `json:"target_weekly_tss"`
	TargetWeeklyHours *float64  `json:"target_weekly_hours"`
}

// Covering is the narrow read the coach-context training bundle needs: the
// season covering an anchor date, its resolved race anchor (name + date, both
// null when unanchored), and the count of member phases. Returned by
// Repo.CoveringFor; never serialized directly (coachcontext maps it to its own
// lite shape).
type Covering struct {
	ID           uuid.UUID
	Name         string
	StartDate    time.Time
	EndDate      time.Time
	RaceID       *uuid.UUID
	RaceName     *string
	RaceDate     *time.Time
	TotalPeriods int
}

// CreateInput is what the POST /macrocycles handler passes after JSON decode.
type CreateInput struct {
	Name        string
	StartDate   time.Time
	EndDate     time.Time
	RaceID      *uuid.UUID
	Methodology *string
	Notes       *string
}

// PatchInput carries the optional editable fields. A nil pointer leaves the
// field unchanged. RaceID is tri-state: a non-nil RaceID sets a new anchor;
// ClearRaceID (the handler's translation of an empty string) clears it; nil
// with Clear=false leaves it unchanged.
type PatchInput struct {
	Name        *string
	StartDate   *time.Time
	EndDate     *time.Time
	RaceID      *uuid.UUID
	ClearRaceID bool
	Methodology *string
	Notes       *string
}

// HasUpdates reports whether at least one field is set.
func (p PatchInput) HasUpdates() bool {
	return p.Name != nil || p.StartDate != nil || p.EndDate != nil ||
		p.RaceID != nil || p.ClearRaceID || p.Methodology != nil || p.Notes != nil
}
