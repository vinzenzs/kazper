package heat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/locations"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/weather"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// ErrWorkoutNotPlanned is returned for a non-planned workout: this is a
// pre-session question. History belongs to the heat-analytics read.
var ErrWorkoutNotPlanned = errors.New("workout is not planned")

// WorkoutsReader is the narrow read over the sessions this needs: the target
// workout, and the completed sessions behind acclimatization.
type WorkoutsReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*workouts.Workout, error)
	List(ctx context.Context, from, to time.Time, sessionGroup, status *string) ([]*workouts.Workout, error)
}

// LocationResolver is the location-periods primitive — the same one
// /locations/resolve exposes, so a surprising forecast is explainable.
type LocationResolver interface {
	LocationOn(ctx context.Context, date time.Time) (*locations.Resolved, error)
}

// WeatherClient is the guarded Open-Meteo client. Fail-open by contract: ok
// false degrades this read, never errors it.
type WeatherClient interface {
	Forecast(ctx context.Context, lat, lon float64, w weather.Window) ([]weather.Hour, bool)
}

// ConfigReader reads the effective athlete config for the baseline.
type ConfigReader interface {
	Get(ctx context.Context) (*athleteconfig.AthleteConfig, error)
}

// SweatRateProvider supplies a personal sweat rate when one is derivable.
// Optional: nil, or a nil result, means the generic default — which is then
// flagged as generic rather than passed off as personal.
type SweatRateProvider interface {
	LatestSweatSignal(ctx context.Context) *SweatSignal
}

// Service computes the heat read. Compute-on-read; writes nothing, anywhere.
type Service struct {
	workouts  WorkoutsReader
	locations LocationResolver
	weather   WeatherClient
	config    ConfigReader
	sweat     SweatRateProvider
	geocoder  Geocoder
	// now is injectable so the race-heat forecast-horizon gate is testable.
	now func() time.Time

	// userLoc + startHour/startMin resolve the scored window when a planned
	// workout carries no real start time. Defaults (UTC, 06:00) keep an unwired
	// service working; SetTrainingStart applies the configured values.
	userLoc   *time.Location
	startHour int
	startMin  int
}

// SetTrainingStart configures the athlete's timezone and habitual start hour —
// the fallback anchor for midnight-stored (date-only scheduled) sessions.
func (s *Service) SetTrainingStart(loc *time.Location, hour, min int) {
	if loc != nil {
		s.userLoc = loc
	}
	s.startHour, s.startMin = hour, min
}

func NewService(w WorkoutsReader, l LocationResolver, wx WeatherClient, c ConfigReader) *Service {
	return &Service{
		workouts: w, locations: l, weather: wx, config: c, now: time.Now,
		userLoc: time.UTC, startHour: defaultStartHour,
	}
}

// SetNow overrides the clock (tests only).
func (s *Service) SetNow(f func() time.Time) { s.now = f }

// SetSweatRateProvider enables the personalized fluid note.
func (s *Service) SetSweatRateProvider(p SweatRateProvider) { s.sweat = p }

// Location echoes where the forecast was taken.
type Location struct {
	Name   string  `json:"name"`
	Source string  `json:"source"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// Report is the response shape for GET /workouts/{id}/heat.
//
// NotApplicable and Reason are the two "no answer" shapes: an indoor session
// has no heat question, and a missing location/forecast means we can't answer
// one. Both are 200s — absence of an answer is a real answer.
type Report struct {
	WorkoutID uuid.UUID `json:"workout_id"`
	Date      string    `json:"date"`

	NotApplicable  bool `json:"not_applicable,omitempty"`
	AssumedOutdoor bool `json:"assumed_outdoor,omitempty"`

	// Window is the time range actually scored and StartSource names the rule
	// that produced it. AssumedStart carries the applied HH:MM when the hour was
	// guessed rather than stated — the time itself rather than a bool, since
	// StartSource already says THAT it was assumed; this says WHICH default
	// applied, so a wrong one is self-evident.
	Window       *ScoredWindow `json:"window,omitempty"`
	StartSource  StartSource   `json:"start_source,omitempty"`
	AssumedStart string        `json:"assumed_start,omitempty"`

	Location    *Location       `json:"location,omitempty"`
	Conditions  *Conditions     `json:"conditions,omitempty"`
	Load        *Load           `json:"load,omitempty"`
	Acclim      *AcclimEvidence `json:"acclimatization,omitempty"`
	Adjustment  *Adjustment     `json:"suggested_adjustment,omitempty"`
	Fluid       *FluidNote      `json:"fluid,omitempty"`
	DurationMin float64         `json:"duration_min,omitempty"`

	Reason *string `json:"reason,omitempty"`
}

// ScoredWindow is the local time range the forecast was averaged over.
type ScoredWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ReportFor computes the heat picture for a planned workout, anchoring the
// window by the default rules (no caller override).
func (s *Service) ReportFor(ctx context.Context, id uuid.UUID) (*Report, error) {
	return s.ReportForWithStart(ctx, id, "")
}

// ReportForWithStart is the same read with an optional `HH:MM` start override —
// the "what if I go out at 10:00 instead" question, answerable in one call.
func (s *Service) ReportForWithStart(ctx context.Context, id uuid.UUID, start string) (*Report, error) {
	w, err := s.workouts.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w.Status != workouts.StatusPlanned {
		return nil, ErrWorkoutNotPlanned
	}

	date := w.StartedAt.UTC()
	out := &Report{WorkoutID: w.ID, Date: date.Format("2006-01-02")}

	// An indoor session has no ambient weather to adjust for — and no forecast
	// is fetched, so this costs nothing.
	if w.Environment != nil && *w.Environment == workouts.EnvironmentIndoor {
		out.NotApplicable = true
		return out, nil
	}
	// Null environment means "not stated" — compute, but say we assumed.
	if w.Environment == nil {
		out.AssumedOutdoor = true
	}

	// A param is parsed before any I/O so a malformed one fails fast.
	var override *startOverride
	if start != "" {
		hh, mm, err := ParseStartParam(start)
		if err != nil {
			return nil, err
		}
		override = &startOverride{hour: hh, min: mm}
	}

	// The window scored is NOT necessarily the stored one: date-only scheduled
	// sessions land at midnight and would otherwise score pre-dawn hours.
	from, to, source, assumed := resolveWindow(w.StartedAt, w.EndedAt, s.userLoc, s.startHour, s.startMin, override)
	out.StartSource = source
	if assumed {
		out.AssumedStart = fmt.Sprintf("%02d:%02d", s.startHour, s.startMin)
	}
	out.Window = &ScoredWindow{
		From: from.In(s.userLoc).Format(time.RFC3339),
		To:   to.In(s.userLoc).Format(time.RFC3339),
	}

	durationMin := to.Sub(from).Minutes()
	if durationMin < 0 {
		durationMin = 0
	}
	out.DurationMin = numfmt.Round1(durationMin)

	loc, err := s.locations.LocationOn(ctx, date)
	if err != nil {
		if errors.Is(err, locations.ErrUnconfigured) {
			reason := ReasonLocationUnconfigured
			out.Reason = &reason
			return out, nil
		}
		return nil, fmt.Errorf("resolve location for heat: %w", err)
	}
	out.Location = &Location{Name: loc.Name, Source: string(loc.Source), Lat: loc.Lat, Lon: loc.Lon}

	hours, ok := s.weather.Forecast(ctx, loc.Lat, loc.Lon, weather.Window{From: from, To: to})
	if !ok {
		reason := ReasonWeatherUnavailable
		out.Reason = &reason
		return out, nil
	}
	mean, ok := weather.MeanOver(hours, from, to)
	if !ok {
		// The fetch worked but covered no hour of the session — a forecast that
		// doesn't reach the session is no forecast at all.
		reason := ReasonWeatherUnavailable
		out.Reason = &reason
		return out, nil
	}

	cond := Conditions{
		TemperatureC: numfmt.Round1(mean.TemperatureC),
		HumidityPct:  numfmt.Round1(mean.HumidityPct),
		WindSpeedMPS: numfmt.Round1(mean.WindSpeedMPS),
		CloudCovPct:  numfmt.Round1(mean.CloudCovPct),
	}
	load := ComputeLoad(Conditions{
		TemperatureC: mean.TemperatureC,
		HumidityPct:  mean.HumidityPct,
		WindSpeedMPS: mean.WindSpeedMPS,
		CloudCovPct:  mean.CloudCovPct,
	})
	out.Conditions = &cond
	out.Load = &load

	acclim := s.acclimatization(ctx, date)
	out.Acclim = &acclim

	adj := Adjustment{ReductionPct: ComputeReductionPct(load.HeatLoadC, durationMin, acclim.Level)}
	s.applyBaseline(ctx, w.Sport, &adj)
	out.Adjustment = &adj

	var signal *SweatSignal
	if s.sweat != nil {
		signal = s.sweat.LatestSweatSignal(ctx)
	}
	fluid := ComputeFluid(load.HeatLoadC, signal)
	out.Fluid = &fluid

	return out, nil
}

// acclimatization gathers the trailing-window completed sessions and bands them.
// Indoor sessions are excluded here rather than in the pure math: an indoor
// session's stored temperature says nothing about heat adaptation.
func (s *Service) acclimatization(ctx context.Context, date time.Time) AcclimEvidence {
	from := date.AddDate(0, 0, -AcclimWindowDays)
	status := string(workouts.StatusCompleted)

	completed, err := s.workouts.List(ctx, from, date, nil, &status)
	if err != nil {
		// Acclimatization evidence is supporting detail: failing to read it
		// must not take the heat report down. Low + zero evidence is the
		// conservative reading, and the empty session list shows why.
		return AcclimEvidence{Level: AcclimLow, WindowDays: AcclimWindowDays, Sessions: []QualifyingSession{}}
	}

	candidates := make([]CandidateSession, 0, len(completed))
	for _, w := range completed {
		if w.Environment != nil && *w.Environment == workouts.EnvironmentIndoor {
			continue
		}
		candidates = append(candidates, CandidateSession{
			WorkoutID:    w.ID,
			Date:         w.StartedAt.UTC().Format("2006-01-02"),
			DurationMin:  w.EndedAt.Sub(w.StartedAt).Minutes(),
			TemperatureC: w.TemperatureC,
			HumidityPct:  w.HumidityPct,
		})
	}
	return ComputeAcclimatization(candidates)
}

// applyBaseline names the baseline the percentage applies to and, when the
// effective config carries it, shows the suggested value outright — a coach
// shouldn't have to do the arithmetic. Fail-open: no config, no baseline, and
// the percentage still stands on its own.
func (s *Service) applyBaseline(ctx context.Context, sport workouts.Sport, adj *Adjustment) {
	if s.config == nil {
		return
	}
	cfg, err := s.config.Get(ctx)
	if err != nil || cfg == nil {
		return
	}
	switch sport {
	case workouts.SportBike:
		if cfg.FtpWatts == nil {
			return
		}
		adj.Baseline = BaselineFTP
		adj.BaselineFrom = float64(*cfg.FtpWatts)
		adj.SuggestedTo = numfmt.Round1(adj.BaselineFrom * (1 - adj.ReductionPct/100))
	case workouts.SportRun:
		if cfg.ThresholdPaceSecPerKm == nil {
			return
		}
		adj.Baseline = BaselineThresholdPace
		adj.BaselineFrom = *cfg.ThresholdPaceSecPerKm
		// Pace is inverted: going slower means MORE seconds per km, so a
		// reduction in speed is an increase in the number.
		adj.SuggestedTo = numfmt.Round1(adj.BaselineFrom / (1 - adj.ReductionPct/100))
	}
}
