// Package coachcontext exposes aggregate, composition-only read bundles the
// in-app coach grounds on before giving training or recovery advice — the
// training and recovery siblings of internal/dailycontext's nutrition bundle.
// Each endpoint fans out across existing read repos in parallel and returns one
// shape; nothing is stored and there is no migration.
package coachcontext

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/coachmemory"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

// PhaseLite is the training-phase slice of the training context.
type PhaseLite struct {
	ID        uuid.UUID                `json:"id"`
	Name      string                   `json:"name"`
	Type      trainingphases.PhaseType `json:"type"`
	StartDate time.Time                `json:"start_date"`
	EndDate   time.Time                `json:"end_date"`
	// Methodology is the covering phase's curated "why" prose (Markdown, null
	// when unset), so the coach has the current phase's reasoning in the same
	// grounding call.
	Methodology *string `json:"methodology"`
}

// MacrocycleLite is the season slice of the training context: the macrocycle
// covering the anchor date, its race anchor (null fields when unanchored), and
// where the current period sits in the progression. Composition-only — it does
// not affect adherence or the covering phase. Null on the bundle when no season
// covers the date. See add-macrocycle-planning.
type MacrocycleLite struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	// Race anchor — all null when the season has no race_id.
	RaceID   *uuid.UUID `json:"race_id"`
	RaceName *string    `json:"race_name"`
	RaceDate *time.Time `json:"race_date"`
	// DaysToRace is race_date − anchor_date in whole days, present only when the
	// season is race-anchored.
	DaysToRace *int `json:"days_to_race"`
	// CurrentPhaseOrdinal is the covering phase's macrocycle_ordinal when that
	// phase belongs to this season, else null. TotalPeriods is the count of
	// phases linked to the season.
	CurrentPhaseOrdinal *int `json:"current_phase_ordinal"`
	TotalPeriods        int  `json:"total_periods"`
}

// WorkoutLite is a compact workout for the recent/upcoming lists — enough to
// reason about load and schedule without the full splits/sets detail (use
// get_workout / list_workouts for that).
type WorkoutLite struct {
	ID          uuid.UUID `json:"id"`
	Sport       string    `json:"sport"`
	Status      string    `json:"status"`
	Name        *string   `json:"name,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	DurationMin float64   `json:"duration_min"`
	KcalBurned  *float64  `json:"kcal_burned,omitempty"`
	TSS         *float64  `json:"tss,omitempty"`
}

// LoadSummary aggregates the recent completed workouts in the lookback window.
type LoadSummary struct {
	Count            int            `json:"count"`
	TotalDurationMin float64        `json:"total_duration_min"`
	TotalKcal        float64        `json:"total_kcal"`
	BySport          map[string]int `json:"by_sport"`
}

// TrainingContext is the GET /context/training bundle.
type TrainingContext struct {
	Date          string     `json:"date"`
	TZ            string     `json:"tz"`
	LookbackDays  int        `json:"lookback_days"`
	LookaheadDays int        `json:"lookahead_days"`
	Phase         *PhaseLite `json:"phase"`
	// Macrocycle is the season covering the anchor date + the current period's
	// position in the progression; null when no season covers the date.
	Macrocycle *MacrocycleLite          `json:"macrocycle"`
	Fitness    *fitnessmetrics.Snapshot `json:"fitness"`
	// ACWR is the acute:chronic load ratio, derived (acute ÷ chronic) only when
	// both loads are present; null otherwise. Never stored.
	ACWR *float64 `json:"acwr"`
	// AthleteConfig is the singleton physiology config (FTP, thresholds, HR/power
	// zones) so the coach grounds intensity advice on the athlete's zones in the
	// same call; null when no config row has been set.
	AthleteConfig *athleteconfig.AthleteConfig `json:"athlete_config"`
	// GarminDetected is the latest Garmin-detected physiology (advisory), so the
	// coach sees drift against the confirmed config in one read; null when no
	// detection has been recorded. (separate-garmin-threshold-detection)
	GarminDetected *athleteconfig.GarminDetectedThresholds `json:"garmin_detected"`
	// ThresholdSources is the active garmin-sourced field policy — which fields
	// computations read from Garmin instead of the confirmed config; empty slice
	// when all-manual.
	ThresholdSources []string `json:"threshold_sources"`
	// Effective is the resolved physiology computations actually consume (config
	// with garmin-sourced fields swapped for the detection, per-field annotated);
	// null when neither a config nor an applied detection exists.
	Effective *athleteconfig.EffectiveConfig `json:"effective"`
	// WattsPerKg is power-to-weight, derived (ftp_watts ÷ latest bodyweight kg)
	// only when both are present and bodyweight is non-zero; null otherwise.
	// Never stored.
	WattsPerKg       *float64       `json:"watts_per_kg"`
	RecentLoad       LoadSummary    `json:"recent_load"`
	RecentWorkouts   []*WorkoutLite `json:"recent_workouts"`
	UpcomingWorkouts []*WorkoutLite `json:"upcoming_workouts"`
	// Memory is the active coach-memory folded into grounding: standing items
	// always, plus recommendations dated within the lookback window; needs_review
	// flagged. Never nil; empty array when there's no active memory. This is what
	// lets the MCP agent ground on what the in-app coach was told (and vice-versa)
	// without sharing conversation transcripts. See widen-coach-recs-to-memory.
	Memory []*coachmemory.Memory `json:"memory"`
}

// RecoveryContext is the GET /context/recovery bundle.
type RecoveryContext struct {
	Date   string                      `json:"date"`
	Days   int                         `json:"days"`
	Latest *recoverymetrics.Snapshot   `json:"latest"`
	Recent []*recoverymetrics.Snapshot `json:"recent"`
}
