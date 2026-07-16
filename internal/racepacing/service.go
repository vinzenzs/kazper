package racepacing

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/races"
)

// Sentinel errors mapping 1:1 to API codes. race_not_found reuses
// races.ErrNotFound; override_not_found reuses the repo's ErrOverrideNotFound.
var (
	ErrLegNotFound                = errors.New("leg not found")                // leg_not_found
	ErrOverrideDisciplineMismatch = errors.New("override discipline mismatch") // override_discipline_mismatch
	ErrOverrideTargetRequired     = errors.New("override target required")     // override_target_required
	ErrOverrideUnitConflict       = errors.New("override unit conflict")       // override_unit_conflict
	ErrOverrideBandInvalid        = errors.New("override band invalid")        // override_band_invalid
)

// RacesRepo reads a race with its legs (owned by race-fueling-plan).
type RacesRepo interface {
	GetRace(ctx context.Context, id uuid.UUID) (*races.Race, error)
}

// ConfigRepo reads the athlete-config singleton (nil,nil when unset).
type ConfigRepo interface {
	Get(ctx context.Context) (*athleteconfig.AthleteConfig, error)
}

// Service computes the pacing plan and manages per-leg overrides.
type Service struct {
	// heat is optional: unwired leaves weather mode inert rather than erroring.
	heat HeatProvider

	races     RacesRepo
	config    ConfigRepo
	overrides *Repo
}

func NewService(racesRepo RacesRepo, configRepo ConfigRepo, overridesRepo *Repo) *Service {
	return &Service{races: racesRepo, config: configRepo, overrides: overridesRepo}
}

// Plan computes the deterministic cool-weather plan. Kept as-is so every
// existing caller is untouched; PlanWithWeather is the opt-in superset.
func (s *Service) Plan(ctx context.Context, raceID uuid.UUID) (*PacingPlan, error) {
	return s.PlanWithWeather(ctx, raceID, false)
}

// PlanWithWeather loads race + legs, thresholds, and overrides and computes the
// plan; with withWeather it additionally annotates heat. The races repo's
// ErrNotFound propagates (→ race_not_found).
func (s *Service) PlanWithWeather(ctx context.Context, raceID uuid.UUID, withWeather bool) (*PacingPlan, error) {
	race, err := s.races.GetRace(ctx, raceID)
	if err != nil {
		return nil, err
	}
	cfg, err := s.config.Get(ctx)
	if err != nil {
		return nil, err
	}
	ovs, err := s.overrides.GetOverridesForRace(ctx, raceID)
	if err != nil {
		return nil, err
	}
	plan := compute(race, cfg, ovs)

	// Weather is strictly additive and strictly opt-in: the base plan above is
	// already complete and is never modified below.
	if withWeather && s.heat != nil {
		location := ""
		if race.Location != nil {
			location = *race.Location
		}
		s.applyHeat(ctx, &plan, location, race.RaceDate)
	}
	return &plan, nil
}

// SetOverride validates and full-replaces the override for (race, ordinal).
func (s *Service) SetOverride(ctx context.Context, raceID uuid.UUID, ordinal int, in OverrideInput) error {
	race, err := s.races.GetRace(ctx, raceID)
	if err != nil {
		return err
	}
	leg := findLeg(race, ordinal)
	if leg == nil {
		return ErrLegNotFound
	}
	fam, err := validateFamily(in)
	if err != nil {
		return err
	}
	if !familyMatchesDiscipline(fam, leg.Discipline) {
		return ErrOverrideDisciplineMismatch
	}

	o := &Override{RaceID: raceID, Ordinal: ordinal, Note: in.Note}
	switch fam {
	case familyPower:
		o.TargetPowerLowW, o.TargetPowerHighW = in.TargetPowerLowW, in.TargetPowerHighW
	case familyPaceKM:
		o.TargetPaceLowSecPerKM, o.TargetPaceHighSecPerKM = in.TargetPaceLowSecPerKM, in.TargetPaceHighSecPerKM
	case familyPace100m:
		o.TargetPaceLowSecPer100m, o.TargetPaceHighSecPer100m = in.TargetPaceLowSecPer100m, in.TargetPaceHighSecPer100m
	}
	return s.overrides.UpsertOverride(ctx, o)
}

// DeleteOverride removes the override; race_not_found when the race is unknown,
// ErrOverrideNotFound (→ override_not_found) when there was nothing to remove.
func (s *Service) DeleteOverride(ctx context.Context, raceID uuid.UUID, ordinal int) error {
	if _, err := s.races.GetRace(ctx, raceID); err != nil {
		return err
	}
	return s.overrides.DeleteOverride(ctx, raceID, ordinal)
}

func findLeg(race *races.Race, ordinal int) *races.RaceLeg {
	for _, l := range race.Legs {
		if l.Ordinal == ordinal {
			return l
		}
	}
	return nil
}

// validateFamily returns the single populated unit family, or a sentinel: zero
// families → override_target_required, more than one → override_unit_conflict, a
// malformed band (missing pair member, non-positive, non-finite, low>high) →
// override_band_invalid.
func validateFamily(in OverrideInput) (targetFamily, error) {
	powerAny := in.TargetPowerLowW != nil || in.TargetPowerHighW != nil
	kmAny := in.TargetPaceLowSecPerKM != nil || in.TargetPaceHighSecPerKM != nil
	m100Any := in.TargetPaceLowSecPer100m != nil || in.TargetPaceHighSecPer100m != nil

	present := 0
	var fam targetFamily
	if powerAny {
		present++
		fam = familyPower
	}
	if kmAny {
		present++
		fam = familyPaceKM
	}
	if m100Any {
		present++
		fam = familyPace100m
	}
	if present == 0 {
		return familyNone, ErrOverrideTargetRequired
	}
	if present > 1 {
		return familyNone, ErrOverrideUnitConflict
	}

	switch fam {
	case familyPower:
		if in.TargetPowerLowW == nil || in.TargetPowerHighW == nil ||
			*in.TargetPowerLowW <= 0 || *in.TargetPowerHighW <= 0 || *in.TargetPowerLowW > *in.TargetPowerHighW {
			return familyNone, ErrOverrideBandInvalid
		}
	case familyPaceKM:
		if !validPaceBand(in.TargetPaceLowSecPerKM, in.TargetPaceHighSecPerKM) {
			return familyNone, ErrOverrideBandInvalid
		}
	case familyPace100m:
		if !validPaceBand(in.TargetPaceLowSecPer100m, in.TargetPaceHighSecPer100m) {
			return familyNone, ErrOverrideBandInvalid
		}
	}
	return fam, nil
}

func validPaceBand(low, high *float64) bool {
	if low == nil || high == nil {
		return false
	}
	l, h := *low, *high
	if math.IsNaN(l) || math.IsNaN(h) || math.IsInf(l, 0) || math.IsInf(h, 0) {
		return false
	}
	return l > 0 && h > 0 && l <= h
}

func familyMatchesDiscipline(f targetFamily, d races.Discipline) bool {
	switch d {
	case races.DisciplineBike:
		return f == familyPower
	case races.DisciplineRun:
		return f == familyPaceKM
	case races.DisciplineSwim:
		return f == familyPace100m
	}
	return false // transition / other accept no override
}
