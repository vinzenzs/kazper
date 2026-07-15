package athleteconfig

import "testing"

// ip/fp pointer helpers are defined in history_internal_test.go.

func TestResolveEffective_AllManualEqualsConfig(t *testing.T) {
	cfg := &AthleteConfig{FtpWatts: ip(278), MaxHR: ip(196), ThresholdHR: ip(170)}
	eff := resolveEffective(cfg, nil, &GarminDetectedThresholds{FtpWatts: ip(285), MaxHR: ip(199)})
	if eff == nil {
		t.Fatal("expected non-nil effective")
	}
	if *eff.FtpWatts != 278 || *eff.MaxHR != 196 || *eff.ThresholdHR != 170 {
		t.Fatalf("all-manual policy must equal config: %+v", eff.AthleteConfig)
	}
	for field, src := range eff.FieldSources {
		if src != SourceManual {
			t.Fatalf("field %s: want manual, got %s", field, src)
		}
	}
}

func TestResolveEffective_GarminSourcedFieldUsesDetection(t *testing.T) {
	cfg := &AthleteConfig{FtpWatts: ip(278), MaxHR: ip(196)}
	det := &GarminDetectedThresholds{FtpWatts: ip(285), MaxHR: ip(199)}
	eff := resolveEffective(cfg, []string{SourceFTPWatts}, det)
	if *eff.FtpWatts != 285 {
		t.Fatalf("garmin-sourced ftp: want 285, got %d", *eff.FtpWatts)
	}
	if eff.FieldSources["ftp_watts"] != SourceGarmin {
		t.Fatalf("ftp_watts source: want garmin, got %s", eff.FieldSources["ftp_watts"])
	}
	// max_hr not sourced → confirmed value, manual annotation.
	if *eff.MaxHR != 196 || eff.FieldSources["max_hr"] != SourceManual {
		t.Fatalf("max_hr should stay manual 196, got %d/%s", *eff.MaxHR, eff.FieldSources["max_hr"])
	}
}

func TestResolveEffective_MissingDetectionFallsBackToManual(t *testing.T) {
	cfg := &AthleteConfig{MaxHR: ip(196)}
	// max_hr sourced but the detection carries no max HR → manual fallback.
	det := &GarminDetectedThresholds{FtpWatts: ip(285)}
	eff := resolveEffective(cfg, []string{SourceMaxHR}, det)
	if *eff.MaxHR != 196 {
		t.Fatalf("missing detection: want manual 196, got %d", *eff.MaxHR)
	}
	if eff.FieldSources["max_hr"] != SourceManual {
		t.Fatalf("max_hr should annotate manual on fallback, got %s", eff.FieldSources["max_hr"])
	}
}

func TestResolveEffective_ZonesFlipAsGroup(t *testing.T) {
	cfg := &AthleteConfig{
		HRZone1Max: ip(120), HRZone2Max: ip(140), HRZone3Max: ip(155), HRZone4Max: ip(168), HRZone5Max: ip(182),
	}
	det := &GarminDetectedThresholds{
		HRZone1Max: ip(122), HRZone2Max: ip(142), HRZone3Max: ip(157), HRZone4Max: ip(170), HRZone5Max: ip(185),
	}
	eff := resolveEffective(cfg, []string{SourceHRZones}, det)
	if *eff.HRZone1Max != 122 || *eff.HRZone5Max != 185 {
		t.Fatalf("hr zones should flip as a set: z1=%d z5=%d", *eff.HRZone1Max, *eff.HRZone5Max)
	}
	if eff.FieldSources["hr_zone_3_max"] != SourceGarmin {
		t.Fatalf("hr_zone_3_max should annotate garmin, got %s", eff.FieldSources["hr_zone_3_max"])
	}
}

func TestResolveEffective_NilWhenNothing(t *testing.T) {
	if eff := resolveEffective(nil, nil, nil); eff != nil {
		t.Fatalf("no config, no policy, no detection → nil, got %+v", eff)
	}
	// Policy set but no detection → still nil (no confirmed config to report).
	if eff := resolveEffective(nil, []string{SourceFTPWatts}, nil); eff != nil {
		t.Fatalf("no config + no detection → nil, got %+v", eff)
	}
}

func TestResolveEffective_ThresholdPaceFloat(t *testing.T) {
	cfg := &AthleteConfig{ThresholdPaceSecPerKm: fp(258.0)}
	det := &GarminDetectedThresholds{ThresholdPaceSecPerKm: fp(251.5)}
	eff := resolveEffective(cfg, []string{SourceThresholdPace}, det)
	if *eff.ThresholdPaceSecPerKm != 251.5 {
		t.Fatalf("threshold pace should use detection: %v", *eff.ThresholdPaceSecPerKm)
	}
}
