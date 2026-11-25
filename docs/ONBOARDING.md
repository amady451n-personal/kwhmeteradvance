# Onboarding: meteran

Terakhir diverifikasi: 2026-11-24 09:48 WIB.

## Ringkasan

PWA pencatat sisa kWh meteran PLN prabayar untuk satu rumah. Fokus v1: catat meter cepat, lihat pemakaian harian, prediksi kapan sisa menyentuh buffer, dan rekomendasi satu token sampai tanggal 1 berikutnya.

Kondisi repo saat ini: **frontend pratinjau saja**. Belum ada backend, API, SQLite, PWA/service worker, antrean offline, PIN, impor, atau push. Data berasal dari snapshot `meters.db` aplikasi basic dan disimpan di `localStorage`.

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

Kontrak adalah sumber kebenaran tipe. Mode default `preview`: klien palsu menjawab bentuk payload yang sama dengan server, menyimpan ke `localStorage('meteran.preview.v2')` dan seed dari `data/fixture.json`. Set `VITE_API_MODE=server` untuk memakai `fetch` ke `/api/*` (backend belum ada).

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
| FR-60/61 push | Kontrak + tipe ada; UI aktivasi menunggu backend |
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
- Repo bukan git repository, jadi konvensi branch/commit tidak dapat dideteksi.

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

Image arm64 (dibangun di amd64, tanpa qemu untuk stage build):

```bash
docker buildx build --platform linux/arm64 -t meteranadvanced:latest --load -f backend/Dockerfile .
```

### Rilis ke STB

```bash
mkdir -p ~/.config/meteran && cp scripts/release.env.example ~/.config/meteran/release.env   # isi STB_DIR; STB_HOST/STB_USER untuk mode remote
# config sengaja di luar repo: update/checkout tidak bisa menghapusnya
bash scripts/release.sh      # menu: build → push → scp → pull → deploy
# .env tidak di-track; menu 5 menyalin .env.example → .env kalau belum ada
bash scripts/backup.sh       # snapshot DB
```

Skrip bisa dijalankan di laptop maupun langsung di STB. `STB_MODE=auto` (default) memilih `local` kalau `STB_DIR` ada di mesin itu, selain itu `remote` lewat ssh. Di mode `local`: scp dan `docker load` dilewati, build memakai `docker build` native kalau mesinnya arm64. Paksa dengan `STB_MODE=local|remote` di `~/.config/meteran/release.env`.

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

Catatan verifikasi: antrean offline (`localStorage('meteran.queue.v1')`) belum diuji terhadap backend nyata di STB; uji manual (putus jaringan → tambah catatan → pulihkan) saat pertama kali server mode dipakai.

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
