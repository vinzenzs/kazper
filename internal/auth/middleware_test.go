package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	testMobileToken = "mobile-token-aaaaaaaaaaaaaa"
	testAgentToken  = "agent-token-bbbbbbbbbbbbbbbb"
	testGarminToken = "garmin-token-cccccccccccccc"
	testWebUser     = "coach"
	testWebPassword = "dashboard-pass-dddddddddddd"
)

func basicHeader(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

func newTestRouter(t *testing.T) (*gin.Engine, *clientCapture) {
	t.Helper()
	return newTestRouterFor(t, Config{MobileToken: testMobileToken, AgentToken: testAgentToken})
}

func newTestRouterFor(t *testing.T, cfg Config) (*gin.Engine, *clientCapture) {
	t.Helper()
	cap := &clientCapture{}
	r := gin.New()
	r.Use(Middleware(cfg))
	r.GET("/protected", func(c *gin.Context) {
		cap.id = ClientFromContext(c)
		c.Status(http.StatusOK)
	})
	return r, cap
}

type clientCapture struct{ id ClientID }

func TestMiddleware_MobileTokenSetsContext(t *testing.T) {
	r, cap := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+testMobileToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ClientMobile, cap.id)
}

func TestMiddleware_AgentTokenSetsContext(t *testing.T) {
	r, cap := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+testAgentToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ClientAgent, cap.id)
}

func TestMiddleware_MissingHeader(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_required"}`, rec.Body.String())
}

func TestMiddleware_WrongScheme(t *testing.T) {
	// An unrecognized scheme (neither Bearer nor Basic) is auth_required.
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token "+testMobileToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_required"}`, rec.Body.String())
}

func newTestRouterWithWeb(t *testing.T) (*gin.Engine, *clientCapture) {
	t.Helper()
	return newTestRouterFor(t, Config{
		MobileToken: testMobileToken,
		AgentToken:  testAgentToken,
		WebUser:     testWebUser,
		WebPassword: testWebPassword,
	})
}

func TestMiddleware_WebBasicSetsContext(t *testing.T) {
	r, cap := newTestRouterWithWeb(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", basicHeader(testWebUser, testWebPassword))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ClientWeb, cap.id)
}

func TestMiddleware_WebBasicWrongPassword(t *testing.T) {
	r, _ := newTestRouterWithWeb(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", basicHeader(testWebUser, "nope"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_invalid"}`, rec.Body.String())
}

func TestMiddleware_WebBasicMalformedPayload(t *testing.T) {
	r, _ := newTestRouterWithWeb(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic not-base64!!")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_invalid"}`, rec.Body.String())
}

func TestMiddleware_WebIdentityNotRecognizedWhenUnset(t *testing.T) {
	// WEB_USER/WEB_PASSWORD omitted → any Basic credential is auth_invalid.
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", basicHeader(testWebUser, testWebPassword))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_invalid"}`, rec.Body.String())
}

func TestMiddleware_BearerUnaffectedByWeb(t *testing.T) {
	// With web configured, the Bearer path still resolves mobile/agent normally.
	r, cap := newTestRouterWithWeb(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+testMobileToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ClientMobile, cap.id)
}

func TestConfig_Validate_WebIncomplete(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, WebUser: testWebUser}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebIncomplete))

	err = Config{MobileToken: testMobileToken, AgentToken: testAgentToken, WebPassword: testWebPassword}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebIncomplete))
}

func TestConfig_Validate_WebSetOk(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, WebUser: testWebUser, WebPassword: testWebPassword}.Validate()
	assert.NoError(t, err)
}

func TestMiddleware_UnknownToken(t *testing.T) {
	r, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer wrong-token-xxxxxxxxxxxxxxx")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_invalid"}`, rec.Body.String())
}

func TestMiddleware_GarminTokenSetsContext(t *testing.T) {
	r, cap := newTestRouterFor(t, Config{
		MobileToken: testMobileToken,
		AgentToken:  testAgentToken,
		GarminToken: testGarminToken,
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+testGarminToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ClientGarmin, cap.id)
}

func TestMiddleware_GarminTokenNotRecognizedWhenUnset(t *testing.T) {
	// GarminToken omitted → the value is not a configured token, so it is 401.
	r, _ := newTestRouterFor(t, Config{MobileToken: testMobileToken, AgentToken: testAgentToken})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+testGarminToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"auth_invalid"}`, rec.Body.String())
}

func TestConfig_Validate_GarminTooShort(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, GarminToken: "short"}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenTooShort))
}

func TestConfig_Validate_GarminEqualMobile(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, GarminToken: testMobileToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrGarminEqual))
}

func TestConfig_Validate_GarminEqualAgent(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, GarminToken: testAgentToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrGarminEqual))
}

func TestConfig_Validate_GarminUnsetOk(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken}.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_GarminSetOk(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken, GarminToken: testGarminToken}.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingMobileToken(t *testing.T) {
	err := Config{AgentToken: testAgentToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenMissing))
}

func TestConfig_Validate_MissingAgentToken(t *testing.T) {
	err := Config{MobileToken: testMobileToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenMissing))
}

func TestConfig_Validate_ShortToken(t *testing.T) {
	err := Config{MobileToken: "short", AgentToken: testAgentToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenTooShort))
}

func TestConfig_Validate_EqualTokens(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testMobileToken}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokensEqual))
}

func TestConfig_Validate_Ok(t *testing.T) {
	err := Config{MobileToken: testMobileToken, AgentToken: testAgentToken}.Validate()
	assert.NoError(t, err)
}
