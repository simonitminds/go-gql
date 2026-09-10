package schedule

import "testing"

func TestWeekday(t *testing.T) {
	cases := []struct {
		date        string
		weekday     int
		offSchedule bool
	}{
		{"2025-02-06", 4, false}, // a Thursday
		{"2025-02-05", 3, true},  // a Wednesday
		{"2025-02-09", 0, true},  // a Sunday
		{"2025-02-08", 6, true},  // a Saturday
		{"2025-03-27", 4, false}, // a Thursday after the DST switch
		{"", UnknownWeekday, true},
		{"not-a-date", UnknownWeekday, true},
	}

	for _, c := range cases {
		if got := Weekday(c.date); got != c.weekday {
			t.Errorf("Weekday(%q) = %d, want %d", c.date, got, c.weekday)
		}
		if got := IsOffSchedule(c.date); got != c.offSchedule {
			t.Errorf("IsOffSchedule(%q) = %v, want %v", c.date, got, c.offSchedule)
		}
	}
}
