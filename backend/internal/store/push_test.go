package store

import (
	"errors"
	"testing"
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