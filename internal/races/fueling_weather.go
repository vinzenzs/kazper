package races

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/heat"
)

// HeatProvider resolves a race's location text + window into a heat picture.
// Narrow on purpose: this package needs the answer, not the weather stack.
type HeatProvider interface {
	RaceHeatFor(ctx context.Context, locationText string, from, to time.Time) *heat.RaceHeat
}

// HeatScaling is the weather-mode block on the fueling plan: what the heat is,
// and the bounded multiplier it applied to fluid and sodium.
//
// Carbs are deliberately absent — heat drives sweat loss, not carbohydrate
// oxidation, and scaling carbs by the weather would be inventing physiology.
type HeatScaling struct {
	LoadC           float64          `json:"load_c"`
	HeatIndexC      float64          `json:"heat_index_c"`
	Conditions      *heat.Conditions `json:"conditions,omitempty"`
	Acclimatization string           `json:"acclimatization,omitempty"`
	Location        string           `json:"location,omitempty"`
	ForecastAt      *time.Time       `json:"forecast_at,omitempty"`
	// FluidMultiplier is echoed so the scaled numbers can be taken back apart.
	FluidMultiplier float64 `json:"fluid_multiplier"`
}

// resolveHeatScaling turns the race into a scaling block, or a reason. Returns
// (nil, nil) when weather mode is off or unwired — the plan then computes
// exactly as it always has.
func (s *Service) resolveHeatScaling(ctx context.Context, race *Race, withWeather bool) (*HeatScaling, *string) {
	if !withWeather || s.heat == nil {
		return nil, nil
	}
	location := ""
	if race.Location != nil {
		location = *race.Location
	}

	from, to := raceWindow(race)
	rh := s.heat.RaceHeatFor(ctx, location, from, to)
	if rh.Reason != nil {
		return nil, rh.Reason
	}

	out := &HeatScaling{
		LoadC:           rh.LoadC,
		HeatIndexC:      rh.HeatIndexC,
		Conditions:      rh.Conditions,
		Location:        rh.Location,
		ForecastAt:      rh.ForecastAt,
		FluidMultiplier: heat.FluidMultiplier(rh.LoadC),
	}
	if rh.Acclimatization != nil {
		out.Acclimatization = string(rh.Acclimatization.Level)
	}
	return out, nil
}

// raceWindow derives the forecast window from the race date and its legs'
// expected durations. Races carry no start time, so the window opens at a
// conventional 08:00 UTC — the honest alternative to inventing a precise one.
func raceWindow(race *Race) (time.Time, time.Time) {
	d, err := time.Parse("2006-01-02", race.RaceDate)
	if err != nil {
		return time.Time{}, time.Time{}
	}
	start := time.Date(d.Year(), d.Month(), d.Day(), 8, 0, 0, 0, time.UTC)

	total := 0
	for _, leg := range race.Legs {
		if leg.ExpectedDurationMin != nil {
			total += *leg.ExpectedDurationMin
		}
	}
	dur := 3 * time.Hour
	if total > 0 {
		dur = time.Duration(total) * time.Minute
	}
	return start, start.Add(dur)
}
