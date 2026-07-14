package publicfeed

import (
	"context"
	"errors"
	"time"
)

// Service assembles the public feed projection, computing the countdown in the
// configured user timezone.
type Service struct {
	repo *Repo
	loc  *time.Location
}

func NewService(repo *Repo, loc *time.Location) *Service {
	return &Service{repo: repo, loc: loc}
}

// Feed resolves the current public feed. "No active anchored race" maps to a
// fully-null Feed (graceful empty), never an error.
func (s *Service) Feed(ctx context.Context) (Feed, error) {
	now := time.Now().In(s.loc)
	name, raceDate, err := s.repo.ActiveRace(ctx, now)
	if errors.Is(err, ErrNoActiveRace) {
		return Feed{}, nil
	}
	if err != nil {
		return Feed{}, err
	}
	days := max(daysBetween(now, raceDate), 0) // floor at zero on and after race day
	return Feed{
		Race:          &Race{Name: name, RaceDate: raceDate.Format("2006-01-02")},
		DaysRemaining: &days,
	}, nil
}

// daysBetween returns the whole-day count from a's calendar date to b's calendar
// date. Both dates are normalized to UTC midnight before subtracting so DST
// transitions never skew the count.
func daysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	au := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	bu := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(bu.Sub(au).Hours() / 24)
}
