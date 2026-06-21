package trainingphases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Phase-level validation errors.
var (
	ErrPhaseNameInvalid   = errors.New("phase_name_invalid")
	ErrPhaseNameTooLong   = errors.New("phase_name_too_long")
	ErrPhaseTypeInvalid   = errors.New("phase_type_invalid")
	ErrDateRangeInvalid   = errors.New("date_range_invalid")
	ErrPatchEmpty         = errors.New("patch_empty")
	ErrMacrocycleNotFound = errors.New("macrocycle_not_found")
	ErrTargetInvalid      = errors.New("target_invalid")
)

// MaxPhaseNameLength matches the migration's CHECK constraint.
const MaxPhaseNameLength = 128

// macrocycleChecker is the narrow read the service needs to FK-validate a
// phase's macrocycle_id. Satisfied by *macrocycle.Repo; cross-injected (nil-safe)
// via SetMacrocycleChecker so trainingphases stays decoupled from macrocycle and
// avoids an import cycle (macrocycle reads training_phases for its member list).
type macrocycleChecker interface {
	MacrocycleExists(ctx context.Context, id uuid.UUID) (bool, error)
}

// PhasesService validates and orchestrates writes to PhasesRepo.
type PhasesService struct {
	repo        *PhasesRepo
	templates   *TemplatesRepo    // for FK pre-validation on Insert/Patch
	macrocycles macrocycleChecker // optional; nil ⇒ skip macrocycle_id FK pre-check
}

func NewPhasesService(repo *PhasesRepo, templates *TemplatesRepo) *PhasesService {
	return &PhasesService{repo: repo, templates: templates}
}

// SetMacrocycleChecker cross-injects the macrocycle-existence reader used to
// pre-validate a phase's macrocycle_id. Optional: when unset, the app-level
// check is skipped and the DB FK is the only guard. Wired in httpserver.Run().
func (s *PhasesService) SetMacrocycleChecker(m macrocycleChecker) { s.macrocycles = m }

// CreateInput is what the POST /phases handler passes in after JSON decode.
type CreateInput struct {
	Name              string
	Type              PhaseType
	StartDate         time.Time
	EndDate           time.Time
	DefaultTemplateID *uuid.UUID
	Notes             *string
	Methodology       *string
	MacrocycleID      *uuid.UUID
	MacrocycleOrdinal *int
	TargetWeeklyTSS   *float64
	TargetWeeklyHours *float64
}

// Create validates input, verifies the template FK if present, inserts the
// phase, then re-reads it via GetByID so the response includes the joined
// DefaultTemplateName.
func (s *PhasesService) Create(ctx context.Context, in CreateInput) (*Phase, error) {
	cleanName := strings.TrimSpace(in.Name)
	if cleanName == "" {
		return nil, ErrPhaseNameInvalid
	}
	if len(cleanName) > MaxPhaseNameLength {
		return nil, ErrPhaseNameTooLong
	}
	if !in.Type.IsValid() {
		return nil, ErrPhaseTypeInvalid
	}
	if in.EndDate.Before(in.StartDate) {
		return nil, ErrDateRangeInvalid
	}
	if in.DefaultTemplateID != nil {
		if _, err := s.templates.GetByID(ctx, *in.DefaultTemplateID); err != nil {
			return nil, err
		}
	}
	if err := validateTarget(in.TargetWeeklyTSS); err != nil {
		return nil, err
	}
	if err := validateTarget(in.TargetWeeklyHours); err != nil {
		return nil, err
	}
	if in.MacrocycleID != nil {
		if err := s.assertMacrocycleExists(ctx, *in.MacrocycleID); err != nil {
			return nil, err
		}
	}
	p := &Phase{
		Name:              cleanName,
		Type:              in.Type,
		StartDate:         in.StartDate,
		EndDate:           in.EndDate,
		DefaultTemplateID: in.DefaultTemplateID,
		Notes:             in.Notes,
		Methodology:       in.Methodology,
		MacrocycleID:      in.MacrocycleID,
		MacrocycleOrdinal: in.MacrocycleOrdinal,
		TargetWeeklyTSS:   in.TargetWeeklyTSS,
		TargetWeeklyHours: in.TargetWeeklyHours,
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return nil, err
	}
	// Re-read so DefaultTemplateName is populated via the JOIN.
	return s.repo.GetByID(ctx, p.ID)
}

// GetByID returns a phase or ErrPhaseNotFound.
func (s *PhasesService) GetByID(ctx context.Context, id uuid.UUID) (*Phase, error) {
	return s.repo.GetByID(ctx, id)
}

// ListIntersecting returns phases intersecting [from, to].
func (s *PhasesService) ListIntersecting(ctx context.Context, from, to time.Time) ([]*Phase, error) {
	return s.repo.ListIntersecting(ctx, from, to)
}

// Patch validates the patch params, then applies. Patching dates requires
// the resulting start <= end check post-merge: fetch the current row first
// when either date is being patched, so the "patched start vs current end"
// (or vice versa) combination is validated.
func (s *PhasesService) Patch(ctx context.Context, id uuid.UUID, p PatchParams) (*Phase, error) {
	if !p.HasUpdates() {
		return nil, ErrPatchEmpty
	}
	if p.Name != nil {
		clean := strings.TrimSpace(*p.Name)
		if clean == "" {
			return nil, ErrPhaseNameInvalid
		}
		if len(clean) > MaxPhaseNameLength {
			return nil, ErrPhaseNameTooLong
		}
		p.Name = &clean
	}
	if p.Type != nil && !p.Type.IsValid() {
		return nil, ErrPhaseTypeInvalid
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
	if !p.ClearDefaultTemplateID && p.DefaultTemplateID != nil {
		if _, err := s.templates.GetByID(ctx, *p.DefaultTemplateID); err != nil {
			return nil, err
		}
	}
	if !p.ClearTargetWeeklyTSS {
		if err := validateTarget(p.TargetWeeklyTSS); err != nil {
			return nil, err
		}
	}
	if !p.ClearTargetWeeklyHours {
		if err := validateTarget(p.TargetWeeklyHours); err != nil {
			return nil, err
		}
	}
	if !p.ClearMacrocycleID && p.MacrocycleID != nil {
		if err := s.assertMacrocycleExists(ctx, *p.MacrocycleID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Patch(ctx, id, p); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a phase. Returns ErrPhaseNotFound on a miss.
func (s *PhasesService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// validateTarget rejects a negative progression target. A nil target (unset) and
// a non-negative value both pass.
func validateTarget(v *float64) error {
	if v != nil && *v < 0 {
		return ErrTargetInvalid
	}
	return nil
}

// assertMacrocycleExists pre-validates a phase's macrocycle_id FK. When no
// checker is wired the app-level check is skipped (the DB FK remains the guard).
func (s *PhasesService) assertMacrocycleExists(ctx context.Context, id uuid.UUID) error {
	if s.macrocycles == nil {
		return nil
	}
	exists, err := s.macrocycles.MacrocycleExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrMacrocycleNotFound
	}
	return nil
}
