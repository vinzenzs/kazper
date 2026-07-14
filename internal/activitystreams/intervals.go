package activitystreams

import "math"

// Interval detection: find sustained work efforts in a 1 Hz power stream with a
// deterministic, parameter-free procedure — smooth, split the ride's own power
// distribution by Otsu's method, assemble supra-threshold spans. No CP, no
// config, no tuning: the ride defines its own work/rest contrast (design D1).

const (
	// smoothWindow is the centered rolling-mean width (seconds); it damps surges
	// so a single spike doesn't read as an interval boundary.
	smoothWindow = 30
	// gapMergeS: sub-threshold dips of this many seconds or fewer don't split an
	// effort (a gear lull mid-interval is still one interval).
	gapMergeS = 30
	// minEffortS: assembled spans shorter than this are sprint/burst territory,
	// not intervals (best-effort ladder owns those).
	minEffortS = 60
	// bimodalValleyMax gates "meaningfully bimodal" (design D3): Otsu locates the
	// work/rest split, but between-class variance alone runs high even for
	// unimodal noise (a Gaussian split at its mean scores ≈0.64), so the gate is
	// a histogram VALLEY test — the sample density at the split must be well below
	// the density of the two surrounding modes. A steady ride's Otsu split sits on
	// its single dense peak (ratio ≈1 → rejected); an interval ride's split sits
	// in a sparse valley between two peaks (ratio ≈0 → accepted).
	bimodalValleyMax = 0.5
	// otsuBins is the histogram resolution for the threshold search.
	otsuBins = 128

	reasonNoEfforts = "no_distinct_efforts"
)

// detectIntervals runs the full pure pipeline over a raw power series and
// returns the (unrounded) result. Boundaries come from the smoothed series;
// avg/max/kj come from the raw series. Rounding happens at the handler boundary.
func detectIntervals(workoutID string, power []float64) IntervalsResult {
	res := IntervalsResult{
		WorkoutID: workoutID,
		Intervals: []Interval{},
		Rests:     []Rest{},
	}
	if len(power) == 0 {
		res.Reason = reasonNoEfforts
		return res
	}

	smoothed := smoothMean(power, smoothWindow)
	thr, valleyRatio, ok := otsuThreshold(smoothed)
	if !ok || valleyRatio > bimodalValleyMax {
		res.Reason = reasonNoEfforts
		return res
	}

	spans := assembleSpans(smoothed, thr, gapMergeS, minEffortS)
	if len(spans) == 0 {
		// Bimodal enough, but no span survived the 60 s minimum — still "no
		// distinct efforts" from the athlete's view.
		res.Reason = reasonNoEfforts
		return res
	}

	t := thr
	res.ThresholdW = &t
	var workTotal int
	var effortSecSum, effortWSum float64
	for i, sp := range spans {
		sum, mx := 0.0, math.Inf(-1)
		for j := sp.start; j < sp.end; j++ {
			sum += power[j]
			if power[j] > mx {
				mx = power[j]
			}
		}
		dur := sp.end - sp.start
		iv := Interval{
			N:         i + 1,
			StartS:    sp.start,
			EndS:      sp.end,
			DurationS: dur,
			AvgW:      sum / float64(dur),
			MaxW:      mx,
			KJ:        sum / 1000,
		}
		res.Intervals = append(res.Intervals, iv)
		workTotal += dur
		effortSecSum += float64(dur)
		effortWSum += iv.AvgW
	}

	for i := 0; i+1 < len(spans); i++ {
		gs, ge := spans[i].end, spans[i+1].start
		if ge <= gs {
			continue
		}
		sum := 0.0
		for j := gs; j < ge; j++ {
			sum += power[j]
		}
		res.Rests = append(res.Rests, Rest{
			AfterN:    i + 1,
			DurationS: ge - gs,
			AvgW:      sum / float64(ge-gs),
		})
	}

	n := float64(len(spans))
	res.Summary = IntervalSummary{
		Count:       len(spans),
		WorkTotalS:  workTotal,
		MeanEffortS: effortSecSum / n,
		MeanEffortW: effortWSum / n,
	}
	return res
}

// smoothMean is a centered rolling mean of width `window` (seconds), clamped at
// the edges. O(n) via a prefix sum.
func smoothMean(x []float64, window int) []float64 {
	n := len(x)
	out := make([]float64, n)
	half := window / 2
	prefix := make([]float64, n+1)
	for i, v := range x {
		prefix[i+1] = prefix[i] + v
	}
	for i := range x {
		lo := i - half
		if lo < 0 {
			lo = 0
		}
		hi := i + half
		if hi > n {
			hi = n
		}
		out[i] = (prefix[hi] - prefix[lo]) / float64(hi-lo)
	}
	return out
}

// otsuThreshold finds the work/rest split that maximizes between-class variance
// over a histogram of the values, and returns the threshold, a VALLEY RATIO
// (density at the split ÷ the smaller of the two surrounding peak densities —
// low means a genuine bimodal valley), and ok=false for a flat/empty series.
func otsuThreshold(x []float64) (threshold, valleyRatio float64, ok bool) {
	n := len(x)
	if n == 0 {
		return 0, 0, false
	}
	mn, mx := x[0], x[0]
	for _, v := range x {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	if mx-mn < 1e-9 {
		return 0, 0, false // flat: one power mode
	}

	hist := make([]int, otsuBins)
	step := (mx - mn) / otsuBins
	for _, v := range x {
		b := int((v - mn) / step)
		if b >= otsuBins {
			b = otsuBins - 1
		}
		hist[b]++
	}
	binVal := func(b int) float64 { return mn + (float64(b)+0.5)*step }

	total := float64(n)
	var sumAll float64
	for b := 0; b < otsuBins; b++ {
		sumAll += binVal(b) * float64(hist[b])
	}

	var w0, sum0, bestVar float64
	bestVar = -1
	bestBin := 0
	for b := 0; b < otsuBins; b++ {
		w0 += float64(hist[b])
		if w0 == 0 {
			continue
		}
		w1 := total - w0
		if w1 == 0 {
			break
		}
		sum0 += binVal(b) * float64(hist[b])
		m0 := sum0 / w0
		m1 := (sumAll - sum0) / w1
		bv := (w0 / total) * (w1 / total) * (m0 - m1) * (m0 - m1)
		if bv > bestVar {
			bestVar = bv
			bestBin = b
		}
	}

	// Bimodality via valley depth: the split bin's density vs the peak density on
	// each side. peak1 (work side) uses bins ABOVE the split.
	peak0, peak1 := 0, 0
	for b := 0; b <= bestBin; b++ {
		if hist[b] > peak0 {
			peak0 = hist[b]
		}
	}
	for b := bestBin + 1; b < otsuBins; b++ {
		if hist[b] > peak1 {
			peak1 = hist[b]
		}
	}
	minPeak := peak0
	if peak1 < minPeak {
		minPeak = peak1
	}
	if minPeak == 0 {
		// One side is empty around the split — degenerate, not two modes.
		return 0, math.Inf(1), true
	}
	valleyRatio = float64(hist[bestBin]) / float64(minPeak)

	// Threshold at the upper edge of the background bin: samples strictly above
	// the background band are "work".
	threshold = mn + float64(bestBin+1)*step
	return threshold, valleyRatio, true
}

type span struct{ start, end int }

// assembleSpans turns the above-threshold mask into work spans, merges spans
// separated by ≤ gapMerge seconds, and discards spans shorter than minEffort.
func assembleSpans(smoothed []float64, threshold float64, gapMerge, minEffort int) []span {
	n := len(smoothed)
	var raw []span
	for i := 0; i < n; {
		if smoothed[i] >= threshold {
			j := i
			for j < n && smoothed[j] >= threshold {
				j++
			}
			raw = append(raw, span{i, j})
			i = j
		} else {
			i++
		}
	}

	var merged []span
	for _, s := range raw {
		if len(merged) > 0 && s.start-merged[len(merged)-1].end <= gapMerge {
			merged[len(merged)-1].end = s.end
		} else {
			merged = append(merged, s)
		}
	}

	var out []span
	for _, s := range merged {
		if s.end-s.start >= minEffort {
			out = append(out, s)
		}
	}
	return out
}
