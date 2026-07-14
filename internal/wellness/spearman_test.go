package wellness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpearman_PerfectMonotone(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{10, 20, 30, 40, 50} // strictly increasing → rho = 1
	assert.InDelta(t, 1.0, spearman(a, b), 1e-9)
}

func TestSpearman_PerfectInverse(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{50, 40, 30, 20, 10} // strictly decreasing → rho = -1
	assert.InDelta(t, -1.0, spearman(a, b), 1e-9)
}

func TestSpearman_WithTies(t *testing.T) {
	// Ties on both sides — average-rank handling must keep |rho| ≤ 1 and match a
	// hand computation. a ranks: {1,1}→1.5, {2}→3, {3,3}→4.5; b similarly.
	a := []float64{1, 1, 2, 3, 3}
	b := []float64{2, 2, 5, 8, 8}
	rho := spearman(a, b)
	assert.InDelta(t, 1.0, rho, 1e-9, "monotone-with-matching-ties → 1")
}

func TestSpearman_NoAssociation(t *testing.T) {
	a := []float64{3, 3, 3, 3, 3} // zero variance → 0 by definition
	b := []float64{1, 2, 3, 4, 5}
	assert.Equal(t, 0.0, spearman(a, b))
}

func TestRank_AveragesTies(t *testing.T) {
	r := rank([]float64{5, 1, 5, 3}) // sorted: 1(r1), 3(r2), 5&5(r3,r4→3.5)
	assert.Equal(t, []float64{3.5, 1, 3.5, 2}, r)
}
