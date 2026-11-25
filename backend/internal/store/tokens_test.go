package store

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newToken(t *testing.T, db *DB, id string) Token {
	t.Helper()
	tok, err := db.CreateToken(TokenInput{
		ID: id, Code: "1234-5678-9012", Kwh: f64(66), Cost: i64(100000),
		PurchasedAt: at(-6 * time.Hour), Type: "additional",
		Confirm: []string{ConfirmDenominationOutOfTol},
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestCreateTokenStoresEncryptedCode(t *testing.T) {
	db := testDB(t)
	tok := newToken(t, db, "t1")
	if tok.Status != "unused" || !tok.HasCode || tok.CanDelete {
		t.Fatalf("token = %+v", tok)
	}
	var raw []byte
	if err := db.SQL.QueryRow(`select code_enc from tokens where id = 't1'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "1234-5678") {
		t.Fatal("code must not be stored in plaintext")
	}
	code, err := db.RevealTokenCode("t1")
	if err != nil || code != "1234-5678-9012" {
		t.Fatalf("reveal = %q %v", code, err)
	}
}

func TestTokenRejectsCodeWhenCreate(t *testing.T) {
	db := testDB(t)
	_, err := db.CreateToken(TokenInput{ID: "t1", Kwh: f64(10), PurchasedAt: at(-time.Hour), Type: "additional"})
	var val ErrValidation
	if !errors.As(err, &val) || val.Details[0].Field != "code" {
		t.Fatalf("got %v", err)
	}
}

func TestPatchUsedTokenRejected(t *testing.T) {
	db := testDB(t)
	newToken(t, db, "t1")
	if _, _, err := db.MarkTokenUsed("t1", ReadingInput{Reading: f64(166), Timestamp: at(-5 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	kind := "bulk"
	_, err := db.PatchToken("t1", TokenPatch{Type: &kind})
	var val ErrValidation
	if !errors.As(err, &val) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDeleteTokenNeedsFiftyRefills(t *testing.T) {
	db := testDB(t)
	newToken(t, db, "t1")
	if _, _, err := db.MarkTokenUsed("t1", ReadingInput{Reading: f64(166), Timestamp: at(-5 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	// Drop the meter reading to lock the token.
	if _, err := db.UpsertReading(ReadingInput{ID: "low", Reading: f64(5), Timestamp: at(-4 * time.Hour)}); err != nil {
		t.Fatalf("lock reading: %v", err)
	}
	locked, err := db.GetToken("t1")
	if err != nil {
		t.Fatal(err)
	}
	if locked.Status != "locked" || locked.CanDelete || locked.DeleteBlockedReason == nil || *locked.DeleteBlockedReason != "refills_pending" {
		t.Fatalf("locked token = %+v", locked)
	}
	if err := db.DeleteToken("t1"); !errors.Is(err, ErrTokenNotDeletable) {
		t.Fatalf("expected not deletable, got %v", err)
	}
	// 50 top-ups after the lock make it historical enough to delete.
	for i := 0; i < 50; i++ {
		_, err := db.UpsertReading(ReadingInput{
			ID: "topup-" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Reading: f64(50), Timestamp: at(-3*time.Hour + time.Duration(i)*time.Minute),
			RechargeAmount: f64(20),
			Confirm:        []string{ConfirmDuplicateTimestamp, ConfirmIncreasedWithoutTopup},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	again, err := db.GetToken("t1")
	if err != nil {
		t.Fatal(err)
	}
	if !again.CanDelete || again.RefillsSinceLock == nil || *again.RefillsSinceLock < 50 {
		t.Fatalf("token should be deletable: %+v", again)
	}
	if err := db.DeleteToken("t1"); err != nil {
		t.Fatal(err)
	}
}

func TestCancelUsedTokenRemovesLinkedReading(t *testing.T) {
	db := testDB(t)
	newToken(t, db, "t1")
	_, reading, err := db.MarkTokenUsed("t1", ReadingInput{Reading: f64(166), Timestamp: at(-4 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	tok, cancelled, err := db.CancelToken("t1")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Status != "unused" || cancelled == nil || *cancelled != reading.ID {
		t.Fatalf("cancel = %+v %v", tok, cancelled)
	}
	if _, err := db.GetReading(reading.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reading must be gone, got %v", err)
	}
	// Cancelling again is a no-op.
	if tok, cancelled, err := db.CancelToken("t1"); err != nil || tok.Status != "unused" || cancelled != nil {
		t.Fatalf("second cancel = %+v %v %v", tok, cancelled, err)
	}
}

func TestCancelLockedTokenFails(t *testing.T) {
	db := testDB(t)
	newToken(t, db, "t1")
	if _, _, err := db.MarkTokenUsed("t1", ReadingInput{Reading: f64(166), Timestamp: at(-5 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertReading(ReadingInput{ID: "low", Reading: f64(1), Timestamp: at(-4 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.CancelToken("t1"); !errors.Is(err, ErrTokenLocked) {
		t.Fatalf("expected token_locked, got %v", err)
	}
}

func TestAutoLockFiresOnceAndNeverUnlocks(t *testing.T) {
	db := testDB(t)
	newToken(t, db, "t1")
	if _, _, err := db.MarkTokenUsed("t1", ReadingInput{Reading: f64(166), Timestamp: at(-5 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertReading(ReadingInput{ID: "lower", Reading: f64(160), Timestamp: at(-4 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	first, err := db.GetToken("t1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "locked" || first.LockedAt == nil {
		t.Fatalf("token must lock: %+v", first)
	}
	// A later reading above the linked one must not unlock it.
	if _, err := db.UpsertReading(ReadingInput{ID: "higher", Reading: f64(200), Timestamp: at(-3 * time.Hour), Confirm: []string{ConfirmIncreasedWithoutTopup}}); err != nil {
		t.Fatal(err)
	}
	second, err := db.GetToken("t1")
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "locked" || *second.LockedAt != *first.LockedAt {
		t.Fatalf("lock must be stable: %+v", second)
	}
}