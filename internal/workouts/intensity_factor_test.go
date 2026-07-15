package workouts_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// setupWithFTP builds a workouts fixture with the athlete-config singleton
// cross-injected (mirroring httpserver wiring) and ftp_watts pre-set when ftp is
// non-nil. This exercises the intensity_factor derivation path end-to-end.
func setupWithFTP(t *testing.T, ftp *int) *fixture {
	t.Helper()
	f := setup(t)
	cfgRepo := athleteconfig.NewRepo(f.pool)
	if ftp != nil {
		require.NoError(t, cfgRepo.Upsert(context.Background(), &athleteconfig.AthleteConfig{FtpWatts: ftp}))
	}
	svc := workouts.NewService(f.repo, f.pool, "UTC")
	svc.SetAthleteConfigRepo(cfgRepo)
	r := gin.New()
	workouts.NewHandlers(svc).Register(r.Group("/"))
	f.r = r
	return f
}

func intVal(i int) *int { return &i }

// ifBody builds a workout payload. sport varies; np and if are injected as raw
// JSON fragments so each can be present or omitted.
func ifBody(extID, sport string, start time.Time, npFrag, ifFrag string) string {
	return fmt.Sprintf(`{
        "external_id":%q,"source":"garmin","sport":%q,
        "started_at":%q,"ended_at":%q,
        "kcal_burned":600,"avg_hr":150%s%s
    }`, extID, sport, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339), npFrag, ifFrag)
}

// 3.1 Bike + FTP + NP + no supplied IF → stored IF == Round2(np/ftp).
func TestIF_BikeDerivesFromFTP(t *testing.T) {
	f := setupWithFTP(t, intVal(250))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if1", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.IntensityFactor)
	assert.InDelta(t, 0.80, *w.IntensityFactor, 0.001, "200/250 = 0.80")

	// Persisted, not just echoed.
	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.IntensityFactor)
	assert.InDelta(t, 0.80, *got.IntensityFactor, 0.001)
}

// 3.2 Caller-supplied IF is stored verbatim and never overridden.
func TestIF_SuppliedIFNotOverridden(t *testing.T) {
	f := setupWithFTP(t, intVal(250))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if2", "bike", at(7), `,"normalized_power_w":200`, `,"intensity_factor":0.95`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.IntensityFactor)
	assert.InDelta(t, 0.95, *w.IntensityFactor, 0.001, "supplied IF wins, no derivation")
}

// 3.3 Non-bike sport (run) with NP + FTP → IF stays NULL.
func TestIF_NonBikeNotDerived(t *testing.T) {
	f := setupWithFTP(t, intVal(250))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if3", "run", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.IntensityFactor, "FTP is a cycling metric; run is not derived")
}

// 3.4 FTP unset → IF stays NULL, create succeeds.
func TestIF_NoFTPLeavesNull(t *testing.T) {
	f := setupWithFTP(t, nil) // wired repo, but no ftp_watts row value
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if4", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.IntensityFactor)
}

// 3.5 NP missing → IF stays NULL.
func TestIF_NoNormalizedPowerLeavesNull(t *testing.T) {
	f := setupWithFTP(t, intVal(250))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if5", "bike", at(7), "", ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.IntensityFactor)
}

// 3.6 Re-sync (full-replace via same external_id) fills a previously-NULL IF.
// First write has no FTP-derivable IF (FTP unset); second write, with FTP now
// configured, fills it.
func TestIF_ResyncFillsNull(t *testing.T) {
	f := setupWithFTP(t, nil)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if6", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.Nil(t, w.IntensityFactor)

	// Configure FTP and re-POST the same external_id (full-replace upsert).
	cfgRepo := athleteconfig.NewRepo(f.pool)
	require.NoError(t, cfgRepo.Upsert(context.Background(), &athleteconfig.AthleteConfig{FtpWatts: intVal(250)}))
	rec2 := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if6", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String()) // re-sync = update, not create
	w2 := decodeWorkout(t, rec2.Body.Bytes())
	require.NotNil(t, w2.IntensityFactor)
	assert.InDelta(t, 0.80, *w2.IntensityFactor, 0.001)
}

// 3.8 A garmin-sourced FTP drives the derivation: config FTP 278, detection FTP
// 285, ftp_watts sourced → IF derives against the EFFECTIVE 285, not the
// confirmed 278. Proves the effective provider is what the workouts service reads.
func TestIF_DerivesAgainstEffectiveFTP(t *testing.T) {
	f := setup(t)
	cfgRepo := athleteconfig.NewRepo(f.pool)
	ctx := context.Background()
	require.NoError(t, cfgRepo.Upsert(ctx, &athleteconfig.AthleteConfig{FtpWatts: intVal(278)}))
	require.NoError(t, cfgRepo.UpsertDetection(ctx, &athleteconfig.GarminDetectedThresholds{FtpWatts: intVal(285)}))
	require.NoError(t, cfgRepo.PutSources(ctx, []string{athleteconfig.SourceFTPWatts}))

	cfgSvc := athleteconfig.NewService(cfgRepo, f.pool)
	svc := workouts.NewService(f.repo, f.pool, "UTC")
	svc.SetAthleteConfigRepo(athleteconfig.NewEffectiveProvider(cfgSvc))
	r := gin.New()
	workouts.NewHandlers(svc).Register(r.Group("/"))

	rec := doReq(t, r, http.MethodPost, "/workouts", ifBody("garmin:ifeff", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.IntensityFactor)
	assert.InDelta(t, 0.70, *w.IntensityFactor, 0.001, "200/285 = 0.70, effective (detected) FTP")
}

// 3.7 Nil athlete-config repo (default setup, unwired) → write succeeds, no
// derivation, no panic.
func TestIF_NilConfigRepoNoPanic(t *testing.T) {
	f := setup(t) // setup does NOT wire SetAthleteConfigRepo
	rec := doReq(t, f.r, http.MethodPost, "/workouts", ifBody("garmin:if7", "bike", at(7), `,"normalized_power_w":200`, ""))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.IntensityFactor)
}
