package macrocycle

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/races"
)

// Validation errors mapping 1:1 to API error codes.
var (
	ErrNameInvalid      = errors.New("macrocycle_name_invalid")
	ErrNameTooLong      = errors.New("macrocycle_name_too_long")
	ErrDateRangeInvalid = errors.New("date_range_invalid")
	ErrRaceNotFound     = errors.New("race_not_found")
	ErrPatchEmpty       = errors.New("patch_empty")
)

// MaxNameLength bounds the user-chosen season name.
const MaxNameLength = 128

// raceLookup is the narrow read the service needs to FK-validate race_id.
// Satisfied by *races.Repo; cross-injected so macrocycle stays decoupled.
type raceLookup interface {
	GetRace(ctx context.Context, id uuid.UUID) (*races.Race, error)
}

// Service validates and orchestrates writes to Repo.
type Service struct {
	repo  *Repo
	races raceLookup
}

func NewService(repo *Repo, races raceLookup) *Service {
	return &Service{repo: repo, races: races}
}

// Create validates input, verifies the race FK when present, inserts the
// macrocycle, then re-reads it via GetByID so the response carries RaceName and
// an (empty) phases array.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Macrocycle, error) {
	cleanName := strings.TrimSpace(in.Name)
	if cleanName == "" {
		return nil, ErrNameInvalid
	}
	if len(cleanName) > MaxNameLength {
		return nil, ErrNameTooLong
	}
	if in.EndDate.Before(in.StartDate) {
		return nil, ErrDateRangeInvalid
	}
	if in.RaceID != nil {
		if err := s.assertRaceExists(ctx, *in.RaceID); err != nil {
			return nil, err
		}
	}
	m := &Macrocycle{
		Name:        cleanName,
		StartDate:   in.StartDate,
		EndDate:     in.EndDate,
		RaceID:      in.RaceID,
		Methodology: in.Methodology,
		Notes:       in.Notes,
	}
	if err := s.repo.Insert(ctx, m); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, m.ID)
}

// GetByID returns a macrocycle (with member phases) or ErrMacrocycleNotFound.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Macrocycle, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns every macrocycle ordered by start_date DESC.
func (s *Service) List(ctx context.Context) ([]*Macrocycle, error) {
	return s.repo.List(ctx)
}

// Patch validates the patch params, then applies. Date edits are validated
// against the merged row so a partial update that inverts the range is rejected.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, p PatchInput) (*Macrocycle, error) {
	if !p.HasUpdates() {
		return nil, ErrPatchEmpty
	}
	if p.Name != nil {
		clean := strings.TrimSpace(*p.Name)
		if clean == "" {
			return nil, ErrNameInvalid
		}
		if len(clean) > MaxNameLength {
			return nil, ErrNameTooLong
		}
		p.Name = &clean
	}
	if p.StartDate != nil || p.EndDate != nil {
		current, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		start := current.StartDate
		end := current.EndDate
		if p.StartDate != nil {
			start = *p.StartDate
		}
		if p.EndDate != nil {
			end = *p.EndDate
		}
		if end.Before(start) {
			return nil, ErrDateRangeInvalid
		}
	}
	if !p.ClearRaceID && p.RaceID != nil {
		if err := s.assertRaceExists(ctx, *p.RaceID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Patch(ctx, id, p); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a macrocycle. Returns ErrMacrocycleNotFound on a miss.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// assertRaceExists maps a races.ErrNotFound into the macrocycle's
// ErrRaceNotFound; any other lookup error propagates.
func (s *Service) assertRaceExists(ctx context.Context, id uuid.UUID) error {
	if _, err := s.races.GetRace(ctx, id); err != nil {
		if errors.Is(err, races.ErrNotFound) {
			return ErrRaceNotFound
		}
		return err
	}
	return nil
}
