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

// --- Intensity distribution (time-in-zone) — add-intensity-distribution ---

// Classification band constants: the 80/20 rule with tolerance. thresholdBandPct
// flags a big-Z3 middle; lowBaseBandPct is the aerobic-base floor for a
// polarized/pyramidal call. Fixed, documented, auditable — the bands are the
// data, the label is advisory.
const (
	thresholdBandPct = 20.0
	lowBaseBandPct   = 75.0
)

// ZoneShare is one HR zone's summed seconds and its share of zoned time.
// share_pct is omitted when the group accrued no zone time (a 0% share would be
// a lie, not a measurement).
type ZoneShare struct {
	Zone     int      `json:"zone"`
	Secs     int      `json:"secs"`
	SharePct *float64 `json:"share_pct,omitempty"`
}

// Bands collapses the five zones into low (Z1+Z2), moderate (Z3), high (Z4+Z5).
type Bands struct {
	LowPct      float64 `json:"low_pct"`
	ModeratePct float64 `json:"moderate_pct"`
	HighPct     float64 `json:"high_pct"`
}

// ZoneAggregate is the per-sport / per-week zone breakdown: the count of
// zone-carrying completed workouts, the total zoned seconds, and the five-entry
// (zone 1→5) share array.
type ZoneAggregate struct {
	WorkoutsCounted int          `json:"workouts_counted"`
	TotalZoneSecs   int          `json:"total_zone_secs"`
	Zones           [5]ZoneShare `json:"zones"`
}

// TotalAggregate is the window-level distribution: a ZoneAggregate plus the
// band collapse and the (nullable) classification label. bands is always present
// so the label can be audited; classification is null when there is no zone time.
type TotalAggregate struct {
	ZoneAggregate
	Bands          Bands   `json:"bands"`
	Classification *string `json:"classification"`
}

// WeekBucket is one Monday-start week's zone aggregate. missing_zone_data_count
// (omitempty) counts the week's completed workouts that carried no HR zones.
type WeekBucket struct {
	WeekStart string `json:"week_start"`
	ZoneAggregate
	MissingZoneDataCount int `json:"missing_zone_data_count,omitempty"`
}

// Distribution is the GET /workouts/intensity-distribution response.
type Distribution struct {
	From                   string                   `json:"from"`
	To                     string                   `json:"to"`
	TZ                     string                   `json:"tz"`
	Total                  TotalAggregate           `json:"total"`
	BySport                map[string]ZoneAggregate `json:"by_sport"`
	Weekly                 []WeekBucket             `json:"weekly"`
	ByTrainingFocus        map[string]int           `json:"by_training_focus"`
	UnclassifiedFocusCount int                      `json:"unclassified_focus_count"`
	MissingZoneDataCount   int                      `json:"missing_zone_data_count"`
}
