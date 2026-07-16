package racepacing

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/numfmt"
)

// HeatProvider resolves a race's location text + window into a heat picture.
// Narrow on purpose: this package needs the answer, not the weather stack.
type HeatProvider interface {
	RaceHeatFor(ctx context.Context, locationText string, from, to time.Time) *heat.RaceHeat
}

// SetHeatProvider enables `weather=true` on the pacing plan.
func (s *Service) SetHeatProvider(p HeatProvider) { s.heat = p }

// HeatBlock is the race-level heat picture attached in weather mode.
type HeatBlock struct {
	LoadC           float64          `json:"load_c"`
	HeatIndexC      float64          `json:"heat_index_c"`
	Conditions      *heat.Conditions `json:"conditions,omitempty"`
	Acclimatization string           `json:"acclimatization,omitempty"`
	Location        string           `json:"location,omitempty"`
	ForecastAt      *time.Time       `json:"forecast_at,omitempty"`
	ReductionPct    float64          `json:"reduction_pct"`
}

// HeatAdjustedLeg is the heat-adjusted sibling of a leg's original band. The
// original is never touched: the point is to hold "plan A cool / plan B hot"
// side by side, not to overwrite the deterministic plan with a forecast.
type HeatAdjustedLeg struct {
	TargetPowerLowW  *int `json:"target_power_low_w,omitempty"`
	TargetPowerHighW *int `json:"target_power_high_w,omitempty"`

	TargetPaceLowSecPerKM  *float64 `json:"target_pace_low_sec_per_km,omitempty"`
	TargetPaceHighSecPerKM *float64 `json:"target_pace_high_sec_per_km,omitempty"`

	TargetPaceLowSecPer100m  *float64 `json:"target_pace_low_sec_per_100m,omitempty"`
	TargetPaceHighSecPer100m *float64 `json:"target_pace_high_sec_per_100m,omitempty"`

	IntensityFactor *float64 `json:"intensity_factor,omitempty"`
	EstimatedTSS    *float64 `json:"estimated_tss,omitempty"`
	ReductionPct    float64  `json:"reduction_pct"`
}

// applyHeat annotates the plan in place. The base plan is already computed and
// is never modified — only `heat` and per-leg `heat_adjusted` are added, so a
// caller that ignores them sees exactly today's contract.
func (s *Service) applyHeat(ctx context.Context, plan *PacingPlan, locationText string, raceDate string) {
	from, to, ok := raceWindow(raceDate, plan)
	if !ok {
		reason := heat.ReasonForecastOutOfRange
		plan.HeatReason = &reason
		return
	}

	rh := s.heat.RaceHeatFor(ctx, locationText, from, to)
	if rh.Reason != nil {
		plan.HeatReason = rh.Reason
		return
	}

	level := heat.AcclimLow
	if rh.Acclimatization != nil {
		level = rh.Acclimatization.Level
	}

	block := &HeatBlock{
		LoadC:      rh.LoadC,
		HeatIndexC: rh.HeatIndexC,
		Conditions: rh.Conditions,
		Location:   rh.Location,
		ForecastAt: rh.ForecastAt,
	}
	if rh.Acclimatization != nil {
		block.Acclimatization = string(level)
	}

	for i := range plan.Legs {
		leg := &plan.Legs[i]
		// Each leg is adjusted on ITS own duration: a 40 km bike and a 10 km run
		// off the same race sit in different duration bands.
		durationMin := 0.0
		if leg.ExpectedDurationMin != nil {
			durationMin = float64(*leg.ExpectedDurationMin)
		}
		pct := heat.ComputeReductionPct(rh.LoadC, durationMin, level)
		adj := adjustLeg(leg, pct)
		if adj != nil {
			leg.HeatAdjusted = adj
		}
	}

	// The race-level percentage is the whole-event view: the total duration
	// band, not any one leg's.
	total := 0.0
	if plan.TotalDurationMin != nil {
		total = float64(*plan.TotalDurationMin)
	}
	block.ReductionPct = heat.ComputeReductionPct(rh.LoadC, total, level)
	plan.Heat = block
}

// adjustLeg derives the heat-adjusted band from a leg's original. Returns nil
// when the leg has no computable target (an unset threshold degraded it) —
// there is nothing to adjust, and inventing a band would be worse than silence.
func adjustLeg(leg *LegPacingPlan, pct float64) *HeatAdjustedLeg {
	factor := 1 - pct/100
	if factor <= 0 {
		return nil
	}
	out := &HeatAdjustedLeg{ReductionPct: pct}
	touched := false

	// Power scales DOWN with the reduction.
	if leg.TargetPowerLowW != nil {
		v := int(float64(*leg.TargetPowerLowW) * factor)
		out.TargetPowerLowW = &v
		touched = true
	}
	if leg.TargetPowerHighW != nil {
		v := int(float64(*leg.TargetPowerHighW) * factor)
		out.TargetPowerHighW = &v
		touched = true
	}
	// Pace is inverted: backing off means MORE seconds per km / per 100 m.
	if leg.TargetPaceLowSecPerKM != nil {
		v := numfmt.Round1(*leg.TargetPaceLowSecPerKM / factor)
		out.TargetPaceLowSecPerKM = &v
		touched = true
	}
	if leg.TargetPaceHighSecPerKM != nil {
		v := numfmt.Round1(*leg.TargetPaceHighSecPerKM / factor)
		out.TargetPaceHighSecPerKM = &v
		touched = true
	}
	if leg.TargetPaceLowSecPer100m != nil {
		v := numfmt.Round1(*leg.TargetPaceLowSecPer100m / factor)
		out.TargetPaceLowSecPer100m = &v
		touched = true
	}
	if leg.TargetPaceHighSecPer100m != nil {
		v := numfmt.Round1(*leg.TargetPaceHighSecPer100m / factor)
		out.TargetPaceHighSecPer100m = &v
		touched = true
	}
	if !touched {
		return nil
	}

	// IF and TSS follow the intensity down. Both are quadratic-ish in IF for
	// TSS (TSS = IF² × hours × 100), so the adjusted TSS uses the adjusted IF.
	if leg.IntensityFactor != nil {
		iff := numfmt.Round2(*leg.IntensityFactor * factor)
		out.IntensityFactor = &iff
		if leg.EstimatedTSS != nil && *leg.IntensityFactor > 0 {
			ratio := factor * factor
			tss := numfmt.Round1(*leg.EstimatedTSS * ratio)
			out.EstimatedTSS = &tss
		}
	}
	return out
}

// raceWindow turns the race's date + expected total duration into the forecast
// window. Races have no start time, so the window opens at a conventional
// 08:00 UTC — the honest alternative to inventing a precise one, and the
// forecast is an hourly mean over the event either way.
func raceWindow(raceDate string, plan *PacingPlan) (time.Time, time.Time, bool) {
	d, err := time.Parse("2006-01-02", raceDate)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	start := time.Date(d.Year(), d.Month(), d.Day(), 8, 0, 0, 0, time.UTC)
	dur := 3 * time.Hour // a sane default when no leg carries a duration
	if plan.TotalDurationMin != nil && *plan.TotalDurationMin > 0 {
		dur = time.Duration(*plan.TotalDurationMin) * time.Minute
	}
	return start, start.Add(dur), true
}
