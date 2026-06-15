package trainingplan

import (
	"fmt"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// resolveTargets is the effective-program target-resolution pass: it walks the
// step tree (recursing one level into repeat groups) and rewrites zone-reference
// targets into absolute power_w/hr_bpm ranges using the athlete-config singleton.
//
// It is best-effort and never errors: a nil config, a sport that doesn't take a
// given zone kind (power zones are bike-only — see D7), or any unset zone
// boundary leaves that target unchanged so the watch resolves it from Garmin
// Connect. Non-zone kinds (pace/power_w/hr_bpm/rpe/none) always pass through.
func resolveTargets(steps []workouttemplates.Step, cfg *athleteconfig.AthleteConfig, sport string) []workouttemplates.Step {
	if cfg == nil || len(steps) == 0 {
		return steps
	}
	out := make([]workouttemplates.Step, len(steps))
	for i, st := range steps {
		out[i] = resolveStep(st, cfg, sport)
	}
	return out
}

func resolveStep(st workouttemplates.Step, cfg *athleteconfig.AthleteConfig, sport string) workouttemplates.Step {
	if st.Type == workouttemplates.NodeRepeat {
		inner := make([]workouttemplates.Step, len(st.Steps))
		for i, child := range st.Steps {
			inner[i] = resolveStep(child, cfg, sport)
		}
		st.Steps = inner
		return st
	}
	if st.Target == nil {
		return st
	}
	if resolved := resolveTarget(*st.Target, cfg, sport); resolved != nil {
		st.Target = resolved
	}
	return st
}

// resolveTarget returns a rewritten absolute target for a zone-reference, or nil
// to signal "leave the original unchanged" (passthrough).
func resolveTarget(t workouttemplates.Target, cfg *athleteconfig.AthleteConfig, sport string) *workouttemplates.Target {
	switch t.Kind {
	case workouttemplates.TargetPowerZone:
		// D7: athlete config's power zones are FTP/bike-derived; only resolve
		// them for bike workouts. Any other sport defers to the watch.
		if sport != workouttemplates.SportBike {
			return nil
		}
		return resolveZone(t, workouttemplates.TargetPowerW, func(n int) *int { return powerZoneMax(cfg, n) })
	case workouttemplates.TargetHRZone:
		// HR zones are max-HR-derived and apply across all sports.
		return resolveZone(t, workouttemplates.TargetHRBpm, func(n int) *int { return hrZoneMax(cfg, n) })
	default:
		return nil
	}
}

// resolveZone expands a zone band [Low, High] (1..5) to an absolute range:
// lower edge is the Max of the zone below Low (0 for zone 1), upper edge is the
// Max of zone High. Returns nil (passthrough) when the band is malformed or any
// required boundary is unset.
func resolveZone(t workouttemplates.Target, newKind string, maxFor func(int) *int) *workouttemplates.Target {
	if t.Low == nil || t.High == nil {
		return nil
	}
	lo, hi := *t.Low, *t.High
	if lo < 1 || hi > 5 || lo > hi {
		return nil
	}
	lowVal := 0 // zone-1 lower edge has no configured boundary
	if lo > 1 {
		p := maxFor(lo - 1)
		if p == nil {
			return nil
		}
		lowVal = *p
	}
	hp := maxFor(hi)
	if hp == nil {
		return nil
	}
	highVal := *hp
	origin := fmt.Sprintf("Z%d", lo)
	if lo != hi {
		origin = fmt.Sprintf("Z%d–Z%d", lo, hi)
	}
	return &workouttemplates.Target{
		Kind:   newKind,
		Low:    &lowVal,
		High:   &highVal,
		Origin: origin,
	}
}

func powerZoneMax(cfg *athleteconfig.AthleteConfig, n int) *int {
	switch n {
	case 1:
		return cfg.PowerZone1Max
	case 2:
		return cfg.PowerZone2Max
	case 3:
		return cfg.PowerZone3Max
	case 4:
		return cfg.PowerZone4Max
	case 5:
		return cfg.PowerZone5Max
	}
	return nil
}

func hrZoneMax(cfg *athleteconfig.AthleteConfig, n int) *int {
	switch n {
	case 1:
		return cfg.HRZone1Max
	case 2:
		return cfg.HRZone2Max
	case 3:
		return cfg.HRZone3Max
	case 4:
		return cfg.HRZone4Max
	case 5:
		return cfg.HRZone5Max
	}
	return nil
}
