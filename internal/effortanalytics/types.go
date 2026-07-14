// Package effortanalytics computes and serves per-activity mean-maximal
// best-effort records (power/speed at a fixed duration ladder) and the windowed
// power/pace curve aggregated from them. It no longer owns stream ingest: the
// activity-streams capability persists the raw 1 Hz arrays and calls
// ComputeAndReplace here to (re)derive the compact best-effort rows. Unit-
// isolated: power (W) and speed (m/s) feed no nutrition/hydration/energy total.
package effortanalytics

import "time"

// Metric is the measured quantity a best effort is over. `speed` (m/s) is
// rendered as pace by the frontend for run/swim; `power` (W) is for cycling.
type Metric string

const (
	MetricPower Metric = "power"
	MetricSpeed Metric = "speed"
)

// Ladder is the fixed set of durations (seconds) the mean-maximal is computed
// at: 5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m. A duration longer than the
// activity yields no record.
var Ladder = []int{5, 15, 30, 60, 300, 600, 1200, 1800, 3600}

// Record is one stored best-effort row (before persistence): the best mean of a
// metric over duration_s seconds. KJTier is 0 for the fresh ladder (the best
// anywhere in the activity) and 500/1000/1500/2000 for a durability tier (the
// best whose window starts after that much accumulated work).
type Record struct {
	Metric    Metric
	DurationS int
	Value     float64
	KJTier    int
}

// Durability tiers (kJ of accumulated work) and the durations they're computed
// at — where fatigue resistance shows (sprint fade is neuromuscular noise;
// 20-min-after-2000kJ is the Ironman question). Constants, not parameters.
var (
	DurabilityTiers     = []int{500, 1000, 1500, 2000}
	DurabilityDurations = []int{60, 300, 1200}
)

// CurvePoint is one duration's best across the window: the max mean value and
// the workout + date it came from.
type CurvePoint struct {
	DurationS int     `json:"duration_s"`
	Value     float64 `json:"value"`
	WorkoutID string  `json:"workout_id"`
	Date      string  `json:"date"`
}

// Curve is the GET /workouts/power-curve response.
type Curve struct {
	From   string       `json:"from"`
	To     string       `json:"to"`
	TZ     string       `json:"tz"`
	Sport  string       `json:"sport"`
	Metric Metric       `json:"metric"`
	Points []CurvePoint `json:"points"`
}

// CurveParams is the resolved input to the curve query.
type CurveParams struct {
	From   time.Time
	To     time.Time
	Loc    *time.Location
	Metric Metric
}

// Critical-power (CP2) model validity band + fit gates. Only ladder durations
// within [cpBandLowS, cpBandHighS] feed the fit (below 2 min the anaerobic term
// inflates W′; above 30 min it's motivation/fueling, not physiology). With the
// current ladder the in-band durations are 5m/10m/20m/30m.
const (
	cpBandLowS     = 120  // 2 minutes
	cpBandHighS    = 1800 // 30 minutes
	cpMinPoints    = 3    // need ≥3 distinct in-band durations to fit
	cpMinSpanRatio = 3.0  // longest ≥ 3× shortest, else the fit extrapolates wildly
)

// CPModel is the fitted 2-parameter critical-power model: CP (asymptotic power,
// the slope of work vs time) and W′ (the finite work above CP, the intercept),
// with the fit quality. Bike/power only in v1.
type CPModel struct {
	CPWatts  float64 `json:"cp_watts"`
	WPrimeKJ float64 `json:"w_prime_kj"`
	RSquared float64 `json:"r_squared"`
	RMSEW    float64 `json:"rmse_w"`
}

// CPPoint is one in-band fit point: a windowed per-duration best (the same MAX
// the power curve serves) with the workout it came from.
type CPPoint struct {
	DurationS int     `json:"duration_s"`
	Watts     float64 `json:"watts"`
	WorkoutID string  `json:"workout_id"`
	Date      string  `json:"date"`
}

// CPModelResult is the GET /workouts/cp-model response. Model is null when the
// window can't support a fit, with Reason naming the gate; Points always carries
// whatever in-band bests were found. Nothing is persisted.
type CPModelResult struct {
	From   string    `json:"from"`
	To     string    `json:"to"`
	TZ     string    `json:"tz"`
	Model  *CPModel  `json:"model"`
	Reason string    `json:"reason,omitempty"`
	Points []CPPoint `json:"points"`
}

// DurabilityPoint is the fresh (tier-0) windowed best power for a duration.
type DurabilityPoint struct {
	Watts     float64 `json:"watts"`
	WorkoutID string  `json:"workout_id"`
	Date      string  `json:"date"`
}

// DurabilityTierPoint is one tier's windowed best plus its fade vs fresh.
type DurabilityTierPoint struct {
	KJTier    int     `json:"kj_tier"`
	Watts     float64 `json:"watts"`
	FadePct   float64 `json:"fade_pct"`
	WorkoutID string  `json:"workout_id"`
	Date      string  `json:"date"`
}

// DurabilityDuration is the fresh-vs-tier fade column for one duration.
type DurabilityDuration struct {
	DurationS int                   `json:"duration_s"`
	Fresh     *DurabilityPoint      `json:"fresh"`
	Tiers     []DurabilityTierPoint `json:"tiers"`
}

// DurabilityResult is the GET /workouts/durability response. Reason is
// "no_tiered_data" when the window holds only fresh rows (recompute needed to
// backfill tiers). Compute-on-read; nothing persisted.
type DurabilityResult struct {
	From      string               `json:"from"`
	To        string               `json:"to"`
	TZ        string               `json:"tz"`
	Durations []DurabilityDuration `json:"durations"`
	Reason    string               `json:"reason,omitempty"`
}

// CPHistoryAnchor is one weekly (Monday) CP fit over its trailing window: the
// fitted model or null with the gate Reason (the anchor is kept even when the
// window can't support a fit — the trend gaps, it doesn't zero).
type CPHistoryAnchor struct {
	Date   string   `json:"date"`
	Model  *CPModel `json:"model"`
	Reason string   `json:"reason,omitempty"`
}

// CPModelHistoryResult is the GET /workouts/cp-model/history response: the CP2
// fit at weekly anchors across the range, each over its trailing WindowDays.
// Compute-on-read; reads no athlete-config; persists nothing.
type CPModelHistoryResult struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	TZ         string            `json:"tz"`
	WindowDays int               `json:"window_days"`
	Anchors    []CPHistoryAnchor `json:"anchors"`
}

// Sex selects the Coggan reference table. Athlete-config has no sex field (the
// advisory posture holds), so this is a query parameter, defaulting to male.
const (
	sexMale   = "male"
	sexFemale = "female"
)

// Weight-source provenance echoed on the power-profile response so the W/kg
// denominator is auditable.
const (
	WeightSourceParam  = "param"
	WeightSourceStored = "stored"
)

// PowerProfileAnchor is one benchmark duration's ranking: the windowed best in
// watts, its W/kg, the Coggan category band, an interpolated percentile, and the
// workout it came from.
type PowerProfileAnchor struct {
	Label      string  `json:"label"`
	DurationS  int     `json:"duration_s"`
	Watts      float64 `json:"watts"`
	WPerKg     float64 `json:"w_per_kg"`
	Category   string  `json:"category"`
	Percentile float64 `json:"percentile"`
	WorkoutID  string  `json:"workout_id"`
	Date       string  `json:"date"`
}

// PowerProfileResult is the GET /workouts/power-profile response. Anchors carries
// the ranked benchmark durations present in the window; MissingAnchors names the
// ones with no best-effort. Phenotype is null unless all four anchors rank.
// Nothing is persisted.
type PowerProfileResult struct {
	From           string               `json:"from"`
	To             string               `json:"to"`
	TZ             string               `json:"tz"`
	Sex            string               `json:"sex"`
	WeightKg       float64              `json:"weight_kg"`
	WeightSource   string               `json:"weight_source"`
	Anchors        []PowerProfileAnchor `json:"anchors"`
	MissingAnchors []string             `json:"missing_anchors"`
	Phenotype      *string              `json:"phenotype"`
}
