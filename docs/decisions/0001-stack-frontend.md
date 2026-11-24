# 0001: Stack frontend dan backend

- Tanggal: 2026-11-22 22:02 WIB
- Status: Accepted

## Keputusan

- Frontend: Vue 3.5 + Vuetify 4.3 (Material 3) + Vite 8 + TypeScript.
- Tema: mengikuti perangkat (light/dark lewat `prefers-color-scheme`), tanpa tema tetap.
- State: Pinia. Routing: Vue Router. PWA: `vite-plugin-pwa`. Antrean offline: Dexie.
- Backend: Go 1.25 stdlib + SQLite pure-Go. API-first agar bisa ditukar ke Rust bila target RAM menuntut.
- Runtime: distroless static nonroot, SPA hasil build dilayani Go.
- Image existing: `node:24.22.3-alpine`, `golang:1.25.14-alpine`, `gcr.io/distroless/static-debian12:nonroot`. Tidak ada pull baru.
- Dev: semua tool Node dijalankan lewat Docker dari `/mnt/d/yasin/meteran`.

## Alasan

- Vuetify 4 menyediakan Material 3 dan pola app shell Android (app bar, bottom navigation, FAB, dialog, bottom sheet).
- Go sudah terbukti memakai 7,23 MiB saat idle di aplikasi basic. Rust menjadi opsi hanya jika pengukuran menuntut.
- Image yang dibutuhkan sudah tersedia, sehingga tidak perlu pull baru.

## Konsekuensi

- Scaffold belum dibuat.
- Versi paket di-pin agar perubahan API Vuetify tidak mengejutkan.
- Tema perangkat wajib diuji pada mode terang dan gelap.
