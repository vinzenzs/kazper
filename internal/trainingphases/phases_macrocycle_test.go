package trainingphases_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/macrocycle"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

// macroFix wires the phase + macrocycle handlers plus a goals resolver, so the
// macrocycle-membership plumbing and the adherence-invariance can be exercised
// together.
type macroFix struct {
	r        *gin.Engine
	resolver *goals.Resolver
}

func setupMacroFix(t *testing.T) *macroFix {
	t.Helper()
	pool := storetest.NewPool(t)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	tplSvc := trainingphases.NewTemplatesService(tplRepo)
	phRepo := trainingphases.NewPhasesRepo(pool)
	phSvc := trainingphases.NewPhasesService(phRepo, tplRepo)
	macroRepo := macrocycle.NewRepo(pool)
	phSvc.SetMacrocycleChecker(macroRepo)
	macroSvc := macrocycle.NewService(macroRepo, nil) // no race anchor needed here

	goalsRepo := goals.NewRepo(pool)
	overridesRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(
		goalsRepo, overridesRepo,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)

	r := gin.New()
	rg := r.Group("/")
	trainingphases.NewTemplatesHandlers(tplSvc).Register(rg)
	trainingphases.NewPhasesHandlers(phSvc).Register(rg)
	macrocycle.NewHandlers(macroSvc).Register(rg)
	return &macroFix{r: r, resolver: resolver}
}

func (f *macroFix) do(method, path, body string) *httptest.ResponseRecorder {
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

func (f *macroFix) createMacrocycle(t *testing.T) string {
	t.Helper()
	rec := f.do(http.MethodPost, "/macrocycles", `{"name":"season","start_date":"2026-01-05","end_date":"2026-12-31"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp struct {
		Macrocycle *macrocycle.Macrocycle `json:"macrocycle"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Macrocycle.ID.String()
}

func (f *macroFix) phase(t *testing.T, rec *httptest.ResponseRecorder) *trainingphases.Phase {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Phase
}

func TestPhase_OmittedMacrocycleFieldsAreNull(t *testing.T) {
	f := setupMacroFix(t)
	rec := f.do(http.MethodPost, "/phases", `{"name":"p","type":"base","start_date":"2026-01-05","end_date":"2026-02-01"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	// omitempty: the four fields are absent in the response.
	body := rec.Body.String()
	assert.NotContains(t, body, "macrocycle_id")
	assert.NotContains(t, body, "macrocycle_ordinal")
	assert.NotContains(t, body, "target_weekly_tss")
	assert.NotContains(t, body, "target_weekly_hours")
}

func TestPhase_PostAcceptsMacrocycleFields(t *testing.T) {
	f := setupMacroFix(t)
	macroID := f.createMacrocycle(t)
	rec := f.do(http.MethodPost, "/phases", `{"name":"build","type":"build","start_date":"2026-03-01","end_date":"2026-03-28","macrocycle_id":"`+macroID+`","macrocycle_ordinal":2,"target_weekly_tss":620.04,"target_weekly_hours":11.5}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Phase.MacrocycleID)
	assert.Equal(t, macroID, resp.Phase.MacrocycleID.String())
	require.NotNil(t, resp.Phase.MacrocycleOrdinal)
	assert.Equal(t, 2, *resp.Phase.MacrocycleOrdinal)
	require.NotNil(t, resp.Phase.TargetWeeklyTSS)
	assert.InDelta(t, 620.0, *resp.Phase.TargetWeeklyTSS, 0.001) // rounded to 1dp
}

func TestPhase_PostMacrocycleNotFound(t *testing.T) {
	f := setupMacroFix(t)
	rec := f.do(http.MethodPost, "/phases", `{"name":"p","type":"base","start_date":"2026-01-05","end_date":"2026-02-01","macrocycle_id":"11111111-1111-1111-1111-111111111111"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "macrocycle_not_found")
}

func TestPhase_PostNegativeTargetRejected(t *testing.T) {
	f := setupMacroFix(t)
	rec := f.do(http.MethodPost, "/phases", `{"name":"p","type":"base","start_date":"2026-01-05","end_date":"2026-02-01","target_weekly_tss":-5}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "target_invalid")
}

func TestPhase_PatchTriStateMacrocycleClear(t *testing.T) {
	f := setupMacroFix(t)
	macroID := f.createMacrocycle(t)
	createRec := f.do(http.MethodPost, "/phases", `{"name":"p","type":"base","start_date":"2026-01-05","end_date":"2026-02-01","macrocycle_id":"`+macroID+`"}`)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	id := created.Phase.ID.String()

	// Empty string clears the link.
	p := f.phase(t, f.do(http.MethodPatch, "/phases/"+id, `{"macrocycle_id":""}`))
	assert.Nil(t, p.MacrocycleID)

	// Setting it again re-links.
	p = f.phase(t, f.do(http.MethodPatch, "/phases/"+id, `{"macrocycle_id":"`+macroID+`"}`))
	require.NotNil(t, p.MacrocycleID)
	assert.Equal(t, macroID, p.MacrocycleID.String())

	// Omitting it leaves the link unchanged (patch a different field).
	p = f.phase(t, f.do(http.MethodPatch, "/phases/"+id, `{"target_weekly_tss":700}`))
	require.NotNil(t, p.MacrocycleID)
	assert.Equal(t, macroID, p.MacrocycleID.String())
}

func TestPhase_LinkingDoesNotChangeAdherence(t *testing.T) {
	f := setupMacroFix(t)
	// A template + a phase pointing at it drives phase_template adherence.
	tplRec := f.do(http.MethodPut, "/goal-templates/build-bounds", `{"kcal":{"min":2090,"max":2310}}`)
	require.Equal(t, http.StatusOK, tplRec.Code, tplRec.Body.String())
	var tpl struct {
		Template *trainingphases.Template `json:"template"`
	}
	require.NoError(t, json.Unmarshal(tplRec.Body.Bytes(), &tpl))

	createRec := f.do(http.MethodPost, "/phases", `{"name":"build","type":"build","start_date":"2026-03-01","end_date":"2026-03-28","default_template_id":"`+tpl.Template.ID.String()+`"}`)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	_, srcBefore, phaseBefore, err := f.resolver.EffectiveFor(context.Background(), date)
	require.NoError(t, err)
	require.Equal(t, goals.GoalSourcePhaseTemplate, srcBefore)
	require.Equal(t, "build", phaseBefore)

	// Link the phase into a season — adherence must be unchanged.
	macroID := f.createMacrocycle(t)
	_ = f.phase(t, f.do(http.MethodPatch, "/phases/"+created.Phase.ID.String(), `{"macrocycle_id":"`+macroID+`","macrocycle_ordinal":1,"target_weekly_tss":640}`))

	_, srcAfter, phaseAfter, err := f.resolver.EffectiveFor(context.Background(), date)
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourcePhaseTemplate, srcAfter)
	assert.Equal(t, "build", phaseAfter)
}
