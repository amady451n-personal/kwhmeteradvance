# meteranadvanced Backend + Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a single arm64 Go binary + Docker image that serves the existing SPA and implements `docs/api/openapi.yaml`, deployable side-by-side with the legacy `kwhmeter basic` app.

**Architecture:** One Go module. Pure-Go SQLite (`modernc.org/sqlite`) behind a thin store layer; `net/http` `ServeMux` handlers map 1:1 to the contract; `embed.FS` serves `frontend/dist`. The frontend already runs against the same contract in `preview` mode, so `VITE_API_MODE=server` is the only frontend switch. Deploy is: build arm64 image here → save → scp → `docker load` → `docker compose up -d` → prune.

**Tech Stack:** Go 1.25 stdlib, `modernc.org/sqlite`, `golang.org/x/crypto/argon2`, `github.com/SherClockHolmes/webpush-go`, `github.com/go-pdf/fpdf`; Docker multi-stage (golang:1.25 builder → arm64v8/alpine); Docker Compose; bash scripts.

**Spec:** `docs/decisions/0003-deploy-plan.md`, `docs/api/openapi.yaml`, `PRD.md`

## Global Constraints

- API prefix `/api/*`, no version. JSON camelCase. Envelope `{data, meta?, links?}`; errors `{error:{code,message,details?}}`.
- Exceptions: `GET /api/homepage` returns the fixed FR-111 shape (no envelope); `GET /api/reports/summary.pdf` returns `application/pdf`.
- IDs are opaque client-supplied strings; `PUT` upserts must be idempotent.
- Soft violations return `409 confirmation_required` with `details[].code`; retry carries `confirm: [...]`. Hard violations return `422`.
- Auth: httpOnly cookie `meteran_session`, idle timeout 30 min, Argon2id PIN, 5 attempts per 15 min. Public: `/api/homepage`, `/api/health`, `GET|POST /api/session`.
- Server value is canonical; client-local values are marked temporary.
- SQLite, PWA, Web Push, Docker Compose are fixed by PRD. Target arm64, ≤512 MB free RAM, `CGO_ENABLED=0`.
- `deleteReading` on a reading linked to a not-locked token reverts that token to `unused`.
- Import runs only when the new DB is empty (FR-100). `import` reads legacy `readings(id, meter_name, reading, timestamp, notes, recharge_amount, cost)`; legacy has no tokens table.
- Tooling: all Node/npm run in Docker, no host installs. Terse output; never create `CLAUDE.md` (repo uses `AGENTS.md`).

## Review Focus

- Concurrent `PUT` of the same reading id must not create two rows (upsert idempotency).
- Token "next reading lower than the linked reading" auto-lock must fire exactly once and never unlock.
- `confirm` retry replay must not double-apply a topup reading.
- Legacy timestamps are WIB wall-clock with no zone; import must not shift them.
- Auth rate-limit state must survive a process restart (persisted, not in-memory only).

---

### Task 1: Module, SQLite, migrations, health

**Files:**
- Create: `backend/go.mod`, `backend/cmd/meteran/main.go`, `backend/internal/store/db.go`, `backend/internal/store/schema.sql`
- Test: `backend/internal/store/db_test.go`

**Interfaces:**
- Produces: `store.Open(path string) (*store.DB, error)`; `(*store.DB).Close() error`; `(*store.DB).Migrate() error`; `(*store.DB).SQL *sql.DB`.

- [ ] **Step 1: Write the failing test**

```go
func TestOpenMigratesAndHealthShape(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if err := db.Migrate(); err != nil { t.Fatal(err) }
	var n int
	if err := db.SQL.QueryRow(`select count(*) from readings`).Scan(&n); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/store/ -run TestOpen -v`
Expected: FAIL (package/function undefined)

- [ ] **Step 3: Implement `store.Open`, `Migrate`, `schema.sql`**

Driver: `modernc.org/sqlite`, DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)`. `schema.sql` = `readings`, `tokens`, `settings`, `auth`, `push_subscriptions`, `import_state`. `Migrate()` runs `schema.sql` statements idempotently inside a tx.

- [ ] **Step 4: Add `GET /api/health`**

Handler returns `{"data":{"status":"ok","version":...}}`. Wire `main.go` to `store.Open` + `Migrate` + `ListenAndServe`.

- [ ] **Step 5: Run tests + build**

Run: `cd backend && go test ./... && go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "feat(backend): module, sqlite store, migrations, health"
```

---

### Task 2: Envelope, errors, router, middleware

**Files:**
- Create: `backend/internal/httpapi/envelope.go`, `backend/internal/httpapi/errors.go`, `backend/internal/httpapi/router.go`, `backend/internal/httpapi/middleware.go`
- Test: `backend/internal/httpapi/envelope_test.go`

**Interfaces:**
- Produces: `httpapi.Data(w, status, v any)`; `httpapi.Fail(w, status int, code, msg string, details ...Detail)`; `Detail{Code string; Message string}`; `httpapi.NewRouter(deps Deps) http.Handler`.
- Consumes: Task 1 `*store.DB`.

- [ ] **Step 1: Write the failing test**

```go
func TestFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	httpapi.Fail(rec, 409, "confirmation_required", "Perlu konfirmasi.", httpapi.Detail{Code: "increased_without_topup", Message: "x"})
	var body struct{ Error struct{ Code string; Details []struct{ Code string } } }
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "confirmation_required" || body.Error.Details[0].Code != "increased_without_topup" { t.Fatal(rec.Body.String()) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/httpapi/ -run TestFail -v`
Expected: FAIL

- [ ] **Step 3: Implement envelope + `Fail` + `Data`**

`Data` wraps in `{"data": v}`. `Fail` writes `{"error":{"code","message","details?"}}`; `details` omitted when empty.

- [ ] **Step 4: Implement `NewRouter` (all routes stubbed 501) + middleware**

Middleware: recover→500, request id, JSON content type, session-load, auth gate (Task 8 fills the checker; until then, gate is a no-op behind an interface `SessionChecker`).

- [ ] **Step 5: Run tests**

Run: `cd backend && go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(backend): envelope, errors, router, middleware"
```

---

### Task 3: Readings CRUD + confirmations

**Files:**
- Create: `backend/internal/store/readings.go`, `backend/internal/httpapi/readings.go`
- Test: `backend/internal/store/readings_test.go`, `backend/internal/httpapi/readings_test.go`

**Interfaces:**
- Produces: `store.Reading{ID, MeterName string; Reading float64; Timestamp string; Notes *string; RechargeAmount *float64; Cost *float64; TokenID *string; Source string; Locked bool; OldID *string; CreatedAt, UpdatedAt string}`; `(*store.DB).ListReadings(from, to, cursor string, limit int) ([]store.Reading, Meta, error)`; `(*store.DB).GetReading(id string) (store.Reading, error)`; `(*store.DB).UpsertReading(r store.Reading, confirm []string) (store.Reading, error)`; `(*store.DB).DeleteReading(id string) error`.
- Errors: `store.ErrNotFound`, `store.ErrConfirmation{Details []Detail}`, `store.ErrReadingLocked`.

- [ ] **Step 1: Write the failing tests**

```go
func TestUpsertReadingIsIdempotent(t *testing.T) { /* open :memory:, migrate; Upsert same id twice; assert count==1 and updatedAt changes */ }
func TestUpsertReadingDuplicateTimestampNeedsConfirm(t *testing.T) { /* two rows same ts, different ids; second returns ErrConfirmation code duplicate_timestamp; with confirm passes */ }
func TestUpsertReadingIncreaseWithoutTopupNeedsConfirm(t *testing.T) { /* prev reading 100; new 120 no recharge; ErrConfirmation increased_without_topup */ }
func TestDeleteReadingRevertsLinkedUsedToken(t *testing.T) { /* token used + linkedReadingId; DeleteReading → token.status unused, linkedReadingId null */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store/ -run Reading -v`
Expected: FAIL

- [ ] **Step 3: Implement store methods**

`UpsertReading` in one tx: `INSERT ... ON CONFLICT(id) DO UPDATE`. Confirmation checks compare against rows with `id <> $id`. Delete cascades token revert (set `status='unused'`, clear `linked_reading_id`) for tokens whose `linked_reading_id = $id` and `status <> 'locked'`; locked linked reading → `ErrReadingLocked`.

- [ ] **Step 4: Implement HTTP handlers**

`GET /api/readings`, `GET /api/readings/{id}`, `PUT /api/readings/{id}`, `DELETE /api/readings/{id}`. Map `ErrConfirmation`→409 `confirmation_required`, `ErrReadingLocked`→409 `reading_locked`, `ErrNotFound`→404. `PUT` reads `confirm` from body.

- [ ] **Step 5: Run tests**

Run: `cd backend && go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/readings.go backend/internal/store/readings_test.go backend/internal/httpapi/readings.go backend/internal/httpapi/readings_test.go
git commit -m "feat(backend): readings CRUD with soft confirmations"
```

---

### Task 4: Tokens CRUD

**Files:**
- Create: `backend/internal/store/tokens.go`, `backend/internal/httpapi/tokens.go`
- Test: `backend/internal/store/tokens_test.go`

**Interfaces:**
- Produces: `store.Token{ID string; Kwh, Cost float64; PurchasedAt string; Type string; Denomination *float64; DenominationMismatch bool; Status string; Source string; Notes *string; HasCode bool; LinkedReadingID *string; CanDelete bool; DeleteBlockedReason *string; LockedAt *string; RefillsSinceLock *int; CreatedAt, UpdatedAt string}`.
- Methods: `ListTokens(status, cursor string, limit int)`, `GetToken(id)`, `CreateToken(TokenCreate)`, `PatchToken(id, TokenPatch)`, `DeleteToken(id)`, `RevealTokenCode(id) string`.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateTokenDefaultsUnused(t *testing.T) {}
func TestPatchUsedTokenRejected(t *testing.T) { /* status used → ErrValidation */ }
func TestDeleteTokenBlockedUnlessCanDelete(t *testing.T) { /* canDelete false → ErrConflict token_not_deletable */ }
func TestTokenCodeEncryptedAtRest(t *testing.T) { /* reveal returns original; raw column != plaintext */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store/ -run Token -v`
Expected: FAIL

- [ ] **Step 3: Implement store**

Code stored AES-GCM encrypted with a key from `METERAN_SECRET` (Task 6 settings); `reveal` decrypts. `canDelete` = `status='locked' && refills_since_lock >= 50`.

- [ ] **Step 4: Implement HTTP handlers** — `GET/POST /api/tokens`, `GET/PATCH/DELETE /api/tokens/{id}`, `POST /api/tokens/{id}/reveal`.

- [ ] **Step 5: Run tests → PASS. Step 6: Commit** `feat(backend): tokens CRUD`

---

### Task 5: Token lifecycle (mark-used, cancel, auto-lock)

**Files:**
- Modify: `backend/internal/store/tokens.go`, `backend/internal/httpapi/tokens.go`
- Test: `backend/internal/store/tokens_test.go`

**Interfaces:**
- Produces: `MarkTokenUsed(id string, reading float64, at string, confirm []string) (Token, Reading, error)`; `CancelToken(id string) (Token, cancelledReadingID *string, err error)`.
- `CancelToken` on `locked` → `ErrTokenLocked` (409 `token_locked`); on `used` → `unused`, delete linked reading.

- [ ] **Step 1: Write the failing tests**

```go
func TestMarkTokenUsedCreatesLinkedReading(t *testing.T) {}
func TestCancelUsedTokenRevertsAndRemovesLinkedReading(t *testing.T) {}
func TestCancelLockedTokenFails(t *testing.T) { /* ErrTokenLocked */ }
func TestLowerReadingLocksUsedTokenOnce(t *testing.T) { /* upsert reading with reading < linked.reading → token locked; second lower reading changes nothing */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store/ -run 'TokenUsed|Cancel|Lock' -v`
Expected: FAIL

- [ ] **Step 3: Implement**

`MarkTokenUsed`: require `unused`; create reading (`rechargeAmount=token.kwh`, `cost=token.cost`, `tokenId`, `source='token'`), set token `used` + `linked_reading_id`. Auto-lock rule runs inside `UpsertReading` tx: for a token `used` whose linked reading's `reading > new.reading`, set `locked`, `locked_at`, clear nothing else. `CancelToken` mirrors preview adapter semantics.

- [ ] **Step 4: Implement handlers** — `POST /api/tokens/{id}/mark-used`, `POST /api/tokens/{id}/cancel`.

- [ ] **Step 5: Run tests → PASS. Step 6: Commit** `feat(backend): token lifecycle + auto-lock`

---

### Task 6: Settings

**Files:**
- Create: `backend/internal/store/settings.go`, `backend/internal/httpapi/settings.go`
- Test: `backend/internal/store/settings_test.go`

**Interfaces:**
- Produces: `store.Settings{Buffer int; AvgWindow int; TargetHour int; ReminderHour int; Denominations []float64; Tariff Tariff; PinSet bool; PushEnabled bool; ImportCompletedAt *string; UpdatedAt string}`; `GetSettings()`, `PatchSettings(SettingsPatch)`.

- [ ] **Step 1: Failing test** — patch buffer→persisted; denominations stored as JSON; defaults inserted on first `Migrate`.

- [ ] **Step 2: Run to fail** — `go test ./internal/store/ -run Settings -v`

- [ ] **Step 3: Implement store + defaults** (`buffer=10, avgWindow=7, targetHour=6, reminderHour=8, denominations=DEFAULT_DENOMINATIONS`).

- [ ] **Step 4: Handlers** — `GET /api/settings`, `PATCH /api/settings`.

- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): settings`

---

### Task 7: Domain port — summary, usage, homepage

**Files:**
- Create: `backend/internal/domain/energy.go`, `backend/internal/httpapi/summary.go`
- Test: `backend/internal/domain/energy_test.go`

**Interfaces:**
- Produces: `domain.Summarize(in SummarizeInput) Summary` with fields `Latest *Reading; TokenStock, WeekKwh, WeekCost, MonthKwh, MonthCost float64; AvgKwh float64; AvgWindow int; Buffer float64; NeedKwh float64; DepletionMs *int64; TargetMs int64; Recommendation *Recommendation; TariffComparison; Alerts []Alert`; `domain.UsageBuckets(readings []Reading, in BucketInput) []Bucket`.

- [ ] **Step 1: Failing test** — port the numeric expectations from `frontend/src/domain/energy.ts` (same inputs → same `weekKwh/avgKwh/needKwh`) using a shared fixture `frontend/src/data/fixture.json`.

- [ ] **Step 2: Run to fail** — `cd backend && go test ./internal/domain/ -v`

- [ ] **Step 3: Implement the port** (1:1 with the TS functions; keep function names).

- [ ] **Step 4: Handlers** — `GET /api/summary?window=`, `GET /api/usage?group=&from=&to=`, `GET /api/homepage` (fixed shape, no envelope).

- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): domain port + summary/usage/homepage`

---

### Task 8: Auth (PIN, session, rate limit)

**Files:**
- Create: `backend/internal/auth/pin.go`, `backend/internal/auth/session.go`, `backend/internal/httpapi/session.go`
- Modify: `backend/internal/httpapi/middleware.go` (wire `SessionChecker`)
- Test: `backend/internal/auth/pin_test.go`, `backend/internal/httpapi/session_test.go`

**Interfaces:**
- Produces: `auth.HashPin(pin string) (string, error)`, `auth.VerifyPin(hash, pin string) bool` (Argon2id, params in `auth.Params`); `auth.Limiter` persisted in `auth_attempts` table, `Allow(key string, now time.Time) bool`.

- [ ] **Step 1: Failing tests**

```go
func TestPinHashVerifyRoundTrip(t *testing.T) {}
func TestLimiterBlocksAfter5In15Min(t *testing.T) {}
func TestLimiterSurvivesReopen(t *testing.T) { /* state in DB, reopen DB → still blocked */ }
func TestProtectedRouteRejectsWithoutSession(t *testing.T) {}
```

- [ ] **Step 2: Run to fail** — `go test ./internal/auth/ ./internal/httpapi/ -run 'Pin|Limit|Session' -v`

- [ ] **Step 3: Implement auth package.**

- [ ] **Step 4: Handlers** — `GET /api/session`, `POST /api/session` (login), `DELETE /api/session` (logout), `PUT /api/session/pin` (set/change; first-time allowed without session). Cookie `meteran_session`, httpOnly, `Secure` when `METERAN_COOKIE_SECURE=1`, idle 30 min (touch on each request).

- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): auth pin, session, rate limit`

---

### Task 9: Reports PDF

**Files:**
- Create: `backend/internal/report/pdf.go`, `backend/internal/httpapi/reports.go`
- Test: `backend/internal/report/pdf_test.go`

**Interfaces:**
- Produces: `report.SummaryPDF(sum domain.Summary, rows []store.Reading, from, to string) ([]byte, error)`.

- [ ] **Step 1: Failing test** — output starts with `%PDF-` and contains the from/to range string.
- [ ] **Step 2: Run to fail** — `go test ./internal/report/ -v`
- [ ] **Step 3: Implement with `go-pdf/fpdf`** (no cgo).
- [ ] **Step 4: Handler** — `GET /api/reports/summary.pdf?from=&to=` → `application/pdf`.
- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): summary pdf report`

---

### Task 10: Web Push

**Files:**
- Create: `backend/internal/httpapi/push.go`, `backend/internal/store/push.go`
- Test: `backend/internal/store/push_test.go`

**Interfaces:**
- Produces: `GetPushKey() (string, error)` (VAPID public), `SubscribePush(PushSubscriptionInput) error`, `UnsubscribePush(endpoint string) error`.

- [ ] **Step 1: Failing test** — subscribe then unsubscribe removes the row; invalid key shape → `ErrValidation`.
- [ ] **Step 2: Run to fail** — `go test ./internal/store/ -run Push -v`
- [ ] **Step 3: Implement store** (`endpoint`, `p256dh`, `auth`, `created_at`); VAPID keys from `METERAN_VAPID_PRIVATE`/`_PUBLIC`.
- [ ] **Step 4: Handlers** — `GET /api/push/key`, `POST /api/push/subscribe`, `POST /api/push/unsubscribe`.
- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): web push subscriptions`

---

### Task 11: Legacy import

**Files:**
- Create: `backend/internal/importer/legacy.go`, `backend/cmd/meteran/import.go`
- Test: `backend/internal/importer/legacy_test.go`

**Interfaces:**
- Produces: `importer.Run(dst *store.DB, legacyPath string) (count int, err error)`. Reads legacy `readings(id, meter_name, reading, timestamp, notes, recharge_amount, cost)`; writes `store.Reading{ID:"legacy-<id>", OldID:<id>, Source:"import", TokenID:nil}` with the timestamp parsed as WIB (`+07:00`) preserving wall-clock.

- [ ] **Step 1: Failing test** — build a temp legacy DB with `sqlite3`-style schema, run import, assert new `readings` count equals legacy and the timestamp string equals the legacy wall-clock with `+07:00`.
- [ ] **Step 2: Run to fail** — `go test ./internal/importer/ -v`
- [ ] **Step 3: Implement** (guard: refuse when `readings` table non-empty → `ErrImportNotEmpty`).
- [ ] **Step 4: CLI** — `meteran import --from /path/meters.db`; multiple `--from` allowed (meter variants), all mapped to `meterName`.
- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): legacy meters.db import`

---

### Task 12: Embed SPA

**Files:**
- Create: `backend/internal/web/embed.go`, `frontend/vite.config.ts` (build outDir stays `dist`)
- Test: `backend/internal/web/embed_test.go`

**Interfaces:**
- Produces: `web.Handler() http.Handler` serving `frontend/dist` with SPA fallback to `index.html`; `web.CopyDist(dst string) error` used at image build to place `dist` under `backend/internal/web/dist`.

- [ ] **Step 1: Failing test** — `GET /` returns `index.html`; `GET /api/nope` returns 404 JSON (not index); `GET /riwayat` returns `index.html`.
- [ ] **Step 2: Run to fail** — `go test ./internal/web/ -v`
- [ ] **Step 3: Implement** `//go:embed dist`.
- [ ] **Step 4: Wire in `NewRouter`** so non-`/api` paths hit `web.Handler()`.
- [ ] **Step 5: PASS. Step 6: Commit** `feat(backend): embed and serve SPA`

---

### Task 13: Image, compose, env

**Files:**
- Create: `backend/Dockerfile`, `compose.yml`, `.env`, `.dockerignore`
- Test: manual (`docker buildx build --platform linux/arm64`)

**Interfaces:**
- Produces: image `meteranadvanced:latest`; compose service `meteranadvanced` on `${METERAN_PORT}:8080`, volume `meteranadvanced-data:/data`, `pull_policy: never`, healthcheck `curl -sf localhost:8080/api/health`.

- [ ] **Step 1: Write `Dockerfile`** — stage 1 `node:24-alpine` runs `npm ci && npm run build` in `frontend`; stage 2 `golang:1.25` copies dist into `internal/web/dist`, `CGO_ENABLED=0 GOARCH=arm64 go build`; stage 3 `arm64v8/alpine` with the binary, `METERAN_DB=/data/meteran.db`, `ENTRYPOINT ["/meteran"]`.
- [ ] **Step 2: `.env`** (tracked) — `METERAN_PORT=8080`.
- [ ] **Step 3: `compose.yml`** — service as above, `restart: unless-stopped`, `env_file: .env`.
- [ ] **Step 4: Verify image builds for arm64 here**

Run: `docker buildx build --platform linux/arm64 -t meteranadvanced:latest --load -f backend/Dockerfile .`
Expected: success; `docker image inspect meteranadvanced:latest --format '{{.Architecture}}'` → `arm64`

- [ ] **Step 5: Commit** `feat: arm64 image, compose, env`

---

### Task 14: Release + deploy script

**Files:**
- Create: `scripts/release.sh`, `scripts/backup.sh`, `scripts/release.env.example`
- Test: manual (menu walkthrough; dry-run flags)

**Interfaces:**
- One interactive bash script, **no CLI params**, config from `scripts/release.env` (`STB_HOST`, `STB_USER`, `STB_DIR`, `IMAGE=meteranadvanced:latest`).

- [ ] **Step 1: `release.sh` menu**

```
1) build & package   (buildx arm64 + docker save | gzip → dist/meteranadvanced-image.tar.gz)
2) git push          (here)
3) scp image to STB
4) git pull          (on STB)
5) deploy            (docker load → docker compose up -d → docker image prune -f → rm -f /tmp/meteranadvanced-image.tar.gz)
6) reset & import    (docker compose down -v → prompt path to basic meters.db → docker run --rm -v <db>:/import/meters.db:ro -v meteranadvanced-data:/data meteranadvanced:latest import --from /import/meters.db)
7) status / health
0) exit
```

- [ ] **Step 2: Confirmations** — destructive steps (5, 6) prompt `yes`; option 6 warns it erases the meteranadvanced volume.
- [ ] **Step 3: `backup.sh`** — `sqlite3 /data/meteran.db ".backup '/backups/meteran-$(date +%F-%H%M).db'"` via `docker run`, keep last N.
- [ ] **Step 4: Dry-run check** — `bash -n scripts/release.sh` and a `--dry-run` env that echoes commands.

- [ ] **Step 5: Commit** `feat: release and deploy scripts`

---

### Task 15: Frontend leftovers for server mode

**Files:**
- Modify: `frontend/src/styles/main.css` (split under 500 lines)
- Verify: `frontend/public/sw.js`, `frontend/src/api/queue.ts`

- [ ] **Step 1: Split `main.css`** into `main.css` + `views.css`/`components.css` imported from `main.ts`; each < 500 lines (NFR-11).
- [ ] **Step 2: Verify offline queue** against the real backend on STB (kill network, add reading, restore, confirm replay) — record result in `docs/ONBOARDING.md`.
- [ ] **Step 3: Build** — `docker compose -f compose.dev.yml run --rm --no-deps web sh -c "npm run build"` → PASS.
- [ ] **Step 4: Commit** `refactor(frontend): split styles; verify offline queue`

---

### Task 16: Contract conformance

**Files:**
- Create: `backend/internal/httpapi/conformance_test.go`, `scripts/check-contract.sh`

**Interfaces:**
- Test validates every response for a scripted happy path parses against `docs/api/openapi.yaml` using generated Go types (`oapi-codegen` → `backend/internal/httpapi/oapi/types.gen.go`).

- [ ] **Step 1: Add `oapi-codegen`** config generating types from the contract into `backend/internal/httpapi/oapi/`.
- [ ] **Step 2: Write the test** — spin the router on `httptest`, walk the happy path (session → settings → token → mark-used → reading → summary → usage → report → push), unmarshal each body into the generated types; fail on mismatch.
- [ ] **Step 3: Run to fail then pass** — `cd backend && go test ./internal/httpapi/ -run Conformance -v`.
- [ ] **Step 4: Commit** `test(backend): openapi conformance`

---

## Self-Review

- **Spec coverage:** health(1) envelope/errors(2) readings+confirmations(3) tokens(4) lifecycle/auto-lock(5) settings(6) summary/usage/homepage(7) auth(8) report PDF(9) push(10) import(11) SPA(12) image/compose/env(13) scripts(14) frontend leftovers(15) conformance(16). Every FR-touching endpoint in `openapi.yaml` maps to a task.
- **Type consistency:** `store.Reading`/`store.Token` defined once (Tasks 3/4) and consumed by 5/7/9/11. `Detail` defined in Task 2, reused by Task 3 store errors.
- **Review Focus tests owned:** idempotent upsert → Task 3; auto-lock once → Task 5; confirm replay → Task 3 (`Upsert` with `confirm`); WIB timestamps → Task 11; persistent limiter → Task 8.

## Open items (yours, not tasks)

- TLS/reverse proxy on the STB (your container).
- `scripts/release.env` values (`STB_HOST`, `STB_USER`, `STB_DIR`).
- Default `METERAN_PORT`.
