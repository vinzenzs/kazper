package fuelplan

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
)

func p(v float64) *float64 { return &v }

// session builds a planned session with an optional TSS and duration.
func session(tss, mins *float64) Session {
	return Session{WorkoutID: uuid.New(), Sport: "bike", PlannedTSS: tss, PlannedDurationMin: mins}
}

func TestClassify_TierThresholds(t *testing.T) {
	cases := []struct {
		name  string
		tss   *float64
		want  Tier
		gPerK float64
	}{
		{"just under easy ceiling", p(59.9), TierEasy, 5},
		{"zero-TSS session is still training", p(0), TierEasy, 5},
		{"exactly 60 is moderate", p(60), TierModerate, 7},
		{"mid moderate", p(100), TierModerate, 7},
		{"exactly 150 is still moderate", p(150), TierModerate, 7},
		{"just over 150 is heavy", p(150.1), TierHeavy, 9},
		{"deep heavy", p(300), TierHeavy, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, total := classify([]Session{session(tc.tss, p(60))})
			assert.Equal(t, tc.want, tier)
			assert.InDelta(t, *tc.tss, total, 0.001)
			assert.Equal(t, tc.gPerK, carbsGPerKg(tier))
		})
	}
}

func TestClassify_NoSessionsIsRest(t *testing.T) {
	tier, total := classify(nil)
	assert.Equal(t, TierRest, tier)
	assert.Zero(t, total)
	assert.Equal(t, 3.0, carbsGPerKg(tier))

	tier, _ = classify([]Session{})
	assert.Equal(t, TierRest, tier)
}

func TestClassify_LongSessionIsHeavyRegardlessOfTSS(t *testing.T) {
	// 160 minutes at only 90 TSS would classify moderate on TSS alone.
	tier, total := classify([]Session{session(p(90), p(160))})
	assert.Equal(t, TierHeavy, tier)
	assert.InDelta(t, 90, total, 0.001, "the TSS that didn't earn heavy is still reported")

	// The rule fires exactly at the 150-minute mark, not below it.
	at, _ := classify([]Session{session(p(10), p(150))})
	assert.Equal(t, TierHeavy, at)
	under, _ := classify([]Session{session(p(10), p(149.9))})
	assert.Equal(t, TierEasy, under)
}

func TestClassify_MultipleSessionsSumTSS(t *testing.T) {
	// Two easy sessions that individually wouldn't clear the bar together do.
	tier, total := classify([]Session{
		session(p(40), p(45)),
		session(p(45), p(50)),
	})
	assert.Equal(t, TierModerate, tier)
	assert.InDelta(t, 85, total, 0.001)

	// A long session anywhere in the day still wins.
	tier, total = classify([]Session{
		session(p(20), p(30)),
		session(p(30), p(155)),
	})
	assert.Equal(t, TierHeavy, tier)
	assert.InDelta(t, 50, total, 0.001)
}

func TestClassify_SessionWithNeitherTSSNorDurationIsEasy(t *testing.T) {
	// A template without estimates still means the athlete trains that day —
	// easy is the honest floor, and the echoed session shows why.
	tier, total := classify([]Session{{WorkoutID: uuid.New(), Sport: "run"}})
	assert.Equal(t, TierEasy, tier)
	assert.Zero(t, total)
}

func TestBuildDay_SuggestionAndDelta(t *testing.T) {
	// Heavy day at 70 kg → 9 × 70 = 630 g.
	goal := goals.Range{Min: p(400), Max: p(500)}
	d := buildDay("2026-07-20", []Session{session(p(180), p(120))}, false, p(70), &goal)

	assert.Equal(t, TierHeavy, d.Tier)
	assert.Equal(t, 9.0, d.CarbsGPerKg)
	require.NotNil(t, d.SuggestedCarbsG)
	assert.InDelta(t, 630, *d.SuggestedCarbsG, 0.001)
	// Delta is measured against the goal midpoint (450), matching summary's
	// targetReference rule.
	require.NotNil(t, d.DeltaG)
	assert.InDelta(t, 180, *d.DeltaG, 0.001)
	assert.Equal(t, &goal, d.GoalCarbsG)
	assert.False(t, d.PlanMissing)
}

func TestBuildDay_DeltaAgainstSingleBoundGoal(t *testing.T) {
	minOnly := goals.Range{Min: p(500)}
	d := buildDay("2026-07-20", []Session{session(p(180), nil)}, false, p(70), &minOnly)
	require.NotNil(t, d.DeltaG)
	assert.InDelta(t, 130, *d.DeltaG, 0.001)

	maxOnly := goals.Range{Max: p(700)}
	d = buildDay("2026-07-20", []Session{session(p(180), nil)}, false, p(70), &maxOnly)
	require.NotNil(t, d.DeltaG)
	assert.InDelta(t, -70, *d.DeltaG, 0.001)
}

func TestBuildDay_NoGoalMeansNoDelta(t *testing.T) {
	d := buildDay("2026-07-20", []Session{session(p(180), nil)}, false, p(70), nil)
	require.NotNil(t, d.SuggestedCarbsG, "a suggestion needs no goal to exist")
	assert.Nil(t, d.DeltaG)
	assert.Nil(t, d.GoalCarbsG)

	// An empty goal range is present but anchors nothing.
	empty := goals.Range{}
	d = buildDay("2026-07-20", []Session{session(p(180), nil)}, false, p(70), &empty)
	assert.Nil(t, d.DeltaG)
	assert.NotNil(t, d.GoalCarbsG)
}

func TestBuildDay_WeightMissingKeepsTierDropsGrams(t *testing.T) {
	goal := goals.Range{Min: p(400), Max: p(500)}
	d := buildDay("2026-07-20", []Session{session(p(180), nil)}, false, nil, &goal)

	assert.Equal(t, TierHeavy, d.Tier, "tiers are weight-free")
	assert.Equal(t, 9.0, d.CarbsGPerKg, "the g/kg prescription still ships")
	assert.Nil(t, d.SuggestedCarbsG, "only the multiplication needs mass")
	assert.Nil(t, d.DeltaG, "no suggestion, nothing to compare")
	assert.NotNil(t, d.GoalCarbsG)
}

func TestBuildDay_RestVsPlanMissing(t *testing.T) {
	// Both classify rest, but they must be distinguishable.
	rest := buildDay("2026-07-20", nil, false, p(70), nil)
	assert.Equal(t, TierRest, rest.Tier)
	assert.False(t, rest.PlanMissing)
	require.NotNil(t, rest.SuggestedCarbsG)
	assert.InDelta(t, 210, *rest.SuggestedCarbsG, 0.001) // 3 × 70

	missing := buildDay("2026-07-27", nil, true, p(70), nil)
	assert.Equal(t, TierRest, missing.Tier)
	assert.True(t, missing.PlanMissing)
}

func TestBuildDay_SessionsSerializeAsEmptyNotNull(t *testing.T) {
	d := buildDay("2026-07-20", nil, false, p(70), nil)
	assert.NotNil(t, d.Sessions)
	assert.Empty(t, d.Sessions)
}

func TestBuildDay_RoundsAtBoundary(t *testing.T) {
	// 7 × 71.43 = 500.01 → 500.0
	goal := goals.Range{Min: p(333.33), Max: p(444.44)}
	d := buildDay("2026-07-20", []Session{session(p(100), nil)}, false, p(71.43), &goal)

	require.NotNil(t, d.SuggestedCarbsG)
	assert.Equal(t, 500.0, *d.SuggestedCarbsG)
	// Delta uses the UNROUNDED suggestion (500.01) against the midpoint
	// (388.885) → 111.125 → 111.1, not a re-rounded 111.2.
	require.NotNil(t, d.DeltaG)
	assert.Equal(t, 111.1, *d.DeltaG)
}

func TestCarbsGPerKg_FullLadder(t *testing.T) {
	assert.Equal(t, 3.0, carbsGPerKg(TierRest))
	assert.Equal(t, 5.0, carbsGPerKg(TierEasy))
	assert.Equal(t, 7.0, carbsGPerKg(TierModerate))
	assert.Equal(t, 9.0, carbsGPerKg(TierHeavy))
}
