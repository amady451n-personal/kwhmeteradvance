package main

import (
	"fmt"
	"log"
	"os"

	"meteran/internal/store"
)

// runBackup writes a consistent snapshot with VACUUM INTO (WAL-safe, no extra
// sqlite binary in the image).
// Usage: meteran backup /backups/meteran.db
func runBackup(args []string) int {
	if len(args) != 1 {
		fmt.Println("usage: meteran backup /path/out.db")
		return 2
	}
	dbPath := env("METERAN_DB", "/data/meteran.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Printf("open database: %v", err)
		return 1
	}
	defer db.Close()
	if _, err := db.SQL.Exec(`vacuum into ?`, args[0]); err != nil {
		log.Printf("backup: %v", err)
		return 1
	}
	fi, err := os.Stat(args[0])
	if err != nil {
		log.Printf("backup: %v", err)
		return 1
	}
	fmt.Printf("%s: %d bytes\n", args[0], fi.Size())
	return 0
}