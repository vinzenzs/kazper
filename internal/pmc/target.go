package pmc

import (
	"context"
	"errors"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// ErrMacrocycleNotFound is returned when no macrocycle resolves (unknown id or
// none active) — mapped to 404 macrocycle_not_found by the handler.
var ErrMacrocycleNotFound = errors.New("macrocycle_not_found")

// MacroPhase is one member phase's plan-relevant slice: its date span and the
// declared weekly TSS target (nil = undeclared).
type MacroPhase struct {
	StartDate       time.Time
	EndDate         time.Time
	TargetWeeklyTSS *float64
}

// Macro is the resolved macrocycle the trajectory simulates over: its span and
// its member phases with targets. Decoupled from the macrocycle package so pmc
// carries no import cycle (the resolver adapter lives in httpserver).
type Macro struct {
	ID        string
	Name      string
	StartDate time.Time
	EndDate   time.Time
	Phases    []MacroPhase
}

// MacroResolver resolves the subject macrocycle. When id is nil it returns the
// active macrocycle (whose [start,end] contains today, latest start_date
// tie-break — the public-race-feed rule); an unknown id or no active macrocycle
// returns ErrMacrocycleNotFound.
type MacroResolver interface {
	Resolve(ctx context.Context, id *string, today time.Time) (*Macro, error)
}

// TargetDay is one day of the simulated trajectory. ActualCTL/Delta are set only
// up to today; future days carry the target alone.
type TargetDay struct {
	Date           string   `json:"date"`
	TargetCTL      float64  `json:"target_ctl"`
	TargetDeclared bool     `json:"target_declared"`
	ActualCTL      *float64 `json:"actual_ctl,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
}

// TargetSummary is the headline read: how far off plan now, which way it's
// trending, and where both trajectories land at macrocycle end.
type TargetSummary struct {
	CurrentDelta           float64 `json:"current_delta"`
	DeltaTrend14d          float64 `json:"delta_trend_14d"`
	ProjectedEndCTLPlanned float64 `json:"projected_end_ctl_planned"`
	ProjectedEndCTLCurrent float64 `json:"projected_end_ctl_current"`
}

// TargetMacrocycle echoes the resolved macrocycle's identity + span.
type TargetMacrocycle struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// TargetTrajectory is the GET /performance/pmc/target-trajectory response.
// Trajectory is null (with Reason "targets_missing") when no phase declares a
// target; Summary is omitted then. Nothing is persisted.
type TargetTrajectory struct {
	Macrocycle         TargetMacrocycle `json:"macrocycle"`
	TZ                 string           `json:"tz"`
	SeedCTL            float64          `json:"seed_ctl"`
	Trajectory         []TargetDay      `json:"trajectory"`
	Reason             string           `json:"reason,omitempty"`
	Summary            *TargetSummary   `json:"summary,omitempty"`
	MissingTSSWorkouts int              `json:"missing_tss_workouts"`
}

// targetTSSFor returns the declared daily target TSS for a date (the containing
// phase's weekly target / 7) and whether a target was declared. A date in no
// phase, or in a phase with a nil target, yields (0, false) — an undeclared span
// decays, which is what it means for the declared plan (design D4).
func targetTSSFor(phases []MacroPhase, d time.Time) (float64, bool) {
	for _, p := range phases {
		if !d.Before(p.StartDate) && !d.After(p.EndDate) && p.TargetWeeklyTSS != nil {
			return *p.TargetWeeklyTSS / 7, true
		}
	}
	return 0, false
}

// anyTargetDeclared reports whether at least one phase declares a weekly target.
func anyTargetDeclared(phases []MacroPhase) bool {
	for _, p := range phases {
		if p.TargetWeeklyTSS != nil {
			return true
		}
	}
	return false
}

// TargetTrajectoryFor simulates the macrocycle's target CTL curve beside the
// measured CTL. It reuses the measured PMC (SeriesFor) for the actual curve and
// the seed (actual CTL on start_date), then folds the declared per-phase targets
// through the same 42-day EWMA. `today` is the current calendar date in `loc`.
// Compute-on-read; nothing persisted.
func (s *Service) TargetTrajectoryFor(ctx context.Context, macro *Macro, loc *time.Location, today time.Time) (*TargetTrajectory, error) {
	start := dateOnly(macro.StartDate)
	end := dateOnly(macro.EndDate)
	today = dateOnly(today)

	out := &TargetTrajectory{
		Macrocycle: TargetMacrocycle{
			ID:        macro.ID,
			Name:      macro.Name,
			StartDate: start.Format(isoDate),
			EndDate:   end.Format(isoDate),
		},
		TZ: loc.String(),
	}

	// The actual PMC runs from macrocycle start through the earlier of today and
	// end (future days have no measured CTL). The EWMA warms up over full history
	// regardless of `from`, so the start-day CTL is the honest seed.
	lastActual := end
	if today.Before(lastActual) {
		lastActual = today
	}
	actualByDate := map[string]float64{}
	seedCTL := 0.0
	if !lastActual.Before(start) {
		series, err := s.SeriesFor(ctx, Params{From: start, To: lastActual, Loc: loc})
		if err != nil {
			return nil, err
		}
		for _, d := range series.Days {
			actualByDate[d.Date] = d.CTL
		}
		seedCTL = actualByDate[start.Format(isoDate)]
		out.MissingTSSWorkouts = series.MissingTSSWorkouts
	}
	out.SeedCTL = numfmt.Round1(seedCTL)

	// No declared target anywhere → degrade with a reason (design D4).
	if !anyTargetDeclared(macro.Phases) {
		out.Reason = "targets_missing"
		return out, nil
	}

	out.Trajectory, out.Summary = simulate(macro.Phases, seedCTL, actualByDate, start, end, lastActual)
	return out, nil
}

// simulate is the pure target-curve math over already-fetched actual CTL — no
// DB, so it is directly unit-testable. It folds the declared per-phase targets
// through the 42-day EWMA from `seedCTL` on `start` (day-0 delta zero, design
// D2), pairs each measured day with its actual/delta, and computes the summary.
func simulate(phases []MacroPhase, seedCTL float64, actualByDate map[string]float64, start, end, lastActual time.Time) ([]TargetDay, *TargetSummary) {
	trajectory := []TargetDay{}
	targetByDate := map[string]float64{}
	ctl := seedCTL
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.Format(isoDate)
		_, declared := targetTSSFor(phases, d)
		if d.Equal(start) {
			ctl = seedCTL
		} else {
			tss, _ := targetTSSFor(phases, d)
			ctl += (tss - ctl) / ctlTimeConstantDays
		}
		targetByDate[k] = ctl

		day := TargetDay{Date: k, TargetCTL: numfmt.Round1(ctl), TargetDeclared: declared}
		if a, ok := actualByDate[k]; ok {
			ac := a
			day.ActualCTL = &ac
			delta := numfmt.Round1(a - ctl)
			day.Delta = &delta
		}
		trajectory = append(trajectory, day)
	}
	return trajectory, targetSummary(phases, actualByDate, targetByDate, start, end, lastActual)
}

// targetSummary computes the headline deltas + projections from the simulated
// and measured curves.
func targetSummary(phases []MacroPhase, actualByDate, targetByDate map[string]float64, start, end, lastActual time.Time) *TargetSummary {
	deltaAt := func(d time.Time) float64 {
		k := d.Format(isoDate)
		return actualByDate[k] - targetByDate[k]
	}

	currentDelta := deltaAt(lastActual)

	// 14-day trend: current delta minus the delta 14 days ago (clamped to start).
	// A simple difference — readable "diverging or converging" (design open Q).
	prior := lastActual.AddDate(0, 0, -14)
	if prior.Before(start) {
		prior = start
	}
	trend := currentDelta - deltaAt(prior)

	// Planned end: the pure target curve at macrocycle end.
	plannedEnd := targetByDate[end.Format(isoDate)]

	// Current end: extend the EWMA from today's actual CTL over the remaining
	// planned daily targets — "if you follow the plan from here, where do you land".
	currentEnd := actualByDate[lastActual.Format(isoDate)]
	for d := lastActual.AddDate(0, 0, 1); !d.After(end); d = d.AddDate(0, 0, 1) {
		tss, _ := targetTSSFor(phases, d)
		currentEnd += (tss - currentEnd) / ctlTimeConstantDays
	}

	return &TargetSummary{
		CurrentDelta:           numfmt.Round1(currentDelta),
		DeltaTrend14d:          numfmt.Round1(trend),
		ProjectedEndCTLPlanned: numfmt.Round1(plannedEnd),
		ProjectedEndCTLCurrent: numfmt.Round1(currentEnd),
	}
}
