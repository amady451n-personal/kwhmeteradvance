# 0002: Kontrak API

- Tanggal: 2026-11-24 10:15 WIB
- Status: Accepted (2026-11-24 22:55 WIB) — frontend memakai kontrak ini
- Menggantikan: tidak ada

## Keputusan

1. **Artefak otoritatif**: `docs/api/openapi.yaml` (OpenAPI 3.1.0). Satu-satunya sumber bentuk payload. Tidak ada salinan tangan di wiki, mock, atau kode backend.
2. **Konsumen**: `frontend/` (Vue). **Provider**: backend Go yang belum dibuat. **Pemilik kontrak**: repo ini; perubahan kontrak disetujui sebelum implementasi.
3. **Bentuk respons**: sukses `{ "data": ..., "meta"?: ..., "links"?: ... }`; gagal `{ "error": { "code", "message", "details"? } }` dengan status HTTP semantik.
   - Pengecualian: `GET /api/homepage` memakai bentuk tetap FR-111, dan `GET /api/reports/summary.pdf` mengembalikan biner.
4. **Path**: `/api/*` tanpa prefiks versi, karena FR-110 mengunci `/api/homepage`. Aplikasi dan server dirilis bersama, jadi perubahan hanya boleh aditif. Perubahan breaking = endpoint baru + klien diperbarui dalam rilis yang sama.
5. **ID**: string opaque (ULID), bukan angka. Klien boleh membuat ID sendiri saat offline, sehingga `PUT /api/readings/{id}` dan `PUT /api/tokens/{id}` bersifat upsert idempoten untuk replay antrean.
6. **Auth**: cookie sesi `meteran_session` (HttpOnly, SameSite=Strict, Secure saat HTTPS), idle 30 menit. PIN disimpan sebagai hash Argon2id di server, tidak pernah dikirim balik. Endpoint publik: `/api/homepage`, `/api/health`, `GET|POST /api/session`. Rate limit PIN: 5 percobaan / 15 menit / IP, balasan `429` + `Retry-After`.
7. **Konfirmasi**: pelanggaran lunak (FR-04 kenaikan tanpa top-up, FR-05 timestamp ganda) dibalas `409 confirmation_required` dengan `details[].code`; klien mengirim ulang dengan `confirm: ["..."]`. Pelanggaran keras (timestamp masa depan, kWh negatif) dibalas `422`.
8. **Server kanonis**: nilai turunan (pemakaian, prediksi, rekomendasi, status token) dihitung server (FR-96). Klien boleh menghitung lokal untuk tampilan instan/offline dan wajib menandainya "sementara" sampai server membalas.
9. **Offline**: antrean tulis berisi payload `PUT` (idempoten), dikirim berurutan saat online. Konflik diselesaikan last-write-wins per ID; server mengembalikan resource hasil merge. Implementasi v1 memakai `localStorage('meteran.queue.v1')`, bukan IndexedDB: antrean hanya menyimpan catatan meter (payload kecil) dan replay dimulai saat `online`/muat ulang. Ganti ke IndexedDB bila antrean melebihi kuota `localStorage` atau butuh lampiran biner.
10. **Waktu**: semua timestamp ISO 8601 dengan offset `+07:00` (WIB). Server menerima `Z`, selalu membalas `+07:00`. Zona tetap `Asia/Jakarta`; tidak ada zona per pengguna.
11. **Angka**: kWh `number` 2 desimal; rupiah `integer`; persen `number` 2 desimal. Tidak ada string berformat di payload kecuali field `formatted` pada `/api/homepage` dan pesan siap tampil pada `alerts[].message`.
12. **Enum**: klien wajib toleran terhadap nilai enum baru (abaikan, jangan crash). Server tidak boleh menghapus atau mengganti nama field.
13. **Verifikasi**: tipe konsumen di-generate dari kontrak (`npm run api:types`, `openapi-typescript`). Fixture/mock divalidasi terhadap tipe hasil generate. Backend nanti wajib memvalidasi respons nyata terhadap kontrak yang sama sebelum integrasi.

## Alasan

- Satu kontrak mencegah drift antara mock frontend dan backend Go yang ditulis belakangan.
- ID dari klien + upsert idempoten menyelesaikan FR-95/96 tanpa protokol sinkronisasi khusus.
- Server kanonis menutup risiko dua rumus berbeda antara klien dan server.

## Konsekuensi

- `stores/app.ts` dan `domain/energy.ts` tetap dipakai, tetapi sumber data pindah ke lapisan `api/`; perhitungan lokal menjadi fallback offline.
- Tipe `Reading.id`/`TokenRecord.id` berubah dari `number` ke `string`; `TokenStatus` bertambah `locked`.
- Frontend butuh adaptor mock (mode pratinjau) agar tetap berjalan sebelum backend ada.
- Penghapusan token historis mengikuti aturan 50 pengisian; server mengirim `canDelete` dan `deleteBlockedReason`, klien tidak menghitung sendiri.

## Hasil implementasi frontend

- Tipe digenerate lewat `npm run api:types` (`openapi-typescript`), bukan ditulis tangan.
- `src/api/client.ts` memakai fetch + cookie sesi; `src/api/preview.ts` menjawab bentuk payload yang sama dari `localStorage` supaya UI tidak bercabang per mode.
- `VITE_API_MODE=server` mengaktifkan jalur HTTP; default `preview` untuk pengembangan tanpa backend.
- Server tetap otoritatif setelah `GET /api/summary` berhasil; hitungan lokal hanya fallback.
- Antrean offline hanya untuk `PUT /api/readings/{id}`; aksi non-idempoten (mark-used, impor, push) menunggu koneksi.
- Efek ke UI: ID string, field camelCase, `409 confirmation_required` ditangani dialog, status token `unused`/`used`/`locked`.

## Revisi 2026-11-27 18:12 WIB

`canDelete` juga true untuk token `unused`. Untuk token `used`/`locked`, true setelah 50 pengisian token lain sejak pembacaan tertaut atau `lockedAt`. `deleteBlockedReason` tidak lagi memakai `not_locked`; nilai yang mungkin hanya `null` atau `refills_pending`. Kontrak, store, dan klien pratinjau sudah diselaraskan.
