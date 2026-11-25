package store

import (
	"errors"
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }
func str(v string) *string   { return &v }

func at(offset time.Duration) string {
	return FormatTime(time.Now().Add(offset))
}

func TestUpsertReadingIsIdempotent(t *testing.T) {
	db := testDB(t)
	in := ReadingInput{ID: "r1", Reading: f64(100), Timestamp: at(-2 * time.Hour)}
	if _, err := db.UpsertReading(in); err != nil {
		t.Fatal(err)
	}
	first, err := db.GetReading("r1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertReading(in); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.SQL.QueryRow(`select count(*) from readings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d", n)
	}
	second, _ := db.GetReading("r1")
	if second.CreatedAt != first.CreatedAt {
		t.Fatal("replay must not change createdAt")
	}
}

func TestUpsertReadingSoftConfirmations(t *testing.T) {
	db := testDB(t)
	if _, err := db.UpsertReading(ReadingInput{ID: "a", Reading: f64(100), Timestamp: at(-3 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	// Same timestamp, different id.
	dup := ReadingInput{ID: "b", Reading: f64(90), Timestamp: at(-3 * time.Hour)}
	_, err := db.UpsertReading(dup)
	var conf ErrConfirmation
	if !errors.As(err, &conf) || conf.Details[0].Code != ConfirmDuplicateTimestamp {
		t.Fatalf("expected duplicate timestamp, got %v", err)
	}
	dup.Confirm = []string{ConfirmDuplicateTimestamp}
	if _, err := db.UpsertReading(dup); err != nil {
		t.Fatal(err)
	}

	// Reading goes up without a top-up.
	up := ReadingInput{ID: "c", Reading: f64(120), Timestamp: at(-1 * time.Hour)}
	_, err = db.UpsertReading(up)
	conf = ErrConfirmation{}
	if !errors.As(err, &conf) || conf.Details[0].Code != ConfirmIncreasedWithoutTopup {
		t.Fatalf("expected increased_without_topup, got %v", err)
	}
	up.Confirm = []string{ConfirmIncreasedWithoutTopup}
	if _, err := db.UpsertReading(up); err != nil {
		t.Fatal(err)
	}
	// A top-up explains the growth, so no confirmation is needed.
	if _, err := db.UpsertReading(ReadingInput{ID: "d", Reading: f64(160), Timestamp: at(-30 * time.Minute), RechargeAmount: f64(50)}); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertReadingHardValidation(t *testing.T) {
	db := testDB(t)
	if _, err := db.UpsertReading(ReadingInput{ID: "x", Reading: f64(10), Timestamp: at(2 * time.Hour)}); err == nil {
		t.Fatal("future timestamp must be rejected")
	} else {
		var val ErrValidation
		if !errors.As(err, &val) || val.Details[0].Code != "future_timestamp" {
			t.Fatalf("got %v", err)
		}
	}
	if _, err := db.UpsertReading(ReadingInput{ID: "y", Reading: f64(-1), Timestamp: at(-time.Hour)}); err == nil {
		t.Fatal("negative reading must be rejected")
	}
	if _, err := db.UpsertReading(ReadingInput{ID: "z", Timestamp: at(-time.Hour)}); err == nil {
		t.Fatal("missing reading must be rejected")
	}
}

func TestDeleteReadingRevertsUsedToken(t *testing.T) {
	db := testDB(t)
	tok, err := db.CreateToken(TokenInput{ID: "t1", Code: "1234-5678", Kwh: f64(66), Cost: i64(100000), PurchasedAt: at(-5 * time.Hour), Type: "additional", Confirm: []string{ConfirmDenominationOutOfTol}})
	if err != nil {
		t.Fatal(err)
	}
	used, reading, err := db.MarkTokenUsed(tok.ID, ReadingInput{Reading: f64(166), Timestamp: at(-4 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if used.Status != "used" || used.LinkedReadingID == nil || *used.LinkedReadingID != reading.ID {
		t.Fatalf("token after use = %+v", used)
	}
	if reading.TokenID == nil || *reading.TokenID != tok.ID || reading.RechargeAmount == nil {
		t.Fatalf("reading after use = %+v", reading)
	}
	if err := db.DeleteReading(reading.ID); err != nil {
		t.Fatal(err)
	}
	back, err := db.GetToken(tok.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Status != "unused" || back.LinkedReadingID != nil {
		t.Fatalf("token after delete = %+v", back)
	}
}

func TestListReadingsNewestFirstWithCursor(t *testing.T) {
	db := testDB(t)
	for i, offset := range []time.Duration{-5 * time.Hour, -4 * time.Hour, -3 * time.Hour} {
		if _, err := db.UpsertReading(ReadingInput{
			ID: string(rune('a' + i)), Reading: f64(float64(100 + i)), Timestamp: at(offset),
			Confirm: []string{ConfirmIncreasedWithoutTopup},
		}); err != nil {
			t.Fatal(err)
		}
	}
	page, meta, err := db.ListReadings(ReadingQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || !meta.HasMore || meta.NextCursor == nil {
		t.Fatalf("page=%d meta=%+v", len(page), meta)
	}
	if page[0].Reading != 102 || page[1].Reading != 101 {
		t.Fatalf("order = %v, %v", page[0].Reading, page[1].Reading)
	}
	next, meta2, err := db.ListReadings(ReadingQuery{Limit: 2, Cursor: *meta.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].Reading != 100 || meta2.HasMore {
		t.Fatalf("second page = %+v meta=%+v", next, meta2)
	}
}