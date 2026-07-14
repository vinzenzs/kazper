package pmc

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const isoDate = "2006-01-02"

// Service computes the PMC series over the read-only repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

// Params is a validated request window: inclusive calendar dates [From, To]
// (midnight UTC, tz-agnostic sequence) and the timezone the day buckets resolve
// in.
type Params struct {
	From  time.Time
	To    time.Time
	Loc   *time.Location
	Sport *string // nil = combined (all sports); non-nil filters the series
}

// SeriesFor computes the CTL/ATL/TSB daily series for the window. The EWMA warms
// up from the seed date (earliest completed workout − 1 day, ctl=atl=0) over the
// full stored history so returned values are window-independent. Empty history
// yields an all-zero series with no seed_date.
func (s *Service) SeriesFor(ctx context.Context, p Params) (*Series, error) {
	tz := p.Loc.String()
	earliest, hasHistory, err := s.repo.EarliestCompletedDate(ctx, tz, p.Sport)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.DailyTSS(ctx, tz, dateOnly(p.To), p.Sport)
	if err != nil {
		return nil, err
	}
	series := computeSeries(dateOnly(p.From), dateOnly(p.To), tz, earliest, hasHistory, daily)
	if p.Sport != nil {
		series.Sport = *p.Sport
	}
	return series, nil
}

// computeSeries is the pure PMC math over already-fetched repo data — no DB, so
// it is directly unit-testable. from/to are UTC-midnight calendar dates.
func computeSeries(from, to time.Time, tz string, earliest time.Time, hasHistory bool, daily []DayTSS) *Series {
	series := &Series{
		From:       from.Format(isoDate),
		To:         to.Format(isoDate),
		TZ:         tz,
		Days:       []Day{},
		RampAlerts: []RampAlert{},
	}

	if !hasHistory {
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			series.Days = append(series.Days, Day{Date: d.Format(isoDate)})
		}
		return series
	}

	seed := dateOnly(earliest).AddDate(0, 0, -1)
	seedStr := seed.Format(isoDate)
	series.SeedDate = &seedStr

	tssByDate := make(map[string]float64, len(daily))
	missByDate := make(map[string]int, len(daily))
	for _, dt := range daily {
		k := dt.Date.Format(isoDate)
		tssByDate[k] = dt.TSSTotal
		missByDate[k] = dt.MissingTSS
	}

	// Full-precision EWMA pass from the seed through `to`, recording each day's
	// ctl/atl/tsb/tss for the window slice and ctl for ramp lookups.
	type vals struct{ ctl, atl, tsb, tss float64 }
	byDate := make(map[string]vals)
	ctlByDate := make(map[string]float64)

	ctl, atl := 0.0, 0.0
	for d := seed; !d.After(to); d = d.AddDate(0, 0, 1) {
		k := d.Format(isoDate)
		tsb := ctl - atl // form going into day d = yesterday's ctl − atl
		if d.Equal(seed) {
			// Seed day: ctl=atl=0, before any workout.
			byDate[k] = vals{ctl: 0, atl: 0, tsb: 0, tss: 0}
			ctlByDate[k] = 0
			continue
		}
		tss := tssByDate[k]
		ctl += (tss - ctl) / ctlTimeConstantDays
		atl += (tss - atl) / atlTimeConstantDays
		byDate[k] = vals{ctl: ctl, atl: atl, tsb: tsb, tss: tss}
		ctlByDate[k] = ctl
	}

	// ctlAt returns the recorded CTL for a date, or 0 before the seed.
	ctlAt := func(d time.Time) float64 { return ctlByDate[d.Format(isoDate)] }

	// Window slice [from, to]: days before the seed carry zeros.
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		k := d.Format(isoDate)
		v, ok := byDate[k]
		day := Day{Date: k}
		if ok {
			day.TSSTotal = numfmt.Round1(v.tss)
			day.CTL = numfmt.Round1(v.ctl)
			day.ATL = numfmt.Round1(v.atl)
			day.TSB = numfmt.Round1(v.tsb)
			day.RampRate = numfmt.Round1(v.ctl - ctlAt(d.AddDate(0, 0, -7)))
			if m := missByDate[k]; m > 0 {
				day.MissingTSSCount = m
				series.MissingTSSWorkouts += m
			}
		}
		series.Days = append(series.Days, day)
	}

	// Ramp alerts: one per Monday-start week whose last day (Sunday) is in the
	// window and whose CTL rise over the week exceeds the safe ceiling.
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Sunday {
			continue
		}
		weekStart := d.AddDate(0, 0, -6)
		ctlStart := ctlAt(weekStart.AddDate(0, 0, -1)) // day before the Monday
		ctlEnd := ctlAt(d)
		delta := ctlEnd - ctlStart
		if delta > rampAlertThreshold {
			series.RampAlerts = append(series.RampAlerts, RampAlert{
				WeekStart: weekStart.Format(isoDate),
				CTLStart:  numfmt.Round1(ctlStart),
				CTLEnd:    numfmt.Round1(ctlEnd),
				CTLDelta:  numfmt.Round1(delta),
			})
		}
	}

	return series
}

// dateOnly strips any time-of-day, keeping the calendar date at UTC midnight so
// day iteration is a clean tz-agnostic sequence.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
