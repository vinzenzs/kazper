package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PlanCarbLoadArgs reflects the carb-load REST surface (GET for the pure
// compute, POST /apply when the agent passes apply=true). Optional fields
// are pointers so the wrapper can omit them — letting the REST defaults
// apply.
type PlanCarbLoadArgs struct {
	RaceDate          string   `json:"race_date" jsonschema:"race date in YYYY-MM-DD (must be today or later in the user's timezone)"`
	BodyWeightKg      float64  `json:"body_weight_kg" jsonschema:"athlete body weight in kilograms, 30..200"`
	DaysBefore        *int     `json:"days_before,omitempty" jsonschema:"carb-load days before race day, 0..7 (default 3). Sprint tri / short races: 1-2. 70.3: 3. Ironman: 3-4."`
	CarbsPerKgPerDay  *float64 `json:"carbs_per_kg_per_day,omitempty" jsonschema:"load-day multiplier, 1..20 g/kg (default 10, mid-range of the documented 8-12 g/kg; lower for athletes with GI sensitivity)"`
	RaceDayCarbsPerKg *float64 `json:"race_day_carbs_per_kg,omitempty" jsonschema:"race-morning multiplier, 0..10 g/kg (default 2)"`

	Apply          *bool  `json:"apply,omitempty" jsonschema:"when true, also writes the carb_g goal bounds (min-only) for each schedule day into per-date goal overrides — preserving any existing kcal/protein/other bounds on those dates. Default false (pure compute, no side effects)."`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; used only when apply=true (the read-only path ignores it)"`
}

func handlePlanCarbLoad(ctx context.Context, c *apiClient, args PlanCarbLoadArgs) *mcp.CallToolResult {
	if args.Apply != nil && *args.Apply {
		return handlePlanCarbLoadApply(ctx, c, args)
	}
	return handlePlanCarbLoadCompute(ctx, c, args)
}

func handlePlanCarbLoadCompute(ctx context.Context, c *apiClient, args PlanCarbLoadArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("race_date", args.RaceDate)
	q.Set("body_weight_kg", strconv.FormatFloat(args.BodyWeightKg, 'f', -1, 64))
	if args.DaysBefore != nil {
		q.Set("days_before", strconv.Itoa(*args.DaysBefore))
	}
	if args.CarbsPerKgPerDay != nil {
		q.Set("carbs_per_kg_per_day", strconv.FormatFloat(*args.CarbsPerKgPerDay, 'f', -1, 64))
	}
	if args.RaceDayCarbsPerKg != nil {
		q.Set("race_day_carbs_per_kg", strconv.FormatFloat(*args.RaceDayCarbsPerKg, 'f', -1, 64))
	}
	status, body, err := c.Get(ctx, "/race-prep/carb-load", q)
	return toToolResult(status, body, err)
}

// applyBody is the POST /race-prep/carb-load/apply request body. Build it
// from the validated args; the wrapper does not invent values.
type applyBody struct {
	RaceDate          string   `json:"race_date"`
	BodyWeightKg      float64  `json:"body_weight_kg"`
	DaysBefore        *int     `json:"days_before,omitempty"`
	CarbsPerKgPerDay  *float64 `json:"carbs_per_kg_per_day,omitempty"`
	RaceDayCarbsPerKg *float64 `json:"race_day_carbs_per_kg,omitempty"`
}

func handlePlanCarbLoadApply(ctx context.Context, c *apiClient, args PlanCarbLoadArgs) *mcp.CallToolResult {
	body := applyBody{
		RaceDate:          args.RaceDate,
		BodyWeightKg:      args.BodyWeightKg,
		DaysBefore:        args.DaysBefore,
		CarbsPerKgPerDay:  args.CarbsPerKgPerDay,
		RaceDayCarbsPerKg: args.RaceDayCarbsPerKg,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	// Derived key uses canonical JSON of args minus apply + idempotency_key;
	// explicit IdempotencyKey wins via effectiveIdempotencyKey. We strip the
	// apply field from the args by zeroing its pointer for the derivation
	// (deriveIdempotencyKey reads the args as-is otherwise).
	derivArgs := args
	derivArgs.Apply = nil
	key := effectiveIdempotencyKey(args.IdempotencyKey, "plan_carb_load", derivArgs)
	status, respBody, err := c.Post(ctx, "/race-prep/carb-load/apply", nil, raw, key)
	return toToolResult(status, respBody, err)
}

func registerRacePrepTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "plan_carb_load",
		Description: "Compute the carb-load schedule for a race. Returns a daily schedule of carb " +
			"targets in grams: 'days_before' load days plus race day. For sprint tri / short races, " +
			"consider days_before=1 or 2 (carb-load benefit plateaus). For 70.3 use the default 3. For " +
			"Ironman consider 3-4 days. The carbs_per_kg_per_day default of 10 sits in the middle of " +
			"the documented 8-12 g/kg range; lower for athletes who handle GI distress.\n\n" +
			"Pass `apply: true` to ALSO write the carb_g goal bounds (min-only) for each schedule day " +
			"into the per-date goal overrides — this is the recommended path for the standard race-prep " +
			"workflow. Existing kcal, protein_g, and other bounds on those dates are preserved (the " +
			"apply step writes only the carb target). The response includes an `applied` array reporting " +
			"per-date outcome: `{date, carbs_g_min, created}` where `created: false` means the apply " +
			"merged into a pre-existing override (e.g. a training-day template). When apply is omitted " +
			"or false, the endpoint stays pure-compute — no side effects — useful for 'what-if' " +
			"exploration before committing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PlanCarbLoadArgs) (*mcp.CallToolResult, any, error) {
		return handlePlanCarbLoad(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "recommend_workout_fuel",
		Description: "Compute a pre/intra/post fueling recommendation for ONE training or race session. " +
			"Two input modes; exactly one must be used:\n\n" +
			"  1. workout_id — pulls sport/duration/intensity_zone from the workouts row (intensity " +
			"derived from `tss` via the Coggan IF mapping; defaults to Z2 with a disclosure note when " +
			"TSS is absent).\n" +
			"  2. Explicit triplet — `sport` + `duration_min` + `intensity_zone` — for planned-tomorrow " +
			"sessions that don't have a workout row yet.\n\n" +
			"Body weight resolution: explicit `body_weight_kg` arg > rolling 7-day mean of stored weight " +
			"entries ending at today (inclusive) > most-recent stored entry strictly before today. With " +
			"no stored data and no override → 400 weight_data_missing.\n\n" +
			"Headline literature ratios (returned verbatim with rationale strings):\n" +
			"  pre:   1.0–2.0 g/kg by zone, [60, 120] min before (strength 0.5 g/kg [30, 90] min);\n" +
			"  intra: 30 g/hr for short Z1–2, 60 g/hr for tempo/threshold or 90–180 min, 90 g/hr for >180 " +
			"min — except sport=run which caps at 60 g/hr (GI tolerance); strength + swim ≤ 120 min " +
			"return `applicable: false`;\n" +
			"  post:  1.0 g/kg CHO + 0.3 g/kg protein in [0, 60] min after — the protein factor is the " +
			"SAME MPS threshold `protein_distribution` uses to flag `mps_effective: true` (single " +
			"literature constant across endpoints).\n\n" +
			"For race-week 24–72h pre-loading, use `plan_carb_load` (this tool answers per-session, not " +
			"per-block). To commit a recommendation as a real fueling entry, use `log_workout_fuel`. " +
			"Read-only; no idempotency-key.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecommendWorkoutFuelArgs) (*mcp.CallToolResult, any, error) {
		return handleRecommendWorkoutFuel(ctx, c, args), nil, nil
	})
}

// RecommendWorkoutFuelArgs is the MCP input shape. All pointers because the
// two input modes are at the args level (workout_id vs explicit triplet) and
// the REST endpoint validates exclusivity.
type RecommendWorkoutFuelArgs struct {
	WorkoutID     *string  `json:"workout_id,omitempty" jsonschema:"workout UUID. Pulls sport/duration/intensity from the row. Mutually exclusive with sport/duration_min/intensity_zone."`
	Sport         *string  `json:"sport,omitempty" jsonschema:"sport (bike|run|swim|strength|other). Required in explicit mode."`
	DurationMin   *int     `json:"duration_min,omitempty" jsonschema:"duration in minutes; > 0. Required in explicit mode."`
	IntensityZone *int     `json:"intensity_zone,omitempty" jsonschema:"intensity zone 1–5. Required in explicit mode."`
	BodyWeightKg  *float64 `json:"body_weight_kg,omitempty" jsonschema:"explicit body-weight override (kg); > 0. Otherwise the resolver picks from stored body-weight entries."`
}

func handleRecommendWorkoutFuel(ctx context.Context, c *apiClient, args RecommendWorkoutFuelArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.WorkoutID != nil {
		q.Set("workout_id", *args.WorkoutID)
	}
	if args.Sport != nil {
		q.Set("sport", *args.Sport)
	}
	if args.DurationMin != nil {
		q.Set("duration_min", strconv.Itoa(*args.DurationMin))
	}
	if args.IntensityZone != nil {
		q.Set("intensity_zone", strconv.Itoa(*args.IntensityZone))
	}
	if args.BodyWeightKg != nil {
		q.Set("body_weight_kg", strconv.FormatFloat(*args.BodyWeightKg, 'f', -1, 64))
	}
	status, body, err := c.Get(ctx, "/race-prep/recommend-workout-fuel", q)
	return toToolResult(status, body, err)
}
