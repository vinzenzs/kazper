// Package racepacing computes a deterministic per-leg race pacing plan on read
// (bike power band as %FTP, run pace band vs threshold pace, swim pace band vs
// CSS — all banded by leg duration) over the race calendar owned by the
// race-fueling-plan capability, plus persisted per-leg manual overrides with a
// compute-on-read fallback.
//
// It is the deliberate sibling of race-fueling-plan (one concern per
// capability, unit-isolated shapes) and reads the race/leg tables without
// owning them — the multi-repo aggregator pattern of internal/workoutfueling:
// it depends on the races repo (race + legs) and the athleteconfig repo
// (thresholds), and owns only its overrides table. Nothing computed is
// persisted; only overrides are.
package racepacing

import (
	"time"

	"github.com/google/uuid"
)

// Source marks where a leg's reported band came from.
const (
	SourceComputed = "computed" // duration-banded from a threshold
	SourceOverride = "override" // a stored manual override
	SourceNone     = "none"     // uncomputable (no model / no duration / no threshold)
)

// PacingPlan is the GET /races/{id}/pacing-plan response.
type PacingPlan struct {
	RaceID            uuid.UUID       `json:"race_id"`
	RaceName          string          `json:"race_name"`
	RaceDate          string          `json:"race_date"`
	TotalDurationMin  *int            `json:"total_duration_min,omitempty"`
	Legs              []LegPacingPlan `json:"legs"`
	EstimatedTSSTotal float64         `json:"estimated_tss_total"`
	TSSComplete       bool            `json:"tss_complete"`
	// MissingThresholds is the union of the legs' missing thresholds (empty slice
	// serialized, never null, so a client can iterate unconditionally).
	MissingThresholds []string `json:"missing_thresholds"`
}

// LegPacingPlan is one leg's pacing entry. Discipline-inappropriate target
// fields are omitted (unit isolation): power only on bike, sec_per_km only on
// run, sec_per_100m only on swim.
type LegPacingPlan struct {
	Ordinal             int    `json:"ordinal"`
	Discipline          string `json:"discipline"`
	ExpectedDurationMin *int   `json:"expected_duration_min,omitempty"`
	Source              string `json:"source"`

	// Bike (power) — watts.
	TargetPowerLowW  *int `json:"target_power_low_w,omitempty"`
	TargetPowerHighW *int `json:"target_power_high_w,omitempty"`
	// Run (pace) — seconds per km.
	TargetPaceLowSecPerKM  *float64 `json:"target_pace_low_sec_per_km,omitempty"`
	TargetPaceHighSecPerKM *float64 `json:"target_pace_high_sec_per_km,omitempty"`
	// Swim (pace) — seconds per 100 m.
	TargetPaceLowSecPer100m  *float64 `json:"target_pace_low_sec_per_100m,omitempty"`
	TargetPaceHighSecPer100m *float64 `json:"target_pace_high_sec_per_100m,omitempty"`

	IntensityFactor *float64 `json:"intensity_factor,omitempty"`
	EstimatedTSS    *float64 `json:"estimated_tss,omitempty"`
	// MissingThresholds names the unset athlete-config field(s) this leg needed.
	MissingThresholds []string `json:"missing_thresholds,omitempty"`
	Rationale         string   `json:"rationale"`
}

// Override mirrors a race_leg_pacing_overrides row. Exactly one unit family is
// populated (both low and high); the others are nil.
type Override struct {
	RaceID    uuid.UUID
	Ordinal   int
	Note      *string
	CreatedAt time.Time
	UpdatedAt time.Time

	TargetPowerLowW          *int
	TargetPowerHighW         *int
	TargetPaceLowSecPerKM    *float64
	TargetPaceHighSecPerKM   *float64
	TargetPaceLowSecPer100m  *float64
	TargetPaceHighSecPer100m *float64
}

// family reports which unit family the override populates.
func (o Override) family() targetFamily {
	switch {
	case o.TargetPowerLowW != nil:
		return familyPower
	case o.TargetPaceLowSecPerKM != nil:
		return familyPaceKM
	case o.TargetPaceLowSecPer100m != nil:
		return familyPace100m
	default:
		return familyNone
	}
}

// targetFamily is a unit family a target/override can belong to.
type targetFamily int

const (
	familyNone targetFamily = iota
	familyPower
	familyPaceKM
	familyPace100m
)

// OverrideInput is the validated PUT body (before persistence). Exactly one unit
// family must be populated; the service validates before it reaches the repo.
type OverrideInput struct {
	TargetPowerLowW          *int     `json:"target_power_low_w,omitempty"`
	TargetPowerHighW         *int     `json:"target_power_high_w,omitempty"`
	TargetPaceLowSecPerKM    *float64 `json:"target_pace_low_sec_per_km,omitempty"`
	TargetPaceHighSecPerKM   *float64 `json:"target_pace_high_sec_per_km,omitempty"`
	TargetPaceLowSecPer100m  *float64 `json:"target_pace_low_sec_per_100m,omitempty"`
	TargetPaceHighSecPer100m *float64 `json:"target_pace_high_sec_per_100m,omitempty"`
	Note                     *string  `json:"note,omitempty"`
}
