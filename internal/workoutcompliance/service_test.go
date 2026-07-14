package workoutcompliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

func iptr(i int) *int         { return &i }
func fptr(f float64) *float64 { return &f }

// --- builders -------------------------------------------------------------

func single(intent string, d *workouttemplates.Duration, t *workouttemplates.Target) workouttemplates.Step {
	return workouttemplates.Step{Type: workouttemplates.NodeStep, Intent: intent, Duration: d, Target: t}
}

func repeat(count int, inner ...workouttemplates.Step) workouttemplates.Step {
	return workouttemplates.Step{Type: workouttemplates.NodeRepeat, Count: count, Steps: inner}
}

func timeDur(s int) *workouttemplates.Duration {
	return &workouttemplates.Duration{Kind: workouttemplates.DurationTime, Seconds: iptr(s)}
}
func distDur(m int) *workouttemplates.Duration {
	return &workouttemplates.Duration{Kind: workouttemplates.DurationDistance, Meters: iptr(m)}
}
func lapButton() *workouttemplates.Duration {
	return &workouttemplates.Duration{Kind: workouttemplates.DurationLapButton}
}

func powerW(lo, hi int) *workouttemplates.Target {
	return &workouttemplates.Target{Kind: workouttemplates.TargetPowerW, Low: iptr(lo), High: iptr(hi)}
}
func hrBpm(lo, hi int) *workouttemplates.Target {
	return &workouttemplates.Target{Kind: workouttemplates.TargetHRBpm, Low: iptr(lo), High: iptr(hi)}
}
func paceT(lo, hi int) *workouttemplates.Target {
	return &workouttemplates.Target{Kind: workouttemplates.TargetPace, LowSecPerKM: iptr(lo), HighSecPerKM: iptr(hi)}
}
func swimPaceT(lo, hi int) *workouttemplates.Target {
	return &workouttemplates.Target{Kind: workouttemplates.TargetSwimPace, LowSecPer100m: iptr(lo), HighSecPer100m: iptr(hi)}
}

func splitPower(idx, watts, durS int) workouts.Split {
	return workouts.Split{SplitIndex: idx, AvgPowerW: iptr(watts), DurationS: fptr(float64(durS))}
}

// --- expansion ------------------------------------------------------------

func TestExpand_Singles(t *testing.T) {
	got := expand([]workouttemplates.Step{
		single("warmup", timeDur(600), nil),
		single("cooldown", timeDur(300), nil),
	})
	require.Len(t, got, 2)
	assert.Nil(t, got[0].iteration)
	assert.Equal(t, "warmup", got[0].step.Intent)
}

func TestExpand_RepeatOrderingAndProvenance(t *testing.T) {
	got := expand([]workouttemplates.Step{
		single("warmup", timeDur(600), nil),
		repeat(5, single("interval", timeDur(180), powerW(250, 265)), single("recovery", timeDur(120), nil)),
		single("cooldown", timeDur(300), nil),
	})
	// 1 + 5*2 + 1 = 12
	require.Len(t, got, 12)
	assert.Equal(t, "warmup", got[0].step.Intent)
	assert.Equal(t, "cooldown", got[11].step.Intent)

	// The third interval (iteration 3 of 5) is at flat index 1 + (3-1)*2 = 5.
	third := got[5]
	assert.Equal(t, "interval", third.step.Intent)
	require.NotNil(t, third.iteration)
	assert.Equal(t, 3, *third.iteration)
	require.NotNil(t, third.of)
	assert.Equal(t, 5, *third.of)
}

func TestExpand_LengthProperty(t *testing.T) {
	steps := []workouttemplates.Step{
		single("warmup", timeDur(600), nil),
		repeat(3, single("interval", timeDur(60), nil), single("recovery", timeDur(60), nil), single("active", timeDur(60), nil)),
		repeat(2, single("interval", timeDur(30), nil)),
		single("cooldown", timeDur(300), nil),
	}
	// singles: 2; repeats: 3*3 + 2*1 = 11; total 13
	assert.Len(t, expand(steps), 13)
}

// --- target scoring -------------------------------------------------------

func TestScoreTarget_PowerUnder(t *testing.T) {
	tr := scoreTarget(powerW(250, 265), splitPower(0, 230, 180))
	require.True(t, tr.Scorable)
	assert.Equal(t, "power_w", tr.Metric)
	assert.Equal(t, ClassUnder, tr.Classification)
	require.NotNil(t, tr.Delta)
	assert.InDelta(t, -20, *tr.Delta, 1e-9) // 230 - 250
	require.NotNil(t, tr.DeviationPct)
	assert.InDelta(t, 20.0/250.0, *tr.DeviationPct, 1e-9)
}

func TestScoreTarget_PowerOverAndInBand(t *testing.T) {
	over := scoreTarget(powerW(250, 265), splitPower(0, 300, 180))
	assert.Equal(t, ClassOver, over.Classification)
	assert.InDelta(t, 35, *over.Delta, 1e-9) // 300 - 265

	in := scoreTarget(powerW(250, 265), splitPower(0, 257, 180))
	assert.Equal(t, ClassInBand, in.Classification)
	assert.InDelta(t, 0, *in.Delta, 1e-9)
	assert.InDelta(t, 100, *in.Score, 1e-9)
}

func TestScoreTarget_PaceFromSpeed(t *testing.T) {
	// avg 3.5 m/s → 1000/3.5 = 285.7 sec/km; target 270–285 → over (slower).
	s := workouts.Split{SplitIndex: 0, AvgSpeedMPS: fptr(3.5), DurationS: fptr(300)}
	tr := scoreTarget(paceT(270, 285), s)
	require.True(t, tr.Scorable)
	assert.Equal(t, "pace", tr.Metric)
	assert.InDelta(t, 285.71, *tr.Actual, 0.01)
	assert.Equal(t, ClassOver, tr.Classification)
}

func TestScoreTarget_SwimPaceFromSpeed(t *testing.T) {
	// avg 1.25 m/s → 100/1.25 = 80 sec/100m; target 85–95 → under (faster).
	s := workouts.Split{SplitIndex: 0, AvgSpeedMPS: fptr(1.25), DurationS: fptr(120)}
	tr := scoreTarget(swimPaceT(85, 95), s)
	require.True(t, tr.Scorable)
	assert.Equal(t, "swim_pace", tr.Metric)
	assert.InDelta(t, 80, *tr.Actual, 1e-9)
	assert.Equal(t, ClassUnder, tr.Classification)
}

func TestScoreTarget_UnscorableKinds(t *testing.T) {
	cases := []struct {
		name   string
		target *workouttemplates.Target
		reason string
	}{
		{"unresolved hr_zone", &workouttemplates.Target{Kind: workouttemplates.TargetHRZone, Low: iptr(4), High: iptr(4)}, ReasonZoneUnresolved},
		{"unresolved power_zone", &workouttemplates.Target{Kind: workouttemplates.TargetPowerZone, Low: iptr(3), High: iptr(3)}, ReasonZoneUnresolved},
		{"cadence", &workouttemplates.Target{Kind: workouttemplates.TargetCadence, Low: iptr(85), High: iptr(95)}, ReasonUnsupportedKind},
		{"rpe", &workouttemplates.Target{Kind: workouttemplates.TargetRPE, Low: iptr(6), High: iptr(7)}, ReasonUnsupportedKind},
		{"none", &workouttemplates.Target{Kind: workouttemplates.TargetNone}, ReasonNoTarget},
		{"nil", nil, ReasonNoTarget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := scoreTarget(tc.target, splitPower(0, 200, 180))
			require.False(t, tr.Scorable)
			require.NotNil(t, tr.Reason)
			assert.Equal(t, tc.reason, *tr.Reason)
		})
	}
}

func TestScoreTarget_HRBpm(t *testing.T) {
	// HR target 150–160 bpm, lap avg 155 → in band, score 100.
	s := workouts.Split{SplitIndex: 0, AvgHR: iptr(155), DurationS: fptr(300)}
	tr := scoreTarget(hrBpm(150, 160), s)
	require.True(t, tr.Scorable)
	assert.Equal(t, "hr_bpm", tr.Metric)
	assert.Equal(t, ClassInBand, tr.Classification)
	assert.InDelta(t, 100, *tr.Score, 1e-9)
}

func TestScoreTarget_ActualMissing(t *testing.T) {
	// power target but the lap has no power meter (AvgPowerW nil).
	s := workouts.Split{SplitIndex: 0, DurationS: fptr(180)}
	tr := scoreTarget(powerW(250, 265), s)
	require.False(t, tr.Scorable)
	require.NotNil(t, tr.Reason)
	assert.Equal(t, ReasonActualMissing, *tr.Reason)
}

func TestTargetScore_Falloff(t *testing.T) {
	assert.InDelta(t, 100, targetScore(0), 1e-9)    // in band
	assert.InDelta(t, 50, targetScore(0.125), 1e-9) // halfway to zero
	assert.InDelta(t, 0, targetScore(0.25), 1e-9)   // zero at 25%
	assert.InDelta(t, 0, targetScore(0.40), 1e-9)   // clamped, never negative
}

// --- duration scoring -----------------------------------------------------

func TestScoreDuration_Ratio(t *testing.T) {
	// planned 180 s, actual 178 s → ratio ≈ 0.989, in band, score 100.
	dr := scoreDuration(timeDur(180), workouts.Split{SplitIndex: 0, DurationS: fptr(178)})
	require.NotNil(t, dr)
	assert.Equal(t, workouttemplates.DurationTime, dr.Kind)
	require.NotNil(t, dr.Ratio)
	assert.InDelta(t, 0.989, *dr.Ratio, 0.001)
	assert.Equal(t, ClassInBand, dr.Classification)
	assert.InDelta(t, 100, *dr.Score, 1e-9)
}

func TestScoreDuration_OverAndUnder(t *testing.T) {
	over := scoreDuration(timeDur(100), workouts.Split{DurationS: fptr(130)}) // ratio 1.3 → over
	assert.Equal(t, ClassOver, over.Classification)
	assert.InDelta(t, 0, *over.Score, 1e-9) // dev 0.30 > 0.25 → 0

	under := scoreDuration(timeDur(100), workouts.Split{DurationS: fptr(80)}) // ratio 0.8 → under
	assert.Equal(t, ClassUnder, under.Classification)
	// dev 0.20: 100*(1 - (0.20-0.10)/0.15) = 100*(1-0.6667) = 33.3
	assert.InDelta(t, 33.33, *under.Score, 0.1)
}

func TestScoreDuration_Distance(t *testing.T) {
	dr := scoreDuration(distDur(1000), workouts.Split{DistanceM: fptr(1000)})
	require.NotNil(t, dr)
	assert.Equal(t, workouttemplates.DurationDistance, dr.Kind)
	assert.InDelta(t, 1.0, *dr.Ratio, 1e-9)
}

func TestScoreDuration_LapButtonAndOpenSkipped(t *testing.T) {
	assert.Nil(t, scoreDuration(lapButton(), workouts.Split{DurationS: fptr(300)}))
	assert.Nil(t, scoreDuration(&workouttemplates.Duration{Kind: workouttemplates.DurationOpen}, workouts.Split{DurationS: fptr(300)}))
}

// --- overall weighting (via Compliance with fakes) ------------------------

type fakeWorkouts struct{ w *workouts.Workout }

func (f fakeWorkouts) GetByIDWithChildren(_ context.Context, _ uuid.UUID) (*workouts.Workout, error) {
	return f.w, nil
}

type fakeProgram struct{ p *trainingplan.Program }

func (f fakeProgram) EffectiveProgram(_ context.Context, _ uuid.UUID) (*trainingplan.Program, error) {
	return f.p, nil
}

func TestCompliance_OverallWeightedByPlannedDuration(t *testing.T) {
	id := uuid.New()
	tid := uuid.New()
	// Step A: 600 s tempo scored 60 (10% under band → target score 60);
	// Step B: 60 s recovery scored 100 (in band). lap_button so only the target
	// dimension scores; weight is the actual lap duration.
	stepA := single("active", lapButton(), powerW(250, 265))
	stepB := single("recovery", lapButton(), powerW(250, 265))
	prog := &trainingplan.Program{WorkoutID: id, Sport: "bike", Steps: []workouttemplates.Step{stepA, stepB}}
	w := &workouts.Workout{
		ID: id, Sport: workouts.SportBike, Status: workouts.StatusCompleted, TemplateID: &tid,
		Splits: []workouts.Split{
			splitPower(0, 225, 600), // 10% under 250 → score 60
			splitPower(1, 257, 60),  // in band → score 100
		},
	}
	svc := NewService(fakeWorkouts{w}, fakeProgram{prog})
	res, err := svc.Compliance(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, StatusScored, res.Status)
	require.NotNil(t, res.Score)
	// (600*60 + 60*100) / 660 = 63.63
	assert.InDelta(t, 63.63, *res.Score, 0.05)
	assert.Equal(t, 2, res.StepsScored)
	assert.Equal(t, 1, res.StepsInBand) // only step B in band
}

func TestCompliance_NullScoreWhenNothingScorable(t *testing.T) {
	id := uuid.New()
	tid := uuid.New()
	// All-RPE targets + lap_button (no scorable duration) → nothing scorable.
	rpe := &workouttemplates.Target{Kind: workouttemplates.TargetRPE, Low: iptr(6), High: iptr(7)}
	prog := &trainingplan.Program{WorkoutID: id, Sport: "run", Steps: []workouttemplates.Step{
		single("active", lapButton(), rpe),
		single("active", lapButton(), rpe),
	}}
	w := &workouts.Workout{
		ID: id, Sport: workouts.SportRun, Status: workouts.StatusCompleted, TemplateID: &tid,
		Splits: []workouts.Split{{SplitIndex: 0, DurationS: fptr(300)}, {SplitIndex: 1, DurationS: fptr(300)}},
	}
	res, err := NewService(fakeWorkouts{w}, fakeProgram{prog}).Compliance(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusScored, res.Status)
	assert.Nil(t, res.Score)
	assert.Equal(t, 0, res.StepsScored)
	require.Len(t, res.Steps, 2)
}

func TestCompliance_LapCountMismatchUnavailable(t *testing.T) {
	id := uuid.New()
	tid := uuid.New()
	prog := &trainingplan.Program{WorkoutID: id, Sport: "bike", Steps: []workouttemplates.Step{
		single("warmup", timeDur(600), nil),
		repeat(5, single("interval", timeDur(180), powerW(250, 265)), single("recovery", timeDur(120), nil)),
		single("cooldown", timeDur(300), nil),
	}} // 12 expanded steps
	w := &workouts.Workout{
		ID: id, Sport: workouts.SportBike, Status: workouts.StatusCompleted, TemplateID: &tid,
		Splits: make([]workouts.Split, 9), // only 9 laps
	}
	res, err := NewService(fakeWorkouts{w}, fakeProgram{prog}).Compliance(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, StatusUnavailable, res.Status)
	require.NotNil(t, res.Reason)
	assert.Equal(t, "lap_count_mismatch", *res.Reason)
	assert.Equal(t, 12, res.PlannedSteps)
	assert.Equal(t, 9, res.ExecutedLaps)
	assert.Nil(t, res.Steps)
}

func TestCompliance_PrerequisiteErrors(t *testing.T) {
	id := uuid.New()
	tid := uuid.New()
	base := func(mut func(*workouts.Workout)) *workouts.Workout {
		w := &workouts.Workout{ID: id, Sport: workouts.SportBike, Status: workouts.StatusCompleted, TemplateID: &tid, Splits: []workouts.Split{{}}}
		mut(w)
		return w
	}
	prog := &trainingplan.Program{WorkoutID: id, Sport: "bike"}

	cases := []struct {
		name string
		w    *workouts.Workout
		want error
	}{
		{"planned", base(func(w *workouts.Workout) { w.Status = workouts.StatusPlanned }), ErrNotCompleted},
		{"multisport", base(func(w *workouts.Workout) { w.Sport = workouts.SportMultisport }), ErrMultisportUnsupported},
		{"no template", base(func(w *workouts.Workout) { w.TemplateID = nil }), ErrNoTemplateLink},
		{"no splits", base(func(w *workouts.Workout) { w.Splits = nil }), ErrSplitsMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(fakeWorkouts{tc.w}, fakeProgram{prog}).Compliance(context.Background(), id)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// Multisport check precedes the template-link check: a brick with no template
// reports multisport_unsupported, not no_template_link.
func TestCompliance_MultisportBeatsNoTemplate(t *testing.T) {
	id := uuid.New()
	w := &workouts.Workout{ID: id, Sport: workouts.SportMultisport, Status: workouts.StatusCompleted, TemplateID: nil}
	_, err := NewService(fakeWorkouts{w}, fakeProgram{&trainingplan.Program{}}).Compliance(context.Background(), id)
	assert.ErrorIs(t, err, ErrMultisportUnsupported)
}
