package expenditure

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/meals"
)

// MealsReader is the narrow read the intake side needs: every meal in the
// window, from which the per-day kcal series and the logged-day rule follow.
type MealsReader interface {
	List(ctx context.Context, p meals.ListParams) ([]*meals.MealEntry, error)
}

// TrendReader is the narrow read the mass side needs. Deliberately the
// body-weight capability's own trend rather than a local smoother: one
// smoothing truth in the system (design D1).
type TrendReader interface {
	TrendFor(ctx context.Context, p bodyweight.TrendParams) (*bodyweight.Trend, error)
}

// WeighInReader supplies the raw weigh-in count backing the gate. The trend
// alone can't answer it — a trailing average reports a value on days with no
// weigh-in of their own.
type WeighInReader interface {
	ListInRange(ctx context.Context, from, to time.Time) ([]*bodyweight.Entry, error)
}

// Service computes the energy-balance expenditure estimate. Compute-on-read:
// it persists nothing and reads neither goals nor athlete-config.
type Service struct {
	meals   MealsReader
	trend   TrendReader
	weighIn WeighInReader
}

func NewService(m MealsReader, t TrendReader, w WeighInReader) *Service {
	return &Service{meals: m, trend: t, weighIn: w}
}

// Params scopes an estimate. From and To are inclusive calendar dates
// interpreted in Loc.
type Params struct {
	From time.Time
	To   time.Time
	Loc  *time.Location
}

// EstimateFor resolves the intake series and trend endpoints over the window,
// then runs the balance.
func (s *Service) EstimateFor(ctx context.Context, p Params) (*Expenditure, error) {
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)
	windowEnd := toDay.Add(24 * time.Hour)

	entries, err := s.meals.List(ctx, meals.ListParams{From: fromDay.UTC(), To: windowEnd.UTC()})
	if err != nil {
		return nil, fmt.Errorf("list meals for expenditure: %w", err)
	}

	// Bucket kcal by the athlete's local calendar date. A day is "logged" when
	// it holds at least one meal — presence, not kcal: a logged day that sums to
	// zero kcal is still a logged day (design D2).
	kcal := map[string]float64{}
	present := map[string]bool{}
	for _, e := range entries {
		key := e.LoggedAt.In(p.Loc).Format("2006-01-02")
		n := e.EffectiveNutrimentsPer100g
		if n.KcalPer100g != nil {
			kcal[key] += *n.KcalPer100g * e.QuantityG / 100.0
		}
		present[key] = true
	}

	var days []DayIntake
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		days = append(days, DayIntake{Date: key, Kcal: kcal[key], Logged: present[key]})
	}

	weighIns, err := s.weighIn.ListInRange(ctx, fromDay.UTC(), windowEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("list weigh-ins for expenditure: %w", err)
	}

	start, end, err := s.trendEnds(ctx, fromDay, toDay, p.Loc)
	if err != nil {
		return nil, err
	}

	win := Window{
		From: fromDay.Format("2006-01-02"),
		To:   toDay.Format("2006-01-02"),
		Days: len(days),
	}
	return estimate(win, days, start, end, len(weighIns)), nil
}

// trendEnds returns the first and last dates in the window carrying a smoothed
// weight. They are not necessarily the window bounds: a window that opens or
// closes on a gap in weighing yields the nearest dates that do have a trend,
// and the response echoes those dates so the shortfall is visible rather than
// silently papered over.
func (s *Service) trendEnds(ctx context.Context, fromDay, toDay time.Time, loc *time.Location) (*trendPoint, *trendPoint, error) {
	tr, err := s.trend.TrendFor(ctx, bodyweight.TrendParams{
		From:       fromDay,
		To:         toDay,
		Loc:        loc,
		WindowDays: trendWindowDays,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("weight trend for expenditure: %w", err)
	}

	var start, end *trendPoint
	for _, pt := range tr.Points {
		if pt.RollingAvgKg == nil {
			continue
		}
		p := trendPoint{kg: *pt.RollingAvgKg, date: pt.Date}
		if start == nil {
			cp := p
			start = &cp
		}
		cp := p
		end = &cp
	}
	return start, end, nil
}
