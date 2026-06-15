package trainingplan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

// zonedConfig is an athlete config with a full power-zone and HR-zone ladder.
func zonedConfig() *athleteconfig.AthleteConfig {
	return &athleteconfig.AthleteConfig{
		PowerZone1Max: intp(140), PowerZone2Max: intp(190), PowerZone3Max: intp(230),
		PowerZone4Max: intp(268), PowerZone5Max: intp(320),
		HRZone1Max: intp(120), HRZone2Max: intp(142), HRZone3Max: intp(167),
		HRZone4Max: intp(178), HRZone5Max: intp(190),
	}
}

func zoneTarget(kind string, lo, hi int) *wt.Target {
	return &wt.Target{Kind: kind, Low: intp(lo), High: intp(hi)}
}

func TestResolveTargets_SingleZonePower(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: zoneTarget(wt.TargetPowerZone, 4, 4)}}
	out := resolveTargets(steps, zonedConfig(), wt.SportBike)
	tg := out[0].Target
	assert.Equal(t, wt.TargetPowerW, tg.Kind)
	assert.Equal(t, 230, *tg.Low)  // PowerZone3Max
	assert.Equal(t, 268, *tg.High) // PowerZone4Max
	assert.Equal(t, "Z4", tg.Origin)
	// original slice not mutated
	assert.Equal(t, wt.TargetPowerZone, steps[0].Target.Kind)
}

func TestResolveTargets_SingleZoneHR(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Target: zoneTarget(wt.TargetHRZone, 4, 4)}}
	out := resolveTargets(steps, zonedConfig(), wt.SportRun)
	tg := out[0].Target
	assert.Equal(t, wt.TargetHRBpm, tg.Kind)
	assert.Equal(t, 167, *tg.Low)  // HRZone3Max
	assert.Equal(t, 178, *tg.High) // HRZone4Max
	assert.Equal(t, "Z4", tg.Origin)
}

func TestResolveTargets_SecondaryZoneResolved(t *testing.T) {
	// A bike step with primary power_zone + secondary hr_zone: both zone-kind
	// targets resolve to absolutes, the secondary using the same rules.
	steps := []wt.Step{{
		Type:            wt.NodeStep,
		Intent:          wt.IntentInterval,
		Target:          zoneTarget(wt.TargetPowerZone, 4, 4),
		SecondaryTarget: zoneTarget(wt.TargetHRZone, 3, 3),
	}}
	out := resolveTargets(steps, zonedConfig(), wt.SportBike)
	primary := out[0].Target
	assert.Equal(t, wt.TargetPowerW, primary.Kind)
	assert.Equal(t, 268, *primary.High) // PowerZone4Max
	secondary := out[0].SecondaryTarget
	require.NotNil(t, secondary)
	assert.Equal(t, wt.TargetHRBpm, secondary.Kind)
	assert.Equal(t, 142, *secondary.Low)  // HRZone2Max
	assert.Equal(t, 167, *secondary.High) // HRZone3Max
	assert.Equal(t, "Z3", secondary.Origin)
	// original slice not mutated
	assert.Equal(t, wt.TargetHRZone, steps[0].SecondaryTarget.Kind)
}

func TestResolveTargets_MultiZoneBand(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Target: zoneTarget(wt.TargetPowerZone, 2, 4)}}
	out := resolveTargets(steps, zonedConfig(), wt.SportBike)
	tg := out[0].Target
	assert.Equal(t, wt.TargetPowerW, tg.Kind)
	assert.Equal(t, 140, *tg.Low)  // PowerZone1Max
	assert.Equal(t, 268, *tg.High) // PowerZone4Max
	assert.Equal(t, "Z2–Z4", tg.Origin)
}

func TestResolveTargets_ZoneOneFloorIsZero(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentRecovery, Target: zoneTarget(wt.TargetHRZone, 1, 1)}}
	out := resolveTargets(steps, zonedConfig(), wt.SportRun)
	tg := out[0].Target
	assert.Equal(t, wt.TargetHRBpm, tg.Kind)
	assert.Equal(t, 0, *tg.Low)    // zone-1 lower edge
	assert.Equal(t, 120, *tg.High) // HRZone1Max
	assert.Equal(t, "Z1", tg.Origin)
}

func TestResolveTargets_NestedInRepeat(t *testing.T) {
	steps := []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentWarmup, Target: zoneTarget(wt.TargetHRZone, 2, 2)},
		{Type: wt.NodeRepeat, Count: 5, Steps: []wt.Step{
			{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: zoneTarget(wt.TargetPowerZone, 5, 5)},
			{Type: wt.NodeStep, Intent: wt.IntentRecovery, Target: zoneTarget(wt.TargetHRZone, 1, 1)},
		}},
	}
	out := resolveTargets(steps, zonedConfig(), wt.SportBike)

	// nested power-zone interval resolved to watts
	interval := out[1].Steps[0].Target
	assert.Equal(t, wt.TargetPowerW, interval.Kind)
	assert.Equal(t, 268, *interval.Low)  // PowerZone4Max
	assert.Equal(t, 320, *interval.High) // PowerZone5Max
	// nested recovery hr-zone resolved to bpm
	assert.Equal(t, wt.TargetHRBpm, out[1].Steps[1].Target.Kind)
	// repeat structure preserved
	assert.Equal(t, 5, out[1].Count)
	// original not mutated
	assert.Equal(t, wt.TargetPowerZone, steps[1].Steps[0].Target.Kind)
}

func TestResolveTargets_RunPowerZonePassesThrough(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: zoneTarget(wt.TargetPowerZone, 4, 4)}}
	out := resolveTargets(steps, zonedConfig(), wt.SportRun)
	tg := out[0].Target
	// D7: power zones are bike-only; a run power_zone is left unchanged
	assert.Equal(t, wt.TargetPowerZone, tg.Kind)
	assert.Equal(t, 4, *tg.Low)
	assert.Equal(t, 4, *tg.High)
	assert.Empty(t, tg.Origin)
}

func TestResolveTargets_PassthroughKinds(t *testing.T) {
	steps := []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive, Target: func() *wt.Target { p := paceTarget(435, 435); return &p }()},
		{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: &wt.Target{Kind: wt.TargetPowerW, Low: intp(230), High: intp(268)}},
		{Type: wt.NodeStep, Intent: wt.IntentActive, Target: &wt.Target{Kind: wt.TargetHRBpm, Low: intp(150), High: intp(160)}},
		{Type: wt.NodeStep, Intent: wt.IntentActive, Target: &wt.Target{Kind: wt.TargetRPE, Low: intp(6), High: intp(7)}},
		{Type: wt.NodeStep, Intent: wt.IntentRest, Target: &wt.Target{Kind: wt.TargetNone}},
	}
	out := resolveTargets(steps, zonedConfig(), wt.SportBike)
	assert.Equal(t, wt.TargetPace, out[0].Target.Kind)
	assert.Equal(t, wt.TargetPowerW, out[1].Target.Kind)
	assert.Empty(t, out[1].Target.Origin) // not a resolution — no origin stamped
	assert.Equal(t, wt.TargetHRBpm, out[2].Target.Kind)
	assert.Empty(t, out[2].Target.Origin)
	assert.Equal(t, wt.TargetRPE, out[3].Target.Kind)
	assert.Equal(t, wt.TargetNone, out[4].Target.Kind)
}

func TestResolveTargets_NilConfigPassesThrough(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: zoneTarget(wt.TargetPowerZone, 4, 4)}}
	out := resolveTargets(steps, nil, wt.SportBike)
	assert.Equal(t, wt.TargetPowerZone, out[0].Target.Kind)
}

func TestResolveTargets_UnsetBoundaryPassesThrough(t *testing.T) {
	cfg := &athleteconfig.AthleteConfig{PowerZone4Max: intp(268)} // PowerZone3Max unset
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: zoneTarget(wt.TargetPowerZone, 4, 4)}}
	out := resolveTargets(steps, cfg, wt.SportBike)
	tg := out[0].Target
	require.Equal(t, wt.TargetPowerZone, tg.Kind) // lower edge (PowerZone3Max) missing → passthrough
	assert.Empty(t, tg.Origin)
}
