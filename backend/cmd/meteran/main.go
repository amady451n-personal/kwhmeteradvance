// Command meteran serves the API and SPA, and imports legacy databases.
package main

import (
	"crypto/sha256"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meteran/internal/httpapi"
	"meteran/internal/store"
	"meteran/internal/web"
)

// version is injected with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import" {
		os.Exit(runImport(os.Args[2:]))
	}

	dbPath := env("METERAN_DB", "/data/meteran.db")
	addr := env("METERAN_ADDR", ":8080")

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if secret := os.Getenv("METERAN_SECRET"); secret != "" {
		sum := sha256.Sum256([]byte(secret))
		db.Secret = sum[:]
	}

	srv := &http.Server{
		Addr: addr,
		Handler: (&httpapi.Server{
			DB:           db,
			Version:      version,
			Secret:       db.Secret,
			VAPIDPublic:  os.Getenv("METERAN_VAPID_PUBLIC"),
			VAPIDPrivate: os.Getenv("METERAN_VAPID_PRIVATE"),
			CookieSecure: os.Getenv("METERAN_COOKIE_SECURE") == "1",
			Web:          web.Handler(),
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("meteran %s listening on %s (db %s)", version, addr, dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}