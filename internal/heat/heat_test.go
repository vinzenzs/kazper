package heat

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f(v float64) *float64 { return &v }

// ============================================================================
// heat index
// ============================================================================

func TestHeatIndexC_HotHumidExceedsDryBulb(t *testing.T) {
	// The whole point of the index: 32 °C at 70% RH feels much hotter than 32 °C.
	hi := HeatIndexC(32, 70)
	assert.Greater(t, hi, 38.0)
	assert.Less(t, hi, 48.0)
}

func TestHeatIndexC_KnownReferencePoint(t *testing.T) {
	// NWS table: 90 °F (32.2 °C) at 70% RH ≈ 105 °F (40.6 °C).
	hi := HeatIndexC(32.2, 70)
	assert.InDelta(t, 40.6, hi, 1.5)
}

func TestHeatIndexC_DryHeatCloseToDryBulb(t *testing.T) {
	// Low humidity: sweat evaporates, so it feels near (or below) air temp.
	hi := HeatIndexC(35, 10)
	assert.InDelta(t, 35, hi, 4.0)
}

func TestHeatIndexC_CoolDayIsAboutTheTemperature(t *testing.T) {
	// Below the regression's floor the simple form is used, so a cool day's
	// index must not wander far from its temperature.
	for _, temp := range []float64{5, 12, 18, 22} {
		hi := HeatIndexC(temp, 60)
		assert.InDelta(t, temp, hi, 3.0, "temp=%v", temp)
	}
}

func TestHeatIndexC_RisesWithHumidity(t *testing.T) {
	prev := HeatIndexC(32, 20)
	for _, rh := range []float64{40, 60, 80, 95} {
		hi := HeatIndexC(32, rh)
		assert.Greater(t, hi, prev, "humidity %v must not lower the index", rh)
		prev = hi
	}
}

// ============================================================================
// composite load
// ============================================================================

func TestComputeLoad_FullSunNoWind(t *testing.T) {
	// 30 °C / 60% RH, still air, clear sky: index + the full solar penalty.
	got := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, WindSpeedMPS: 0, CloudCovPct: 0})

	assert.InDelta(t, 3.0, got.SolarPenaltyC, 0.001, "clear sky → full penalty")
	assert.Zero(t, got.WindCoolingC, "still air cools nothing")
	assert.InDelta(t, got.HeatIndexC+3.0, got.HeatLoadC, 0.05)
}

func TestComputeLoad_OvercastRemovesSolarPenalty(t *testing.T) {
	clear := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, CloudCovPct: 0})
	overcast := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, CloudCovPct: 100})

	assert.Zero(t, overcast.SolarPenaltyC)
	assert.InDelta(t, clear.HeatLoadC-3.0, overcast.HeatLoadC, 0.05)

	// Half cloud → half the penalty (linear in v1).
	half := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, CloudCovPct: 50})
	assert.InDelta(t, 1.5, half.SolarPenaltyC, 0.001)
}

func TestComputeLoad_WindCoolingStartsAtThresholdAndCaps(t *testing.T) {
	// Below the threshold, nothing.
	calm := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, WindSpeedMPS: 2.9, CloudCovPct: 100})
	assert.Zero(t, calm.WindCoolingC)

	// Just above: 4 m/s → (4−3)×0.4 = 0.4 °C.
	breeze := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, WindSpeedMPS: 4, CloudCovPct: 100})
	assert.InDelta(t, 0.4, breeze.WindCoolingC, 0.001)

	// A gale can't argue the heat away: the cooling is capped.
	gale := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, WindSpeedMPS: 25, CloudCovPct: 100})
	assert.InDelta(t, 3.0, gale.WindCoolingC, 0.001)
	assert.Greater(t, gale.HeatLoadC, 20.0, "a hot day stays hot in the wind")
}

func TestComputeLoad_CloudCoverClamped(t *testing.T) {
	// Defensive: a nonsense cloud value must not invert the solar term.
	over := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, CloudCovPct: 150})
	assert.Zero(t, over.SolarPenaltyC)
	under := ComputeLoad(Conditions{TemperatureC: 30, HumidityPct: 60, CloudCovPct: -20})
	assert.InDelta(t, 3.0, under.SolarPenaltyC, 0.001)
}

func TestComputeLoad_HandComputedFixture(t *testing.T) {
	// 31 °C / 55% RH, 4 m/s wind, 50% cloud — the spec's happy-path shape.
	got := ComputeLoad(Conditions{TemperatureC: 31, HumidityPct: 55, WindSpeedMPS: 4, CloudCovPct: 50})

	hi := HeatIndexC(31, 55)
	want := hi + 1.5 - 0.4
	assert.InDelta(t, want, got.HeatLoadC, 0.05)
	assert.Greater(t, got.HeatLoadC, 31.0, "hot and humid reads above the dry bulb")
}

// ============================================================================
// acclimatization
// ============================================================================

func candidate(hot bool, mins float64) CandidateSession {
	temp := 18.0
	if hot {
		temp = 30.0
	}
	return CandidateSession{
		WorkoutID:    uuid.New(),
		Date:         "2026-07-10",
		DurationMin:  mins,
		TemperatureC: &temp,
		HumidityPct:  f(55),
	}
}

func TestComputeAcclimatization_Bands(t *testing.T) {
	cases := []struct {
		qualifying int
		want       Acclimatization
	}{
		{0, AcclimLow},
		{1, AcclimLow},
		{2, AcclimMedium},
		{3, AcclimMedium},
		{4, AcclimMedium},
		{5, AcclimGood},
		{9, AcclimGood},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d sessions", tc.qualifying), func(t *testing.T) {
			var in []CandidateSession
			for i := 0; i < tc.qualifying; i++ {
				in = append(in, candidate(true, 90))
			}
			got := ComputeAcclimatization(in)
			assert.Equal(t, tc.want, got.Level)
			assert.Equal(t, tc.qualifying, got.Count)
			assert.Len(t, got.Sessions, tc.qualifying, "every counted session is echoed")
			assert.Equal(t, AcclimWindowDays, got.WindowDays)
		})
	}
}

func TestComputeAcclimatization_ShortSessionsDontQualify(t *testing.T) {
	// Hot but brief: 59 minutes is below the floor, 60 is on it.
	assert.Equal(t, 0, ComputeAcclimatization([]CandidateSession{candidate(true, 59)}).Count)
	assert.Equal(t, 1, ComputeAcclimatization([]CandidateSession{candidate(true, 60)}).Count)
}

func TestComputeAcclimatization_CoolSessionsDontQualify(t *testing.T) {
	// Long but cool — no heat adaptation on offer.
	got := ComputeAcclimatization([]CandidateSession{
		candidate(false, 180), candidate(false, 240), candidate(false, 300),
	})
	assert.Equal(t, 0, got.Count)
	assert.Equal(t, AcclimLow, got.Level)
	assert.Empty(t, got.Sessions)
}

func TestComputeAcclimatization_MissingTemperatureCantQualify(t *testing.T) {
	// No stored temperature → we don't know how hot it was; it cannot count.
	got := ComputeAcclimatization([]CandidateSession{
		{WorkoutID: uuid.New(), Date: "2026-07-10", DurationMin: 180, TemperatureC: nil, HumidityPct: f(60)},
	})
	assert.Equal(t, 0, got.Count)
}

func TestComputeAcclimatization_MissingHumidityFallsBackNotDropped(t *testing.T) {
	// A genuinely hot ride must not be discarded over a missing humidity field.
	temp := 34.0
	got := ComputeAcclimatization([]CandidateSession{
		{WorkoutID: uuid.New(), Date: "2026-07-10", DurationMin: 120, TemperatureC: &temp, HumidityPct: nil},
	})
	assert.Equal(t, 1, got.Count)
	assert.Greater(t, got.Sessions[0].HeatIndexC, 25.0)
}

func TestComputeAcclimatization_EvidenceIsAuditable(t *testing.T) {
	c := candidate(true, 120)
	got := ComputeAcclimatization([]CandidateSession{c, candidate(false, 200), candidate(true, 30)})

	require.Len(t, got.Sessions, 1, "only the hot, long session counts")
	assert.Equal(t, c.WorkoutID, got.Sessions[0].WorkoutID)
	assert.Equal(t, "2026-07-10", got.Sessions[0].Date)
	assert.InDelta(t, 120, got.Sessions[0].DurationM, 0.001)
	assert.Greater(t, got.Sessions[0].HeatIndexC, 25.0)
}

func TestComputeAcclimatization_EmptyInput(t *testing.T) {
	got := ComputeAcclimatization(nil)
	assert.Equal(t, AcclimLow, got.Level, "no evidence reads conservative, not optimistic")
	assert.Equal(t, 0, got.Count)
	assert.NotNil(t, got.Sessions, "serializes as [] not null")
}

// ============================================================================
// adjustment table
// ============================================================================

func TestComputeReductionPct_HeatLoadBands(t *testing.T) {
	// Moderate duration + medium acclimatization isolates the load axis.
	cases := []struct {
		loadC float64
		want  float64
	}{
		{18, 0}, {23.9, 0},
		{24, 2}, {27.9, 2},
		{28, 5}, {31.9, 5},
		{32, 9}, {35.9, 9},
		{36, 14}, {42, 14},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%.1fC", tc.loadC), func(t *testing.T) {
			assert.InDelta(t, tc.want, ComputeReductionPct(tc.loadC, 90, AcclimMedium), 0.001)
		})
	}
}

func TestComputeReductionPct_DurationAxis(t *testing.T) {
	// 32 °C load, medium acclimatization → base 9.
	assert.InDelta(t, 4.5, ComputeReductionPct(33, 30, AcclimMedium), 0.001)   // short: ×0.5
	assert.InDelta(t, 9.0, ComputeReductionPct(33, 90, AcclimMedium), 0.001)   // moderate: ×1.0
	assert.InDelta(t, 12.6, ComputeReductionPct(33, 200, AcclimMedium), 0.001) // long: ×1.4

	// Band edges.
	assert.InDelta(t, 4.5, ComputeReductionPct(33, 44.9, AcclimMedium), 0.001)
	assert.InDelta(t, 9.0, ComputeReductionPct(33, 45, AcclimMedium), 0.001)
	assert.InDelta(t, 9.0, ComputeReductionPct(33, 150, AcclimMedium), 0.001)
	assert.InDelta(t, 12.6, ComputeReductionPct(33, 150.1, AcclimMedium), 0.001)
}

func TestComputeReductionPct_AcclimatizationAxis(t *testing.T) {
	// Same day, same session — adaptation is what changes the answer.
	good := ComputeReductionPct(33, 90, AcclimGood)
	medium := ComputeReductionPct(33, 90, AcclimMedium)
	low := ComputeReductionPct(33, 90, AcclimLow)

	assert.InDelta(t, 5.4, good, 0.001)   // 9 × 0.6
	assert.InDelta(t, 9.0, medium, 0.001) // 9 × 1.0
	assert.InDelta(t, 11.7, low, 0.001)   // 9 × 1.3
	assert.Less(t, good, medium)
	assert.Less(t, medium, low)
}

// A cool day is cool no matter how long or unadapted — the zero base must not
// be multiplied into a phantom penalty.
func TestComputeReductionPct_CoolDayStaysZero(t *testing.T) {
	for _, dur := range []float64{20, 90, 400} {
		for _, a := range []Acclimatization{AcclimLow, AcclimMedium, AcclimGood} {
			assert.Zero(t, ComputeReductionPct(20, dur, a), "dur=%v acclim=%v", dur, a)
		}
	}
}

func TestComputeReductionPct_WorstCase(t *testing.T) {
	// Brutal day, long session, unadapted: 14 × 1.4 × 1.3 = 25.48 → 25.5.
	assert.InDelta(t, 25.5, ComputeReductionPct(38, 240, AcclimLow), 0.001)
}

// ============================================================================
// fluid
// ============================================================================

func TestComputeFluid_PersonalSignalScales(t *testing.T) {
	got := ComputeFluid(30, &SweatSignal{MlPerHour: 1200, Source: SourceGarminSweatLoss, Sessions: 3})

	assert.Equal(t, SourceGarminSweatLoss, got.Source)
	// 1 + (30−24)×0.03 = 1.18 → 1416
	assert.InDelta(t, 1416, got.MlPerHour, 0.5)
	assert.Contains(t, got.Note, "3 recent session")
	assert.Contains(t, got.Note, "device estimate, not a field test",
		"a device estimate must never read as a measured field test")
}

func TestComputeFluid_GenericDefaultIsFlagged(t *testing.T) {
	got := ComputeFluid(30, nil)

	assert.Equal(t, SourceGenericDefault, got.Source, "a default must never pass as personal")
	assert.Contains(t, got.Note, "generic")
	assert.InDelta(t, 708, got.MlPerHour, 0.5) // 600 × 1.18
}

func TestComputeFluid_MultiplierCapped(t *testing.T) {
	// A 45 °C load would otherwise scale by 1.63 — undrinkable.
	got := ComputeFluid(45, &SweatSignal{MlPerHour: 1000, Source: SourceGarminSweatLoss, Sessions: 1})
	assert.InDelta(t, 1500, got.MlPerHour, 0.5, "capped at 1.5×")
}

func TestComputeFluid_CoolDayNoScaling(t *testing.T) {
	got := ComputeFluid(20, &SweatSignal{MlPerHour: 1000, Source: SourceGarminSweatLoss, Sessions: 1})
	assert.InDelta(t, 1000, got.MlPerHour, 0.001)
}

func TestComputeFluid_NonPositiveSignalFallsBackToGeneric(t *testing.T) {
	// A zero/negative rate is not data.
	got := ComputeFluid(30, &SweatSignal{MlPerHour: 0, Source: SourceGarminSweatLoss})
	assert.Equal(t, SourceGenericDefault, got.Source)
}
