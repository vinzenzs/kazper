// Package stats holds the small pure statistics shared across capabilities.
//
// It exists so the tie-aware Spearman has ONE implementation: it was extracted
// from the wellness capability when heat-analytics needed the same correlation,
// and a second copy would have been free to drift from the first.
package stats

import (
	"math"
	"sort"
)

// Spearman is the rank correlation (tie-aware): rank both series with average
// ranks for ties, then Pearson on the ranks. The right measure when the inputs
// are ordinal or non-linear — wellness scores are 1–5, and a heat response is
// monotone but hardly linear. Pure — hand-computable fixtures.
func Spearman(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 2 {
		return 0
	}
	return pearson(rank(a), rank(b))
}

// rank returns the average (fractional) ranks of the values — tied values share
// the mean of the ranks they span.
func rank(x []float64) []float64 {
	n := len(x)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool { return x[idx[i]] < x[idx[j]] })

	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j+1 < n && x[idx[j+1]] == x[idx[i]] {
			j++
		}
		// Positions i..j (0-based) are tied → average rank (1-based).
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			ranks[idx[k]] = avg
		}
		i = j + 1
	}
	return ranks
}

// pearson is the Pearson correlation of two equal-length series (0 for a
// degenerate/zero-variance input).
func pearson(a, b []float64) float64 {
	n := float64(len(a))
	var sa, sb float64
	for i := range a {
		sa += a[i]
		sb += b[i]
	}
	ma, mb := sa/n, sb/n
	var cov, va, vb float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}
