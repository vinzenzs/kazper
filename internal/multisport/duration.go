package multisport

import "github.com/vinzenzs/kazper/internal/workouttemplates"

// estimatedDurationSec derives a multisport template's total time: the sum of
// every transition segment's time duration plus every sport segment's
// time-bounded step durations. It returns nil (not determinable) when any sport
// segment contains a step that is not time-bounded
// (distance/lap_button/open) or any transition's duration is not of kind time —
// mirroring how the materializer falls back when a program isn't fully timed.
// Computed on read from the stored segments; never persisted.
func estimatedDurationSec(segments []Segment) *int {
	total := 0
	for _, seg := range segments {
		if seg.IsTransition() {
			d := seg.Duration
			if d == nil || d.Kind != workouttemplates.DurationTime || d.Seconds == nil {
				return nil
			}
			total += *d.Seconds
			continue
		}
		if !allStepsTimed(seg.Steps) {
			return nil
		}
		total += workouttemplates.SumTimedDurationSec(seg.Steps)
	}
	return &total
}

// allStepsTimed reports whether every leaf step (recursing into repeat groups)
// is bounded by a positive time duration.
func allStepsTimed(steps []workouttemplates.Step) bool {
	for _, st := range steps {
		if st.Type == workouttemplates.NodeRepeat {
			if !allStepsTimed(st.Steps) {
				return false
			}
			continue
		}
		if st.Duration == nil || st.Duration.Kind != workouttemplates.DurationTime || st.Duration.Seconds == nil {
			return false
		}
	}
	return true
}
