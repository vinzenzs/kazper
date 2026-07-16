package expenditure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loggedDays builds n consecutive logged days at a constant kcal.
func loggedDays(n int, kcal float64) []DayIntake {
	out := make([]DayIntake, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, DayIntake{
			Date:   fmt.Sprintf("2026-03-%02d", i+1),
			Kcal:   kcal,
			Logged: true,
		})
	}
	return out
}

func win(days int) Window {
	return Window{From: "2026-03-01", To: fmt.Sprintf("2026-03-%02d", days), Days: days}
}

func TestEstimate_FallingTrendAddsToExpenditure(t *testing.T) {
	// 28 days at 2,800 kcal, trend down 0.5 kg:
	// expenditure = 2800 + 0.5 × 7700 / 28 = 2800 + 137.5 = 2937.5
	days := loggedDays(28, 2800)
	start := &trendPoint{kg: 72.0, date: "2026-03-01"}
	end := &trendPoint{kg: 71.5, date: "2026-03-28"}

	out := estimate(win(28), days, start, end, 12)

	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.Nil(t, out.Reason)
	assert.InDelta(t, 2937.5, *out.ExpenditureKcalPerDay, 0.001)
	require.NotNil(t, out.Trend)
	assert.InDelta(t, -0.5, out.Trend.DeltaKg, 0.001)
	assert.Equal(t, "2026-03-01", out.Trend.StartDate)
	assert.Equal(t, "2026-03-28", out.Trend.EndDate)
	assert.Equal(t, 28, out.Intake.DaysLogged)
	assert.Equal(t, 0, out.Intake.DaysUnlogged)
	assert.InDelta(t, 2800, out.Intake.MeanKcalLoggedDays, 0.001)
}

func TestEstimate_RisingTrendSubtractsFromExpenditure(t *testing.T) {
	// Gaining 0.7 kg over 28 days on 3,000 kcal means expenditure ran BELOW
	// intake: 3000 − 0.7 × 7700 / 28 = 3000 − 192.5 = 2807.5
	days := loggedDays(28, 3000)
	start := &trendPoint{kg: 70.0, date: "2026-03-01"}
	end := &trendPoint{kg: 70.7, date: "2026-03-28"}

	out := estimate(win(28), days, start, end, 10)

	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.InDelta(t, 2807.5, *out.ExpenditureKcalPerDay, 0.001)
	assert.InDelta(t, 0.7, out.Trend.DeltaKg, 0.001)
}

func TestEstimate_FlatTrendReturnsMeanIntake(t *testing.T) {
	days := loggedDays(21, 2500)
	start := &trendPoint{kg: 68.0, date: "2026-03-01"}
	end := &trendPoint{kg: 68.0, date: "2026-03-21"}

	out := estimate(win(21), days, start, end, 8)

	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.InDelta(t, 2500, *out.ExpenditureKcalPerDay, 0.001)
}

func TestEstimate_UnloggedDaysExcludedFromMeanAndCounted(t *testing.T) {
	// 20 logged days at 3,000 + 6 unlogged days. The mean must stay 3,000 —
	// were the empty days read as zero it would collapse to ~2,308.
	days := loggedDays(20, 3000)
	for i := 0; i < 6; i++ {
		days = append(days, DayIntake{Date: fmt.Sprintf("2026-03-%02d", 21+i), Kcal: 0, Logged: false})
	}
	start := &trendPoint{kg: 70.0, date: "2026-03-01"}
	end := &trendPoint{kg: 70.0, date: "2026-03-26"}

	out := estimate(win(26), days, start, end, 7)

	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.InDelta(t, 3000, out.Intake.MeanKcalLoggedDays, 0.001)
	assert.InDelta(t, 3000, *out.ExpenditureKcalPerDay, 0.001)
	assert.Equal(t, 20, out.Intake.DaysLogged)
	assert.Equal(t, 6, out.Intake.DaysUnlogged)
}

func TestEstimate_ZeroKcalDayStillCountsAsLogged(t *testing.T) {
	// Presence, not kcal, defines a logged day: a logged 0-kcal entry is data.
	days := loggedDays(20, 2000)
	days = append(days, DayIntake{Date: "2026-03-21", Kcal: 0, Logged: true})
	start := &trendPoint{kg: 70.0, date: "2026-03-01"}
	end := &trendPoint{kg: 70.0, date: "2026-03-21"}

	out := estimate(win(21), days, start, end, 6)

	assert.Equal(t, 21, out.Intake.DaysLogged)
	assert.Equal(t, 0, out.Intake.DaysUnlogged)
	// 40,000 kcal over 21 logged days.
	assert.InDelta(t, 40000.0/21.0, out.Intake.MeanKcalLoggedDays, 0.05)
}

func TestEstimate_InsufficientLoggedDaysGate(t *testing.T) {
	days := loggedDays(9, 2800)
	for i := 0; i < 19; i++ {
		days = append(days, DayIntake{Date: fmt.Sprintf("2026-03-%02d", 10+i), Logged: false})
	}
	start := &trendPoint{kg: 72.0, date: "2026-03-01"}
	end := &trendPoint{kg: 71.5, date: "2026-03-28"}

	out := estimate(win(28), days, start, end, 12)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonInsufficientLoggedDays, *out.Reason)
	assert.Equal(t, 9, out.Intake.DaysLogged)
	// The inputs that did arrive stay visible under the gate.
	assert.NotNil(t, out.Trend)
}

func TestEstimate_LoggedDaysGateBoundary(t *testing.T) {
	start := &trendPoint{kg: 72.0, date: "2026-03-01"}
	end := &trendPoint{kg: 72.0, date: "2026-03-28"}

	gated := estimate(win(28), loggedDays(13, 2800), start, end, 12)
	assert.Nil(t, gated.ExpenditureKcalPerDay, "13 logged days must gate")

	passed := estimate(win(28), loggedDays(14, 2800), start, end, 12)
	assert.NotNil(t, passed.ExpenditureKcalPerDay, "14 logged days must pass")
	assert.Nil(t, passed.Reason)
}

func TestEstimate_InsufficientWeighInsGate(t *testing.T) {
	days := loggedDays(28, 2800)
	start := &trendPoint{kg: 72.0, date: "2026-03-01"}
	end := &trendPoint{kg: 71.5, date: "2026-03-28"}

	out := estimate(win(28), days, start, end, 3)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonInsufficientWeighIns, *out.Reason)
	assert.Equal(t, 3, out.Intake.WeighIns)
}

func TestEstimate_WeighInGateBoundary(t *testing.T) {
	days := loggedDays(28, 2800)
	start := &trendPoint{kg: 72.0, date: "2026-03-01"}
	end := &trendPoint{kg: 71.5, date: "2026-03-28"}

	gated := estimate(win(28), days, start, end, 4)
	assert.Nil(t, gated.ExpenditureKcalPerDay, "4 weigh-ins must gate")

	passed := estimate(win(28), days, start, end, 5)
	assert.NotNil(t, passed.ExpenditureKcalPerDay, "5 weigh-ins must pass")
}

func TestEstimate_MissingTrendEndpointsGateAsWeighIns(t *testing.T) {
	// The count can clear 5 while the trend still resolves nothing (e.g. every
	// weigh-in landing outside the smoothing reach) — no mass signal, no number.
	out := estimate(win(28), loggedDays(28, 2800), nil, nil, 6)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonInsufficientWeighIns, *out.Reason)
	assert.Nil(t, out.Trend)
}

func TestEstimate_LoggedDayGateTakesPrecedence(t *testing.T) {
	// Both gates failing reports the intake one — it is the first thing to fix.
	out := estimate(win(28), loggedDays(5, 2800), nil, nil, 2)

	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonInsufficientLoggedDays, *out.Reason)
}

func TestEstimate_RoundsAtBoundaryOnly(t *testing.T) {
	// Intake mean that doesn't terminate: 3 days at 2000/2001/2003 → 2001.333…
	days := []DayIntake{
		{Date: "2026-03-01", Kcal: 2000.04, Logged: true},
		{Date: "2026-03-02", Kcal: 2001.04, Logged: true},
		{Date: "2026-03-03", Kcal: 2003.04, Logged: true},
	}
	days = append(days, loggedDays(14, 2001.373333)[3:]...)
	start := &trendPoint{kg: 70.06, date: "2026-03-01"}
	end := &trendPoint{kg: 70.04, date: "2026-03-14"}

	out := estimate(win(14), days, start, end, 6)

	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.Equal(t, numfmtRound1(*out.ExpenditureKcalPerDay), *out.ExpenditureKcalPerDay)
	assert.Equal(t, numfmtRound1(out.Trend.StartKg), out.Trend.StartKg)
	assert.Equal(t, numfmtRound1(out.Intake.Days[0].Kcal), out.Intake.Days[0].Kcal)
}

// numfmtRound1 mirrors the boundary rounding so the assertion above states the
// property (already rounded → rounding again is a no-op) rather than a literal.
func numfmtRound1(v float64) float64 {
	return float64(int64(v*10+sign(v)*0.5)) / 10
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

func TestEstimate_EmptyWindow(t *testing.T) {
	out := estimate(win(0), nil, nil, nil, 0)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonInsufficientLoggedDays, *out.Reason)
	assert.Equal(t, 0, out.Intake.DaysLogged)
	assert.NotNil(t, out.Intake.Days, "series serializes as [] not null")
}
