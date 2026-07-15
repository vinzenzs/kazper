package supplements

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors map 1:1 to API error codes.
var (
	ErrNameRequired     = errors.New("name_required")
	ErrDosePairRequired = errors.New("dose_pair_required")
	ErrDoseInvalid      = errors.New("dose_invalid")
	ErrNoteTooLong      = errors.New("note_too_long")
	ErrRangeInvalid     = errors.New("range_invalid")
	ErrRangeTooLarge    = errors.New("range_too_large")
)

const (
	maxNameLen   = 200
	maxUnitLen   = 40
	maxNoteLen   = 2000
	maxRangeDays = 92
)

// MaxRangeDays exposes the window cap for the handler's error payload.
const MaxRangeDays = maxRangeDays

// Service validates and persists supplement entries.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

// CreateInput is the decoded POST body.
type CreateInput struct {
	LoggedAt time.Time
	Name     string
	Dose     *float64
	DoseUnit *string
	Note     *string
}

// Create validates and inserts a supplement entry.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Entry, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > maxNameLen {
		return nil, ErrNameRequired
	}
	// Dose and unit are paired: both present or both absent.
	unitSet := in.DoseUnit != nil && strings.TrimSpace(*in.DoseUnit) != ""
	if (in.Dose != nil) != unitSet {
		return nil, ErrDosePairRequired
	}
	if in.Dose != nil && *in.Dose <= 0 {
		return nil, ErrDoseInvalid
	}
	if in.DoseUnit != nil && len(*in.DoseUnit) > maxUnitLen {
		return nil, ErrDoseInvalid
	}
	if in.Note != nil && len(*in.Note) > maxNoteLen {
		return nil, ErrNoteTooLong
	}
	return s.repo.Insert(ctx, &Entry{
		LoggedAt: in.LoggedAt,
		Name:     name,
		Dose:     in.Dose,
		DoseUnit: in.DoseUnit,
		Note:     in.Note,
	})
}

// Get returns an entry by id (ErrNotFound when absent).
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Entry, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete removes an entry (ErrNotFound when absent).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List returns entries in [from, to) ascending, enforcing the 92-day cap.
func (s *Service) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	if from.After(to) {
		return nil, ErrRangeInvalid
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		return nil, ErrRangeTooLarge
	}
	return s.repo.List(ctx, from, to)
}
