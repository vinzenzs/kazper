package racepacing

import (
	"fmt"
	"math"
	"sort"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/races"
)

// Machine-readable athlete-config field names surfaced in missing_thresholds.
const (
	fieldFTP  = "ftp_watts"
	fieldPace = "threshold_pace_sec_per_km"
	fieldCSS  = "threshold_swim_pace_sec_per_100m"
)

// band is an intensity band as a [low, high] pair. For bike it is a fraction of
// FTP; for run/swim it is a multiplier of threshold pace / CSS.
type band struct{ low, high float64 }

func (b band) mid() float64 { return (b.low + b.high) / 2 }

// Bike power bands as a fraction of FTP, keyed on the leg's own duration.
// Sources: Coggan/Allen "Training and Racing with a Power Meter" IF-by-duration
// tables; Friel/TrainingPeaks race-plan IF guidance (full-distance ~0.68–0.78,
// 70.3 ~0.75–0.83, olympic ~0.83–0.90, sprint near threshold).
func bikeBand(durMin int) band {
	switch {
	case durMin < 45:
		return band{0.90, 1.00} // sprint-distance effort
	case durMin < 90:
		return band{0.83, 0.90} // olympic
	case durMin < 180:
		return band{0.75, 0.83} // 70.3 / middle distance
	default:
		return band{0.68, 0.78} // full distance
	}
}

// Run pace multipliers vs threshold pace (higher = slower), multisport-calibrated.
func runBand(durMin int) band {
	switch {
	case durMin < 30:
		return band{1.00, 1.04}
	case durMin < 60:
		return band{1.04, 1.10}
	case durMin < 150:
		return band{1.10, 1.18} // 70.3 run off the bike
	default:
		return band{1.18, 1.28} // full-distance marathon
	}
}

// Swim pace multipliers vs CSS (per 100 m), keyed on leg duration.
func swimBand(durMin int) band {
	switch {
	case durMin < 20:
		return band{1.00, 1.05} // sprint 750 m
	case durMin < 45:
		return band{1.03, 1.08} // olympic 1500 m
	default:
		return band{1.06, 1.12} // long-course 1.9–3.8 km
	}
}

// compute builds the full-precision pacing plan; rounding happens at the handler
// boundary. cfg may be nil (no athlete_config row → all thresholds unset).
func compute(race *races.Race, cfg *athleteconfig.AthleteConfig, overrides map[int]*Override) PacingPlan {
	ftp, pace, css := thresholds(cfg)
	plan := PacingPlan{
		RaceID:            race.ID,
		RaceName:          race.Name,
		RaceDate:          race.RaceDate,
		Legs:              make([]LegPacingPlan, 0, len(race.Legs)),
		TSSComplete:       true,
		MissingThresholds: []string{},
	}
	missing := map[string]bool{}
	totalDur := 0
	haveDur := false
	bikeBefore := false

	for _, leg := range race.Legs {
		lp := computeLeg(leg, ftp, pace, css, overrides[leg.Ordinal], bikeBefore)
		plan.Legs = append(plan.Legs, lp)

		if leg.ExpectedDurationMin != nil {
			totalDur += *leg.ExpectedDurationMin
			haveDur = true
		}
		for _, m := range lp.MissingThresholds {
			missing[m] = true
		}
		if lp.EstimatedTSS != nil {
			plan.EstimatedTSSTotal += *lp.EstimatedTSS
		}
		// tss_complete: every non-transition leg must have produced an estimate.
		switch leg.Discipline {
		case races.DisciplineTransition:
			// never counts against completeness
		case races.DisciplineOther:
			plan.TSSComplete = false
		default:
			if lp.EstimatedTSS == nil {
				plan.TSSComplete = false
			}
		}
		if leg.Discipline == races.DisciplineBike {
			bikeBefore = true
		}
	}
	if haveDur {
		plan.TotalDurationMin = &totalDur
	}
	plan.MissingThresholds = sortedKeys(missing)
	return plan
}

// computeLeg produces one leg's plan entry (full precision).
func computeLeg(leg *races.RaceLeg, ftp *int, pace, css *float64, ov *Override, bikeBefore bool) LegPacingPlan {
	lp := LegPacingPlan{
		Ordinal:             leg.Ordinal,
		Discipline:          string(leg.Discipline),
		ExpectedDurationMin: leg.ExpectedDurationMin,
		Source:              SourceComputed,
	}

	// Transition — rest, no target.
	if leg.Discipline == races.DisciplineTransition {
		lp.Source = SourceNone
		zero := 0.0
		lp.EstimatedTSS = &zero
		lp.Rationale = "Transition — not paced; a rest between disciplines (estimated TSS 0)."
		if ov != nil {
			lp.Rationale += " A stored override is ignored (transitions accept no pacing target)."
		}
		return lp
	}

	// Other — no threshold model.
	if leg.Discipline == races.DisciplineOther {
		lp.Source = SourceNone
		lp.Rationale = "No pacing model for an 'other'-discipline leg."
		if ov != nil {
			lp.Rationale += " A stored override is ignored (no pacing model for this discipline)."
		}
		return lp
	}

	// A matching override wins; a mismatched one is ignored (noted) and we fall
	// through to the computed band.
	if ov != nil {
		if overrideMatches(leg.Discipline, ov) {
			return applyOverride(lp, leg, ov, ftp, pace, css)
		}
		lp.Rationale = "A stored override is ignored — its unit no longer matches this leg's discipline. "
	}

	if leg.ExpectedDurationMin == nil {
		lp.Source = SourceNone
		lp.Rationale += "Duration unknown — cannot compute a duration-banded target."
		return lp
	}
	dur := *leg.ExpectedDurationMin
	durHr := durationHr(dur)

	switch leg.Discipline {
	case races.DisciplineBike:
		if ftp == nil {
			lp.Source = SourceNone
			lp.MissingThresholds = []string{fieldFTP}
			lp.Rationale += "FTP is unset in athlete-config — set ftp_watts to get a power band."
			return lp
		}
		b := bikeBand(dur)
		lo := int(math.Round(float64(*ftp) * b.low))
		hi := int(math.Round(float64(*ftp) * b.high))
		lp.TargetPowerLowW, lp.TargetPowerHighW = &lo, &hi
		ifv := b.mid()
		lp.IntensityFactor = &ifv
		tss := durHr * ifv * ifv * 100
		lp.EstimatedTSS = &tss
		lp.Rationale += fmt.Sprintf("Duration-banded baseline: %.0f–%.0f%% of FTP for a %d-min bike leg. Adjust for course, weather, and fitness.", b.low*100, b.high*100, dur)

	case races.DisciplineRun:
		if pace == nil {
			lp.Source = SourceNone
			lp.MissingThresholds = []string{fieldPace}
			lp.Rationale += "Threshold run pace is unset — set threshold_pace_sec_per_km to get a pace band."
			return lp
		}
		b := runBand(dur)
		lo := *pace * b.low // fast end (fewer sec/km)
		hi := *pace * b.high
		lp.TargetPaceLowSecPerKM, lp.TargetPaceHighSecPerKM = &lo, &hi
		ifv := 1 / b.mid()
		lp.IntensityFactor = &ifv
		tss := durHr * ifv * ifv * 100
		lp.EstimatedTSS = &tss
		lp.Rationale += fmt.Sprintf("Duration-banded baseline: ×%.2f–%.2f of threshold pace for a %d-min run leg.", b.low, b.high, dur)
		if bikeBefore {
			lp.Rationale += " The band accounts for running off the bike (multisport context)."
		}
		lp.Rationale += " Adjust for course, weather, and fitness."

	case races.DisciplineSwim:
		if css == nil {
			lp.Source = SourceNone
			lp.MissingThresholds = []string{fieldCSS}
			lp.Rationale += "CSS is unset — set threshold_swim_pace_sec_per_100m to get a pace band."
			return lp
		}
		b := swimBand(dur)
		lo := *css * b.low
		hi := *css * b.high
		lp.TargetPaceLowSecPer100m, lp.TargetPaceHighSecPer100m = &lo, &hi
		ifv := 1 / b.mid()
		lp.IntensityFactor = &ifv
		tss := durHr * ifv * ifv * ifv * 100 // sTSS: swim cost ∝ v³
		lp.EstimatedTSS = &tss
		lp.Rationale += fmt.Sprintf("Duration-banded baseline: ×%.2f–%.2f of CSS for a %d-min swim leg. Adjust for conditions and fitness.", b.low, b.high, dur)
	}
	return lp
}

// applyOverride reports the override's absolute band with source=override. IF and
// estimated TSS are re-derived from the override midpoint only when the relevant
// threshold is set (IF) and the leg has a duration (TSS).
func applyOverride(lp LegPacingPlan, leg *races.RaceLeg, ov *Override, ftp *int, pace, css *float64) LegPacingPlan {
	lp.Source = SourceOverride
	var ifv *float64
	var exp int // TSS exponent on IF (2 for power/pace, 3 for swim)

	switch leg.Discipline {
	case races.DisciplineBike:
		lp.TargetPowerLowW, lp.TargetPowerHighW = ov.TargetPowerLowW, ov.TargetPowerHighW
		exp = 2
		if ftp != nil {
			mid := float64(*ov.TargetPowerLowW+*ov.TargetPowerHighW) / 2
			v := mid / float64(*ftp)
			ifv = &v
		}
	case races.DisciplineRun:
		lp.TargetPaceLowSecPerKM, lp.TargetPaceHighSecPerKM = ov.TargetPaceLowSecPerKM, ov.TargetPaceHighSecPerKM
		exp = 2
		if pace != nil {
			mid := (*ov.TargetPaceLowSecPerKM + *ov.TargetPaceHighSecPerKM) / 2
			v := *pace / mid // faster pace (fewer sec) → higher IF
			ifv = &v
		}
	case races.DisciplineSwim:
		lp.TargetPaceLowSecPer100m, lp.TargetPaceHighSecPer100m = ov.TargetPaceLowSecPer100m, ov.TargetPaceHighSecPer100m
		exp = 3
		if css != nil {
			mid := (*ov.TargetPaceLowSecPer100m + *ov.TargetPaceHighSecPer100m) / 2
			v := *css / mid
			ifv = &v
		}
	}

	lp.Rationale = "Manual override — reported as-set."
	if ifv != nil {
		lp.IntensityFactor = ifv
		if leg.ExpectedDurationMin != nil {
			tss := durationHr(*leg.ExpectedDurationMin) * math.Pow(*ifv, float64(exp)) * 100
			lp.EstimatedTSS = &tss
		} else {
			lp.Rationale += " Estimated TSS omitted (leg duration unknown)."
		}
	} else {
		lp.Rationale += " Intensity factor and TSS omitted (relevant threshold unset)."
	}
	return lp
}

// overrideMatches reports whether the override's unit family matches the leg's
// current discipline.
func overrideMatches(d races.Discipline, ov *Override) bool {
	switch d {
	case races.DisciplineBike:
		return ov.family() == familyPower
	case races.DisciplineRun:
		return ov.family() == familyPaceKM
	case races.DisciplineSwim:
		return ov.family() == familyPace100m
	}
	return false
}

func thresholds(cfg *athleteconfig.AthleteConfig) (ftp *int, pace, css *float64) {
	if cfg == nil {
		return nil, nil, nil
	}
	return cfg.FtpWatts, cfg.ThresholdPaceSecPerKm, cfg.ThresholdSwimPaceSecPer100m
}

func durationHr(min int) float64 { return float64(min) / 60 }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
