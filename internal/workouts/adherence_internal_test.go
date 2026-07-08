package workouts

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func slotPtr() *uuid.UUID { u := uuid.New(); return &u }
func f64(v float64) *float64 { return &v }

// now anchor for classification: a planned session before it is "missed", at/after is "upcoming".
var adhNow = time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

func TestComputeAdherence_FullWindow(t *testing.T) {
	rows := []AdherenceRow{
		// completed (on-plan), bike, 60 min, tss 50
		{Status: StatusCompleted, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-2 * time.Hour), EndedAt: adhNow.Add(-1 * time.Hour), TSS: f64(50)},
		// completed (on-plan), run, 60 min, tss 70
		{Status: StatusCompleted, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-26 * time.Hour), EndedAt: adhNow.Add(-25 * time.Hour), TSS: f64(70)},
		// missed (planned, overdue), swim, 60 min, tss 40
		{Status: StatusPlanned, Sport: SportSwim, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-1 * time.Hour), EndedAt: adhNow, TSS: f64(40)},
		// upcoming (planned, future), bike
		{Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(2 * time.Hour), EndedAt: adhNow.Add(3 * time.Hour)},
		// unplanned (completed, no slot), run, tss 99
		{Status: StatusCompleted, Sport: SportRun, PlanSlotID: nil, StartedAt: adhNow.Add(-3 * time.Hour), EndedAt: adhNow.Add(-2 * time.Hour), TSS: f64(99)},
	}

	s := computeAdherence(rows, adhNow, time.UTC)

	assert.Equal(t, 2, s.Completed)
	assert.Equal(t, 1, s.Missed)
	assert.Equal(t, 1, s.Upcoming)
	assert.Equal(t, 1, s.Unplanned)

	require.NotNil(t, s.AdherenceRate)
	assert.InDelta(t, 0.7, *s.AdherenceRate, 0.0001, "2/(2+1) = 0.666… → 0.7; upcoming + unplanned excluded")

	require.NotNil(t, s.PlannedDurationMin)
	assert.InDelta(t, 180.0, *s.PlannedDurationMin, 0.001, "completed(60+60) + missed(60)")
	require.NotNil(t, s.CompletedDurationMin)
	assert.InDelta(t, 120.0, *s.CompletedDurationMin, 0.001, "completed only (60+60)")

	require.NotNil(t, s.PlannedTSS)
	assert.InDelta(t, 160.0, *s.PlannedTSS, 0.001, "50+70 completed + 40 missed")
	require.NotNil(t, s.CompletedTSS)
	assert.InDelta(t, 120.0, *s.CompletedTSS, 0.001, "50+70")

	assert.Equal(t, BySportCount{Completed: 1}, s.BySport["bike"], "the upcoming bike is not a completed/missed tally")
	assert.Equal(t, BySportCount{Completed: 1}, s.BySport["run"], "unplanned run is excluded from by_sport")
	assert.Equal(t, BySportCount{Missed: 1}, s.BySport["swim"])

	// The one missed (swim) session is named; nothing else is.
	require.Len(t, s.MissedSessions, 1)
	assert.Equal(t, SportSwim, s.MissedSessions[0].Sport)
	assert.False(t, s.MissedSessionsTruncated)

	// The weekly trend reconciles exactly with the top-line counts.
	var wc, wm int
	for _, w := range s.Weekly {
		wc += w.Completed
		wm += w.Missed
	}
	assert.Equal(t, s.Completed, wc, "sum of weekly.completed == top-line completed")
	assert.Equal(t, s.Missed, wm, "sum of weekly.missed == top-line missed")
}

// ----- 2.5 missed list + weekly trend -----

func TestComputeAdherence_MissedSessionsCompactAndOrdered(t *testing.T) {
	rows := []AdherenceRow{
		// missed run, 60m, tss 40 — older (2 days ago)
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-48 * time.Hour), EndedAt: adhNow.Add(-47 * time.Hour), TSS: f64(40)},
		// missed bike, 90m, no tss — more recent (still overdue)
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-2 * time.Hour), EndedAt: adhNow.Add(-30 * time.Minute)},
		// a fulfilled, an upcoming, an unplanned — none belong in the list
		{ID: uuid.New(), Status: StatusCompleted, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-3 * time.Hour), EndedAt: adhNow.Add(-2 * time.Hour)},
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportSwim, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(24 * time.Hour), EndedAt: adhNow.Add(25 * time.Hour)},
		{ID: uuid.New(), Status: StatusCompleted, Sport: SportBike, PlanSlotID: nil, StartedAt: adhNow.Add(-5 * time.Hour), EndedAt: adhNow.Add(-4 * time.Hour)},
	}
	s := computeAdherence(rows, adhNow, time.UTC)

	require.Len(t, s.MissedSessions, 2, "only the two missed sessions")
	assert.False(t, s.MissedSessionsTruncated)
	// Oldest first.
	assert.Equal(t, SportRun, s.MissedSessions[0].Sport)
	assert.Equal(t, "2026-06-14", s.MissedSessions[0].Date)
	assert.InDelta(t, 60.0, s.MissedSessions[0].PlannedDurationMin, 0.001)
	require.NotNil(t, s.MissedSessions[0].PlannedTSS)
	assert.InDelta(t, 40.0, *s.MissedSessions[0].PlannedTSS, 0.001)
	assert.Equal(t, SportBike, s.MissedSessions[1].Sport)
	assert.InDelta(t, 90.0, s.MissedSessions[1].PlannedDurationMin, 0.001)
	assert.Nil(t, s.MissedSessions[1].PlannedTSS, "absent tss → null")
}

func TestComputeAdherence_MissedTruncationBoundary(t *testing.T) {
	mk := func(n int) []AdherenceRow {
		rows := make([]AdherenceRow, 0, n)
		for i := range n {
			rows = append(rows, AdherenceRow{
				ID: uuid.New(), Status: StatusPlanned, Sport: SportRun, PlanSlotID: slotPtr(),
				StartedAt: adhNow.Add(-time.Duration(n-i) * time.Hour),
				EndedAt:   adhNow.Add(-time.Duration(n-i)*time.Hour + time.Hour),
			})
		}
		return rows
	}

	exact := computeAdherence(mk(missedSessionsCap), adhNow, time.UTC)
	assert.Len(t, exact.MissedSessions, missedSessionsCap)
	assert.False(t, exact.MissedSessionsTruncated, "exactly the cap → not truncated")

	over := computeAdherence(mk(missedSessionsCap+1), adhNow, time.UTC)
	assert.Len(t, over.MissedSessions, missedSessionsCap, "capped")
	assert.True(t, over.MissedSessionsTruncated, "cap+1 → truncated")
}

func TestComputeAdherence_CalendarWeekBuckets(t *testing.T) {
	// adhNow is Tue 2026-06-16; its Monday is 2026-06-15, prior week 2026-06-08.
	rows := []AdherenceRow{
		// this week: completed (Mon 06-15) + missed (06-16)
		{ID: uuid.New(), Status: StatusCompleted, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-24 * time.Hour), EndedAt: adhNow.Add(-23 * time.Hour), TSS: f64(30)},
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-1 * time.Hour), EndedAt: adhNow.Add(-30 * time.Minute)},
		// prior week: completed (06-08)
		{ID: uuid.New(), Status: StatusCompleted, Sport: SportSwim, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-8 * 24 * time.Hour), EndedAt: adhNow.Add(-8*24*time.Hour + time.Hour), TSS: f64(20)},
	}
	s := computeAdherence(rows, adhNow, time.UTC)

	require.Len(t, s.Weekly, 2)
	assert.Equal(t, "2026-06-08", s.Weekly[0].WeekStart, "sorted by week_start ascending")
	assert.Nil(t, s.Weekly[0].Ordinal, "calendar mode → no ordinal")
	assert.Nil(t, s.Weekly[0].Phase)
	assert.Equal(t, 1, s.Weekly[0].Completed)
	assert.Equal(t, "2026-06-15", s.Weekly[1].WeekStart)
	assert.Equal(t, 1, s.Weekly[1].Completed)
	assert.Equal(t, 1, s.Weekly[1].Missed)
	require.NotNil(t, s.Weekly[1].AdherenceRate)
	assert.InDelta(t, 0.5, *s.Weekly[1].AdherenceRate, 0.0001)
}

func TestComputeAdherence_PlanWeekBuckets(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // Monday of plan week 1
	ord1, ord2 := 1, 2
	base, build := "Base", "Build"
	rows := []AdherenceRow{
		{ID: uuid.New(), Status: StatusCompleted, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-100 * time.Hour), EndedAt: adhNow.Add(-99 * time.Hour), TSS: f64(50),
			PlanWeekOrdinal: &ord1, PlanPhase: &base, PlanStartDate: &start},
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-2 * time.Hour), EndedAt: adhNow.Add(-1 * time.Hour),
			PlanWeekOrdinal: &ord2, PlanPhase: &build, PlanStartDate: &start},
	}
	s := computeAdherence(rows, adhNow, time.UTC)

	require.Len(t, s.Weekly, 2)
	assert.Equal(t, "2026-06-01", s.Weekly[0].WeekStart, "ordinal 1 → plan start_date")
	require.NotNil(t, s.Weekly[0].Ordinal)
	assert.Equal(t, 1, *s.Weekly[0].Ordinal)
	require.NotNil(t, s.Weekly[0].Phase)
	assert.Equal(t, "Base", *s.Weekly[0].Phase)
	assert.Equal(t, "2026-06-08", s.Weekly[1].WeekStart, "ordinal 2 → start_date + 7d")
	require.NotNil(t, s.Weekly[1].Ordinal)
	assert.Equal(t, 2, *s.Weekly[1].Ordinal)
	assert.Equal(t, "Build", *s.Weekly[1].Phase)
	assert.Equal(t, 1, s.Weekly[1].Missed)
}

func TestComputeAdherence_FutureOnlyWeekNullRate(t *testing.T) {
	rows := []AdherenceRow{
		{ID: uuid.New(), Status: StatusPlanned, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(72 * time.Hour), EndedAt: adhNow.Add(73 * time.Hour)},
	}
	s := computeAdherence(rows, adhNow, time.UTC)
	require.Len(t, s.Weekly, 1, "an all-future week is still a real bucket")
	assert.Nil(t, s.Weekly[0].AdherenceRate, "nothing due → null rate")
	assert.Equal(t, 0, s.Weekly[0].Completed)
	assert.Equal(t, 0, s.Weekly[0].Missed)
}

func TestComputeAdherence_NullRateWhenNothingDue(t *testing.T) {
	rows := []AdherenceRow{
		{Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(2 * time.Hour), EndedAt: adhNow.Add(3 * time.Hour)},
		{Status: StatusPlanned, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(48 * time.Hour), EndedAt: adhNow.Add(49 * time.Hour)},
	}
	s := computeAdherence(rows, adhNow, time.UTC)
	assert.Equal(t, 0, s.Completed)
	assert.Equal(t, 0, s.Missed)
	assert.Equal(t, 2, s.Upcoming)
	assert.Nil(t, s.AdherenceRate, "nothing due → null, not 0")
	assert.Nil(t, s.PlannedDurationMin, "no completed+missed → null")
	assert.Nil(t, s.CompletedDurationMin)
}

func TestComputeAdherence_NullTSSWhenNonePresent(t *testing.T) {
	rows := []AdherenceRow{
		{Status: StatusCompleted, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(-2 * time.Hour), EndedAt: adhNow.Add(-1 * time.Hour)},
	}
	s := computeAdherence(rows, adhNow, time.UTC)
	require.NotNil(t, s.CompletedDurationMin, "duration is present even when tss is absent")
	assert.Nil(t, s.PlannedTSS, "sum over zero present tss → null")
	assert.Nil(t, s.CompletedTSS)
}
