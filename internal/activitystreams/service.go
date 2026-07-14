package activitystreams

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Sentinel errors mapping 1:1 to API error codes.
var (
	ErrWorkoutNotFound   = errors.New("workout not found")    // workout_not_found
	ErrStreamsNotFound   = errors.New("streams not found")    // streams_not_found
	ErrDownsampleInvalid = errors.New("downsample invalid")   // downsample_invalid
	ErrPowerStreamMissing = errors.New("power stream missing") // power_stream_missing
)

const (
	// minMetricSamples is the activity length (1 Hz samples ≈ seconds) below
	// which stream-derived metrics are not meaningful. 1200 = 20 minutes, the
	// conventional floor for NP/VI/decoupling to say anything honest.
	minMetricSamples = 1200
	// hrCoverageMin is the fraction of non-zero HR samples required before a
	// heart-rate-based metric (EF, decoupling) is trusted. Dropouts read as 0.
	hrCoverageMin = 0.80
	// npWindow is the Coggan normalized-power rolling-average window (seconds).
	npWindow = 30
	// downsampleMin/Max bound the retrieval bucket target.
	downsampleMin = 10
	downsampleMax = 5000
)

// StreamsRepo persists and loads raw streams (satisfied by *Repo).
type StreamsRepo interface {
	Replace(ctx context.Context, workoutID uuid.UUID, series map[StreamType][]float64) (int, error)
	LoadForWorkout(ctx context.Context, workoutID uuid.UUID) (map[StreamType][]float64, int, error)
}

// WorkoutsRepo resolves the workout and stores the derived execution metrics.
type WorkoutsRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*workouts.Workout, error)
	SetExecutionMetrics(ctx context.Context, id uuid.UUID, vi, ef, decoupling *float64) error
}

// EffortComputer recomputes the mean-maximal best-effort ladder from the
// power/speed series (satisfied by *effortanalytics.Service.ComputeAndReplace).
type EffortComputer interface {
	ComputeAndReplace(ctx context.Context, w *workouts.Workout, power, speed []float64) (int, error)
}

// Service owns stream ingest, retrieval and metric recomputation.
type Service struct {
	streams StreamsRepo
	workout WorkoutsRepo
	effort  EffortComputer
}

func NewService(streams StreamsRepo, workout WorkoutsRepo, effort EffortComputer) *Service {
	return &Service{streams: streams, workout: workout, effort: effort}
}

// Ingest persists the workout's streams, refreshes its best-effort ladder, and
// derives + stores its execution metrics. An empty payload writes nothing and
// leaves existing streams/metrics untouched. ErrWorkoutNotFound if the id has no
// workout.
func (s *Service) Ingest(ctx context.Context, id uuid.UUID, p StreamPayload) (IngestResult, error) {
	w, err := s.resolve(ctx, id)
	if err != nil {
		return IngestResult{}, err
	}
	if p.empty() {
		return IngestResult{}, nil
	}
	stored, err := s.streams.Replace(ctx, id, p.series())
	if err != nil {
		return IngestResult{}, err
	}
	records, err := s.effort.ComputeAndReplace(ctx, w, p.Power, p.Speed)
	if err != nil {
		return IngestResult{}, err
	}
	m := executionMetrics(p.Power, p.Speed, p.HeartRate)
	if err := s.workout.SetExecutionMetrics(ctx, id, m.VariabilityIndex, m.EfficiencyFactor, m.DecouplingPct); err != nil {
		return IngestResult{}, err
	}
	return IngestResult{RecordsWritten: records, StreamsStored: stored}, nil
}

// Retrieve returns the workout's stored streams, optionally bucket-mean
// downsampled to `downsample` points per series (bounds [10, 5000]).
// ErrWorkoutNotFound / ErrStreamsNotFound / ErrDownsampleInvalid.
func (s *Service) Retrieve(ctx context.Context, id uuid.UUID, downsample *int) (StreamsResponse, error) {
	if downsample != nil && (*downsample < downsampleMin || *downsample > downsampleMax) {
		return StreamsResponse{}, ErrDownsampleInvalid
	}
	if _, err := s.resolve(ctx, id); err != nil {
		return StreamsResponse{}, err
	}
	series, rate, err := s.streams.LoadForWorkout(ctx, id)
	if err != nil {
		return StreamsResponse{}, err
	}
	if len(series) == 0 {
		return StreamsResponse{}, ErrStreamsNotFound
	}
	if rate == 0 {
		rate = 1
	}
	duration := 0
	out := make(map[StreamType][]float64, len(series))
	for st, samples := range series {
		if len(samples) > duration {
			duration = len(samples)
		}
		vals := samples
		if downsample != nil {
			vals = bucketMean(samples, *downsample)
		}
		out[st] = roundSamples(vals)
	}
	return StreamsResponse{
		WorkoutID:    id.String(),
		SampleRateHz: rate,
		DurationS:    duration,
		Downsample:   downsample,
		Streams:      out,
	}, nil
}

// Recompute reloads the stored streams and rewrites the best-effort ladder and
// execution metrics from them, without a re-post. ErrWorkoutNotFound /
// ErrStreamsNotFound.
func (s *Service) Recompute(ctx context.Context, id uuid.UUID) (RecomputeResult, error) {
	w, err := s.resolve(ctx, id)
	if err != nil {
		return RecomputeResult{}, err
	}
	series, _, err := s.streams.LoadForWorkout(ctx, id)
	if err != nil {
		return RecomputeResult{}, err
	}
	if len(series) == 0 {
		return RecomputeResult{}, ErrStreamsNotFound
	}
	power, speed, hr := series[StreamPower], series[StreamSpeed], series[StreamHeartRate]
	records, err := s.effort.ComputeAndReplace(ctx, w, power, speed)
	if err != nil {
		return RecomputeResult{}, err
	}
	m := executionMetrics(power, speed, hr)
	if err := s.workout.SetExecutionMetrics(ctx, id, m.VariabilityIndex, m.EfficiencyFactor, m.DecouplingPct); err != nil {
		return RecomputeResult{}, err
	}
	return RecomputeResult{RecordsWritten: records, StreamsUsed: len(series)}, nil
}

// WPrimeBalance computes the differential W′-balance over the workout's stored
// power stream given caller-supplied CP/W′. Compute-on-read; reads no
// athlete-config and persists nothing. summaryOnly omits the (optionally
// downsampled) series. Sentinels: ErrWorkoutNotFound / ErrStreamsNotFound /
// ErrPowerStreamMissing / ErrDownsampleInvalid.
func (s *Service) WPrimeBalance(ctx context.Context, id uuid.UUID, cpWatts, wPrimeKJ float64, downsample *int, summaryOnly bool) (*WPrimeBalanceResult, error) {
	if downsample != nil && (*downsample < downsampleMin || *downsample > downsampleMax) {
		return nil, ErrDownsampleInvalid
	}
	if _, err := s.resolve(ctx, id); err != nil {
		return nil, err
	}
	series, _, err := s.streams.LoadForWorkout(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(series) == 0 {
		return nil, ErrStreamsNotFound
	}
	power := series[StreamPower]
	if len(power) == 0 {
		return nil, ErrPowerStreamMissing
	}

	wPrimeJ := wPrimeKJ * 1000
	bal := wPrimeBalance(power, cpWatts, wPrimeJ)
	res := &WPrimeBalanceResult{
		WorkoutID: id.String(),
		Params:    WPrimeParams{CPWatts: cpWatts, WPrimeKJ: wPrimeKJ},
		DurationS: len(power),
		Summary:   wPrimeSummarize(bal, wPrimeJ),
	}
	if !summaryOnly {
		vals := bal
		if downsample != nil {
			vals = bucketMean(bal, *downsample)
		}
		series := make([]float64, len(vals))
		for i, v := range vals {
			series[i] = v / 1000 // kJ; rounded at the handler boundary
		}
		res.Series = series
		res.Downsample = downsample
	}
	return res, nil
}

func (s *Service) resolve(ctx context.Context, id uuid.UUID) (*workouts.Workout, error) {
	w, err := s.workout.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			return nil, ErrWorkoutNotFound
		}
		return nil, err
	}
	return w, nil
}

// executionMetrics derives VI, EF and aerobic decoupling from the 1 Hz series.
// Any metric whose inputs are missing (no power → no VI; no/low HR coverage → no
// EF/decoupling) is left nil. Activities shorter than minMetricSamples yield no
// metrics at all — too short to be meaningful.
func executionMetrics(power, speed, hr []float64) ExecutionMetrics {
	var m ExecutionMetrics
	if maxLen(power, speed, hr) < minMetricSamples {
		return m
	}

	hrMean, hrCov := nonZeroMean(hr)
	hasHR := len(hr) > 0 && hrCov >= hrCoverageMin

	np, npOK := normalizedPower(power)
	meanP := mean(power)
	if npOK && meanP > 0 {
		vi := numfmt.Round2(np / meanP)
		m.VariabilityIndex = &vi
	}

	if hasHR && hrMean > 0 {
		switch {
		case npOK:
			ef := numfmt.Round3(np / hrMean)
			m.EfficiencyFactor = &ef
		case len(speed) > 0:
			ef := numfmt.Round3(mean(speed) / hrMean)
			m.EfficiencyFactor = &ef
		}
	}

	if hasHR {
		output := power
		if len(output) == 0 {
			output = speed
		}
		if d, ok := decoupling(output, hr); ok {
			m.DecouplingPct = &d
		}
	}
	return m
}

// normalizedPower is the Coggan NP: the 4th root of the mean of the 4th powers
// of the 30-second rolling-average power. Returns ok=false for series shorter
// than the window.
func normalizedPower(power []float64) (float64, bool) {
	n := len(power)
	if n < npWindow {
		return 0, false
	}
	// Rolling mean over npWindow samples via a running sum.
	var sum float64
	for i := 0; i < npWindow; i++ {
		sum += power[i]
	}
	var quartic float64
	count := 0
	for i := npWindow; ; i++ {
		avg := sum / float64(npWindow)
		quartic += avg * avg * avg * avg
		count++
		if i >= n {
			break
		}
		sum += power[i] - power[i-npWindow]
	}
	if count == 0 {
		return 0, false
	}
	return math.Pow(quartic/float64(count), 0.25), true
}

// decoupling is the aerobic (Pw:HR or Spd:HR) decoupling percentage: the drop in
// output-per-heartbeat from the first half of the activity to the second, as a
// percentage of the first half. Positive = fatigue (efficiency fell); negative =
// the second half was more efficient. ok=false without adequate HR coverage in
// both halves.
func decoupling(output, hr []float64) (float64, bool) {
	n := len(output)
	if len(hr) < n {
		n = len(hr)
	}
	if n < minMetricSamples {
		return 0, false
	}
	half := n / 2
	o1, o2 := mean(output[:half]), mean(output[half:n])
	h1, c1 := nonZeroMean(hr[:half])
	h2, c2 := nonZeroMean(hr[half:n])
	if c1 < hrCoverageMin || c2 < hrCoverageMin || h1 == 0 || h2 == 0 {
		return 0, false
	}
	r1, r2 := o1/h1, o2/h2
	if r1 == 0 {
		return 0, false
	}
	return numfmt.Round1((r1 - r2) / r1 * 100), true
}

// bucketMean reduces samples to `target` contiguous equal-width buckets, each the
// mean of its span. Returns the input untouched when target is >= the sample
// count (never upsamples) or non-positive.
func bucketMean(samples []float64, target int) []float64 {
	n := len(samples)
	if target <= 0 || target >= n {
		return samples
	}
	out := make([]float64, target)
	for b := 0; b < target; b++ {
		start := b * n / target
		end := (b + 1) * n / target
		if end <= start {
			end = start + 1
		}
		out[b] = mean(samples[start:end])
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}

// nonZeroMean returns the mean over non-zero samples and the fraction of samples
// that were non-zero (0..1). Used to ignore HR dropouts (stored as 0).
func nonZeroMean(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	nz := 0
	for _, v := range xs {
		if v != 0 {
			sum += v
			nz++
		}
	}
	if nz == 0 {
		return 0, 0
	}
	return sum / float64(nz), float64(nz) / float64(len(xs))
}

func maxLen(series ...[]float64) int {
	m := 0
	for _, s := range series {
		if len(s) > m {
			m = len(s)
		}
	}
	return m
}

func roundSamples(xs []float64) []float64 {
	out := make([]float64, len(xs))
	for i, v := range xs {
		out[i] = numfmt.Round2(v)
	}
	return out
}
