// Package multisport is the multisport workout-template library
// (add-multisport-structured-workouts, Phase 1): a triathlon/brick session
// modeled as an ordered list of per-sport SEGMENTS (swim → T1 → bike → T2 →
// run), each with its own sport and step program, plus transition (T1/T2)
// segments. It exists alongside — not inside — workouttemplates so the
// single-sport invariant the rest of the system relies on stays intact; the
// step model and its validation are reused from workouttemplates. No Garmin
// coupling: the segment model is compiled into Garmin's multi-segment payload by
// the bridge.
package multisport

import (
	"time"

	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// SportTransition marks a transition (T1/T2) segment. It is not a real sport: a
// transition segment carries a Duration and no steps, and the bridge maps it to
// Garmin's transition sport type.
const SportTransition = "transition"

// Template mirrors a multisport_templates row: a name plus an ordered,
// non-empty list of segments. Unlike a single-sport template there is no
// top-level sport — the sport lives per segment.
type Template struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// EstimatedDurationSec is derived on read (sum of segment durations), never
	// persisted; omitted when the total is not fully time-bounded. See
	// estimatedDurationSec.
	EstimatedDurationSec *int      `json:"estimated_duration_sec,omitempty"`
	Segments             []Segment `json:"segments"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Segment is one leg of a multisport session. A non-transition segment carries
// a Sport (run/bike/swim/…) and a Steps program in the workouttemplates step
// model. A transition segment has Sport == SportTransition, a single Duration,
// and no steps.
type Segment struct {
	Sport string                  `json:"sport"`
	Steps []workouttemplates.Step `json:"steps,omitempty"`
	// EstimatedDurationSec is derived on read for SPORT segments (the sum of the
	// segment's time-bound step durations, null when not fully time-bounded);
	// transitions carry their explicit Duration instead. Never persisted.
	EstimatedDurationSec *int                       `json:"estimated_duration_sec,omitempty"`
	Duration             *workouttemplates.Duration `json:"duration,omitempty"`
}

// IsTransition reports whether the segment is a T1/T2 transition.
func (s Segment) IsTransition() bool { return s.Sport == SportTransition }
