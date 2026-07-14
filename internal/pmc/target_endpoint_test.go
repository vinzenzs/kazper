package pmc_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/pmc"
)

func getTraj(t *testing.T, f *fixture, query string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/performance/pmc/target-trajectory"
	if query != "" {
		path += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func decodeTraj(t *testing.T, rec *httptest.ResponseRecorder) pmc.TargetTrajectory {
	t.Helper()
	var tr pmc.TargetTrajectory
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tr))
	return tr
}

func date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// A macrocycle fully in the past → the whole span has measured CTL, so the
// trajectory is deterministic regardless of wall-clock "today".
func pastMacro(target *float64) *pmc.Macro {
	return &pmc.Macro{
		ID:        "11111111-1111-1111-1111-111111111111",
		Name:      "Winter base",
		StartDate: date("2026-01-01"),
		EndDate:   date("2026-01-31"),
		Phases: []pmc.MacroPhase{
			{StartDate: date("2026-01-01"), EndDate: date("2026-01-31"), TargetWeeklyTSS: target},
		},
	}
}

func TestTargetTrajectory_ActiveHappyPath(t *testing.T) {
	f := setup(t)
	// Seed a ramp of completed workouts across the macrocycle so actual CTL builds.
	for _, d := range []string{"2026-01-03", "2026-01-06", "2026-01-09", "2026-01-13", "2026-01-17", "2026-01-22", "2026-01-27"} {
		seedW(t, f.repo, "bike", "completed", date(d).Add(9*time.Hour), fp(90))
	}
	f.resolver.macro = pastMacro(fp(420)) // daily target 60

	rec := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	tr := decodeTraj(t, rec)

	assert.Equal(t, "Winter base", tr.Macrocycle.Name)
	assert.Equal(t, "2026-01-01", tr.Macrocycle.StartDate)
	require.NotNil(t, tr.Trajectory)
	require.Len(t, tr.Trajectory, 31) // Jan 1..31 inclusive
	// Full span in the past → every day has actual_ctl + delta.
	for _, d := range tr.Trajectory {
		require.NotNilf(t, d.ActualCTL, "day %s should carry actual (macrocycle is in the past)", d.Date)
		require.NotNil(t, d.Delta)
		assert.True(t, d.TargetDeclared)
	}
	// Day-0 target equals the seed (delta ≈ 0 at start).
	assert.InDelta(t, tr.SeedCTL, tr.Trajectory[0].TargetCTL, 0.05)
	require.NotNil(t, tr.Summary)
}

func TestTargetTrajectory_ExplicitID(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", date("2026-01-10").Add(9*time.Hour), fp(100))
	f.resolver.macro = pastMacro(fp(350))

	rec := getTraj(t, f, "macrocycle_id=11111111-1111-1111-1111-111111111111&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	tr := decodeTraj(t, rec)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", tr.Macrocycle.ID)
	require.NotNil(t, tr.Trajectory)
}

func TestTargetTrajectory_GapFlaggedSpans(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", date("2026-01-05").Add(9*time.Hour), fp(80))
	macro := &pmc.Macro{
		ID:        "22222222-2222-2222-2222-222222222222",
		Name:      "Base + gap",
		StartDate: date("2026-01-01"),
		EndDate:   date("2026-01-20"),
		Phases: []pmc.MacroPhase{
			{StartDate: date("2026-01-01"), EndDate: date("2026-01-10"), TargetWeeklyTSS: fp(420)},
			// 2026-01-11 .. 2026-01-20 is an undeclared gap.
		},
	}
	f.resolver.macro = macro

	rec := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	tr := decodeTraj(t, rec)

	byDate := map[string]pmc.TargetDay{}
	for _, d := range tr.Trajectory {
		byDate[d.Date] = d
	}
	assert.True(t, byDate["2026-01-08"].TargetDeclared, "in-phase day is declared")
	assert.False(t, byDate["2026-01-18"].TargetDeclared, "gap day is undeclared")
}

func TestTargetTrajectory_TargetsMissing(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", date("2026-01-05").Add(9*time.Hour), fp(80))
	f.resolver.macro = pastMacro(nil) // phase declares no target

	rec := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	tr := decodeTraj(t, rec)
	assert.Nil(t, tr.Trajectory, "no declared target → trajectory null")
	assert.Equal(t, "targets_missing", tr.Reason)
	assert.Nil(t, tr.Summary)
	// The macrocycle identity + seed still resolve.
	assert.Equal(t, "Winter base", tr.Macrocycle.Name)
}

func TestTargetTrajectory_NoActiveOrUnknownIs404(t *testing.T) {
	f := setup(t)
	// Default fake resolver returns ErrMacrocycleNotFound.
	rec := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusNotFound, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "macrocycle_not_found", body["error"])
}

func TestTargetTrajectory_FutureDaysTargetOnly(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", date("2026-01-10").Add(9*time.Hour), fp(90))
	macro := &pmc.Macro{
		ID:        "33333333-3333-3333-3333-333333333333",
		Name:      "Long build",
		StartDate: date("2026-01-01"),
		EndDate:   date("2099-12-31"), // far future
		Phases: []pmc.MacroPhase{
			{StartDate: date("2026-01-01"), EndDate: date("2099-12-31"), TargetWeeklyTSS: fp(420)},
		},
	}
	f.resolver.macro = macro

	rec := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	tr := decodeTraj(t, rec)
	require.NotEmpty(t, tr.Trajectory)
	// The last day is decades in the future → target only, no actual.
	last := tr.Trajectory[len(tr.Trajectory)-1]
	assert.Equal(t, "2099-12-31", last.Date)
	assert.Nil(t, last.ActualCTL, "future days carry the target only")
	assert.Nil(t, last.Delta)
	// An early past day does carry actual.
	assert.NotNil(t, tr.Trajectory[0].ActualCTL)
}

func TestTargetTrajectory_BadTZ(t *testing.T) {
	f := setup(t)
	f.resolver.macro = pastMacro(fp(420))
	rec := getTraj(t, f, "tz=Nowhere/Land")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "tz_invalid", body["error"])
}

func TestTargetTrajectory_NothingPersisted(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", date("2026-01-10").Add(9*time.Hour), fp(90))
	f.resolver.macro = pastMacro(fp(420))

	first := getTraj(t, f, "tz=UTC")
	second := getTraj(t, f, "tz=UTC")
	require.Equal(t, http.StatusOK, first.Code)
	assert.JSONEq(t, first.Body.String(), second.Body.String())
}
