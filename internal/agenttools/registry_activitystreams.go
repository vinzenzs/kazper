package agenttools

import (
	"encoding/json"
	"net/url"
)

// Activity-streams write surface. Raw 1 Hz streams are ingested by the Garmin
// bridge (not the agent) and are deliberately NOT exposed as an MCP read — they
// are large and only useful as charts. The agent gets exactly one tool: recompute
// the stream-derived execution metrics + best-effort ladder from the already-
// stored streams. One HTTP call to POST /workouts/{id}/streams/recompute, per the
// REST↔MCP 1:1 convention.

func init() { registerMCPDomain(activityStreamsSpecs()) }

// RecomputeWorkoutStreamsArgs is the input to recompute_workout_streams.
type RecomputeWorkoutStreamsArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"the workout id whose stored streams to recompute metrics from"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
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
	}
}
