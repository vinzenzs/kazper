package coachrecs_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/coachrecs"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type fixture struct{ r *gin.Engine }

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := coachrecs.NewService(coachrecs.NewRepo(pool))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := gin.New()
	coachrecs.NewHandlers(svc, "UTC", logger).Register(r.Group("/"))
	return &fixture{r: r}
}

func setupWithMiddleware(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := coachrecs.NewService(coachrecs.NewRepo(pool))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	coachrecs.NewHandlers(svc, "UTC", logger).Register(r.Group("/"))
	return &fixture{r: r}
}

func doReq(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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
	r.ServeHTTP(rec, req)
	return rec
}

func create(t *testing.T, f *fixture, body string) coachrecs.Recommendation {
	t.Helper()
	rec := doReq(t, f.r, http.MethodPost, "/coach/recommendations", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var out coachrecs.Recommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// ----- POST -----

func TestCreate_HappyPath(t *testing.T) {
	f := setup(t)
	out := create(t, f, `{"date":"2026-06-17","scope":"fueling","recommendation":"Target 220g carbs","reason":"long ride tomorrow"}`)
	assert.NotEqual(t, "", out.ID.String())
	assert.Equal(t, "2026-06-17", out.Date)
	assert.Equal(t, coachrecs.ScopeFueling, out.Scope)
	assert.Equal(t, "Target 220g carbs", out.Recommendation)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "long ride tomorrow", *out.Reason)
}

func TestCreate_ReasonOptional(t *testing.T) {
	f := setup(t)
	out := create(t, f, `{"date":"2026-06-17","scope":"general","recommendation":"Sleep 8h"}`)
	assert.Nil(t, out.Reason)
}

func TestCreate_RecommendationRequired(t *testing.T) {
	f := setup(t)
	for _, body := range []string{
		`{"date":"2026-06-17","scope":"fueling","recommendation":""}`,
		`{"date":"2026-06-17","scope":"fueling","recommendation":"   "}`,
		`{"date":"2026-06-17","scope":"fueling"}`,
	} {
		rec := doReq(t, f.r, http.MethodPost, "/coach/recommendations", body, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"recommendation_required"}`, rec.Body.String())
	}
}

func TestCreate_ScopeInvalid(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/coach/recommendations",
		`{"date":"2026-06-17","scope":"nonsense","recommendation":"x"}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"scope_invalid"}`, rec.Body.String())
}

func TestCreate_DateInvalid(t *testing.T) {
	f := setup(t)
	for _, body := range []string{
		`{"date":"nope","scope":"fueling","recommendation":"x"}`,
		`{"scope":"fueling","recommendation":"x"}`,
	} {
		rec := doReq(t, f.r, http.MethodPost, "/coach/recommendations", body, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
	}
}

// ----- GET list -----

func listRecs(t *testing.T, f *fixture, query string) []coachrecs.Recommendation {
	t.Helper()
	rec := doReq(t, f.r, http.MethodGet, "/coach/recommendations?"+query, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Recommendations []coachrecs.Recommendation `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Recommendations
}

func TestList_WindowNewestFirstAndScopeFilter(t *testing.T) {
	f := setup(t)
	create(t, f, `{"date":"2026-06-15","scope":"fueling","recommendation":"old fueling"}`)
	create(t, f, `{"date":"2026-06-17","scope":"recovery","recommendation":"newer recovery"}`)
	create(t, f, `{"date":"2026-06-18","scope":"fueling","recommendation":"newest fueling"}`)
	// Out of window (before from).
	create(t, f, `{"date":"2026-06-10","scope":"fueling","recommendation":"way old"}`)

	got := listRecs(t, f, "from=2026-06-15&to=2026-06-18&tz=UTC")
	require.Len(t, got, 3, "the 2026-06-10 row is outside the window")
	assert.Equal(t, "newest fueling", got[0].Recommendation, "newest date first")
	assert.Equal(t, "newer recovery", got[1].Recommendation)
	assert.Equal(t, "old fueling", got[2].Recommendation)

	fueling := listRecs(t, f, "from=2026-06-15&to=2026-06-18&scope=fueling")
	require.Len(t, fueling, 2)
	for _, r := range fueling {
		assert.Equal(t, coachrecs.ScopeFueling, r.Scope)
	}
}

func TestList_Validation(t *testing.T) {
	f := setup(t)
	cases := []struct{ query, want string }{
		{"", "window_required"},
		{"from=nope&to=2026-06-18", "date_invalid"},
		{"from=2026-06-18&to=2026-06-15", "window_invalid"},
		{"from=2026-06-15&to=2026-06-18&tz=Mars/Phobos", "tz_invalid"},
		{"from=2026-06-15&to=2026-06-18&scope=bogus", "scope_invalid"},
	}
	for _, tc := range cases {
		rec := doReq(t, f.r, http.MethodGet, "/coach/recommendations?"+tc.query, "", nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, tc.query)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, tc.want, body["error"], tc.query)
	}
}

// ----- GET one / DELETE -----

func TestGetAndDelete(t *testing.T) {
	f := setup(t)
	out := create(t, f, `{"date":"2026-06-17","scope":"training","recommendation":"easy spin"}`)

	got := doReq(t, f.r, http.MethodGet, "/coach/recommendations/"+out.ID.String(), "", nil)
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())

	del := doReq(t, f.r, http.MethodDelete, "/coach/recommendations/"+out.ID.String(), "", nil)
	require.Equal(t, http.StatusNoContent, del.Code)

	gone := doReq(t, f.r, http.MethodGet, "/coach/recommendations/"+out.ID.String(), "", nil)
	require.Equal(t, http.StatusNotFound, gone.Code)
	assert.JSONEq(t, `{"error":"recommendation_not_found"}`, gone.Body.String())
}

func TestGetAndDelete_MissingIs404(t *testing.T) {
	f := setup(t)
	missing := "11111111-1111-1111-1111-111111111111"
	get := doReq(t, f.r, http.MethodGet, "/coach/recommendations/"+missing, "", nil)
	require.Equal(t, http.StatusNotFound, get.Code)
	del := doReq(t, f.r, http.MethodDelete, "/coach/recommendations/"+missing, "", nil)
	require.Equal(t, http.StatusNotFound, del.Code)
}

// ----- idempotency (middleware) -----

func TestIdempotency_ReplayOnPost(t *testing.T) {
	f := setupWithMiddleware(t)
	body := `{"date":"2026-06-17","scope":"fueling","recommendation":"Target 220g carbs"}`
	headers := map[string]string{
		"Authorization":   "Bearer " + agentToken,
		"Idempotency-Key": "coachrec-key-1",
	}
	first := doReq(t, f.r, http.MethodPost, "/coach/recommendations", body, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	second := doReq(t, f.r, http.MethodPost, "/coach/recommendations", body, headers)
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, first.Body.String(), second.Body.String(), "replay returns the original body verbatim")
}
