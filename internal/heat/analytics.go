package heat

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/stats"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Heat-index buckets for the analytics read. Fixed, and deliberately the same
// shape the adjustment table bands on, so the evidence lines up with the thing
// it is evidence for.
const (
	bucketCool          = "<20"
	bucketMild          = "20-25"
	bucketWarm          = "25-30"
	bucketHot           = ">30"
	minCorrelationPairs = 10
)

// bucketOrder fixes the response order — a gradient is only readable in order.
var bucketOrder = []string{bucketCool, bucketMild, bucketWarm, bucketHot}

func bucketFor(heatIndexC float64) string {
	switch {
	case heatIndexC < 20:
		return bucketCool
	case heatIndexC < 25:
		return bucketMild
	case heatIndexC <= 30:
		return bucketWarm
	default:
		return bucketHot
	}
}

// Bucket is one heat band's summary. Every mean is nullable: a bucket can hold
// sessions that carry no EF, and a zero would read as "terrible" rather than
// "not measured".
type Bucket struct {
	Bucket        string   `json:"bucket"`
	Sessions      int      `json:"sessions"`
	MeanDurationM *float64 `json:"mean_duration_min,omitempty"`
	MeanHeatIndex *float64 `json:"mean_heat_index_c,omitempty"`
	MeanEF        *float64 `json:"mean_ef,omitempty"`
	MeanDecoupPct *float64 `json:"mean_decoupling_pct,omitempty"`
	// MeanOutputRelPct is the bucket's mean power relative to the window's
	// overall mean (100 = the window baseline), so the gradient reads without
	// knowing the athlete's absolute numbers.
	MeanOutputRelPct *float64 `json:"mean_output_rel_pct,omitempty"`
}

// Correlation is one metric's association with heat index.
type Correlation struct {
	N      int      `json:"n"`
	Rho    *float64 `json:"rho,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// Analytics is the response shape for GET /workouts/heat-analytics.
type Analytics struct {
	From string `json:"from"`
	To   string `json:"to"`
	TZ   string `json:"tz"`

	Sessions int      `json:"sessions"`
	Buckets  []Bucket `json:"buckets"`

	EFVsHeat         Correlation `json:"ef_vs_heat_index"`
	DecouplingVsHeat Correlation `json:"decoupling_vs_heat_index"`

	// AssumedOutdoor counts sessions with a null environment that were included
	// anyway — the caveat, made visible rather than buried.
	AssumedOutdoor int `json:"assumed_outdoor"`
}

// analyticsSession is one workout reduced to what the aggregation needs.
type analyticsSession struct {
	heatIndexC float64
	durationM  float64
	ef         *float64
	decoupling *float64
	powerW     *float64
}

// AnalyticsFor buckets the window's outdoor completed sessions by heat index
// and correlates the performance metrics against it.
//
// Deliberately DESCRIPTIVE, not a model fit: this is the evidence stream a
// human reads before proposing new adjustment constants. Nothing here refits
// anything automatically.
func (s *Service) AnalyticsFor(ctx context.Context, from, to time.Time, loc *time.Location) (*Analytics, error) {
	status := string(workouts.StatusCompleted)
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour)

	completed, err := s.workouts.List(ctx, fromDay.UTC(), toDay.UTC(), nil, &status)
	if err != nil {
		return nil, err
	}

	out := &Analytics{
		From:    fromDay.Format("2006-01-02"),
		To:      time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).Format("2006-01-02"),
		TZ:      loc.String(),
		Buckets: []Bucket{},
	}

	sessions := make([]analyticsSession, 0, len(completed))
	for _, w := range completed {
		// Indoor is excluded outright: a trainer session's temperature says
		// nothing about racing in the heat, and including it would flatten the
		// very gradient this read exists to find.
		if w.Environment != nil && *w.Environment == workouts.EnvironmentIndoor {
			continue
		}
		if w.TemperatureC == nil {
			continue // no temperature, no heat index, no evidence
		}
		if w.Environment == nil {
			out.AssumedOutdoor++
		}
		humidity := 50.0
		if w.HumidityPct != nil {
			humidity = *w.HumidityPct
		}
		sess := analyticsSession{
			heatIndexC: HeatIndexC(*w.TemperatureC, humidity),
			durationM:  w.EndedAt.Sub(w.StartedAt).Minutes(),
			ef:         w.EfficiencyFactor,
			decoupling: w.DecouplingPct,
		}
		if w.AvgPowerW != nil && *w.AvgPowerW > 0 {
			p := float64(*w.AvgPowerW)
			sess.powerW = &p
		}
		sessions = append(sessions, sess)
	}
	out.Sessions = len(sessions)

	// The output baseline is the window's own mean power: the question is
	// "slower than usual when hot", and "usual" is this athlete, this window.
	baseline := meanOf(sessions, func(a analyticsSession) *float64 { return a.powerW })

	byBucket := map[string][]analyticsSession{}
	for _, sess := range sessions {
		key := bucketFor(sess.heatIndexC)
		byBucket[key] = append(byBucket[key], sess)
	}
	for _, key := range bucketOrder {
		group := byBucket[key]
		if len(group) == 0 {
			continue // an unrepresented bucket is omitted, not zero-filled
		}
		b := Bucket{Bucket: key, Sessions: len(group)}
		b.MeanDurationM = round1Ptr(meanOf(group, func(a analyticsSession) *float64 { return &a.durationM }))
		hi := meanOf(group, func(a analyticsSession) *float64 { return &a.heatIndexC })
		b.MeanHeatIndex = round1Ptr(hi)
		b.MeanEF = round2Ptr(meanOf(group, func(a analyticsSession) *float64 { return a.ef }))
		b.MeanDecoupPct = round1Ptr(meanOf(group, func(a analyticsSession) *float64 { return a.decoupling }))
		if baseline != nil && *baseline > 0 {
			if bp := meanOf(group, func(a analyticsSession) *float64 { return a.powerW }); bp != nil {
				rel := numfmt.Round1(*bp / *baseline * 100)
				b.MeanOutputRelPct = &rel
			}
		}
		out.Buckets = append(out.Buckets, b)
	}

	out.EFVsHeat = correlate(sessions, func(a analyticsSession) *float64 { return a.ef })
	out.DecouplingVsHeat = correlate(sessions, func(a analyticsSession) *float64 { return a.decoupling })
	return out, nil
}

// correlate pairs a metric against heat index over the sessions that carry it.
// Below the pair floor it reports the count and a reason instead of a rho —
// early sparse data can't produce a confident number (the wellness-correlation
// posture).
func correlate(sessions []analyticsSession, pick func(analyticsSession) *float64) Correlation {
	var xs, ys []float64
	for _, s := range sessions {
		v := pick(s)
		if v == nil {
			continue
		}
		xs = append(xs, s.heatIndexC)
		ys = append(ys, *v)
	}
	out := Correlation{N: len(xs)}
	if len(xs) < minCorrelationPairs {
		out.Reason = "insufficient_pairs"
		return out
	}
	rho := numfmt.Round2(stats.Spearman(xs, ys))
	out.Rho = &rho
	return out
}

// meanOf averages a nullable field over the sessions that carry it; nil when
// none do.
func meanOf(sessions []analyticsSession, pick func(analyticsSession) *float64) *float64 {
	var sum float64
	var n int
	for _, s := range sessions {
		if v := pick(s); v != nil {
			sum += *v
			n++
		}
	}
	if n == 0 {
		return nil
	}
	m := sum / float64(n)
	return &m
}

func round1Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := numfmt.Round1(*v)
	return &r
}

func round2Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := numfmt.Round2(*v)
	return &r
}
