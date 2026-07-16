package racepacing_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/racepacing"
	"github.com/vinzenzs/kazper/internal/races"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r   *gin.Engine
	svc *racepacing.Service
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	// idempotency middleware so the PUT-rejects-Idempotency-Key rule is exercised.
	r.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	g := r.Group("/")

	racesRepo := races.NewRepo(pool)
	races.NewHandlers(races.NewService(pool, racesRepo)).Register(g)
	acRepo := athleteconfig.NewRepo(pool)
	athleteconfig.NewHandlers(athleteconfig.NewService(acRepo, pool)).Register(g)
	svc := racepacing.NewService(racesRepo, acRepo, racepacing.NewRepo(pool))
	racepacing.NewHandlers(svc).Register(g)
	return &fixture{r: r, svc: svc}
}

func (f *fixture) do(t *testing.T, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) createRace(t *testing.T, body string) races.Race {
	t.Helper()
	rec := f.do(t, http.MethodPost, "/races", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var race races.Race
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &race))
	return race
}

func (f *fixture) setConfig(t *testing.T, body string) {
	t.Helper()
	rec := f.do(t, http.MethodPut, "/athlete-config", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func (f *fixture) plan(t *testing.T, raceID string) (racepacing.PacingPlan, int) {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/races/"+raceID+"/pacing-plan", "", nil)
	if rec.Code != http.StatusOK {
		return racepacing.PacingPlan{}, rec.Code
	}
	var p racepacing.PacingPlan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	return p, rec.Code
}

const fullTri = `{"name":"IM","race_date":"2026-09-01","legs":[
	{"ordinal":1,"discipline":"swim","expected_duration_min":70},
	{"ordinal":2,"discipline":"transition","expected_duration_min":5},
	{"ordinal":3,"discipline":"bike","expected_duration_min":300},
	{"ordinal":4,"discipline":"run","expected_duration_min":240}]}`

const allThresholds = `{"ftp_watts":265,"threshold_pace_sec_per_km":270,"threshold_swim_pace_sec_per_100m":105}`

func legByOrdinal(p racepacing.PacingPlan, ord int) *racepacing.LegPacingPlan {
	for i := range p.Legs {
		if p.Legs[i].Ordinal == ord {
			return &p.Legs[i]
		}
	}
	return nil
}

func TestPlan_HappyPathFullTri(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)

	p, code := f.plan(t, race.ID.String())
	require.Equal(t, http.StatusOK, code)
	assert.True(t, p.TSSComplete)
	assert.Empty(t, p.MissingThresholds)

	bike := legByOrdinal(p, 3)
	require.NotNil(t, bike.TargetPowerLowW)
	assert.Equal(t, 180, *bike.TargetPowerLowW)
	assert.Equal(t, 207, *bike.TargetPowerHighW)
	assert.InDelta(t, 0.73, *bike.IntensityFactor, 0.001)

	swim := legByOrdinal(p, 1)
	require.NotNil(t, swim.TargetPaceLowSecPer100m)
	assert.InDelta(t, 111.3, *swim.TargetPaceLowSecPer100m, 0.05)

	run := legByOrdinal(p, 4)
	require.NotNil(t, run.TargetPaceLowSecPerKM)
	assert.Contains(t, run.Rationale, "off the bike")
}

func TestPlan_UnitIsolation(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// The bike leg's JSON object carries no pace keys, and vice versa. Assert on
	// the raw serialized leg objects.
	var raw struct {
		Legs []map[string]any `json:"legs"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for _, leg := range raw.Legs {
		switch leg["discipline"] {
		case "bike":
			assert.NotContains(t, leg, "target_pace_low_sec_per_km")
			assert.NotContains(t, leg, "target_pace_low_sec_per_100m")
		case "run":
			assert.NotContains(t, leg, "target_power_low_w")
			assert.NotContains(t, leg, "target_pace_low_sec_per_100m")
		case "swim":
			assert.NotContains(t, leg, "target_power_low_w")
			assert.NotContains(t, leg, "target_pace_low_sec_per_km")
		}
	}
}

func TestPlan_MissingFTPPartial(t *testing.T) {
	f := setup(t)
	f.setConfig(t, `{"threshold_pace_sec_per_km":270,"threshold_swim_pace_sec_per_100m":105}`) // no ftp
	race := f.createRace(t, fullTri)

	p, code := f.plan(t, race.ID.String())
	require.Equal(t, http.StatusOK, code)
	bike := legByOrdinal(p, 3)
	assert.Nil(t, bike.TargetPowerLowW)
	assert.Equal(t, []string{"ftp_watts"}, bike.MissingThresholds)
	run := legByOrdinal(p, 4)
	require.NotNil(t, run.TargetPaceLowSecPerKM) // run still computed
	assert.Contains(t, p.MissingThresholds, "ftp_watts")
	assert.False(t, p.TSSComplete)
}

func TestPlan_NoConfigRow200(t *testing.T) {
	f := setup(t)
	race := f.createRace(t, fullTri)
	p, code := f.plan(t, race.ID.String())
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, legByOrdinal(p, 3).TargetPowerLowW)
	assert.NotEmpty(t, p.MissingThresholds)
}

func TestPlan_UnknownRace404(t *testing.T) {
	f := setup(t)
	rec := f.do(t, http.MethodGet, "/races/00000000-0000-0000-0000-000000000000/pacing-plan", "", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "race_not_found")
}

func TestOverride_PutPlanRoundTripAndDelete(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	base := "/races/" + race.ID.String() + "/pacing-plan/overrides/3"

	rec := f.do(t, http.MethodPut, base, `{"target_power_low_w":190,"target_power_high_w":200,"note":"holding 195"}`, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	p, _ := f.plan(t, race.ID.String())
	bike := legByOrdinal(p, 3)
	assert.Equal(t, racepacing.SourceOverride, bike.Source)
	assert.Equal(t, 190, *bike.TargetPowerLowW)
	assert.Equal(t, 200, *bike.TargetPowerHighW)
	assert.Contains(t, bike.Rationale, "override")

	// DELETE reverts to computed.
	rec = f.do(t, http.MethodDelete, base, "", nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	p, _ = f.plan(t, race.ID.String())
	assert.Equal(t, racepacing.SourceComputed, legByOrdinal(p, 3).Source)

	// Second DELETE → override_not_found.
	rec = f.do(t, http.MethodDelete, base, "", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "override_not_found")
}

func TestOverride_SurvivesLegReplacingPatch(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	require.Equal(t, http.StatusNoContent,
		f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/3",
			`{"target_power_low_w":190,"target_power_high_w":200}`, nil).Code)

	// PATCH the race with a legs array that still has a bike leg at ordinal 3.
	patch := `{"legs":[
		{"ordinal":1,"discipline":"swim","expected_duration_min":70},
		{"ordinal":2,"discipline":"transition","expected_duration_min":5},
		{"ordinal":3,"discipline":"bike","expected_duration_min":300},
		{"ordinal":4,"discipline":"run","expected_duration_min":240}]}`
	require.Equal(t, http.StatusOK, f.do(t, http.MethodPatch, "/races/"+race.ID.String(), patch, nil).Code)

	p, _ := f.plan(t, race.ID.String())
	assert.Equal(t, racepacing.SourceOverride, legByOrdinal(p, 3).Source)
}

func TestOverride_DisciplineMismatchAtWriteAndIgnoreAtRead(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)

	// Power override on the run leg (ordinal 4) → 400 at write.
	rec := f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/4",
		`{"target_power_low_w":190,"target_power_high_w":200}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "override_discipline_mismatch")

	// Store a valid bike override at ordinal 3, then swap ordinal 3 to a run leg.
	require.Equal(t, http.StatusNoContent,
		f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/3",
			`{"target_power_low_w":190,"target_power_high_w":200}`, nil).Code)
	patch := `{"legs":[
		{"ordinal":1,"discipline":"swim","expected_duration_min":70},
		{"ordinal":3,"discipline":"run","expected_duration_min":60}]}`
	require.Equal(t, http.StatusOK, f.do(t, http.MethodPatch, "/races/"+race.ID.String(), patch, nil).Code)

	p, _ := f.plan(t, race.ID.String())
	run := legByOrdinal(p, 3)
	assert.Equal(t, racepacing.SourceComputed, run.Source) // mismatched override ignored
	assert.Contains(t, run.Rationale, "ignored")
}

func TestOverride_LegNotFound(t *testing.T) {
	f := setup(t)
	race := f.createRace(t, fullTri)
	rec := f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/99",
		`{"target_power_low_w":190,"target_power_high_w":200}`, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "leg_not_found")
}

func TestOverride_BandInvalid(t *testing.T) {
	f := setup(t)
	race := f.createRace(t, fullTri)
	// low > high on the bike leg.
	rec := f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/3",
		`{"target_power_low_w":250,"target_power_high_w":200}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "override_band_invalid")
}

func TestOverride_UnitConflict(t *testing.T) {
	f := setup(t)
	race := f.createRace(t, fullTri)
	rec := f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/3",
		`{"target_power_low_w":190,"target_power_high_w":200,"target_pace_low_sec_per_km":300,"target_pace_high_sec_per_km":320}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "override_unit_conflict")
}

func TestOverride_IdempotencyKeyOnPutRejected(t *testing.T) {
	f := setup(t)
	race := f.createRace(t, fullTri)
	rec := f.do(t, http.MethodPut, "/races/"+race.ID.String()+"/pacing-plan/overrides/3",
		`{"target_power_low_w":190,"target_power_high_w":200}`, map[string]string{"Idempotency-Key": "abc"})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "idempotency_unsupported_for_put")
}
