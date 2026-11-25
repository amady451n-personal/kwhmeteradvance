// Package importer reads the legacy "kwhmeter basic" SQLite snapshot
// (readings table only) and maps it 1:1 onto the new readings table.
package importer

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	"meteran/internal/store"
)

// numeric accepts the driver values a legacy cost column can produce.
func numeric(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case []byte:
		f, err := strconv.ParseFloat(strings.TrimSpace(string(x)), 64)
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

// Issue is one problem attached to an imported row.
type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Row is one legacy reading after normalisation.
type Row struct {
	OldID          *string  `json:"oldId"`
	Timestamp      *string  `json:"timestamp"`
	Reading        *float64 `json:"reading"`
	RechargeAmount *float64 `json:"rechargeAmount"`
	Cost           *int64   `json:"cost"`
	Notes          *string  `json:"notes"`
	Issues         []Issue  `json:"issues"`
}

// Stats summarise a set of rows (FR-105 verification).
type Stats struct {
	Rows      int     `json:"rows"`
	TopupKwh  float64 `json:"topupKwh"`
	TopupCost int64   `json:"topupCost"`
}

// Preview is the FR-104 grouping.
type Preview struct {
	Rows           int
	MainRows       int
	OtherMeterRows int
	Valid          []Row
	NeedsConfirm   []Row
	Rejected       []Row
	Stats          Stats // importable rows only
}

// Scan reads a legacy database. The file is opened read-only and never written.
func Scan(path string) (Preview, error) {
	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return Preview{}, err
	}
	defer conn.Close()

	rows, err := conn.Query(`select id, meter_name, reading, timestamp, notes, recharge_amount, cost
		from readings order by timestamp, id`)
	if err != nil {
		return Preview{}, fmt.Errorf("legacy schema: %w", err)
	}
	defer rows.Close()

	var p Preview
	seenTimestamps := map[string]bool{}
	var prev *Row
	for rows.Next() {
		var id, meter sql.NullString
		var reading, recharge sql.NullFloat64
		// legacy cost can be INTEGER, REAL, or TEXT depending on the sqlite version
		var cost any
		var ts, notes sql.NullString
		if err := rows.Scan(&id, &meter, &reading, &ts, &notes, &recharge, &cost); err != nil {
			return Preview{}, err
		}
		p.Rows++
		row := Row{}
		if id.Valid {
			row.OldID = &id.String
		}
		if notes.Valid {
			row.Notes = &notes.String
		}
		if recharge.Valid {
			row.RechargeAmount = &recharge.Float64
		}
		if v, ok := numeric(cost); ok {
			n := int64(math.Round(v))
			row.Cost = &n
		}
		if meter.Valid && !strings.EqualFold(strings.TrimSpace(meter.String), "main") {
			p.OtherMeterRows++
			row.Issues = append(row.Issues, Issue{"other_meter", "Baris milik meter lain."})
		} else {
			p.MainRows++
		}

		var parsed string
		if ts.Valid && ts.String != "" {
			if t, err := store.ParseTime(ts.String); err == nil {
				parsed = store.FormatTime(t)
				row.Timestamp = &parsed
			}
		}
		if row.Timestamp == nil {
			row.Issues = append(row.Issues, Issue{"empty_value", "Waktu catat kosong atau tidak dikenal."})
		}
		if reading.Valid {
			v := reading.Float64
			row.Reading = &v
			if v < 0 {
				row.Issues = append(row.Issues, Issue{"negative_value", "Angka meter negatif."})
			}
		} else {
			row.Issues = append(row.Issues, Issue{"empty_value", "Angka meter kosong."})
		}

		topup := 0.0
		if row.RechargeAmount != nil {
			topup = *row.RechargeAmount
		}
		if topup > 0 && (row.Cost == nil || *row.Cost <= 0) {
			row.Issues = append(row.Issues, Issue{"topup_without_cost", "Top-up tanpa biaya."})
		}
		if row.Cost != nil && *row.Cost > 0 && topup <= 0 {
			row.Issues = append(row.Issues, Issue{"cost_without_topup", "Biaya tanpa top-up."})
		}
		if row.Timestamp != nil {
			if seenTimestamps[*row.Timestamp] {
				row.Issues = append(row.Issues, Issue{"duplicate_timestamp", "Waktu catat dipakai lebih dari satu baris."})
			}
			seenTimestamps[*row.Timestamp] = true
		}
		if prev != nil && prev.Reading != nil && row.Reading != nil && *row.Reading > *prev.Reading && topup <= 0 {
			row.Issues = append(row.Issues, Issue{"increased_without_topup", "Angka naik tanpa top-up."})
		}

		switch {
		case hasReject(row.Issues):
			p.Rejected = append(p.Rejected, row)
		case len(row.Issues) > 0:
			p.NeedsConfirm = append(p.NeedsConfirm, row)
		default:
			p.Valid = append(p.Valid, row)
		}
		if !hasReject(row.Issues) {
			p.Stats.Rows++
			p.Stats.TopupKwh += topup
			if row.Cost != nil {
				p.Stats.TopupCost += *row.Cost
			}
		}
		prev = &row
	}
	return p, rows.Err()
}

var rejectCodes = map[string]bool{
	"empty_value": true, "negative_value": true, "other_meter": true, "invalid_timestamp": true,
}

func hasReject(issues []Issue) bool {
	for _, i := range issues {
		if rejectCodes[i.Code] {
			return true
		}
	}
	return false
}

// Importable returns the rows a commit will write.
func (p Preview) Importable() []Row {
	out := make([]Row, 0, len(p.Valid)+len(p.NeedsConfirm))
	out = append(out, p.Valid...)
	out = append(out, p.NeedsConfirm...)
	return out
}

// Commit writes all importable rows in one transaction (FR-101) and refuses a
// non-empty database (FR-100).
func Commit(db *store.DB, p Preview) (Stats, error) {
	var count int
	if err := db.SQL.QueryRow(`select count(*) from readings`).Scan(&count); err != nil {
		return Stats{}, err
	}
	if count > 0 {
		return Stats{}, store.ErrImportNotAllowed
	}
	now := store.Now()
	tx, err := db.SQL.Begin()
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback()

	var written Stats
	for _, row := range p.Importable() {
		if row.Timestamp == nil || row.Reading == nil {
			continue
		}
		id := "legacy-" + value(row.OldID)
		_, err := tx.Exec(`insert into readings
			(id, meter_name, reading, timestamp, notes, recharge_amount, cost, source, old_id, created_at, updated_at)
			values (?, 'main', ?, ?, ?, ?, ?, 'import', ?, ?, ?)`,
			id, *row.Reading, *row.Timestamp, row.Notes, row.RechargeAmount, row.Cost, row.OldID, now, now)
		if err != nil {
			return Stats{}, fmt.Errorf("insert %s: %w", id, err)
		}
		written.Rows++
		if row.RechargeAmount != nil {
			written.TopupKwh += *row.RechargeAmount
		}
		if row.Cost != nil {
			written.TopupCost += *row.Cost
		}
	}
	if err := tx.Commit(); err != nil {
		return Stats{}, err
	}
	return written, nil
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
