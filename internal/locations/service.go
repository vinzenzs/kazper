package locations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/weather"
)

// Sentinel errors — mapped 1:1 to API error codes by the handler.
var (
	ErrNameRequired  = errors.New("name_required")
	ErrLatLonInvalid = errors.New("lat_lon_invalid")
	ErrDateInvalid   = errors.New("date_invalid")
	ErrRangeInvalid  = errors.New("range_invalid")
	ErrNoteTooLong   = errors.New("note_too_long")
	ErrUnconfigured  = errors.New("location_unconfigured")

	// ErrPlaceNotFound: geocoding worked and matched nothing.
	ErrPlaceNotFound = errors.New("place_not_found")
	// ErrGeocodingUnavailable: geocoding could not be performed. Distinct from
	// ErrPlaceNotFound on purpose — "no such place" and "the lookup is down"
	// call for different reactions, and the write is refused either way rather
	// than stored without coordinates.
	ErrGeocodingUnavailable = errors.New("geocoding_unavailable")
)

// Geocoder resolves a place name to coordinates. Optional: an unwired geocoder
// makes `place` writes fail with ErrGeocodingUnavailable while explicit
// lat/lon writes keep working.
type Geocoder interface {
	Geocode(ctx context.Context, place string) ([]weather.Place, bool)
}

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
	repo     *Repo
	home     Home
	geocoder Geocoder
}

func NewService(repo *Repo, home Home) *Service {
	return &Service{repo: repo, home: home}
}

// SetGeocoder enables `place`-by-name writes.
func (s *Service) SetGeocoder(g Geocoder) { s.geocoder = g }

// CreateInput is the payload for POST /locations.
type CreateInput struct {
	StartDate string
	EndDate   string
	Name      string
	Lat       *float64
	Lon       *float64
	Note      *string
	// Place is an alternative to explicit coordinates: geocoded server-side.
	// Explicit Lat/Lon win when both are supplied.
	Place string
}

// Create validates and stores a period. Overlaps with existing periods are
// accepted — nesting a weekend trip inside a camp is a real thing to log, and
// resolution's latest-start rule makes it unambiguous.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Period, error) {
	// A place name stands in for coordinates AND, when no name was given, for
	// the name too — "log Mallorca July 20–28" should need nothing else.
	// Explicit coordinates win: an athlete who gives both means the numbers.
	if place := strings.TrimSpace(in.Place); place != "" && (in.Lat == nil || in.Lon == nil) {
		resolved, err := s.geocode(ctx, place)
		if err != nil {
			return nil, err
		}
		in.Lat, in.Lon = &resolved.Lat, &resolved.Lon
		if strings.TrimSpace(in.Name) == "" {
			in.Name = resolved.Name
		}
	}

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

// geocode resolves a place name to its top match. The write is refused rather
// than stored ungeocoded: a period without real coordinates would silently
// resolve every forecast to the wrong city.
func (s *Service) geocode(ctx context.Context, place string) (*weather.Place, error) {
	if s.geocoder == nil {
		return nil, ErrGeocodingUnavailable
	}
	matches, ok := s.geocoder.Geocode(ctx, place)
	if !ok {
		return nil, ErrGeocodingUnavailable
	}
	if len(matches) == 0 {
		return nil, ErrPlaceNotFound
	}
	return &matches[0], nil
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
