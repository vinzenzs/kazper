package supplements_test

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

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/supplements"
)

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := supplements.NewService(supplements.NewRepo(pool))
	r := gin.New()
	supplements.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func setupWithMiddleware(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := supplements.NewService(supplements.NewRepo(pool))
	idemRepo := idempotency.NewRepo(pool)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	supplements.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	s, _ := m["error"].(string)
	return s
}

func entry(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m struct {
		Supplement map[string]any `json:"supplement"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m.Supplement
}

func TestCreate_BareName(t *testing.T) {
	r := setup(t)
	rec := do(r, http.MethodPost, "/supplements", `{"name":"vitamin D","logged_at":"2026-07-14T08:00:00Z"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	e := entry(t, rec)
	assert.Equal(t, "vitamin D", e["name"])
	_, hasDose := e["dose"]
	assert.False(t, hasDose, "bare-name entry has no dose")
}

func TestCreate_DosePairMatrix(t *testing.T) {
	r := setup(t)
	cases := []struct{ body, code string; status int }{
		{`{"name":"creatine","dose":5,"dose_unit":"g","logged_at":"2026-07-14T08:00:00Z"}`, "", http.StatusCreated},
		{`{"name":"creatine","dose":5,"logged_at":"2026-07-14T08:00:00Z"}`, "dose_pair_required", http.StatusBadRequest},
		{`{"name":"creatine","dose_unit":"g","logged_at":"2026-07-14T08:00:00Z"}`, "dose_pair_required", http.StatusBadRequest},
		{`{"name":"creatine","dose":0,"dose_unit":"g","logged_at":"2026-07-14T08:00:00Z"}`, "dose_invalid", http.StatusBadRequest},
		{`{"logged_at":"2026-07-14T08:00:00Z"}`, "name_required", http.StatusBadRequest},
		{`{"name":"x"}`, "logged_at_required", http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := do(r, http.MethodPost, "/supplements", c.body, nil)
		require.Equal(t, c.status, rec.Code, c.body)
		if c.code != "" {
			assert.Equal(t, c.code, errCode(t, rec), c.body)
		}
	}
}

func TestList_AscendingWindowAndEmpty(t *testing.T) {
	r := setup(t)
	for _, ts := range []string{"2026-07-14T18:00:00Z", "2026-07-14T07:00:00Z", "2026-07-14T12:00:00Z"} {
		do(r, http.MethodPost, "/supplements", `{"name":"mag","logged_at":"`+ts+`"}`, nil)
	}
	rec := do(r, http.MethodGet, "/supplements?from=2026-07-14T00:00:00Z&to=2026-07-14T23:59:59Z", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Entries, 3)
	// Ascending by instant (the DB may render logged_at in its session tz).
	parse := func(i int) time.Time {
		ts, err := time.Parse(time.RFC3339, out.Entries[i]["logged_at"].(string))
		require.NoError(t, err)
		return ts
	}
	assert.True(t, parse(0).Equal(time.Date(2026, 7, 14, 7, 0, 0, 0, time.UTC)))
	assert.True(t, parse(0).Before(parse(1)) && parse(1).Before(parse(2)), "ascending")

	empty := do(r, http.MethodGet, "/supplements?from=2026-06-01T00:00:00Z&to=2026-06-02T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, empty.Code)
	assert.JSONEq(t, `{"entries":[]}`, empty.Body.String())
}

func TestList_RangeErrors(t *testing.T) {
	r := setup(t)
	cases := []struct{ path, code string }{
		{"/supplements", "range_required"},
		{"/supplements?from=nope&to=2026-07-14T00:00:00Z", "date_invalid"},
		{"/supplements?from=2026-07-14T00:00:00Z&to=2026-07-13T00:00:00Z", "range_invalid"},
		{"/supplements?from=2026-01-01T00:00:00Z&to=2026-07-01T00:00:00Z", "range_too_large"},
	}
	for _, c := range cases {
		rec := do(r, http.MethodGet, c.path, "", nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, c.path)
		assert.Equal(t, c.code, errCode(t, rec), c.path)
	}
}

func TestGetDelete_RoundTripAnd404(t *testing.T) {
	r := setup(t)
	rec := do(r, http.MethodPost, "/supplements", `{"name":"iron","dose":18,"dose_unit":"mg","logged_at":"2026-07-14T08:00:00Z"}`, nil)
	id := entry(t, rec)["id"].(string)

	got := do(r, http.MethodGet, "/supplements/"+id, "", nil)
	require.Equal(t, http.StatusOK, got.Code)
	assert.EqualValues(t, 18, entry(t, got)["dose"])

	del := do(r, http.MethodDelete, "/supplements/"+id, "", nil)
	require.Equal(t, http.StatusNoContent, del.Code)
	after := do(r, http.MethodGet, "/supplements/"+id, "", nil)
	require.Equal(t, http.StatusNotFound, after.Code)
	assert.Equal(t, "not_found", errCode(t, after))
	delAgain := do(r, http.MethodDelete, "/supplements/"+id, "", nil)
	require.Equal(t, http.StatusNotFound, delAgain.Code)
}

func TestCreate_IdempotentReplay(t *testing.T) {
	r := setupWithMiddleware(t)
	h := map[string]string{"Authorization": "Bearer " + mobileToken, "Idempotency-Key": "sup-1"}
	body := `{"name":"creatine","dose":5,"dose_unit":"g","logged_at":"2026-07-14T08:00:00Z"}`
	a := do(r, http.MethodPost, "/supplements", body, h)
	b := do(r, http.MethodPost, "/supplements", body, h)
	require.Equal(t, http.StatusCreated, a.Code, a.Body.String())
	assert.Equal(t, a.Body.String(), b.Body.String(), "replay returns the cached response")
}

func TestNoPatchRoute(t *testing.T) {
	r := setup(t)
	rec := do(r, http.MethodPost, "/supplements", `{"name":"x","logged_at":"2026-07-14T08:00:00Z"}`, nil)
	id := entry(t, rec)["id"].(string)
	// PATCH is not registered → gin returns 404 (no route).
	patch := do(r, http.MethodPatch, "/supplements/"+id, `{"name":"y"}`, nil)
	assert.Equal(t, http.StatusNotFound, patch.Code)
}
