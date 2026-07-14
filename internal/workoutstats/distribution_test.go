package workoutstats_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workouts"
)

func ipz(v int) *int { return &v }

// zn builds a [5]*int zone-seconds array.
func zn(z1, z2, z3, z4, z5 int) [5]*int {
	return [5]*int{ipz(z1), ipz(z2), ipz(z3), ipz(z4), ipz(z5)}
}

var noZones = [5]*int{}

// seedZ inserts a completed/planned workout with HR-zone seconds + optional
// training_focus.
func seedZ(t *testing.T, repo *workouts.Repo, sport, status string, start time.Time, z [5]*int, focus *string) {
	t.Helper()
	var tf *workouts.TrainingFocus
	if focus != nil {
		v := workouts.TrainingFocus(*focus)
		tf = &v
	}
	_, err := repo.Upsert(context.Background(), &workouts.Workout{
		Source:        workouts.SourceManual,
		Sport:         workouts.Sport(sport),
		Status:        workouts.Status(status),
		StartedAt:     start,
		EndedAt:       start.Add(time.Hour),
		SecsInZone1:   z[0],
		SecsInZone2:   z[1],
		SecsInZone3:   z[2],
		SecsInZone4:   z[3],
		SecsInZone5:   z[4],
		TrainingFocus: tf,
	})
	require.NoError(t, err)
}

func decodeDist(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m
}

func utc(s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tm
}

// 2.1: populated window shape — ordered 5-entry zones, by_sport split, weekly
// Monday-start buckets, planned excluded.
func TestDistribution_PopulatedWindow(t *testing.T) {
	f := setup(t)
	// Two completed workouts (bike + run) in the same week + one planned (excluded).
	seedZ(t, f.repo, "bike", "completed", utc("2026-06-02T09:00:00Z"), zn(3000, 2000, 500, 300, 200), nil)
	seedZ(t, f.repo, "run", "completed", utc("2026-06-04T09:00:00Z"), zn(1000, 900, 400, 100, 100), nil)
	seedZ(t, f.repo, "run", "planned", utc("2026-06-03T09:00:00Z"), zn(9999, 9999, 9999, 9999, 9999), nil)

	rec := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m := decodeDist(t, rec)

	total := m["total"].(map[string]any)
	assert.EqualValues(t, 2, total["workouts_counted"], "planned excluded")
	zones := total["zones"].([]any)
	require.Len(t, zones, 5, "always 5 entries")
	// Ordered zone 1→5 and shares sum to ~100.
	sum := 0.0
	for i, z := range zones {
		zm := z.(map[string]any)
		assert.EqualValues(t, i+1, zm["zone"])
		sum += zm["share_pct"].(float64)
	}
	assert.InDelta(t, 100.0, sum, 0.2)

	bySport := m["by_sport"].(map[string]any)
	assert.Contains(t, bySport, "bike")
	assert.Contains(t, bySport, "run")

	weekly := m["weekly"].([]any)
	require.Len(t, weekly, 1, "both workouts fall in one Monday-start week")
	assert.Equal(t, mondayISO("2026-06-02"), weekly[0].(map[string]any)["week_start"])
}

// 2.2: missing zone data is counted, not hidden; a missing-only week still emits
// a bucket; counted + missing == completed count.
func TestDistribution_MissingZoneHonesty(t *testing.T) {
	f := setup(t)
	seedZ(t, f.repo, "bike", "completed", utc("2026-06-02T09:00:00Z"), zn(1000, 800, 200, 0, 0), nil)
	seedZ(t, f.repo, "strength", "completed", utc("2026-06-15T09:00:00Z"), noZones, nil) // all-null, different week

	rec := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m := decodeDist(t, rec)

	assert.EqualValues(t, 1, m["missing_zone_data_count"])
	assert.EqualValues(t, 1, m["total"].(map[string]any)["workouts_counted"])
	// counted (1) + missing (1) == the 2 completed workouts.

	weekly := m["weekly"].([]any)
	require.Len(t, weekly, 2, "the zone-less week still emits a bucket")
	// The strength-only week carries a missing counter and no counted workouts.
	var missingWeek map[string]any
	for _, w := range weekly {
		wm := w.(map[string]any)
		if wm["workouts_counted"].(float64) == 0 {
			missingWeek = wm
		}
	}
	require.NotNil(t, missingWeek)
	assert.EqualValues(t, 1, missingWeek["missing_zone_data_count"])
}

// 2.3: classification + bands through the API; zone-less window → null label,
// omitted shares, still 200.
func TestDistribution_ClassificationAndEmpty(t *testing.T) {
	f := setup(t)
	// A clearly polarized block: heavy Z1/Z2, tiny Z3, some Z4/Z5.
	seedZ(t, f.repo, "bike", "completed", utc("2026-06-02T09:00:00Z"), zn(5000, 3000, 500, 1000, 500), nil)

	rec := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m := decodeDist(t, rec)
	total := m["total"].(map[string]any)
	assert.Equal(t, "polarized", total["classification"])
	bands := total["bands"].(map[string]any)
	assert.Contains(t, bands, "low_pct")
	// by_sport/weekly entries carry no classification.
	bike := m["by_sport"].(map[string]any)["bike"].(map[string]any)
	assert.NotContains(t, bike, "classification")
	assert.NotContains(t, bike, "bands")

	// Zone-less window: a completed workout with all-null zones only.
	f2 := setup(t)
	seedZ(t, f2.repo, "strength", "completed", utc("2026-06-02T09:00:00Z"), noZones, nil)
	rec2 := get(t, f2.r, "/workouts/intensity-distribution?from=2026-06-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, rec2.Code)
	total2 := decodeDist(t, rec2)["total"].(map[string]any)
	assert.Nil(t, total2["classification"], "no zone time → classification null")
	zones2 := total2["zones"].([]any)
	assert.NotContains(t, zones2[0].(map[string]any), "share_pct", "0% share omitted")
	assert.Contains(t, total2, "bands")
}

// 2.4: training-focus axis; annotations don't alter the measured classification.
func TestDistribution_TrainingFocusAxis(t *testing.T) {
	f := setup(t)
	ga1 := "basic_endurance_1"
	seedZ(t, f.repo, "bike", "completed", utc("2026-06-02T09:00:00Z"), zn(5000, 3000, 500, 1000, 500), &ga1)
	seedZ(t, f.repo, "run", "completed", utc("2026-06-04T09:00:00Z"), zn(1000, 800, 100, 50, 50), nil) // unannotated

	rec := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m := decodeDist(t, rec)
	focus := m["by_training_focus"].(map[string]any)
	assert.EqualValues(t, 1, focus["basic_endurance_1"])
	assert.EqualValues(t, 1, m["unclassified_focus_count"])
}

// 2.5: timezone bucketing across midnight, validation codes, unit isolation.
func TestDistribution_TZValidationAndIsolation(t *testing.T) {
	f := setup(t)
	// 22:30Z Jun 7 = 00:30 Jun 8 in Berlin → next local day (and next week if Jun 8 is Monday).
	seedZ(t, f.repo, "bike", "completed", utc("2026-06-07T22:30:00Z"), zn(2000, 1000, 500, 0, 0), nil)
	rec := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-08&to=2026-06-08&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.EqualValues(t, 1, decodeDist(t, rec)["total"].(map[string]any)["workouts_counted"],
		"attributed to Jun 8 local")

	// Validation codes.
	for q, code := range map[string]string{
		"to=2026-06-30":                          "range_required",
		"from=x&to=2026-06-30":                    "date_invalid",
		"from=2026-06-30&to=2026-06-01":           "range_invalid",
		"from=2024-01-01&to=2026-06-30":           "range_too_large",
		"from=2026-06-01&to=2026-06-02&tz=Nowhere": "tz_invalid",
	} {
		r := get(t, f.r, "/workouts/intensity-distribution?"+q)
		require.Equalf(t, http.StatusBadRequest, r.Code, q)
		assert.Containsf(t, r.Body.String(), code, q)
	}

	// Unit isolation: no nutrition/hydration keys in the distribution body.
	ok := get(t, f.r, "/workouts/intensity-distribution?from=2026-06-08&to=2026-06-08&tz=UTC")
	body := ok.Body.String()
	for _, k := range []string{"kcal", "protein", "hydration", "total_ml"} {
		assert.NotContains(t, body, k)
	}
}

// mondayISO returns the ISO date of the Monday of the given ISO date's week.
func mondayISO(iso string) string {
	d, _ := time.Parse("2006-01-02", iso)
	off := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -off).Format("2006-01-02")
}
