package workouts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

// win returns a started/ended pair spanning h hours from a fixed base.
func win(h float64) (time.Time, time.Time) {
	base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	return base, base.Add(time.Duration(h * float64(time.Hour)))
}

func fullCfg() *athleteconfig.AthleteConfig {
	return &athleteconfig.AthleteConfig{
		FtpWatts:                    ip(250),
		ThresholdPaceSecPerKm:       fp(270), // 4:30/km
		ThresholdSwimPaceSecPer100m: fp(90),
		LactateThresholdHR:          ip(170),
		ThresholdHR:                 ip(160),
	}
}

func TestDeriveTSS_Formulas(t *testing.T) {
	cfg := fullCfg()
	s2, e2 := win(2)
	s1, e1 := win(1)

	t.Run("power: 2h @ IF 0.80 -> 128", func(t *testing.T) {
		tss, src := deriveTSS(SportBike, s2, e2, nil, nil, fp(0.80), cfg)
		require.NotNil(t, tss)
		assert.Equal(t, 128.0, *tss)
		assert.Equal(t, "power", *src)
	})

	t.Run("power needs no athlete-config", func(t *testing.T) {
		tss, src := deriveTSS(SportBike, s2, e2, nil, nil, fp(0.80), nil)
		require.NotNil(t, tss)
		assert.Equal(t, 128.0, *tss)
		assert.Equal(t, "power", *src)
	})

	t.Run("rTSS: 300 vs 270 sec/km, 1h -> 81", func(t *testing.T) {
		tss, src := deriveTSS(SportRun, s1, e1, fp(12000), nil, nil, cfg) // 12km in 1h => 300 s/km
		require.NotNil(t, tss)
		assert.Equal(t, 81.0, *tss)
		assert.Equal(t, "pace", *src)
	})

	t.Run("sTSS: cubic, 100 vs 90 sec/100m, 1h -> 72.9", func(t *testing.T) {
		tss, src := deriveTSS(SportSwim, s1, e1, fp(3600), nil, nil, cfg) // 3600m in 1h => 100 s/100m
		require.NotNil(t, tss)
		assert.Equal(t, 72.9, *tss)
		assert.Equal(t, "pace", *src)
	})

	t.Run("hrTSS: 153/170, 1h -> 81 (any sport)", func(t *testing.T) {
		// Run with avg_hr but NO distance: pace gate fails, HR is the fallback.
		tss, src := deriveTSS(SportRun, s1, e1, nil, ip(153), nil, cfg)
		require.NotNil(t, tss)
		assert.Equal(t, 81.0, *tss)
		assert.Equal(t, "hr", *src)

		// Sport-agnostic: strength derives hrTSS the same way.
		tss2, src2 := deriveTSS(SportStrength, s1, e1, nil, ip(153), nil, cfg)
		require.NotNil(t, tss2)
		assert.Equal(t, 81.0, *tss2)
		assert.Equal(t, "hr", *src2)
	})
}

func TestDeriveTSS_LTHRPreference(t *testing.T) {
	s1, e1 := win(1)
	// Both set (lactate 170, threshold 160): avg_hr 170 => IF 1.0 => 100 uses lactate.
	tss, _ := deriveTSS(SportStrength, s1, e1, nil, ip(170),
		nil, &athleteconfig.AthleteConfig{LactateThresholdHR: ip(170), ThresholdHR: ip(160)})
	require.NotNil(t, tss)
	assert.Equal(t, 100.0, *tss, "lactate_threshold_hr must be preferred")

	// Only threshold_hr set: it is used instead.
	tss2, _ := deriveTSS(SportStrength, s1, e1, nil, ip(160),
		nil, &athleteconfig.AthleteConfig{ThresholdHR: ip(160)})
	require.NotNil(t, tss2)
	assert.Equal(t, 100.0, *tss2)
}

func TestDeriveTSS_GatesAndFailOpen(t *testing.T) {
	s1, e1 := win(1)

	t.Run("unset pace threshold falls through to hr", func(t *testing.T) {
		cfg := &athleteconfig.AthleteConfig{LactateThresholdHR: ip(170)} // no pace
		tss, src := deriveTSS(SportRun, s1, e1, fp(12000), ip(153), nil, cfg)
		require.NotNil(t, tss)
		assert.Equal(t, 81.0, *tss)
		assert.Equal(t, "hr", *src)
	})

	t.Run("no method applies -> nil, nil", func(t *testing.T) {
		cfg := &athleteconfig.AthleteConfig{} // no thresholds
		tss, src := deriveTSS(SportRun, s1, e1, fp(12000), ip(153), nil, cfg)
		assert.Nil(t, tss)
		assert.Nil(t, src)
	})

	t.Run("nil config disables threshold methods (fail-open)", func(t *testing.T) {
		tss, src := deriveTSS(SportRun, s1, e1, fp(12000), ip(153), nil, nil)
		assert.Nil(t, tss)
		assert.Nil(t, src)
	})

	t.Run("implausible IF > 2.5 skips", func(t *testing.T) {
		cfg := fullCfg()
		// 100 km in 1h => 36 s/km => IF = 270/36 = 7.5 (a mis-tagged car ride).
		tss, src := deriveTSS(SportRun, s1, e1, fp(100000), nil, nil, cfg)
		assert.Nil(t, tss)
		assert.Nil(t, src)
	})

	t.Run("zero-length window yields nothing", func(t *testing.T) {
		base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
		tss, src := deriveTSS(SportBike, base, base, nil, nil, fp(0.8), nil)
		assert.Nil(t, tss)
		assert.Nil(t, src)
	})
}
