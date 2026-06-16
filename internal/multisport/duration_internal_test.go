package multisport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

func durTimeStep(sec int) wt.Step {
	return wt.Step{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(sec)}}
}

func transSec(sec int) Segment {
	return Segment{Sport: SportTransition, Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(sec)}}
}

func TestEstimatedDurationSec_FullyTimeBounded(t *testing.T) {
	segs := []Segment{
		{Sport: "swim", Steps: []wt.Step{durTimeStep(600)}},
		transSec(60),
		{Sport: "bike", Steps: []wt.Step{
			{Type: wt.NodeRepeat, Count: 3, Steps: []wt.Step{durTimeStep(120), durTimeStep(60)}}, // 3*(180)=540
			durTimeStep(300),
		}},
		transSec(45),
		{Sport: "run", Steps: []wt.Step{durTimeStep(1500)}},
	}
	got := estimatedDurationSec(segs)
	require.NotNil(t, got)
	// 600 + 60 + (540+300) + 45 + 1500
	assert.Equal(t, 600+60+840+45+1500, *got)
}

func TestEstimatedDurationSec_NonTimeStepYieldsNil(t *testing.T) {
	for name, seg := range map[string]Segment{
		"distance": {Sport: "swim", Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: ptrInt(750)}}}},
		"open":     {Sport: "run", Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationOpen}}}},
		"lap":      {Sport: "bike", Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationLapButton}}}},
	} {
		t.Run(name, func(t *testing.T) {
			segs := []Segment{seg, {Sport: "bike", Steps: []wt.Step{durTimeStep(600)}}}
			assert.Nil(t, estimatedDurationSec(segs))
		})
	}
}

func TestEstimatedDurationSec_NonTimeTransitionYieldsNil(t *testing.T) {
	segs := []Segment{
		{Sport: "swim", Steps: []wt.Step{durTimeStep(600)}},
		transition(), // helper from multisport_test.go: a lap_button transition → not determinable
		{Sport: "run", Steps: []wt.Step{durTimeStep(600)}},
	}
	assert.Nil(t, estimatedDurationSec(segs))
}

func TestEstimatedDurationSec_NestedRepeatTimed(t *testing.T) {
	segs := []Segment{
		{Sport: "bike", Steps: []wt.Step{durTimeStep(600)}},
		{Sport: "run", Steps: []wt.Step{{Type: wt.NodeRepeat, Count: 4, Steps: []wt.Step{durTimeStep(30), durTimeStep(30)}}}}, // 4*60=240
	}
	got := estimatedDurationSec(segs)
	require.NotNil(t, got)
	assert.Equal(t, 600+240, *got)
}
