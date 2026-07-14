package pmc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f64(v float64) *float64 { return &v }

// trajDay pulls a trajectory day out by its date string.
func trajDay(traj []TargetDay, date string) (TargetDay, bool) {
	for _, d := range traj {
		if d.Date == date {
			return d, true
		}
	}
	return TargetDay{}, false
}

// targetTSSFor: a date inside a phase with a declared target yields weekly/7;
// a gap or a null-target phase yields (0, false).
func TestTargetTSSFor(t *testing.T) {
	phases := []MacroPhase{
		{StartDate: day("2026-01-01"), EndDate: day("2026-01-31"), TargetWeeklyTSS: f64(294)},
		{StartDate: day("2026-02-01"), EndDate: day("2026-02-28"), TargetWeeklyTSS: nil},
	}
	tss, declared := targetTSSFor(phases, day("2026-01-15"))
	assert.True(t, declared)
	assert.InDelta(t, 42.0, tss, 1e-9) // 294 / 7

	tss, declared = targetTSSFor(phases, day("2026-02-15")) // null-target phase
	assert.False(t, declared)
	assert.Equal(t, 0.0, tss)

	tss, declared = targetTSSFor(phases, day("2026-03-15")) // gap (no phase)
	assert.False(t, declared)
	assert.Equal(t, 0.0, tss)
}

func TestAnyTargetDeclared(t *testing.T) {
	assert.True(t, anyTargetDeclared([]MacroPhase{{TargetWeeklyTSS: f64(300)}}))
	assert.False(t, anyTargetDeclared([]MacroPhase{{TargetWeeklyTSS: nil}}))
	assert.False(t, anyTargetDeclared(nil))
}

// Equilibrium invariant: when the daily target equals the seed CTL, the target
// curve stays flat (ctl += (ctl-ctl)/42 = ctl). A clean hand-checkable anchor.
func TestSimulate_FlatAtEquilibrium(t *testing.T) {
	start, end := day("2026-01-01"), day("2026-01-10")
	phases := []MacroPhase{{StartDate: start, EndDate: end, TargetWeeklyTSS: f64(42 * 7)}} // daily 42
	traj, _ := simulate(phases, 42.0, map[string]float64{}, start, end, start)

	require.Len(t, traj, 10)
	for _, d := range traj {
		assert.InDelta(t, 42.0, d.TargetCTL, 0.05, "day %s should stay at equilibrium", d.Date)
		assert.True(t, d.TargetDeclared)
	}
}

// Day-0 target CTL equals the seed exactly (we ramp from where the athlete is).
func TestSimulate_SeededStart(t *testing.T) {
	start, end := day("2026-01-01"), day("2026-01-05")
	phases := []MacroPhase{{StartDate: start, EndDate: end, TargetWeeklyTSS: f64(700)}}
	traj, _ := simulate(phases, 50.0, map[string]float64{}, start, end, start)
	require.NotEmpty(t, traj)
	assert.Equal(t, "2026-01-01", traj[0].Date)
	assert.InDelta(t, 50.0, traj[0].TargetCTL, 1e-9)
}

// A gap after a phase decays the target CTL and flags the undeclared span.
func TestSimulate_GapDecaysAndFlags(t *testing.T) {
	start, end := day("2026-01-01"), day("2026-01-20")
	phases := []MacroPhase{
		{StartDate: day("2026-01-01"), EndDate: day("2026-01-10"), TargetWeeklyTSS: f64(42 * 7)}, // daily 42
		// 2026-01-11 .. 2026-01-20 is a gap (no phase) → 0 target, decays.
	}
	traj, _ := simulate(phases, 42.0, map[string]float64{}, start, end, start)

	inPhase, ok := trajDay(traj, "2026-01-10")
	require.True(t, ok)
	assert.True(t, inPhase.TargetDeclared)
	assert.InDelta(t, 42.0, inPhase.TargetCTL, 0.05)

	inGap, ok := trajDay(traj, "2026-01-20")
	require.True(t, ok)
	assert.False(t, inGap.TargetDeclared)
	assert.Less(t, inGap.TargetCTL, 42.0, "target CTL decays through the undeclared gap")
}

// Actual/delta present up to lastActual, absent for future days.
func TestSimulate_ActualAndDeltaUpToToday(t *testing.T) {
	start, end := day("2026-01-01"), day("2026-01-10")
	lastActual := day("2026-01-05")
	phases := []MacroPhase{{StartDate: start, EndDate: end, TargetWeeklyTSS: f64(700)}}
	actual := map[string]float64{
		"2026-01-01": 50, "2026-01-02": 50.5, "2026-01-03": 51,
		"2026-01-04": 51.5, "2026-01-05": 52,
	}
	traj, _ := simulate(phases, 50.0, actual, start, end, lastActual)

	d3, _ := trajDay(traj, "2026-01-03")
	require.NotNil(t, d3.ActualCTL)
	require.NotNil(t, d3.Delta)
	assert.InDelta(t, 51.0, *d3.ActualCTL, 1e-9)

	future, _ := trajDay(traj, "2026-01-09")
	assert.Nil(t, future.ActualCTL, "future days carry the target only")
	assert.Nil(t, future.Delta)
}

// Under-training makes projected_end_current land below projected_end_planned.
func TestSimulate_ProjectionsDivergeWhenBehind(t *testing.T) {
	start, end := day("2026-01-01"), day("2026-03-01")
	lastActual := day("2026-01-20")
	phases := []MacroPhase{{StartDate: start, EndDate: end, TargetWeeklyTSS: f64(60 * 7)}} // daily 60

	// Actual CTL lags the target (athlete under-training): flat ~40 while the
	// target ramps from 40 toward 60.
	actual := map[string]float64{}
	for d := start; !d.After(lastActual); d = d.AddDate(0, 0, 1) {
		actual[d.Format(isoDate)] = 40.0
	}
	_, summary := simulate(phases, 40.0, actual, start, end, lastActual)
	require.NotNil(t, summary)

	assert.Less(t, summary.CurrentDelta, 0.0, "behind plan → negative current delta")
	assert.Less(t, summary.ProjectedEndCTLCurrent, summary.ProjectedEndCTLPlanned,
		"catching up from a lower actual lands below the pure plan")
}
