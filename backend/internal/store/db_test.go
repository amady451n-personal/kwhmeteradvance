package store

import (
	"testing"
)

func TestOpenMigrates(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	// Second run must be a no-op.
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"readings", "tokens", "settings", "pin", "sessions", "auth_attempts", "push_subscriptions"} {
		var n int
		if err := db.SQL.QueryRow(`select count(*) from ` + table).Scan(&n); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}
}