package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Activity-streams MCP surface. Raw 1 Hz streams are ingested by the Garmin
// bridge (not the agent) and are deliberately NOT exposed as an MCP read — they
// are large and only useful as charts. The agent gets: recompute the stream-
// derived execution metrics + best-effort ladder (write), and read a workout's
// W′-balance SUMMARY (the series stays chart-only). Each is one HTTP call, per
// the REST↔MCP 1:1 convention.

func init() { registerMCPDomain(activityStreamsSpecs()) }

// RecomputeWorkoutStreamsArgs is the input to recompute_workout_streams.
type RecomputeWorkoutStreamsArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"the workout id whose stored streams to recompute metrics from"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// WPrimeBalanceArgs is the input to w_prime_balance. cp_watts / w_prime_kj come
// from the athlete's critical-power model (the cp_model tool).
type WPrimeBalanceArgs struct {
	WorkoutID string  `json:"workout_id" jsonschema:"the workout id (must have a stored power stream) to compute W′ balance for"`
	CPWatts   float64 `json:"cp_watts" jsonschema:"critical power in watts, from the cp_model fit (> 0)"`
	WPrimeKJ  float64 `json:"w_prime_kj" jsonschema:"anaerobic work capacity W′ in kJ, from the cp_model fit (> 0)"`
}

// DetectIntervalsArgs is the input to detect_intervals.
type DetectIntervalsArgs struct {
	WorkoutID string `json:"workout_id" jsonschema:"the workout id (must have a stored power stream) to detect work intervals in"`
}

// QuadrantAnalysisArgs is the input to quadrant_analysis. cp_watts comes from the
// cp_model fit; cadence_rpm is the athlete's reference (self-selected) cadence.
type QuadrantAnalysisArgs struct {
	WorkoutID  string  `json:"workout_id" jsonschema:"the workout id (must have stored power AND cadence streams) to analyze"`
	CPWatts    float64 `json:"cp_watts" jsonschema:"critical power / threshold in watts, from the cp_model fit (> 0)"`
	CadenceRPM float64 `json:"cadence_rpm" jsonschema:"the athlete's reference self-selected cadence in rpm (> 0), e.g. 90"`
	CrankMM    float64 `json:"crank_mm,omitempty" jsonschema:"optional crank length in mm (default 172.5; 100-220)"`
}

// StrideAnalysisArgs is the input to stride_analysis.
type StrideAnalysisArgs struct {
	WorkoutID   string   `json:"workout_id" jsonschema:"the RUN workout id to decompose (needs stored speed + cadence streams)"`
	MinSpeedMps *float64 `json:"min_speed_mps,omitempty" jsonschema:"optional: exclude samples slower than this in m/s (0.5..5.0) — e.g. trim walk breaks from a long run. Off by default."`
}

func activityStreamsSpecs() []Spec {
	return []Spec{
		{
			Name: "stride_analysis",
			Description: "For a RUN: where does this athlete's speed come from — turnover or step length, and " +
				"which one plateaus? Speed = cadence × step length, so the decomposition is exact. From the " +
				"stored speed + cadence streams it returns speed bins (each with its seconds, mean cadence in " +
				"spm, and mean step length in metres per single step) plus a contribution split — " +
				"`cadence_contribution_pct` and `step_contribution_pct`, summing to 100 — saying how much of " +
				"the speed gain came from each. " +
				"USE THE BINS, not just the split: the split is one number for the whole run, while the bins " +
				"show WHERE a plateau starts (e.g. step length flat above threshold pace while cadence keeps " +
				"climbing). Never report a bare \"you are stride-limited\" verdict — show the decomposition. " +
				"A steady-state run returns `contribution: null` with `reason: \"insufficient_speed_range\"` " +
				"and the bins only: a run without pace variety genuinely cannot answer the question, so say " +
				"that rather than reaching for the split. This reads best on runs with variety — intervals, " +
				"fartlek, progressions. " +
				"Sanity-check the numbers before quoting them: a real run sits around 160–180 spm with ~1.0–1.4 m " +
				"steps. An ~85 spm mean means the device reported single-foot cadence, so the step length is " +
				"about double reality — say so instead of reporting a 2.4 m stride. " +
				"`min_speed_mps` trims walk breaks when a long run has them. " +
				"Runs only — a ride returns 409 `sport_unsupported` (a bike's cadence × \"step length\" is " +
				"meaningless). Needs stored cadence: `cadence_stream_missing` means the run predates the " +
				"bridge's run-cadence extraction, not that the athlete has no data — a re-sync/backfill of that " +
				"activity's streams fixes it. Summary only (the scatter is chart data). Read-only.",
			SchemaType: StrideAnalysisArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a StrideAnalysisArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				// The scatter is chart data, never reasoning data (the
				// quadrant/W′bal convention): always summary-only over MCP.
				q.Set("summary_only", "true")
				if a.MinSpeedMps != nil {
					q.Set("min_speed_mps", strconv.FormatFloat(*a.MinSpeedMps, 'f', -1, 64))
				}
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/stride",
					Query:  q,
				}, nil
			},
		},
		{
			Name: "recompute_workout_streams",
			Description: "Recompute a workout's stream-derived execution metrics (variability_index, efficiency_factor, " +
				"decoupling_pct) and its mean-maximal best-effort ladder from the streams already stored for it — " +
				"without a re-sync from Garmin. Use this after changing something that feeds the math (e.g. an FTP/" +
				"threshold-HR update) or when an earlier sync stored streams before the metrics logic existed. Needs " +
				"stored streams: returns streams_not_found if the workout has none (only power-metered / HR-strapped " +
				"activities carry them). The raw sample arrays are intentionally not exposed over MCP — they are chart " +
				"data, not reasoning inputs. Returns {records_written, streams_used}.",
			SchemaType: RecomputeWorkoutStreamsArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a RecomputeWorkoutStreamsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{
					Method: "POST",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/streams/recompute",
				}, nil
			},
		},
		{
			Name: "w_prime_balance",
			Description: "Compute a workout's W′-balance SUMMARY — the anaerobic-battery story of the ride — from its " +
				"stored power stream and the athlete's critical-power model. Pass `cp_watts` and `w_prime_kj` from the " +
				"`cp_model` fit (this tool never reads config — the parameters are explicit and echoed back). Returns " +
				"how deep W′ was drained: min_w_prime_kj (+ when), end_w_prime_kj, max_depletion_pct, and " +
				"time_below_25_pct_s — the signal for whether the athlete finished an interval session comfortably or " +
				"nearly empty. A min below 0 / depletion over 100% means the supplied W′ is too low (re-fit via " +
				"cp_model). Needs a stored power stream: power_stream_missing if the workout has none, " +
				"streams_not_found if nothing is stored. The raw time series is chart data and is NOT returned here " +
				"(summary only). Read-only.",
			SchemaType: WPrimeBalanceArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WPrimeBalanceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("cp_watts", strconv.FormatFloat(a.CPWatts, 'f', -1, 64))
				q.Set("w_prime_kj", strconv.FormatFloat(a.WPrimeKJ, 'f', -1, 64))
				// The agent gets the summary only — the series stays chart data.
				q.Set("summary_only", "true")
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/w-prime-balance",
					Query:  q,
				}, nil
			},
		},
		{
			Name: "detect_intervals",
			Description: "Detect the WORK INTERVALS actually done in a ride from its stored power stream — for " +
				"unstructured / non-lap-buttoned sessions where only the athlete's legs know the structure. A " +
				"deterministic, parameter-free procedure (30 s smoothing → an Otsu work/rest threshold derived from " +
				"the ride's own power distribution → gap-merge ≤ 30 s → discard efforts < 60 s) returns the derived " +
				"threshold_w, each interval's {n, start_s, end_s, duration_s, avg_w, max_w, kj}, the rest gaps, and a " +
				"summary (count, work_total_s, mean_effort_s, mean_effort_w) — the compact structure to describe what " +
				"was done ('5 efforts averaging 4:10 at 305 W'). A genuinely steady ride returns intervals: [] with " +
				"threshold_w: null and reason 'no_distinct_efforts' — absence of intervals is a real finding, not an " +
				"error. Unlike the raw series this list IS returned in full (compact reasoning data). Needs a stored " +
				"power stream: power_stream_missing if none, streams_not_found if nothing is stored. Cycling power " +
				"only. Read-only.",
			SchemaType: DetectIntervalsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DetectIntervalsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/intervals",
				}, nil
			},
		},
		{
			Name: "quadrant_analysis",
			Description: "Force/velocity quadrant analysis of a ride — HOW power was produced (grinding at high " +
				"force/low cadence vs spinning at low force/high cadence), not just how much. From the stored power " +
				"AND cadence streams it classifies each pedaling second into a Coggan quadrant relative to a reference " +
				"point: pass `cp_watts` (from the `cp_model` fit) and `cadence_rpm` (the athlete's self-selected " +
				"cadence, e.g. 90); `crank_mm` defaults to 172.5. Returns the SUMMARY — the share of pedaling time in " +
				"each quadrant (q1 high-force/high-velocity … q4 low-force/high-velocity), pedaling_s vs excluded_s " +
				"(coasting/dropout), and the reference AEPF/CPV — for cadence prescription and race-position work. The " +
				"raw scatter is chart data and is NOT returned here (summary only). Needs stored power + cadence: " +
				"power_stream_missing / cadence_stream_missing (cadence exists only for rides synced after the bridge " +
				"cadence update). Cycling only. Read-only.",
			SchemaType: QuadrantAnalysisArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a QuadrantAnalysisArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("cp_watts", strconv.FormatFloat(a.CPWatts, 'f', -1, 64))
				q.Set("cadence_rpm", strconv.FormatFloat(a.CadenceRPM, 'f', -1, 64))
				if a.CrankMM > 0 {
					q.Set("crank_mm", strconv.FormatFloat(a.CrankMM, 'f', -1, 64))
				}
				// The agent gets shares only — the scatter stays chart data.
				q.Set("summary_only", "true")
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/quadrant",
					Query:  q,
				}, nil
			},
		},
	}
}
