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

// IntensityDistributionArgs is the input to intensity_distribution.
type IntensityDistributionArgs struct {
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
		{
			Name: "intensity_distribution",
			Description: "Time-in-zone intensity distribution over a date window: how COMPLETED workouts' HR-zone " +
				"seconds split across zones 1–5, with band shares (low=Z1+Z2, moderate=Z3, high=Z4+Z5) and a " +
				"classification label (polarized | pyramidal | threshold | mixed | null). Answers 'am I training " +
				"polarized / is my base actually easy'. Returns a window total (with bands + label), a by-sport " +
				"breakdown, a Monday-start weekly trend, and a by_training_focus session-count axis. Shares are of " +
				"ZONED time (total_zone_secs), not elapsed time — for volume (distance/duration/kcal) use " +
				"training_totals instead. A completed workout with no HR zones (many strength/pool sessions) is " +
				"excluded from the shares but counted in missing_zone_data_count so the split isn't silently " +
				"biased. Range up to 400 days. Read-only; no idempotency key is sent.",
			SchemaType: IntensityDistributionArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a IntensityDistributionArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/workouts/intensity-distribution", Query: q}, nil
			},
		},
	}
}
