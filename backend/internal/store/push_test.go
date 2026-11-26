package store

import (
	"errors"
	"testing"
	"time"
)

func TestSubscribeIsUpsertByEndpoint(t *testing.T) {
	db := testDB(t)
	first, err := db.Subscribe("https://push.example/1", "BKeyOne", "auth-one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Subscribe("https://push.example/1", "BKeyTwo", "auth-two")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("re-subscribe must keep the row: %s vs %s", first.ID, second.ID)
	}
	if second.Keys.P256dh != "BKeyTwo" || second.Keys.Auth != "auth-two" {
		t.Fatalf("keys not refreshed: %+v", second)
	}
	if n, err := db.CountSubscriptions(); err != nil || n != 1 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if err := db.Unsubscribe(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Unsubscribe(first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second unsubscribe = %v", err)
	}
	if n, _ := db.CountSubscriptions(); n != 0 {
		t.Fatalf("count after unsubscribe = %d", n)
	}
}

func TestListSubscriptionsAndPushLog(t *testing.T) {
	db := testDB(t)
	if list, err := db.ListSubscriptions(); err != nil || len(list) != 0 {
		t.Fatalf("empty list = %v, %v", list, err)
	}
	if _, err := db.Subscribe("https://push.example/1", "key", "auth"); err != nil {
		t.Fatal(err)
	}
	second, err := db.Subscribe("https://push.example/2", "key2", "auth2")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.ListSubscriptions()
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %v, %v", list, err)
	}
	if list[1].ID != second.ID {
		t.Fatalf("oldest first: got %s want %s", list[1].ID, second.ID)
	}

	if day, err := db.LastPushDay(); err != nil || day != "" {
		t.Fatalf("empty log = %q, %v", day, err)
	}
	if err := db.MarkPushDay("2026-11-25"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkPushDay("2026-11-26"); err != nil {
		t.Fatal(err)
	}
	if day, _ := db.LastPushDay(); day != "2026-11-26" {
		t.Fatalf("last day = %q", day)
	}
	if err := db.MarkPushDay("2026-11-26"); err != nil {
		t.Fatalf("repeat day: %v", err)
	}
}

func TestLatestReadingAt(t *testing.T) {
	db := testDB(t)
	if at, err := db.LatestReadingAt(); err != nil || at != "" {
		t.Fatalf("empty db = %q, %v", at, err)
	}
	newer := at(-1 * time.Hour)
	if _, err := db.UpsertReading(ReadingInput{ID: "r1", Reading: f64(100), Timestamp: at(-3 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertReading(ReadingInput{ID: "r2", Reading: f64(90), Timestamp: newer}); err != nil {
		t.Fatal(err)
	}
	got, err := db.LatestReadingAt()
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseTime(got)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ParseTime(newer)
	if !g.Equal(want) {
		t.Fatalf("latest = %q want %q", got, newer)
	}
}