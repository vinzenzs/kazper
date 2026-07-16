package heat

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// GarminSweatSignal derives a personal sweat rate from the device's own
// estimated sweat loss on recent long outdoor sessions.
//
// Why this and not the sweat-rate field test: that capability requires explicit
// pre/post weights by design — "inferring pre/post would be a guess dressed as
// data" — and nothing persists its result, so there is no stored field test to
// read. Garmin's per-workout sweat_loss_ml IS stored and IS real device data,
// which makes it the best personal signal available. It is reported under its
// own source label rather than as a measurement, so a device estimate can never
// pass for a field test.
type GarminSweatSignal struct {
	Repo *workouts.Repo
	// WindowDays bounds how far back a comparable session may be. Zero means
	// the acclimatization window, which is the same "recent enough to describe
	// this athlete right now" idea.
	WindowDays int
	// Now is injectable for tests.
	Now func() time.Time
}

// minSweatSessionMin ignores short sessions: sweat loss over 20 minutes is
// mostly noise and start-up transient.
const minSweatSessionMin = 60.0

// LatestSweatSignal averages ml/hr across qualifying recent sessions. Returns
// nil when nothing qualifies — the caller then flags the generic default rather
// than inventing a personal number.
func (g GarminSweatSignal) LatestSweatSignal(ctx context.Context) *SweatSignal {
	if g.Repo == nil {
		return nil
	}
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	days := g.WindowDays
	if days <= 0 {
		days = AcclimWindowDays
	}

	to := now().UTC()
	from := to.AddDate(0, 0, -days)
	status := string(workouts.StatusCompleted)

	sessions, err := g.Repo.List(ctx, from, to, nil, &status)
	if err != nil {
		return nil // fail-open: no signal, flagged default
	}

	var sum float64
	var n int
	for _, w := range sessions {
		// Indoor sweat loss says nothing about outdoor fluid needs, and a
		// session without a device estimate says nothing at all.
		if w.Environment != nil && *w.Environment == workouts.EnvironmentIndoor {
			continue
		}
		if w.SweatLossML == nil || *w.SweatLossML <= 0 {
			continue
		}
		hours := w.EndedAt.Sub(w.StartedAt).Hours()
		if hours <= 0 || hours*60 < minSweatSessionMin {
			continue
		}
		sum += *w.SweatLossML / hours
		n++
	}
	if n == 0 {
		return nil
	}
	return &SweatSignal{
		MlPerHour: sum / float64(n),
		Source:    SourceGarminSweatLoss,
		Sessions:  n,
	}
}
