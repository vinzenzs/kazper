package httpserver

import (
	"context"
	"errors"
	"time"

	"github.com/vinzenzs/kazper/internal/bodyweight"
)

// latestBodyWeight adapts *bodyweight.Repo to effortanalytics.WeightProvider so
// the power-profile endpoint can fall back to the most-recent stored weight
// without effort-analytics importing the bodyweight package. `found=false` (not
// an error) when no entry has ever been logged.
type latestBodyWeight struct {
	repo *bodyweight.Repo
}

func (l latestBodyWeight) LatestWeightKg(ctx context.Context) (float64, bool, error) {
	// A small forward buffer so an entry logged "now" still counts (LatestBefore
	// is strict-less-than on logged_at).
	e, err := l.repo.LatestBefore(ctx, time.Now().Add(24*time.Hour))
	if err != nil {
		if errors.Is(err, bodyweight.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return e.WeightKg, true, nil
}
