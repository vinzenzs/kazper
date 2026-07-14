package wellness_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/wellness"
)

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := wellness.NewService(wellness.NewRepo(pool))
	r := gin.New()
	rg := r.Group("/")
	wellness.NewHandlers(svc).Register(rg)
	return r
}

// setupWithMiddleware mounts the idempotency middleware so the PUT-key rejection
// (enforced centrally, not in the handler) is exercised.
func setupWithMiddleware(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := wellness.NewService(wellness.NewRepo(pool))
	idemRepo := idempotency.NewRepo(pool)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	wellness.NewHandlers(svc).Register(rg)
	return r
}

func doReq(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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
	r.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m
}

// entry pulls the {"wellness": ...} envelope out of a PUT/GET response.
func entry(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m struct {
		Wellness map[string]any `json:"wellness"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m.Wellness
}

// ============================================================================
// PUT /wellness/{date}
// ============================================================================

func TestPut_PartialEntryIsFirstClass(t *testing.T) {
	r := setup(t)
	rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"soreness":4}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	e := entry(t, rec)
	assert.EqualValues(t, 4, e["soreness"])
	// Only the provided field echoes back — omitempty drops the rest.
	for _, k := range []string{"fatigue", "stress", "mood", "motivation", "note"} {
		_, present := e[k]
		assert.False(t, present, "field %q should be omitted", k)
	}
	assert.Equal(t, "2026-07-14", e["date"])
}

func TestPut_ReplacesRatherThanMerges(t *testing.T) {
	r := setup(t)
	first := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"fatigue":3,"note":"heavy legs"}`, nil)
	require.Equal(t, http.StatusOK, first.Code)

	second := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"mood":4}`, nil)
	require.Equal(t, http.StatusOK, second.Code)

	e := entry(t, second)
	assert.EqualValues(t, 4, e["mood"])
	_, hasFatigue := e["fatigue"]
	_, hasNote := e["note"]
	assert.False(t, hasFatigue, "full-replace should clear fatigue")
	assert.False(t, hasNote, "full-replace should clear note")
}

func TestPut_EmptyEntryRejected(t *testing.T) {
	r := setup(t)
	rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "wellness_empty", errCode(t, rec)["error"])
}

func TestPut_NoteOnlyEntryIsValid(t *testing.T) {
	r := setup(t)
	rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"note":"felt off, no numbers"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "felt off, no numbers", entry(t, rec)["note"])
}

func TestPut_ScoreRangeMatrixNamesField(t *testing.T) {
	r := setup(t)
	cases := []struct {
		body, field string
	}{
		{`{"mood":7}`, "mood"},
		{`{"mood":0}`, "mood"},
		{`{"fatigue":6}`, "fatigue"},
		{`{"soreness":-1}`, "soreness"},
		{`{"stress":9}`, "stress"},
		{`{"motivation":0}`, "motivation"},
	}
	for _, c := range cases {
		rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", c.body, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, c.body)
		m := errCode(t, rec)
		assert.Equal(t, "wellness_score_invalid", m["error"], c.body)
		assert.Equal(t, c.field, m["field"], c.body)
	}
}

func TestPut_NonIntegerScoreNamesField(t *testing.T) {
	r := setup(t)
	for _, body := range []string{`{"mood":3.5}`, `{"fatigue":"high"}`} {
		rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", body, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		m := errCode(t, rec)
		assert.Equal(t, "wellness_score_invalid", m["error"], body)
		assert.NotEmpty(t, m["field"], body)
	}
}

func TestPut_NoteTooLong(t *testing.T) {
	r := setup(t)
	body := `{"note":"` + strings.Repeat("x", 2001) + `"}`
	rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", body, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "note_too_long", errCode(t, rec)["error"])
}

func TestPut_BadDate(t *testing.T) {
	r := setup(t)
	rec := doReq(r, http.MethodPut, "/wellness/not-a-date", `{"mood":3}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "date_invalid", errCode(t, rec)["error"])
}

func TestPut_IdempotencyKeyRejected(t *testing.T) {
	r := setupWithMiddleware(t)
	rec := doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"mood":3}`, map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "abc-123",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "idempotency_unsupported_for_put", errCode(t, rec)["error"])
}

// ============================================================================
// GET / DELETE /wellness/{date}
// ============================================================================

func TestGetDelete_RoundTripAnd404s(t *testing.T) {
	r := setup(t)

	// Unlogged day → 404.
	miss := doReq(r, http.MethodGet, "/wellness/2026-07-14", "", nil)
	require.Equal(t, http.StatusNotFound, miss.Code)
	assert.Equal(t, "not_found", errCode(t, miss)["error"])

	// Log then read back.
	doReq(r, http.MethodPut, "/wellness/2026-07-14", `{"fatigue":2,"motivation":5}`, nil)
	got := doReq(r, http.MethodGet, "/wellness/2026-07-14", "", nil)
	require.Equal(t, http.StatusOK, got.Code)
	e := entry(t, got)
	assert.EqualValues(t, 2, e["fatigue"])
	assert.EqualValues(t, 5, e["motivation"])

	// Delete → 204, then GET and DELETE both 404.
	del := doReq(r, http.MethodDelete, "/wellness/2026-07-14", "", nil)
	require.Equal(t, http.StatusNoContent, del.Code)

	after := doReq(r, http.MethodGet, "/wellness/2026-07-14", "", nil)
	require.Equal(t, http.StatusNotFound, after.Code)
	delAgain := doReq(r, http.MethodDelete, "/wellness/2026-07-14", "", nil)
	require.Equal(t, http.StatusNotFound, delAgain.Code)
	assert.Equal(t, "not_found", errCode(t, delAgain)["error"])
}

// ============================================================================
// GET /wellness?from=&to=
// ============================================================================

func TestList_AscendingWindow(t *testing.T) {
	r := setup(t)
	for _, d := range []string{"2026-07-10", "2026-07-12", "2026-07-14"} {
		doReq(r, http.MethodPut, "/wellness/"+d, `{"mood":3}`, nil)
	}
	// A date outside the window must not appear.
	doReq(r, http.MethodPut, "/wellness/2026-06-01", `{"mood":1}`, nil)

	rec := doReq(r, http.MethodGet, "/wellness?from=2026-07-09&to=2026-07-15", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Entries, 3)
	assert.Equal(t, "2026-07-10", out.Entries[0]["date"])
	assert.Equal(t, "2026-07-12", out.Entries[1]["date"])
	assert.Equal(t, "2026-07-14", out.Entries[2]["date"])
}

func TestList_EmptyIs200(t *testing.T) {
	r := setup(t)
	rec := doReq(r, http.MethodGet, "/wellness?from=2026-07-09&to=2026-07-15", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entries":[]}`, rec.Body.String())
}

func TestList_400Matrix(t *testing.T) {
	r := setup(t)
	cases := []struct {
		path, code string
	}{
		{"/wellness", "range_required"},
		{"/wellness?from=2026-07-09", "range_required"},
		{"/wellness?from=bad&to=2026-07-15", "date_invalid"},
		{"/wellness?from=2026-07-09&to=bad", "date_invalid"},
		{"/wellness?from=2026-07-15&to=2026-07-09", "range_invalid"},
		{"/wellness?from=2026-01-01&to=2026-07-15", "range_too_large"},
	}
	for _, c := range cases {
		rec := doReq(r, http.MethodGet, c.path, "", nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, c.path)
		assert.Equal(t, c.code, errCode(t, rec)["error"], c.path)
	}
}
