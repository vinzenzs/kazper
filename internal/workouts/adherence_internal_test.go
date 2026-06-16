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

	s := computeAdherence(rows, adhNow)

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
}

func TestComputeAdherence_NullRateWhenNothingDue(t *testing.T) {
	rows := []AdherenceRow{
		{Status: StatusPlanned, Sport: SportBike, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(2 * time.Hour), EndedAt: adhNow.Add(3 * time.Hour)},
		{Status: StatusPlanned, Sport: SportRun, PlanSlotID: slotPtr(), StartedAt: adhNow.Add(48 * time.Hour), EndedAt: adhNow.Add(49 * time.Hour)},
	}
	s := computeAdherence(rows, adhNow)
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
	s := computeAdherence(rows, adhNow)
	require.NotNil(t, s.CompletedDurationMin, "duration is present even when tss is absent")
	assert.Nil(t, s.PlannedTSS, "sum over zero present tss → null")
	assert.Nil(t, s.CompletedTSS)
}
