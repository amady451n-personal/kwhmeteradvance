package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Reading is one meter snapshot. locked is derived from the linked token.
type Reading struct {
	ID             string   `json:"id"`
	MeterName      string   `json:"meterName"`
	Reading        float64  `json:"reading"`
	Timestamp      string   `json:"timestamp"`
	Notes          *string  `json:"notes"`
	RechargeAmount *float64 `json:"rechargeAmount"`
	Cost           *int64   `json:"cost"`
	TokenID        *string  `json:"tokenId"`
	Source         string   `json:"source"`
	Locked         bool     `json:"locked"`
	OldID          *string  `json:"oldId"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// ReadingInput is the accepted write shape (ReadingInput / TokenMarkUsed).
type ReadingInput struct {
	ID             string   `json:"id"`
	MeterName      string   `json:"meterName"`
	Reading        *float64 `json:"reading"`
	Timestamp      string   `json:"timestamp"`
	Notes          *string  `json:"notes"`
	RechargeAmount *float64 `json:"rechargeAmount"`
	Cost           *int64   `json:"cost"`
	TokenID        *string  `json:"tokenId"`
	Source         string   `json:"source"`
	OldID          *string  `json:"oldId"`
	Confirm        []string `json:"confirm"`
}

// PageMeta is the contract's cursor metadata.
type PageMeta struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
	Total      *int    `json:"total,omitempty"`
}

// ReadingQuery filters and pages the reading list.
type ReadingQuery struct {
	From, To string
	Limit    int
	Cursor   string
}

const readingCols = `r.id, r.meter_name, r.reading, r.timestamp, r.notes, r.recharge_amount,
	r.cost, r.token_id, r.source, r.old_id, r.created_at, r.updated_at,
	coalesce(t.status = 'locked', 0)`

func scanReading(sc interface{ Scan(...any) error }) (Reading, error) {
	var x Reading
	var notes, tokenID, oldID sql.NullString
	var recharge sql.NullFloat64
	var cost sql.NullInt64
	var locked int
	err := sc.Scan(&x.ID, &x.MeterName, &x.Reading, &x.Timestamp, &notes, &recharge,
		&cost, &tokenID, &x.Source, &oldID, &x.CreatedAt, &x.UpdatedAt, &locked)
	if err != nil {
		return Reading{}, err
	}
	if notes.Valid {
		x.Notes = &notes.String
	}
	if recharge.Valid {
		x.RechargeAmount = &recharge.Float64
	}
	if cost.Valid {
		x.Cost = &cost.Int64
	}
	if tokenID.Valid {
		x.TokenID = &tokenID.String
	}
	if oldID.Valid {
		x.OldID = &oldID.String
	}
	x.Locked = locked == 1
	return x, nil
}

const readingSelect = `select ` + readingCols + ` from readings r left join tokens t on t.id = r.token_id`

func (db *DB) ListReadings(q ReadingQuery) ([]Reading, PageMeta, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 20
	}
	where := []string{"1 = 1"}
	args := []any{}
	if q.From != "" {
		where = append(where, "r.timestamp >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "r.timestamp < ?")
		args = append(args, q.To)
	}
	if q.Cursor != "" {
		ts, id, ok := decodeCursor(q.Cursor)
		if ok {
			where = append(where, "(r.timestamp < ? or (r.timestamp = ? and r.id < ?))")
			args = append(args, ts, ts, id)
		}
	}
	args = append(args, q.Limit+1) // one extra row tells us hasMore
	rows, err := db.SQL.Query(readingSelect+` where `+strings.Join(where, " and ")+
		` order by r.timestamp desc, r.id desc limit ?`, args...)
	if err != nil {
		return nil, PageMeta{}, err
	}
	defer rows.Close()
	out := []Reading{}
	for rows.Next() {
		x, err := scanReading(rows)
		if err != nil {
			return nil, PageMeta{}, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, err
	}
	meta := PageMeta{}
	if len(out) > q.Limit {
		out = out[:q.Limit]
		last := out[len(out)-1]
		c := encodeCursor(last.Timestamp, last.ID)
		meta.NextCursor = &c
		meta.HasMore = true
	}
	return out, meta, nil
}

func (db *DB) GetReading(id string) (Reading, error) {
	x, err := scanReading(db.SQL.QueryRow(readingSelect+` where r.id = ?`, id))
	if isNoRows(err) {
		return Reading{}, ErrNotFound
	}
	return x, err
}

// UpsertReading creates or replaces a reading. Soft violations return
// ErrConfirmation unless the matching code is in the confirm list.
func (db *DB) UpsertReading(in ReadingInput) (Reading, error) {
	if in.Reading == nil || in.Timestamp == "" {
		return Reading{}, validation("reading", "required", "Angka meter dan waktu wajib diisi.")
	}
	ts, err := ParseTime(in.Timestamp)
	if err != nil {
		return Reading{}, validation("timestamp", "invalid_timestamp", "Waktu catat tidak valid.")
	}
	if ts.After(time.Now()) {
		return Reading{}, validation("timestamp", "future_timestamp", "Waktu catat tidak boleh di masa depan.")
	}
	if *in.Reading < 0 {
		return Reading{}, validation("reading", "negative_value", "Angka meter tidak boleh negatif.")
	}
	if in.ID == "" {
		in.ID = NewID("reading")
	}
	if in.MeterName == "" {
		in.MeterName = "main"
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	stamp := FormatTime(ts)

	tx, err := db.SQL.Begin()
	if err != nil {
		return Reading{}, err
	}
	defer tx.Rollback()

	var existingToken sql.NullString
	err = tx.QueryRow(`select coalesce(token_id, '') from readings where id = ?`, in.ID).Scan(&existingToken)
	if err != nil && !isNoRows(err) {
		return Reading{}, err
	}
	if existingToken.String != "" {
		var status string
		if err := tx.QueryRow(`select status from tokens where id = ?`, existingToken.String).Scan(&status); err == nil && status == "locked" {
			return Reading{}, ErrReadingLocked
		}
	}

	if err := checkReadingTx(tx, in, stamp); err != nil {
		return Reading{}, err
	}

	now := Now()
	var createdAt string
	err = tx.QueryRow(`select created_at from readings where id = ?`, in.ID).Scan(&createdAt)
	if isNoRows(err) {
		createdAt = now
	} else if err != nil {
		return Reading{}, err
	}
	_, err = tx.Exec(`insert into readings
		(id, meter_name, reading, timestamp, notes, recharge_amount, cost, token_id, source, old_id, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (id) do update set
			meter_name = excluded.meter_name, reading = excluded.reading, timestamp = excluded.timestamp,
			notes = excluded.notes, recharge_amount = excluded.recharge_amount, cost = excluded.cost,
			token_id = excluded.token_id, source = excluded.source, old_id = excluded.old_id,
			updated_at = excluded.updated_at`,
		in.ID, in.MeterName, *in.Reading, stamp, in.Notes, in.RechargeAmount, in.Cost,
		in.TokenID, in.Source, in.OldID, createdAt, now)
	if err != nil {
		return Reading{}, err
	}
	if err := lockLoweredTokens(tx, in.ID, *in.Reading, stamp, now); err != nil {
		return Reading{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reading{}, err
	}
	return db.GetReading(in.ID)
}

// lockLoweredTokens applies FR-13: a token in use becomes historical once the
// meter reads lower than the reading recorded when it was inserted. locked_at
// is that triggering reading's timestamp, so "refills since lock" counts the
// top-ups that came after it. Locking is one-way, so re-running it changes
// nothing.
func lockLoweredTokens(tx *sql.Tx, readingID string, value float64, at, now string) error {
	_, err := tx.Exec(`update tokens set status = 'locked', locked_at = ?, updated_at = ?
		where status = 'used'
		  and linked_reading_id is not null
		  and linked_reading_id <> ?
		  and (select reading from readings where id = tokens.linked_reading_id) > ?`,
		at, now, readingID, value)
	return err
}

func checkReadingTx(tx *sql.Tx, in ReadingInput, stamp string) error {
	confirmed := map[string]bool{}
	for _, c := range in.Confirm {
		confirmed[c] = true
	}
	var details []Detail

	if !confirmed[ConfirmDuplicateTimestamp] {
		var n int
		if err := tx.QueryRow(`select count(*) from readings where timestamp = ? and id <> ?`, stamp, in.ID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			details = append(details, Detail{Field: "timestamp", Code: ConfirmDuplicateTimestamp, Message: "Waktu catatan sudah dipakai."})
		}
	}
	if !confirmed[ConfirmIncreasedWithoutTopup] && (in.RechargeAmount == nil || *in.RechargeAmount <= 0) {
		var prev float64
		err := tx.QueryRow(`select reading from readings where id <> ? and timestamp < ?
			order by timestamp desc, id desc limit 1`, in.ID, stamp).Scan(&prev)
		if err != nil && !isNoRows(err) {
			return err
		}
		if err == nil && *in.Reading > prev {
			details = append(details, Detail{Field: "reading", Code: ConfirmIncreasedWithoutTopup, Message: "Angka naik tanpa isi ulang. Pastikan tidak salah ketik."})
		}
	}
	if len(details) > 0 {
		return ErrConfirmation{Details: details}
	}
	return nil
}

// DeleteReading removes a reading and reverts a not-yet-locked token that
// pointed at it (FR-13 step 4).
func (db *DB) DeleteReading(id string) error {
	tx, err := db.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tokenID sql.NullString
	err = tx.QueryRow(`select coalesce(token_id, '') from readings where id = ?`, id).Scan(&tokenID)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if tokenID.String != "" {
		var status string
		if err := tx.QueryRow(`select status from tokens where id = ?`, tokenID.String).Scan(&status); err == nil && status == "locked" {
			return ErrReadingLocked
		}
	}
	if _, err := tx.Exec(`delete from readings where id = ?`, id); err != nil {
		return err
	}
	_, err = tx.Exec(`update tokens set status = 'unused', linked_reading_id = null, locked_at = null, updated_at = ?
		where linked_reading_id = ? and status = 'used'`, Now(), id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// NormalizeRange parses the from/to query values into stored timestamp form.
func NormalizeRange(from, to string) (string, string, error) {
	var lo, hi string
	if from != "" {
		t, err := ParseTime(from)
		if err != nil {
			return "", "", validation("from", "invalid_timestamp", "Batas awal tidak valid.")
		}
		lo = FormatTime(t)
	}
	if to != "" {
		t, err := ParseTime(to)
		if err != nil {
			return "", "", validation("to", "invalid_timestamp", "Batas akhir tidak valid.")
		}
		hi = FormatTime(t)
	}
	return lo, hi, nil
}

// AllReadings returns the full history in ascending order, used by summaries.
// LatestReadingAt is the newest reading timestamp, empty when the meter has no
// readings yet.
func (db *DB) LatestReadingAt() (string, error) {
	var ts sql.NullString
	if err := db.SQL.QueryRow(`select max(timestamp) from readings`).Scan(&ts); err != nil {
		return "", err
	}
	return ts.String, nil
}

func (db *DB) AllReadings() ([]Reading, error) {
	rows, err := db.SQL.Query(readingSelect + ` order by r.timestamp asc, r.id asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reading
	for rows.Next() {
		x, err := scanReading(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func encodeCursor(ts, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(ts + "\x00" + id))
}

func decodeCursor(c string) (ts, id string, ok bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(b), "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// NewID returns an opaque client-compatible identifier.
func NewID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b)
}