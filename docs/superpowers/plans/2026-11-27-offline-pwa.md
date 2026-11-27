# Offline PWA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) atau superpowers:executing-plans untuk mengerjakan rencana ini task-per-task. Setiap langkah memakai checkbox (`- [ ]`).

**Goal:** Aplikasi bisa dibuka offline dengan PIN, menampilkan saldo dan lima catatan terakhir dari salinan perangkat, dan menerima token baru yang terkirim otomatis di background.

**Architecture:** Modul murni `offline.ts` menangani jejak PIN dan enkripsi berbasis DEK; `queue.ts` diperluas menjadi antrean reading + token dengan status `pending`/`failed`; store menghubungkan keduanya dan service worker meng-cache shell. Tidak ada perubahan backend atau kontrak.

**Tech Stack:** Vue 3 + TypeScript, Web Crypto (`PBKDF2`, `AES-GCM`), `localStorage`, `node:test` bawaan Node 24 (tanpa dependensi baru), service worker vanilla.

**Spec:** `docs/superpowers/specs/2026-11-27-offline-pwa-design.md`

## Global Constraints

- PIN 6 angka; jejak = PBKDF2-SHA256, salt 16 byte, 120.000 iterasi, 32 byte.
- DEK = 32 byte acak, disimpan di `localStorage('meteran.key.v1')`; tidak bergantung PIN.
- Kunci localStorage: `meteran.pin.v1`, `meteran.key.v1`, `meteran.snapshot.v1`, `meteran.queue.v2`.
- Service worker: cache `meteran-v2`; **jangan pernah** meng-cache respons `/api/*`.
- Tidak ada perubahan backend maupun `docs/api/openapi.yaml`. `TokenCreate.id` sudah opsional dan `CreateToken` backend idempoten per ID.
- Teks UI Bahasa Indonesia; tanggal lewat `domain/format.ts`; setiap file kode < 500 baris.
- Semua perintah npm/node dijalankan lewat `compose.dev.yml`, tidak di host.
- Test: `docker compose -f compose.dev.yml run --rm --no-deps web npm test` (script baru `node --test`).
- Build: `docker compose -f compose.dev.yml run --rm --no-deps web sh -c "npm run build"`.

## Review Focus

- Kode token tertulis sebagai teks polos di `localStorage` → Task 1 test `sealText/openText` dan Task 2 test `enqueueToken` memastikan payload tersandi.
- Antrean tidak bisa dikirim sebelum PIN diketik → Task 1 test `ensureDataKey` tanpa `verifyPin` dan Task 2 test `drainQueue` tanpa PIN.
- PIN diganti → antrean lama tetap terbaca → Task 1 test DEK stabil setelah `rememberPin` ulang.
- Token ditolak server dengan 4xx saat replay → Task 2 test `delivery='failed'`, retry berhenti, entri lain tetap diproses.
- PIN cocok tetapi salinan rusak/hilang → Task 1 test `loadSnapshot()` mengembalikan `null` tanpa membuang data.

---

### Task 1: Jejak PIN dan penyimpanan terenkripsi

**Files:**
- Create: `frontend/src/api/offline.ts`, `frontend/src/api/offline.test.ts`
- Modify: `frontend/package.json` (tambah script `test`), `frontend/tsconfig.json` (`allowImportingTsExtensions: true`, aman karena `noEmit: true`)

**Interfaces:**
- Produces:
  - `rememberPin(pin: string): Promise<void>`
  - `verifyPin(pin: string): Promise<boolean>`
  - `hasPinTrace(): boolean`
  - `ensureDataKey(): Promise<CryptoKey>`
  - `saveSnapshot(data: unknown): Promise<void>`
  - `loadSnapshot<T>(): Promise<T | null>`
  - `sealText(text: string): Promise<string>` / `openText(sealed: string): Promise<string>`
  - `clearOffline(): void` (hapus `meteran.pin.v1` + `meteran.snapshot.v1`, biarkan `meteran.key.v1`)

- [ ] **Step 1: Tambah script test dan izinkan ekstensi `.ts`**

Di `frontend/package.json`, di `scripts`, tambah `"test": "node --test"`. Di `frontend/tsconfig.json`, tambah `"allowImportingTsExtensions": true` (diperlukan karena `node --test` menuntut impor dengan ekstensi `.ts`, dan `noEmit` sudah true). Jalankan `npm test` di container; Expected: exit 0.

- [ ] **Step 2: Tulis test yang gagal**

`frontend/src/api/offline.test.ts`:

```ts
import test from 'node:test'
import assert from 'node:assert/strict'

class Memory {
  m = new Map<string, string>()
  getItem(k: string) { return this.m.get(k) ?? null }
  setItem(k: string, v: string) { this.m.set(k, v) }
  removeItem(k: string) { this.m.delete(k) }
}
Object.defineProperty(globalThis, 'localStorage', { value: new Memory(), configurable: true })

const o = await import('./offline.ts')

test('PIN trace verifies, never stores PIN', async () => {
  assert.equal(o.hasPinTrace(), false)
  await o.rememberPin('123456')
  assert.equal(o.hasPinTrace(), true)
  assert.equal(await o.verifyPin('123456'), true)
  assert.equal(await o.verifyPin('000000'), false)
  assert.ok(!JSON.stringify(localStorage).includes('123456'))
})

test('data key exists without PIN and survives PIN change', async () => {
  const first = await o.ensureDataKey()
  const raw = localStorage.getItem('meteran.key.v1')
  await o.rememberPin('654321')
  const second = await o.ensureDataKey()
  assert.equal(localStorage.getItem('meteran.key.v1'), raw)
  assert.equal(await o.verifyPin('654321'), true)
  assert.ok(first && second)
})

test('seal/open text and snapshot round trip', async () => {
  const sealed = await o.sealText('rahasia-token')
  assert.ok(!sealed.includes('rahasia-token'))
  assert.equal(await o.openText(sealed), 'rahasia-token')
  await o.saveSnapshot({ readings: [1, 2] })
  assert.deepEqual(await o.loadSnapshot(), { readings: [1, 2] })
})

test('broken snapshot returns null without deleting trace', async () => {
  localStorage.setItem('meteran.snapshot.v1', '{rusak')
  assert.equal(await o.loadSnapshot(), null)
  assert.equal(o.hasPinTrace(), true)
})
```

- [ ] **Step 3: Jalankan test, pastikan gagal**

Run: `docker compose -f compose.dev.yml run --rm --no-deps web npm test`
Expected: FAIL (modul `offline` belum ada).

- [ ] **Step 4: Implementasi `offline.ts`**

`rememberPin`: `crypto.getRandomValues(new Uint8Array(16))` sebagai salt, `crypto.subtle.importKey('raw', encoder.encode(pin), 'PBKDF2', false, ['deriveBits'])`, `deriveBits({name:'PBKDF2', hash:'SHA-256', salt, iterations:120000}, 256)`; simpan `{v:1, salt:base64, verifier:base64}` ke `meteran.pin.v1`. `verifyPin` hitung ulang dengan salt tersimpan lalu bandingkan memakai helper XOR konstan-waktu (Web Crypto tidak punya `timingSafeEqual`). `ensureDataKey`: bila `meteran.key.v1` ada, `importKey` JWK/raw; bila tidak, buat 32 byte acak, simpan base64, lalu `importKey('raw', ..., 'AES-GCM', false, ['encrypt','decrypt'])`. `sealText`/`openText`: IV 12 byte acak + `AES-GCM`, payload base64 dari `iv||ciphertext`. `saveSnapshot`/`loadSnapshot`: `sealText(JSON.stringify(data))` / `openText` + `JSON.parse`; kembalikan `null` pada error. `clearOffline` seperti kontrak di atas.

- [ ] **Step 5: Jalankan test, pastikan lolos**

Run: perintah Step 3. Expected: PASS 4 test.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/src/api/offline.ts frontend/src/api/offline.test.ts
git commit -m "feat(offline): encrypted PIN trace and device snapshot"
```

---

### Task 2: Antrean token terv2 dengan status gagal

**Files:**
- Modify: `frontend/src/api/queue.ts`, create `frontend/src/api/queue.test.ts`

**Interfaces:**
- Produces:
  - `QueuedReading` (ada) dan `QueuedToken { kind:'token'; id; input: TokenCreate; queuedAt; delivery:'pending'|'failed'; error?: string }`
  - `enqueueToken(id: string, input: TokenCreate): Promise<number>`
  - `queueSize(): number` (hitung metadata, sinkron)
  - `getPending(): Promise<QueuedToken[]>`
  - `removePending(id: string): Promise<number>`
  - `DrainOutcome = { ok: true } | { ok: false; retryable: boolean; message: string }`
  - `drainQueue(upsertReading, createToken): Promise<number>` — callbacks mengembalikan `Promise<DrainOutcome>`

- [ ] **Step 1: Tulis test yang gagal**

`frontend/src/api/queue.test.ts` dengan stub `localStorage` seperti Task 1, lalu `const q = await import('./queue.ts')`, lalu:

```ts
test('migrates v1 readings into v2 without loss', async () => {
  localStorage.setItem('meteran.queue.v1', JSON.stringify([
    { kind: 'reading', id: 'r1', input: { id: 'r1', reading: 90 }, queuedAt: 1 },
  ]))
  assert.equal(q.queueSize(), 1)
  assert.equal((await q.getPending()).length, 1)
})

test('queued token code is sealed', async () => {
  await q.enqueueToken('t1', { code: 'RAHASIA123', kwh: 100, cost: 150000 } as never)
  const raw = localStorage.getItem('meteran.queue.v2')!
  assert.ok(!raw.includes('RAHASIA123'))
  assert.equal((await q.getPending())[0].input.code, 'RAHASIA123')
})

test('drain skips network failure, dead-letters 4xx, continues next', async () => {
  await q.enqueueToken('t1', { code: 'A', kwh: 1, cost: 1 } as never)
  await q.enqueueToken('t2', { code: 'B', kwh: 2, cost: 2 } as never)
  const seen: string[] = []
  const remaining = await q.drainQueue(
    async () => ({ ok: true }),
    async (id) => {
      seen.push(id)
      if (id === 't1') return { ok: false, retryable: true, message: 'offline' }
      return { ok: false, retryable: false, message: 'data tidak valid' }
    },
  )
  assert.deepEqual(seen, ['t1', 't2'])
  assert.equal(remaining, 1)
  const [left] = await q.getPending()
  assert.equal(left.delivery, 'pending')
  assert.equal(left.id, 't1')
  await q.drainQueue(async () => ({ ok: true }), async (id) =>
    id === 't1' ? { ok: true } : ({ ok: true } as const),
  )
})
```

Tambah test terpisah untuk 4xx: setelah `drainQueue` dengan `{retryable:false}`, entri bersifat `delivery:'failed'`, `error` terisi, dan tidak ikut diproses pada drain berikutnya.

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: perintah Task 1 Step 3. Expected: FAIL (`enqueueToken`/`drainQueue` belum ada).

- [ ] **Step 3: Implementasi `queue.ts`**

Ganti kunci ke `meteran.queue.v2`. Simpan metadata tiap entri polos, tetapi `input` dienkripsi memakai `sealText`/`openText` dari `./offline`. Saat `load()`, bila `v2` belum ada dan `v1` ada, migrasi entri reading (buka dari plaintext lama), tulis ulang `v2`, hapus `v1`. `drainQueue`: proses berurutan; `ok:true` → hapus entri; `retryable:true` → break, kembalikan sisa; `retryable:false` → tandai `failed` dengan pesan, lanjut entri berikutnya. Impor `./offline.ts` memakai ekstensi `.ts` supaya `node --test` bisa memuatnya. `getPending` mengembalikan entri terbuka (termasuk `failed`). Semua fungsi tulis async; `queueSize()` tetap sinkron dengan membaca panjang array metadata.

- [ ] **Step 4: Jalankan test, pastikan lolos**

Run: perintah Task 1 Step 3. Expected: PASS semua test queue.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/queue.ts frontend/src/api/queue.test.ts
git commit -m "feat(offline): token queue with encrypted payload and failed state"
```

---

### Task 3: Sambung ke store

**Files:** Modify `frontend/src/stores/app.ts`

**Interfaces:**
- Consumes: semua fungsi Task 1 dan Task 2; `ApiError.isOffline` (sudah ada di `api/http.ts`), `client.createToken`.
- Produces: state baru `pendingTokens: Ref<QueuedToken[]>`, aksi `unlockOffline(pin): Promise<boolean>`, `discardPending(id): Promise<void>`, dan `pendingCount` (turunan dari `queuedCount`).

- [ ] **Step 1: Tulis test pendukung (pure helper)**

Tambah di `frontend/src/api/queue.test.ts` satu test: `drainQueue` memanggil callback reading lebih dulu daripada token saat keduanya tertunda. Run → PASS setelah Task 2; ini mengunci urutan replay yang dipakai Task 3.

- [ ] **Step 2: Ubah `init()`**

Bila `client.*` gagal dengan `ApiError.isOffline` dan `hasPinTrace()`, jangan hapus state; panggil `lock()` agar `LockView` menampilkan input PIN. Bila jejak tidak ada, pesan tetap "Perlu jaringan sekali untuk menyiapkan PIN." Bila sukses online: simpan salinan lewat `saveSnapshot({readings, tokens, settings})`.

- [ ] **Step 3: Tambah `unlockOffline(pin)`**

`verifyPin(pin)` → bila benar, `loadSnapshot()` isi `readings`/`tokens`/settings, `offline=true`, `ready=true`, `muat getPending()` ke `pendingTokens`, lalu jadwalkan satu `init()` senyap (jangan await) untuk mencoba tersambung kembali; 401 → `lock()`, gagal lain → tetap memakai salinan. Bila salinan `null`, biarkan array kosong dengan pesan di LockView.

- [ ] **Step 4: Ubah `addToken`**

Pada `ApiError.isOffline`, buat ID dengan `crypto.randomUUID()`, `enqueueToken(id, body)`, unshift token lokal bersifat sementara dari `body` ke `tokens`, set `offline=true`, perbarui `queuedCount`/`pendingTokens`, dan `notify('Token menunggu antrean.')`. Jangan lempar error. `markTokenUsed`/`cancelToken` tidak berubah (tetap online-only).

- [ ] **Step 5: Background drain**

Di `App.vue`, ganti `retry()` menjadi `void store.init().then(() => store.syncPending())`; tambah interval 30 detik yang memanggil `store.syncPending()` saja. `syncPending()` memanggil `drainQueue` dengan callback yang mengubah hasil `client.*` menjadi `DrainOutcome`: `isOffline` atau status ≥ 500 → `retryable:true`; status lain → `retryable:false` dengan `caught.message`; sukses → `{ok:true}`. Setelah drain, muat ulang `pendingTokens`, `queuedCount`, dan simpan salinan bila ada perubahan.

- [ ] **Step 6: Simpan salinan tiap mutasi sukses**

Di akhir `addReading`, `updateReading`, `deleteReading`, `addToken`, `patchToken`, `updateSettings`, panggil `saveSnapshot` dengan state terkini (fire-and-forget, `.catch(() => {})`). `logout()` memanggil `clearOffline()` dan mengosongkan `pendingTokens` dari tampilan tetapi tidak menghapus antrean.

- [ ] **Step 7: Verifikasi tipe + build**

Run: perintah build di Global Constraints. Expected: `vue-tsc --noEmit` lolos dan `✓ built`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/stores/app.ts frontend/src/App.vue
git commit -m "feat(offline): wire snapshot, PIN unlock and background token replay"
```

---

### Task 4: Tampilan tertunda dan pesan kunci

**Files:** Modify `frontend/src/views/LockView.vue`, `frontend/src/views/TokensView.vue`, `frontend/src/layouts/AppShell.vue` (label sumber bila perlu)

**Interfaces:**
- Consumes: `store.unlockOffline`, `store.pendingTokens`, `store.discardPending`, `store.offline`, `store.sourceLabel`.

- [ ] **Step 1: LockView saat offline**

Pada `catch` dari `store.login(pin)`: bila `store.offline` dan `hasPinTrace()`, jangan pakai pesan server; panggil `store.unlockOffline(pin)`. Bila `false`, tampilkan "PIN salah". Bila jejak tidak ada, "Perlu jaringan sekali untuk menyiapkan PIN." Bila PIN benar tetapi salinan kosong, "Belum ada salinan di perangkat. Hubungkan sekali untuk mengisi."

- [ ] **Step 2: Daftar token tertunda**

Di `TokensView.vue`, render `store.pendingTokens` sebagai kartu ringkas sebelum daftar token server, dengan chip "Tertunda" atau "Gagal: <error>" (tanpa menampilkan kode token), tanggal `formatDateTime`, kWh, dan rupiah. Tombol "Hapus" hanya untuk `delivery === 'failed'`, memanggil `store.discardPending(id)`.

- [ ] **Step 3: Label sumber**

Bila `store.offline`, `sourceLabel` menampilkan "Data perangkat" dan jumlah tertunda; hapus penanda saat `offline` kembali false.

- [ ] **Step 4: Verifikasi build + browser desktop/mobile**

Build seperti Task 3. Buka di browser 390 px dan 1280 px; pastikan tidak ada overflow horizontal dan chip terbaca.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/LockView.vue frontend/src/views/TokensView.vue frontend/src/layouts/AppShell.vue
git commit -m "feat(offline): show offline unlock messages and pending token cards"
```

---

### Task 5: Service worker cache v2

**Files:** Modify `frontend/public/sw.js`

**Interfaces:** tidak mengubah API aplikasi.

- [ ] **Step 1: Ubah strategi cache**

`CACHE = 'meteran-v2'`; `activate` hapus cache selain itu. Precache shell seperti sekarang. Handler `fetch`: kembalikan tanpa `respondWith` bila URL bukan same-origin atau path diawali `/api/`; untuk navigasi pakai network-first dengan fallback `/index.html` dari cache; untuk aset statis GET lain pakai cache-first, simpan respons `ok` ke cache. Pertahankan handler `push` dan `notificationclick` utuh.

- [ ] **Step 2: Verifikasi statis**

Run: `grep -n 'api/\|CACHE' frontend/public/sw.js` — Expected: ada `/api/` guard dan `meteran-v2`. Build ulang image arm64 bila perlu.

- [ ] **Step 3: Commit**

```bash
git add frontend/public/sw.js
git commit -m "feat(offline): cache app shell and assets without caching API"
```

---

### Task 6: Verifikasi ujung-ke-ujung dan dokumentasi

**Files:** Modify `docs/ONBOARDING.md`

- [ ] **Step 1: Siapkan server uji**

Jalankan image `meteranadvanced:latest` mode arm64 di `localhost:8096` dengan folder data kosong; set PIN `123456` lewat `PUT /api/session/pin`. Build SPA `VITE_API_MODE=server` sudah ada di image.

- [ ] **Step 2: Skenario offline**

Buka aplikasi di browser, masuk dengan PIN, pastikan saldo dan lima baris tampil. Putuskan jaringan lewat DevTools/CDP, reload, unlock dengan PIN → saldo + lima baris tetap tampil berlabel "Data perangkat". Isi token baru → kartu "Tertunda" muncul. Pulihkan jaringan → dalam 30 detik token terkirim, kartu hilang, tidak duplikat setelah reload.

- [ ] **Step 3: Skenario negatif**

PIN salah saat offline tetap terkunci; kode token tidak ditemukan sebagai substring di `localStorage`; token yang ditolak server (duplikat timestamp) berlabel "Gagal" dan tidak dikirim ulang; logout menghapus salinan tetapi antrean tetap ada.

- [ ] **Step 4: Catat hasil**

Tulis hasil verifikasi di `docs/ONBOARDING.md` bagian offline, termasuk tanggal WIB. Commit:

```bash
git add docs/ONBOARDING.md
git commit -m "docs: record offline PWA verification"
```

---

## Self-Review

- **Spec coverage:** jejak PIN + DEK (Task 1), salinan terenkripsi (1, 3), antrean token + status gagal (2), PIN wajib offline (3, 4), lima data terakhir (3), background reconnect (3, 5), penanda UI (4), service worker v2 (5), verifikasi (6).
- **Review Focus:** kelima input berisiko dalam bagian Review Focus masing-masing dipin test di Task 1/2 atau skenario Task 6.
- **Type consistency:** `DrainOutcome`, `QueuedToken`, `unlockOffline`, `syncPending`, `discardPending` memakai nama yang sama di semua task.
- **Proportion:** rencana lebih panjang dari spesifikasi karena memuat test yang bisa dijalankan; tidak ada body fungsi yang sudah ditentukan seluruhnya oleh signature.
