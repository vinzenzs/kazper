package pmc

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/store"
)

// Repo is a read-only view over completed-workout TSS, aggregated by local day.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo { return &Repo{q: q} }

// EarliestCompletedDate returns the local (tz) date of the earliest completed
// workout. hasHistory is false when there are no completed workouts at all.
// A nil `sport` counts every completed workout; a non-nil value filters to that
// sport (per-discipline PMC — add-per-sport-pmc). The `$2 IS NULL OR sport = $2`
// predicate keeps the combined (omitted-param) query byte-identical.
func (r *Repo) EarliestCompletedDate(ctx context.Context, tz string, sport *string) (date time.Time, hasHistory bool, err error) {
	var d *time.Time
	err = r.q.QueryRow(ctx, `
		SELECT MIN((started_at AT TIME ZONE $1)::date)
		FROM workouts
		WHERE status = 'completed'
		  AND ($2::text IS NULL OR sport = $2)`, tz, sport).Scan(&d)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("earliest completed date: %w", err)
	}
	if d == nil {
		return time.Time{}, false, nil
	}
	return *d, true, nil
}

// DayTSS is one local-day aggregate over completed workouts.
type DayTSS struct {
	Date       time.Time
	TSSTotal   float64
	MissingTSS int // completed workouts on this day with NULL tss
}

// DailyTSS returns, per local day (tz) up to and including `through`, the sum of
// tss and the count of NULL-tss completed workouts. Bucketed by the local date
// of started_at (start-day attribution). Days with no completed workouts are
// simply absent (the caller zero-fills the calendar).
func (r *Repo) DailyTSS(ctx context.Context, tz string, through time.Time, sport *string) ([]DayTSS, error) {
	rows, err := r.q.Query(ctx, `
		SELECT (w.started_at AT TIME ZONE $1)::date AS d,
		       COALESCE(SUM(w.tss), 0) AS tss_total,
		       COUNT(*) FILTER (WHERE w.tss IS NULL) AS missing
		FROM workouts w
		WHERE w.status = 'completed'
		  AND (w.started_at AT TIME ZONE $1)::date <= $2::date
		  AND ($3::text IS NULL OR w.sport = $3)
		GROUP BY d
		ORDER BY d`, tz, through.Format("2006-01-02"), sport)
	if err != nil {
		return nil, fmt.Errorf("daily tss: %w", err)
	}
	defer rows.Close()

	var out []DayTSS
	for rows.Next() {
		var dt DayTSS
		if err := rows.Scan(&dt.Date, &dt.TSSTotal, &dt.MissingTSS); err != nil {
			return nil, fmt.Errorf("scan daily tss: %w", err)
		}
		out = append(out, dt)
	}
	return out, rows.Err()
}
