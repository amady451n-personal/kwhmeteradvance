package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"meteran/internal/importer"
	"meteran/internal/store"
)

// runImport maps one or more legacy meters.db files into the new database.
// Usage: meteran import [--from] /path/a.db [/path/b.db ...]
func runImport(args []string) int {
	var paths []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		paths = append(paths, a)
	}
	if len(paths) == 0 {
		fmt.Println("usage: meteran import [--from] /path/meters.db ...")
		return 2
	}
	dbPath := env("METERAN_DB", "/data/meteran.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Printf("open database: %v", err)
		return 1
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		log.Printf("migrate: %v", err)
		return 1
	}

	for _, path := range paths {
		// The legacy file is usually mounted read-only, and SQLite may want to
		// recover a WAL; work on a writable copy instead.
		tmp, err := os.CreateTemp("", "legacy-*.db")
		if err != nil {
			log.Printf("temp: %v", err)
			return 1
		}
		src, err := os.Open(path)
		if err == nil {
			_, err = io.Copy(tmp, src)
			src.Close()
		}
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
			log.Printf("%s: %v", path, err)
			return 1
		}
		preview, err := importer.Scan(tmp.Name())
		os.Remove(tmp.Name())
		if err != nil {
			log.Printf("%s: %v", path, err)
			return 1
		}
		stats, err := importer.Commit(db, preview)
		if err != nil {
			log.Printf("%s: %v", path, err)
			return 1
		}
		fmt.Printf("%s: %d/%d baris diimpor (%d valid, %d perlu konfirmasi, %d ditolak), %.2f kWh, Rp%d\n",
			path, stats.Rows, preview.Rows, len(preview.Valid), len(preview.NeedsConfirm),
			len(preview.Rejected), stats.TopupKwh, stats.TopupCost)
	}
	if err := db.MarkImported(); err != nil {
		log.Printf("mark imported: %v", err)
		return 1
	}
	return 0
}