// Package activitystreams persists a workout's raw 1 Hz sample streams (power,
// speed, heart_rate) and derives execution-quality metrics from them. It owns
// the stream-ingest entrypoint: on POST it stores the arrays, delegates the
// mean-maximal best-effort computation to the effort-analytics service, and
// computes/stores the workout's variability index, efficiency factor, and
// aerobic decoupling. Streams can be retrieved (optionally downsampled) and the
// metrics recomputed from the stored arrays without a re-post.
//
// Unit-isolated: power (W), speed (m/s) and heart rate (bpm) are workout-only
// signals and feed no nutrition/hydration/energy total.
package activitystreams

// StreamType is a stored series kind. Matches the workout_streams.stream_type
// CHECK constraint.
type StreamType string

const (
	StreamPower     StreamType = "power"
	StreamSpeed     StreamType = "speed"
	StreamHeartRate StreamType = "heart_rate"
)

// StreamPayload is the POST /workouts/{id}/streams body: contiguous 1 Hz sample
// arrays (one value per second, coasting/idle seconds included as 0). Any subset
// of series may be present; an empty body writes nothing. The Garmin bridge
// resamples the activity detail to 1 Hz before posting.
type StreamPayload struct {
	Power     []float64 `json:"power,omitempty"`
	Speed     []float64 `json:"speed,omitempty"`
	HeartRate []float64 `json:"heart_rate,omitempty"`
}

func (p StreamPayload) empty() bool {
	return len(p.Power) == 0 && len(p.Speed) == 0 && len(p.HeartRate) == 0
}

// series returns the non-empty streams keyed by type, in a stable order.
func (p StreamPayload) series() map[StreamType][]float64 {
	m := map[StreamType][]float64{}
	if len(p.Power) > 0 {
		m[StreamPower] = p.Power
	}
	if len(p.Speed) > 0 {
		m[StreamSpeed] = p.Speed
	}
	if len(p.HeartRate) > 0 {
		m[StreamHeartRate] = p.HeartRate
	}
	return m
}

// IngestResult is the streams-POST response: how many best-effort ladder records
// were written and how many raw streams were stored.
type IngestResult struct {
	RecordsWritten int `json:"records_written"`
	StreamsStored  int `json:"streams_stored"`
}

// RecomputeResult is the recompute response: records rewritten and streams read.
type RecomputeResult struct {
	RecordsWritten int `json:"records_written"`
	StreamsUsed    int `json:"streams_used"`
}

// StreamsResponse is the GET /workouts/{id}/streams body. Streams holds each
// stored series by type (absent types omitted). Downsample echoes the applied
// bucket target when the caller requested one; nil means the full-resolution
// series was returned. DurationS is the stored (full-resolution) sample count of
// the longest series — the real activity length, independent of downsampling.
type StreamsResponse struct {
	WorkoutID    string                 `json:"workout_id"`
	SampleRateHz int                    `json:"sample_rate_hz"`
	DurationS    int                    `json:"duration_s"`
	Downsample   *int                   `json:"downsample,omitempty"`
	Streams      map[StreamType][]float64 `json:"streams"`
}

// WPrimeParams echoes the (caller-supplied) critical-power model parameters the
// W′bal series was computed with — reproducibility: same params, same answer.
type WPrimeParams struct {
	CPWatts  float64 `json:"cp_watts"`
	WPrimeKJ float64 `json:"w_prime_kj"`
}

// WPrimeSummary is the anaerobic-battery story of the ride. MinWPrimeKJ can be
// negative (the ride showed more anaerobic work than the supplied W′ allows —
// stale params), and MaxDepletionPct can accordingly exceed 100. The minimum is
// taken at full stream resolution, so downsampling the series loses nothing.
type WPrimeSummary struct {
	MinWPrimeKJ     float64 `json:"min_w_prime_kj"`
	MinAtS          int     `json:"min_at_s"`
	EndWPrimeKJ     float64 `json:"end_w_prime_kj"`
	MaxDepletionPct float64 `json:"max_depletion_pct"`
	TimeBelow25PctS int     `json:"time_below_25_pct_s"`
}

// WPrimeBalanceResult is the GET /workouts/{id}/w-prime-balance body. Series is
// the W′bal (kJ) per (optionally downsampled) point; omitted when summary_only.
// DurationS is the full-resolution stream length. Compute-on-read, nothing
// persisted; no athlete-config read (params are explicit).
type WPrimeBalanceResult struct {
	WorkoutID  string        `json:"workout_id"`
	Params     WPrimeParams  `json:"params"`
	DurationS  int           `json:"duration_s"`
	Summary    WPrimeSummary `json:"summary"`
	Downsample *int          `json:"downsample,omitempty"`
	Series     []float64     `json:"series,omitempty"`
}

// ExecutionMetrics are the stream-derived quality signals stored back onto the
// workout. Any field may be nil when its inputs are absent (e.g. no power → no
// VI; insufficient HR coverage → no decoupling).
type ExecutionMetrics struct {
	VariabilityIndex *float64
	EfficiencyFactor *float64
	DecouplingPct    *float64
}
