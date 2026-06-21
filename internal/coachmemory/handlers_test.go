package coachmemory_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/coachmemory"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r    *gin.Engine
	repo *coachmemory.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := coachmemory.NewRepo(pool)
	svc := coachmemory.NewService(repo)
	r := gin.New()
	coachmemory.NewHandlers(svc, "UTC", nil).Register(r.Group("/"))
	return &fixture{r: r, repo: repo}
}

func doReq(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func today() string { return time.Now().UTC().Format("2006-01-02") }

// ---------------------------------------------------------------------------
// POST /coach/memory
// ---------------------------------------------------------------------------

func TestPost_StandingFact_NoDate_Returns201(t *testing.T) {
	f := setup(t)
	body := `{"kind":"constraint","text":"Right knee niggle, easy running only","review_at":"2026-07-05"}`
	rec := doReq(t, f.r, http.MethodPost, "/coach/memory", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m coachmemory.Memory
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Equal(t, coachmemory.KindConstraint, m.Kind)
	assert.Equal(t, coachmemory.StatusActive, m.Status)
	assert.Nil(t, m.Date)
	require.NotNil(t, m.ReviewAt)
	assert.Equal(t, "2026-07-05", *m.ReviewAt)
}

func TestPost_RecommendationWithoutDate_Returns400(t *testing.T) {
	f := setup(t)
	body := `{"kind":"recommendation","scope":"fueling","text":"Target 220g carbs"}`
	rec := doReq(t, f.r, http.MethodPost, "/coach/memory", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_required"}`, rec.Body.String())
}

func TestPost_TextRequired(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"   "}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"text_required"}`, rec.Body.String())
}

func TestPost_KindInvalid(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"reminder","text":"x"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"kind_invalid"}`, rec.Body.String())
}

func TestPost_DateInvalid(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"x","review_at":"07-05-2026"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

// ---------------------------------------------------------------------------
// GET /coach/memory (list)
// ---------------------------------------------------------------------------

func TestList_StandingReturnedRegardlessOfWindow_RecommendationsFiltered(t *testing.T) {
	f := setup(t)
	// A dateless standing preference, plus a recommendation dated far outside the
	// requested window.
	require.Equal(t, http.StatusCreated, doReq(t, f.r, http.MethodPost, "/coach/memory",
		`{"kind":"preference","text":"prefers gels over drink mix"}`).Code)
	require.Equal(t, http.StatusCreated, doReq(t, f.r, http.MethodPost, "/coach/memory",
		`{"kind":"recommendation","scope":"fueling","text":"old advice","date":"2026-01-01"}`).Code)

	rec := doReq(t, f.r, http.MethodGet, "/coach/memory?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Memory []coachmemory.Memory `json:"memory"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Memory, 1)
	assert.Equal(t, coachmemory.KindPreference, resp.Memory[0].Kind)
}

func TestList_ExpiredExcludedByDefault(t *testing.T) {
	f := setup(t)
	// expires_at in the past → excluded from the default list.
	body := `{"kind":"observation","text":"stale note","expires_at":"2020-01-01"}`
	require.Equal(t, http.StatusCreated, doReq(t, f.r, http.MethodPost, "/coach/memory", body).Code)
	rec := doReq(t, f.r, http.MethodGet, "/coach/memory?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Memory []coachmemory.Memory `json:"memory"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Memory)
}

// ---------------------------------------------------------------------------
// PATCH /coach/memory/{id}
// ---------------------------------------------------------------------------

func TestPatch_ConfirmReviewAt_PreservesCreatedAt(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/coach/memory",
		`{"kind":"constraint","text":"knee","review_at":"2026-06-01"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var created coachmemory.Memory
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &created))

	patchRec := doReq(t, f.r, http.MethodPatch, "/coach/memory/"+created.ID.String(),
		`{"review_at":"2026-07-20"}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var got coachmemory.Memory
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &got))
	require.NotNil(t, got.ReviewAt)
	assert.Equal(t, "2026-07-20", *got.ReviewAt)
	assert.WithinDuration(t, created.CreatedAt, got.CreatedAt, time.Millisecond)
}

func TestPatch_ContentEditRejected(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"x"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var m coachmemory.Memory
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &m))

	patchRec := doReq(t, f.r, http.MethodPatch, "/coach/memory/"+m.ID.String(), `{"text":"different"}`)
	require.Equal(t, http.StatusBadRequest, patchRec.Code)
	assert.Contains(t, patchRec.Body.String(), "field_immutable")
}

func TestPatch_ArchiveHidesFromDefaultList(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"archive me"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var m coachmemory.Memory
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &m))

	patchRec := doReq(t, f.r, http.MethodPatch, "/coach/memory/"+m.ID.String(), `{"status":"archived"}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())

	win := fmt.Sprintf("/coach/memory?from=%s&to=%s", today(), today())
	def := doReq(t, f.r, http.MethodGet, win, "")
	assert.NotContains(t, def.Body.String(), "archive me")
	inc := doReq(t, f.r, http.MethodGet, win+"&include_archived=true", "")
	assert.Contains(t, inc.Body.String(), "archive me")
}

func TestPatch_StatusInvalid(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"x"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var m coachmemory.Memory
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &m))
	rec := doReq(t, f.r, http.MethodPatch, "/coach/memory/"+m.ID.String(), `{"status":"deleted"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"status_invalid"}`, rec.Body.String())
}

func TestGetAndDelete_404OnMissing(t *testing.T) {
	f := setup(t)
	missing := "/coach/memory/00000000-0000-0000-0000-000000000000"
	assert.Equal(t, http.StatusNotFound, doReq(t, f.r, http.MethodGet, missing, "").Code)
	assert.Equal(t, http.StatusNotFound, doReq(t, f.r, http.MethodPatch, missing, `{"status":"archived"}`).Code)
	assert.Equal(t, http.StatusNotFound, doReq(t, f.r, http.MethodDelete, missing, "").Code)
}

func TestDelete_RemovesItem(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/coach/memory", `{"kind":"fact","text":"x"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var m coachmemory.Memory
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &m))
	require.Equal(t, http.StatusNoContent, doReq(t, f.r, http.MethodDelete, "/coach/memory/"+m.ID.String(), "").Code)
	assert.Equal(t, http.StatusNotFound, doReq(t, f.r, http.MethodGet, "/coach/memory/"+m.ID.String(), "").Code)
}
