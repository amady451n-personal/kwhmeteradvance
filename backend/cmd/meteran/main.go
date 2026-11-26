// Command meteran serves the API and SPA, and imports legacy databases.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"meteran/internal/httpapi"
	"meteran/internal/push"
	"meteran/internal/store"
	"meteran/internal/web"
)

// version is injected with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			os.Exit(runImport(os.Args[2:]))
		case "backup":
			os.Exit(runBackup(os.Args[2:]))
		}
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
	secret, err := loadSecret()
	if err != nil {
		log.Fatalf("secret: %v", err)
	}
	db.Secret = secret

	pushCtx, stopPush := context.WithCancel(context.Background())
	defer stopPush()

	// Env keys win; otherwise a pair is generated once and cached in /data.
	vapid, vapidErr := push.LoadOrCreate(
		os.Getenv("METERAN_VAPID_PUBLIC"),
		os.Getenv("METERAN_VAPID_PRIVATE"),
		env("METERAN_VAPID_FILE", "/data/vapid.json"),
	)
	vapidPublic := ""
	if vapidErr == nil {
		vapidPublic = vapid.Public
	}

	srv := &http.Server{
		Addr: addr,
		Handler: (&httpapi.Server{
			DB:           db,
			Version:      version,
			Secret:       db.Secret,
			VAPIDPublic:  vapidPublic,
			CookieSecure: os.Getenv("METERAN_COOKIE_SECURE") == "1",
			Web:          web.Handler(),
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if vapidErr != nil {
		log.Printf("push: notifikasi nonaktif: %v", vapidErr)
	} else {
		go (&push.Client{VAPID: vapid, TTL: 3600}).RunDaily(pushCtx, db, time.Minute)
		log.Printf("push: pengingat harian aktif")
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

// loadSecret gives the AES key for token codes. METERAN_SECRET wins; otherwise
// a random 32-byte key is generated once inside the data volume, so the key
// never sits in the repo and survives container restarts.
func loadSecret() ([]byte, error) {
	if v := os.Getenv("METERAN_SECRET"); v != "" {
		sum := sha256.Sum256([]byte(v))
		return sum[:], nil
	}
	path := env("METERAN_SECRET_FILE", "/data/secret")
	if raw, err := os.ReadFile(path); err == nil && len(raw) == 32 {
		return raw, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}