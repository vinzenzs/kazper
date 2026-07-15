package athleteconfig_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

const (
	mobileToken = "mobile-token-abcdefghij"
	agentToken  = "agent-token-abcdefghijk"
	garminToken = "garmin-token-abcdefghij"
)

// setupAuth wires the handlers behind the real auth + idempotency middleware, so
// the identity guards on the detection/config/sources PUTs are exercised end to
// end. Returns the engine and the athlete-config service for direct assertions.
func setupAuth(t *testing.T) (*gin.Engine, *athleteconfig.Service) {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := athleteconfig.NewService(athleteconfig.NewRepo(pool), pool)
	r := gin.New()
	api := r.Group("/")
	api.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken, GarminToken: garminToken}))
	api.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	athleteconfig.NewHandlers(svc).Register(api)
	return r, svc
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// --- detection singleton ---

func TestDetection_GarminWritesWithoutTouchingConfig(t *testing.T) {
	r, _ := setupAuth(t)

	// Seed a deliberate config as the agent, plus a source policy, so we can prove
	// the detection write leaves both untouched.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":278,"max_hr":196}`, bearer(agentToken)).Code)
	configBefore := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken)).Body.String()

	rec := do(t, r, http.MethodPut, "/athlete-config/garmin-detected",
		`{"ftp_watts":285,"max_hr":199}`, bearer(garminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Detection reads back.
	got := do(t, r, http.MethodGet, "/athlete-config/garmin-detected", "", bearer(agentToken))
	var body struct {
		Detected *athleteconfig.GarminDetectedThresholds `json:"garmin_detected"`
	}
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	require.NotNil(t, body.Detected)
	assert.Equal(t, 285, *body.Detected.FtpWatts)
	assert.False(t, body.Detected.DetectedAt.IsZero(), "detected_at stamped")

	// Config is byte-identical to before the detection write.
	configAfter := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken)).Body.String()
	assert.JSONEq(t, configBefore, configAfter, "detection write must not touch athlete_config")

	// And no history snapshot was written by the detection.
	hist := do(t, r, http.MethodGet, "/athlete-config/history", "", bearer(agentToken)).Body.String()
	assert.NotContains(t, hist, "285", "detection value must not appear in threshold history")
}

func TestDetection_NonGarminForbidden(t *testing.T) {
	r, _ := setupAuth(t)
	for _, tok := range []string{mobileToken, agentToken} {
		rec := do(t, r, http.MethodPut, "/athlete-config/garmin-detected",
			`{"ftp_watts":285}`, bearer(tok))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	}
	// Nothing stored.
	got := do(t, r, http.MethodGet, "/athlete-config/garmin-detected", "", bearer(agentToken))
	assert.JSONEq(t, `{"garmin_detected":null}`, got.Body.String())
}

func TestDetection_NullBeforeAnySync(t *testing.T) {
	r, _ := setupAuth(t)
	rec := do(t, r, http.MethodGet, "/athlete-config/garmin-detected", "", bearer(agentToken))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"garmin_detected":null}`, rec.Body.String())
}

// --- config PUT garmin guard ---

func TestConfigPut_GarminForbidden(t *testing.T) {
	r, _ := setupAuth(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":285}`, bearer(garminToken))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	// Config unchanged (still null), no history.
	got := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken))
	assert.JSONEq(t, `{"athlete_config":null,"sources":[]}`, got.Body.String())
}

func TestConfigPut_AgentUnaffected(t *testing.T) {
	r, _ := setupAuth(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":278}`, bearer(agentToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// --- sources policy ---

func TestSources_FlipEchoedAndPhysiologyUntouched(t *testing.T) {
	r, _ := setupAuth(t)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":278}`, bearer(agentToken)).Code)
	histBefore := do(t, r, http.MethodGet, "/athlete-config/history", "", bearer(agentToken)).Body.String()

	rec := do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts"]}`, bearer(agentToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"sources":["ftp_watts"]}`, rec.Body.String())

	// GET /athlete-config echoes the policy and keeps the confirmed value.
	got := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken)).Body.String()
	assert.Contains(t, got, `"sources":["ftp_watts"]`)
	assert.Contains(t, got, `"ftp_watts":278`)

	// A policy change writes no new history snapshot.
	histAfter := do(t, r, http.MethodGet, "/athlete-config/history", "", bearer(agentToken)).Body.String()
	assert.JSONEq(t, histBefore, histAfter, "policy change must not snapshot history")
}

func TestSources_InvalidTokenRejected(t *testing.T) {
	r, _ := setupAuth(t)
	do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts"]}`, bearer(agentToken))
	rec := do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts","vo2max"]}`, bearer(agentToken))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "source_field_invalid")
	// Prior policy unchanged.
	got := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken)).Body.String()
	assert.Contains(t, got, `"sources":["ftp_watts"]`)
}

func TestSources_GarminForbidden(t *testing.T) {
	r, _ := setupAuth(t)
	rec := do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts"]}`, bearer(garminToken))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestSources_PreservedAcrossConfigPut(t *testing.T) {
	r, _ := setupAuth(t)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts","hr_zones"]}`, bearer(agentToken)).Code)
	// A full-replace config PUT must not touch the policy column.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":280,"max_hr":196}`, bearer(agentToken)).Code)
	got := do(t, r, http.MethodGet, "/athlete-config", "", bearer(agentToken)).Body.String()
	assert.Contains(t, got, `"ftp_watts"`)
	assert.Contains(t, got, `"hr_zones"`)
}

// --- effective resolution over HTTP ---

func TestEffective_GarminSourcedAndFallback(t *testing.T) {
	r, _ := setupAuth(t)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":278,"max_hr":196}`, bearer(agentToken)).Code)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config/garmin-detected", `{"ftp_watts":285}`, bearer(garminToken)).Code)
	// Source ftp_watts AND max_hr; ftp has a detection, max_hr does not.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config/sources", `{"sources":["ftp_watts","max_hr"]}`, bearer(agentToken)).Code)

	rec := do(t, r, http.MethodGet, "/athlete-config/effective", "", bearer(agentToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Effective *athleteconfig.EffectiveConfig `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Effective)
	assert.Equal(t, 285, *body.Effective.FtpWatts)
	assert.Equal(t, "garmin", body.Effective.FieldSources["ftp_watts"])
	// max_hr sourced but no detection → manual fallback.
	assert.Equal(t, 196, *body.Effective.MaxHR)
	assert.Equal(t, "manual", body.Effective.FieldSources["max_hr"])
}

func TestEffective_AllManualEqualsConfig(t *testing.T) {
	r, _ := setupAuth(t)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":278,"threshold_hr":168,"max_hr":196}`, bearer(agentToken)).Code)
	// A detection exists but no field is sourced → effective equals the config.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPut, "/athlete-config/garmin-detected", `{"ftp_watts":285}`, bearer(garminToken)).Code)

	rec := do(t, r, http.MethodGet, "/athlete-config/effective", "", bearer(agentToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Effective *athleteconfig.EffectiveConfig `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Effective)
	assert.Equal(t, 278, *body.Effective.FtpWatts, "empty policy → confirmed value, not detection")
	assert.Equal(t, 168, *body.Effective.ThresholdHR)
	assert.Equal(t, 196, *body.Effective.MaxHR)
	for field, src := range body.Effective.FieldSources {
		assert.Equal(t, "manual", src, "field %s", field)
	}
}
