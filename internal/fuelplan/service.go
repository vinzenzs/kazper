package fuelplan

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// planHorizonLookaheadDays bounds the search for the last materialized plan
// day. The same query serves the window's own sessions, so the horizon costs
// nothing extra.
const planHorizonLookaheadDays = 400

// trendLookbackDays bounds the search for the latest smoothed weight behind the
// window. A trend older than this is not a denominator worth multiplying by.
const trendLookbackDays = 30

// PlannedWorkoutsReader is the narrow load read: planned workouts by start
// time. Status filtering is the caller's (this package only ever asks for
// planned — the tier reads intent, never results, design D1).
type PlannedWorkoutsReader interface {
	List(ctx context.Context, from, to time.Time, sessionGroup, status *string) ([]*workouts.Workout, error)
}

// TrendReader supplies the g/kg denominator from the body-weight capability's
// own smoothing — the same signal adaptive-expenditure consumes.
type TrendReader interface {
	TrendFor(ctx context.Context, p bodyweight.TrendParams) (*bodyweight.Trend, error)
}

// GoalsReader resolves each date's effective goals through the full chain
// (per-date override > phase template > singleton default), so the comparison
// is against what actually applies that day, not the base target.
type GoalsReader interface {
	EffectiveForRange(ctx context.Context, from, to time.Time) (map[string]*goals.Goals, map[string]goals.GoalSource, map[string]string, error)
}

// Service builds the fuel plan. Compute-on-read: it persists nothing and writes
// no goal or override.
type Service struct {
	workouts PlannedWorkoutsReader
	trend    TrendReader
	goals    GoalsReader
}

func NewService(w PlannedWorkoutsReader, t TrendReader, g GoalsReader) *Service {
	return &Service{workouts: w, trend: t, goals: g}
}

// Params scopes a fuel plan. From and To are inclusive calendar dates in Loc.
type Params struct {
	From time.Time
	To   time.Time
	Loc  *time.Location
}

// PlanFor classifies every date in the window and prices each tier.
func (s *Service) PlanFor(ctx context.Context, p Params) (*Plan, error) {
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)

	sessions, horizon, err := s.plannedSessions(ctx, fromDay, p.Loc)
	if err != nil {
		return nil, err
	}

	effective, _, _, err := s.goals.EffectiveForRange(ctx, fromDay, toDay)
	if err != nil {
		return nil, fmt.Errorf("resolve effective goals for fuel plan: %w", err)
	}

	weight, err := s.latestTrend(ctx, fromDay, p.Loc)
	if err != nil {
		return nil, err
	}

	out := &Plan{
		From: fromDay.Format("2006-01-02"),
		To:   toDay.Format("2006-01-02"),
		TZ:   p.Loc.String(),
		Days: []Day{},
	}
	var weightKg *float64
	if weight != nil {
		out.Weight = weight
		weightKg = &weight.TrendKg
	} else {
		reason := ReasonWeightMissing
		out.Reason = &reason
	}

	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")

		// A day is beyond the plan when nothing is materialized at or after it.
		// A genuine rest day INSIDE the plan is not plan_missing; only the tail
		// past the last planned session is. The one blur this accepts: a plan
		// whose final day is itself a rest day reads as plan_missing.
		planMissing := horizon == nil || d.After(*horizon)

		var goalCarbs *goals.Range
		if g := effective[key]; g != nil {
			goalCarbs = g.CarbsG
		}
		out.Days = append(out.Days, buildDay(key, sessions[key], planMissing, weightKg, goalCarbs))
	}
	return out, nil
}

// plannedSessions returns the window's planned sessions bucketed by local date
// and the plan horizon — the latest materialized planned day at or after the
// window start. One query serves both.
func (s *Service) plannedSessions(ctx context.Context, fromDay time.Time, loc *time.Location) (map[string][]Session, *time.Time, error) {
	status := string(workouts.StatusPlanned)
	lookahead := fromDay.AddDate(0, 0, planHorizonLookaheadDays)

	planned, err := s.workouts.List(ctx, fromDay.UTC(), lookahead.UTC(), nil, &status)
	if err != nil {
		return nil, nil, fmt.Errorf("list planned workouts for fuel plan: %w", err)
	}

	out := map[string][]Session{}
	var horizon *time.Time
	for _, w := range planned {
		local := w.StartedAt.In(loc)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		if horizon == nil || day.After(*horizon) {
			d := day
			horizon = &d
		}

		sess := Session{WorkoutID: w.ID, Sport: string(w.Sport), PlannedTSS: w.TSS}
		if mins := w.EndedAt.Sub(w.StartedAt).Minutes(); mins > 0 {
			m := mins
			sess.PlannedDurationMin = &m
		}
		key := day.Format("2006-01-02")
		out[key] = append(out[key], sess)
	}
	return out, horizon, nil
}

// latestTrend returns the most recent smoothed weight at or before the window
// start, with its date. Nil when nothing in reach — tiers still ship, gram
// targets don't (design D3).
func (s *Service) latestTrend(ctx context.Context, fromDay time.Time, loc *time.Location) (*Weight, error) {
	tr, err := s.trend.TrendFor(ctx, bodyweight.TrendParams{
		From:       fromDay.AddDate(0, 0, -trendLookbackDays),
		To:         fromDay,
		Loc:        loc,
		WindowDays: 7,
	})
	if err != nil {
		return nil, fmt.Errorf("weight trend for fuel plan: %w", err)
	}
	var out *Weight
	for _, pt := range tr.Points {
		if pt.RollingAvgKg == nil {
			continue
		}
		out = &Weight{TrendKg: *pt.RollingAvgKg, Date: pt.Date}
	}
	return out, nil
}
