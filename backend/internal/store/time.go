package store

import "time"

// WIB is the only zone this app writes. Offsets are explicit in every
// timestamp so imported legacy wall-clock values never shift.
var WIB = time.FixedZone("WIB", 7*3600)

const layout = "2006-01-02T15:04:05-07:00"

// Now returns the current time as a WIB ISO-8601 string.
func Now() string { return FormatTime(time.Now()) }

func FormatTime(t time.Time) string { return t.In(WIB).Format(layout) }

// ParseTime accepts RFC3339 (Z or offset), a bare date, or the legacy
// "YYYY-MM-DD HH:MM:SS[.fff]" form which is interpreted as WIB wall-clock.
func ParseTime(s string) (time.Time, error) {
	for _, l := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
	} {
		if t, err := time.ParseInLocation(l, s, WIB); err == nil {
			return t, nil
		}
	}
	return time.ParseInLocation("2006-01-02", s, WIB)
}

// Day returns the WIB midnight that starts t's day.
func Day(t time.Time) time.Time {
	y, m, d := t.In(WIB).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, WIB)
}