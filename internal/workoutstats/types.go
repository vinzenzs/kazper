// Package workoutstats aggregates completed workouts into per-day + windowed
// volume totals (count, duration, distance, elevation, kcal) with a by-sport
// breakdown. It is a read-side composition over the workouts table — no schema
// — mirroring how internal/summary composes meals. Workout distance/elevation/
// duration live only in this response shape and are deliberately NOT merged into
// any nutrition/hydration/energy total (unit isolation).
package workoutstats

import "time"

// Bucket carries the volume totals for one calendar day (Date set) or for the
// whole window (Date empty on the response's Total). Sums over nullable metrics
// are present-only: a workout missing a metric contributes nothing to that sum
// rather than zeroing the day. by_sport is count-per-sport, matching
// coachcontext.LoadSummary; a multisport session is a single `multisport` entry.
type Bucket struct {
	Date                string         `json:"date,omitempty"`
	Count               int            `json:"count"`
	TotalDurationMin    float64        `json:"total_duration_min"`
	TotalDistanceM      float64        `json:"total_distance_m"`
	TotalElevationGainM float64        `json:"total_elevation_gain_m"`
	TotalKcal           float64        `json:"total_kcal"`
	BySport             map[string]int `json:"by_sport"`
}

// Summary is the GET /workouts/summary response: a per-day series covering every
// calendar day in [from, to] (zero-filled days included so the heatmap has a
// complete grid) plus the window Total.
type Summary struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	TZ    string   `json:"tz"`
	Days  []Bucket `json:"days"`
	Total Bucket   `json:"total"`
}

// Params is the resolved input to SummaryFor. From/To are calendar dates; the
// service expands them to tz-local day bounds and buckets by tz-local day.
type Params struct {
	From time.Time
	To   time.Time
	Loc  *time.Location
}
