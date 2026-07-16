package workoutfueling_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/workoutfueling"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// fakeConfig stands in for the effective-config provider. err/nil cases model
// the fail-open contract: an unset FTP must degrade, never error.
type fakeConfig struct {
	cfg *athleteconfig.AthleteConfig
	err error
}

func (f fakeConfig) Get(context.Context) (*athleteconfig.AthleteConfig, error) {
	return f.cfg, f.err
}

func withFTP(watts int) fakeConfig {
	return fakeConfig{cfg: &athleteconfig.AthleteConfig{FtpWatts: &watts}}
}

// planFixture mounts the fueling-plan endpoint with a given config reader.
func planFixture(t *testing.T, cfg workoutfueling.ConfigReader) *fixture {
	t.Helper()
	f := setup(t)
	if cfg != nil {
		f.svc.SetConfigReader(cfg)
	}
	return f
}

// makePlannedWorkoutTSS: a planned ride of `mins` with optional planned TSS.
// tss_source is paired with tss by a DB CHECK.
func makePlannedWorkoutTSS(t *testing.T, repo *workouts.Repo, mins float64, tss *float64) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: start,
		EndedAt:   start.Add(time.Duration(mins) * time.Minute),
		TSS:       tss,
	}
	if tss != nil {
		src := "manual"
		w.TSSSource = &src
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func getPlan(t *testing.T, f *fixture, path string) workoutfueling.FuelingPlan {
	t.Helper()
	rec := doGet(t, f.r, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.FuelingPlan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func fp(v float64) *float64 { return &v }

// ============================================================================

func TestFuelingPlan_LongPlannedRide(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

	out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

	assert.Equal(t, wid, out.WorkoutID)
	assert.Nil(t, out.Reason)
	require.NotNil(t, out.EstimatedKJ)
	assert.InDelta(t, 1814.4, *out.EstimatedKJ, 0.1)
	require.NotNil(t, out.Inputs.PlannedIF)
	assert.InDelta(t, 0.77, *out.Inputs.PlannedIF, 0.01)
	require.NotNil(t, out.Inputs.CHOFraction)
	assert.Equal(t, 0.70, *out.Inputs.CHOFraction)
	require.NotNil(t, out.EstimatedCarbBurnG)
	assert.InDelta(t, 317.5, *out.EstimatedCarbBurnG, 0.2)

	require.NotNil(t, out.Prescription)
	assert.Equal(t, 60.0, out.Prescription.PerHourMinG)
	assert.Equal(t, 90.0, out.Prescription.PerHourMaxG)
	assert.InDelta(t, 270, out.Prescription.SessionTotalMaxG, 0.1)
	require.NotNil(t, out.ProjectedDeficitG)
	assert.InDelta(t, 47.5, *out.ProjectedDeficitG, 0.3)

	// The FTP came from the effective config, and is echoed.
	require.NotNil(t, out.Inputs.FTPWatts)
	assert.Equal(t, 280, *out.Inputs.FTPWatts)
}

func TestFuelingPlan_CapacityClamps(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

	out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan?carbs_per_hr=70")

	assert.Equal(t, 70.0, out.Prescription.PerHourMaxG)
	assert.InDelta(t, 210, out.Prescription.SessionTotalMaxG, 0.1)
	require.NotNil(t, out.Inputs.CarbsPerHrLimit)
	assert.Equal(t, 70.0, *out.Inputs.CarbsPerHrLimit)
	// Less in means a bigger hole to fill afterwards.
	require.NotNil(t, out.ProjectedDeficitG)
	assert.InDelta(t, 107.5, *out.ProjectedDeficitG, 0.3)
}

func TestFuelingPlan_ShortSessionPrescribesNothing(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 45, fp(60))

	out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

	assert.Zero(t, out.Prescription.PerHourMaxG)
	assert.Zero(t, out.Prescription.SessionTotalMaxG)
	// Burn is still estimated — the ride costs what it costs.
	require.NotNil(t, out.EstimatedCarbBurnG)
	assert.Greater(t, *out.EstimatedCarbBurnG, 0.0)
}

func TestFuelingPlan_FTPMissingDegradesToGuidance(t *testing.T) {
	// A config with no FTP set at all.
	f := planFixture(t, fakeConfig{cfg: &athleteconfig.AthleteConfig{}})
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

	out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

	require.NotNil(t, out.Reason)
	assert.Equal(t, "ftp_missing", *out.Reason)
	require.NotNil(t, out.Prescription)
	assert.Equal(t, 90.0, out.Prescription.PerHourMaxG)
	assert.Nil(t, out.EstimatedKJ)
	assert.Nil(t, out.EstimatedCarbBurnG)
}

// Fail-open: neither an unwired reader nor a failing one may turn a fueling
// plan into an error.
func TestFuelingPlan_ConfigReaderUnwiredOrFailingFailsOpen(t *testing.T) {
	t.Run("unwired", func(t *testing.T) {
		f := planFixture(t, nil)
		wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

		out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

		require.NotNil(t, out.Reason)
		assert.Equal(t, "ftp_missing", *out.Reason)
		assert.NotNil(t, out.Prescription)
	})

	t.Run("read error", func(t *testing.T) {
		f := planFixture(t, fakeConfig{err: errors.New("config store unreachable")})
		wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

		out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

		require.NotNil(t, out.Reason)
		assert.Equal(t, "ftp_missing", *out.Reason)
		assert.NotNil(t, out.Prescription)
	})
}

func TestFuelingPlan_TSSMissingDegradesToGuidance(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, nil)

	out := getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan")

	require.NotNil(t, out.Reason)
	assert.Equal(t, "tss_missing", *out.Reason)
	require.NotNil(t, out.Prescription)
	assert.Equal(t, 60.0, out.Prescription.PerHourMinG)
	assert.Nil(t, out.EstimatedCarbBurnG)
}

// plan_data_missing needs a workout with NO duration, which the workouts table
// forbids (`CHECK (ended_at > started_at)`, migration 012) — so through this
// endpoint a duration always exists and the worst degradation reachable is
// tss_missing. The branch stays as defense-in-depth (covered by the pure-math
// tests); this test pins the reason WHY it can't fire here, so a future
// relaxation of the constraint doesn't quietly leave the branch untested.
func TestFuelingPlan_ZeroLengthWorkoutIsRejectedByTheSchema(t *testing.T) {
	f := planFixture(t, withFTP(280))
	start := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)

	_, err := f.workoutsRepo.Upsert(context.Background(), &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: start,
		EndedAt:   start, // zero length
	})

	require.Error(t, err, "a zero-length workout must not be storable")
	assert.Contains(t, err.Error(), "workouts_check")
}

func TestFuelingPlan_CompletedWorkoutIsRefused(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makeTimedWorkout(t, f.workoutsRepo, 3) // completed

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling-plan")

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "workout_not_planned", body["error"])
}

func TestFuelingPlan_NotFound(t *testing.T) {
	f := planFixture(t, withFTP(280))

	rec := doGet(t, f.r, "/workouts/"+uuid.New().String()+"/fueling-plan")
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = doGet(t, f.r, "/workouts/not-a-uuid/fueling-plan")
	require.Equal(t, http.StatusNotFound, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "not_found", body["error"])
}

func TestFuelingPlan_CarbsPerHrValidation(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))
	base := "/workouts/" + wid.String() + "/fueling-plan?carbs_per_hr="

	for _, bad := range []string{"0", "-5", "131", "abc"} {
		t.Run(bad, func(t *testing.T) {
			rec := doGet(t, f.r, base+bad)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "carbs_per_hr_invalid", body["error"])
		})
	}

	// The boundary itself is valid.
	rec := doGet(t, f.r, base+"130")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestFuelingPlan_ReadOnly(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

	getPlan(t, f, "/workouts/"+wid.String()+"/fueling-plan?carbs_per_hr=70")

	// The workout is untouched — still planned, same TSS.
	w, err := f.workoutsRepo.GetByID(context.Background(), wid)
	require.NoError(t, err)
	assert.Equal(t, workouts.StatusPlanned, w.Status)
	require.NotNil(t, w.TSS)
	assert.InDelta(t, 180, *w.TSS, 0.001)

	// The plan is intent, not a log: no workout-fuel entry was created.
	fuel, err := f.fuelRepo.ListByWorkout(context.Background(), wid)
	require.NoError(t, err)
	assert.Empty(t, fuel)
}

// Unit isolation: the plan talks in session grams and kJ. It must not leak the
// daily-nutrition vocabulary — the prescription is not food that was eaten.
func TestFuelingPlan_UnitIsolation(t *testing.T) {
	f := planFixture(t, withFTP(280))
	wid := makePlannedWorkoutTSS(t, f.workoutsRepo, 180, fp(180))

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling-plan")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.NotContains(t, body, "protein_g")
	assert.NotContains(t, body, "fat_g")
	assert.NotContains(t, body, "total_ml")
	assert.NotContains(t, body, "sodium_mg")
	assert.NotContains(t, body, "adherence")
}
