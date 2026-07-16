package activitystreams

import "math"

// Stride analysis: where does a runner's speed come from — turnover or step
// length, and which one plateaus?
//
// Speed = cadence × step length, so the decomposition is fully determined by the
// two stored streams:
//
//	step_length_m = speed(m/s) / (cadence(spm) / 60)   — metres per single step
//
// "Step", not "stride", deliberately: this is metres per ground contact
// (Garmin's step length, ~1.0–1.3 m), and "stride" conventionally means two
// steps. The field names carry the unit.
//
// Because ln(speed) = ln(cadence) + ln(step_length), the slopes of ln(cadence)
// and ln(step_length) against ln(speed) sum to exactly 1 — which is what makes a
// contribution SPLIT meaningful rather than two unrelated correlations. Pure: no
// I/O, full precision; rounding and summary_only happen at the handler boundary.

const (
	// speedBinWidthMps buckets the observed speed range. 0.25 m/s is ~4 s/km
	// apart at running pace: fine enough to show a plateau, coarse enough that
	// bins hold real seconds.
	speedBinWidthMps = 0.25

	// minSpeedRangeMps gates the contribution split. A steady-state run simply
	// does not contain the information — fitting a slope across 0.2 m/s of
	// spread would produce a confident number about nothing.
	minSpeedRangeMps = 0.5

	strideScatterMax = 1000

	// The optional walk-break cutoff's bounds.
	minSpeedParamLow  = 0.5
	minSpeedParamHigh = 5.0
)

// ReasonInsufficientSpeedRange is returned instead of a split when the run holds
// too little speed variety to say anything about the limiter.
const ReasonInsufficientSpeedRange = "insufficient_speed_range"

// strideSample is one qualifying second.
type strideSample struct {
	speed float64
	spm   float64
	stepM float64
}

// strideAnalysis decomposes paired speed+cadence samples. minSpeed of 0 means
// "no cutoff" (only the non-positive filter applies).
func strideAnalysis(speed, cadence []float64, minSpeed float64) StrideResult {
	n := len(speed)
	if len(cadence) > n {
		n = len(cadence)
	}

	samples := make([]strideSample, 0, n)
	excluded := 0
	for i := 0; i < n; i++ {
		var s, c float64
		if i < len(speed) {
			s = speed[i]
		}
		if i < len(cadence) {
			c = cadence[i]
		}
		// Standing, dropouts, and (when asked) walk breaks are excluded and
		// counted — never diluted into a bin, the quadrant convention.
		if s <= 0 || c <= 0 || (minSpeed > 0 && s < minSpeed) {
			excluded++
			continue
		}
		samples = append(samples, strideSample{
			speed: s,
			spm:   c,
			stepM: s / (c / 60),
		})
	}

	out := StrideResult{
		Bins:      []StrideBin{},
		AnalyzedS: len(samples),
		ExcludedS: excluded,
	}
	if len(samples) == 0 {
		reason := ReasonInsufficientSpeedRange
		out.Reason = &reason
		return out
	}

	out.Bins = binSamples(samples)
	out.Scatter = downsampleStride(scatterOf(samples), strideScatterMax)

	// The split is fitted over BIN MEANS, not raw samples: thousands of
	// easy-pace seconds would otherwise swamp the sparse fast tail and the fit
	// would describe the warm-up. Weighting by each bin's seconds keeps a
	// well-populated bin influential without letting it erase the tail.
	if split, ok := contributionSplit(out.Bins); ok {
		out.Contribution = &split
	} else {
		reason := ReasonInsufficientSpeedRange
		out.Reason = &reason
	}
	return out
}

// binSamples buckets into fixed-width speed bins over the observed range,
// ascending. Empty bins are omitted — a gap in the pace distribution is not a
// zero-cadence bin.
func binSamples(samples []strideSample) []StrideBin {
	type acc struct {
		seconds int
		spmSum  float64
		stepSum float64
	}
	byBin := map[int]*acc{}
	for _, s := range samples {
		k := int(math.Floor(s.speed / speedBinWidthMps))
		a := byBin[k]
		if a == nil {
			a = &acc{}
			byBin[k] = a
		}
		a.seconds++
		a.spmSum += s.spm
		a.stepSum += s.stepM
	}

	lo, hi := math.MaxInt32, math.MinInt32
	for k := range byBin {
		if k < lo {
			lo = k
		}
		if k > hi {
			hi = k
		}
	}

	out := make([]StrideBin, 0, len(byBin))
	for k := lo; k <= hi; k++ {
		a := byBin[k]
		if a == nil {
			continue
		}
		f := float64(a.seconds)
		out = append(out, StrideBin{
			SpeedLowMps:  float64(k) * speedBinWidthMps,
			SpeedHighMps: float64(k+1) * speedBinWidthMps,
			Seconds:      a.seconds,
			CadenceSPM:   a.spmSum / f,
			StepLengthM:  a.stepSum / f,
		})
	}
	return out
}

// contributionSplit fits ln(cadence) and ln(step) against ln(speed) over the bin
// means, weighted by each bin's seconds.
//
// The identity ln(speed) = ln(cadence) + ln(step) makes the two slopes sum to 1
// exactly, so they partition the speed gain. Reported as percentages.
//
// ok=false when the qualifying bins span less than minSpeedRangeMps — the run
// holds no answer, and saying so beats inventing one.
func contributionSplit(bins []StrideBin) (StrideContribution, bool) {
	if len(bins) < 2 {
		return StrideContribution{}, false
	}
	// The span is measured across the bins' own mean speeds, not the nominal
	// edges: two adjacent bins are 0.25 m/s apart by construction and would
	// otherwise fake a range the run doesn't have.
	minMid, maxMid := math.MaxFloat64, -math.MaxFloat64
	for _, b := range bins {
		mid := (b.SpeedLowMps + b.SpeedHighMps) / 2
		minMid = math.Min(minMid, mid)
		maxMid = math.Max(maxMid, mid)
	}
	if maxMid-minMid < minSpeedRangeMps {
		return StrideContribution{}, false
	}

	// Weighted least squares of y against x in log space.
	slope := func(y func(StrideBin) float64) (float64, bool) {
		var sw, sx, sy float64
		for _, b := range bins {
			w := float64(b.Seconds)
			mid := (b.SpeedLowMps + b.SpeedHighMps) / 2
			if mid <= 0 || y(b) <= 0 {
				return 0, false
			}
			sw += w
			sx += w * math.Log(mid)
			sy += w * math.Log(y(b))
		}
		if sw == 0 {
			return 0, false
		}
		mx, my := sx/sw, sy/sw
		var num, den float64
		for _, b := range bins {
			w := float64(b.Seconds)
			dx := math.Log((b.SpeedLowMps+b.SpeedHighMps)/2) - mx
			num += w * dx * (math.Log(y(b)) - my)
			den += w * dx * dx
		}
		if den == 0 {
			return 0, false
		}
		return num / den, true
	}

	cadSlope, ok1 := slope(func(b StrideBin) float64 { return b.CadenceSPM })
	stepSlope, ok2 := slope(func(b StrideBin) float64 { return b.StepLengthM })
	if !ok1 || !ok2 {
		return StrideContribution{}, false
	}

	// The slopes sum to 1 analytically; normalising by their actual sum keeps
	// the reported pair at exactly 100% against float drift, and is a no-op
	// when the identity holds.
	total := cadSlope + stepSlope
	if total == 0 {
		return StrideContribution{}, false
	}
	return StrideContribution{
		CadencePct: cadSlope / total * 100,
		StepPct:    stepSlope / total * 100,
	}, true
}

func scatterOf(samples []strideSample) []StridePoint {
	out := make([]StridePoint, 0, len(samples))
	for _, s := range samples {
		out = append(out, StridePoint{
			SpeedMps:    s.speed,
			CadenceSPM:  s.spm,
			StepLengthM: s.stepM,
		})
	}
	return out
}

// downsampleStride systematically thins the scatter to at most `max` — a
// scatter needs shape, not fidelity (the quadrant convention).
func downsampleStride(pts []StridePoint, max int) []StridePoint {
	if len(pts) <= max {
		return pts
	}
	stride := (len(pts) + max - 1) / max // ceil
	out := make([]StridePoint, 0, max)
	for i := 0; i < len(pts); i += stride {
		out = append(out, pts[i])
	}
	return out
}
