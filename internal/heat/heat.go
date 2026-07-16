// Package heat turns a planned session's forecast into a coaching suggestion:
// how hot it will effectively be, how acclimatized the athlete is, and how much
// to back off.
//
// It is openly a HEURISTIC, in the spirit of the practice calculators coaches
// already use — not WBGT, and not physiology. There is no solar sensor here;
// cloud cover is the v1 proxy. Every constant is documented, echoed in the
// response, and meant to be refined once the heat-analytics evidence exists.
//
// Strictly advisory: nothing in this package writes a target anywhere. The
// coach turns a suggestion into a workout change through the existing confirmed
// flows.
package heat

import (
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Acclimatization is how heat-adapted the athlete looks, derived from recent
// outdoor work rather than asked for in a dropdown.
type Acclimatization string

const (
	AcclimLow    Acclimatization = "low"
	AcclimMedium Acclimatization = "medium"
	AcclimGood   Acclimatization = "good"
)

// Acclimatization thresholds and the qualifying-session rule (design D3).
// Constants v1 — the count and the sessions behind it are echoed, so the level
// is auditable back to actual rides.
const (
	AcclimWindowDays  = 14
	acclimMinMinutes  = 60.0
	acclimMinHeatIdxC = 25.0

	acclimGoodSessions   = 5 // >= 5 → good
	acclimMediumSessions = 2 // 2..4 → medium; < 2 → low
)

// Heat-load composite constants (design D2). Wind cools; cloud removes part of
// the solar penalty. Both nudges are bounded so a breezy overcast day can never
// argue the heat away entirely.
const (
	// windCoolingStartMPS is where convective cooling starts to count.
	windCoolingStartMPS = 3.0
	// windCoolingPerMPS is the °C removed per m/s above the start, capped by
	// maxWindCoolingC.
	windCoolingPerMPS = 0.4
	maxWindCoolingC   = 3.0

	// solarPenaltyC is the °C added for full sun, scaled down linearly by cloud
	// cover. A crude stand-in for radiation (Open-Meteo serves it; using it is
	// the documented next step once constants get their evidence pass).
	solarPenaltyC = 3.0
)

// Duration bands for the adjustment table (design D4).
const (
	shortSessionMin = 45.0
	longSessionMin  = 150.0
)

// Sport-agnostic baseline kinds. The percentage is a reduction off whichever
// baseline anchors the session's targets.
const (
	BaselineFTP           = "ftp_watts"
	BaselineThresholdPace = "threshold_pace_sec_per_km"
)

// Degradation reasons.
const (
	ReasonLocationUnconfigured = "location_unconfigured"
	ReasonWeatherUnavailable   = "weather_unavailable"
)

// HeatIndexC is the apparent temperature from dry-bulb temperature and relative
// humidity — the Rothfusz regression the US NWS uses, converted to °C.
//
// The regression is only meaningful in the heat; below ~26.7 °C (80 °F) it
// diverges, so the simple Steadman average is used there instead — which is
// also why a cool day's heat index reads essentially as its temperature.
func HeatIndexC(tempC, humidityPct float64) float64 {
	t := tempC*9/5 + 32 // the regression is defined in °F
	rh := humidityPct

	// Steadman's simple form, valid at moderate temperatures.
	simple := 0.5 * (t + 61.0 + ((t - 68.0) * 1.2) + (rh * 0.094))
	if (simple+t)/2 < 80.0 {
		return (simple - 32) * 5 / 9
	}

	hi := -42.379 +
		2.04901523*t +
		10.14333127*rh -
		0.22475541*t*rh -
		0.00683783*t*t -
		0.05481717*rh*rh +
		0.00122874*t*t*rh +
		0.00085282*t*rh*rh -
		0.00000199*t*t*rh*rh

	// The NWS adjustments at the regression's edges.
	switch {
	case rh < 13 && t >= 80 && t <= 112:
		hi -= ((13 - rh) / 4) * math.Sqrt((17-math.Abs(t-95))/17)
	case rh > 85 && t >= 80 && t <= 87:
		hi += ((rh - 85) / 10) * ((87 - t) / 5)
	}
	return (hi - 32) * 5 / 9
}

// Conditions is the session window's mean weather.
type Conditions struct {
	TemperatureC float64 `json:"temperature_c"`
	HumidityPct  float64 `json:"humidity_pct"`
	WindSpeedMPS float64 `json:"wind_speed_mps"`
	CloudCovPct  float64 `json:"cloud_cover_pct"`
}

// Load is the composite heat picture, with the parts that produced it.
type Load struct {
	HeatLoadC     float64 `json:"heat_load_c"`
	HeatIndexC    float64 `json:"heat_index_c"`
	WindCoolingC  float64 `json:"wind_cooling_c"`
	SolarPenaltyC float64 `json:"solar_penalty_c"`
}

// ComputeLoad folds the mean conditions into a °C-equivalent load:
//
//	heat_load = heat_index + solar_penalty×(1 − cloud) − wind_cooling
//
// Every term is reported so a surprising number can be taken apart rather than
// argued with.
func ComputeLoad(c Conditions) Load {
	hi := HeatIndexC(c.TemperatureC, c.HumidityPct)

	cooling := 0.0
	if c.WindSpeedMPS > windCoolingStartMPS {
		cooling = math.Min((c.WindSpeedMPS-windCoolingStartMPS)*windCoolingPerMPS, maxWindCoolingC)
	}

	cloud := math.Max(0, math.Min(100, c.CloudCovPct))
	solar := solarPenaltyC * (1 - cloud/100)

	return Load{
		HeatLoadC:     numfmt.Round1(hi + solar - cooling),
		HeatIndexC:    numfmt.Round1(hi),
		WindCoolingC:  numfmt.Round1(cooling),
		SolarPenaltyC: numfmt.Round1(solar),
	}
}

// QualifyingSession is one ride/run that counts toward acclimatization —
// echoed so the level traces back to real work.
type QualifyingSession struct {
	WorkoutID  uuid.UUID `json:"workout_id"`
	Date       string    `json:"date"`
	HeatIndexC float64   `json:"heat_index_c"`
	DurationM  float64   `json:"duration_min"`
}

// AcclimEvidence is the acclimatization level and what produced it.
type AcclimEvidence struct {
	Level      Acclimatization     `json:"level"`
	Count      int                 `json:"qualifying_sessions"`
	WindowDays int                 `json:"window_days"`
	Sessions   []QualifyingSession `json:"sessions"`
}

// CandidateSession is a completed workout considered for acclimatization.
// TemperatureC/HumidityPct are the stored per-workout weather; a session
// missing temperature can't qualify (we don't know how hot it was).
type CandidateSession struct {
	WorkoutID    uuid.UUID
	Date         string
	DurationMin  float64
	TemperatureC *float64
	HumidityPct  *float64
}

// ComputeAcclimatization counts sessions that were long enough, hot enough and
// outdoors within the trailing window, then bands the count.
//
// Callers pass only outdoor-or-unknown completed sessions — the indoor filter
// belongs to the query, since an indoor session's stored temperature says
// nothing about heat adaptation.
func ComputeAcclimatization(candidates []CandidateSession) AcclimEvidence {
	out := AcclimEvidence{WindowDays: AcclimWindowDays, Sessions: []QualifyingSession{}}

	for _, c := range candidates {
		if c.TemperatureC == nil || c.DurationMin < acclimMinMinutes {
			continue
		}
		// A missing humidity reading falls back to a neutral 50% rather than
		// dropping the session: the temperature is the dominant term, and
		// discarding a genuinely hot ride over a missing humidity would
		// under-report adaptation.
		humidity := 50.0
		if c.HumidityPct != nil {
			humidity = *c.HumidityPct
		}
		hi := HeatIndexC(*c.TemperatureC, humidity)
		if hi < acclimMinHeatIdxC {
			continue
		}
		out.Sessions = append(out.Sessions, QualifyingSession{
			WorkoutID:  c.WorkoutID,
			Date:       c.Date,
			HeatIndexC: numfmt.Round1(hi),
			DurationM:  numfmt.Round1(c.DurationMin),
		})
	}

	out.Count = len(out.Sessions)
	switch {
	case out.Count >= acclimGoodSessions:
		out.Level = AcclimGood
	case out.Count >= acclimMediumSessions:
		out.Level = AcclimMedium
	default:
		out.Level = AcclimLow
	}
	return out
}

// Adjustment is the suggested reduction off the effective baseline.
type Adjustment struct {
	ReductionPct float64 `json:"reduction_pct"`
	Baseline     string  `json:"baseline,omitempty"`
	BaselineFrom float64 `json:"baseline_value,omitempty"`
	SuggestedTo  float64 `json:"suggested_value,omitempty"`
}

// baseReductionPct is the heat-load axis of the table (design D4): the
// reduction for a moderate-length session at medium acclimatization.
//
//	< 24 °C  → 0 %   (no meaningful heat cost)
//	24–28    → 2 %
//	28–32    → 5 %
//	32–36    → 9 %
//	>= 36    → 14 %
func baseReductionPct(loadC float64) float64 {
	switch {
	case loadC < 24:
		return 0
	case loadC < 28:
		return 2
	case loadC < 32:
		return 5
	case loadC < 36:
		return 9
	default:
		return 14
	}
}

// durationFactor scales the reduction by exposure: a short session barely
// accumulates heat strain, a long one compounds it.
func durationFactor(durationMin float64) float64 {
	switch {
	case durationMin < shortSessionMin:
		return 0.5
	case durationMin <= longSessionMin:
		return 1.0
	default:
		return 1.4
	}
}

// acclimFactor: adaptation shaves the penalty, its absence deepens it.
func acclimFactor(a Acclimatization) float64 {
	switch a {
	case AcclimGood:
		return 0.6
	case AcclimMedium:
		return 1.0
	default:
		return 1.3
	}
}

// ComputeReductionPct combines the three axes. Rounded to 1dp at the boundary;
// a zero base (a cool day) stays zero regardless of the other factors — no
// amount of duration makes 20 °C hot.
func ComputeReductionPct(loadC, durationMin float64, a Acclimatization) float64 {
	base := baseReductionPct(loadC)
	if base == 0 {
		return 0
	}
	return numfmt.Round1(base * durationFactor(durationMin) * acclimFactor(a))
}

// FluidNote is the hydration guidance beside the adjustment.
type FluidNote struct {
	MlPerHour float64 `json:"ml_per_hour"`
	// Source states where the number came from — a flagged default is honest, a
	// silent one would not be. See the Source* constants.
	Source string `json:"source"`
	Note   string `json:"note,omitempty"`
}

// Fluid-note sources, in descending order of trust.
const (
	// SourceGarminSweatLoss: derived from the device's own estimated sweat loss
	// on recent comparable sessions. Deliberately NOT called "measured": the
	// sweat-rate capability's field test requires explicit pre/post weights by
	// design (inferring them would be "a guess dressed as data"), and a device
	// estimate must not pass as one.
	SourceGarminSweatLoss = "garmin_sweat_loss_estimate"
	// SourceGenericDefault: no personal signal at all.
	SourceGenericDefault = "generic_default"
)

// SweatSignal is a personalized sweat rate and where it came from.
type SweatSignal struct {
	MlPerHour float64
	Source    string
	// Sessions is how many sessions backed the estimate — echoed so a number
	// resting on one ride is visibly thin.
	Sessions int
}

// Fluid guidance constants. The generic baseline is the same 600 ml/hr the race
// fueling plan falls back to, kept consistent on purpose.
const (
	genericSweatRateMlPerHr = 600.0
	// maxHeatFluidMultiplier bounds the heat scaling: heat raises sweat loss,
	// but a multiplier without a ceiling would prescribe undrinkable volumes.
	maxHeatFluidMultiplier = 1.5
	fluidPerLoadCAbove24   = 0.03 // +3% per °C of load above 24
)

// ComputeFluid scales a personal sweat rate by the heat load. signal is nil
// when nothing personal is derivable, in which case the generic default is used
// AND flagged as such.
func ComputeFluid(loadC float64, signal *SweatSignal) FluidNote {
	base := genericSweatRateMlPerHr
	source := SourceGenericDefault
	note := "No personal sweat data available — this is a generic starting point, not a personal number. A sweat-rate field test on a hot session gives a real one."
	if signal != nil && signal.MlPerHour > 0 {
		base = signal.MlPerHour
		source = signal.Source
		note = fmt.Sprintf("Scaled from the athlete's own sweat data across %d recent session(s). This is a device estimate, not a field test.", signal.Sessions)
	}

	mult := 1.0
	if loadC > 24 {
		mult = math.Min(1+(loadC-24)*fluidPerLoadCAbove24, maxHeatFluidMultiplier)
	}
	return FluidNote{
		MlPerHour: numfmt.Round1(base * mult),
		Source:    source,
		Note:      note,
	}
}
