package pmc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func dt(date string, tss float64, miss int) DayTSS {
	return DayTSS{Date: day(date), TSSTotal: tss, MissingTSS: miss}
}

// dayByDate finds a day entry by ISO date.
func dayByDate(s *Series, date string) Day {
	for _, d := range s.Days {
		if d.Date == date {
			return d
		}
	}
	panic("day not found: " + date)
}

// A single 100-TSS day, then rest — EWMA recurrence against hand-computed values.
func TestComputeSeries_Recurrence(t *testing.T) {
	daily := []DayTSS{dt("2026-06-01", 100, 0)}
	s := computeSeries(day("2026-06-01"), day("2026-06-03"), "UTC", day("2026-06-01"), true, daily)

	require.NotNil(t, s.SeedDate)
	assert.Equal(t, "2026-05-31", *s.SeedDate, "seed = earliest − 1 day")
	require.Len(t, s.Days, 3)

	// 06-01: ctl=100/42=2.38→2.4, atl=100/7=14.29→14.3, tsb from seed (0−0)=0.
	d1 := s.Days[0]
	assert.Equal(t, 100.0, d1.TSSTotal)
	assert.Equal(t, 2.4, d1.CTL)
	assert.Equal(t, 14.3, d1.ATL)
	assert.Equal(t, 0.0, d1.TSB)
	// ramp_rate uses the zero baseline (d−7 before seed) → equals ctl.
	assert.Equal(t, 2.4, d1.RampRate)

	// 06-02 (rest): ctl decays to 2.32→2.3, atl to 12.24→12.2, tsb = yesterday's
	// ctl−atl = 2.38−14.29 = −11.9 (unaffected by today's own tss).
	d2 := s.Days[1]
	assert.Equal(t, 0.0, d2.TSSTotal)
	assert.Equal(t, 2.3, d2.CTL)
	assert.Equal(t, 12.2, d2.ATL)
	assert.Equal(t, -11.9, d2.TSB)
	assert.Less(t, d2.CTL, d1.CTL, "rest day decays, no gap/null")
}

// Same date returns identical values regardless of the window (warm-up from seed).
func TestComputeSeries_WindowIndependence(t *testing.T) {
	daily := []DayTSS{dt("2026-06-01", 100, 0)}
	narrow := computeSeries(day("2026-06-01"), day("2026-06-03"), "UTC", day("2026-06-01"), true, daily)
	wide := computeSeries(day("2026-05-20"), day("2026-06-10"), "UTC", day("2026-06-01"), true, daily)

	n := dayByDate(narrow, "2026-06-02")
	w := dayByDate(wide, "2026-06-02")
	assert.Equal(t, n.CTL, w.CTL)
	assert.Equal(t, n.ATL, w.ATL)
	assert.Equal(t, n.TSB, w.TSB)

	// Days before the seed carry zeros.
	before := dayByDate(wide, "2026-05-20")
	assert.Equal(t, Day{Date: "2026-05-20"}, before)
}

// Empty history → all-zero series, 200-shaped, no seed_date, empty alerts.
func TestComputeSeries_EmptyHistory(t *testing.T) {
	s := computeSeries(day("2026-06-01"), day("2026-06-03"), "UTC", time.Time{}, false, nil)
	assert.Nil(t, s.SeedDate)
	require.Len(t, s.Days, 3)
	for _, d := range s.Days {
		assert.Equal(t, 0.0, d.CTL)
		assert.Equal(t, 0.0, d.ATL)
		assert.Equal(t, 0.0, d.TSB)
		assert.Equal(t, 0.0, d.TSSTotal)
	}
	assert.NotNil(t, s.RampAlerts)
	assert.Empty(t, s.RampAlerts)
	assert.Equal(t, 0, s.MissingTSSWorkouts)
}

// missing_tss_count is per-day (omitempty) and totals into the window.
func TestComputeSeries_MissingTSS(t *testing.T) {
	daily := []DayTSS{dt("2026-06-01", 80, 1), dt("2026-06-02", 50, 0)}
	s := computeSeries(day("2026-06-01"), day("2026-06-02"), "UTC", day("2026-06-01"), true, daily)

	assert.Equal(t, 1, s.Days[0].MissingTSSCount)
	assert.Equal(t, 0, s.Days[1].MissingTSSCount, "full-coverage day omits the counter")
	assert.Equal(t, 1, s.MissingTSSWorkouts)
}

// A hard build week is flagged; a later all-rest window is not.
func TestComputeSeries_RampAlerts(t *testing.T) {
	// Anchor on a real Monday.
	mon := day("2026-06-01")
	for mon.Weekday() != time.Monday {
		mon = mon.AddDate(0, 0, 1)
	}
	var daily []DayTSS
	for i := 0; i < 14; i++ { // two hard weeks, 200 TSS/day
		daily = append(daily, DayTSS{Date: mon.AddDate(0, 0, i), TSSTotal: 200})
	}

	build := computeSeries(mon, mon.AddDate(0, 0, 13), "UTC", mon, true, daily)
	require.NotEmpty(t, build.RampAlerts, "a 200/day block ramps CTL far past 8/week")
	for _, a := range build.RampAlerts {
		ws := day(a.WeekStart)
		assert.Equal(t, time.Monday, ws.Weekday(), "alerts are Monday-start weeks")
		assert.Greater(t, a.CTLDelta, rampAlertThreshold)
	}
	// The first week's alert anchors on the training Monday.
	assert.Equal(t, mon.Format(isoDate), build.RampAlerts[0].WeekStart)

	// A window two months later (pure rest): CTL only decays → no alerts.
	rest := computeSeries(mon.AddDate(0, 0, 60), mon.AddDate(0, 0, 74), "UTC", mon, true, daily)
	assert.Empty(t, rest.RampAlerts, "a decaying window raises no ramp alerts")
	for _, d := range rest.Days { // ramp_rate still present (<= 0 here)
		assert.LessOrEqual(t, d.RampRate, 0.0)
	}
}
