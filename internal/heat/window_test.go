package heat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func vienna(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Vienna")
	require.NoError(t, err)
	return loc
}

// ============================================================================
// the midnight sentinel
// ============================================================================

// The bug this fixes: the date-only scheduling path parses "2006-01-02", which
// yields UTC midnight. For a Vienna athlete that stored instant reads 02:00
// LOCAL — so a local-only midnight test would miss the very rows this exists to
// catch, and the read would keep scoring pre-dawn hours.
func TestIsMidnightSentinel_UTCMidnightCaughtFromANonUTCZone(t *testing.T) {
	loc := vienna(t)
	stored := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) // what the scheduler writes

	assert.Equal(t, 2, stored.In(loc).Hour(), "sanity: 00:00 UTC is 02:00 in Vienna")
	assert.True(t, isMidnightSentinel(stored, loc), "the actual scheduled-row shape must be caught")
}

func TestIsMidnightSentinel_LocalMidnightAlsoCaught(t *testing.T) {
	loc := vienna(t)
	// A hypothetical path anchoring in the athlete's own zone: 00:00 Vienna is
	// 22:00 UTC the previous day — not UTC midnight, but still no real time.
	stored := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)

	assert.Equal(t, 22, stored.UTC().Hour(), "sanity: 00:00 Vienna is 22:00 UTC")
	assert.True(t, isMidnightSentinel(stored, loc))
}

func TestIsMidnightSentinel_RealTimesAreNotSentinels(t *testing.T) {
	loc := vienna(t)
	cases := []time.Time{
		time.Date(2026, 7, 20, 8, 0, 0, 0, loc),      // a plan-materialized 08:00
		time.Date(2026, 7, 20, 6, 0, 0, 0, loc),      // the materialize default
		time.Date(2026, 7, 20, 0, 1, 0, 0, time.UTC), // one minute off midnight anchors exactly
		time.Date(2026, 7, 20, 0, 0, 30, 0, time.UTC),
		time.Date(2026, 7, 20, 23, 59, 0, 0, loc),
	}
	for _, c := range cases {
		assert.False(t, isMidnightSentinel(c, loc), "%s must anchor as stated", c)
	}
}

// ============================================================================
// precedence
// ============================================================================

func TestResolveWindow_MidnightAssumesTheHabitualStart(t *testing.T) {
	loc := vienna(t)
	// A 2.5 h session scheduled by date alone.
	started := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	ended := started.Add(150 * time.Minute)

	from, to, source, assumed := resolveWindow(started, ended, loc, 6, 0, nil)

	assert.Equal(t, StartAssumed, source)
	assert.True(t, assumed, "an assumed hour must be flagged")
	assert.Equal(t, 6, from.In(loc).Hour(), "re-anchored at the habitual start")
	assert.Equal(t, 0, from.In(loc).Minute())
	// The calendar day is the athlete's reading of the stored instant, and the
	// duration survives the move.
	assert.Equal(t, 20, from.In(loc).Day())
	assert.Equal(t, 150*time.Minute, to.Sub(from))
}

func TestResolveWindow_RealTimeAnchorsThere(t *testing.T) {
	loc := vienna(t)
	started := time.Date(2026, 7, 20, 8, 30, 0, 0, loc)
	ended := started.Add(90 * time.Minute)

	from, to, source, assumed := resolveWindow(started, ended, loc, 6, 0, nil)

	assert.Equal(t, StartFromWorkout, source)
	assert.False(t, assumed, "nothing was assumed")
	assert.True(t, from.Equal(started), "a stated time is used verbatim")
	assert.True(t, to.Equal(ended))
}

func TestResolveWindow_ParamWinsOverEverything(t *testing.T) {
	loc := vienna(t)

	// Over a real time...
	started := time.Date(2026, 7, 20, 8, 0, 0, 0, loc)
	from, to, source, assumed := resolveWindow(started, started.Add(2*time.Hour), loc, 6, 0,
		&startOverride{hour: 10, min: 30})
	assert.Equal(t, StartFromParam, source)
	assert.False(t, assumed, "an explicit ask is not an assumption")
	assert.Equal(t, 10, from.In(loc).Hour())
	assert.Equal(t, 30, from.In(loc).Minute())
	assert.Equal(t, 2*time.Hour, to.Sub(from))

	// ...and over the midnight sentinel.
	midnight := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	from, _, source, assumed = resolveWindow(midnight, midnight.Add(time.Hour), loc, 6, 0,
		&startOverride{hour: 17, min: 0})
	assert.Equal(t, StartFromParam, source)
	assert.False(t, assumed)
	assert.Equal(t, 17, from.In(loc).Hour())
	assert.Equal(t, 20, from.In(loc).Day(), "still the same calendar day")
}

func TestResolveWindow_ConfiguredStartIsHonoured(t *testing.T) {
	loc := vienna(t)
	midnight := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	from, _, _, _ := resolveWindow(midnight, midnight.Add(time.Hour), loc, 17, 45, nil)
	assert.Equal(t, 17, from.In(loc).Hour(), "an evening athlete's config is respected")
	assert.Equal(t, 45, from.In(loc).Minute())
}

func TestResolveWindow_ZeroLengthSessionDoesNotInvert(t *testing.T) {
	loc := vienna(t)
	started := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	from, to, _, _ := resolveWindow(started, started.Add(-time.Hour), loc, 6, 0, nil)
	assert.False(t, to.Before(from), "a negative duration must not produce an inverted window")
}

// ============================================================================
// param parsing
// ============================================================================

func TestParseStartParam(t *testing.T) {
	h, m, err := ParseStartParam("10:30")
	require.NoError(t, err)
	assert.Equal(t, 10, h)
	assert.Equal(t, 30, m)

	// Midnight is a legitimate ASK, even though it's the sentinel when stored:
	// the param path never consults the sentinel.
	h, m, err = ParseStartParam("00:00")
	require.NoError(t, err)
	assert.Zero(t, h)
	assert.Zero(t, m)

	for _, bad := range []string{"", "25:00", "10:70", "10", "10:00:00", "abc", "10.30", "-1:00"} {
		_, _, err := ParseStartParam(bad)
		assert.ErrorIs(t, err, ErrStartInvalid, "start=%q must be rejected", bad)
	}
}

// A param of exactly midnight still scores midnight — the caller asked.
func TestResolveWindow_ParamMidnightIsNotTreatedAsUnknown(t *testing.T) {
	loc := vienna(t)
	started := time.Date(2026, 7, 20, 8, 0, 0, 0, loc)

	from, _, source, assumed := resolveWindow(started, started.Add(time.Hour), loc, 6, 0,
		&startOverride{hour: 0, min: 0})

	assert.Equal(t, StartFromParam, source)
	assert.False(t, assumed)
	assert.Equal(t, 0, from.In(loc).Hour(), "an explicit 00:00 is an answer, not a sentinel")
}
