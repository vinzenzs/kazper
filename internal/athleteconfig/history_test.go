package athleteconfig_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
)

func getHistory(t *testing.T, rec []byte) []athleteconfig.ThresholdSnapshot {
	t.Helper()
	var wrap struct {
		History []athleteconfig.ThresholdSnapshot `json:"history"`
	}
	require.NoError(t, json.Unmarshal(rec, &wrap))
	return wrap.History
}

// A PUT that changes physiology records today's snapshot; the PUT response is
// the unchanged singleton shape (history is a side effect).
func TestHistory_SnapshotOnChange(t *testing.T) {
	r := setup(t)
	put := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":240,"threshold_pace_sec_per_km":270}`, nil)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.Contains(t, put.Body.String(), `"athlete_config"`)
	assert.NotContains(t, put.Body.String(), `"history"`) // PUT response unchanged

	rec := do(t, r, http.MethodGet, "/athlete-config/history", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := getHistory(t, rec.Body.Bytes())
	require.Len(t, hist, 1)
	require.NotNil(t, hist[0].FtpWatts)
	assert.Equal(t, 240, *hist[0].FtpWatts)
	assert.InDelta(t, 270.0, *hist[0].ThresholdPaceSecPerKm, 0.001) // pace rounded at boundary
}

// A no-op re-PUT (Garmin daily-sync shape) records nothing; the count is stable.
func TestHistory_NoOpDedup(t *testing.T) {
	r := setup(t)
	body := `{"ftp_watts":240}`
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", body, nil).Code)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", body, nil).Code)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", body, nil).Code)

	rec := do(t, r, http.MethodGet, "/athlete-config/history", "", nil)
	hist := getHistory(t, rec.Body.Bytes())
	assert.Len(t, hist, 1, "identical re-PUTs append nothing")
}

// Same-day changes collapse to one row at today's date.
func TestHistory_SameDayCollapse(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":240}`, nil).Code)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":255}`, nil).Code)

	hist := getHistory(t, do(t, r, http.MethodGet, "/athlete-config/history", "", nil).Body.Bytes())
	require.Len(t, hist, 1)
	assert.Equal(t, 255, *hist[0].FtpWatts)
}

// Empty history is 200 {"history":[]}, not a 404.
func TestHistory_EmptyReturns200(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodGet, "/athlete-config/history", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"history":[]}`, rec.Body.String())
}

func TestHistory_RangeErrors(t *testing.T) {
	r := setup(t)
	// Malformed date → 400 date_invalid with field hint.
	rec := do(t, r, http.MethodGet, "/athlete-config/history?from=nope", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "date_invalid", body["error"])
	assert.Equal(t, "from", body["field"])

	// Inverted range → 400 range_invalid.
	rec = do(t, r, http.MethodGet, "/athlete-config/history?from=2026-06-01&to=2026-05-01", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "range_invalid")
}

// Regression: the singleton GET/PUT contract is unaffected by the history hook.
func TestHistory_SingletonContractUnchanged(t *testing.T) {
	r := setup(t)
	// Valid PUT still round-trips through GET.
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":250}`, nil).Code)
	get := do(t, r, http.MethodGet, "/athlete-config", "", nil)
	require.Equal(t, http.StatusOK, get.Code)
	assert.Contains(t, get.Body.String(), `"ftp_watts":250`)

	// Invalid value still rejected.
	bad := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":-5}`, nil)
	require.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Contains(t, bad.Body.String(), "athlete_config_value_invalid")

	// Idempotency-Key on PUT still rejected.
	idem := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":250}`, map[string]string{"Idempotency-Key": "k"})
	require.Equal(t, http.StatusBadRequest, idem.Code)
	assert.Contains(t, idem.Body.String(), "idempotency_unsupported_for_put")
}
