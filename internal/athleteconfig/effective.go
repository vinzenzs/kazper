package athleteconfig

import "context"

// ConfigProvider is the narrow read the computational consumers (per-sport TSS
// derivation, trainingplan zone resolution, race pacing, step compliance) use to
// fetch the athlete's physiology. Both *Repo (raw confirmed values) and
// *EffectiveProvider (the resolved effective view) satisfy it; the server trunk
// wires the latter so a garmin-sourced field flows into computations without any
// per-consumer change.
type ConfigProvider interface {
	Get(ctx context.Context) (*AthleteConfig, error)
}

// EffectiveProvider adapts the Service's effective resolution to the
// ConfigProvider shape: its Get returns the resolved physiology (garmin-sourced
// fields swapped for the latest detection, manual otherwise), dropping the
// per-field annotations the consumers do not need.
type EffectiveProvider struct {
	svc *Service
}

// NewEffectiveProvider wires the provider over the athlete-config service.
func NewEffectiveProvider(svc *Service) *EffectiveProvider {
	return &EffectiveProvider{svc: svc}
}

// Get returns the resolved effective config, or nil when neither a confirmed
// config nor an applied garmin-sourced detection exists.
func (p *EffectiveProvider) Get(ctx context.Context) (*AthleteConfig, error) {
	eff, err := p.svc.EffectiveConfig(ctx)
	if err != nil || eff == nil {
		return nil, err
	}
	cfg := eff.AthleteConfig
	return &cfg, nil
}

// EffectiveConfig resolves the confirmed config against the source policy and the
// latest detection. Returns nil when there is genuinely nothing to report (no
// confirmed config and no garmin-sourced field that a detection could fill).
func (s *Service) EffectiveConfig(ctx context.Context) (*EffectiveConfig, error) {
	cfg, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	sources, err := s.repo.GetSources(ctx)
	if err != nil {
		return nil, err
	}
	det, err := s.repo.GetDetection(ctx)
	if err != nil {
		return nil, err
	}
	return resolveEffective(cfg, sources, det), nil
}

// resolveEffective builds the effective config: per field, the detection value
// when the field's source token is active AND the detection carries a non-null
// value (manual fallback otherwise — a garmin-sourced field with no detection
// never yields a hole); the confirmed value for every non-sourced field. Zone
// tokens control their whole ladder. threshold_hr and swim CSS are always
// manual (Garmin exposes neither).
func resolveEffective(cfg *AthleteConfig, sources []string, det *GarminDetectedThresholds) *EffectiveConfig {
	srcSet := make(map[string]struct{}, len(sources))
	for _, f := range sources {
		srcSet[f] = struct{}{}
	}
	sourced := func(token string) bool {
		if det == nil {
			return false
		}
		_, ok := srcSet[token]
		return ok
	}

	var base AthleteConfig
	hasBase := cfg != nil
	if hasBase {
		base = *cfg
	}
	fs := make(map[string]string, 16)
	applied := false

	resolveInt := func(name, token string, manual, detected *int) *int {
		if sourced(token) && detected != nil {
			fs[name] = SourceGarmin
			applied = true
			return detected
		}
		fs[name] = SourceManual
		return manual
	}
	resolveFloat := func(name, token string, manual, detected *float64) *float64 {
		if sourced(token) && detected != nil {
			fs[name] = SourceGarmin
			applied = true
			return detected
		}
		fs[name] = SourceManual
		return manual
	}

	var detFTP, detLTHR, detMaxHR *int
	var detPace *float64
	var dHR, dPow [5]*int
	if det != nil {
		detFTP, detLTHR, detMaxHR, detPace = det.FtpWatts, det.LactateThresholdHR, det.MaxHR, det.ThresholdPaceSecPerKm
		dHR = [5]*int{det.HRZone1Max, det.HRZone2Max, det.HRZone3Max, det.HRZone4Max, det.HRZone5Max}
		dPow = [5]*int{det.PowerZone1Max, det.PowerZone2Max, det.PowerZone3Max, det.PowerZone4Max, det.PowerZone5Max}
	}

	base.FtpWatts = resolveInt("ftp_watts", SourceFTPWatts, base.FtpWatts, detFTP)
	base.LactateThresholdHR = resolveInt("lactate_threshold_hr", SourceLactateThresholdHR, base.LactateThresholdHR, detLTHR)
	base.MaxHR = resolveInt("max_hr", SourceMaxHR, base.MaxHR, detMaxHR)
	base.ThresholdPaceSecPerKm = resolveFloat("threshold_pace_sec_per_km", SourceThresholdPace, base.ThresholdPaceSecPerKm, detPace)

	hrManual := [5]*int{base.HRZone1Max, base.HRZone2Max, base.HRZone3Max, base.HRZone4Max, base.HRZone5Max}
	hrResolved := [5]*int{
		resolveInt("hr_zone_1_max", SourceHRZones, hrManual[0], dHR[0]),
		resolveInt("hr_zone_2_max", SourceHRZones, hrManual[1], dHR[1]),
		resolveInt("hr_zone_3_max", SourceHRZones, hrManual[2], dHR[2]),
		resolveInt("hr_zone_4_max", SourceHRZones, hrManual[3], dHR[3]),
		resolveInt("hr_zone_5_max", SourceHRZones, hrManual[4], dHR[4]),
	}
	base.HRZone1Max, base.HRZone2Max, base.HRZone3Max, base.HRZone4Max, base.HRZone5Max = hrResolved[0], hrResolved[1], hrResolved[2], hrResolved[3], hrResolved[4]

	powManual := [5]*int{base.PowerZone1Max, base.PowerZone2Max, base.PowerZone3Max, base.PowerZone4Max, base.PowerZone5Max}
	powResolved := [5]*int{
		resolveInt("power_zone_1_max", SourcePowerZones, powManual[0], dPow[0]),
		resolveInt("power_zone_2_max", SourcePowerZones, powManual[1], dPow[1]),
		resolveInt("power_zone_3_max", SourcePowerZones, powManual[2], dPow[2]),
		resolveInt("power_zone_4_max", SourcePowerZones, powManual[3], dPow[3]),
		resolveInt("power_zone_5_max", SourcePowerZones, powManual[4], dPow[4]),
	}
	base.PowerZone1Max, base.PowerZone2Max, base.PowerZone3Max, base.PowerZone4Max, base.PowerZone5Max = powResolved[0], powResolved[1], powResolved[2], powResolved[3], powResolved[4]

	// The two never-sourced fields are always manual.
	fs["threshold_hr"] = SourceManual
	fs["threshold_swim_pace_sec_per_100m"] = SourceManual

	if !hasBase && !applied {
		return nil
	}
	return &EffectiveConfig{AthleteConfig: base, FieldSources: fs}
}
