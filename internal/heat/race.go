package heat

import (
	"context"
	"strings"
	"time"

	"github.com/vinzenzs/kazper/internal/weather"
)

// forecastHorizonDays bounds how far ahead Open-Meteo's forecast reaches. A
// race beyond it gets an honest reason rather than a fabricated hot day.
const forecastHorizonDays = 16

// Race-heat degradation reasons. Each keeps the caller's base plan intact.
const (
	ReasonLocationUngeocodable = "location_ungeocodable"
	ReasonForecastOutOfRange   = "forecast_out_of_range"
)

// RaceHeat is the race-day heat picture behind the pacing/fueling weather
// modes. Reason is set exactly when the picture couldn't be computed — the
// caller then returns its unadjusted plan and says why.
type RaceHeat struct {
	LoadC           float64         `json:"load_c"`
	HeatIndexC      float64         `json:"heat_index_c"`
	Conditions      *Conditions     `json:"conditions,omitempty"`
	Acclimatization *AcclimEvidence `json:"acclimatization,omitempty"`
	Location        string          `json:"location,omitempty"`
	ForecastAt      *time.Time      `json:"forecast_at,omitempty"`
	Reason          *string         `json:"reason,omitempty"`
}

// Geocoder resolves the race's free-text location. Kept narrow so the heat
// service depends on the operation, not the vendor.
type Geocoder interface {
	Geocode(ctx context.Context, place string) ([]weather.Place, bool)
}

// SetGeocoder enables race-location resolution.
func (s *Service) SetGeocoder(g Geocoder) { s.geocoder = g }

// RaceHeatFor resolves a race's location text and window into a heat picture.
//
// Nothing is stored: the race's authored location text stays the truth, and the
// coordinates are re-resolved per read (cached in the client). A race two weeks
// out simply has no reliable forecast, which is why weather mode is opt-in and
// this returns a reason rather than a number when it can't know.
func (s *Service) RaceHeatFor(ctx context.Context, locationText string, from, to time.Time) *RaceHeat {
	out := &RaceHeat{}

	place := strings.TrimSpace(locationText)
	if place == "" {
		reason := ReasonLocationUngeocodable
		out.Reason = &reason
		return out
	}
	// Beyond the forecast horizon there is nothing to fetch. Checked before the
	// geocode so a distant race costs no lookups at all.
	if s.now().AddDate(0, 0, forecastHorizonDays).Before(from) {
		reason := ReasonForecastOutOfRange
		out.Reason = &reason
		return out
	}
	if s.geocoder == nil {
		reason := ReasonWeatherUnavailable
		out.Reason = &reason
		return out
	}

	matches, ok := s.geocoder.Geocode(ctx, place)
	if !ok {
		reason := ReasonWeatherUnavailable
		out.Reason = &reason
		return out
	}
	if len(matches) == 0 {
		// The lookup ran and found nothing: the athlete's location text is the
		// thing to fix ("local crit" geocodes nowhere), so name that.
		reason := ReasonLocationUngeocodable
		out.Reason = &reason
		return out
	}
	match := matches[0]
	out.Location = match.Name
	if match.Country != "" {
		out.Location = match.Name + ", " + match.Country
	}

	hours, ok := s.weather.Forecast(ctx, match.Lat, match.Lon, weather.Window{From: from, To: to})
	if !ok {
		reason := ReasonWeatherUnavailable
		out.Reason = &reason
		return out
	}
	mean, ok := weather.MeanOver(hours, from, to)
	if !ok {
		reason := ReasonForecastOutOfRange
		out.Reason = &reason
		return out
	}

	load := ComputeLoad(Conditions{
		TemperatureC: mean.TemperatureC,
		HumidityPct:  mean.HumidityPct,
		WindSpeedMPS: mean.WindSpeedMPS,
		CloudCovPct:  mean.CloudCovPct,
	})
	acclim := s.acclimatization(ctx, s.now().UTC())

	forecastAt := s.now().UTC()
	out.LoadC = load.HeatLoadC
	out.HeatIndexC = load.HeatIndexC
	out.Conditions = &Conditions{
		TemperatureC: roundTo1(mean.TemperatureC),
		HumidityPct:  roundTo1(mean.HumidityPct),
		WindSpeedMPS: roundTo1(mean.WindSpeedMPS),
		CloudCovPct:  roundTo1(mean.CloudCovPct),
	}
	out.Acclimatization = &acclim
	out.ForecastAt = &forecastAt
	return out
}

// FluidMultiplier maps a heat load onto the bounded fluid/sodium scaler the
// race fueling plan applies. Same shape and bounds as the training-day fluid
// note — heat raises sweat loss, but an unbounded multiplier would prescribe
// undrinkable volumes.
func FluidMultiplier(loadC float64) float64 {
	if loadC <= 24 {
		return 1.0
	}
	m := 1 + (loadC-24)*fluidPerLoadCAbove24
	if m > maxHeatFluidMultiplier {
		m = maxHeatFluidMultiplier
	}
	return roundTo2(m)
}
