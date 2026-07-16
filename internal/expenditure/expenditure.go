// Package expenditure estimates average daily energy expenditure (TDEE) from
// energy balance over a window: intake − expenditure = Δ stored energy. It is a
// read-only aggregator over meals (intake) and body weight (the mass signal),
// living in its own package for the workoutfueling reason — it composes several
// capture capabilities and belongs to none of them.
//
// The estimate is advisory: nothing here reads goals or athlete-config, and
// nothing is persisted. Comparing the number against a configured target — and
// deciding whether to move that target — belongs to the consumer.
package expenditure

import (
	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Gates and constants. See design D1/D3.
const (
	// kcalPerKg is the standard mixed-tissue energy density. A simplification
	// (real tissue partitioning varies with the deficit), documented as such —
	// the error is second-order at the deltas these windows see.
	kcalPerKg = 7700.0

	// minLoggedDays is the intake-mean floor: below ~2 logged weeks the mean is
	// noise, not a signal.
	minLoggedDays = 14

	// minWeighIns is the mass-signal floor: without weigh-ins spread across the
	// window the trend delta is fiction.
	minWeighIns = 5

	// trendWindowDays matches the body-weight capability's own default trailing
	// window — the smoothing that suppresses the 1–2 kg daily noise (hydration,
	// glycogen) which would otherwise dominate the balance.
	trendWindowDays = 7

	// MaxRangeDays caps the window at the nutrition tier.
	MaxRangeDays = 92
)

// Gate reasons — returned with a null estimate at 200 (the CP-model posture:
// an honest "not enough data" beats a confident wrong number).
const (
	ReasonInsufficientLoggedDays = "insufficient_logged_days"
	ReasonInsufficientWeighIns   = "insufficient_weigh_ins"
)

// DayIntake is one calendar day's logged intake. Logged is false when the day
// held no meals at all — such a day is excluded from the mean and counted,
// never read as a zero-kcal day (design D2).
type DayIntake struct {
	Date   string  `json:"date"`
	Kcal   float64 `json:"kcal"`
	Logged bool    `json:"logged"`
}

// Window echoes the requested range and its length in days — the denominator of
// the balance.
type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
	Days int    `json:"days"`
}

// TrendEnds carries the two smoothed weights the balance was computed from,
// with the dates they were taken at. The dates are echoed (rather than assumed
// to be the window bounds) so a stale endpoint is visible to the reader.
type TrendEnds struct {
	StartKg   float64 `json:"start_kg"`
	StartDate string  `json:"start_date"`
	EndKg     float64 `json:"end_kg"`
	EndDate   string  `json:"end_date"`
	DeltaKg   float64 `json:"delta_kg"`
}

// Intake reports the mean over logged days and the day accounting behind it.
type Intake struct {
	MeanKcalLoggedDays float64     `json:"mean_kcal_logged_days"`
	DaysLogged         int         `json:"days_logged"`
	DaysUnlogged       int         `json:"days_unlogged"`
	WeighIns           int         `json:"weigh_ins"`
	Days               []DayIntake `json:"days"`
}

// Expenditure is the response shape for GET /nutrition/expenditure.
// ExpenditureKcalPerDay is nil exactly when Reason is set.
type Expenditure struct {
	ExpenditureKcalPerDay *float64   `json:"expenditure_kcal_per_day"`
	Window                Window     `json:"window"`
	Trend                 *TrendEnds `json:"trend,omitempty"`
	Intake                Intake     `json:"intake"`
	Reason                *string    `json:"reason,omitempty"`
}

// estimate is the pure balance computation over already-resolved inputs: the
// per-day intake series, the trend endpoints (nil when the window holds no
// smoothed weight at all), and the weigh-in count.
//
//	expenditure = mean(intake over logged days) − Δtrend × 7700 / window_days
//
// A falling trend (negative Δ) adds to expenditure — the deficit came out of
// stored energy. Gates are checked before the arithmetic and short-circuit to a
// null estimate + reason.
func estimate(win Window, days []DayIntake, start, end *trendPoint, weighIns int) *Expenditure {
	var (
		sum      float64
		logged   int
		unlogged int
		daysOut  = make([]DayIntake, 0, len(days))
	)
	for _, d := range days {
		daysOut = append(daysOut, DayIntake{
			Date:   d.Date,
			Kcal:   numfmt.Round1(d.Kcal),
			Logged: d.Logged,
		})
		if d.Logged {
			sum += d.Kcal
			logged++
			continue
		}
		unlogged++
	}

	var mean float64
	if logged > 0 {
		mean = sum / float64(logged)
	}

	out := &Expenditure{
		Window: win,
		Intake: Intake{
			MeanKcalLoggedDays: numfmt.Round1(mean),
			DaysLogged:         logged,
			DaysUnlogged:       unlogged,
			WeighIns:           weighIns,
			Days:               daysOut,
		},
	}

	// Trend endpoints are reported whenever they exist, gated or not — a reader
	// diagnosing an "insufficient" answer wants to see what data did arrive.
	if start != nil && end != nil {
		out.Trend = &TrendEnds{
			StartKg:   numfmt.Round1(start.kg),
			StartDate: start.date,
			EndKg:     numfmt.Round1(end.kg),
			EndDate:   end.date,
			DeltaKg:   numfmt.Round1(end.kg - start.kg),
		}
	}

	switch {
	case logged < minLoggedDays:
		reason := ReasonInsufficientLoggedDays
		out.Reason = &reason
		return out
	case weighIns < minWeighIns || start == nil || end == nil:
		reason := ReasonInsufficientWeighIns
		out.Reason = &reason
		return out
	}

	// Unrounded inputs feed the balance; rounding happens at the boundary only.
	delta := end.kg - start.kg
	exp := mean - delta*kcalPerKg/float64(win.Days)
	rounded := numfmt.Round1(exp)
	out.ExpenditureKcalPerDay = &rounded
	return out
}

// trendPoint is a resolved smoothed weight and the date it was taken at.
type trendPoint struct {
	kg   float64
	date string
}
