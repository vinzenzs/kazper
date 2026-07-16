package wellness

import "github.com/vinzenzs/kazper/internal/stats"

// spearman delegates to the shared tie-aware implementation. It was extracted
// to internal/stats when heat-analytics needed the same correlation — one
// implementation, so the two can't drift apart.
func spearman(a, b []float64) float64 { return stats.Spearman(a, b) }
