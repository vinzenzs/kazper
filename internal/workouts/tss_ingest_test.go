package workouts_test

import (
	"context"
	"encoding/json"
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

// setupWithConfig wires the workouts service with the athlete-config singleton
// cross-injected and seeded with cfg (nil → unset). Returns the fixture.
func setupWithConfig(t *testing.T, cfg *athleteconfig.AthleteConfig) *fixture {
	t.Helper()
	f := setup(t)
	if cfg != nil {
		require.NoError(t, athleteconfig.NewRepo(f.pool).Upsert(context.Background(), cfg))
	}
	svc := workouts.NewService(f.repo, f.pool, "UTC")
	svc.SetAthleteConfigRepo(athleteconfig.NewRepo(f.pool))
	r := gin.New()
	workouts.NewHandlers(svc).Register(r.Group("/"))
	f.r = r
	return f
}

func putConfig(t *testing.T, f *fixture, cfg *athleteconfig.AthleteConfig) {
	t.Helper()
	require.NoError(t, athleteconfig.NewRepo(f.pool).Upsert(context.Background(), cfg))
}

// tssBody builds a workout payload spanning durH hours from start. extra is a
// raw JSON fragment (leading comma) for the varying fields.
func tssBody(extID, source, sport string, start time.Time, durH float64, extra string) string {
	end := start.Add(time.Duration(durH * float64(time.Hour)))
	return fmt.Sprintf(`{"external_id":%q,"source":%q,"sport":%q,"started_at":%q,"ended_at":%q%s}`,
		extID, source, sport, start.Format(time.RFC3339), end.Format(time.RFC3339), extra)
}

func fullTSSConfig() *athleteconfig.AthleteConfig {
	return &athleteconfig.AthleteConfig{
		FtpWatts:                    intVal(250),
		ThresholdPaceSecPerKm:       f64(270),
		ThresholdSwimPaceSecPer100m: f64(90),
		LactateThresholdHR:          intVal(170),
	}
}

func f64(v float64) *float64 { return &v }

// 4.1: POST /workouts derivation + explicit-provenance + no-method + ignored key.
func TestTSS_PostDerivation(t *testing.T) {
	f := setupWithConfig(t, fullTSSConfig())

	// Explicit tss wins with garmin provenance; no derivation.
	rec := doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:exp", "garmin", "bike", at(7), 2, `,"tss":78,"normalized_power_w":200`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.TSS)
	assert.Equal(t, 78.0, *w.TSS)
	require.NotNil(t, w.TSSSource)
	assert.Equal(t, "garmin", *w.TSSSource)

	// Same explicit tss, source manual → 'manual'.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("m:exp", "manual", "bike", at(7), 2, `,"tss":78`))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "manual", *decodeWorkout(t, rec.Body.Bytes()).TSSSource)

	// Power: 2h bike, NP 200, FTP 250 → IF 0.80 → tss 128, source power.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:pow", "garmin", "bike", at(7), 2, `,"normalized_power_w":200`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w = decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.TSS)
	assert.Equal(t, 128.0, *w.TSS)
	assert.Equal(t, "power", *w.TSSSource)

	// Pace: 1h run, 12km (300 s/km), threshold 270 → tss 81, source pace.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:pace", "garmin", "run", at(8), 1, `,"distance_m":12000`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w = decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.TSS)
	assert.Equal(t, 81.0, *w.TSS)
	assert.Equal(t, "pace", *w.TSSSource)

	// HR fallback: run with avg_hr, no distance → tss 81, source hr.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:hr", "garmin", "run", at(9), 1, `,"avg_hr":153`))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "hr", *decodeWorkout(t, rec.Body.Bytes()).TSSSource)

	// No method applies → 201 with tss + tss_source omitted.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:none", "garmin", "run", at(10), 1, ``))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w = decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.TSS)
	assert.Nil(t, w.TSSSource)
	assert.NotContains(t, rec.Body.String(), "tss_source")

	// A caller-sent tss_source key is ignored (provenance derived from arrival).
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("m:ignore", "manual", "bike", at(11), 1, `,"tss":50,"tss_source":"power"`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "manual", *decodeWorkout(t, rec.Body.Bytes()).TSSSource)
}

// 4.1: derivation never runs for planned workouts, but an explicit planned tss stores.
func TestTSS_PlannedNeverDerives(t *testing.T) {
	f := setupWithConfig(t, fullTSSConfig())

	rec := doReq(t, f.r, http.MethodPost, "/workouts", tssBody("m:pl1", "manual", "run", at(7), 1, `,"status":"planned","distance_m":12000`))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	assert.Nil(t, w.TSS, "planned workouts never derive")

	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("m:pl2", "manual", "run", at(7), 1, `,"status":"planned","tss":60`))
	require.Equal(t, http.StatusCreated, rec.Code)
	w = decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.TSS)
	assert.Equal(t, 60.0, *w.TSS)
	assert.Equal(t, "manual", *w.TSSSource)
}

// 4.3: re-sync full-replace re-applies precedence — explicit tss flips pace→garmin.
func TestTSS_ResyncFlipsProvenance(t *testing.T) {
	f := setupWithConfig(t, fullTSSConfig())

	rec := doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:rs", "garmin", "run", at(7), 1, `,"distance_m":12000`))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "pace", *decodeWorkout(t, rec.Body.Bytes()).TSSSource)

	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:rs", "garmin", "run", at(7), 1, `,"distance_m":12000,"tss":95`))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String()) // update, not create
	w := decodeWorkout(t, rec.Body.Bytes())
	require.NotNil(t, w.TSS)
	assert.Equal(t, 95.0, *w.TSS)
	assert.Equal(t, "garmin", *w.TSSSource)
}

// 4.2: bulk items derive independently (power / pace / none).
func TestTSS_BulkPerItem(t *testing.T) {
	f := setupWithConfig(t, fullTSSConfig())
	body := fmt.Sprintf(`{"workouts":[%s,%s,%s]}`,
		tssBody("g:b1", "garmin", "bike", at(7), 2, `,"normalized_power_w":200`),
		tssBody("g:b2", "garmin", "run", at(8), 1, `,"distance_m":12000`),
		tssBody("g:b3", "garmin", "swim", at(9), 1, ``)) // no distance/hr → none
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, "power", *loadByExternal(t, f, "g:b1").TSSSource)
	assert.Equal(t, "pace", *loadByExternal(t, f, "g:b2").TSSSource)
	assert.Nil(t, loadByExternal(t, f, "g:b3").TSSSource)
}

// 4.4: PATCH tss marks manual; PATCH null clears both; omitempty on responses.
func TestTSS_PatchInterplay(t *testing.T) {
	f := setupWithConfig(t, fullTSSConfig())
	rec := doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:pt", "garmin", "run", at(7), 1, `,"distance_m":12000`))
	require.Equal(t, http.StatusCreated, rec.Code)
	w := decodeWorkout(t, rec.Body.Bytes())
	require.Equal(t, "pace", *w.TSSSource)

	// Patch a value → tss_source becomes 'manual'.
	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"tss":85}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	pw := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, 85.0, *pw.TSS)
	assert.Equal(t, "manual", *pw.TSSSource)

	// Patch null → both cleared; response omits both keys.
	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"tss":null}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, decodeWorkout(t, rec.Body.Bytes()).TSS)
	got, _ := getWorkout(t, f, w.ID)
	assert.Nil(t, got.TSS)
	assert.Nil(t, got.TSSSource)
}

// 5.3: recompute fills NULL rows, never touches measured, refreshes on threshold
// change, clears when no method applies, and no-ops when nothing changes.
func TestTSS_Recompute(t *testing.T) {
	// Start with NO thresholds so runs land NULL and a manual/garmin row is measured.
	f := setupWithConfig(t, &athleteconfig.AthleteConfig{})

	// A run with distance but no thresholds → tss NULL (a recompute candidate).
	rec := doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:rc1", "garmin", "run", at(7), 1, `,"distance_m":12000`))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Nil(t, decodeWorkout(t, rec.Body.Bytes()).TSSSource)

	// A measured garmin tss (immutable to recompute).
	rec = doReq(t, f.r, http.MethodPost, "/workouts", tssBody("g:rc2", "garmin", "bike", at(8), 1, `,"tss":40`))
	require.Equal(t, http.StatusCreated, rec.Code)

	// Configure thresholds, then recompute: the NULL run gains a pace value.
	putConfig(t, f, fullTSSConfig())
	res := recompute(t, f)
	assert.GreaterOrEqual(t, res.Updated, 1)
	assert.Equal(t, 1, res.BySource["pace"])
	assert.Equal(t, "pace", *loadByExternal(t, f, "g:rc1").TSSSource)
	assert.Equal(t, "garmin", *loadByExternal(t, f, "g:rc2").TSSSource, "measured row untouched")
	assert.Equal(t, 40.0, *loadByExternal(t, f, "g:rc2").TSS)

	// Change the threshold pace → the pace row is recomputed to a new value.
	before := *loadByExternal(t, f, "g:rc1").TSS
	putConfig(t, f, &athleteconfig.AthleteConfig{ThresholdPaceSecPerKm: f64(300)})
	res = recompute(t, f)
	assert.GreaterOrEqual(t, res.Updated, 1)
	assert.NotEqual(t, before, *loadByExternal(t, f, "g:rc1").TSS)

	// Clear all thresholds → the computed pace row resets to NULL (by_source.none).
	putConfig(t, f, &athleteconfig.AthleteConfig{})
	res = recompute(t, f)
	assert.GreaterOrEqual(t, res.BySource["none"], 1)
	assert.Nil(t, loadByExternal(t, f, "g:rc1").TSSSource)

	// Nothing left to change → updated 0.
	res = recompute(t, f)
	assert.Equal(t, 0, res.Updated)
}

type recomputeResp struct {
	Examined int            `json:"examined"`
	Updated  int            `json:"updated"`
	BySource map[string]int `json:"by_source"`
}

func recompute(t *testing.T, f *fixture) recomputeResp {
	t.Helper()
	rec := doReq(t, f.r, http.MethodPost, "/workouts/recompute-tss", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out recomputeResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// loadByExternal fetches a workout by external id via a window list + filter.
func loadByExternal(t *testing.T, f *fixture, extID string) *workouts.Workout {
	t.Helper()
	got, err := f.repo.GetByExternalID(context.Background(), extID)
	require.NoError(t, err)
	return got
}
