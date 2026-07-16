package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Workouts domain — the desktop coach's full workout CRUD plus brick/multisport
// fulfillment and the per-workout fueling summary. Ported from
// internal/mcpserver/tools_workouts.go onto the shared registry
// (unify-mcp-tool-registry). The arg structs, descriptions, and REST mappings
// are byte-faithful to the prior bespoke registrations so the announced schema
// and the wire shape are unchanged.
//
// log_workout, list_workouts, get_workout, patch_workout, and delete_workout
// share names with no chat tool here; they are MCP-only by construction.

func init() { registerMCPDomain(workoutsSpecs()) }

// LogWorkoutArgs is the input for the log_workout tool. Every nullable column
// is a pointer so absent → null in the POST body.
type LogWorkoutArgs struct {
	ExternalID      *string  `json:"external_id,omitempty" jsonschema:"writer-supplied dedup key, e.g. 'garmin:1234567'. Garmin and other sourced writers SHOULD set this. For agent-driven manual entries, leave it unset."`
	Source          string   `json:"source" jsonschema:"provenance: 'garmin' | 'manual' | 'other'. Use 'manual' for agent-driven entries."`
	Sport           string   `json:"sport" jsonschema:"'run' | 'bike' | 'swim' | 'strength' | 'other'"`
	Status          string   `json:"status,omitempty" jsonschema:"'completed' (default) for a session that happened, or 'planned' for a scheduled future session. Planned workouts may be future-dated (up to 1 year); completed ones cannot be more than 24h ahead."`
	Name            *string  `json:"name,omitempty" jsonschema:"optional human-readable label, e.g. 'Morning Z2 ride'"`
	StartedAt       string   `json:"started_at" jsonschema:"RFC 3339 timestamp the workout started"`
	EndedAt         string   `json:"ended_at" jsonschema:"RFC 3339 timestamp the workout ended; must be after started_at"`
	KcalBurned      *float64 `json:"kcal_burned,omitempty" jsonschema:"calories burned during the session; positive number"`
	AvgHR           *int     `json:"avg_hr,omitempty" jsonschema:"average heart rate in bpm; positive integer"`
	TSS             *float64 `json:"tss,omitempty" jsonschema:"Training Stress Score; the intensity signal. Non-negative."`
	RPE             *int     `json:"rpe,omitempty" jsonschema:"Borg CR-10 perceived effort, integer 1..10. Per session — logged after the workout. Nullable; omit for non-rehearsal rides (Z1 spins, gym sessions, etc.)."`
	GIDistressScore *int     `json:"gi_distress_score,omitempty" jsonschema:"GI distress severity, integer 1..5 (1 = no distress, 5 = severe / couldn't continue). Per session — the rehearsal-outcome signal that lets you iterate fueling strategy. Nullable."`
	DistanceM       *float64 `json:"distance_m,omitempty" jsonschema:"session distance in METRES (> 0). Nullable; omit if the source did not measure it."`
	AvgPowerW       *int     `json:"avg_power_w,omitempty" jsonschema:"average power in WATTS (positive integer). Nullable."`
	TemperatureC    *float64 `json:"temperature_c,omitempty" jsonschema:"ambient temperature in °C, range -40..60. Heat context for fueling. Nullable."`
	SweatLossML     *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"estimated sweat loss in MILLILITRES (> 0). Personalises fluid targets. Nullable."`
	SessionGroup    *string  `json:"session_group,omitempty" jsonschema:"free-text key linking the legs of a brick/multisport session — set the SAME key on every leg (e.g. the Garmin parent activity id) so they can be fetched together. Nullable."`
	Notes           *string  `json:"notes,omitempty" jsonschema:"free-text notes (e.g. how the fueling went, which product caused issues at minute N)"`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. Note: writers normally rely on external_id for dedup, not this header."`
}

type ListWorkoutsArgs struct {
	From         string  `json:"from" jsonschema:"inclusive RFC 3339 lower bound on started_at"`
	To           string  `json:"to" jsonschema:"inclusive RFC 3339 upper bound on started_at; max 92 days from 'from'"`
	SessionGroup *string `json:"session_group,omitempty" jsonschema:"optional: narrow to the legs of one brick/multisport session (exact-match on the session_group key). The from/to window is still required."`
	Status       *string `json:"status,omitempty" jsonschema:"optional: filter by lifecycle status 'planned' or 'completed'. The from/to window is still required."`
}

type GetWorkoutArgs struct {
	ID string `json:"id" jsonschema:"the workout id"`
}

type WorkoutAdherenceArgs struct {
	From        string  `json:"from" jsonschema:"inclusive start date YYYY-MM-DD (local)"`
	To          string  `json:"to" jsonschema:"inclusive end date YYYY-MM-DD (local); max 92-day span"`
	TZ          *string `json:"tz,omitempty" jsonschema:"optional IANA timezone for the date window and the now-comparison (defaults to the configured user timezone)"`
	PlanID      *string `json:"plan_id,omitempty" jsonschema:"optional: restrict to workouts whose plan slot belongs to this training-plan id (off-plan completed work is excluded)"`
	MissedLimit *int    `json:"missed_limit,omitempty" jsonschema:"optional cap on the missed_sessions list (1-200; default 50). missed_sessions_truncated flags a dropped tail."`
	ZeroFill    *bool   `json:"zero_fill,omitempty" jsonschema:"optional: when true the weekly trend emits every week in span (zeroed empty weeks) for a continuous axis"`
}

type WorkoutComplianceArgs struct {
	WorkoutID string `json:"workout_id" jsonschema:"the completed, template-linked workout id to score per-step execution compliance for"`
}

type PatchWorkoutArgs struct {
	ID         string   `json:"id" jsonschema:"the workout id to update"`
	Name       *string  `json:"name,omitempty" jsonschema:"new label"`
	Notes      *string  `json:"notes,omitempty" jsonschema:"new free-text notes"`
	KcalBurned *float64 `json:"kcal_burned,omitempty" jsonschema:"corrected kcal_burned (positive)"`
	AvgHR      *int     `json:"avg_hr,omitempty" jsonschema:"corrected average heart rate (positive)"`
	TSS        *float64 `json:"tss,omitempty" jsonschema:"corrected TSS (non-negative)"`
	// RPE + GI use json.RawMessage so the wrapper can encode three states:
	// absent (leave unchanged), integer (set), JSON null (clear to NULL).
	// Use the helper fields RPE/GIDistressScore for set, ClearRPE/ClearGI for null-clear.
	RPE                  *int `json:"rpe,omitempty" jsonschema:"Borg CR-10 perceived effort 1..10. Per session, set after the workout. Omit to leave unchanged."`
	ClearRPE             bool `json:"clear_rpe,omitempty" jsonschema:"set true to clear rpe to null (retract a previously logged value). Mutually exclusive with rpe."`
	GIDistressScore      *int `json:"gi_distress_score,omitempty" jsonschema:"GI distress severity 1..5. Per session. Omit to leave unchanged."`
	ClearGIDistressScore bool `json:"clear_gi_distress_score,omitempty" jsonschema:"set true to clear gi_distress_score to null. Mutually exclusive with gi_distress_score."`

	// Ingestion metrics — same tri-state: value sets, Clear* clears to null, omit leaves unchanged.
	DistanceM         *float64 `json:"distance_m,omitempty" jsonschema:"corrected distance in METRES (> 0). Omit to leave unchanged."`
	ClearDistanceM    bool     `json:"clear_distance_m,omitempty" jsonschema:"set true to clear distance_m to null."`
	AvgPowerW         *int     `json:"avg_power_w,omitempty" jsonschema:"corrected average power in WATTS (positive integer). Omit to leave unchanged."`
	ClearAvgPowerW    bool     `json:"clear_avg_power_w,omitempty" jsonschema:"set true to clear avg_power_w to null."`
	TemperatureC      *float64 `json:"temperature_c,omitempty" jsonschema:"corrected ambient temperature in °C (-40..60). Omit to leave unchanged."`
	ClearTemperatureC bool     `json:"clear_temperature_c,omitempty" jsonschema:"set true to clear temperature_c to null."`
	SweatLossML       *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"corrected estimated sweat loss in MILLILITRES (> 0). Omit to leave unchanged."`
	ClearSweatLossML  bool     `json:"clear_sweat_loss_ml,omitempty" jsonschema:"set true to clear sweat_loss_ml to null."`
	SessionGroup      *string  `json:"session_group,omitempty" jsonschema:"set the brick/multisport group key. Omit to leave unchanged."`
	ClearSessionGroup bool     `json:"clear_session_group,omitempty" jsonschema:"set true to un-group this leg (clear session_group to null)."`
	Status            *string  `json:"status,omitempty" jsonschema:"set the lifecycle status: 'planned' or 'completed'. Typical use: promote a planned session to completed once it happens. Omit to leave unchanged."`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteWorkoutArgs struct {
	ID             string `json:"id" jsonschema:"the workout id to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type WorkoutFuelingSummaryArgs struct {
	WorkoutID     string `json:"workout_id" jsonschema:"the workout id to summarise fueling for"`
	PreWindowMin  *int   `json:"pre_window_min,omitempty" jsonschema:"pre-workout window in minutes (default 240, range 0..720)"`
	PostWindowMin *int   `json:"post_window_min,omitempty" jsonschema:"post-workout window in minutes (default 60, range 0..720)"`
}

type SweatRateArgs struct {
	WorkoutID       string   `json:"workout_id" jsonschema:"the COMPLETED workout id to compute sweat rate for"`
	PreWeightKg     float64  `json:"pre_weight_kg" jsonschema:"pre-session body weight in kg (required, positive)"`
	PostWeightKg    float64  `json:"post_weight_kg" jsonschema:"post-session body weight in kg (required, positive)"`
	FluidMlOverride *float64 `json:"fluid_ml_override,omitempty" jsonschema:"optional override (ml, >= 0) for in-session fluid; replaces the derived hydration + workout-fuel sum"`
}

type FulfillWorkoutArgs struct {
	PlannedID      string `json:"planned_id" jsonschema:"the PLANNED workout id to fulfill (the merge target that holds the plan_slot_id)"`
	CompletedID    string `json:"completed_id" jsonschema:"the standalone COMPLETED activity id to merge into the planned workout"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type UnfulfillWorkoutArgs struct {
	ID             string `json:"id" jsonschema:"the fulfilled workout id to revert back to planned"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// RecomputeWorkoutTSSArgs has no inputs — the recompute spans all recomputable
// completed workouts.
type RecomputeWorkoutTSSArgs struct{}

func workoutsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_workout",
			Description: "Record a workout. Most workouts come from the Garmin importer with `source: garmin` " +
				"and an `external_id` (e.g. 'garmin:1234567') — that flow lives outside the agent. Use this tool " +
				"for MANUAL entries: gym sessions without a watch, sweat-rate test windows, untracked workouts the " +
				"user describes after the fact. For manual writes, leave `external_id` null — `external_id` is the " +
				"dedup mechanism; setting it on an agent-driven entry risks colliding with a future Garmin sync. " +
				"`tss` is the intensity signal; supply it if you know it, otherwise leave it null and downstream " +
				"tools will handle the gap. " +
				"`rpe` and `gi_distress_score` are the rehearsal-outcome signals — log them after fueling-rehearsal " +
				"workouts (e.g. long Z2 rides in race-prep blocks). RPE is Borg CR-10 perceived effort, integer 1..10. " +
				"GI distress is 1=no distress through 5=severe / had to stop. Both are nullable and only meaningful for " +
				"sessions you're actively iterating fueling on; skip for everyday Z1 spins and gym work. " +
				"Ingestion metrics (all nullable, units fixed): `distance_m` metres, `avg_power_w` watts, " +
				"`temperature_c` °C, `sweat_loss_ml` millilitres. `session_group` links the legs of a brick/multisport " +
				"session — set the SAME key on every leg (e.g. the Garmin parent activity id) so they can be fetched " +
				"together; omit for single-sport sessions.",
			SchemaType: LogWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					ExternalID      *string  `json:"external_id,omitempty"`
					Source          string   `json:"source"`
					Sport           string   `json:"sport"`
					Status          string   `json:"status,omitempty"`
					Name            *string  `json:"name,omitempty"`
					StartedAt       string   `json:"started_at"`
					EndedAt         string   `json:"ended_at"`
					KcalBurned      *float64 `json:"kcal_burned,omitempty"`
					AvgHR           *int     `json:"avg_hr,omitempty"`
					TSS             *float64 `json:"tss,omitempty"`
					RPE             *int     `json:"rpe,omitempty"`
					GIDistressScore *int     `json:"gi_distress_score,omitempty"`
					DistanceM       *float64 `json:"distance_m,omitempty"`
					AvgPowerW       *int     `json:"avg_power_w,omitempty"`
					TemperatureC    *float64 `json:"temperature_c,omitempty"`
					SweatLossML     *float64 `json:"sweat_loss_ml,omitempty"`
					SessionGroup    *string  `json:"session_group,omitempty"`
					Notes           *string  `json:"notes,omitempty"`
				}{
					ExternalID:      a.ExternalID,
					Source:          a.Source,
					Sport:           a.Sport,
					Status:          a.Status,
					Name:            a.Name,
					StartedAt:       a.StartedAt,
					EndedAt:         a.EndedAt,
					KcalBurned:      a.KcalBurned,
					AvgHR:           a.AvgHR,
					TSS:             a.TSS,
					RPE:             a.RPE,
					GIDistressScore: a.GIDistressScore,
					DistanceM:       a.DistanceM,
					AvgPowerW:       a.AvgPowerW,
					TemperatureC:    a.TemperatureC,
					SweatLossML:     a.SweatLossML,
					SessionGroup:    a.SessionGroup,
					Notes:           a.Notes,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/workouts", Body: body}, nil
			},
		},
		{
			Name: "list_workouts",
			Description: "List workouts whose started_at falls within the RFC 3339 window. Window is capped at 92 days. " +
				"Use this when answering 'what did I train this week?' or aggregating fueling-relevant workouts. " +
				"Pass `session_group` to fetch just the legs of one brick/multisport session (the window stays required).",
			SchemaType: ListWorkoutsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListWorkoutsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.SessionGroup != nil {
					q.Set("session_group", *a.SessionGroup)
				}
				if a.Status != nil {
					q.Set("status", *a.Status)
				}
				return HTTPCall{Method: "GET", Path: "/workouts", Query: q}, nil
			},
		},
		{
			Name:        "get_workout",
			Description: "Fetch a single workout by id. Returns the row with all stored fields.",
			SchemaType:  GetWorkoutArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/workouts/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "patch_workout",
			Description: "Adjust mutable fields on an existing workout. PATCH-able: `name`, `notes`, `kcal_burned`, " +
				"`avg_hr`, `tss`, `rpe`, `gi_distress_score`, `distance_m`, `avg_power_w`, `temperature_c`, " +
				"`sweat_loss_ml`, `session_group`. IMMUTABLE (delete + re-create if these are wrong): " +
				"`sport`, `started_at`, `ended_at`, `source`, `external_id`. " +
				"Typical post-ride flow: set `rpe` (Borg CR-10 perceived effort, 1..10) and `gi_distress_score` " +
				"(1=no distress, 5=severe) on the workout you just rehearsed fueling on. Other typical uses: " +
				"capture how the fueling went in `notes`, supply a missing kcal estimate, correct TSS after an FTP " +
				"change, or un-group a mis-linked brick leg (`clear_session_group: true`). " +
				"Every nullable field is tri-state: omit to leave unchanged, set a value to overwrite, OR set the " +
				"matching `clear_*` flag to retract a value (clears to null on the backend).",
			SchemaType: PatchWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.Name != nil {
					payload["name"] = *a.Name
				}
				if a.Notes != nil {
					payload["notes"] = *a.Notes
				}
				if a.KcalBurned != nil {
					payload["kcal_burned"] = *a.KcalBurned
				}
				if a.AvgHR != nil {
					payload["avg_hr"] = *a.AvgHR
				}
				if a.TSS != nil {
					payload["tss"] = *a.TSS
				}
				// RPE tri-state: ClearRPE wins (encodes as JSON null → backend clears);
				// otherwise integer if set; otherwise field absent from payload.
				if a.ClearRPE {
					payload["rpe"] = nil
				} else if a.RPE != nil {
					payload["rpe"] = *a.RPE
				}
				if a.ClearGIDistressScore {
					payload["gi_distress_score"] = nil
				} else if a.GIDistressScore != nil {
					payload["gi_distress_score"] = *a.GIDistressScore
				}
				// Ingestion-metric tri-state: Clear* wins (JSON null → backend clears),
				// otherwise value if set, otherwise field absent from payload.
				if a.ClearDistanceM {
					payload["distance_m"] = nil
				} else if a.DistanceM != nil {
					payload["distance_m"] = *a.DistanceM
				}
				if a.ClearAvgPowerW {
					payload["avg_power_w"] = nil
				} else if a.AvgPowerW != nil {
					payload["avg_power_w"] = *a.AvgPowerW
				}
				if a.ClearTemperatureC {
					payload["temperature_c"] = nil
				} else if a.TemperatureC != nil {
					payload["temperature_c"] = *a.TemperatureC
				}
				if a.ClearSweatLossML {
					payload["sweat_loss_ml"] = nil
				} else if a.SweatLossML != nil {
					payload["sweat_loss_ml"] = *a.SweatLossML
				}
				if a.ClearSessionGroup {
					payload["session_group"] = nil
				} else if a.SessionGroup != nil {
					payload["session_group"] = *a.SessionGroup
				}
				if a.Status != nil {
					payload["status"] = *a.Status
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/workouts/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_workout",
			Description: "Delete a workout. Returns an empty result on success.",
			SchemaType:  DeleteWorkoutArgs{},
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/workouts/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "fulfill_workout",
			Description: "Manually merge a completed activity into a planned workout — the explicit version of the " +
				"automatic reconciliation that runs on Garmin sync. Use this for cases the auto-match declined: a session " +
				"done on a different calendar day than planned, or a day with two same-sport planned workouts where the " +
				"import was flagged `needs_link`. Pass `planned_id` (the scheduled session, which holds the plan_slot_id) " +
				"and `completed_id` (the standalone imported activity). The planned row survives with the activity's " +
				"actuals and external_id and flips to completed; the standalone row is removed. Both must be the same sport.",
			SchemaType: FulfillWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a FulfillWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					CompletedID string `json:"completed_id"`
				}{CompletedID: a.CompletedID})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/workouts/" + url.PathEscape(a.PlannedID) + "/fulfill", Body: body}, nil
			},
		},
		{
			Name: "unfulfill_workout",
			Description: "Reverse a fulfilled workout back to planned — undo a wrong merge (auto or manual). Clears the " +
				"workout's external_id and actual metrics and restores status=planned, keeping its template_id and " +
				"plan_slot_id. The activity is not re-fetched; the next Garmin sync re-imports it as a fresh standalone row.",
			SchemaType: UnfulfillWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UnfulfillWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/workouts/" + url.PathEscape(a.ID) + "/unfulfill"}, nil
			},
		},
		{
			Name: "workout_fueling_summary",
			Description: "Return pre/intra/post fueling totals for a workout. Three time-anchored buckets " +
				"(pre, intra, post), each carrying THREE separate sub-objects: `nutrition` (kcal + macros + " +
				"nullable micros from meals), `hydration` (total_ml from hydration entries), and " +
				"`workout_fuel` (carbs/sodium/potassium/caffeine/ml from workout-fuel entries — gels, " +
				"electrolyte drinks, salt tabs, caffeine). Aggregation is by `logged_at` time-window matching, " +
				"NOT by the `workout_id` tag on intake rows — an untagged meal logged in the pre-window still " +
				"contributes. Defaults: pre_window_min=240 (4h), post_window_min=60. Both bounded [0, 720].",
			SchemaType: WorkoutFuelingSummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WorkoutFuelingSummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.PreWindowMin != nil {
					q.Set("pre_window_min", strconv.Itoa(*a.PreWindowMin))
				}
				if a.PostWindowMin != nil {
					q.Set("post_window_min", strconv.Itoa(*a.PostWindowMin))
				}
				return HTTPCall{Method: "GET", Path: "/workouts/" + url.PathEscape(a.WorkoutID) + "/fueling", Query: q}, nil
			},
		},
		{
			Name: "sweat_rate",
			Description: "Compute a completed workout's sweat rate (ml/hr) — the standard field test: " +
				"`sweat_loss_ml = (pre_weight_kg − post_weight_kg) × 1000 + fluid_ml`, over the workout's elapsed " +
				"hours. `pre_weight_kg` and `post_weight_kg` are REQUIRED (the bodyweight log is daily-grained, not " +
				"pre/post-session — the caller must supply the actual before/after readings). Fluid is summed from " +
				"the workout's LINKED hydration + workout-fuel `quantity_ml` and itemized; pass `fluid_ml_override` " +
				"for unlogged bottles. Result quality follows the supplied weights — a negative loss or a rate above " +
				"5000 ml/hr returns the numbers with `warning: \"implausible_result\"` rather than refusing. Planned " +
				"workouts are rejected. Read-only; anchors race hydration planning.",
			SchemaType: SweatRateArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a SweatRateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("pre_weight_kg", strconv.FormatFloat(a.PreWeightKg, 'f', -1, 64))
				q.Set("post_weight_kg", strconv.FormatFloat(a.PostWeightKg, 'f', -1, 64))
				if a.FluidMlOverride != nil {
					q.Set("fluid_ml_override", strconv.FormatFloat(*a.FluidMlOverride, 'f', -1, 64))
				}
				return HTTPCall{Method: "GET", Path: "/workouts/" + url.PathEscape(a.WorkoutID) + "/sweat-rate", Query: q}, nil
			},
		},
		{
			Name: "workout_adherence",
			Description: "Plan-adherence analytics over a date window: how well the athlete followed the plan. " +
				"Classifies each workout in [from, to] as completed (a planned session that was done), missed " +
				"(a planned session now overdue), upcoming (planned, not yet due), or unplanned (completed with " +
				"no plan slot). Returns the four counts, `adherence_rate` = completed / (completed + missed) over " +
				"DUE sessions only (null when none are due — e.g. a future-only window), planned-vs-actual " +
				"duration_min and tss, and a `by_sport` completed/missed breakdown. Pass `plan_id` to scope to one " +
				"plan. Read-only; 'now' is the server clock in the resolved timezone.",
			SchemaType: WorkoutAdherenceArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WorkoutAdherenceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != nil {
					q.Set("tz", *a.TZ)
				}
				if a.PlanID != nil {
					q.Set("plan_id", *a.PlanID)
				}
				if a.MissedLimit != nil {
					q.Set("missed_limit", strconv.Itoa(*a.MissedLimit))
				}
				if a.ZeroFill != nil && *a.ZeroFill {
					q.Set("zero_fill", "true")
				}
				return HTTPCall{Method: "GET", Path: "/workouts/adherence", Query: q}, nil
			},
		},
		{
			Name: "workout_compliance",
			Description: "Per-STEP execution compliance for one completed workout — how well it was executed " +
				"INSIDE, vs the template it was compiled from (adherence answers 'did it happen'; compliance " +
				"answers 'was it executed as written'). Expands the effective program's repeat groups into a flat " +
				"step sequence, matches laps to steps positionally, and for each step reports the resolved target " +
				"vs the lap's actual (in_band/under/over with a signed `delta` and `deviation_pct` — '20W under " +
				"target' is delta -20), planned-vs-actual duration, and a 0–100 step score, plus an overall " +
				"planned-duration-weighted `score` and steps_scored/steps_in_band. Targets are judged against the " +
				"ATHLETE-CONFIG-RESOLVED bands (a zone that couldn't resolve is reported unscorable, not an error). " +
				"When the lap count does not equal the expanded step count it returns `status:\"unavailable\"` with " +
				"`reason:\"lap_count_mismatch\"` and the two counts instead of guessing an alignment. Errors: 409 " +
				"workout_not_completed / multisport_unsupported / no_template_link / splits_missing, 404 not_found. " +
				"Read-only; computed on read, nothing persisted.",
			SchemaType: WorkoutComplianceArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WorkoutComplianceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/workouts/" + url.PathEscape(a.WorkoutID) + "/compliance"}, nil
			},
		},
		{
			Name: "recompute_workout_tss",
			Description: "Recompute derived Training Stress Score across completed workouts against the CURRENT " +
				"athlete-config thresholds (FTP, threshold pace/CSS, LTHR). Fills previously-missing TSS on runs/" +
				"swims/HR-only sessions and refreshes rows computed against old thresholds; measured values " +
				"(tss_source garmin/manual) are never touched. Run once after configuring thresholds, or after " +
				"changing FTP/paces. Returns {examined, updated, by_source:{power,pace,hr,none}}.",
			SchemaType: RecomputeWorkoutTSSArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "POST", Path: "/workouts/recompute-tss"}, nil
			},
		},
	}
}
