package heat

import (
	"errors"
	"time"
)

// ErrStartInvalid is returned for a malformed `start` parameter.
var ErrStartInvalid = errors.New("start must be HH:MM (24-hour)")

// StartSource names which rule produced the scored window — the difference
// between "this is when you're riding" and "this is when we guessed".
type StartSource string

const (
	// StartFromWorkout: the planned workout carries a real time; nothing assumed.
	StartFromWorkout StartSource = "workout"
	// StartAssumed: the workout is midnight-anchored (the date-only scheduling
	// path stores no time of day), so the configured habitual start applied.
	StartAssumed StartSource = "assumed"
	// StartFromParam: the caller asked "what if I start at HH:MM".
	StartFromParam StartSource = "param"
)

// defaultStartHour backs an unconfigured training start.
const defaultStartHour = 6

// isMidnightSentinel reports whether a stored start instant carries no real
// time of day.
//
// The scheduling path that produces these rows parses a date-only string
// (`time.Parse("2006-01-02")`), which yields **UTC** midnight — so the check
// must look at UTC, not only at the athlete's local zone: for a Vienna athlete
// a stored 00:00 UTC reads as 02:00 local and a local-only test would miss the
// very rows this exists to catch. Local midnight is checked too, so a future
// path that anchors in the athlete's zone is caught the same way.
//
// The false positive — someone genuinely starting at exactly midnight (or at
// exactly 02:00 in Vienna, which is 00:00 UTC) — is accepted and made visible:
// the response says the start was assumed, and a real time one minute off
// either midnight anchors exactly.
func isMidnightSentinel(startedAt time.Time, loc *time.Location) bool {
	utc := startedAt.UTC()
	if utc.Hour() == 0 && utc.Minute() == 0 && utc.Second() == 0 && utc.Nanosecond() == 0 {
		return true
	}
	local := startedAt.In(loc)
	return local.Hour() == 0 && local.Minute() == 0 && local.Second() == 0 && local.Nanosecond() == 0
}

// ParseStartParam validates an `HH:MM` override.
func ParseStartParam(s string) (hour, min int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, ErrStartInvalid
	}
	return t.Hour(), t.Minute(), nil
}

// anchorAt re-anchors a session onto `date`'s calendar day at hh:mm in loc,
// preserving its duration.
func anchorAt(startedAt time.Time, duration time.Duration, hh, mm int, loc *time.Location) (time.Time, time.Time) {
	// The calendar day comes from the athlete's local reading of the stored
	// instant: a 00:00 UTC row is "that date" to them, not the previous evening.
	local := startedAt.In(loc)
	from := time.Date(local.Year(), local.Month(), local.Day(), hh, mm, 0, 0, loc)
	return from, from.Add(duration)
}

// resolveWindow applies the precedence: an explicit param wins, else a real
// start time on the workout, else the configured habitual start.
//
// Returns the window to score, which rule produced it, and whether an
// assumption was made (the caller echoes all three — a suggestion resting on a
// guessed hour must say so).
func resolveWindow(
	startedAt, endedAt time.Time,
	loc *time.Location,
	defaultHour, defaultMin int,
	param *startOverride,
) (from, to time.Time, source StartSource, assumed bool) {
	duration := endedAt.Sub(startedAt)
	if duration < 0 {
		duration = 0
	}

	if param != nil {
		f, t := anchorAt(startedAt, duration, param.hour, param.min, loc)
		return f, t, StartFromParam, false
	}
	if !isMidnightSentinel(startedAt, loc) {
		return startedAt, endedAt, StartFromWorkout, false
	}
	f, t := anchorAt(startedAt, duration, defaultHour, defaultMin, loc)
	return f, t, StartAssumed, true
}

// startOverride is a parsed `start=HH:MM`.
type startOverride struct {
	hour int
	min  int
}
