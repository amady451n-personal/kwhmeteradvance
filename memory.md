# memory.md — kWh Tracker: kondisi basic vs PRD advanced

Terakhir diverifikasi: 2026-11-22. Sumber: server `root@192.168.8.200`, live `https://kwh.wareng.duckdns.org`, dan `PRD.md` (v1.16, 24 Okt 2026).
Dokumen ini ringkasan kerja. PRD lengkap tetap di `PRD.md` (384 baris). Baca ulang PRD hanya untuk detail FR/AC tertentu.

## 0. Arah proyek (diputuskan 2026-11-22)

- Aplikasi advanced dibangun **baru dari nol** di `D:\yasin\meteran`. Repo/server basic **tidak diubah** dan bukan basis kode.
- Pondasi hanya **konsep, logika domain, dan PRD**; tidak menyalin kode repo basic.
- Basic v1.11.1 sudah dimigrasikan backend-nya ke **Go** dan RAM turun (live terukur 7,23 MiB idle).
- Advanced: backend **minimal Go**. Jika target RAM belum tercapai, opsi lanjut ke **Rust** dengan target lebih kecil dari Go. API contract dijaga agar backend bisa ditukar.
- Prioritas utama: **visual frontend** (Material 3, mobile-first, Bahasa Indonesia).
- Server target: arm64, RAM ketat (~512 MB bebas; basic dan advanced berjalan paralel selama masa uji).
- Stack final belum diputuskan; akan dibahas terpisah. Catat keputusannya di dokumen ini saat sudah ada.
- Migrasi data: impor sekali snapshot SQLite basic (225 baris) mengikuti FR-100–107; bukan integrasi berkelanjutan.
- Belum ada scaffold/kode baru. Jangan mulai implementasi sebelum stack disepakati.

## 0a. Environment & stack (2026-11-22)

- WSL: Ubuntu 22.04, Docker 28.0.1 native + Compose v2.40.3, 4 CPU, RAM WSL 5,8 GB.
- Path yang sama: `D:\yasin\meteran` = `/mnt/d/yasin/meteran`. Bind mount Docker sudah diuji; file dari container terlihat sebagai `node:node`, permission 777 dari Windows.
- Tool WSL: Go 1.25 ada; Node/npm/Rust tidak ada di WSL (npm yang terlihat berasal dari PATH Windows). Semua tooling Node dijalankan lewat container.
- Image existing yang dipakai, tanpa pull: `node:24.22.3-alpine`, `golang:1.25.14-alpine`, `gcr.io/distroless/static-debian12:nonroot`.
- Kebijakan image: `infra/images.base.txt` = image shared, jangan hapus. `infra/images.pulled.txt` = pull baru proyek ini. Helper: `bash scripts/dev-images.sh list|add|rm`.
- Frontend diputuskan 2026-11-22 22:02 WIB: Vue 3.5 + Vuetify 4.3 (Material 3) + Vite 8 + TypeScript; shell bergaya aplikasi Android (top app bar, bottom navigation, FAB, dialog/bottom sheet), Pinia, Vue Router, `vite-plugin-pwa`, Dexie untuk antrean offline. Tema mengikuti perangkat (`prefers-color-scheme`), tanpa tema tetap.
- Backend: Go 1.25 stdlib + SQLite pure-Go (`modernc.org/sqlite`); Rust hanya jika pengukuran RAM menuntut. Kontrak API dijaga agar backend bisa ditukar.
- Runtime: distroless static nonroot; SPA hasil build dilayani Go.
- Kompatibilitas: Vuetify 4 peer Vue `^3.5`, `vite-plugin-vuetify >=2.1`; Vite 8 butuh Node `^20.19` atau `>=22.12`.
- Status: stack disetujui 2026-11-22 22:02 WIB. Scaffold belum dibuat. Tidak ada image baru yang di-pull.

## 1. Dua hal yang dibandingkan

| | Basic (live) | Advanced (PRD) |
|---|---|---|
| Versi | v1.11.1, rilis 12 Agu 2026 | PRD v1.16, 24 Okt 2026, status Disetujui |
| Tujuan | Catat sisa kWh, proyeksi dasar, tren, PDF, CSV | PWA pencatat + manajemen token + rekomendasi beli + PIN + offline + push + backup |
| Bahasa UI | Inggris | Indonesia |
| Stack | Go (live), Python FastAPI (legacy di repo) | Teknologi bebas (diputuskan di luar PRD), SQLite + PWA + Web Push + Docker Compose wajib |

## 2. Fakta deployment basic

- Host `aml-s9xx-box`, aarch64, RAM 2 GB.
- Path app: `/media/devmon/redext4/kwhtracker`.
- Domain `kwh.wareng.duckdns.org` (alias `kwh.wareng.dpdns.org`) → Nginx Proxy Manager (Let's Encrypt, force SSL, HTTP/2) → `192.168.8.200:4040` → container `:8000`.
- Container `kwh-tracker`, image `kwhtracker:arm64-go` (11 MB, `FROM scratch`), healthcheck via binary `-healthcheck`.
- Data bind mount `./data` → `/app/data`; SQLite `data/meters.db`; `TZ=Asia/Jakarta`.
- Compose aktif: `docker-compose.go.yml`. Compose Python lama: `docker-compose.yml` (tidak jalan).
- Build: `buildarm64-go.sh`, dua opsi (WSL amd64 cross-compile + test + tar; server load + compose up). Sesuai NFR-14/15.
- RAM idle terukur: **7.23 MiB** (`docker stats --no-stream`). Target PRD <15 MB idle, ≤60 MB puncak → sudah lolos.
- Image Python lama 725 MB; image Go 11 MB. Python idle 27–28 MB (fakta PRD).
- Tidak ada auth, CORS `*`, domain publik. NPM tanpa Access List.
- Repo: Go ~4.300 baris termasuk test; `frontend/app.js` 914 baris (melanggar NFR-11 ≤500 baris).

## 3. Data live (snapshot)

- Tabel tunggal `readings(id, meter_name, reading, timestamp, notes, recharge_amount, cost)`.
- 225 baris, semua `main`, 2026-01-02 s/d 2026-11-22 15:20.
- Terakhir: 121.72 kWh. Recharge terakhir: 199.7 kWh / Rp301.500 → Rp1.509,7646/kWh.
- Backup pra-cutover Go: `data/meters.db.pre-go-20261121-2332`.
- Timestamp DB: teks WIB naive (`YYYY-MM-DD HH:MM:SS.ffffff`), dibaca ulang sebagai Jakarta.
- Tanpa migrasi; hanya `CREATE TABLE IF NOT EXISTS`.
- Statistik live Nov 1–22: avg 9.09, min 3.10, max 11.78, total 189.25 kWh, 18 reading, usable 111.72, proyeksi 36.03 hari → 28 Des 2026 21:22 WIB.
  - Proyeksi memakai laju interval terakhir (3.10 kWh/hari) → optimistis. Pakai avg 9.09 ≈ 12 hari.

## 4. Cara kerja basic (yang harus dipertahankan/diperbaiki)

- Konsumsi per interval = `reading_sebelumnya - reading_sekarang`; interval negatif (top-up) atau durasi ≤0 dibuang total.
- Tidak ada pembagian proporsional ke hari kalender; tidak ada agregasi mingguan; bulanan hanya tabel tren.
- `kwh_per_day` = konsumsi / jam * 24.
- Proyeksi = `(saldo - buffer) / laju_interval_terakhir`; buffer default 10, diatur di UI/localStorage.
- Harga token = `cost / recharge_amount` recharge terakhir. `token_price` == `cost_per_used_kwh`.
- `cost_per_kwh` bulanan = total `cost` recharge bulan itu ÷ total kWh terpakai bulan itu → bukan tarif, mudah menyesatkan.
- Chart: satu titik per reading (saldo + laju harian); top-up di-clamp 0.
- List reading: delta vs tetangga lebih lama + perbandingan absolut (less/more/same), heuristik top-up jika lompatan >10 kWh. Tidak ada ambang ±10%.
- Jika rentang tanggal <2 reading, statistik fallback ke seluruh histori; `period_days` tetap rentang UI.
- Input timestamp masa depan diterima; kenaikan saldo tanpa top-up disimpan diam-diam; timestamp ganda tidak diperingatkan.
- Meter `main` hardcoded di frontend; API mendukung `meter_name` lain tetapi UI tidak menampilkannya.

## 5. Peta API basic

| Endpoint | Catatan |
|---|---|
| `GET /health` | versi dari `VERSION.md`, waktu WIB |
| `GET /api/last-input` | lintas semua meter |
| `GET /api/stats/{meter}` | `start_date`, `end_date`, `buffer`, `days` |
| `GET /api/trends/{meter}` | `months` 1–24, rentang tanggal |
| `GET /api/chart/{meter}` | `days` 1–365, rentang tanggal |
| `GET /api/readings/list` | `limit` ≤200, `offset`, default UI 20 |
| `GET/PUT/DELETE /api/readings/{id}` | query string; `PUT` menerima literal `"undefined"` |
| `POST /api/readings` | `meter_name`, `reading`, `timestamp`, `recharge_amount`, `cost`, `notes` |
| `POST /api/import/csv` | multipart `file`; format ekspor PLN; atomik; tanpa dedupe/preview |
| `GET /api/export/pdf/{meter}` | ringkasan + detail interval |
| `GET /api/homepage` | widget gethomepage.dev |
| `GET /mobilestatus` | polling baterai 3 HP (`192.168.8.100/103/110:8765`) |

Frontend: 8 kartu (Add, Recent, Status, Trend, Monthly, Export, Import, API), vanilla JS, Chart.js CDN, modal edit/hapus, toast.
localStorage: `readingsDateRange`, `trendDateRange`, `monthlyDateRange`, `bufferValue`.
PWA: `manifest.json` + `sw.js`; **ikon 192/512 404**; SW didaftarkan 2x; cache `v1` tidak pernah di-bump.
Last-input urgency basic sudah 3 tingkat (<24 jam netral, 24–40 hijau, >40 merah) — cocok FR-50b, teks Inggris.

## 6. Ringkasan PRD advanced (yang wajib ada)

- Satu pengguna, satu meter, tanpa akun; akses dilindungi PIN (hash, min 4 karakter, sesi 30 menit idle, rate limit). Endpoint homepage read-only tanpa PIN.
- Pembacaan: timestamp default sekarang, boleh masa lalu, tolak masa depan; edit/hapus dengan hitung ulang; konfirmasi jika saldo naik tanpa top-up; peringatan timestamp ganda; tolak kWh negatif/kosong.
- Token sebagai entitas: kode (wajib, terenkripsi, disembunyikan), kWh, biaya bayar, waktu beli, jenis (Token tambahan / Token isi besar), nominal otomatis dari denominasi ±10%, status **Belum dipakai / Dipakai / Terpakai (terkunci)**.
  - Tandai Dipakai → input sisa kWh baru → jadi reading tertaut.
  - Reading berikutnya lebih rendah → token Terpakai & terkunci.
  - Sebelum terkunci bisa dibatalkan (reading tertaut dihapus).
  - Terkunci = historis, tidak bisa edit; hapus opsional setelah 50 pengisian; ringkasan tetap dihitung.
  - Token tambahan Belum dipakai = stok; Token isi besar Belum dipakai tidak dihitung stok.
- Konsumsi: `prev + topup - cur`; alokasi proporsional per hari kalender; hari tanpa reading ditandai estimasi; agregasi mingguan (Senin–Minggu) & bulanan.
- Harga: efektif per pembelian = biaya ÷ kWh; rata-rata tertimbang = total biaya ÷ total kWh semua pembelian; dipakai untuk nilai rupiah pemakaian; bandingkan dengan tarif resmi PLN (info saja, tidak dipakai hitung).
- Prediksi: rata-rata N hari (default 7; opsi 14/30); tanggal sentuh buffer = `(sisa + stok token tambahan - buffer) / rata-rata`; tampil "beli paling lambat tanggal X"; konsisten dengan homepage.
- Rekomendasi: satu token tambahan agar cukup sampai tanggal 1 berikutnya pukul 06:00 WIB (dapat diatur); kebutuhan kWh → rupiah (harga tertimbang) → denominasi terkecil yang cukup; jika tidak cukup pakai terbesar + peringatan; jika denominasi kosong hanya kWh+rupiah.
- Dashboard: sisa terakhir, pemakaian hari ini, grafik 24 jam/harian, ringkasan minggu/bulan, rata-rata, last input relatif 3 tingkat urgensi, tanggal sentuh buffer, rekomendasi, stok token, estimasi harga/kWh; banner; filter tanggal per bagian disimpan localStorage.
- Push: pengingat 20:00 WIB hanya jika belum ada reading hari itu; aktivasi manual (iOS); subscription kedaluwarsa didaftarkan ulang.
- PDF: ringkasan + 10 reading terakhir tanpa baris top-up.
- Pengaturan: buffer, jendela rata-rata, golongan tarif/daya (info), tarif resmi, denominasi token, jam target isi besar (06:00), jam pengingat, PIN, notifikasi, impor.
- Offline: input reading & token disimpan lokal, antre, sync otomatis tanpa duplikasi; hasil resmi tetap dari server.
- Impor sekali dari SQLite lama: hanya jika DB baru kosong; preview → konfirmasi → commit atomik; validasi (saldo naik tanpa top-up, timestamp ganda, top-up tanpa biaya, nilai invalid, nominal di luar ±10%); hanya `main`; `old_id`; urut timestamp; baris recharge → token Terpakai tanpa kode "belum lengkap"; rekonsiliasi jumlah baris/kWh/biaya.
- Backup: harian 02:00 WIB, retensi 14 harian + 8 mingguan; upload terenkripsi ke Google Drive via container rclone tiap 2 hari (OAuth Production), bersihkan berdasarkan umur; gagal upload = peringatan.
- NFR: UI Indonesia, WIB, PWA installable + ikon/splash, Material 3 mobile-first (Chrome Android, Safari iOS 16.4+), aksesibilitas, Sonar way + max 500 baris/file, RAM <15 MB idle/60 MB puncak per container, Docker Compose 1 container utama + uploader, HTTPS di luar compose, SQLite di-mount, rahasia via env, build arm64 di WSL (tidak di server).
- Non-goal: integrasi/beli PLN, OCR, multi-meter/multi-user, tarif resmi sebagai perhitungan, ekspor CSV, sync terus-menerus dengan app lama, push selain pengingat.

## 7. Gap matrix basic → PRD

| Area | Basic | PRD | Status |
|---|---|---|---|
| CRUD reading | Ada | + tolak masa depan, konfirmasi kenaikan, timestamp ganda, lock | Partial |
| Hitung konsumsi | Buang interval top-up | Sertakan kWh top-up | **Beda rumus** |
| Alokasi harian/mingguan | Tidak ada | Proporsional + estimasi + mingguan | **Belum ada** |
| Tren bulanan | Ada | + penanda estimasi | Partial |
| Proyeksi | Laju interval terakhir | Rata-rata 7/14/30 hari + stok token | **Beda dasar** |
| Rekomendasi token | Tidak ada | Satu token sampai tgl 1 06:00 | **Belum ada** |
| Token lifecycle | Hanya field recharge/cost | Entitas + kode + status + lock | **Belum ada** |
| Harga efektif | Harga pembelian terakhir | Rata-rata tertimbang + tarif pembanding | Partial |
| List comparison | Absolut, heuristik >10 kWh | ±10% hemat/boros/tetap, skip top-up | Partial |
| Dashboard | 8 kartu | + hari ini, 24 jam, minggu, stok, rekomendasi | Partial |
| Filter tanggal | 3 rentang localStorage | Per bagian, termasuk chart/mingguan | Partial |
| Urgensi last input | 3 tingkat | 3 tingkat + banner | Hampir |
| PIN/auth | Tidak ada | PIN + sesi + rate limit | **Belum ada** |
| Offline | Cache statis saja | Antrean input + sync | **Belum ada** |
| Push | Tidak ada | 20:00 WIB kondisional | **Belum ada** |
| PDF | Ringkasan + interval | Ringkasan + 10 reading non-top-up | Beda isi |
| Homepage | Struktur mirip | value = epoch depletion; null/N/A; N-day avg | Beda semantik |
| Impor | CSV langsung | SQLite sekali + preview + atomik + rekonsiliasi | **Beda jalur** |
| Backup | Manual | Harian + retensi + GDrive | **Belum ada** |
| PWA | Manifest/SW, ikon 404 | Installable, ikon/splash | Rusak sebagian |
| UI | Inggris, CSS biasa | Indonesia, Material 3 | **Belum** |
| Kualitas kode | Test Go ada, tanpa CI/Sonar | Sonar way, ≤500 baris | Partial |
| RAM | 7.23 MB idle | <15 MB | **Lolos** |
| Build/deploy | Cross-build WSL, 2 opsi | Sama | **Lolos** |
| Compose | 1 container utama | 1 utama + uploader | Partial |

## 8. Konflik/keputusan yang perlu diklarifikasi

1. **Jaringan:** PRD constraint bilang server hanya LAN/VPN, tetapi basic live memakai domain publik tanpa auth. Advanced wajib PIN; homepage tetap publik read-only.
2. **Impor:** PRD minta SQLite lama; basic hanya punya CSV. Perlu endpoint impor baru + preview, bukan sekadar memakai CSV.
3. **Teknologi:** PRD v1.10 melepas pilihan framework (Vuetify batal). Stack Go sekarang cocok NFR RAM/build; frontend perlu rewrite untuk M3/Indonesia.
4. **Baseline:** PRD menganggap app lama punya rata-rata kWh/hari; basic punya angka avg, tetapi proyeksi memakai interval terakhir. Advanced harus memakai N-day avg.
5. **Snapshot impor:** PRD §15 masih terbuka (tanggal snapshot).
6. **Penyimpanan settings:** PRD tidak menetapkan tabel/skema; perlu diputuskan (DB vs env).
7. **CSV:** PRD non-goal ekspor CSV; impor CSV basic tidak perlu dipertahankan sebagai fitur utama.

## 9. Prinsip yang dibawa vs dibuang (untuk aplikasi baru)

**Dibawa (konsep/logika, bukan kode):**

- `reading` = sisa kWh, bukan angka meter yang menanjak.
- Konsumsi dari penurunan saldo; kWh top-up ditambahkan ke rumus, bukan membuang interval.
- Buffer = sisa kWh minimum; WIB; kWh 2 desimal; rupiah bulat.
- Urgensi input 3 tingkat; filter tanggal per bagian disimpan di browser.
- Kontrak `/api/homepage` (dengan perbaikan semantik sesuai PRD).
- SQLite lokal sebagai sumber data; impor snapshot lama sekali.
- Deployment arm64, cross-build dari amd64, Docker Compose, HTTPS di luar compose.

**Dibuang/diganti:**

- Hardcode meter `main` di UI.
- Proyeksi memakai laju interval terakhir.
- `cost_per_kwh` bulanan (biaya recharge ÷ konsumsi).
- Heuristik top-up >10 kWh.
- CSV import sebagai jalur migrasi utama.
- Tanpa auth dan CORS `*`.
- UI Inggris dan `app.js` monolit 914 baris.
- Service worker cache `v1` ganda dan ikon PWA 404.
- Satu tabel `readings` tanpa token/settings.

## 10. Cara verifikasi cepat (tanpa baca ulang kode)

```bash
ssh root@192.168.8.200
docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
docker stats --no-stream kwh-tracker
curl -s https://kwh.wareng.duckdns.org/health
curl -s "https://kwh.wareng.duckdns.org/api/stats/main?start_date=2026-11-01&end_date=2026-11-22&buffer=10"
```

DB tanpa sqlite3 di host: kirim skrip python3 lewat stdin (`python3 -`), modul `sqlite3` tersedia.
File kunci: `internal/domain/{consumption,stats,trends,chart,listview}.go`, `internal/api/*`, `frontend/app.js`, `VERSION.md`, `ONBOARDING.md` (masih menjelaskan Python), `PRD.md`.
