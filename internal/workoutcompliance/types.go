// Package workoutcompliance scores how a completed workout was executed against
// the template it was compiled from — the TrainingPeaks "compliance" read
// (adherence = did it happen; compliance = was it executed as written). It is a
// compute-on-read leaf package: it owns no table and no repo, composing the
// workout row + its splits (via the workouts repo) with the workout's effective
// program (via the training-plan service) at request time and persisting
// nothing.
//
// It lives in its own package because the route is workout-anchored
// (`GET /workouts/{id}/compliance`) yet the logic needs workouts, workout-
// templates and training-plan, and training-plan already imports workouts — so
// the aggregator cannot live in either without an import cycle (the same
// situation as workoutfueling).
package workoutcompliance

import (
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Status values for a compliance Result.
const (
	StatusScored      = "scored"
	StatusUnavailable = "unavailable"
)

// Classification of an actual value against a resolved band.
const (
	ClassInBand = "in_band"
	ClassUnder  = "under"
	ClassOver   = "over"
)

// Unscorable reasons for a step's target.
const (
	ReasonZoneUnresolved = "zone_unresolved"  // a zone target reached the read unresolved (no/partial athlete config)
	ReasonActualMissing  = "actual_missing"   // the split lacks the metric this target needs (e.g. no power meter)
	ReasonUnsupportedKind = "unsupported_kind" // cadence / rpe / none — nothing per-lap to compare
	ReasonNoTarget       = "no_target"        // the step carries no target at all
)

// Result is the GET /workouts/{id}/compliance response. Status is "scored" (per-
// step array present, Score possibly null when nothing was scorable) or
// "unavailable" (lap count did not match the expanded step count — Reason and
// the two counts explain why, no per-step array).
type Result struct {
	WorkoutID    uuid.UUID    `json:"workout_id"`
	TemplateID   *uuid.UUID   `json:"template_id,omitempty"`
	Status       string       `json:"status"`
	Reason       *string      `json:"reason,omitempty"`
	PlannedSteps int          `json:"planned_steps"`
	ExecutedLaps int          `json:"executed_laps"`
	Score        *float64     `json:"score,omitempty"`
	StepsScored  int          `json:"steps_scored"`
	StepsInBand  int          `json:"steps_in_band"`
	Steps        []StepResult `json:"steps,omitempty"`
}

// StepResult is one expanded step matched to its lap. Iteration/Of carry repeat
// provenance ("interval 3 of 5") for steps that originated inside a repeat group.
type StepResult struct {
	StepIndex int         `json:"step_index"`
	Intent    string      `json:"intent,omitempty"`
	Iteration *int        `json:"iteration,omitempty"`
	Of        *int        `json:"of,omitempty"`
	Planned   PlannedStep `json:"planned"`
	Actual    ActualLap   `json:"actual"`
	// Target/Secondary/Duration are each present only when that dimension applies
	// to the step (a step may have a scorable duration but an unscorable target).
	Target    *TargetResult   `json:"target,omitempty"`
	Secondary *TargetResult   `json:"secondary,omitempty"`
	Duration  *DurationResult `json:"duration,omitempty"`
	Score     *float64        `json:"score,omitempty"`
}

// PlannedStep echoes the resolved (effective-program) target + duration the step
// was judged against, including the target's zone Origin provenance.
type PlannedStep struct {
	Duration        *workouttemplates.Duration `json:"duration,omitempty"`
	Target          *workouttemplates.Target   `json:"target,omitempty"`
	SecondaryTarget *workouttemplates.Target   `json:"secondary_target,omitempty"`
}

// ActualLap is the matched split's metrics (only what the split carried).
type ActualLap struct {
	DurationS   *float64 `json:"duration_s,omitempty"`
	DistanceM   *float64 `json:"distance_m,omitempty"`
	AvgHR       *int     `json:"avg_hr,omitempty"`
	AvgPowerW   *int     `json:"avg_power_w,omitempty"`
	AvgSpeedMPS *float64 `json:"avg_speed_mps,omitempty"`
}

// TargetResult scores one effort target (primary or secondary) against the lap.
// When Scorable is false only Reason is meaningful; otherwise Metric/Low/High/
// Actual/Classification/Delta/DeviationPct/Score are populated.
type TargetResult struct {
	Scorable       bool     `json:"scorable"`
	Reason         *string  `json:"reason,omitempty"`
	Metric         string   `json:"metric,omitempty"`
	Low            *float64 `json:"low,omitempty"`
	High           *float64 `json:"high,omitempty"`
	Actual         *float64 `json:"actual,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
	DeviationPct   *float64 `json:"deviation_pct,omitempty"`
	Score          *float64 `json:"score,omitempty"`
}

// DurationResult scores planned-vs-actual duration for a time/distance step.
// Kind is "time" or "distance"; lap_button/open steps produce no DurationResult.
type DurationResult struct {
	Kind           string   `json:"kind"`
	Planned        *float64 `json:"planned,omitempty"`
	Actual         *float64 `json:"actual,omitempty"`
	Ratio          *float64 `json:"ratio,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Score          *float64 `json:"score,omitempty"`
}
