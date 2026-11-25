package importer

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"meteran/internal/store"
)

func legacyDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meters.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.Exec(`create table readings (
		id integer primary key, meter_name text, reading real, timestamp text,
		notes text, recharge_amount real, cost integer)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`insert into readings values
		(1, 'main', 100, '2026-11-24 05:59:00', null, 0, 0),
		(2, 'main', 166, '2026-11-25 06:10:00', 'token', 66, 100000),
		(3, 'main', 160, '2026-11-25 12:00:00', null, null, null),
		(4, 'garage', 5, '2026-11-25 13:00:00', null, null, null)`)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanGroupsAndNormalisesTimestamps(t *testing.T) {
	preview, err := Scan(legacyDB(t))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Rows != 4 || preview.MainRows != 3 || preview.OtherMeterRows != 1 {
		t.Fatalf("source = %+v", preview)
	}
	if len(preview.Valid) != 3 {
		t.Fatalf("valid = %+v", preview.Valid)
	}
	if len(preview.Rejected) != 1 || preview.Rejected[0].Issues[0].Code != "other_meter" {
		t.Fatalf("rejected = %+v", preview.Rejected)
	}
	if len(preview.NeedsConfirm) != 0 {
		t.Fatalf("needsConfirm = %+v", preview.NeedsConfirm)
	}
	if got := *preview.Valid[0].Timestamp; got != "2026-11-24T05:59:00+07:00" {
		t.Fatalf("timestamp shifted: %s", got)
	}
	if preview.Stats.Rows != 3 || preview.Stats.TopupKwh != 66 || preview.Stats.TopupCost != 100000 {
		t.Fatalf("stats = %+v", preview.Stats)
	}
}

func TestCommitWritesOnceAndRefusesNonEmptyDB(t *testing.T) {
	preview, err := Scan(legacyDB(t))
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	stats, err := Commit(db, preview)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rows != 3 {
		t.Fatalf("stats = %+v", stats)
	}
	rows, err := db.AllReadings()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Source != "import" || rows[0].OldID == nil || *rows[0].OldID != "1" {
		t.Fatalf("rows = %+v", rows)
	}
	if _, err := Commit(db, preview); !errors.Is(err, store.ErrImportNotAllowed) {
		t.Fatalf("second commit must be refused, got %v", err)
	}
}

func TestScanAcceptsRealAndTextCost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meters.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`create table readings (
		id integer primary key, meter_name text, reading real, timestamp text,
		notes text, recharge_amount real, cost)`)
	if err == nil {
		_, err = conn.Exec(`insert into readings values
			(1, 'main', 100, '2026-11-24 05:59:00', null, 10, 50000.75),
			(2, 'main', 120, '2026-11-25 06:10:00', null, 20, '12000')`)
	}
	_ = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	preview, err := Scan(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Valid) != 2 {
		t.Fatalf("valid = %+v", preview.Rejected)
	}
	if got := *preview.Valid[0].Cost; got != 50001 {
		t.Fatalf("real cost = %d", got)
	}
	if got := *preview.Valid[1].Cost; got != 12000 {
		t.Fatalf("text cost = %d", got)
	}
}
