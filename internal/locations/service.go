package locations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors — mapped 1:1 to API error codes by the handler.
var (
	ErrNameRequired  = errors.New("name_required")
	ErrLatLonInvalid = errors.New("lat_lon_invalid")
	ErrDateInvalid   = errors.New("date_invalid")
	ErrRangeInvalid  = errors.New("range_invalid")
	ErrNoteTooLong   = errors.New("note_too_long")
	ErrUnconfigured  = errors.New("location_unconfigured")
)

const (
	maxNameLen = 200
	maxNoteLen = 2000
	dateLayout = "2006-01-02"
)

// Home is the configured home location. Set is false when HOME_LAT/HOME_LON are
// unset — a legitimate state, resolved as `location_unconfigured` rather than a
// guessed city.
type Home struct {
	Lat float64
	Lon float64
	Set bool
}

// Service validates and resolves location periods.
type Service struct {
	repo *Repo
	home Home
}

func NewService(repo *Repo, home Home) *Service {
	return &Service{repo: repo, home: home}
}

// CreateInput is the payload for POST /locations.
type CreateInput struct {
	StartDate string
	EndDate   string
	Name      string
	Lat       *float64
	Lon       *float64
	Note      *string
}

// Create validates and stores a period. Overlaps with existing periods are
// accepted — nesting a weekend trip inside a camp is a real thing to log, and
// resolution's latest-start rule makes it unambiguous.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Period, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > maxNameLen {
		return nil, ErrNameRequired
	}
	// Both coordinates are required and range-checked. A pointer distinguishes
	// "absent" from a legitimate 0 (Null Island is a coordinate; a missing lat
	// is not).
	if in.Lat == nil || in.Lon == nil ||
		*in.Lat < -90 || *in.Lat > 90 || *in.Lon < -180 || *in.Lon > 180 {
		return nil, ErrLatLonInvalid
	}
	start, err := parseDate(in.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(in.EndDate)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, ErrRangeInvalid
	}
	if in.Note != nil && len(*in.Note) > maxNoteLen {
		return nil, ErrNoteTooLong
	}

	return s.repo.Insert(ctx, &Period{
		StartDate: start.Format(dateLayout),
		EndDate:   end.Format(dateLayout),
		Name:      name,
		Lat:       *in.Lat,
		Lon:       *in.Lon,
		Note:      in.Note,
	})
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Period, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List returns the periods overlapping [from, to] ascending.
func (s *Service) List(ctx context.Context, from, to string) ([]*Period, error) {
	f, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	if t.Before(f) {
		return nil, ErrRangeInvalid
	}
	return s.repo.ListOverlapping(ctx, f.Format(dateLayout), t.Format(dateLayout))
}

// LocationOn is THE resolution primitive: the covering period with the latest
// start_date, else configured home, else ErrUnconfigured. Every weather
// consumer reads through this — and so does GET /locations/resolve, which is
// why the endpoint can never disagree with what a forecast actually used.
func (s *Service) LocationOn(ctx context.Context, date time.Time) (*Resolved, error) {
	key := date.Format(dateLayout)

	covering, err := s.repo.CoveringOn(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("resolve location on %s: %w", key, err)
	}
	if covering != nil {
		return &Resolved{
			Date:   key,
			Lat:    covering.Lat,
			Lon:    covering.Lon,
			Name:   covering.Name,
			Source: SourceTravel,
		}, nil
	}
	if !s.home.Set {
		return nil, ErrUnconfigured
	}
	return &Resolved{
		Date:   key,
		Lat:    s.home.Lat,
		Lon:    s.home.Lon,
		Name:   HomeName,
		Source: SourceHome,
	}, nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, ErrDateInvalid
	}
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, ErrDateInvalid
	}
	return d, nil
}
