package workouttemplates

import (
	"context"
	"errors"
	"strings"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrSportInvalid        = errors.New("sport_invalid")
	ErrNameRequired        = errors.New("name_required")
	ErrEstimatedInvalid    = errors.New("estimated_duration_sec_invalid")
	ErrStepsEmpty          = errors.New("steps_empty")
	ErrStepTypeInvalid     = errors.New("step_type_invalid")
	ErrIntentInvalid       = errors.New("intent_invalid")
	ErrDurationInvalid     = errors.New("duration_invalid")
	ErrTargetInvalid       = errors.New("target_invalid")
	ErrTargetRangeInvalid  = errors.New("target_range_invalid")
	ErrTargetSportMismatch = errors.New("target_sport_mismatch")
	ErrSecondaryTarget     = errors.New("secondary_target_invalid")
	ErrRepeatInvalid       = errors.New("repeat_invalid")
	ErrRepeatNested        = errors.New("repeat_nested")
)

// Service orchestrates template CRUD with structured-step validation.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new template.
func (s *Service) Create(ctx context.Context, t *Template) (*Template, error) {
	if err := validateTemplate(t); err != nil {
		return nil, err
	}
	t.Name = strings.TrimSpace(t.Name)
	return s.repo.Create(ctx, t)
}

// Get returns one template by id.
func (s *Service) Get(ctx context.Context, id string) (*Template, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns templates, optionally filtered by sport.
func (s *Service) List(ctx context.Context, sport string) ([]*Template, error) {
	if sport != "" && !validSport(sport) {
		return nil, ErrSportInvalid
	}
	return s.repo.List(ctx, sport)
}

// PatchInput is a partial update. A nil pointer / false Set flag means "leave
// unchanged"; for the nullable fields a true Set flag with a nil value clears
// the column (design D5).
type PatchInput struct {
	Name  *string
	Sport *string
	Steps *[]Step

	SetDescription bool
	Description    *string

	SetEstimated         bool
	EstimatedDurationSec *int
}

// Patch loads the template, applies the supplied fields, validates the result,
// and writes it back. Returns ErrNotFound when the id is unknown.
func (s *Service) Patch(ctx context.Context, id string, in PatchInput) (*Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if in.Sport != nil {
		t.Sport = *in.Sport
	}
	if in.Steps != nil {
		t.Steps = *in.Steps
	}
	if in.SetDescription {
		t.Description = in.Description
	}
	if in.SetEstimated {
		t.EstimatedDurationSec = in.EstimatedDurationSec
	}
	if err := validateTemplate(t); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, t)
}

// Delete removes a template by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ----- validation -----

func validateTemplate(t *Template) error {
	if !validSport(t.Sport) {
		return ErrSportInvalid
	}
	if strings.TrimSpace(t.Name) == "" {
		return ErrNameRequired
	}
	if t.EstimatedDurationSec != nil && *t.EstimatedDurationSec <= 0 {
		return ErrEstimatedInvalid
	}
	if len(t.Steps) == 0 {
		return ErrStepsEmpty
	}
	for i := range t.Steps {
		if err := validateNode(t.Steps[i], false, t.Sport); err != nil {
			return err
		}
	}
	return nil
}

// validateNode validates one step node. nested=true forbids further repeats
// (single-level repeat groups only). sport is the template's sport, threaded
// through so pace-kind targets can be swim-restricted at validation time.
func validateNode(n Step, nested bool, sport string) error {
	switch n.Type {
	case NodeStep:
		return validateSingleStep(n, sport)
	case NodeRepeat:
		if nested {
			return ErrRepeatNested
		}
		if n.Count < 2 {
			return ErrRepeatInvalid
		}
		if len(n.Steps) == 0 {
			return ErrRepeatInvalid
		}
		for i := range n.Steps {
			// A repeat's children must be single steps; a nested repeat trips
			// ErrRepeatNested via the nested=true flag.
			if err := validateNode(n.Steps[i], true, sport); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrStepTypeInvalid
	}
}

func validateSingleStep(n Step, sport string) error {
	switch n.Intent {
	case IntentWarmup, IntentActive, IntentInterval, IntentRecovery, IntentRest, IntentCooldown:
	default:
		return ErrIntentInvalid
	}
	if err := validateDuration(n.Duration); err != nil {
		return err
	}
	if err := validateTarget(n.Target); err != nil {
		return err
	}
	if err := validateTargetSport(n.Target, sport); err != nil {
		return err
	}
	return validateSecondaryTarget(n.Target, n.SecondaryTarget, sport)
}

// validateSecondaryTarget enforces the bike-only second-target rules: a
// secondary target is accepted only on bike steps, its kind SHALL NOT be none,
// it SHALL be a valid target in a metric family different from the primary, and
// it SHALL satisfy the same sport rules as any target. A nil secondary is a
// no-op. Applies to top-level and repeat-group child steps alike.
func validateSecondaryTarget(primary, secondary *Target, sport string) error {
	if secondary == nil {
		return nil
	}
	if sport != SportBike {
		return ErrSecondaryTarget
	}
	if secondary.Kind == TargetNone {
		return ErrSecondaryTarget
	}
	if err := validateTarget(secondary); err != nil {
		return err
	}
	if err := validateTargetSport(secondary, sport); err != nil {
		return err
	}
	if primary != nil && metricFamily(primary.Kind) == metricFamily(secondary.Kind) {
		return ErrSecondaryTarget
	}
	return nil
}

// metricFamily groups target kinds that measure the same thing, so a primary and
// secondary target can be required to gate on different metrics (e.g. power +
// cadence, never power + power).
func metricFamily(kind string) string {
	switch kind {
	case TargetPowerZone, TargetPowerW:
		return "power"
	case TargetHRZone, TargetHRBpm:
		return "hr"
	case TargetPace, TargetSwimPace:
		return "pace"
	case TargetCadence:
		return "cadence"
	case TargetRPE:
		return "rpe"
	default:
		return kind
	}
}

// validateTargetSport enforces sport-dependent target rules against the
// workout's sport: swim_pace (sec/100m) is swim-only, pace (sec/km) is rejected
// on swim, and cadence (rpm/spm) is accepted only on bike or run. Other kinds
// are sport-agnostic. A nil target is a no-op (validateTarget already rejected
// it where required).
func validateTargetSport(t *Target, sport string) error {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case TargetSwimPace:
		if sport != SportSwim {
			return ErrTargetSportMismatch
		}
	case TargetPace:
		if sport == SportSwim {
			return ErrTargetSportMismatch
		}
	case TargetCadence:
		if sport != SportBike && sport != SportRun {
			return ErrTargetSportMismatch
		}
	}
	return nil
}

// ValidIntent reports whether s is a recognized step intent. Exported so other
// capabilities (e.g. training-plan slot overrides) can validate against the
// same intent vocabulary.
func ValidIntent(s string) bool {
	switch s {
	case IntentWarmup, IntentActive, IntentInterval, IntentRecovery, IntentRest, IntentCooldown:
		return true
	}
	return false
}

// ValidateSteps validates an ordered, non-empty step program under the given
// sport, applying the same rules as a template's own steps (intent, duration,
// per-sport target rules, secondary-target pairing, single-level repeats).
// Exported for reuse by the multisport capability, which validates each
// segment's steps under that segment's sport. Returns ErrSportInvalid for an
// unknown sport and ErrStepsEmpty for an empty program.
func ValidateSteps(steps []Step, sport string) error {
	if !validSport(sport) {
		return ErrSportInvalid
	}
	if len(steps) == 0 {
		return ErrStepsEmpty
	}
	for i := range steps {
		if err := validateNode(steps[i], false, sport); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTarget validates a single effort target using the same rules applied
// to template steps. Exported for reuse by training-plan slot target overrides.
func ValidateTarget(t *Target) error { return validateTarget(t) }

// ValidateDuration validates a single duration using the same rules applied to
// template steps. Exported for reuse by training-plan slot duration overrides.
func ValidateDuration(d *Duration) error { return validateDuration(d) }

func validateDuration(d *Duration) error {
	if d == nil {
		return ErrDurationInvalid
	}
	switch d.Kind {
	case DurationTime:
		if d.Seconds == nil || *d.Seconds <= 0 {
			return ErrDurationInvalid
		}
	case DurationDistance:
		if d.Meters == nil || *d.Meters <= 0 {
			return ErrDurationInvalid
		}
	case DurationLapButton, DurationOpen:
		// no quantity
	default:
		return ErrDurationInvalid
	}
	return nil
}

func validateTarget(t *Target) error {
	if t == nil {
		return ErrTargetInvalid
	}
	switch t.Kind {
	case TargetNone:
		return nil
	case TargetHRZone, TargetPowerZone:
		return validateRange(t.Low, t.High, 1, 5)
	case TargetHRBpm, TargetPowerW, TargetCadence, TargetRPE:
		return validatePositiveRange(t.Low, t.High)
	case TargetPace:
		return validatePositiveRange(t.LowSecPerKM, t.HighSecPerKM)
	case TargetSwimPace:
		return validatePositiveRange(t.LowSecPer100m, t.HighSecPer100m)
	default:
		return ErrTargetInvalid
	}
}

// validateRange enforces both bounds (when present) within [lo, hi] and low <= high.
func validateRange(low, high *int, lo, hi int) error {
	for _, v := range []*int{low, high} {
		if v != nil && (*v < lo || *v > hi) {
			return ErrTargetRangeInvalid
		}
	}
	if low != nil && high != nil && *low > *high {
		return ErrTargetRangeInvalid
	}
	return nil
}

// validatePositiveRange enforces positive bounds (when present) and low <= high.
func validatePositiveRange(low, high *int) error {
	for _, v := range []*int{low, high} {
		if v != nil && *v <= 0 {
			return ErrTargetRangeInvalid
		}
	}
	if low != nil && high != nil && *low > *high {
		return ErrTargetRangeInvalid
	}
	return nil
}

// SumTimedDurationSec totals the seconds of every time-based step in a program,
// recursing one level into repeat groups (each child's seconds counted Count
// times). Non-time durations (distance/lap_button/open) contribute nothing —
// they have no wall-clock length. Returns 0 for an all-untimed program; callers
// that need a concrete session length apply their own fallback.
func SumTimedDurationSec(steps []Step) int {
	total := 0
	for _, st := range steps {
		if st.Type == NodeRepeat {
			total += max(st.Count, 1) * SumTimedDurationSec(st.Steps)
			continue
		}
		if st.Duration != nil && st.Duration.Kind == DurationTime && st.Duration.Seconds != nil {
			total += *st.Duration.Seconds
		}
	}
	return total
}
