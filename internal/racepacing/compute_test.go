package racepacing

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/races"
)

func iptr(i int) *int         { return &i }
func fptr(f float64) *float64 { return &f }

func leg(ordinal int, d races.Discipline, durMin *int) *races.RaceLeg {
	return &races.RaceLeg{Ordinal: ordinal, Discipline: d, ExpectedDurationMin: durMin}
}

func race(legs ...*races.RaceLeg) *races.Race {
	return &races.Race{ID: uuid.New(), Name: "Test Race", RaceDate: "2026-09-01", Legs: legs}
}

func cfg(ftp *int, pace, css *float64) *athleteconfig.AthleteConfig {
	return &athleteconfig.AthleteConfig{FtpWatts: ftp, ThresholdPaceSecPerKm: pace, ThresholdSwimPaceSecPer100m: css}
}

// --- exact spec scenarios -------------------------------------------------

func TestCompute_FullDistanceBike(t *testing.T) {
	p := compute(race(leg(1, races.DisciplineBike, iptr(300))), cfg(iptr(265), nil, nil), nil)
	require.Len(t, p.Legs, 1)
	lp := p.Legs[0]
	assert.Equal(t, SourceComputed, lp.Source)
	require.NotNil(t, lp.TargetPowerLowW)
	assert.Equal(t, 180, *lp.TargetPowerLowW)  // round(265 × 0.68)
	assert.Equal(t, 207, *lp.TargetPowerHighW) // round(265 × 0.78)
	require.NotNil(t, lp.IntensityFactor)
	assert.InDelta(t, 0.73, *lp.IntensityFactor, 1e-9)
	require.NotNil(t, lp.EstimatedTSS)
	assert.InDelta(t, 266.45, *lp.EstimatedTSS, 0.01) // 5h × 0.73² × 100 → 266.5 @1dp
}

func TestCompute_RunOffTheBike(t *testing.T) {
	// swim → bike → run; run leg 100 min, threshold 270.
	p := compute(
		race(
			leg(1, races.DisciplineSwim, iptr(30)),
			leg(2, races.DisciplineBike, iptr(150)),
			leg(3, races.DisciplineRun, iptr(100)),
		),
		cfg(iptr(250), fptr(270), fptr(100)), nil)
	run := p.Legs[2]
	require.NotNil(t, run.TargetPaceLowSecPerKM)
	assert.InDelta(t, 297.0, *run.TargetPaceLowSecPerKM, 1e-9)  // 270 × 1.10
	assert.InDelta(t, 318.6, *run.TargetPaceHighSecPerKM, 1e-9) // 270 × 1.18
	assert.InDelta(t, 0.877193, *run.IntensityFactor, 1e-5)     // 1 / 1.14 → 0.88 @2dp
	assert.Contains(t, run.Rationale, "off the bike")
	// Unit isolation: a run leg carries no power/swim fields.
	assert.Nil(t, run.TargetPowerLowW)
	assert.Nil(t, run.TargetPaceLowSecPer100m)
}

func TestCompute_StandaloneShortRunNoBikeMention(t *testing.T) {
	p := compute(race(leg(1, races.DisciplineRun, iptr(25))), cfg(nil, fptr(270), nil), nil)
	run := p.Legs[0]
	assert.InDelta(t, 270.0, *run.TargetPaceLowSecPerKM, 1e-9)  // ×1.00
	assert.InDelta(t, 280.8, *run.TargetPaceHighSecPerKM, 1e-9) // ×1.04
	assert.NotContains(t, run.Rationale, "off the bike")
}

func TestCompute_LongCourseSwim(t *testing.T) {
	p := compute(race(leg(1, races.DisciplineSwim, iptr(70))), cfg(nil, nil, fptr(105)), nil)
	sw := p.Legs[0]
	assert.InDelta(t, 111.3, *sw.TargetPaceLowSecPer100m, 1e-9)  // 105 × 1.06
	assert.InDelta(t, 117.6, *sw.TargetPaceHighSecPer100m, 1e-9) // 105 × 1.12
	assert.InDelta(t, 0.917431, *sw.IntensityFactor, 1e-5)       // 1 / 1.09 → 0.92 @2dp
	assert.InDelta(t, 90.088, *sw.EstimatedTSS, 0.01)            // (70/60) × IF³ × 100 → 90.1 @1dp
}

// --- band boundaries ------------------------------------------------------

func TestCompute_BikeBandBoundary180(t *testing.T) {
	ftp := iptr(200)
	p := compute(race(leg(1, races.DisciplineBike, iptr(179)), leg(2, races.DisciplineBike, iptr(180))), cfg(ftp, nil, nil), nil)
	// 179 → 75–83% band; 180 → 68–78% band.
	assert.Equal(t, 150, *p.Legs[0].TargetPowerLowW)          // round(200 × 0.75)
	assert.InDelta(t, 0.79, *p.Legs[0].IntensityFactor, 1e-9) // mid of 0.75–0.83
	assert.InDelta(t, 0.73, *p.Legs[1].IntensityFactor, 1e-9) // mid of 0.68–0.78
}

func TestCompute_RunBandBoundaries(t *testing.T) {
	pace := fptr(300)
	for _, tc := range []struct {
		dur    int
		wantIF float64 // 1 / midpoint
	}{
		{29, 1 / 1.02}, {30, 1 / 1.07}, {59, 1 / 1.07}, {60, 1 / 1.14}, {149, 1 / 1.14}, {150, 1 / 1.23},
	} {
		p := compute(race(leg(1, races.DisciplineRun, iptr(tc.dur))), cfg(nil, pace, nil), nil)
		assert.InDelta(t, tc.wantIF, *p.Legs[0].IntensityFactor, 1e-9, "run dur %d", tc.dur)
	}
}

func TestCompute_SwimBandBoundaries(t *testing.T) {
	css := fptr(100)
	for _, tc := range []struct {
		dur      int
		wantMult band
	}{
		{19, band{1.00, 1.05}}, {20, band{1.03, 1.08}}, {44, band{1.03, 1.08}}, {45, band{1.06, 1.12}},
	} {
		p := compute(race(leg(1, races.DisciplineSwim, iptr(tc.dur))), cfg(nil, nil, css), nil)
		assert.InDelta(t, 100*tc.wantMult.low, *p.Legs[0].TargetPaceLowSecPer100m, 1e-9, "swim dur %d", tc.dur)
	}
}

// --- degradation ----------------------------------------------------------

func TestCompute_MissingFTPDegradesOnlyBike(t *testing.T) {
	p := compute(
		race(leg(1, races.DisciplineBike, iptr(150)), leg(2, races.DisciplineRun, iptr(60))),
		cfg(nil, fptr(270), nil), nil)
	bike := p.Legs[0]
	assert.Nil(t, bike.TargetPowerLowW)
	assert.Equal(t, []string{fieldFTP}, bike.MissingThresholds)
	assert.Equal(t, SourceNone, bike.Source)
	run := p.Legs[1]
	require.NotNil(t, run.TargetPaceLowSecPerKM) // run intact
	assert.Contains(t, p.MissingThresholds, fieldFTP)
	assert.False(t, p.TSSComplete)
}

func TestCompute_NoConfigStillReturnsSkeleton(t *testing.T) {
	p := compute(race(leg(1, races.DisciplineBike, iptr(150)), leg(2, races.DisciplineRun, iptr(60))), nil, nil)
	assert.Nil(t, p.Legs[0].TargetPowerLowW)
	assert.Nil(t, p.Legs[1].TargetPaceLowSecPerKM)
	assert.ElementsMatch(t, []string{fieldFTP, fieldPace}, p.MissingThresholds)
	assert.False(t, p.TSSComplete)
}

func TestCompute_TransitionAndOther(t *testing.T) {
	p := compute(
		race(
			leg(1, races.DisciplineBike, iptr(60)),
			leg(2, races.DisciplineTransition, iptr(2)),
			leg(3, races.DisciplineOther, iptr(30)),
		),
		cfg(iptr(250), nil, nil), nil)
	tr := p.Legs[1]
	assert.Nil(t, tr.TargetPowerLowW)
	require.NotNil(t, tr.EstimatedTSS)
	assert.Equal(t, 0.0, *tr.EstimatedTSS)
	other := p.Legs[2]
	assert.Equal(t, SourceNone, other.Source)
	assert.Nil(t, other.EstimatedTSS)
	assert.False(t, p.TSSComplete) // 'other' leg makes it incomplete
}

func TestCompute_UnknownDurationNotBanded(t *testing.T) {
	p := compute(race(leg(1, races.DisciplineBike, nil)), cfg(iptr(250), nil, nil), nil)
	lp := p.Legs[0]
	assert.Equal(t, SourceNone, lp.Source)
	assert.Nil(t, lp.TargetPowerLowW)
	assert.Nil(t, lp.EstimatedTSS)
	assert.Contains(t, lp.Rationale, "Duration unknown")
}

// --- override merge -------------------------------------------------------

func TestCompute_OverrideWinsAndRederivesTSS(t *testing.T) {
	ov := map[int]*Override{1: {TargetPowerLowW: iptr(190), TargetPowerHighW: iptr(200)}}
	p := compute(race(leg(1, races.DisciplineBike, iptr(300))), cfg(iptr(265), nil, nil), ov)
	lp := p.Legs[0]
	assert.Equal(t, SourceOverride, lp.Source)
	assert.Equal(t, 190, *lp.TargetPowerLowW)
	assert.Equal(t, 200, *lp.TargetPowerHighW)
	require.NotNil(t, lp.IntensityFactor)
	assert.InDelta(t, 195.0/265.0, *lp.IntensityFactor, 1e-9) // midpoint 195 / FTP
	require.NotNil(t, lp.EstimatedTSS)
	assert.Contains(t, lp.Rationale, "override")
}

func TestCompute_OverrideAppliesWithoutThreshold(t *testing.T) {
	// No FTP but an absolute override → targets apply, IF/TSS omitted.
	ov := map[int]*Override{1: {TargetPowerLowW: iptr(190), TargetPowerHighW: iptr(200)}}
	p := compute(race(leg(1, races.DisciplineBike, iptr(300))), nil, ov)
	lp := p.Legs[0]
	assert.Equal(t, SourceOverride, lp.Source)
	assert.Equal(t, 190, *lp.TargetPowerLowW)
	assert.Nil(t, lp.IntensityFactor)
	assert.Nil(t, lp.EstimatedTSS)
}

func TestCompute_MismatchedOverrideIgnored(t *testing.T) {
	// A power override stored on what is now a run leg → ignored, computed instead.
	ov := map[int]*Override{1: {TargetPowerLowW: iptr(190), TargetPowerHighW: iptr(200)}}
	p := compute(race(leg(1, races.DisciplineRun, iptr(60))), cfg(nil, fptr(270), nil), ov)
	lp := p.Legs[0]
	assert.Equal(t, SourceComputed, lp.Source)
	require.NotNil(t, lp.TargetPaceLowSecPerKM) // computed run band
	assert.Nil(t, lp.TargetPowerLowW)
	assert.Contains(t, lp.Rationale, "ignored")
}

// --- totals honesty -------------------------------------------------------

func TestCompute_TotalsSumAndComplete(t *testing.T) {
	p := compute(
		race(
			leg(1, races.DisciplineSwim, iptr(30)),
			leg(2, races.DisciplineTransition, iptr(3)),
			leg(3, races.DisciplineBike, iptr(150)),
			leg(4, races.DisciplineRun, iptr(60)),
		),
		cfg(iptr(250), fptr(270), fptr(100)), nil)
	assert.True(t, p.TSSComplete)
	require.NotNil(t, p.TotalDurationMin)
	assert.Equal(t, 243, *p.TotalDurationMin)
	// Total equals the sum of the four legs' estimates (transition contributes 0).
	var sum float64
	for _, l := range p.Legs {
		if l.EstimatedTSS != nil {
			sum += *l.EstimatedTSS
		}
	}
	assert.InDelta(t, sum, p.EstimatedTSSTotal, 1e-9)
}
