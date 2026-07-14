package activitystreams

import "math"

// Quadrant analysis (Coggan force/velocity): each pedaling sample plotted as
// average effective pedal force (AEPF, N) vs circumferential pedal velocity
// (CPV, m/s), split into four quadrants at a reference point implied by the
// athlete's CP at a self-selected cadence. Shows HOW power is produced —
// grinding high-force/low-cadence vs spinning — not just how much. Pure: no I/O,
// no athlete-config; the reference params are explicit (the W′bal convention).
//
//	CPV  = cadence(rpm) × crank(m) × 2π / 60
//	AEPF = power(W) / CPV
//
// Quadrants: I high-force/high-velocity, II high-force/low-velocity,
// III low-force/low-velocity, IV low-force/high-velocity.

const (
	defaultCrankMM    = 172.5
	crankMinMM        = 100.0
	crankMaxMM        = 220.0
	quadrantScatterMax = 1000
)

// cpv converts a cadence (rpm) to circumferential pedal velocity (m/s).
func cpv(cadenceRPM, crankM float64) float64 {
	return cadenceRPM * crankM * 2 * math.Pi / 60
}

// quadrantAnalysis classifies paired power+cadence samples against the reference
// point from (cpWatts, cadenceRPM, crankMM). Samples with power ≤ 0 or cadence
// ≤ 0 (coasting/dropout) are excluded and counted, never diluting the shares.
// Full precision; rounding + summary_only happen at the handler boundary.
func quadrantAnalysis(power, cadence []float64, cpWatts, cadenceRPM, crankMM float64) QuadrantResult {
	crankM := crankMM / 1000
	cpvRef := cpv(cadenceRPM, crankM)
	aepfRef := 0.0
	if cpvRef > 0 {
		aepfRef = cpWatts / cpvRef
	}

	n := len(power)
	if len(cadence) > n {
		n = len(cadence)
	}

	var q [4]int
	var scatter []QuadrantPoint
	excluded := 0
	for i := 0; i < n; i++ {
		var p, c float64
		if i < len(power) {
			p = power[i]
		}
		if i < len(cadence) {
			c = cadence[i]
		}
		if p <= 0 || c <= 0 {
			excluded++
			continue
		}
		v := cpv(c, crankM)
		aepf := 0.0
		if v > 0 {
			aepf = p / v
		}
		switch {
		case aepf >= aepfRef && v >= cpvRef:
			q[0]++
		case aepf >= aepfRef && v < cpvRef:
			q[1]++
		case aepf < aepfRef && v < cpvRef:
			q[2]++
		default:
			q[3]++
		}
		scatter = append(scatter, QuadrantPoint{AEPFN: aepf, CPVMps: v})
	}

	pedaling := len(scatter)
	share := func(count int) float64 {
		if pedaling == 0 {
			return 0
		}
		return float64(count) / float64(pedaling) * 100
	}

	return QuadrantResult{
		Params: QuadrantParams{CPWatts: cpWatts, CadenceRPM: cadenceRPM, CrankMM: crankMM},
		Summary: QuadrantSummary{
			Q1Pct:     share(q[0]),
			Q2Pct:     share(q[1]),
			Q3Pct:     share(q[2]),
			Q4Pct:     share(q[3]),
			PedalingS: pedaling,
			ExcludedS: excluded,
			AEPFRefN:  aepfRef,
			CPVRefMps: cpvRef,
		},
		Scatter: downsampleScatter(scatter, quadrantScatterMax),
	}
}

// downsampleScatter systematically thins the paired points to at most `max` —
// a scatter needs shape, not fidelity. Returns the input unchanged when small.
func downsampleScatter(pts []QuadrantPoint, max int) []QuadrantPoint {
	if len(pts) <= max {
		return pts
	}
	stride := (len(pts) + max - 1) / max // ceil
	out := make([]QuadrantPoint, 0, max)
	for i := 0; i < len(pts); i += stride {
		out = append(out, pts[i])
	}
	return out
}
