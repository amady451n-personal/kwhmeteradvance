# Desain: PWA yang benar-benar bisa dipakai offline

- Tanggal: 2026-11-27 10:07 WIB
- Status: Disetujui 2026-11-27 10:14 WIB (termasuk persetujuan menyimpan DEK di perangkat).
- Ruang lingkup: frontend + service worker. Kontrak API dan backend tidak berubah.

## Intent dan alasan

Aplikasi dipakai dari HP di rumah yang jaringannya bisa putus. Saat putus, pengguna masih ingin melihat saldo kWh terakhir dan lima catatan terakhir, dan tetap bisa mencatat token baru. Token itu boleh tertunda sampai tersambung; pengiriman terjadi di background tanpa membuat pengguna menunggu.

Keadaan sekarang: service worker hanya meng-cache shell dan aset yang kebetulan diminta, data meter tidak disimpan di perangkat, dan antrean `meteran.queue.v1` hanya menampung catatan meter. Setelah reload saat offline, daftar bisa kosong dan input token gagal.

## Tujuan

1. Setelah pernah masuk online dengan PIN, aplikasi bisa dibuka lagi tanpa jaringan, meminta PIN, lalu menampilkan saldo terakhir dan lima catatan terakhir dari salinan perangkat.
2. Form token baru bisa dipakai saat offline. Token tersimpan dengan penanda "tertunda" dan ikut terkirim otomatis saat koneksi kembali.
3. Sinkronisasi berjalan di background; tidak ada layar tunggu, tidak ada tombol "kirim" tambahan, dan input berikutnya tidak diblokir.
4. Data sensitif (kode token, salinan data) tidak tersimpan sebagai teks biasa di `localStorage`.

## Non-goal dan anti-goal

- Menandai token sebagai "dipakai" tidak tersedia saat offline. Aksi itu identik dengan memasukkan sisa kWh terbaru dan tetap memerlukan server.
- Tidak ada PERUBAHAN kontrak OpenAPI. `TokenCreate.id` sudah opsional dan `CreateToken` backend sudah idempoten per ID, jadi antrean token memakai ID buatan klien.
- Tidak ada pengiriman lewat Background Sync API / push saat aplikasi tertutup. Antrean dikirim saat aplikasi dibuka atau saat event `online` terjadi selagi aplikasi terbuka.
- Bukan offline-first penuh: angka server tetap otoritatif, dan salinan perangkat diberi penanda jelas.
- Anti-goal: salinan offline terlihat seolah data terbaru dari server, atau kode token bocor ke penyimpanan polos.

## Keputusan yang sudah dipegang

- **PIN tetap wajib.** Jejak PIN disimpan di perangkat sebagai hash PBKDF2-SHA256 (salt acak 16 byte, 120.000 iterasi, turunan 32 byte), bukan PIN. Jejak ini hanya dipakai untuk memverifikasi PIN saat offline.
- Kunci enkripsi data (`DEK`) adalah byte acak 32 yang dibuat sekali dan disimpan di `localStorage('meteran.key.v1')`. DEK menyandikan salinan data dan antrean token. Alasannya: drain background harus bisa berjalan sebelum PIN diketik saat aplikasi dibuka ulang; bila DEK bergantung pada PIN, antrean tidak bisa terkirim sampai pengguna membuka kunci. PIN yang diganti tidak mengganti DEK, jadi antrean lama tetap terbaca. Ini konsekuensi yang disengaja — sudah disetujui.
- Jejak hanya dibuat setelah `login` atau `setPin` berhasil terhadap server. Bila PIN belum pernah diatur online, aplikasi tidak bisa dibuka offline dan menampilkan pesan bahwa jaringan diperlukan sekali.
- Salinan data dienkripsi AES-GCM dengan DEK. Isinya: halaman pembacaan terakhir yang berhasil dimuat (bukan hanya lima baris), token, dan settings. Kode token hasil input offline ikut terenkripsi di dalam antrean sampai terkirim.
- Salinan diperbarui setiap `init()` berhasil dan setelah mutasi yang sukses, bukan hanya saat login. Kunci DEK dipertahankan di memori selama aplikasi aktif.
- Antrean token memakai skema `meteran.queue.v2` dengan operasi `token` di samping `reading`; migrasi sekali jalan dari `v1` tanpa kehilangan catatan meter yang tertunda. `clearOffline()` saat logout menghapus verifier dan salinan, tetapi mempertahankan DEK dan antrean agar token tertunda tidak hilang.
- Penanda UI: "Data perangkat" untuk salinan offline, "Tertunda" untuk entri yang belum dikirim. Keduanya hilang setelah server membalas.
- Retry background: `online` event + interval saat aplikasi terbuka. Kesalahan jaringan/5xx ditunda dan dicoba lagi. Respons 4xx tidak dicoba lagi: entri diberi penanda "Gagal: <pesan server>", dikeluarkan dari retry otomatis, dan bisa dihapus pengguna dari daftar token.
- Setelah offline unlock berhasil, aplikasi mencoba `init()` sekali di background. Bila berhasil, status offline hilang dan data diperbarui; bila 401, layar kunci muncul; bila tetap gagal, aplikasi tetap memakai salinan lokal.
- Bila PIN cocok tetapi salinan tidak ada atau gagal dibuka, tampilkan keadaan kosong dengan pesan "Belum ada salinan di perangkat. Hubungkan sekali untuk mengisi." — jangan menghapus verifier dan jangan diam-diam membuang data.
- Service worker tetap menangani notifikasi push seperti sekarang; cache API tidak dilakukan di service worker.

## Konsekuensi dan batasan

- Salinan hanya sejumlah data terakhir yang pernah berhasil dimuat. Kalau pengguna belum pernah online di perangkat itu, tidak ada yang bisa ditampilkan.
- PIN yang diganti di perangkat lain membuat jejak lokal usang; setelah login online dengan PIN baru, jejak dan salinan dibuat ulang.
- Tidak ada pembatasan percobaan PIN offline seperti rate limit server; ini konsekuensi dari verifikasi lokal.
- localStorage punya kuota kecil; salinan dibatasi ke ukuran halaman readings yang sudah dipakai sekarang (`limit=500`) dan token (`limit=200`).
- Service worker hanya aktif di build produksi (sudah demikian di `main.ts`).

## Yang diserahkan ke builder

### 1. Jejak PIN + penyimpanan terenkripsi

Buat modul baru di `frontend/src/api/offline.ts`:

- `rememberPin(pin: string): Promise<void>` — buat salt + hash verifier; simpan ke `localStorage('meteran.pin.v1')`. Dipanggil setelah `client.login()` / `client.setPin()` berhasil.
- `ensureDataKey(): Promise<CryptoKey>` — buat DEK sekali lalu simpan di `localStorage('meteran.key.v1')`; dipakai snapshot dan antrean.
- `verifyPin(pin: string): Promise<boolean>` — hitung ulang verifier; hanya menentukan boleh buka atau tidak, tidak menghasilkan kunci data.
- `saveSnapshot(data)` / `loadSnapshot()` — AES-GCM memakai DEK; `localStorage('meteran.snapshot.v1')`; kembalikan `null` bila tidak ada atau gagal dibuka (jangan diam-diam membuang data).
- `clearOffline()` — dipanggil saat logout: hapus verifier + salinan, biarkan DEK + antrean.

### 2. Antrean token

Ubah `frontend/src/api/queue.ts`:

- `Operation = QueuedReading | QueuedToken`; `QueuedToken` berisi `kind: 'token'`, `id`, `input: TokenCreate`, `queuedAt`.
- `enqueueToken(id, input)`; `queueSize()` menghitung keduanya.
- `drainQueue` menerima dua callback (`upsertReading`, `createToken`) dan memproses berurutan. Entri berhasil dihapus satu per satu; jaringan/5xx dicoba lagi; 4xx ditandai `failed` dengan pesan dan tidak dicoba lagi.
- Entri dan salinan disandikan dengan DEK sebelum ditulis ke `localStorage`.
- Migrasi `v1` → `v2` saat `load()`.

### 3. Alur store dan UI

Di `frontend/src/stores/app.ts`:

- `login`/`setPin`: setelah sukses server, panggil `rememberPin`, lalu simpan salinan dari data yang baru dimuat.
- `init`: bila `client.*` gagal karena offline dan ada jejak PIN, jangan langsung mengosongkan data. Biarkan `lock()` meminta PIN; setelah `verifyPin` sukses, muat salinan terenkripsi, set `offline=true`, dan `ready=true`, lalu jadwalkan satu `init()` senyap untuk mencoba tersambung kembali.
- `createToken` baru: di `frontend/src/views/TokensView.vue` dan `frontend/src/components/AddTokenDialog.vue`, bila offline, buat ID klien, `enqueueToken`, dan tampilkan token lokal dengan penanda "tertunda". Bila online, pakai jalur server seperti sekarang. Entri `failed` bisa dihapus dari daftar token.
- Pasang listener `online` dan interval drainQueue di `init`/`main.ts` agar sinkron berjalan tanpa interaksi. `queuedCount` mencakup token.
- `sourceLabel`: bila `offline`, tampilkan "Data perangkat"; bila ada antrean, tampilkan jumlah tertunda. Token yang ditolak server memakai penanda "Gagal".

Di `frontend/src/views/LockView.vue`: saat offline dan verifikasi lokal gagal, tampilkan "PIN salah"; bila jejak belum ada, "Perlu jaringan sekali untuk menyiapkan PIN"; bila PIN cocok tetapi salinan kosong, "Belum ada salinan di perangkat. Hubungkan sekali untuk mengisi."

### 4. Service worker

Di `frontend/public/sw.js`:

- Naikkan versi cache ke `meteran-v2` dan hapus cache lama saat `activate`.
- Precache shell seperti sekarang.
- Untuk GET same-origin di luar `/api/`: aset statis cache-first dengan fallback jaringan; navigasi network-first dengan fallback `/index.html`. Jangan pernah cache respons `/api/*`.
- Pertahankan handler `push` dan `notificationclick` apa adanya.

### 5. Verifikasi

- Build: `docker compose -f compose.dev.yml run --rm --no-deps web sh -c "npm run build"`.
- Uji browser dengan backend mode server yang berjalan di localhost: login + set PIN → matikan jaringan lewat DevTools/CDP → reload → unlock dengan PIN → saldo dan lima baris tampil → tambah token offline → nyalakan jaringan → token tersinkron, penanda hilang, tidak duplikat.
- Uji negatif: PIN salah saat offline tetap terkunci; logout menghapus salinan; kode token tidak ditemukan sebagai teks polos di `localStorage`.
- Catat hasil verifikasi di `docs/ONBOARDING.md` bagian offline.
