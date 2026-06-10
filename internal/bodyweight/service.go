package bodyweight

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrWeightKgInvalid     = errors.New("weight_kg_invalid")
	ErrBodyFatPctInvalid   = errors.New("body_fat_pct_invalid")
	ErrMuscleMassInvalid   = errors.New("muscle_mass_kg_invalid")
	ErrBodyWaterPctInvalid = errors.New("body_water_pct_invalid")
	ErrBoneMassInvalid     = errors.New("bone_mass_kg_invalid")
	ErrBMIInvalid          = errors.New("bmi_invalid")
	ErrLoggedAtFuture      = errors.New("logged_at_too_far_future")
	ErrNoteTooLong         = errors.New("note_too_long")
)

const maxNoteLen = 500

// Service orchestrates body-weight CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput is the payload for POST /weight.
type CreateInput struct {
	WeightKg     float64
	LoggedAt     time.Time
	BodyFatPct   *float64
	MuscleMassKg *float64
	BodyWaterPct *float64
	BoneMassKg   *float64
	BMI          *float64
	Note         *string
}

// Create validates and inserts a body-weight entry.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Entry, error) {
	if err := validateWeight(in.WeightKg); err != nil {
		return nil, err
	}
	if err := validateBodyFat(in.BodyFatPct); err != nil {
		return nil, err
	}
	if err := validateBiometrics(in.MuscleMassKg, in.BodyWaterPct, in.BoneMassKg, in.BMI); err != nil {
		return nil, err
	}
	if err := validateLoggedAt(in.LoggedAt); err != nil {
		return nil, err
	}
	if err := validateNote(in.Note); err != nil {
		return nil, err
	}
	e := &Entry{
		LoggedAt:     in.LoggedAt,
		WeightKg:     in.WeightKg,
		BodyFatPct:   in.BodyFatPct,
		MuscleMassKg: in.MuscleMassKg,
		BodyWaterPct: in.BodyWaterPct,
		BoneMassKg:   in.BoneMassKg,
		BMI:          in.BMI,
		Note:         in.Note,
	}
	if err := s.repo.Insert(ctx, e); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, e.ID)
}

// PatchInput is the editable subset on PATCH /weight/{id}.
type PatchInput struct {
	WeightKg     *float64
	BodyFatPct   *float64
	MuscleMassKg *float64
	BodyWaterPct *float64
	BoneMassKg   *float64
	BMI          *float64
	LoggedAt     *time.Time
	Note         *string
}

// Patch validates and applies a partial update.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*Entry, error) {
	if in.WeightKg != nil {
		if err := validateWeight(*in.WeightKg); err != nil {
			return nil, err
		}
	}
	if in.BodyFatPct != nil {
		if err := validateBodyFat(in.BodyFatPct); err != nil {
			return nil, err
		}
	}
	if err := validateBiometrics(in.MuscleMassKg, in.BodyWaterPct, in.BoneMassKg, in.BMI); err != nil {
		return nil, err
	}
	if in.LoggedAt != nil {
		if err := validateLoggedAt(*in.LoggedAt); err != nil {
			return nil, err
		}
	}
	if err := validateNote(in.Note); err != nil {
		return nil, err
	}
	if err := s.repo.Patch(ctx, id, PatchParams{
		WeightKg:     in.WeightKg,
		BodyFatPct:   in.BodyFatPct,
		MuscleMassKg: in.MuscleMassKg,
		BodyWaterPct: in.BodyWaterPct,
		BoneMassKg:   in.BoneMassKg,
		BMI:          in.BMI,
		LoggedAt:     in.LoggedAt,
		Note:         in.Note,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes an entry.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List proxies to the repo.
func (s *Service) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	return s.repo.List(ctx, from, to)
}

// ----- validators -----

func validateWeight(w float64) error {
	if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
		return ErrWeightKgInvalid
	}
	return nil
}

func validateBodyFat(p *float64) error {
	if p == nil {
		return nil
	}
	v := *p
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
		return ErrBodyFatPctInvalid
	}
	return nil
}

// validateBiometrics checks the four optional smart-scale fields. Masses must
// be positive; body water is a 0–100 percentage.
func validateBiometrics(muscleKg, waterPct, boneKg, bmi *float64) error {
	if err := positive(muscleKg, ErrMuscleMassInvalid); err != nil {
		return err
	}
	if err := percent(waterPct, ErrBodyWaterPctInvalid); err != nil {
		return err
	}
	if err := positive(boneKg, ErrBoneMassInvalid); err != nil {
		return err
	}
	if err := positive(bmi, ErrBMIInvalid); err != nil {
		return err
	}
	return nil
}

func positive(v *float64, e error) error {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0 {
		return e
	}
	return nil
}

func percent(v *float64, e error) error {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 100 {
		return e
	}
	return nil
}

func validateLoggedAt(ts time.Time) error {
	if ts.After(time.Now().Add(24 * time.Hour)) {
		return ErrLoggedAtFuture
	}
	return nil
}

func validateNote(n *string) error {
	if n == nil {
		return nil
	}
	if len(*n) > maxNoteLen {
		return ErrNoteTooLong
	}
	return nil
}
