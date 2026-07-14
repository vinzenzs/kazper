package activitystreams

// wPrimeBalance computes the differential (Froncioni–Clarke–Skiba) W′ balance
// over a 1 Hz power series given critical power `cp` (W) and anaerobic work
// capacity `wPrimeJ` (J). It starts full (bal = W′) and, per second: above CP,
// drains at `P − CP` J/s; at or below CP, recharges toward W′ at a rate
// proportional to the deficit and how far below CP the athlete is. The balance
// is NOT clamped at zero — a negative floor is diagnostic (the ride demonstrated
// more anaerobic work than the supplied W′ allows, i.e. stale params). Returns
// the balance in JOULES per sample.
func wPrimeBalance(power []float64, cp, wPrimeJ float64) []float64 {
	bal := make([]float64, len(power))
	cur := wPrimeJ
	for i, p := range power {
		if p > cp {
			cur -= p - cp
		} else {
			cur += (wPrimeJ - cur) * (cp - p) / wPrimeJ
		}
		bal[i] = cur
	}
	return bal
}

// wPrimeSummarize derives the ride's summary from the full-resolution balance
// series (J) and the supplied W′ (J): the minimum balance and the second it
// occurred, the ending balance, the max depletion as a percentage of W′ (100% =
// fully emptied; >100% when the minimum went negative), and the total seconds
// spent below 25% of W′. An empty series yields the zero summary.
func wPrimeSummarize(bal []float64, wPrimeJ float64) WPrimeSummary {
	if len(bal) == 0 {
		return WPrimeSummary{}
	}
	minJ, minAt := bal[0], 0
	threshold := 0.25 * wPrimeJ
	below := 0
	for i, b := range bal {
		if b < minJ {
			minJ, minAt = b, i
		}
		if b < threshold {
			below++
		}
	}
	return WPrimeSummary{
		MinWPrimeKJ:     minJ / 1000,
		MinAtS:          minAt,
		EndWPrimeKJ:     bal[len(bal)-1] / 1000,
		MaxDepletionPct: (wPrimeJ - minJ) / wPrimeJ * 100,
		TimeBelow25PctS: below,
	}
}
