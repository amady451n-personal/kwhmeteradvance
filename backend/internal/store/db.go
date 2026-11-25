package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DB is the single SQLite handle shared by all stores.
type DB struct {
	SQL *sql.DB
	// Secret is the AES-256 key for token codes (METERAN_SECRET).
	Secret []byte
}

// Open opens the database and applies pragmas. WAL keeps readers from blocking
// the single writer on the STB.
func Open(path string) (*DB, error) {
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + path +
			"?_pragma=journal_mode(WAL)" +
			"&_pragma=foreign_keys(1)" +
			"&_pragma=busy_timeout(5000)" +
			"&_pragma=synchronous(NORMAL)"
	}
	dbc, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// An in-memory database lives per connection; one connection keeps state.
	if path == ":memory:" {
		dbc.SetMaxOpenConns(1)
	} else {
		dbc.SetMaxOpenConns(4)
	}
	if err := dbc.Ping(); err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &DB{SQL: dbc}, nil
}

func (db *DB) Close() error { return db.SQL.Close() }

// Migrate applies schema.sql. Every statement is IF NOT EXISTS, so running it
// on each boot is the migration strategy for v1.
func (db *DB) Migrate() error {
	if _, err := db.SQL.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func isNoRows(err error) bool { return err == sql.ErrNoRows }

func queryList(ss []string) string {
	return strings.Join(ss, ",")
}