package agenttools

import (
	"encoding/json"
	"net/url"
)

// Workout volume totals read — aggregates completed workouts into per-day +
// windowed totals (count, duration, distance, elevation, kcal) with a by-sport
// breakdown. One HTTP call to GET /workouts/summary, per the REST↔MCP 1:1
// convention, so the coach can answer volume questions without client math.

func init() { registerMCPDomain(workoutStatsSpecs()) }

// TrainingTotalsArgs is the input to training_totals. `from`/`to` are inclusive
// calendar dates (YYYY-MM-DD); the range supports up to a full year.
type TrainingTotalsArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; up to 400 days from 'from'"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func workoutStatsSpecs() []Spec {
	return []Spec{
		{
			Name: "training_totals",
			Description: "Aggregate COMPLETED workouts over a date window into volume totals: per-day buckets " +
				"plus a window total, each with count, total_duration_min, total_distance_m, " +
				"total_elevation_gain_m, total_kcal, and a by-sport count breakdown. Distance/elevation are " +
				"metres; duration is elapsed minutes. Use it for 'how far / how long / how much climbing this " +
				"week/month/year' questions. Planned workouts are excluded; nullable metrics are summed " +
				"present-only (a workout missing distance doesn't zero the day). Range supports year-to-date " +
				"(up to 400 days). Read-only; no idempotency key is sent.",
			SchemaType: TrainingTotalsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a TrainingTotalsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/workouts/summary", Query: q}, nil
			},
		},
	}
}
