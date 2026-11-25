package store

import (
	"errors"
	"testing"
	"time"
)

func TestParseTimeLegacyIsWIB(t *testing.T) {
	for _, in := range []string{"2026-11-24 05:59:00", "2026-11-24 05:59:00.123456", "2026-11-24T05:59:00+07:00"} {
		got, err := ParseTime(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if FormatTime(got) != "2026-11-24T05:59:00+07:00" {
			t.Fatalf("%s formatted as %s", in, FormatTime(got))
		}
	}
	// A UTC instant keeps its instant, only the offset changes.
	got, err := ParseTime("2026-11-24T00:00:00Z")
	if err != nil || FormatTime(got) != "2026-11-24T07:00:00+07:00" {
		t.Fatalf("utc: %v %s", err, FormatTime(got))
	}
	if _, err := ParseTime("nope"); err == nil || errors.Is(err, nil) {
		t.Fatal("expected parse error")
	}
}

func TestDayIsWIBMidnight(t *testing.T) {
	got := Day(time.Date(2026, 11, 24, 23, 30, 0, 0, time.UTC)) // 06:30 WIB next day
	if FormatTime(got) != "2026-11-25T00:00:00+07:00" {
		t.Fatal(FormatTime(got))
	}
}