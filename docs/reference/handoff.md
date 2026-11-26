# Handoff — meteranadvanced (`dipsik`)

- Dibuat: 2026-11-26 17:30 WIB
- Tujuan: lanjutkan sesi ini di host lain tanpa kehilangan konteks atau file pendukung.
- Salinan di repo: `docs/reference/handoff.md` (file temp asal sudah dihapus).
- HEAD: `b6a7d0a` — `feat(push): send daily reminders with self-managed VAPID keys`.

## Status singkat

Aplikasi sudah berjalan penuh (frontend Vue 3 + backend Go + SQLite + PWA). Fase terakhir adalah iterasi dari pemakaian nyata di STB: perbaikan timezone impor, DB read-only, sesi kedaluwarsa, dua kartu tren, dan pengingat push. Tidak ada pekerjaan yang setengah jadi di kode.

Yang **belum** selesai dan jadi fokus sesi berikutnya:
1. Deploy ke STB: menu 1 (build) lalu menu 5 (deploy), lalu menu 6 (reset & impor) agar timestamp legacy masuk dengan perbaikan wall-clock.
2. Uji antrean offline (`localStorage('meteran.queue.v1')`) terhadap backend nyata: putus jaringan → tambah catatan → pulihkan → cek tidak duplikat.
3. Repo belum punya `git remote`; menu 2 (push) dan menu 4 (pull) baru jalan setelah remote ditambahkan.
4. `docs/ONBOARDING.md` masih punya kalimat lama di bagian awal (baris 9, 49, 127) yang menyebut backend belum ada / repo bukan git. Sudah tidak sesuai; perbarui saat ada kesempatan.

## Dokumen sumber (jangan duplikasi di sini)

| Kebutuhan | File |
|---|---|
| Requirement, FR/NFR/AC | `PRD.md` |
| Arah visual | `DESIGN.md` |
| Konvensi, perintah, troubleshooting, status fitur | `docs/ONBOARDING.md` |
| Keputusan arsitektur | `docs/decisions/0001..0003` |
| Rencana implementasi backend | `docs/superpowers/plans/2026-11-25-meteranadvanced-backend-deploy.md` |
| Kontrak API (sumber kebenaran, 30 rute) | `docs/api/openapi.yaml` |
| Ringkasan kondisi aplikasi lama | `memory.md` (bagian bertanda "belum ada" sudah usang) |
| Catatan kerja/keputusan harian | commit message + `git log` |

## Lingkungan host lama

- Node/npm **hanya** lewat Docker (`compose.dev.yml`); jangan install di host. Go tersedia di host.
- Build arm64 di amd64: `docker buildx build --platform linux/arm64 ...`. Tahap `web`/`build` native, tahap runtime arm64 menjalankan `apk/adduser/chmod` → butuh qemu.
- Setelah WSL restart, binfmt hilang; pulihkan dengan:
  `docker run --privileged --rm tonistiigi/binfmt --install arm64`
- Port dev 5174 (5173 terpakai), backend uji 809x, preview build 5199.
- Container uji: `docker run --platform linux/arm64 -e METERAN_UID=$(id -u) -e METERAN_GID=$(id -g) -v <dir>:/data ...`.
- Kunci VAPID dibuat otomatis di `<METERAN_DATA_DIR>/vapid.json` (0600); env `METERAN_VAPID_*` menang bila diisi. Jangan ikutkan kunci production ke dalam doc ini.

## Konfigurasi yang ikut berpindah

- `scripts/release.env` (gitignored, 0600) sekarang menjadi satu-satunya file konfigurasi rilis. Isinya spesifik host (alamat/user STB, folder deploy) — baca dari situ, jangan tempel di dokumen.
- `~/.config/meteran/release.env` di host lama sudah dipindahkan ke dalam repo dan dihapus. Skrip masih punya migrasi sekali jalan untuk checkout lama.
- `.env` (gitignored) dibuat dari `.env.example` oleh menu deploy bila belum ada.

## Referensi eksternal

Semua file pendukung sesi sudah ada di dalam repo. Tidak ada file referensi yang tertinggal di host ini. Handoff lama milik proyek lain (`handoff-garis-keluarga.md` di `/tmp/opencode`) juga dibersihkan; salinan kanoniknya ada di repo proyek tersebut.

## Skill yang disarankan untuk sesi berikutnya

- `codebase-onboarding` — orientasi cepat jika agen baru belum pernah melihat repo ini.
- `verification-before-completion` — sebelum klaim deploy/perbaikan selesai.
- `systematic-debugging` — jika ada bug sesungguhnya muncul di STB (jangan menebak).
- `antislop` — untuk pekerjaan UI/copy; baca juga aturan di `AGENTS.md`.
