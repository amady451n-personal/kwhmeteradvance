# Onboarding: meteran

Terakhir diverifikasi: 2026-11-24 09:48 WIB.

## Ringkasan

PWA pencatat sisa kWh meteran PLN prabayar untuk satu rumah. Fokus v1: catat meter cepat, lihat pemakaian harian, prediksi kapan sisa menyentuh buffer, dan rekomendasi satu token sampai tanggal 1 berikutnya.

Kondisi repo saat ini: frontend + backend + API + SQLite + PWA/service worker + antrean offline + PIN + impor + push sudah ada. Mode pratinjau memakai fixture; mode server memakai backend Go.

## Stack

| Lapisan | Teknologi | Versi (pinned) |
|---|---|---|
| Bahasa | TypeScript | 5.9.3 |
| Framework | Vue | 3.5.46 |
| UI | Vuetify (Material 3) | 4.3.4 |
| Build | Vite + `vite-plugin-vuetify` | 8.3.8 / 2.1.3 |
| State | Pinia | 4.1.0 |
| Routing | Vue Router | 5.5.1 |
| Font/ikon | Plus Jakarta Sans Variable, MDI | 5.3.0 / 7.4.47 |
| Tipe | `vue-tsc` | 3.3.14 |
| Backend | Go 1.26 (stdlib mux) + `modernc.org/sqlite` | 1.26 |

Backend: `backend/` (lihat `docs/superpowers/plans/2026-11-25-meteranadvanced-backend-deploy.md`). Satu binary statis arm64, `CGO_ENABLED=0`, SPA di-embed lewat `//go:embed`.

Keputusan stack: `docs/decisions/0001-stack-frontend.md`. Rencana backend: `memory.md` bagian 0a.

## Arsitektur

```
docs/api/openapi.yaml  ->  npm run api:types  ->  src/api/schema.d.ts
        |
        v
src/api/client.ts (fetch, envelope, ApiError)   src/api/preview.ts (localStorage + domain/energy)
        |                                                 |
        +---------->  src/api/index.ts (VITE_API_MODE) --+
                              |
                              v
                     stores/app.ts (Pinia)  --cache lokal + antrean offline--> browser
                              |  summary server, fallback summarize() lokal
                              v
                     domain/energy.ts (fungsi murni) + domain/format.ts (WIB, id-ID)
                              |
                              v
                     views/*.vue  <-  layouts/AppShell.vue (drawer/rail/bottom nav)
```

Kontrak adalah sumber kebenaran tipe. Mode default `preview`: klien palsu menjawab bentuk payload yang sama dengan server, menyimpan ke `localStorage('meteran.preview.v2')` dan seed dari `data/fixture.json`. Set `VITE_API_MODE=server` untuk memakai `fetch` ke `/api/*` (image produksi memakai mode ini).

## Peta direktori

| Path | Isi |
|---|---|
| `docs/api/openapi.yaml` | Kontrak OpenAPI 3.1 (auth sesi, readings, tokens, settings, usage, summary, impor, PDF, push, homepage) |
| `frontend/src/api/` | `schema.d.ts` hasil generate, `types.ts`, `http.ts`, `client.ts` (server), `preview.ts` (pratinjau), `mappers.ts`, `queue.ts` |
| `frontend/src/domain/` | Logika energi murni: interval, alokasi harian, harga efektif, prediksi, rekomendasi, agregasi hari/minggu/bulan, tarif, urgensi, format WIB/Rupiah |
| `frontend/src/stores/app.ts` | Satu store Pinia: data server, fallback lokal, antrean offline, sesi, aksi async |
| `frontend/src/views/` | Beranda, Riwayat, Token, Laporan, Pengaturan, Lock (PIN) |
| `frontend/src/components/` | Dialog input/ubah catatan, dialog token, baris riwayat, runway rail, grafik batang |
| `frontend/src/composables/` | `useFocusReturn`: kembalikan fokus ke pemicu setelah dialog ditutup (Vuetify hanya melakukannya bila `activator` dipakai) |
| `frontend/src/layouts/AppShell.vue` | Shell aplikasi: drawer >=960 px, rail >=600 px, bottom nav <600 px |
| `frontend/src/plugins/vuetify.ts` | Token warna light/dark dan default komponen |
| `frontend/src/styles/` | Global CSS dipecah per area (`main`, `dashboard`, `views`, `dialogs`), tiap file < 500 baris (NFR-11) |
| `frontend/src/api/schema.d.ts` | Tipe hasil `openapi-typescript`; jangan diedit tangan |
| `frontend/public/` | `manifest.webmanifest`, `sw.js`, `icon.svg` (PWA tanpa plugin) |
| `scripts/generate-fixture.py` | Regenerasi fixture dari SQLite aplikasi basic |
| `scripts/dev-images.sh` | Kelola daftar image yang di-pull proyek |
| `infra/images.base.txt` | Image shared, jangan dihapus |
| `infra/images.pulled.txt` | Image pull khusus proyek ini |
| `compose.dev.yml` | Dev server Node di container, port 5173; mount `docs/api` read-only |
| `backend/` | API Go: `cmd/meteran` (serve/import/backup), `internal/{store,httpapi,domain,auth,report,importer,web}` |
| `compose.yml` | Produksi di STB: image `meteranadvanced:latest`, port + folder data dari `.env` (untracked, dibuat dari `.env.example`), bind mount `${METERAN_DATA_DIR:-./data}:/data` |
| `backend/Dockerfile` | Multi-stage: SPA (node, build-platform) → cross-compile arm64 → alpine runtime nonroot |
| `scripts/release.sh` | Menu interaktif build/push/scp/pull/deploy/reset-impor/status |
| `scripts/backup.sh` | Snapshot `VACUUM INTO` di STB, simpan N terakhir |
| `scripts/check-contract.sh` | Bandingkan path+method `openapi.yaml` dengan `router.go` |
| `PRD.md` | Sumber requirement (v1.16), kode FR/NFR/AC |
| `DESIGN.md` | Arah visual: palet, tipografi, bentuk, motion, layout |
| `memory.md` | Ringkasan kondisi aplikasi basic dan keputusan awal |

## Alur data dan rumus inti

Semua ada di `frontend/src/domain/energy.ts`:

1. `buildIntervals`: pemakaian = `reading_sebelum + recharge_amount - reading_sekarang`. Interval dengan durasi <= 0 atau konsumsi negatif dibuang.
2. `dailyUsage`: konsumsi dibagi proporsional per hari kalender WIB (batas 00:00 WIB), bukan ditempel ke tanggal pembacaan.
3. `windowUsage` + `summarize`: rata-rata N hari (7/14/30), sisa + stok token, buffer, `daysLeft = usable / avg`, `needKwh` sampai jam target tanggal 1, rekomendasi denominasi, minggu/bulan berjalan, perbandingan tarif, urgensi 24/40 jam, daftar `alerts`.
4. `effectivePrice`: total biaya / total kWh dari token yang punya biaya (rata-rata tertimbang).
5. `readingViews`: delta dan laju per interval, indikator `hemat`/`boros`/`tetap` dengan ambang 10 persen; baris top-up dan interval sesudahnya tidak dibandingkan.
6. `usageBuckets`: agregasi hari/minggu/bulan dengan penanda `estimated` untuk hari tanpa pembacaan.
7. Server tetap otoritatif: `GET /api/summary` menimpa hasil lokal; hasil lokal dipakai saat offline atau sebelum jawaban server tiba.

Waktu: semua perhitungan memakai offset WIB tetap (`+07:00`), bukan zona perangkat. Format tanggal/uang lewat `Intl` locale `id-ID`.

## Rute

| Path | Layar | Isi utama |
|---|---|---|
| `/` | Beranda | Sisa kWh, runway rail, pemakaian 7 hari, ringkasan, 5 catatan terakhir |
| `/riwayat` | Riwayat | Filter tanggal, daftar catatan, muat 20 per klik |
| `/token` | Token | Stok, target isi, kode (buka sesuai permintaan), tandai terpakai, batalkan token terkunci, hapus bila diizinkan server |
| `/laporan` | Laporan | Cetak browser, agregasi hari/minggu/bulan, unduh PDF server (mode server) |
| `/pengaturan` | Pengaturan | Buffer, jendela, jam target, denominasi, tarif resmi, impor berkas, status sesi/PIN |

## Status fitur terhadap PRD

| Area | Status |
|---|---|
| FR-01/02 input meter, tolak masa depan | Ada di `AddReadingDialog.vue` |
| FR-01/02 input meter, tolak masa depan | `AddReadingDialog.vue`; id dibuat klien agar `PUT` idempoten |
| FR-03 edit/hapus pembacaan | Ada di Riwayat; catatan `locked` tidak bisa dihapus |
| FR-04/05 konfirmasi kenaikan dan timestamp ganda | Klien menampilkan `409 confirmation_required`, kirim ulang dengan `confirm` |
| FR-10/11 kode token dan enkripsi | Form meminta kode; server menyimpan terenkripsi, `reveal` eksplisit |
| FR-12/13 status token (termasuk terkunci) | `unused`/`used`/`locked`, batalkan token terkunci, hapus hanya `canDelete` |
| FR-14/15 nominal otomatis dan token isi besar | Nominal opsional, tipe `additional`/`bulk`, status mismatch ditampilkan |
| FR-20 s/d FR-24 pemakaian, agregasi mingguan/bulanan | `weekKwh`, `monthKwh`, `usageBuckets` (hari/minggu/bulan) |
| FR-30 s/d FR-33 harga efektif dan pembanding tarif | `effectivePrice`, `tariffComparison`, field tarif di Pengaturan |
| FR-40 s/d FR-47 prediksi dan rekomendasi | `dataQuality`, status rekomendasi (`ok`, `largest_only`, `no_denomination_available`, `price_unknown`), `alerts` |
| FR-50 s/d FR-51 dashboard | Grafik 7 hari, minggu berjalan, banner urgensi/alerts |
| FR-60/61 push | Aktivasi di Pengaturan, endpoint VAPID di server, pengingat harian sekali per hari WIB (`internal/push`) |
| FR-70/71 PDF server | Tombol + unduhan blob di Laporan (mode server) |
| FR-80/81 pengaturan | Buffer, jendela, jam target, denominasi, tarif; impor berkas ada |
| FR-90/91 PIN dan rate limit | Layar kunci PIN (setup/masuk) untuk mode server; rate limit di server |
| FR-95/96 offline queue | Antrean `localStorage('meteran.queue.v1')`, replay `PUT` idempoten, penanda "antre" |
| FR-100 s/d FR-107 impor SQLite | Pratinjau + commit lewat kontrak; UI di Pengaturan |
| FR-110 s/d FR-113 `GET /api/homepage` | Klien dan tipe siap; backend belum |

Catatan: `memory.md` masih menyebut "scaffold belum dibuat"; bagian itu sudah tidak sesuai kondisi repo.

## Konvensi

- Nama file komponen dan view: PascalCase (`DashboardView.vue`), modul domain: camelCase (`energy.ts`).
- Teks UI Bahasa Indonesia, tanpa emoji dan tanpa istilah marketing. Angka: `121,72 kWh`, `Rp1.510`, `9,09 kWh/hari`. Tanggal: `22 Nov 2026, 15:20 WIB`.
- Format angka dan tanggal hanya lewat `domain/format.ts`, jangan `toLocaleString` ad hoc.
- Logika bisnis di `domain/`, bukan di komponen. Komponen hanya memformat dan memanggil aksi store.
- State global hanya satu store (`useAppStore`). Pengaturan ikut dipersist di kunci yang sama.
- CSS global terpusat di `styles/main.css`; style scoped di komponen hampir tidak dipakai.
- Aksesibilitas: setiap layar punya satu `h1` (Beranda memakai `h1.sr-only`), dialog memakai `aria-labelledby` ke judulnya, dan fokus dikembalikan ke pemicu lewat `composables/useFocusReturn.ts`. Target sentuh minimal 44 px pada layout sentuh (≤959 px).
- Batas 500 baris per file (NFR-11). `main.css` (1602) dan `domain/energy.ts` (514) masih melebihi; `api/preview.ts` mendekati batas.
- Tidak ada test runner. `npm test` masih placeholder yang keluar dengan kode 1.
- Repo adalah git repository; riwayat dan konvensi commit bisa dilihat lewat `git log`.

## Perintah

Semua tool Node dijalankan lewat Docker. Jangan `npm install`, `npm run`, atau `npx` di host: `node_modules` host sudah dihapus dan dependensi disimpan di named volume `*_meteran_node_modules`.

```bash
# Install dependensi + typecheck + build ke frontend/dist
docker compose -f compose.dev.yml run --rm --no-deps web sh -c "npm ci && npm run build"

# Regenerasi tipe dari kontrak (butuh mount docs/api)
docker compose -f compose.dev.yml run --rm --no-deps web sh -c "npm run api:types"

# Dev server, 0.0.0.0:5173
docker compose -f compose.dev.yml up

# Jika port 5173 sudah dipakai proyek lain, pakai port lain
docker compose -f compose.dev.yml run --rm -p 5174:5173 web

# Cek hasil build di http://localhost:4173
docker compose -f compose.dev.yml run --rm --no-deps -p 4173:4173 web npm run preview

# Regenerasi fixture dari SQLite aplikasi basic
python3 scripts/generate-fixture.py /path/to/meters.db > frontend/src/data/fixture.json

# Kelola image Docker proyek
bash scripts/dev-images.sh list
```

Debug layout: buka `/?debug` untuk overlay lebar viewport (kode di `main.ts`).

### Backend

```bash
cd backend
go test ./...                 # store, domain, httpapi, importer, report, web
go run ./cmd/meteran          # METERAN_ADDR=:8080 METERAN_DB=/tmp/meteran.db
bash ../scripts/check-contract.sh
```

Image arm64 (dibangun di amd64, tanpa qemu untuk stage build). `release.sh` menu 1 mengirim `--build-arg VERSION=<WIB>-<commit>`; versi ini dipakai Go (`-X main.version`) dan ditampilkan di **Pengaturan → Versi aplikasi**:

```bash
docker buildx build --platform linux/arm64 -t meteranadvanced:latest --load -f backend/Dockerfile .
```

### Rilis ke STB

```bash
cp scripts/release.env.example scripts/release.env   # gitignored; isi STB_DIR; STB_HOST/STB_USER untuk mode remote
bash scripts/release.sh      # menu: build → push → scp → pull → deploy
# .env tidak di-track; menu 5 menyalin .env.example → .env kalau belum ada
bash scripts/backup.sh       # snapshot DB
```

Konfigurasi ada di dalam repo (`scripts/release.env`, gitignored) supaya ikut saat checkout dipindah ke host lain. File lama di `~/.config/meteran/release.env` disalin sekali otomatis bila file dalam repo belum ada; bisa dihapus setelah itu.

Skrip bisa dijalankan di laptop maupun langsung di STB. `STB_MODE=auto` (default) memilih `local` kalau `STB_DIR` ada di mesin itu, selain itu `remote` lewat ssh. Di mode `local`: scp dan `docker load` dilewati, build memakai `docker build` native kalau mesinnya arm64. Paksa dengan `STB_MODE=local|remote` di `scripts/release.env`.

### Troubleshooting: `attempt to write a readonly database (8)` / `unable to open database file (14)`

Folder data atau `meteran.db` bukan milik `${METERAN_UID}:${METERAN_GID}` (biasanya karena dibuat `root` oleh Docker atau impor yang dijalankan via `sudo`). Entrypoint memperbaikinya otomatis saat container mulai sebagai root, lalu menolak start dengan pesan jelas kalau `/data` tetap tidak bisa ditulis. Perbaikan manual sekali jalan:

```bash
sudo chown -R 1000:1000 <METERAN_DATA_DIR>
```

Deploy memuat image baru dulu (`docker load`) baru `docker compose up -d` di mode remote, jadi container lama tetap melayani sampai image siap. Menu 6 mengosongkan folder data host (`METERAN_DATA_DIR`) lalu mengimpor `meters.db` aplikasi basic (hanya boleh saat DB kosong). Container `kwhmeter basic` tidak pernah disentuh skrip ini.

Data ada di folder host (`METERAN_DATA_DIR`, default `./data` di samping `compose.yml`), bukan named volume. Kalau sebuah STB masih punya volume lama `meteranadvanced-data`, salin isinya sekali sebelum deploy:

```bash
docker run --rm -v meteranadvanced-data:/from -v "$PWD/data:/to" alpine sh -c 'cp -a /from/. /to/'
```

Catatan verifikasi (2026-11-27 11:21 WIB): antrean reading sudah diuji terhadap backend nyata; skenario offline penuh untuk token juga sudah diuji dengan backend `VITE_API_MODE=server` di `localhost` dan permintaan `/api/*` diblokir. Hasil: PIN wajib tetap berlaku, saldo + enam catatan tampil dari salinan terenkripsi, token baru muncul sebagai "Tertunda" tanpa kode terbaca di `localStorage`, PIN salah tetap terkunci, dan logout menghapus PIN + salinan tetapi mempertahankan antrean. Token dengan biaya di luar toleransi denominasi saat replay tidak langsung hilang: ia berlabel "Perlu konfirmasi"; tombol "Konfirmasi" mengirim ulang dengan kode konfirmasi, lalu token muncul di daftar server tanpa reload. Token yang benar-benar ditolak 4xx berlabel "Gagal" dan bisa dihapus. Service worker `meteran-v2` tidak meng-cache respons `/api/*`.

Catatan cold start (2026-11-27 19:00 WIB): akar "data kosong setelah online lalu offline" adalah service worker menyajikan `index.html` lama dari cache, sehingga PWA memakai bundle JavaScript versi lama. Strategi navigasi diperbaiki: respons jaringan dipakai bila sampai dalam ~1,2 detik, cache hanya fallback; cache dinaikkan ke `meteran-v3` sehingga bundle lama dibuang saat update. Selain itu, saat `navigator.onLine=false` aplikasi langsung mengunci tanpa memanggil server, dan bila server menggantung lebih dari 8 detik aplikasi menganggap offline. Tampilan "Menyiapkan data…" dipakai selama inisialisasi agar tidak pernah menampilkan beranda kosong.

Catatan UI (2026-11-27 20:16 WIB): area pesan PIN memiliki tinggi tetap sehingga kegagalan input tidak menggeser keypad. Tooltip grafik (tren dan batang) muncul berdasarkan posisi horizontal di plot — titik terdekat dipakai, jadi kursor tidak perlu tepat berada di titik; berlaku juga untuk sentuhan. Verifikasi CDP: kursor di 27% lebar plot tren dan 18% lebar plot batang menampilkan tooltip yang benar.

Catatan ERR_FAILED final (2026-11-28 07:27 WIB): penyebab sebenarnya bukan cache, melainkan syntax TypeScript `new Promise<null>(...)` di `public/sw.js` (file JavaScript). SW melempar `Promise resolver undefined is not a function` pada setiap navigasi, sehingga Chrome mengembalikan `ERR_FAILED`; hard reload berhasil karena melewati SW. Generic sudah dihapus dan tes VM `scripts/sw.test.mjs` menjalankan handler navigasi SW untuk mencegah regresi.

Catatan loop 429 (2026-11-28 00:14 WIB): akar 429 berulang adalah `syncPending()` memanggil `init(true)` saat antrean kosong, sementara `init()` memanggil `syncPending()` — memproduksi 4 request API tanpa henti untuk setiap IP publik yang sama. Sekarang refresh hanya terjadi bila antrean benar-benar berkurang ke nol (`shouldRefreshAfterDrain`), dan event `online` tidak memanggil `syncPending` dua kali. Verifikasi CDP: 5 request API dalam 8 detik setelah login, lalu diam — bukan ratusan.

Catatan 429 lanjutan (2026-11-27 23:35 WIB): log memperlihatkan 429 untuk IP publik yang sama beberapa detik setelah login, padahal logger hanya menampilkan error — masih ada ratusan request 200 yang tidak terlihat dari satu IP/publik yang sama (PC + Android lewat NPM). Batas default dinaikkan ke 3000/menit; `/api/health` dan `/api/session` dikecualikan agar login tidak pernah terhalang. Setiap respons API kini punya header `X-RateLimit-Limit` dan `X-RateLimit-Remaining`; log 429 memuat IP, path, dan limit. Jika masih 429, baca header tersebut di DevTools untuk melihat sisa kuota dan cari klien yang memakai IP sama.

Catatan ERR_FAILED pasca-restart (2026-11-27 22:44 WIB): bila navigasi lebih lambat dari jendela 1,2 detik, SW terpaksa menyajikan shell lama yang merujuk aset hasil build sebelumnya. SW `meteran-v6` sekarang membandingkan shell lama dengan respons segar; bila berbeda, ia mengirim pesan `reload` ke semua tab dan aplikasi memuat ulang otomatis — tidak perlu hapus cookie. Log 429 kini mencatat IP dan path, serta response menyertakan `X-RateLimit-Limit` untuk melacak sumber badai request lintas perangkat yang memakai IP publik sama.

Catatan laporan dan rate limit (2026-11-27 22:17 WIB): `/api/usage` sekarang mengembalikan periode **terbaru lebih dulu** sebelum paging, jadi rentang 1–27 menampilkan 27…8 bukan 20…1; berlaku untuk hari/minggu/bulan. Halaman laporan hanya meminta satu bucket per perubahan filter. Rate limit tetap ada untuk menghentikan badai request lama, tetapi hanya untuk `/api/*`, 600 permintaan/menit per IP, bisa diubah lewat `METERAN_RATE_LIMIT`. Kunci IP diambil dari `X-Real-IP` lalu `X-Forwarded-For`. `BLOCK I/O` di `docker stats` adalah total sejak container dibuat, bukan laju saat ini; angka tulis besar berasal dari log JSON dan WAL SQLite. Aplikasi basic yang 8 MB tidak menyimpan SPA di binary dan tidak punya tabel token/settings/push, jadi perbandingan langsung tidak setara.

Catatan akses domain lanjutan (2026-11-27 21:49 WIB): 429 yang muncul setelah hard reload disebabkan rate limiter sebelumnya ikut membatasi aset SPA. Sekarang pembatasan hanya untuk `/api/*`, batas 600 permintaan/menit per IP, static/SW selalu lolos. SW `meteran-v5` tidak lagi mengembalikan `Response.error()` — bila jaringan gagal dan cache kosong, ia menampilkan halaman "Coba lagi" mandiri, jadi Chrome tidak terjebak `ERR_FAILED`. Aset baru hanya di-cache bila belum ada, menghindari badai request tiap navigasi.

Catatan domain/NPM (2026-11-27 21:32 WIB): perpindahan origin (port → domain) membuat registrasi service worker dan cache baru, jadi data lama tidak otomatis terbawa. SW kini: install tidak gagal kalau sebagian precache gagal, navigasi memakai `fetch(..., {cache:'reload'})` agar tidak menerima `index.html` basi dari cache HTTP, dan aset yang dirujuk index baru di-cache di background. Cache dinaikkan ke `meteran-v5`. Uji lokal dengan mengubah `index.html` setelah SW terpasang: reload langsung menampilkan konten baru (bukan cache lama). `ERR_FAILED` terjadi bila fetch gagal dan cache kosong; dengan install yang tahan gagal, cache shell tersedia sebagai fallback.

Catatan sumber daya dan akses HTTP (2026-11-27 20:59 WIB): `compose.yml` membatasi container ke 0,5 CPU dan 96 MB memori (reservasi 16 MB). Runtime di-set `GOMAXPROCS=1`, `GOGC=50`, `GOMEMLIMIT=16MiB`. Target `<12 MB` tidak tercapai dengan SPA yang di-embed — baseline native terukur ~13,4 MB — jadi 12 MB bukan angka yang bisa dijamin tanpa memisahkan aset dari binary. `crypto.subtle` tidak ada di HTTP biasa; kini `rememberPin`/`verifyPin` diamalkan (no-op) di konteks tidak aman sehingga login PIN lewat port tetap berhasil, tetapi antrean/offline dinonaktifkan dengan pesan "Mode offline memerlukan HTTPS.".

Catatan beban (2026-11-27 20:43 WIB): `docker stats` di STB menunjukkan CPU ~98% dengan `BLOCK I/O` 79 MB ditulis — pola badai request yang memenuhi log JSON, bukan loop idle di aplikasi. Middleware kini membatasi 300 permintaan/menit per IP (memakai `X-Forwarded-For` bila ada) dan hanya mencatat respons error/request >500 ms. Uji lokal 305 request menghasilkan 299×200, 6×429, dan hanya 8 baris log. Jika CPU masih tinggi setelah deploy, kumpulkan `docker logs --tail 50` untuk mencari sumber request, lalu tutup akses publik/non-VPN di reverse proxy.

Verifikasi lanjutan (2026-11-27 18:02 WIB): input pembacaan kini ditolak saat offline meskipun form masih terbuka; antrean pembacaan lama tampil di Riwayat dengan tombol Hapus. Token offline tidak menunggu server: kondisi offline terdeteksi lebih awal, dan request yang menggantung dibatasi 4 detik sehingga antrean tetap bisa dihapus. Kartu token tertunda menampilkan "Akan dikirim otomatis saat tersambung", tombol **Sinkron sekarang**, dan **Hapus** untuk semua status. Kode token yang dibuka tampil per 4 digit (`1234 5678 9012`). Token yang belum dipakai bisa dihapus langsung. Token terpakai/terkunci bisa dihapus setelah 50 pengisian token lain; sebelum itu berlabel tertunda penghapusan sesuai `deleteBlockedReason=refills_pending`.

### Notifikasi push

Tidak perlu konfigurasi: backend membuat pasangan kunci VAPID sendiri saat start pertama, lalu menyimpannya di `<METERAN_DATA_DIR>/vapid.json` (0600) agar langganan browser tidak rusak setelah restart. `/api/push/public-key` mengembalikan kunci itu, jadi tombol notifikasi di Pengaturan langsung bekerja.

Kalau mau memakai kunci sendiri, isi `.env`:

```bash
npx web-push generate-vapid-keys
# METERAN_VAPID_PUBLIC=...  METERAN_VAPID_PRIVATE=...
```

Kunci env selalu menang atas file cache. Backend memvalidasi pasangan kunci saat start; kalau tidak cocok atau salah format, log berisi `push: notifikasi nonaktif: ...` dan tombol notifikasi menolak dengan pesan jelas alih-alih error browser `applicationServerKey is not valid`.

Pengingat dikirim sekali per hari WIB setelah `reminderHour`, hanya bila catatan meter terakhir lebih dari 24 jam; endpoint yang membalas 404/410 otomatis dihapus dan `push_enabled` ikut turun saat langganan terakhir hilang. Web Push butuh HTTPS (atau `localhost`).

## Batasan yang membentuk keputusan

- Server target arm64, RAM ketat (~512 MB bebas), aplikasi basic masih berjalan paralel.
- Build hanya di mesin amd64 (WSL), server hanya menjalankan image.
- Runtime akhir: alpine arm64 nonroot, SPA dilayani Go dari `//go:embed`, SQLite di folder host (`METERAN_DATA_DIR`, default `./data`) lewat bind mount, container mulai sebagai root hanya untuk `chown -R` `/data` lalu turun ke `${METERAN_UID}:${METERAN_GID}` lewat `su-exec` (`backend/docker-entrypoint.sh`), jadi file milik user host walau folder sebelumnya dibuat root oleh Docker.
- API harus stabil agar backend Go bisa ditukar ke Rust bila target RAM menuntut.
- PWA, Web Push, dan HTTPS wajib untuk fitur notifikasi dan instalasi.

## Ke mana mencari

| Kebutuhan | Lihat |
|---|---|
| Ubah rumus pemakaian/prediksi | `frontend/src/domain/energy.ts` |
| Ubah format angka/tanggal | `frontend/src/domain/format.ts` |
| Tambah field data atau aksi | `frontend/src/domain/types.ts`, `frontend/src/stores/app.ts` |
| Ubah navigasi/shell | `frontend/src/layouts/AppShell.vue`, `frontend/src/router/index.ts` |
| Ubah warna/tema | `frontend/src/plugins/vuetify.ts`, `DESIGN.md` |
| Ubah layout/responsif | `frontend/src/styles/main.css` |
| Cek requirement | `PRD.md` (FR/NFR/AC) |
| Cek keputusan stack | `docs/decisions/` |
| Data contoh | `frontend/src/data/fixture.json` |
| Ubah kontrak API | `docs/api/openapi.yaml`, lalu `npm run api:types` |
| Ganti mode data | env `VITE_API_MODE` (`server`), lihat `frontend/src/api/index.ts` |
| Lanjutkan di host lain | `docs/reference/handoff.md` |
