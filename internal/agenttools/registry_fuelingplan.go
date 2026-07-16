package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Workout fueling plan read — a planned session's burn estimate + intake
// prescription. One GET, body forwarded verbatim.

func init() { registerMCPDomain(fuelingPlanSpecs()) }

// WorkoutFuelingPlanArgs is the input shape for `workout_fueling_plan`.
type WorkoutFuelingPlanArgs struct {
	WorkoutID  string   `json:"workout_id" jsonschema:"the PLANNED workout id to build a fueling plan for"`
	CarbsPerHr *float64 `json:"carbs_per_hr,omitempty" jsonschema:"the athlete's TESTED gut capacity in g/hr (> 0, <= 130); clamps the prescription's upper bound. Only pass a number the athlete has actually rehearsed — never a guess."`
}

func fuelingPlanSpecs() []Spec {
	return []Spec{
		{
			Name: "workout_fueling_plan",
			Description: "Plan the fueling for a PLANNED training session: what it will burn and what to " +
				"take in. Work is estimated as `planned_tss / 100 × effective FTP × 3.6` kJ (exact under " +
				"the TSS definition), energy via the standard cycling kJ≈kcal convention. Carb burn is " +
				"kcal × a CHO fraction chosen by the planned intensity factor (`< 0.60` → 45%, " +
				"`0.60–0.75` → 55%, `0.75–0.85` → 70%, `> 0.85` → 80% — harder efforts burn " +
				"proportionally more carbohydrate) ÷ 4 kcal/g. Intake follows the duration ladder: " +
				"`< 60 min` → nothing, `60–150 min` → 30–60 g/hr, `> 150 min` → 60–90 g/hr. " +
				"`projected_deficit_g` (burn − maximum prescribed intake) is the coaching number: a large " +
				"deficit means post-session carbs matter. Every input behind the numbers is echoed. " +
				"DIVISION OF LABOR: races carry AUTHORED per-leg plans — use `plan_race_fueling` for those. " +
				"This computes TRAINING-DAY prescriptions. For which DAYS need carbs (day-level " +
				"periodization), that's `fuel_plan`; this is the within-session plan for one workout. " +
				"CAPACITY: `carbs_per_hr` is the athlete's tested gut tolerance and comes from REHEARSAL " +
				"experience — this endpoint cannot discover it. Pass it only when the athlete has actually " +
				"trained that intake; otherwise leave it out and let the ladder stand. " +
				"Degradations state what they lack rather than guessing: `tss_missing` or `ftp_missing` " +
				"return the duration-based intake guidance with no burn estimate — still useful advice. " +
				"A non-planned workout returns 409: this is a pre-session question. " +
				"These are conventions and a planning anchor, not a metabolic lab — present the numbers as " +
				"a starting point to adjust from, not a measurement. Read-only; no idempotency-key is sent.",
			SchemaType: WorkoutFuelingPlanArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WorkoutFuelingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.CarbsPerHr != nil {
					q.Set("carbs_per_hr", strconv.FormatFloat(*a.CarbsPerHr, 'f', -1, 64))
				}
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/fueling-plan",
					Query:  q,
				}, nil
			},
		},
	}
}
