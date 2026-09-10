// Package schedule holds the single definition of when burger day is supposed to
// happen. Burger day is always on a Thursday; days on any other weekday are
// almost always test runs and are considered "off-schedule".
package schedule

import "time"

// BurgerWeekday is the weekday burger day is held on: Thursday (4).
const BurgerWeekday = int(time.Thursday)

// UnknownWeekday is returned for burger days whose stored date cannot be parsed.
const UnknownWeekday = -1

// dateLayout is the layout burger day dates are stored in, e.g. "2025-02-06".
const dateLayout = "2006-01-02"

// location is Europe/Copenhagen, falling back to UTC when the zone database is
// unavailable. A date-only value has the same weekday in either zone, so the
// fallback never changes an answer; it only keeps the rule explicit.
var location = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// Weekday returns the weekday of a stored burger day date in Europe/Copenhagen,
// with 0 = Sunday ... 6 = Saturday. It returns UnknownWeekday for dates it cannot
// parse, so a single bad row never fails a whole query.
func Weekday(date string) int {
	t, err := time.ParseInLocation(dateLayout, date, location)
	if err != nil {
		return UnknownWeekday
	}
	return int(t.Weekday())
}

// IsOffSchedule reports whether a burger day date falls outside the Thursday
// schedule. Unparseable dates count as off-schedule.
func IsOffSchedule(date string) bool {
	return Weekday(date) != BurgerWeekday
}
