package macrocycle_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/macrocycle"
	"github.com/vinzenzs/kazper/internal/races"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

func init() { gin.SetMode(gin.TestMode) }

type fix struct {
	r      *gin.Engine
	phases *trainingphases.PhasesRepo
}

func setup(t *testing.T) *fix {
	t.Helper()
	pool := storetest.NewPool(t)
	macroRepo := macrocycle.NewRepo(pool)
	racesRepo := races.NewRepo(pool)
	macroSvc := macrocycle.NewService(macroRepo, racesRepo)
	phRepo := trainingphases.NewPhasesRepo(pool)
	phSvc := trainingphases.NewPhasesService(phRepo, trainingphases.NewTemplatesRepo(pool))
	phSvc.SetMacrocycleChecker(macroRepo)

	r := gin.New()
	rg := r.Group("/")
	macrocycle.NewHandlers(macroSvc).Register(rg)
	trainingphases.NewPhasesHandlers(phSvc).Register(rg)
	races.NewHandlers(races.NewService(pool, racesRepo)).Register(rg)
	return &fix{r: r, phases: phRepo}
}

func (f *fix) do(method, path, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

// createRace POSTs a race and returns its id.
func (f *fix) createRace(t *testing.T, name, date string) string {
	t.Helper()
	rec := f.do(http.MethodPost, "/races", `{"name":"`+name+`","race_date":"`+date+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var race struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &race))
	return race.ID
}

// createMacrocycle POSTs and returns the decoded macrocycle.
func (f *fix) createMacrocycle(t *testing.T, body string) *macrocycle.Macrocycle {
	t.Helper()
	rec := f.do(http.MethodPost, "/macrocycles", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp struct {
		Macrocycle *macrocycle.Macrocycle `json:"macrocycle"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Macrocycle
}

func TestCreate_HappyPath_WithRaceAnchor(t *testing.T) {
	f := setup(t)
	raceID := f.createRace(t, "IM 70.3", "2026-09-27")
	m := f.createMacrocycle(t, `{"name":"2026 season","start_date":"2026-01-05","end_date":"2026-09-27","race_id":"`+raceID+`","methodology":"build to a late peak","notes":"A-race"}`)
	assert.Equal(t, "2026 season", m.Name)
	require.NotNil(t, m.RaceID)
	require.NotNil(t, m.RaceName)
	assert.Equal(t, "IM 70.3", *m.RaceName)
	require.NotNil(t, m.Phases)
	assert.Empty(t, m.Phases)
}

func TestCreate_UnanchoredSeason(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"base block","start_date":"2026-01-05","end_date":"2026-03-01"}`)
	assert.Nil(t, m.RaceID)
	assert.Nil(t, m.RaceName)
}

func TestCreate_DateRangeInvalid(t *testing.T) {
	f := setup(t)
	rec := f.do(http.MethodPost, "/macrocycles", `{"name":"x","start_date":"2026-09-27","end_date":"2026-01-05"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "date_range_invalid")
}

func TestCreate_NameInvalid(t *testing.T) {
	f := setup(t)
	rec := f.do(http.MethodPost, "/macrocycles", `{"name":"   ","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "macrocycle_name_invalid")
}

func TestCreate_NameTooLong(t *testing.T) {
	f := setup(t)
	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	rec := f.do(http.MethodPost, "/macrocycles", `{"name":"`+string(long)+`","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "macrocycle_name_too_long")
}

func TestCreate_RaceNotFound(t *testing.T) {
	f := setup(t)
	rec := f.do(http.MethodPost, "/macrocycles", `{"name":"x","start_date":"2026-01-05","end_date":"2026-09-27","race_id":"11111111-1111-1111-1111-111111111111"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "race_not_found")
}

func TestGetByID_NestedProgressionOrdering(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	// Three phases linked with ordinals 3, 1, 2 — expect output ordered 1,2,3.
	f.createPhase(t, "peak", "build", "2026-06-01", "2026-06-28", m.ID.String(), 3, 700, 12)
	f.createPhase(t, "base", "base", "2026-01-05", "2026-02-01", m.ID.String(), 1, 500, 9)
	f.createPhase(t, "build", "build", "2026-03-01", "2026-03-28", m.ID.String(), 2, 620, 11)

	rec := f.do(http.MethodGet, "/macrocycles/"+m.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Macrocycle *macrocycle.Macrocycle `json:"macrocycle"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Macrocycle.Phases, 3)
	assert.Equal(t, "base", resp.Macrocycle.Phases[0].Name)
	assert.Equal(t, "build", resp.Macrocycle.Phases[1].Name)
	assert.Equal(t, "peak", resp.Macrocycle.Phases[2].Name)
	require.NotNil(t, resp.Macrocycle.Phases[0].TargetWeeklyTSS)
	assert.InDelta(t, 500, *resp.Macrocycle.Phases[0].TargetWeeklyTSS, 0.01)
}

func TestGetByID_EmptyPhasesArray(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	rec := f.do(http.MethodGet, "/macrocycles/"+m.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	// phases is present and an empty array, not null.
	assert.Contains(t, rec.Body.String(), `"phases":[]`)
}

func TestGetByID_NotFound(t *testing.T) {
	f := setup(t)
	rec := f.do(http.MethodGet, "/macrocycles/11111111-1111-1111-1111-111111111111", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "macrocycle_not_found")
}

func TestList_OrderedByStartDateDesc(t *testing.T) {
	f := setup(t)
	f.createMacrocycle(t, `{"name":"older","start_date":"2025-01-05","end_date":"2025-09-27"}`)
	f.createMacrocycle(t, `{"name":"newer","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	rec := f.do(http.MethodGet, "/macrocycles", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Macrocycles []*macrocycle.Macrocycle `json:"macrocycles"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Macrocycles, 2)
	assert.Equal(t, "newer", resp.Macrocycles[0].Name)
	assert.Nil(t, resp.Macrocycles[0].Phases) // list omits nested phases
}

func TestPatch_SubsetAndRaceTriState(t *testing.T) {
	f := setup(t)
	raceID := f.createRace(t, "A race", "2026-09-27")
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27","race_id":"`+raceID+`"}`)

	// Subset update: only end_date + methodology.
	rec := f.do(http.MethodPatch, "/macrocycles/"+m.ID.String(), `{"end_date":"2026-10-04","methodology":"pushed a week"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Clear the race anchor via empty string.
	rec = f.do(http.MethodPatch, "/macrocycles/"+m.ID.String(), `{"race_id":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Macrocycle *macrocycle.Macrocycle `json:"macrocycle"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Nil(t, resp.Macrocycle.RaceID)
	assert.Nil(t, resp.Macrocycle.RaceName)
	assert.Equal(t, "season", resp.Macrocycle.Name) // unchanged
}

func TestPatch_RaceNotFound(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	rec := f.do(http.MethodPatch, "/macrocycles/"+m.ID.String(), `{"race_id":"11111111-1111-1111-1111-111111111111"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "race_not_found")
}

func TestPatch_Empty(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	rec := f.do(http.MethodPatch, "/macrocycles/"+m.ID.String(), `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "patch_empty")
}

func TestDelete_OrphansMemberPhases(t *testing.T) {
	f := setup(t)
	m := f.createMacrocycle(t, `{"name":"season","start_date":"2026-01-05","end_date":"2026-09-27"}`)
	phaseID := f.createPhase(t, "base", "base", "2026-01-05", "2026-02-01", m.ID.String(), 1, 0, 0)

	rec := f.do(http.MethodDelete, "/macrocycles/"+m.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	// The phase survives, unlinked.
	pRec := f.do(http.MethodGet, "/phases/"+phaseID, "")
	require.Equal(t, http.StatusOK, pRec.Code, pRec.Body.String())
	var pResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(pRec.Body.Bytes(), &pResp))
	assert.Nil(t, pResp.Phase.MacrocycleID)

	// The macrocycle is gone.
	mRec := f.do(http.MethodGet, "/macrocycles/"+m.ID.String(), "")
	assert.Equal(t, http.StatusNotFound, mRec.Code)
}

func TestDelete_NotFound(t *testing.T) {
	f := setup(t)
	rec := f.do(http.MethodDelete, "/macrocycles/11111111-1111-1111-1111-111111111111", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "macrocycle_not_found")
}

// createPhase POSTs a phase linked to a macrocycle and returns its id. Targets
// of 0 are sent as-is (valid non-negative).
func (f *fix) createPhase(t *testing.T, name, ptype, start, end, macroID string, ordinal int, tss, hours float64) string {
	t.Helper()
	body := mustJSON(map[string]any{
		"name":               name,
		"type":               ptype,
		"start_date":         start,
		"end_date":           end,
		"macrocycle_id":      macroID,
		"macrocycle_ordinal": ordinal,
		"target_weekly_tss":  tss,
		"target_weekly_hours": hours,
	})
	rec := f.do(http.MethodPost, "/phases", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Phase.ID.String()
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
