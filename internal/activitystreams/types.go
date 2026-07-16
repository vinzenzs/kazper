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

import "github.com/google/uuid"

// StreamType is a stored series kind. Matches the workout_streams.stream_type
// CHECK constraint.
type StreamType string

const (
	StreamPower     StreamType = "power"
	StreamSpeed     StreamType = "speed"
	StreamHeartRate StreamType = "heart_rate"
	StreamCadence   StreamType = "cadence"
)

// StreamPayload is the POST /workouts/{id}/streams body: contiguous 1 Hz sample
// arrays (one value per second, coasting/idle seconds included as 0). Any subset
// of series may be present; an empty body writes nothing. The Garmin bridge
// resamples the activity detail to 1 Hz before posting.
type StreamPayload struct {
	Power     []float64 `json:"power,omitempty"`
	Speed     []float64 `json:"speed,omitempty"`
	HeartRate []float64 `json:"heart_rate,omitempty"`
	Cadence   []float64 `json:"cadence,omitempty"`
}

func (p StreamPayload) empty() bool {
	return len(p.Power) == 0 && len(p.Speed) == 0 && len(p.HeartRate) == 0 && len(p.Cadence) == 0
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
	if len(p.Cadence) > 0 {
		m[StreamCadence] = p.Cadence
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
	WorkoutID    string                   `json:"workout_id"`
	SampleRateHz int                      `json:"sample_rate_hz"`
	DurationS    int                      `json:"duration_s"`
	Downsample   *int                     `json:"downsample,omitempty"`
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

// QuadrantParams echoes the (caller-supplied) reference-point parameters the
// quadrant classification used — reproducibility, the W′bal convention.
type QuadrantParams struct {
	CPWatts    float64 `json:"cp_watts"`
	CadenceRPM float64 `json:"cadence_rpm"`
	CrankMM    float64 `json:"crank_mm"`
}

// QuadrantSummary is the force/velocity breakdown: the share of pedaling time in
// each Coggan quadrant (I high-force/high-velocity … IV low-force/high-velocity),
// the pedaling vs excluded (coasting/dropout) seconds, and the reference point.
type QuadrantSummary struct {
	Q1Pct     float64 `json:"q1_pct"`
	Q2Pct     float64 `json:"q2_pct"`
	Q3Pct     float64 `json:"q3_pct"`
	Q4Pct     float64 `json:"q4_pct"`
	PedalingS int     `json:"pedaling_s"`
	ExcludedS int     `json:"excluded_s"`
	AEPFRefN  float64 `json:"aepf_ref_n"`
	CPVRefMps float64 `json:"cpv_ref_mps"`
}

// QuadrantPoint is one pedaling sample's force/velocity coordinate.
type QuadrantPoint struct {
	AEPFN  float64 `json:"aepf_n"`
	CPVMps float64 `json:"cpv_mps"`
}

// QuadrantResult is the GET /workouts/{id}/quadrant body. Scatter is the (always
// downsampled) paired points, omitted under summary_only. Compute-on-read;
// reads no athlete-config (params explicit); persists nothing.
type QuadrantResult struct {
	WorkoutID string          `json:"workout_id"`
	Params    QuadrantParams  `json:"params"`
	Summary   QuadrantSummary `json:"summary"`
	Scatter   []QuadrantPoint `json:"scatter,omitempty"`
}

// Interval is one detected work effort: its ordinal, span (seconds from ride
// start), duration, and average/max/total power/work over the RAW stream (the
// smoothed series only sets the boundaries).
type Interval struct {
	N         int     `json:"n"`
	StartS    int     `json:"start_s"`
	EndS      int     `json:"end_s"`
	DurationS int     `json:"duration_s"`
	AvgW      float64 `json:"avg_w"`
	MaxW      float64 `json:"max_w"`
	KJ        float64 `json:"kj"`
}

// Rest is the recovery gap after interval AfterN.
type Rest struct {
	AfterN    int     `json:"after_n"`
	DurationS int     `json:"duration_s"`
	AvgW      float64 `json:"avg_w"`
}

// IntervalSummary is the headline structure of the detected efforts.
type IntervalSummary struct {
	Count       int     `json:"count"`
	WorkTotalS  int     `json:"work_total_s"`
	MeanEffortS float64 `json:"mean_effort_s"`
	MeanEffortW float64 `json:"mean_effort_w"`
}

// IntervalsResult is the GET /workouts/{id}/intervals body. ThresholdW is the
// Otsu-derived work/rest split (null when the ride isn't meaningfully bimodal,
// with Reason "no_distinct_efforts"). Compute-on-read; nothing persisted.
type IntervalsResult struct {
	WorkoutID  string          `json:"workout_id"`
	ThresholdW *float64        `json:"threshold_w"`
	Intervals  []Interval      `json:"intervals"`
	Rests      []Rest          `json:"rests"`
	Reason     string          `json:"reason,omitempty"`
	Summary    IntervalSummary `json:"summary"`
}

// ExecutionMetrics are the stream-derived quality signals stored back onto the
// workout. Any field may be nil when its inputs are absent (e.g. no power → no
// VI; insufficient HR coverage → no decoupling).
type ExecutionMetrics struct {
	VariabilityIndex *float64
	EfficiencyFactor *float64
	DecouplingPct    *float64
}

// ----- stride analysis (add-run-stride-analysis) -----

// StrideBin is one speed bucket's mean composition. Seconds is the bucket's
// sample count at 1 Hz, so it reads as time spent there.
type StrideBin struct {
	SpeedLowMps  float64 `json:"speed_low_mps"`
	SpeedHighMps float64 `json:"speed_high_mps"`
	Seconds      int     `json:"seconds"`
	CadenceSPM   float64 `json:"cadence_spm"`
	StepLengthM  float64 `json:"step_length_m"`
}

// StrideContribution splits a run's speed gain between turnover and step
// length. The two sum to 100 by construction (ln(speed) = ln(cadence) +
// ln(step)), which is what makes the pair a partition rather than two
// unrelated numbers.
type StrideContribution struct {
	CadencePct float64 `json:"cadence_contribution_pct"`
	StepPct    float64 `json:"step_contribution_pct"`
}

// StridePoint is one scatter sample.
type StridePoint struct {
	SpeedMps    float64 `json:"speed_mps"`
	CadenceSPM  float64 `json:"cadence_spm"`
	StepLengthM float64 `json:"step_length_m"`
}

// StrideResult is the response shape for GET /workouts/{id}/stride.
//
// Contribution is nil exactly when Reason is set: a steady-state run cannot say
// where speed comes from, and the bins are still returned so the reader sees
// what data there was. The verdict is always decomposed — never a bare
// "you are stride-limited" label.
type StrideResult struct {
	WorkoutID    uuid.UUID           `json:"workout_id"`
	Bins         []StrideBin         `json:"bins"`
	Contribution *StrideContribution `json:"contribution,omitempty"`
	Reason       *string             `json:"reason,omitempty"`
	AnalyzedS    int                 `json:"analyzed_s"`
	ExcludedS    int                 `json:"excluded_s"`
	MinSpeedMps  *float64            `json:"min_speed_mps,omitempty"`
	Scatter      []StridePoint       `json:"scatter,omitempty"`
}
