// Package fuelplan classifies each day by its PLANNED training load and
// suggests a carbohydrate target to match — "fuel for the work required".
//
// It is a read-only aggregator over planned workouts (the load signal), the
// body-weight trend (the g/kg denominator), and goals (the comparison), living
// in its own package for the workoutfueling reason.
//
// Everything here is a suggestion. The package never writes a goal or an
// override: applying a day's number is the existing deliberate per-date
// override PUT, proposed by the coach and confirmed by the athlete.
package fuelplan

import (
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Tier is a day's planned-load classification.
type Tier string

const (
	TierRest     Tier = "rest"
	TierEasy     Tier = "easy"
	TierModerate Tier = "moderate"
	TierHeavy    Tier = "heavy"
)

// Tier thresholds and the g/kg ladder — fixed constants, not configuration
// (design D2; the intensity-distribution 20/75 precedent). The sports-nutrition
// literature brackets these ranges; a single athlete doesn't need a knob, and
// both the tier and the inputs behind it are in the response, so a disagreement
// is auditable rather than buried.
const (
	easyMaxTSS     = 60.0  // total planned TSS < 60 → easy
	moderateMaxTSS = 150.0 // 60..150 → moderate; > 150 → heavy

	// longSessionMin makes a single long session heavy regardless of TSS — a
	// 4-hour endurance ride is glycogen-expensive even at a low intensity
	// factor, which TSS alone under-reports.
	longSessionMin = 150.0

	restCarbsGPerKg     = 3.0
	easyCarbsGPerKg     = 5.0
	moderateCarbsGPerKg = 7.0
	heavyCarbsGPerKg    = 9.0
)

// MaxRangeDays caps the window; DefaultWindowDays is today plus six.
const (
	MaxRangeDays      = 14
	DefaultWindowDays = 7
)

// ReasonWeightMissing degrades gram targets (not tiers) when no weigh-in backs
// the g/kg denominator. Tiers are weight-free — only the multiplication needs
// mass, so the classification still ships (design D3).
const ReasonWeightMissing = "weight_missing"

// Session is one planned workout on a day, echoed so the tier is auditable
// against the inputs that produced it.
type Session struct {
	WorkoutID          uuid.UUID `json:"workout_id"`
	Sport              string    `json:"sport"`
	PlannedTSS         *float64  `json:"planned_tss,omitempty"`
	PlannedDurationMin *float64  `json:"planned_duration_min,omitempty"`
}

// Day is one classified date.
//
// PlanMissing distinguishes "the athlete rests that day" from "the plan doesn't
// reach that far" — both classify rest, but a rest suggestion and a no-data
// suggestion must not look alike (design D1).
type Day struct {
	Date            string       `json:"date"`
	Tier            Tier         `json:"tier"`
	CarbsGPerKg     float64      `json:"carbs_g_per_kg"`
	PlannedTSSTotal float64      `json:"planned_tss_total"`
	Sessions        []Session    `json:"sessions"`
	PlanMissing     bool         `json:"plan_missing,omitempty"`
	SuggestedCarbsG *float64     `json:"suggested_carbs_g,omitempty"`
	GoalCarbsG      *goals.Range `json:"goal_carbs_g,omitempty"`
	DeltaG          *float64     `json:"delta_g,omitempty"`
}

// Weight is the g/kg denominator the suggestions were multiplied by, echoed
// with the date it was taken at so a stale trend is visible.
type Weight struct {
	TrendKg float64 `json:"trend_kg"`
	Date    string  `json:"date"`
}

// Plan is the response shape for GET /nutrition/fuel-plan.
type Plan struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	TZ     string  `json:"tz"`
	Weight *Weight `json:"weight,omitempty"`
	Reason *string `json:"reason,omitempty"`
	Days   []Day   `json:"days"`
}

// classify derives a day's tier from its planned sessions. No sessions is rest.
// A session carrying neither TSS nor duration still counts as training — it
// contributes 0 to the total and lands the day in easy, with the empty inputs
// visible in the echoed session (design: plan-quality risk).
func classify(sessions []Session) (Tier, float64) {
	if len(sessions) == 0 {
		return TierRest, 0
	}

	var total float64
	var hasLongSession bool
	for _, s := range sessions {
		if s.PlannedTSS != nil {
			total += *s.PlannedTSS
		}
		if s.PlannedDurationMin != nil && *s.PlannedDurationMin >= longSessionMin {
			hasLongSession = true
		}
	}
	// The long-session rule wins outright — the summed total is still reported
	// so the override is visible against the TSS that didn't earn it.
	if hasLongSession {
		return TierHeavy, total
	}

	switch {
	case total > moderateMaxTSS:
		return TierHeavy, total
	case total >= easyMaxTSS:
		return TierModerate, total
	default:
		return TierEasy, total
	}
}

// carbsGPerKg maps a tier onto the ladder.
func carbsGPerKg(t Tier) float64 {
	switch t {
	case TierHeavy:
		return heavyCarbsGPerKg
	case TierModerate:
		return moderateCarbsGPerKg
	case TierEasy:
		return easyCarbsGPerKg
	default:
		return restCarbsGPerKg
	}
}

// targetReference reduces a goal range to the scalar the delta is measured
// against: the midpoint when both bounds are set, else whichever bound exists.
// Mirrors summary.targetReference so "distance from goal" means the same thing
// in both places.
func targetReference(r goals.Range) *float64 {
	switch {
	case r.Min != nil && r.Max != nil:
		mid := (*r.Min + *r.Max) / 2
		return &mid
	case r.Min != nil:
		return r.Min
	case r.Max != nil:
		return r.Max
	}
	return nil
}

// buildDay classifies one date and, when a weight is available, prices the tier
// in grams and compares it against the date's effective goal.
func buildDay(date string, sessions []Session, planMissing bool, weightKg *float64, goal *goals.Range) Day {
	if sessions == nil {
		sessions = []Session{}
	}
	tier, total := classify(sessions)
	gPerKg := carbsGPerKg(tier)

	d := Day{
		Date:            date,
		Tier:            tier,
		CarbsGPerKg:     gPerKg,
		PlannedTSSTotal: numfmt.Round1(total),
		Sessions:        sessions,
		PlanMissing:     planMissing,
	}
	if goal != nil {
		d.GoalCarbsG = goal
	}
	if weightKg == nil {
		return d
	}

	suggested := gPerKg * *weightKg
	rounded := numfmt.Round1(suggested)
	d.SuggestedCarbsG = &rounded

	if goal != nil {
		if ref := targetReference(*goal); ref != nil {
			// Delta is computed on the unrounded suggestion and rounded once,
			// at the boundary, like every other nutrient number.
			delta := numfmt.Round1(suggested - *ref)
			d.DeltaG = &delta
		}
	}
	return d
}
