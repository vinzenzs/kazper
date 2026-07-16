package workoutfueling

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Sweat-rate sentinel errors — mapped 1:1 to API error codes by the handler.
var (
	ErrWorkoutNotCompleted  = errors.New("workout not completed")
	ErrPreWeightInvalid     = errors.New("pre_weight_kg must be a positive number")
	ErrPostWeightInvalid    = errors.New("post_weight_kg must be a positive number")
	ErrFluidOverrideInvalid = errors.New("fluid_ml_override must be non-negative")
)

// implausibleWarning is emitted (values still returned) when the result is out
// of the physiological band — a negative loss (net weight gain) or a rate above
// maxPlausibleRateMlPerHr. See design D4: warn, don't refuse.
const (
	implausibleWarning      = "implausible_result"
	maxPlausibleRateMlPerHr = 5000.0
)

// SweatRateInput carries the caller-supplied field-test parameters. Weights are
// explicit (the bodyweight log is daily-grained, not pre/post-session); the
// override replaces the derived fluid sum when non-nil (design D1/D2).
type SweatRateInput struct {
	PreWeightKg     float64
	PostWeightKg    float64
	FluidMlOverride *float64
}

// SweatRateFluid itemizes the fluid intake so a surprising rate is auditable.
// HydrationMl + WorkoutFuelMl are always the derived (workout-linked) sums; when
// an override is supplied it is echoed and TotalMl reflects the override — the
// derived pair stays visible as the value it replaced (design D2).
type SweatRateFluid struct {
	HydrationMl     float64  `json:"hydration_ml"`
	WorkoutFuelMl   float64  `json:"workout_fuel_ml"`
	FluidMlOverride *float64 `json:"fluid_ml_override,omitempty"`
	TotalMl         float64  `json:"total_ml"`
}

// SweatRate is the response shape for GET /workouts/{id}/sweat-rate. Unit-
// isolated: ml / ml-per-hr / kg only — no kcal, no sodium (design D5).
type SweatRate struct {
	WorkoutID        uuid.UUID      `json:"workout_id"`
	DurationHr       float64        `json:"duration_hr"`
	PreWeightKg      float64        `json:"pre_weight_kg"`
	PostWeightKg     float64        `json:"post_weight_kg"`
	Fluid            SweatRateFluid `json:"fluid"`
	SweatLossMl      float64        `json:"sweat_loss_ml"`
	SweatRateMlPerHr float64        `json:"sweat_rate_ml_per_hr"`
	Warning          *string        `json:"warning,omitempty"`
}

// SweatRateFor computes the standard sweat-rate field test over a completed
// workout: loss_ml = (pre − post) × 1000 + fluid_ml, rate over elapsed hours.
// Persists nothing; feeds no daily total.
func (s *Service) SweatRateFor(ctx context.Context, id uuid.UUID, in SweatRateInput) (*SweatRate, error) {
	if in.PreWeightKg <= 0 {
		return nil, ErrPreWeightInvalid
	}
	if in.PostWeightKg <= 0 {
		return nil, ErrPostWeightInvalid
	}
	if in.FluidMlOverride != nil && *in.FluidMlOverride < 0 {
		return nil, ErrFluidOverrideInvalid
	}

	w, err := s.workouts.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w.Status != workouts.StatusCompleted {
		return nil, ErrWorkoutNotCompleted
	}

	// Derived fluid = workout-linked hydration ml + workout-fuel quantity_ml.
	hyd, err := s.hydration.ListByWorkout(ctx, w.ID)
	if err != nil {
		return nil, fmt.Errorf("list linked hydration for sweat rate: %w", err)
	}
	fuel, err := s.workoutFuel.ListByWorkout(ctx, w.ID)
	if err != nil {
		return nil, fmt.Errorf("list linked workout-fuel for sweat rate: %w", err)
	}

	var hydMl float64
	for _, h := range hyd {
		hydMl += h.QuantityMl
	}
	var fuelMl float64
	for _, f := range fuel {
		if f.QuantityMl != nil {
			fuelMl += *f.QuantityMl
		}
	}

	fluidMl := hydMl + fuelMl
	if in.FluidMlOverride != nil {
		fluidMl = *in.FluidMlOverride
	}

	lossMl := (in.PreWeightKg-in.PostWeightKg)*1000 + fluidMl

	durationHr := w.EndedAt.Sub(w.StartedAt).Hours()
	var rate float64
	if durationHr > 0 {
		rate = lossMl / durationHr
	}

	out := &SweatRate{
		WorkoutID:    w.ID,
		DurationHr:   numfmt.Round1(durationHr),
		PreWeightKg:  numfmt.Round1(in.PreWeightKg),
		PostWeightKg: numfmt.Round1(in.PostWeightKg),
		Fluid: SweatRateFluid{
			HydrationMl:     numfmt.Round1(hydMl),
			WorkoutFuelMl:   numfmt.Round1(fuelMl),
			FluidMlOverride: numfmt.Round1Ptr(in.FluidMlOverride),
			TotalMl:         numfmt.Round1(fluidMl),
		},
		SweatLossMl:      numfmt.Round1(lossMl),
		SweatRateMlPerHr: numfmt.Round1(rate),
	}
	if lossMl < 0 || rate > maxPlausibleRateMlPerHr || durationHr <= 0 {
		wstr := implausibleWarning
		out.Warning = &wstr
	}
	return out, nil
}
