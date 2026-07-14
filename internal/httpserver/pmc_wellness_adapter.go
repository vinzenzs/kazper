package httpserver

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/pmc"
	"github.com/vinzenzs/kazper/internal/wellness"
)

// pmcWellnessAdapter adapts *pmc.Service to wellness.PMCProvider so the wellness
// correlation read can pair against the PMC series without internal/wellness
// importing internal/pmc (the cross-injection pattern — no package cycle, no
// duplicate EWMA).
type pmcWellnessAdapter struct {
	svc *pmc.Service
}

func (a pmcWellnessAdapter) PMCValues(ctx context.Context, from, to time.Time, loc *time.Location) ([]wellness.PMCDayValue, error) {
	series, err := a.svc.SeriesFor(ctx, pmc.Params{From: from, To: to, Loc: loc})
	if err != nil {
		return nil, err
	}
	out := make([]wellness.PMCDayValue, 0, len(series.Days))
	for _, d := range series.Days {
		out = append(out, wellness.PMCDayValue{
			Date:     d.Date,
			TSB:      d.TSB,
			CTL:      d.CTL,
			RampRate: d.RampRate,
		})
	}
	return out, nil
}
