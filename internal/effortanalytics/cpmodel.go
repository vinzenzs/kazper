package effortanalytics

import "math"

// selectInBand keeps only the windowed-best points whose duration is inside the
// CP validity band, projecting each CurvePoint (power metric) into a CPPoint.
// The curve already returns one MAX point per duration, so the result has one
// point per distinct in-band duration.
func selectInBand(curve []CurvePoint) []CPPoint {
	out := []CPPoint{}
	for _, c := range curve {
		if c.DurationS >= cpBandLowS && c.DurationS <= cpBandHighS {
			out = append(out, CPPoint{
				DurationS: c.DurationS,
				Watts:     c.Value,
				WorkoutID: c.WorkoutID,
				Date:      c.Date,
			})
		}
	}
	return out
}

// gateCP reports why the in-band points cannot support a fit, or "" when they
// can. Checked in order: too few distinct durations, then too narrow a span.
func gateCP(pts []CPPoint) string {
	if len(pts) < cpMinPoints {
		return "insufficient_points"
	}
	minD, maxD := pts[0].DurationS, pts[0].DurationS
	for _, p := range pts[1:] {
		if p.DurationS < minD {
			minD = p.DurationS
		}
		if p.DurationS > maxD {
			maxD = p.DurationS
		}
	}
	if float64(maxD) < cpMinSpanRatio*float64(minD) {
		return "span_too_narrow"
	}
	return ""
}

// fitCPModel fits the 2-parameter critical-power model in work–time form:
// work_j = cp·t + w_prime, by ordinary least squares over the (t, watts·t)
// points. CP is the slope (W), W′ the intercept (J). Fit quality: r_squared of
// the linear work–time regression, and rmse_w = RMSE of predicted-vs-actual
// POWER (cp + w′/t) per point, so residuals read in watts. Assumes at least two
// points with a non-degenerate t spread (the caller gates ≥3 distinct durations
// first). Full precision — rounding happens at the response boundary.
func fitCPModel(pts []CPPoint) CPModel {
	n := float64(len(pts))
	var sx, sy, sxx, sxy float64
	for _, p := range pts {
		t := float64(p.DurationS)
		w := p.Watts * t // work (J)
		sx += t
		sy += w
		sxx += t * t
		sxy += t * w
	}
	denom := n*sxx - sx*sx
	cp := (n*sxy - sx*sy) / denom  // slope → CP (W)
	wPrime := (sy - cp*sx) / n     // intercept → W′ (J)

	// R² of the work–time linear fit.
	yBar := sy / n
	var ssRes, ssTot float64
	for _, p := range pts {
		t := float64(p.DurationS)
		w := p.Watts * t
		pred := cp*t + wPrime
		ssRes += (w - pred) * (w - pred)
		ssTot += (w - yBar) * (w - yBar)
	}
	r2 := 1.0
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}

	// RMSE in power space: modeled power at duration t is cp + w′/t.
	var sqErr float64
	for _, p := range pts {
		predP := cp + wPrime/float64(p.DurationS)
		sqErr += (p.Watts - predP) * (p.Watts - predP)
	}
	rmse := math.Sqrt(sqErr / n)

	return CPModel{
		CPWatts:  cp,
		WPrimeKJ: wPrime / 1000,
		RSquared: r2,
		RMSEW:    rmse,
	}
}
