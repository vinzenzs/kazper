package bodyweight

import (
	"context"
	"errors"
	"math"
	"time"
)

// Body-weight resolution source constants — exported so the callers'
// responses can refer to the same canonical strings.
const (
	SourceExplicit       = "explicit"
	SourceRolling7dAvg   = "rolling_7d_avg"
	SourceLastBeforeDate = "last_before_date"
)

// ErrWeightDataMissing is returned by ResolveAtDate when the resolver has
// neither an explicit override nor any stored body-weight entry to fall back
// on. Callers typically map this to a 400 weight_data_missing response.
var ErrWeightDataMissing = errors.New("weight_data_missing")

// ResolveAtDate applies the canonical 4-tier date-anchored body-weight
// resolution rule used by add-protein-distribution and add-recommend-workout-fuel:
//
//  1. Explicit override (if non-nil and finite > 0) → SourceExplicit.
//  2. Rolling 7-day mean of entries in
//     [localMidnight(date − 6d), localMidnight(date + 1d)) → SourceRolling7dAvg.
//  3. Most-recent entry strictly before localMidnight(date) → SourceLastBeforeDate.
//  4. No data anywhere → ErrWeightDataMissing.
//
// `date` is interpreted as a calendar date in `loc`; the rolling window is
// computed against local midnights so DST boundaries don't shift the bucket.
// Callers validate `override` shape before calling (NaN/Inf/<=0 should be
// rejected with the appropriate per-endpoint error code, not this sentinel).
//
// Returns (kg, source, err). On error, the float and string are zero values.
func ResolveAtDate(ctx context.Context, repo *Repo, date time.Time, loc *time.Location, override *float64) (float64, string, error) {
	if override != nil {
		v := *override
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			// Defensive: callers should validate before calling, but the
			// sentinel keeps the resolver honest if they don't. The handler
			// won't see this because the per-endpoint validation rejects
			// invalid overrides earlier with its own error code.
			return 0, "", ErrWeightDataMissing
		}
		return v, SourceExplicit, nil
	}
	if repo == nil {
		return 0, "", ErrWeightDataMissing
	}

	startMidnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	endMidnight := startMidnight.Add(24 * time.Hour)
	rollingStart := startMidnight.AddDate(0, 0, -6)

	rolling, err := repo.List(ctx, rollingStart.UTC(), endMidnight.UTC())
	if err != nil {
		return 0, "", err
	}
	if len(rolling) > 0 {
		return meanWeight(rolling), SourceRolling7dAvg, nil
	}

	latest, err := repo.LatestBefore(ctx, startMidnight.UTC())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, "", ErrWeightDataMissing
		}
		return 0, "", err
	}
	return latest.WeightKg, SourceLastBeforeDate, nil
}

// meanWeight returns the arithmetic mean of an entry slice; pulled out for
// the unit tests and to keep ResolveAtDate readable.
func meanWeight(entries []*Entry) float64 {
	if len(entries) == 0 {
		return 0
	}
	var sum float64
	for _, e := range entries {
		sum += e.WeightKg
	}
	return sum / float64(len(entries))
}
