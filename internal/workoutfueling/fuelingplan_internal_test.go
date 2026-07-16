package workoutfueling

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fptr(v float64) *float64 { return &v }
func iptr(v int) *int         { return &v }

// The design's worked fixture: a planned 3-hour ride at 180 TSS with FTP 280.
//
//	kJ  = 180/100 × 280 × 3.6 = 1814.4
//	IF  = sqrt(1.8 / 3)       ≈ 0.7746 → the 0.75–0.85 rung → 70%
//	burn = 1814.4 × 0.70 / 4  ≈ 317.5 g
//	intake (180 min → long ladder) = 60–90 g/hr → 180–270 g over 3 h
//	deficit = 317.5 − 270     ≈ 47.5 g
func TestBuildFuelingPlan_WorkedLongRide(t *testing.T) {
	out := buildFuelingPlan(uuid.New(), fptr(180), 180, iptr(280), nil)

	assert.Nil(t, out.Reason)
	require.NotNil(t, out.EstimatedKJ)
	assert.InDelta(t, 1814.4, *out.EstimatedKJ, 0.05)
	require.NotNil(t, out.EstimatedKcal)
	assert.InDelta(t, 1814.4, *out.EstimatedKcal, 0.05, "kJ ≈ kcal by convention")

	require.NotNil(t, out.Inputs.PlannedIF)
	assert.InDelta(t, 0.77, *out.Inputs.PlannedIF, 0.01)
	require.NotNil(t, out.Inputs.CHOFraction)
	assert.Equal(t, 0.70, *out.Inputs.CHOFraction)

	require.NotNil(t, out.EstimatedCarbBurnG)
	assert.InDelta(t, 317.5, *out.EstimatedCarbBurnG, 0.1)

	require.NotNil(t, out.Prescription)
	assert.Equal(t, 60.0, out.Prescription.PerHourMinG)
	assert.Equal(t, 90.0, out.Prescription.PerHourMaxG)
	assert.InDelta(t, 180, out.Prescription.SessionTotalMinG, 0.1)
	assert.InDelta(t, 270, out.Prescription.SessionTotalMaxG, 0.1)

	require.NotNil(t, out.ProjectedDeficitG)
	assert.InDelta(t, 47.5, *out.ProjectedDeficitG, 0.2)

	// Every input behind the numbers is echoed.
	require.NotNil(t, out.Inputs.PlannedTSS)
	assert.Equal(t, 180.0, *out.Inputs.PlannedTSS)
	require.NotNil(t, out.Inputs.DurationMin)
	assert.Equal(t, 180.0, *out.Inputs.DurationMin)
	require.NotNil(t, out.Inputs.FTPWatts)
	assert.Equal(t, 280, *out.Inputs.FTPWatts)
}

func TestChoFraction_LadderBands(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"recovery spin", 0.45, 0.45},
		{"just under the first rung", 0.599, 0.45},
		{"exactly 0.60 steps up", 0.60, 0.55},
		{"mid endurance", 0.70, 0.55},
		{"exactly 0.75 stays moderate", 0.75, 0.55},
		{"just over 0.75", 0.751, 0.70},
		{"exactly 0.85 stays hard", 0.85, 0.70},
		{"just over 0.85", 0.851, 0.80},
		{"race pace", 1.0, 0.80},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, choFraction(tc.in))
		})
	}
}

func TestPlannedIF_DerivedFromTSSAndHours(t *testing.T) {
	// One hour at FTP is TSS 100 → IF 1.0, by definition.
	assert.InDelta(t, 1.0, plannedIF(100, 1), 0.0001)
	// 180 TSS over 3 h → sqrt(0.6) ≈ 0.7746.
	assert.InDelta(t, 0.7746, plannedIF(180, 3), 0.0001)
	// A zero-length session can't imply an intensity.
	assert.Zero(t, plannedIF(100, 0))
}

func TestPrescribe_DurationLadder(t *testing.T) {
	cases := []struct {
		name             string
		durationMin      float64
		wantMin, wantMax float64
	}{
		{"45 min needs nothing", 45, 0, 0},
		{"59 min still nothing", 59, 0, 0},
		{"exactly 60 enters the ladder", 60, 30, 60},
		{"90 min", 90, 30, 60},
		{"exactly 150 stays medium", 150, 30, 60},
		{"151 min goes long", 151, 60, 90},
		{"5 hours", 300, 60, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := prescribe(tc.durationMin, nil)
			assert.Equal(t, tc.wantMin, got.PerHourMinG)
			assert.Equal(t, tc.wantMax, got.PerHourMaxG)
			// Totals follow the per-hour range across the session's length.
			hours := tc.durationMin / 60
			assert.InDelta(t, tc.wantMin*hours, got.SessionTotalMinG, 0.1)
			assert.InDelta(t, tc.wantMax*hours, got.SessionTotalMaxG, 0.1)
		})
	}
}

func TestPrescribe_CapacityClamp(t *testing.T) {
	// A 3-hour ride's ladder allows 60–90; a 70 g/hr gut caps it at 70.
	got := prescribe(180, fptr(70))
	assert.Equal(t, 60.0, got.PerHourMinG)
	assert.Equal(t, 70.0, got.PerHourMaxG)
	assert.InDelta(t, 180, got.SessionTotalMinG, 0.1)
	assert.InDelta(t, 210, got.SessionTotalMaxG, 0.1)

	// A capacity ABOVE the ladder doesn't raise the prescription — the ladder is
	// the guidance; capacity only ever limits it.
	got = prescribe(180, fptr(120))
	assert.Equal(t, 90.0, got.PerHourMaxG)

	// A capacity below the ladder's FLOOR pulls the floor down too: prescribing
	// a minimum the athlete can't take would be advice they cannot follow.
	got = prescribe(180, fptr(50))
	assert.Equal(t, 50.0, got.PerHourMaxG)
	assert.Equal(t, 50.0, got.PerHourMinG)

	// A short session stays at zero regardless of a big gut.
	got = prescribe(45, fptr(90))
	assert.Zero(t, got.PerHourMaxG)
	assert.Zero(t, got.SessionTotalMaxG)
}

func TestBuildFuelingPlan_ShortSessionPrescribesNothingButStillEstimatesBurn(t *testing.T) {
	// 45 min, 50 TSS, FTP 280 → burn is real, intake is not needed.
	out := buildFuelingPlan(uuid.New(), fptr(50), 45, iptr(280), nil)

	assert.Nil(t, out.Reason)
	require.NotNil(t, out.EstimatedCarbBurnG)
	assert.Greater(t, *out.EstimatedCarbBurnG, 0.0)
	require.NotNil(t, out.Prescription)
	assert.Zero(t, out.Prescription.PerHourMaxG)
	assert.Zero(t, out.Prescription.SessionTotalMaxG)
	// With no intake prescribed, the whole burn is the deficit.
	require.NotNil(t, out.ProjectedDeficitG)
	assert.InDelta(t, *out.EstimatedCarbBurnG, *out.ProjectedDeficitG, 0.05)
}

func TestBuildFuelingPlan_CapacityClampMovesTheDeficit(t *testing.T) {
	unclamped := buildFuelingPlan(uuid.New(), fptr(180), 180, iptr(280), nil)
	clamped := buildFuelingPlan(uuid.New(), fptr(180), 180, iptr(280), fptr(70))

	require.NotNil(t, clamped.ProjectedDeficitG)
	assert.Equal(t, 70.0, clamped.Prescription.PerHourMaxG)
	// Taking in less leaves a bigger hole: 317.5 − 210 ≈ 107.5.
	assert.InDelta(t, 107.5, *clamped.ProjectedDeficitG, 0.2)
	assert.Greater(t, *clamped.ProjectedDeficitG, *unclamped.ProjectedDeficitG)
	// The clamp is echoed as an input.
	require.NotNil(t, clamped.Inputs.CarbsPerHrLimit)
	assert.Equal(t, 70.0, *clamped.Inputs.CarbsPerHrLimit)
}

func TestBuildFuelingPlan_PlanDataMissing(t *testing.T) {
	// Neither a load estimate nor a duration: nothing is computable.
	out := buildFuelingPlan(uuid.New(), nil, 0, iptr(280), nil)

	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonPlanDataMissing, *out.Reason)
	assert.Nil(t, out.Prescription)
	assert.Nil(t, out.EstimatedKJ)
	assert.Nil(t, out.EstimatedCarbBurnG)
	assert.Nil(t, out.ProjectedDeficitG)
}

func TestBuildFuelingPlan_TSSMissingKeepsIntakeGuidance(t *testing.T) {
	// A duration without a TSS estimate: the ladder still applies.
	out := buildFuelingPlan(uuid.New(), nil, 180, iptr(280), nil)

	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonTSSMissing, *out.Reason)
	require.NotNil(t, out.Prescription)
	assert.Equal(t, 60.0, out.Prescription.PerHourMinG)
	assert.Equal(t, 90.0, out.Prescription.PerHourMaxG)
	// Burn needs load — nothing is invented.
	assert.Nil(t, out.EstimatedKJ)
	assert.Nil(t, out.EstimatedCarbBurnG)
	assert.Nil(t, out.ProjectedDeficitG)
	assert.Nil(t, out.Inputs.PlannedIF)
	assert.Nil(t, out.Inputs.CHOFraction)
}

func TestBuildFuelingPlan_FTPMissingKeepsIntakeGuidance(t *testing.T) {
	// TSS but no FTP: TSS alone can't be turned into work.
	out := buildFuelingPlan(uuid.New(), fptr(180), 180, nil, nil)

	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonFTPMissing, *out.Reason)
	require.NotNil(t, out.Prescription)
	assert.Equal(t, 90.0, out.Prescription.PerHourMaxG)
	assert.Nil(t, out.EstimatedKJ)
	assert.Nil(t, out.EstimatedCarbBurnG)
	assert.Nil(t, out.ProjectedDeficitG)
	// The TSS that arrived is still echoed, so the gap is obviously the FTP.
	require.NotNil(t, out.Inputs.PlannedTSS)
	assert.Equal(t, 180.0, *out.Inputs.PlannedTSS)
	assert.Nil(t, out.Inputs.FTPWatts)
}

// The layering is ordered: a missing TSS is reported even when the FTP is also
// missing — TSS is the more fundamental gap.
func TestBuildFuelingPlan_TSSMissingWinsOverFTPMissing(t *testing.T) {
	out := buildFuelingPlan(uuid.New(), nil, 180, nil, nil)

	require.NotNil(t, out.Reason)
	assert.Equal(t, ReasonTSSMissing, *out.Reason)
	require.NotNil(t, out.Prescription)
}

// A TSS with no duration can't imply an intensity, so burn stays out — but the
// answer is still not plan_data_missing, because a load estimate did arrive.
func TestBuildFuelingPlan_TSSWithoutDuration(t *testing.T) {
	out := buildFuelingPlan(uuid.New(), fptr(180), 0, iptr(280), nil)

	assert.Nil(t, out.Prescription, "no duration, no intake ladder")
	require.NotNil(t, out.EstimatedKJ, "work is duration-free under the TSS definition")
	assert.InDelta(t, 1814.4, *out.EstimatedKJ, 0.05)
	// IF collapses to 0 without hours → the easy rung, honestly echoed.
	require.NotNil(t, out.Inputs.PlannedIF)
	assert.Zero(t, *out.Inputs.PlannedIF)
	assert.Nil(t, out.ProjectedDeficitG, "no prescription, no deficit")
}

func TestBuildFuelingPlan_RoundsAtBoundary(t *testing.T) {
	out := buildFuelingPlan(uuid.New(), fptr(183.33), 187, iptr(277), nil)

	require.NotNil(t, out.EstimatedKJ)
	assert.Equal(t, roundTo1(*out.EstimatedKJ), *out.EstimatedKJ)
	require.NotNil(t, out.EstimatedCarbBurnG)
	assert.Equal(t, roundTo1(*out.EstimatedCarbBurnG), *out.EstimatedCarbBurnG)
	require.NotNil(t, out.Inputs.PlannedIF)
	assert.Equal(t, roundTo2(*out.Inputs.PlannedIF), *out.Inputs.PlannedIF)
	assert.Equal(t, roundTo1(out.Prescription.SessionTotalMaxG), out.Prescription.SessionTotalMaxG)
}

func roundTo1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
func roundTo2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
