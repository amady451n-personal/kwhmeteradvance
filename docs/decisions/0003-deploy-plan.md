# 0003: Rencana deploy (STB arm64)

- Tanggal: 2026-11-25 18:31 WIB
- Status: Accepted (2026-11-25 18:31 WIB) — dikunci sebelum implementasi backend
- Menggantikan: tidak ada
- Rencana implementasi: `docs/superpowers/plans/2026-11-25-meteranadvanced-backend-deploy.md`

## Fakta target

- Perangkat: STB yang kini murni Armbian Linux, `arm64`.
- Build di WSL `amd64`; produksi `arm64`. Pola lama: build silang → kirim ke server → deploy.
- Aplikasi baru berjalan **berdampingan** dengan `kwhmeter basic` (compose project dan port terpisah).
- TLS/reverse proxy bukan tanggung jawab skrip; skrip hanya mengekspos port. Container proxy sudah ada.

## Keputusan

1. **Satu artefak**: image Docker `meteranadvanced:latest`, dibangun di sini untuk `linux/arm64` lewat `docker buildx build --platform linux/arm64 --load`. Binary Go `CGO_ENABLED=0`, SQLite pure-Go, SPA di-embed (`embed.FS`). Tidak ada build/pull di STB.
2. **Kirim sebagai image**: `docker save meteranadvanced:latest | gzip > dist/meteranadvanced-image.tar.gz`, lalu `scp`. Di STB: `docker load` sebelum menyentuh compose lama, supaya downtime hanya stop+start container.
3. **Compose**: `image: meteranadvanced:latest`, `pull_policy: never`, port `${METERAN_PORT}:8080`, mount `${METERAN_DATA_DIR:-./data}:/data`, `restart: unless-stopped`, healthcheck `curl -sf localhost:8080/api/health`.
4. **Port dari `.env` yang di-track** (`METERAN_PORT`), agar port bisa diganti lewat repo tanpa file untracked.
5. **Deploy**: `docker load` → `docker compose up -d` → `docker image prune -f` → `rm -f /tmp/meteranadvanced-image.tar.gz`. Container lama `meteranadvanced` dihentikan oleh compose hanya saat image baru sudah ada di host.
6. **Satu skrip interaktif**, tanpa argumen posisional: `scripts/release.sh`, konfigurasi `scripts/release.env`. Menu: build & package, git push (sini), scp ke STB, git pull (STB), deploy (STB), reset & import, status/health, keluar.
7. **Reset & import**: `docker compose down -v` untuk `meteranadvanced`, lalu tanya path `meters.db` aplikasi basic, impor 1:1 (tabel `readings`; legacy tidak punya tabel token), menghasilkan data produksi. Hanya boleh jalan saat DB kosong (FR-100). Pemulihan manual di server (FR-106).
8. **Cadangan**: `scripts/backup.sh` (`.backup` SQLite, simpan N terakhir) untuk cron.
9. **Tes sampel** (mode preview + browser) dianggap cukup sebagai gerbang frontend; backend diverifikasi lewat tes kontrak OpenAPI.

## Alasan

- Build di `amd64` lalu `docker load` di `arm64` menghilangkan waktu build/pull dari jendela downtime.
- `.env` di-track mencegah drift port antara repo dan server.
- Berdampingan dengan aplikasi lama memungkinkan masa paralel tanpa memutus layanan lama.

## Konsekuensi

- Image harus dibangun ulang tiap rilis (`buildx`), tidak ada registry.
- `docker compose down -v` menghapus data `meteranadvanced`; skrip wajib konfirmasi.
- TLS/proxy, akses jaringan, dan Web Push HTTPS tetap di luar repo ini.
## Revisi 2026-11-25 21:38 WIB

Volume data diganti bind mount ke folder host (`METERAN_DATA_DIR`, default `./data`)
supaya DB bisa di-backup, diinspeksi, dan tidak hilang saat `docker volume prune`.
Container berjalan sebagai `${METERAN_UID}:${METERAN_GID}` (default `1000:1000`)
agar file SQLite milik user host. Reset & impor kini mengosongkan isi folder,
bukan `docker compose down -v`.

## Revisi 2026-11-25 21:52 WIB

`.env` tidak lagi di-track (menggantikan poin 4). File `.env.example` yang
di-track memuat default; `release.sh` menyalinnya menjadi `.env` di STB bila
belum ada. Alasan: `.env` yang tracked membuat setiap perubahan port/uid di
server tampil sebagai perubahan belum di-stage dan bisa menggagalkan
`git pull --ff-only`.

## Revisi 2026-11-25 22:24 WIB

Poin `user:` di compose dihapus. Container mulai sebagai root, entrypoint
meng-`chown -R` `/data` ke `${METERAN_UID}:${METERAN_GID}`, lalu `exec su-exec`
sehingga proses aplikasi tetap nonroot. Alasan: folder bind mount yang dibuat
Docker (atau sebelum `mkdir` skrip) milik root, dan `user: 1000:1000` membuat
SQLite gagal dengan `unable to open database file (14)`.

## Revisi 2026-11-25 23:01 WIB

`attempt to write a readonly database (8)` muncul saat `meteran.db` tertinggal
milik root (impor dijalankan sebagai root) sementara app jalan sebagai
`METERAN_UID`. Entrypoint kini: `chown -R` `/data`, cek tulis folder + file DB
sebagai uid target, dan `exit 1` dengan pesan perbaikan bila gagal (tidak lagi
menyembunyikan error chown). Impor di `release.sh` juga meneruskan
`METERAN_UID`/`METERAN_GID` sehingga file hasil impor langsung milik uid app.
