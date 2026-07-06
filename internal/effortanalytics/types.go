// Package effortanalytics computes and serves per-activity mean-maximal
// best-effort records (power/speed at a fixed duration ladder) and the windowed
// power/pace curve aggregated from them. Streams are computed-over in-request
// and NOT persisted; only the compact best-effort rows survive. Unit-isolated:
// power (W) and speed (m/s) feed no nutrition/hydration/energy total.
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

// StreamPayload is the POST /workouts/{id}/streams body: contiguous 1 Hz sample
// arrays (one value per second, coasting seconds included as 0). Either or both
// series may be present; an empty body writes nothing. The bridge resamples
// Garmin's activity detail to 1 Hz before posting.
type StreamPayload struct {
	Power []float64 `json:"power,omitempty"`
	Speed []float64 `json:"speed,omitempty"`
}

func (p StreamPayload) empty() bool { return len(p.Power) == 0 && len(p.Speed) == 0 }

// Record is one stored best-effort row (before persistence): the best mean of a
// metric over duration_s seconds anywhere in the activity.
type Record struct {
	Metric    Metric
	DurationS int
	Value     float64
}

// IngestResult is the streams-POST response.
type IngestResult struct {
	RecordsWritten int `json:"records_written"`
}

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
