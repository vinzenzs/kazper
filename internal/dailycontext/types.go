// Package dailycontext exposes GET /context/daily — a single read that
// returns the day's adherence, totals, hydration, workouts, fueling, weight,
// training-phase context, and goal-override presence in one bundle.
// Composition-only over existing primitives: no schema, no writes.
package dailycontext

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/coachmemory"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/hydrationbalance"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/supplements"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/wellness"
)

// DailyContext is the top-level response shape. Each sub-block re-uses the
// existing capability shapes verbatim where possible (Adherence, Totals,
// Goals) so the agent's mental model — already trained on those shapes —
// doesn't have to relearn anything.
type DailyContext struct {
	Date        string             `json:"date"`
	TZ          string             `json:"tz"`
	Adherence   AdherenceBlock     `json:"adherence"`
	Nutrition   NutritionBlock     `json:"nutrition"`
	Hydration   HydrationBlock     `json:"hydration"`
	Workouts    []*WorkoutLite     `json:"workouts"`     // never nil; empty array on quiet days
	WorkoutFuel []*WorkoutFuelLite `json:"workout_fuel"` // never nil; empty array on quiet days
	// Memory is the active coach-memory folded into grounding (standing items +
	// recommendations for the date), needs_review flagged. Never nil; empty array
	// when there's no active memory. See widen-coach-recs-to-memory.
	Memory       []*coachmemory.Memory `json:"memory"`
	Weight       *WeightBlock          `json:"weight"` // nil when no entry ever logged
	Phase        *PhaseBlock           `json:"phase"`  // nil when no phase covers the date
	GoalOverride GoalOverrideBlock     `json:"goal_override"`
	// Today's and tomorrow's planned-load classification + suggested carb target,
	// beside the goals data so the morning check-in reads it without another
	// call. Omitted entirely when nothing is computable. Suggestions only — the
	// full week and the inputs behind each tier stay behind /nutrition/fuel-plan,
	// and applying a number is the goal-override PUT (add-periodized-fuel-targets).
	FuelPlan *FuelPlanBlock `json:"fuel_plan,omitempty"`
	// Today's and tomorrow's heat picture for planned OUTDOOR sessions — the
	// trigger for the coach's confirmed update flow when weather says a session
	// needs changing. Omitted entirely when nothing is heat-relevant or
	// computable (add-heat-adjusted-training).
	Heat *HeatBlock `json:"heat,omitempty"`
	// Same-day-or-null Garmin snapshots — no carryover (a stale recovery/fitness
	// reading is misleading). nil when no snapshot exists for the date.
	Recovery *recoverymetrics.Snapshot `json:"recovery"`
	Fitness  *fitnessmetrics.Snapshot  `json:"fitness"`
	// Garmin's daily water-balance estimate (sweat out, activity intake in, goal).
	// Same-day-or-null. Distinct from the Hydration block (logged intake).
	HydrationBalance *hydrationbalance.Snapshot `json:"hydration_balance"`
	// Today's subjective wellness entry (self-reported scores + note), beside the
	// objective recovery snapshot. Omitted entirely when unlogged — never an empty
	// object. History stays behind the wellness endpoints (add-wellness-diary).
	Wellness *wellness.Entry `json:"wellness,omitempty"`
	// Today's supplement intakes (creatine, iron, …), ascending. Omitted entirely
	// when none were logged today. Unit-isolated — feeds no macro total
	// (add-supplement-log).
	Supplements []*supplements.Entry `json:"supplements,omitempty"`
}

// AdherenceBlock mirrors the summary.Daily adherence + source fields.
type AdherenceBlock struct {
	GoalSource string            `json:"goal_source"`
	PhaseName  string            `json:"phase_name,omitempty"`
	Adherence  summary.Adherence `json:"adherence,omitempty"`
}

// NutritionBlock carries the day's totals plus a count. Full meal entries
// are intentionally omitted — call daily_summary for the per-entry view.
type NutritionBlock struct {
	Totals       summary.Totals `json:"totals"`
	EntriesCount int            `json:"entries_count"`
}

// HydrationBlock carries the day's total ml + entry count. Total is a
// scalar (not nullable) — zero is the meaningful empty state.
type HydrationBlock struct {
	TotalMl      float64 `json:"total_ml"`
	EntriesCount int     `json:"entries_count"`
}

// WorkoutLite is a compact projection of a workouts row. Drops fields the
// aggregator doesn't surface (avg_hr, tss, external_id) to keep the bundle
// agent-readable; the agent calls get_workout if it needs the full row.
type WorkoutLite struct {
	ID          uuid.UUID `json:"id"`
	Sport       string    `json:"sport"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	DurationMin float64   `json:"duration_min"`
	KcalBurned  *float64  `json:"kcal_burned,omitempty"`
	Notes       *string   `json:"notes,omitempty"`
}

// WorkoutFuelLite mirrors workoutfuel.Entry minus updated_at / created_at.
type WorkoutFuelLite struct {
	ID          uuid.UUID  `json:"id"`
	LoggedAt    time.Time  `json:"logged_at"`
	Name        string     `json:"name"`
	QuantityMl  *float64   `json:"quantity_ml,omitempty"`
	CarbsG      *float64   `json:"carbs_g,omitempty"`
	SodiumMg    *float64   `json:"sodium_mg,omitempty"`
	PotassiumMg *float64   `json:"potassium_mg,omitempty"`
	CaffeineMg  *float64   `json:"caffeine_mg,omitempty"`
	WorkoutID   *uuid.UUID `json:"workout_id,omitempty"`
}

// WeightBlock carries the latest body-weight reading relevant to the
// requested day. IsCarryover discriminates "fresh entry today" from
// "last seen N days ago" — the agent uses it to decide whether to nudge.
type WeightBlock struct {
	LoggedAt     time.Time `json:"logged_at"`
	WeightKg     float64   `json:"weight_kg"`
	BodyFatPct   *float64  `json:"body_fat_pct,omitempty"`
	MuscleMassKg *float64  `json:"muscle_mass_kg,omitempty"`
	BodyWaterPct *float64  `json:"body_water_pct,omitempty"`
	BoneMassKg   *float64  `json:"bone_mass_kg,omitempty"`
	BMI          *float64  `json:"bmi,omitempty"`
	IsCarryover  bool      `json:"is_carryover"`
}

// PhaseBlock is the full phase row covering the date (resolver-picked when
// overlapping). Carries default_template_name so the agent can describe the
// period without a follow-up call.
type PhaseBlock struct {
	ID                  uuid.UUID                `json:"id"`
	Name                string                   `json:"name"`
	Type                trainingphases.PhaseType `json:"type"`
	StartDate           time.Time                `json:"start_date"`
	EndDate             time.Time                `json:"end_date"`
	DefaultTemplateID   *uuid.UUID               `json:"default_template_id,omitempty"`
	DefaultTemplateName *string                  `json:"default_template_name,omitempty"`
	Notes               *string                  `json:"notes,omitempty"`
}

// GoalOverrideBlock uses a two-field shape so the agent's check is
// `if context.goal_override.present { ... }` — stable regardless of
// whether the goals object is null or non-null.
type GoalOverrideBlock struct {
	Present bool         `json:"present"`
	Goals   *goals.Goals `json:"goals"`
}

// FuelPlanBlock is the check-in view of fuel periodization: "today easy, 5 g/kg;
// tomorrow heavy, 9 — front-load tonight". Deliberately compact — the sessions,
// effective goals and deltas behind each tier live on /nutrition/fuel-plan.
type FuelPlanBlock struct {
	Today    *FuelPlanDay `json:"today,omitempty"`
	Tomorrow *FuelPlanDay `json:"tomorrow,omitempty"`
}

// HeatBlock is the check-in view of heat: only the days that actually have a
// heat-relevant session. Indoor, absent and uncomputable sessions are left out
// rather than represented as nulls — the block exists to say "this needs
// attention", so a day with nothing to say has no entry.
type HeatBlock struct {
	Today    *HeatDay `json:"today,omitempty"`
	Tomorrow *HeatDay `json:"tomorrow,omitempty"`
}

// HeatDay is one planned session's heat summary. Deliberately compact — the
// conditions, evidence and fluid note live on /workouts/{id}/heat.
type HeatDay struct {
	WorkoutID       uuid.UUID `json:"workout_id"`
	Date            string    `json:"date"`
	LocationName    string    `json:"location_name"`
	HeatLoadC       float64   `json:"heat_load_c"`
	Acclimatization string    `json:"acclimatization"`
	ReductionPct    float64   `json:"suggested_reduction_pct"`
	AssumedOutdoor  bool      `json:"assumed_outdoor,omitempty"`
	// AssumedStart carries the habitual start that was assumed when the session
	// was scheduled by date alone — the check-in should read "at your usual
	// 06:00", not quote an hour it invented silently.
	AssumedStart string `json:"assumed_start,omitempty"`
}

// FuelPlanDay is one day's classification. SuggestedCarbsG is absent when no
// body-weight data backs the g/kg denominator — the tier still means something
// without it. PlanMissing marks a day the plan doesn't reach, so its rest tier
// doesn't read as a planned rest day.
type FuelPlanDay struct {
	Date            string   `json:"date"`
	Tier            string   `json:"tier"`
	CarbsGPerKg     float64  `json:"carbs_g_per_kg"`
	SuggestedCarbsG *float64 `json:"suggested_carbs_g,omitempty"`
	PlanMissing     bool     `json:"plan_missing,omitempty"`
}
