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

func activityStreamsSpecs() []Spec {
	return []Spec{
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
	}
}
