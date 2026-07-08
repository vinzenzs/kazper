package garminsyncstatus_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/garminsyncstatus"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
	garminToken = "garmin-token-cccccccccccccc"
)

type fixture struct {
	r    *gin.Engine
	pool *pgxpool.Pool
}

func setup(t *testing.T, enabled bool) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	authCfg := auth.Config{MobileToken: mobileToken, AgentToken: agentToken}
	if enabled {
		authCfg.GarminToken = garminToken
	}
	r := gin.New()
	r.Use(auth.Middleware(authCfg))
	rg := r.Group("/")
	garminsyncstatus.NewHandlers(garminsyncstatus.NewService(garminsyncstatus.NewRepo(pool)), enabled).Register(rg)
	return &fixture{r: r, pool: pool}
}

func doReq(t *testing.T, r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// openRun POSTs a run as the garmin identity and returns its id.
func openRun(t *testing.T, f *fixture, body string) string {
	t.Helper()
	rec := doReq(t, f.r, http.MethodPost, "/garmin/sync-runs", garminToken, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var run garminsyncstatus.SyncRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	return run.ID.String()
}

func TestOpenThenCloseSuccess(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodPost, "/garmin/sync-runs", garminToken,
		`{"window_from":"2026-06-20","window_to":"2026-06-22"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var run garminsyncstatus.SyncRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	assert.Equal(t, garminsyncstatus.StatusRunning, run.Status)
	assert.Nil(t, run.FinishedAt)
	require.NotNil(t, run.WindowFrom)
	assert.Equal(t, "2026-06-20", *run.WindowFrom)

	closeRec := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+run.ID.String(), garminToken, `{"status":"success"}`)
	require.Equal(t, http.StatusOK, closeRec.Code, closeRec.Body.String())
	var closed garminsyncstatus.SyncRun
	require.NoError(t, json.Unmarshal(closeRec.Body.Bytes(), &closed))
	assert.Equal(t, garminsyncstatus.StatusSuccess, closed.Status)
	require.NotNil(t, closed.FinishedAt)
}

func TestCloseError_StoresMessage(t *testing.T) {
	f := setup(t, true)
	id := openRun(t, f, "")
	rec := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+id, garminToken,
		`{"status":"error","error":"garmin 429 rate limited"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var run garminsyncstatus.SyncRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	assert.Equal(t, garminsyncstatus.StatusError, run.Status)
	require.NotNil(t, run.Error)
	assert.Contains(t, *run.Error, "rate limited")
}

func TestStatus_LatestVsLastSuccessfulIndependent(t *testing.T) {
	f := setup(t, true)
	// One successful run, then a later error run.
	ok := openRun(t, f, "")
	require.Equal(t, http.StatusOK, doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+ok, garminToken, `{"status":"success"}`).Code)
	bad := openRun(t, f, "")
	require.Equal(t, http.StatusOK, doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+bad, garminToken, `{"status":"error","error":"boom"}`).Code)

	rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status", mobileToken, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var st garminsyncstatus.SyncStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	require.NotNil(t, st.Latest)
	assert.Equal(t, garminsyncstatus.StatusError, st.Latest.Status, "latest reflects the error run")
	require.NotNil(t, st.LastSuccessfulAt, "last_successful_at survives a later failure")
	assert.False(t, st.IsStale, "a success within the threshold is not stale")
}

func TestStatus_NoRuns(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status", mobileToken, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var st garminsyncstatus.SyncStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	assert.Nil(t, st.Latest)
	assert.Nil(t, st.LastSuccessfulAt)
	assert.True(t, st.IsStale)
}

func TestWrites_RequireGarminIdentity(t *testing.T) {
	f := setup(t, true)
	for _, tok := range []string{mobileToken, agentToken} {
		post := doReq(t, f.r, http.MethodPost, "/garmin/sync-runs", tok, "")
		assert.Equal(t, http.StatusForbidden, post.Code, "POST with %s", tok)
		patch := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/00000000-0000-0000-0000-000000000000", tok, `{"status":"success"}`)
		assert.Equal(t, http.StatusForbidden, patch.Code, "PATCH with %s", tok)
	}
}

func TestStatus_ReadableByMobileAndAgent(t *testing.T) {
	f := setup(t, true)
	for _, tok := range []string{mobileToken, agentToken} {
		rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status", tok, "")
		assert.Equal(t, http.StatusOK, rec.Code, "GET with %s", tok)
	}
}

func TestClose_UnknownIs404(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/11111111-1111-1111-1111-111111111111", garminToken, `{"status":"success"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"sync_run_not_found"}`, rec.Body.String())
}

func TestClose_InvalidStatusIs400(t *testing.T) {
	f := setup(t, true)
	id := openRun(t, f, "")
	for _, bad := range []string{`{"status":"running"}`, `{"status":"done"}`} {
		rec := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+id, garminToken, bad)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
		assert.JSONEq(t, `{"error":"status_invalid"}`, rec.Body.String())
	}
}

func TestUnconfigured_Returns503(t *testing.T) {
	f := setup(t, false)
	// Even with a (now-unrecognized) garmin token, disabled short-circuits to 503.
	assert.Equal(t, http.StatusServiceUnavailable, doReq(t, f.r, http.MethodGet, "/garmin/sync-status", mobileToken, "").Code)
	assert.Equal(t, http.StatusServiceUnavailable, doReq(t, f.r, http.MethodPost, "/garmin/sync-runs", mobileToken, "").Code)
}

// --- async-backfill additions (garmin-bridge-call-resilience) ---

func TestClosePartial_WithSummary_RoundTrips(t *testing.T) {
	f := setup(t, true)
	id := openRun(t, f, `{"window_from":"2026-03-01","window_to":"2026-03-03"}`)
	rec := doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+id, garminToken,
		`{"status":"partial","summary":{"days_total":3,"days_ok":2,"days_failed":1}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var run garminsyncstatus.SyncRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	assert.Equal(t, garminsyncstatus.StatusPartial, run.Status)
	require.NotNil(t, run.Summary)
	assert.JSONEq(t, `{"days_total":3,"days_ok":2,"days_failed":1}`, string(run.Summary))
}

func TestStatus_ByRunID_ReturnsThatRun(t *testing.T) {
	f := setup(t, true)
	// A backfill run, then a NEWER daily-sync run that would otherwise be "latest".
	backfill := openRun(t, f, `{"window_from":"2026-03-01","window_to":"2026-03-03"}`)
	require.Equal(t, http.StatusOK, doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+backfill, garminToken,
		`{"status":"partial","summary":{"days_failed":1}}`).Code)
	newer := openRun(t, f, "")
	require.Equal(t, http.StatusOK, doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+newer, garminToken, `{"status":"success"}`).Code)

	// Polling by the backfill's run_id returns IT, not the newer run.
	rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status?run_id="+backfill, agentToken, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var st garminsyncstatus.SyncStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	require.NotNil(t, st.Latest)
	assert.Equal(t, backfill, st.Latest.ID.String())
	assert.Equal(t, garminsyncstatus.StatusPartial, st.Latest.Status)
	// A partial run is NOT a success: last_successful_at reflects the newer success.
	require.NotNil(t, st.LastSuccessfulAt)
}

func TestStatus_ByUnknownRunID_Is404(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status?run_id=11111111-1111-1111-1111-111111111111", mobileToken, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatus_PartialDoesNotCountAsSuccess(t *testing.T) {
	f := setup(t, true)
	// The only run is a partial ⇒ no success ever ⇒ stale, no last_successful_at.
	id := openRun(t, f, "")
	require.Equal(t, http.StatusOK, doReq(t, f.r, http.MethodPatch, "/garmin/sync-runs/"+id, garminToken,
		`{"status":"partial","summary":{"days_failed":1}}`).Code)
	rec := doReq(t, f.r, http.MethodGet, "/garmin/sync-status", mobileToken, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var st garminsyncstatus.SyncStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	require.NotNil(t, st.Latest)
	assert.Equal(t, garminsyncstatus.StatusPartial, st.Latest.Status)
	assert.Nil(t, st.LastSuccessfulAt, "a partial run is not a success")
	assert.True(t, st.IsStale)
}
