package publicfeed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNoActiveRace signals there is no active-macrocycle-anchored race for the
// given day — distinct from a query error, so the service maps it to a graceful
// null feed rather than a 500.
var ErrNoActiveRace = errors.New("no active anchored race")

// Repo is a read-only view over macrocycles + races for the public feed.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo { return &Repo{q: q} }

// ActiveRace resolves the A-race of the macrocycle whose [start_date, end_date]
// (inclusive) contains `today`, breaking ties by the latest start_date, and
// returns that race's name and date. The INNER JOIN means a macrocycle with a
// NULL race_id (or a dangling reference) yields no row → ErrNoActiveRace, which
// is exactly the graceful-empty case. `today` is compared as a calendar date.
func (r *Repo) ActiveRace(ctx context.Context, today time.Time) (name string, raceDate time.Time, err error) {
	const q = `
		SELECT rc.name, rc.race_date
		FROM macrocycles m
		JOIN races rc ON rc.id = m.race_id
		WHERE m.start_date <= $1::date AND m.end_date >= $1::date
		ORDER BY m.start_date DESC
		LIMIT 1`
	err = r.q.QueryRow(ctx, q, today.Format("2006-01-02")).Scan(&name, &raceDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, ErrNoActiveRace
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("query active race: %w", err)
	}
	return name, raceDate, nil
}
