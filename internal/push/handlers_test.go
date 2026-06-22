package push_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/push"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
	garminToken = "garmin-token-cccccccccccccc"
)

type fixture struct {
	r    *gin.Engine
	pool *pgxpool.Pool
}

// setup builds a router with the auth middleware and the push handlers. The
// sender is nil (push disabled) — registration must work regardless.
func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	authCfg := auth.Config{MobileToken: mobileToken, AgentToken: agentToken, GarminToken: garminToken}
	svc := push.NewService(push.NewRepo(pool), nil, nil)

	r := gin.New()
	r.Use(auth.Middleware(authCfg))
	rg := r.Group("/")
	push.NewHandlers(svc).Register(rg)
	return &fixture{r: r, pool: pool}
}

func doReq(t *testing.T, r *gin.Engine, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func countTokens(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM push_tokens`).Scan(&n))
	return n
}

func TestRegister_StoresToken(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/push/tokens", mobileToken, []byte(`{"token":"abc123"}`))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "abc123")
	assert.Equal(t, 1, countTokens(t, f.pool))
}

func TestRegister_IsIdempotentByToken(t *testing.T) {
	f := setup(t)
	require.Equal(t, http.StatusCreated,
		doReq(t, f.r, http.MethodPost, "/push/tokens", mobileToken, []byte(`{"token":"abc123"}`)).Code)
	require.Equal(t, http.StatusCreated,
		doReq(t, f.r, http.MethodPost, "/push/tokens", mobileToken, []byte(`{"token":"abc123"}`)).Code)
	assert.Equal(t, 1, countTokens(t, f.pool), "re-registering the same token must not duplicate")
}

func TestRegister_MissingToken_400(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/push/tokens", mobileToken, []byte(`{}`))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"token_required"}`, rec.Body.String())
}

func TestRemove_DeletesToken(t *testing.T) {
	f := setup(t)
	require.Equal(t, http.StatusCreated,
		doReq(t, f.r, http.MethodPost, "/push/tokens", mobileToken, []byte(`{"token":"abc123"}`)).Code)

	rec := doReq(t, f.r, http.MethodDelete, "/push/tokens", mobileToken, []byte(`{"token":"abc123"}`))
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 0, countTokens(t, f.pool))
}

func TestRemove_UnknownToken_NoContent(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodDelete, "/push/tokens", mobileToken, []byte(`{"token":"nope"}`))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestNonMobileIdentities_Forbidden(t *testing.T) {
	f := setup(t)
	for _, tok := range []string{agentToken, garminToken} {
		rec := doReq(t, f.r, http.MethodPost, "/push/tokens", tok, []byte(`{"token":"abc123"}`))
		assert.Equal(t, http.StatusForbidden, rec.Code, "POST with non-mobile token")
		assert.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())

		rec = doReq(t, f.r, http.MethodDelete, "/push/tokens", tok, []byte(`{"token":"abc123"}`))
		assert.Equal(t, http.StatusForbidden, rec.Code, "DELETE with non-mobile token")
	}
	assert.Equal(t, 0, countTokens(t, f.pool), "forbidden requests must not write")
}
