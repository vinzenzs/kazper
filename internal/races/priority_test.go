package races_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/races"
)

func decodeRace(t *testing.T, body []byte) races.Race {
	t.Helper()
	var r races.Race
	require.NoError(t, json.Unmarshal(body, &r))
	return r
}

const priBody = `{"name":"Prio","race_date":"2026-08-01","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`

// --- create ---------------------------------------------------------------

func TestPriority_CreatePersistsAndEchoes(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/races", `{"name":"A race","race_date":"2026-08-01","priority":"A","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	race := decodeRace(t, rec.Body.Bytes())
	require.NotNil(t, race.Priority)
	assert.Equal(t, races.PriorityA, *race.Priority)

	// Persisted: a subsequent GET echoes it.
	got := do(t, r, http.MethodGet, "/races/"+race.ID.String(), "")
	require.Equal(t, http.StatusOK, got.Code)
	assert.Equal(t, races.PriorityA, *decodeRace(t, got.Body.Bytes()).Priority)
}

func TestPriority_CreateRejectsInvalid(t *testing.T) {
	r := setup(t)
	for _, bad := range []string{"D", "a", "high"} {
		rec := do(t, r, http.MethodPost, "/races", `{"name":"X","race_date":"2026-08-01","priority":"`+bad+`","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, "priority=%s", bad)
		assert.Contains(t, rec.Body.String(), "race_priority_invalid")
	}
	// Nothing persisted.
	list := do(t, r, http.MethodGet, "/races", "")
	assert.NotContains(t, list.Body.String(), `"name":"X"`)
}

func TestPriority_CreateWithoutPriorityOmitsKey(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/races", priBody)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"priority"`)
	race := decodeRace(t, rec.Body.Bytes())
	assert.Nil(t, race.Priority)
}

// --- PATCH tri-state ------------------------------------------------------

func TestPriority_PatchTriState(t *testing.T) {
	r := setup(t)
	race := decodeRace(t, do(t, r, http.MethodPost, "/races", priBody).Body.Bytes())
	id := race.ID.String()

	// Set to B (name preserved).
	rec := do(t, r, http.MethodPatch, "/races/"+id, `{"priority":"B"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	after := decodeRace(t, rec.Body.Bytes())
	require.NotNil(t, after.Priority)
	assert.Equal(t, races.PriorityB, *after.Priority)
	assert.Equal(t, "Prio", after.Name)

	// Omitting priority leaves it unchanged.
	rec = do(t, r, http.MethodPatch, "/races/"+id, `{"notes":"noted"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, races.PriorityB, *decodeRace(t, rec.Body.Bytes()).Priority)

	// Empty string clears it → subsequent GET omits the key.
	rec = do(t, r, http.MethodPatch, "/races/"+id, `{"priority":""}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, decodeRace(t, rec.Body.Bytes()).Priority)
	got := do(t, r, http.MethodGet, "/races/"+id, "")
	assert.NotContains(t, got.Body.String(), `"priority"`)
}

func TestPriority_PatchRejectsInvalidWithoutChange(t *testing.T) {
	r := setup(t)
	race := decodeRace(t, do(t, r, http.MethodPost, "/races", `{"name":"P","race_date":"2026-08-01","priority":"A","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`).Body.Bytes())
	id := race.ID.String()

	rec := do(t, r, http.MethodPatch, "/races/"+id, `{"priority":"Z"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "race_priority_invalid")

	// Row unchanged — still A.
	got := decodeRace(t, do(t, r, http.MethodGet, "/races/"+id, "").Body.Bytes())
	assert.Equal(t, races.PriorityA, *got.Priority)
}

// --- list filter ----------------------------------------------------------

func TestPriority_ListFilter(t *testing.T) {
	r := setup(t)
	do(t, r, http.MethodPost, "/races", `{"name":"AA","race_date":"2026-08-01","priority":"A","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`)
	do(t, r, http.MethodPost, "/races", `{"name":"CC","race_date":"2026-08-02","priority":"C","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`)
	do(t, r, http.MethodPost, "/races", `{"name":"UU","race_date":"2026-08-03","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`)

	// ?priority=A → only the A race.
	var wrap struct {
		Races []races.Race `json:"races"`
	}
	rec := do(t, r, http.MethodGet, "/races?priority=A", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wrap))
	require.Len(t, wrap.Races, 1)
	assert.Equal(t, "AA", wrap.Races[0].Name)

	// No filter → all three.
	rec = do(t, r, http.MethodGet, "/races", "")
	wrap.Races = nil
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wrap))
	assert.Len(t, wrap.Races, 3)

	// Invalid filter → 400.
	rec = do(t, r, http.MethodGet, "/races?priority=X", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "race_priority_invalid")
}

// --- advisory stance ------------------------------------------------------

// The races write path has no macrocycle-anchor coupling: PATCHing priority
// always succeeds regardless of whether a macrocycle anchors the race. (The
// races package doesn't wire the macrocycle repo — the absence of any coupling
// check in this path is the guarantee under test.)
func TestPriority_AdvisoryNoCouplingError(t *testing.T) {
	r := setup(t)
	race := decodeRace(t, do(t, r, http.MethodPost, "/races", `{"name":"Anchor","race_date":"2026-08-01","priority":"A","legs":[{"ordinal":1,"discipline":"run","expected_duration_min":50}]}`).Body.Bytes())
	rec := do(t, r, http.MethodPatch, "/races/"+race.ID.String(), `{"priority":"C"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, races.PriorityC, *decodeRace(t, rec.Body.Bytes()).Priority)
}
