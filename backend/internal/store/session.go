package store

import (
	"database/sql"
	"time"
)

// SessionIdle is the contract's idle timeout.
const SessionIdle = 30 * time.Minute

func (db *DB) HasPin() (bool, error) {
	var n int
	err := db.SQL.QueryRow(`select count(*) from pin`).Scan(&n)
	return n > 0, err
}

func (db *DB) PinHash() (string, error) {
	var h string
	err := db.SQL.QueryRow(`select hash from pin where id = 1`).Scan(&h)
	if isNoRows(err) {
		return "", ErrNotFound
	}
	return h, err
}

func (db *DB) SetPin(hash string) error {
	_, err := db.SQL.Exec(`insert into pin (id, hash, updated_at) values (1, ?, ?)
		on conflict (id) do update set hash = excluded.hash, updated_at = excluded.updated_at`,
		hash, Now())
	return err
}

func (db *DB) CreateSession(token string) error {
	now := Now()
	_, err := db.SQL.Exec(`insert into sessions (token, created_at, last_seen_at) values (?, ?, ?)`, token, now, now)
	return err
}

// SessionValid checks the idle window and slides it forward.
func (db *DB) SessionValid(token string) bool {
	if token == "" {
		return false
	}
	cutoff := FormatTime(time.Now().Add(-SessionIdle))
	res, err := db.SQL.Exec(`update sessions set last_seen_at = ? where token = ? and last_seen_at > ?`,
		Now(), token, cutoff)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

// SessionExpiry returns the ISO time the session dies if it is not touched.
func (db *DB) SessionExpiry(token string) (sql.NullString, error) {
	var last string
	err := db.SQL.QueryRow(`select last_seen_at from sessions where token = ?`, token).Scan(&last)
	if isNoRows(err) {
		return sql.NullString{}, nil
	}
	if err != nil {
		return sql.NullString{}, err
	}
	t, err := ParseTime(last)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: FormatTime(t.Add(SessionIdle)), Valid: true}, nil
}

func (db *DB) DeleteSession(token string) error {
	_, err := db.SQL.Exec(`delete from sessions where token = ?`, token)
	return err
}

func (db *DB) pruneSessions() {
	cutoff := FormatTime(time.Now().Add(-SessionIdle))
	_, _ = db.SQL.Exec(`delete from sessions where last_seen_at <= ?`, cutoff)
}