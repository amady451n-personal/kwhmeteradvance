package auth

import (
	"path/filepath"
	"testing"

	"meteran/internal/store"
)

func TestPinHashVerifyRoundTrip(t *testing.T) {
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	if h == "1234" || !VerifyPin(h, "1234") {
		t.Fatal("hash must verify")
	}
	if VerifyPin(h, "4321") {
		t.Fatal("wrong pin must fail")
	}
	if VerifyPin("garbage", "1234") {
		t.Fatal("garbage hash must fail")
	}
}

func TestLimiterBlocksAfterFiveAndSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meteran.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	l := Limiter{DB: db}
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("first attempt must be allowed")
	}
	for i := 0; i < MaxAttempts; i++ {
		if err := l.Fail("1.2.3.4"); err != nil {
			t.Fatal(err)
		}
	}
	if ok, wait := l.Allow("1.2.3.4"); ok || wait <= 0 {
		t.Fatalf("must be blocked with a wait, got ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Fatal("other keys must be unaffected")
	}
	db.Close()

	db2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if ok, _ := (Limiter{DB: db2}).Allow("1.2.3.4"); ok {
		t.Fatal("block must survive a restart")
	}
	if err := (Limiter{DB: db2}).Reset("1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := (Limiter{DB: db2}).Allow("1.2.3.4"); !ok {
		t.Fatal("reset must unblock")
	}
}