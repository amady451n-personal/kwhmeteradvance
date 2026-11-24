# PRD: Pencatat Meteran PLN Prabayar

Versi 1.16 | Diperbarui 24 Oktober 2026 | Status: Disetujui | Bahasa antarmuka: Indonesia

## 1. Ringkasan

Aplikasi web progresif (PWA) pribadi untuk mencatat sisa kWh meteran PLN prabayar, menghitung pemakaian harian, mingguan, dan bulanan, memprediksi kapan sisa kWh menyentuh batas buffer, serta merekomendasikan satu kali pembelian token agar sisa kWh tetap di atas buffer sampai tanggal 1 berikutnya. Aplikasi juga melacak token yang dibeli (harga, kWh, status pemakaian) dan menggantikan aplikasi pencatat lama melalui impor data satu kali.

### Konvensi dokumen

Setiap butir diberi label agar jelas statusnya:

- **\[Fakta\]** informasi yang diketahui atau terlihat dari data.
- **\[Keputusan\]** hal yang sudah diputuskan.
- **\[Asumsi\]** dugaan yang dipakai sampai dikoreksi.
- **\[Constraint\]** batasan yang tidak bisa diubah.
- **\[Risiko\]**, **\[Non-goal\]**, **\[Terbuka\]** sesuai namanya.
- Requirement diberi kode FR (fungsional) dan NFR (non-fungsional).

## 2. Problem dan Objective

### Problem

- &#91;Fakta\] Pengguna memakai meteran prabayar dan hanya melihat sisa kWh yang terus turun, tanpa gambaran pemakaian per hari, tren, atau kapan harus membeli token lagi.
- &#91;Fakta\] Pengguna punya pola isi token besar setiap tanggal 1 untuk target 2 bulan. Pada bulan kedua, sekitar tanggal 10, pengguna membeli token untuk isi besar tanggal 1 berikutnya, lalu membeli token tambahan agar sisa kWh tetap di atas buffer sampai isi besar itu dimasukkan (biasanya mulai pukul 06:00 WIB).
- &#91;Fakta\] Pengguna ingin membandingkan harga bayar token dengan kWh yang didapat dan pemakaiannya.
- &#91;Fakta\] Sudah ada aplikasi pencatat yang berjalan, dengan fitur prediksi dasar: tanggal sisa kWh mencapai buffer (dari input harian dan buffer yang diatur), rata-rata kWh per hari dari input harian, dan waktu input terakhir dalam format relatif ("ago"). Aplikasi itu belum memiliki kode token dan status token, dan datanya harus bisa dilanjutkan.

### Objective

1. Pengguna dapat mencatat sisa kWh dalam 15 detik atau kurang.
2. Pengguna melihat pemakaian per hari kalender, lalu agregasi mingguan dan bulanan.
3. Pengguna tahu tanggal sisa kWh diprediksi menyentuh buffer.
4. Pengguna mendapat rekomendasi satu token yang mencukupi sampai tanggal 1 berikutnya.
5. Pengguna melihat harga efektif per kWh dan nilai rupiah pemakaian.
6. Data aplikasi lama berpindah utuh dan aplikasi lama dapat dipensiunkan.

## 3. Pengguna

- &#91;Fakta\] Satu pengguna (pemilik), satu meteran, perangkat utama HP Android, sebagian kemungkinan iOS.
- &#91;Keputusan\] Tanpa akun login. Akses dilindungi satu PIN.
- &#91;Keputusan\] Sistem hanya satu meter. Tidak ada multi-user.

## 4. Scope

### In scope (v1)

Pencatatan sisa kWh, manajemen token dan statusnya, perhitungan pemakaian, prediksi, rekomendasi pembelian, dashboard, laporan PDF, pengingat push, pengaturan, impor sekali dari aplikasi lama, backup database, mode offline untuk input.

### Non-goals

- &#91;Non-goal\] Integrasi dengan PLN, termasuk pengambilan daftar denominasi otomatis (kandidat pasca-v1).
- &#91;Non-goal\] Pembelian token di dalam aplikasi.
- &#91;Non-goal\] OCR atau foto meteran.
- &#91;Non-goal\] Multi-meter dan multi-user.
- &#91;Non-goal\] Pengecekan tarif resmi PLN. Tarif tidak dipakai menghitung.
- &#91;Non-goal\] Ekspor CSV (backup database tetap requirement).
- &#91;Non-goal\] Sinkronisasi berkelanjutan dengan aplikasi lama.
- &#91;Non-goal\] Push notifikasi selain pengingat catat meteran.

## 5. Fakta, Keputusan, dan Asumsi

### Fakta

- Input manual berupa sisa kWh dari meteran, kira-kira sekali sehari di jam tidak tetap, dan boleh bolong 1 sampai 3 hari.
- Kenaikan kWh di meteran sama dengan kWh pada struk top-up.
- Aplikasi lama menyimpan satu tabel dengan kolom id, meter\_name, reading, timestamp, notes, recharge\_amount, cost.
- Contoh data lama: id 6, main, 157.64, 2026-05-11 08:16, tanpa top-up; id 7, main, 163.99, 2026-05-10 16:20, notes "Charged: True", recharge\_amount 133.2, cost 200000.
- id lama tidak berurutan waktu (id 6 lebih baru dari id 7), jadi urutan harus berdasarkan timestamp.
- Data lama berawal 1 Januari 2026 sampai tanggal snapshot saat impor. Semua baris meter\_name "main".
- timestamp lama berzona WIB dan merupakan isian form (bukan waktu otomatis).
- Pada baris top-up, reading adalah sisa kWh sesudah token dimasukkan. cost adalah biaya bayar.
- Aplikasi lama tidak punya kode token, status token, atau nominal terpisah. Kolom notes hanya catatan.
- Server hanya dapat dijangkau dari jaringan lokal atau VPN, dan memiliki akses internet keluar.
- &#91;Fakta\] Aplikasi lama berbasis Python memakai RAM sekitar 27 sampai 28 MB saat idle dan mencapai sekitar 80 MB saat beban tinggi.
- &#91;Fakta\] Server target berarsitektur arm64 dengan RAM total 2 GB. Sebanyak 75% sudah terpakai (termasuk aplikasi lama), sehingga sisa sekitar 512 MB. Aplikasi lama dan baru berjalan bersamaan selama masa paralel.
- &#91;Fakta\] Development dan build dikerjakan di Linux amd64 (WSL), sedangkan produksi berjalan di Linux arm64. Aplikasi lama saat ini di-build silang di amd64 menjadi tar.gz lalu dikirim ke server dengan skrip scp.

### Keputusan

- Pilihan teknologi frontend dan backend (bahasa, framework, pustaka, termasuk pustaka komponen UI untuk Material 3) tidak ditetapkan di PRD dan dipilih di luar PRD, selama memenuhi requirement dan constraint dokumen ini. Yang tetap: SQLite sebagai database, PWA, Web Push, dan pengemasan Docker Compose. Server sudah tersedia.
- Zona waktu WIB. Minggu mulai Senin. Bulan mengikuti kalender. kWh disimpan 2 desimal.
- Buffer adalah sisa kWh minimum, diatur pengguna.
- Prediksi memakai rata-rata pemakaian harian, default 7 hari terakhir (opsi 14 dan 30).
- Rata-rata tertimbang untuk harga efektif dan nilai rupiah pemakaian.
- Golongan tarif/daya adalah pengaturan global, hanya informasi.
- Biaya hanya satu angka: **biaya bayar** (tanpa pemisahan admin/pajak, data lama dan baru diperlakukan sama).
- Aplikasi dijalankan melalui domain dengan HTTPS sebagai syarat PWA.
- Peralihan: impor sekali, paralel seminggu, lalu aplikasi lama dipensiunkan.

### Asumsi

- Pengguna memasukkan sisa kWh sesuai angka yang tampil di meteran.
- Hanya satu top-up yang terjadi di antara dua pembacaan berurutan pada kondisi normal.
- Isi besar tanggal 1 biasanya mulai dimasukkan sekitar pukul 06:00 WIB (jika tidak terlupa), sehingga prediksi dan rekomendasi dihitung sampai jam target tanggal 1 berikutnya (default 06:00 WIB, dapat diatur di Pengaturan).

## 6. User Stories

1. Sebagai pemilik, saya ingin mencatat sisa kWh dengan cepat dan dengan jam yang bisa saya atur, agar pencatatan tetap akurat walau dilakukan terlambat.
2. Sebagai pemilik, saya ingin melihat pemakaian per hari, minggu, dan bulan, agar tahu pola pemakaian saya.
3. Sebagai pemilik, saya ingin tahu tanggal sisa kWh menyentuh buffer, agar tidak kehabisan token.
4. Sebagai pemilik, saya ingin menyimpan token yang saya beli beserta statusnya, agar tidak ada token yang terlupa atau terpakai ganda.
5. Sebagai pemilik, saya ingin rekomendasi satu token yang cukup sampai tanggal 1 berikutnya, agar saya cukup membeli satu kali.
6. Sebagai pemilik, saya ingin tahu harga efektif per kWh dari pembelian saya, agar bisa membandingkan pembelian.
7. Sebagai pemilik, saya ingin pengingat bila belum mencatat hari ini, agar data harian tidak bolong.
8. Sebagai pemilik, saya ingin laporan PDF ringkas, agar bisa dibagikan atau disimpan.
9. Sebagai pemilik, saya ingin memindahkan semua data aplikasi lama, agar riwayat sejak Januari 2026 tidak hilang.
10. Sebagai pemilik, saya ingin mencatat saat tidak ada koneksi, agar pencatatan tidak gagal di luar jaringan lokal/VPN.

## 7. Functional Requirements

### 7.1 Pembacaan meteran

- **FR-01** Form pembacaan berisi timestamp (default waktu sekarang, dapat diubah), sisa kWh, dan catatan opsional.
- **FR-02** Timestamp masa lalu diizinkan, termasuk di luar urutan input. Timestamp masa depan ditolak.
- **FR-03** Pembacaan dapat diedit dan dihapus, kecuali pembacaan yang tertaut ke token historis (terkunci). Setiap perubahan memicu perhitungan ulang semua pemakaian, prediksi, dan status token terkait.
- **FR-04** Jika sisa kWh lebih tinggi dari pembacaan sebelumnya dan tidak ada top-up tercatat di antara keduanya, aplikasi meminta konfirmasi sebelum menyimpan dan tidak menyimpan diam-diam.
- **FR-05** Dua pembacaan dengan timestamp yang sama ditandai sebagai peringatan dan menunggu konfirmasi.
- **FR-06** Nilai sisa kWh negatif atau kosong ditolak.

### 7.2 Token

- **FR-10** Data token: kode token (wajib untuk input baru), kWh didapat, biaya bayar (rupiah), tanggal-jam pembelian, jenis token (Token tambahan atau Token isi besar), nominal token (lihat FR-14), dan status.
- **FR-11** Kode token disimpan terenkripsi dan disembunyikan secara default. Kode tampil hanya setelah pengguna menekan "tampilkan".
- **FR-12** Status token: **Belum dipakai**, **Dipakai**, **Terpakai (terkunci)**.
- **FR-13** Alur status:
  1. Token baru berstatus Belum dipakai.
  2. Saat pengguna menandai token "Dipakai", aplikasi sekaligus meminta input sisa kWh baru sesudah token dimasukkan. Input itu menjadi satu pembacaan tertaut ke token.
  3. Ketika pembacaan berikutnya lebih rendah dari pembacaan pada langkah 2, token otomatis menjadi Terpakai dan terkunci.
  4. Sebelum terkunci, token dapat dibatalkan ke Belum dipakai (pembacaan tertaut ikut dibatalkan setelah konfirmasi). Token yang terkunci menjadi historis dan tidak dapat diedit. Penghapusan token historis bersifat opsional dan baru dapat dilakukan setelah pengguna melakukan 50 kali pengisian token sejak token itu terkunci. Penghapusan membuang kode token dan detailnya, sedangkan ringkasan (kWh, biaya, tanggal) tetap tersimpan dan tetap dihitung dalam rata-rata harga dan pemakaian.
- **FR-14** Nominal token dihitung otomatis sebagai denominasi terdekat dari daftar denominasi di Pengaturan terhadap biaya bayar (contoh: 98.000 menjadi 100.000, 105.000 menjadi 100.000). Nominal hanya informasi (untuk melihat selisih diskon atau biaya). Pengguna dapat menimpa nilainya. Jika biaya bayar berada di luar toleransi ±10% dari semua denominasi, atau daftar denominasi kosong, nominal dikosongkan dan ditandai untuk konfirmasi, tidak ditebak.
- **FR-15** Token berstatus Belum dipakai dihitung sebagai stok untuk prediksi dan rekomendasi. Pengecualian: token berjenis Token isi besar tidak dihitung sebagai stok selama belum dipakai, karena baru dimasukkan ke meteran pada isi besar tanggal 1. Jenis token wajib dipilih saat input token baru dan dapat diubah sebelum token terkunci.

### 7.3 Perhitungan pemakaian

- **FR-20** Pemakaian antara dua pembacaan berurutan (berdasar timestamp) = sisa sebelumnya + kWh top-up di antara keduanya − sisa sekarang.
- **FR-21** Pemakaian antar pembacaan dibagi proporsional waktu ke tiap hari kalender (00:00 sampai 24:00 WIB). Contoh: pembacaan 100,00 kWh pada 10 Okt 20:00 dan 94,00 kWh pada 11 Okt 08:00 memberi 6,00 kWh dalam 12 jam; 2,00 kWh dialokasikan ke 10 Okt dan 4,00 kWh ke 11 Okt.
- **FR-22** Hari tanpa pembacaan sendiri ditandai "estimasi" (nilainya hasil pembagian proporsional).
- **FR-23** Agregasi mingguan (Senin sampai Minggu) dan bulanan (kalender) adalah jumlah pemakaian harian, dengan penanda bila mengandung hari estimasi.
- **FR-24** Nilai rupiah pemakaian = kWh terpakai × harga efektif rata-rata tertimbang.

### 7.4 Harga efektif

- **FR-30** Harga efektif satu pembelian = biaya bayar ÷ kWh didapat.
- **FR-31** Harga efektif rata-rata tertimbang = total biaya bayar ÷ total kWh seluruh pembelian. Nilai ini yang dipakai untuk konversi rupiah dan ditampilkan sebagai "estimasi harga per kWh".
- **FR-32** Daftar pembelian menampilkan perbandingan harga efektif tiap token terhadap rata-rata tertimbang.
- **FR-33** Pembanding tarif: aplikasi menampilkan estimasi harga per kWh dari pembelian (rata-rata tertimbang) berdampingan dengan tarif resmi per kWh yang diisi pengguna di Pengaturan, beserta selisihnya (rupiah dan persen). Tarif resmi hanya informasi pembanding dan tidak dipakai dalam perhitungan pemakaian, prediksi, atau rekomendasi. Aplikasi tidak mengambil tarif dari sumber luar.

### 7.5 Prediksi dan rekomendasi

- **FR-40** Rata-rata pemakaian harian dihitung dari N hari terakhir (default 7, opsi 14 dan 30).
- **FR-40a** Daftar pembacaan menampilkan pembanding per baris: Δ kWh sejak pembacaan sebelumnya, kWh/hari pada interval tersebut, dan indikator arah dibanding interval sebelumnya: "hemat" (kWh/hari turun 10% atau lebih), "boros" (naik 10% atau lebih), atau "tetap" (berubah kurang dari 10%). Interval yang mengandung isi token tidak dibandingkan dan ditandai sebagai baris top-up. Interval pertama sesudah baris top-up juga tidak diberi indikator, karena interval sebelumnya tidak dapat dibandingkan. Δ kWh adalah pemakaian pada interval (bernilai positif), dan kWh/hari adalah Δ kWh dibagi lama interval dalam hari.
- **FR-41** Tanggal sentuh buffer = tanggal ketika sisa kWh terprediksi mencapai buffer, dihitung dari sisa terakhir ditambah Token tambahan yang Belum dipakai, dikurangi rata-rata pemakaian harian, dinyatakan sebagai tanggal dan jam (WIB). Waktu yang sama ditampilkan di aplikasi dan dipakai oleh endpoint homepage. Ditampilkan sebagai "beli paling lambat tanggal X".
- **FR-42** Rekomendasi adalah token tambahan agar sisa kWh bertahan sampai isi besar berikutnya (pada jam target tanggal 1 berikutnya, default 06:00 WIB dan dapat diatur di Pengaturan). Aplikasi tidak membedakan bulan pertama dan kedua dalam siklus, dan rekomendasi boleh muncul kapan pun stok diprediksi kurang.
- **FR-43** Kebutuhan kWh = prediksi pemakaian sampai target + buffer − (sisa terakhir + kWh Token tambahan yang Belum dipakai). Jika hasil nol atau negatif, tidak ada rekomendasi.
- **FR-44** Kebutuhan kWh dikonversi ke rupiah dengan harga efektif tertimbang, lalu aplikasi memilih satu denominasi terkecil dari daftar di Pengaturan yang mencukupi. Hanya satu token yang direkomendasikan. Contoh: kebutuhan Rp150.000 dengan denominasi tersedia 100.000, 200.000 memberi rekomendasi 200.000.
- **FR-45** Jika tidak ada denominasi tunggal yang cukup, aplikasi menampilkan denominasi terbesar disertai peringatan.
- **FR-46** Jika daftar denominasi kosong, rekomendasi hanya menampilkan kebutuhan kWh dan rupiah, tanpa denominasi.
- **FR-47** Jika data kurang untuk prediksi (misalnya kurang dari 2 pembacaan atau belum ada pemakaian), aplikasi menampilkan pesan data belum cukup, bukan angka tebakan.

### 7.6 Dashboard

- **FR-50** Dashboard menampilkan: sisa kWh terakhir, pemakaian hari ini, grafik 24 jam terakhir/harian, ringkasan minggu dan bulan, rata-rata pemakaian harian (kWh/hari), waktu input terakhir dalam format relatif (misalnya "3 jam yang lalu"), tanggal sentuh buffer, rekomendasi pembelian, stok token Belum dipakai, estimasi harga per kWh.
- **FR-50a** Dashboard: setiap bagian analitik (daftar pembacaan, grafik, ringkasan minggu/bulan) memiliki filter rentang tanggal sendiri, dengan default tanggal 1 bulan berjalan sampai hari ini. Pilihan tiap bagian disimpan di browser (localStorage) dan dipulihkan saat aplikasi dibuka lagi. Pilihan ini tidak disimpan di server dan tidak disinkronkan antar perangkat.
- **FR-50b** Waktu input terakhir di dashboard memakai tiga tingkat urgensi berdasarkan lama sejak pembacaan terakhir: netral untuk kurang dari 24 jam; hijau dengan ajakan "catat sekarang" untuk 24 sampai 40 jam; merah dengan peringatan "perlu input!" untuk lebih dari 40 jam. Tampilan ini berlaku di dashboard (bukan push) dan sekaligus menjadi banner input pada FR-51.
- **FR-51** Banner input (mengikuti tiga tingkat urgensi pada FR-50b), peringatan mendekati buffer, dan pengingat top-up tampil di dashboard (bukan push).

### 7.7 Notifikasi

- **FR-60** Pengingat push mencatat meteran pukul 20:00 WIB (dapat diatur), terkirim hanya jika belum ada pembacaan pada hari itu.
- **FR-61** Pengguna mengaktifkan notifikasi lewat tombol sendiri (syarat iOS). Subscription yang kedaluwarsa didaftarkan ulang otomatis bila memungkinkan.

### 7.8 Laporan PDF

- **FR-70** Laporan dibuat di server dan dapat diunduh atau dibagikan dari HP.
- **FR-71** Isi: tanggal laporan, sisa kWh terakhir, pemakaian hari ini, tanggal prediksi menyentuh buffer, estimasi harga per kWh dari pembelian, dan 10 pembacaan meteran terakhir (tanpa baris top-up).

### 7.9 Pengaturan

- **FR-80** Pengaturan: buffer (kWh), jendela rata-rata (7/14/30), golongan tarif/daya (informasi), tarif resmi per kWh (informasi pembanding, diisi pengguna), daftar denominasi token (diisi pengguna), jam target isi besar (default 06:00 WIB), jam pengingat, PIN, aktivasi notifikasi, impor data.
- **FR-81** Daftar denominasi awal berisi satu sampel Rp100.000 yang ditandai "contoh, ganti dengan daftar asli".

### 7.10 Keamanan akses

- **FR-90** Akses ke aplikasi, termasuk semua input, ubah, dan hapus data, dilindungi PIN. Pengecualian: endpoint homepage (FR-110) yang hanya mengembalikan informasi. PIN disimpan sebagai hash, tidak pernah plaintext. PIN minimal 4 karakter. Sesi login berakhir setelah 30 menit tanpa aktivitas, lalu PIN diminta lagi.
- **FR-91** Percobaan PIN salah berulang dibatasi (rate limit).

### 7.11 Offline

- **FR-95** Input pembacaan dan pencatatan token dapat dilakukan tanpa koneksi, disimpan di perangkat, dan otomatis dikirim saat tersambung kembali. Pengguna melihat status antrean.
- **FR-96** Hasil perhitungan resmi ditentukan server setelah sinkronisasi. Nilai sementara saat offline diberi penanda.

### 7.12 Endpoint Homepage

- **FR-110** Aplikasi menyediakan satu endpoint baca-saja `GET /api/homepage` yang mengembalikan ringkasan status dalam JSON, untuk ditampilkan oleh layanan lain. Endpoint ini hanya membaca data dan tidak memerlukan PIN.
- **FR-111** Struktur respons: `label` bernilai "kWh Tracker"; `value` berupa waktu estimated\_depletion dalam epoch detik (angka desimal); `textValue` berupa teks estimated\_depletion; dan `targets` berisi empat item, masing-masing dengan `name`, `value`, dan `formatted`.
- **FR-112** Isi `targets`: `last_input_ago` (jam sejak pembacaan terakhir, 2 desimal, formatted "24.51h ago"); `last_kwh` (sisa kWh terakhir, formatted "182.61 kWh"); `last_kwh_per_day` (rata-rata pemakaian harian sesuai FR-40, formatted "10.24 kWh/day"); `estimated_depletion` (waktu prediksi sisa kWh mencapai buffer sesuai FR-41, value dan formatted berupa teks yang sama).
- **FR-113** Format teks waktu mengikuti contoh: tanggal, bulan tiga huruf, tahun, jam 12 jam AM/PM, dan WIB. Nilai angka dibulatkan 2 desimal, kecuali value utama. Jika data belum cukup untuk prediksi, field yang tidak dapat dihitung bernilai null dan formatted berisi "N/A".
- &#91;Keputusan\] estimated\_depletion adalah waktu sisa kWh mencapai buffer (FR-41), dan nilai yang sama juga ditampilkan di aplikasi.

Contoh respons:

```json
{"label":"kWh Tracker","value":1794351550.049376,"textValue":"11 Nov 2026, 05:59 AM WIB","targets":[{"name":"last_input_ago","value":24.51,"formatted":"24.51h ago"},{"name":"last_kwh","value":182.61,"formatted":"182.61 kWh"},{"name":"last_kwh_per_day","value":10.24,"formatted":"10.24 kWh/day"},{"name":"estimated_depletion","value":"11 Nov 2026, 05:59 AM WIB","formatted":"11 Nov 2026, 05:59 AM WIB"}]}
```

## 8. Impor Data dari Aplikasi Lama

- **FR-100** Impor dilakukan sekali dari halaman Pengaturan dengan mengunggah file SQLite lama. Fitur impor hanya aktif bila database aplikasi baru kosong.
- **FR-101** Alur: unggah, **pratinjau** (tanpa menyimpan), konfirmasi, **commit atomik** (semua berhasil atau tidak sama sekali).
- **FR-102** Pemetaan: setiap baris menjadi satu pembacaan. Baris dengan recharge\_amount terisi juga menjadi satu token berstatus Terpakai: kWh = recharge\_amount, biaya bayar = cost, tanpa kode token, ditandai "dari impor, belum lengkap". Jenis token dikosongkan. reading pada baris itu dipahami sebagai sisa sesudah top-up. notes disalin apa adanya. id lama disimpan sebagai old\_id. Data diurutkan berdasarkan timestamp, bukan id.
- **FR-103** Hanya baris meter\_name "main" yang diimpor. Jumlah baris non-"main" dilaporkan di pratinjau (diharapkan nol).
- **FR-104** Pratinjau mengelompokkan data menjadi: valid, perlu konfirmasi, dan ditolak. Yang ditandai: sisa naik tanpa top-up, timestamp ganda, top-up tanpa biaya atau sebaliknya, nilai kosong atau negatif, dan nominal di luar toleransi ±10% dari daftar denominasi.
- **FR-105** Setelah commit, aplikasi menjalankan pemeriksaan otomatis: jumlah baris, total kWh top-up, dan total biaya harus sama dengan sumber (untuk baris yang diimpor), dan hasilnya ditampilkan.
- **FR-106** Pemulihan bila hasil impor salah dilakukan manual di server (restore backup atau kosongkan database). Aplikasi tidak menyediakan tombol reset. Pengguna tidak boleh menginput apa pun di aplikasi baru sebelum impor.
- **FR-107** Daftar denominasi lengkap diisi di Pengaturan sebelum impor.

## 9. Non-Functional Requirements

- **NFR-01 Performa input:** membuka aplikasi sampai tersimpan ≤ 15 detik untuk pengguna terbiasa.
- **NFR-02 Responsivitas:** dashboard memuat data (sampai satu tahun data) dalam 3 detik pada jaringan lokal.
- **NFR-03 Platform:** Chrome Android terbaru dan Safari iOS 16.4 ke atas. Tampilan mobile-first dengan Material 3 yang tetap wajar di iOS (tanpa meniru kontrol native iOS). Pustaka komponen yang dipakai tidak ditetapkan di PRD.
- **NFR-04 Instal:** dapat dipasang ke Home Screen, dengan ikon dan splash.
- **NFR-05 Keamanan:** hanya HTTPS, kode token terenkripsi saat disimpan, PIN di-hash, tidak ada data sensitif di log.
- **NFR-06 Keandalan data:** backup otomatis database SQLite terjadwal, dengan prosedur restore yang teruji. Salinan backup diunggah ke Google Drive oleh container terpisah memakai rclone, yang hanya dijalankan saat dibutuhkan dan tidak berjalan terus, yaitu setiap 2 hari sekali. Backup dienkripsi sebelum diunggah ke folder khusus, dan berkas lama di Drive dibersihkan berdasarkan umur (bukan sinkronisasi dua arah). Otorisasi memakai OAuth dengan client ID sendiri berstatus Production agar token tidak kedaluwarsa dalam 7 hari. Kegagalan unggah ditampilkan sebagai peringatan di aplikasi. Backup berjalan harian pukul 02:00 WIB dengan retensi 14 backup harian terakhir dan 8 backup mingguan terakhir.
- **NFR-07 Ketepatan:** kWh 2 desimal, rupiah bilangan bulat, perhitungan diuji dengan kasus acceptance di bagian 11.
- **NFR-08 Aksesibilitas:** kontras dan ukuran target sentuh mengikuti panduan Material 3.
- **NFR-09 Bahasa dan waktu:** seluruh teks Bahasa Indonesia, semua waktu WIB.
- **NFR-10 Kualitas kode:** seluruh kode (frontend dan backend) mematuhi aturan SonarQube dan lolos quality gate-nya. Profil aturan dan quality gate yang dipakai adalah bawaan Sonar way.
- **NFR-11 Keterbacaan oleh agentic coding:** setiap file kode ringkas, berfokus pada satu tanggung jawab, dan mudah dibaca serta diubah oleh agentic coding. Setiap file kode maksimal 500 baris.
- **NFR-12 Pengemasan:** output aplikasi berupa Docker Compose. Satu perintah `docker compose up` menjalankan seluruh komponen sebagai satu container utama (frontend, backend beserta penjadwal notifikasi, dan engine database) ditambah satu container pengunggah backup yang hanya dijalankan saat dibutuhkan. HTTPS dihentikan di luar Docker Compose, dan compose tidak mengelola sertifikat. File SQLite di-mount dari host, dan backup berada di volume persisten. Konfigurasi dan rahasia (misalnya kunci VAPID dan kunci enkripsi kode token) diberikan lewat environment variable, bukan ditulis di kode.
- **NFR-13 Penggunaan memori:** penggunaan RAM seluruh komponen harus sangat kecil, termasuk saat membuat laporan PDF dan saat pengunggahan backup. Sasaran: total seluruh container di bawah 15 MB saat idle, atau lebih kecil jika memungkinkan. Batas saat puncak: maksimal 60 MB untuk container utama, dan maksimal 60 MB untuk container pengunggah backup yang dihitung sendiri (kedua angka tidak dijumlahkan menjadi satu batas). Pengukuran memakai docker stats, sama seperti angka aplikasi lama, dan sasaran idle dihitung dari container yang berjalan terus. Pemilihan teknologi harus memenuhi batas ini. Angka pada NFR ini adalah sasaran perencanaan yang bersifat longgar: nilai final ditetapkan dari hasil pengukuran docker stats di server arm64 saat development, lalu PRD direvisi sesuai hasil itu. Sebagai pembanding, aplikasi lama memakai sekitar 27 sampai 28 MB saat idle dan sekitar 80 MB saat beban tinggi.
- **NFR-14 Platform target dan build:** image dibangun untuk linux/arm64 dan di-build di mesin development Linux amd64 (WSL) lewat cross build, lalu dijalankan di produksi Linux arm64. Proses build tidak pernah dijalankan di server.
- **NFR-15 Skrip rilis:** satu skrip dengan dua opsi. Opsi 1 (dijalankan di mesin development): build untuk arm64, hasilkan file tar.gz, lalu kirim ke server lewat scp. Opsi 2 (dijalankan di server): muat image dari file tar.gz lalu jalankan Docker Compose. Server hanya menjalankan Docker Compose tanpa proses build. Kode dan konfigurasi dikirim lewat git (push dari mesin amd64, pull di server arm64), sedangkan file tar.gz image dikirim lewat scp. Skrip yang sama dijalankan di dua tempat: opsi build di amd64 dan opsi deploy di arm64.

## 10. Constraints

- &#91;Constraint\] Aplikasi harus dijalankan melalui domain dengan HTTPS. Ini syarat PWA (pemasangan, service worker, Web Push); tanpa itu aplikasi tidak berfungsi sempurna.
- &#91;Constraint\] Server hanya diakses dari jaringan lokal atau VPN dengan domain yang sama. Server perlu akses internet keluar untuk push.
- &#91;Constraint\] Web Push di iOS hanya berfungsi bila PWA terpasang di Home Screen (iOS 16.4 ke atas).
- &#91;Constraint\] SQLite memerlukan satu server dengan disk persisten.
- &#91;Constraint\] Data PWA, service worker, dan langganan push terikat ke alamat.
- &#91;Constraint\] Token PLN spesifik untuk satu meter (sistem satu meter, tidak ada pemisahan antar meter).
- &#91;Constraint\] Aplikasi berjalan di server arm64, jadi semua image yang dipakai harus tersedia untuk linux/arm64.
- &#91;Constraint\] Server tidak dipakai untuk proses build karena berisiko membebani server. Server hanya memuat image dan menjalankan Docker Compose.
- &#91;Constraint\] Sisa RAM server sekitar 512 MB dan dipakai bersama aplikasi lama selama masa paralel.

## 11. Acceptance Criteria

**AC-01 (pembacaan baru)** Diberikan pembacaan terakhir 100,00 kWh, ketika pengguna menyimpan 94,00 kWh dengan jam sekarang, maka pembacaan tersimpan dan pemakaian muncul di dashboard dalam satu langkah. Waktu input terakhir tampil dalam format relatif (misalnya "baru saja", "3 jam yang lalu").

**AC-02 (pembagian proporsional)** Diberikan 100,00 kWh pada 10 Okt 20:00 dan 94,00 kWh pada 11 Okt 08:00, maka 10 Okt mendapat 2,00 kWh dan 11 Okt mendapat 4,00 kWh. Hari tanpa pembacaan sendiri bertanda "estimasi".

**AC-03 (pembacaan di luar urutan)** Diberikan pembacaan 10 Okt dan 12 Okt, ketika pengguna menambah pembacaan 11 Okt, maka pemakaian 10 sampai 12 Okt dihitung ulang dan total tetap konsisten.

**AC-04 (kenaikan tanpa top-up)** Diberikan pembacaan terakhir 94,00, ketika pengguna menyimpan 120,00 tanpa token terkait, maka aplikasi meminta konfirmasi dan tidak menyimpan sebelum pengguna menyetujui.

**AC-05 (waktu masa depan)** Ketika timestamp pembacaan lebih lambat dari waktu sekarang, maka penyimpanan ditolak dengan pesan jelas.

**AC-06 (alur token)** Diberikan token Belum dipakai 133,20 kWh, ketika pengguna menandai Dipakai dan memasukkan sisa 173,20 kWh, maka status menjadi Dipakai. Ketika pembacaan berikutnya 165,00 kWh disimpan, maka status menjadi Terpakai dan terkunci, dan tidak dapat diedit atau dibatalkan.

**AC-07 (pembatalan)** Diberikan token berstatus Dipakai, ketika pengguna membatalkan dan mengonfirmasi, maka token kembali Belum dipakai dan pembacaan tertaut dihapus.

**AC-08 (nominal otomatis)** Diberikan denominasi 20.000, 50.000, 100.000, 200.000, ketika biaya bayar 98.000 atau 105.000, maka nominal terisi 100.000. Ketika biaya bayar 150.000 (di luar ±10% dari semua denominasi), maka nominal dikosongkan dan ditandai konfirmasi.

**AC-09 (harga efektif)** Diberikan pembelian A: Rp200.000 untuk 133,2 kWh dan B: Rp100.000 untuk 64,0 kWh, maka harga efektif A sekitar Rp1.501,5, B sekitar Rp1.562,5, dan rata-rata tertimbang sekitar Rp1.521,7 per kWh (total Rp300.000 ÷ 197,2 kWh).

**AC-10 (rekomendasi)** Diberikan kebutuhan kWh yang setara Rp150.000, denominasi 100.000 dan 200.000, maka rekomendasi adalah satu token 200.000. Ketika kebutuhan melebihi denominasi terbesar, maka tampil denominasi terbesar beserta peringatan. Ketika denominasi kosong, maka tampil kebutuhan kWh dan rupiah saja.

**AC-11 (tanpa kebutuhan)** Diberikan stok (sisa + Token tambahan Belum dipakai) sudah mencukupi pemakaian sampai tanggal 1 berikutnya ditambah buffer, maka tidak ada rekomendasi pembelian ditampilkan.

**AC-12 (isi besar bulan berurutan)** Diberikan tanggal 2 setelah isi besar tanggal 1 dan stok tetap kurang untuk tanggal 1 berikutnya, maka rekomendasi muncul sama seperti hari lain. Diberikan token berjenis Token isi besar berstatus Belum dipakai, maka kWh-nya tidak dihitung sebagai stok dan rekomendasi Token tambahan tetap muncul bila sisa tidak cukup sampai tanggal 1 berikutnya. Token berjenis Token tambahan berstatus Belum dipakai tetap dihitung sebagai stok.

**AC-13 (data kurang)** Diberikan hanya satu pembacaan, maka endpoint homepage mengembalikan field yang tidak dapat dihitung bernilai null dengan formatted "N/A", dan prediksi serta rekomendasi menampilkan "data belum cukup".

**AC-14 (pengingat)** Diberikan belum ada pembacaan hari ini, ketika pukul 20:00 WIB, maka push terkirim. Diberikan sudah ada pembacaan hari ini, maka push tidak terkirim.

**AC-15 (offline)** Diberikan HP tanpa koneksi, ketika pengguna menyimpan pembacaan, maka data tersimpan lokal dengan penanda antrean dan terkirim otomatis setelah tersambung tanpa duplikasi.

**AC-16 (PIN)** Tanpa PIN yang benar, tidak ada data yang dapat dibaca atau diubah. Percobaan salah berulang diperlambat atau diblokir sementara.

**AC-17 (laporan PDF)** Ketika pengguna meminta laporan, maka PDF berisi seluruh item pada FR-71 dan dapat diunduh atau dibagikan dari HP.

**AC-18 (impor)**

- Impor tidak tersedia bila database sudah berisi data.
- Pratinjau menampilkan jumlah valid, perlu konfirmasi, ditolak, dan jumlah baris non-"main".
- Commit gagal di tengah jalan tidak meninggalkan data sebagian.
- Setelah commit, jumlah baris, total kWh top-up, dan total biaya sama dengan sumber.
- Urutan data ditentukan oleh timestamp, dan old\_id tersimpan.
- Token hasil impor berstatus Terpakai, tanpa kode, dan ditandai belum lengkap.

**AC-19 (backup)** Backup otomatis terjadwal berjalan dan prosedur restore pernah diuji pada salinan. Salinan terenkripsi muncul di Google Drive, dan kegagalan unggah tampil sebagai peringatan.

**AC-20 (tanggal sentuh buffer)** Diberikan sisa 50,00 kWh, buffer 10,00 kWh, rata-rata pemakaian 8,00 kWh/hari, dan tidak ada token Belum dipakai yang dihitung sebagai stok, maka tanggal sentuh buffer adalah 5 hari dari waktu pembacaan ((50 − 10) ÷ 8), tampil sebagai tanggal kalender di dashboard ("beli paling lambat tanggal X") dan di laporan PDF. Ketika pembacaan baru mengubah sisa atau rata-rata, tanggal diperbarui. Ketika sisa sudah sama dengan atau di bawah buffer, dashboard menampilkan peringatan bahwa buffer sudah tercapai.

**AC-21 (kualitas kode)** Analisis SonarQube atas seluruh kode lolos quality gate bawaan Sonar way, dan setiap file kode maksimal 500 baris.

**AC-22 (Docker Compose)** Diberikan mesin dengan Docker dan konfigurasi yang diperlukan, ketika `docker compose up` dijalankan, maka aplikasi dapat diakses. Data tetap ada setelah container dihentikan dan dibuat ulang.

**AC-23 (endpoint homepage)** Diberikan data cukup untuk prediksi, ketika klien memanggil `GET /api/homepage`, maka respons berisi label "kWh Tracker", value epoch detik dari estimated\_depletion, textValue, dan empat target (last\_input\_ago, last\_kwh, last\_kwh\_per\_day, estimated\_depletion) yang nilainya sama dengan dashboard. Contoh format: sisa 182,61 menjadi "182.61 kWh", rata-rata 10,24 menjadi "10.24 kWh/day", 24,51 jam sejak input terakhir menjadi "24.51h ago".

**AC-24 (jam target)** Ketika pengguna mengubah jam target di Pengaturan, maka prediksi dan rekomendasi memakai jam baru. Tanpa perubahan, jam target adalah 06:00 WIB.

**AC-25 (PIN dan sesi)** PIN kurang dari 4 karakter ditolak. Sesi yang tidak ada aktivitas lebih dari 30 menit meminta PIN lagi.

**AC-26 (token historis)** Token yang terkunci tidak dapat diedit. Opsi hapus hanya tersedia setelah 50 kali pengisian token sejak token itu terkunci. Setelah dihapus, kode token hilang, sedangkan rata-rata harga dan pemakaian tidak berubah. Pembacaan yang tertaut ke token historis tidak dapat diedit atau dihapus.

**AC-27 (pembanding tarif)** Diberikan tarif resmi Rp1.500 per kWh di Pengaturan dan harga efektif tertimbang Rp1.521,7 per kWh, maka aplikasi menampilkan selisih Rp21,7 (sekitar 1,4%). Nilai tarif resmi tidak mengubah hasil pemakaian, prediksi, maupun rekomendasi.

**AC-40a (pembanding interval)** Diberikan dua pembacaan berurutan tanpa isi token di antaranya, ketika kWh/hari interval kedua turun 10% atau lebih dari interval pertama, maka baris kedua menampilkan indikator "hemat" beserta Δ kWh dan kWh/hari intervalnya. Interval yang mengandung isi token ditandai sebagai baris top-up dan tidak diberi indikator. Interval pertama sesudah baris top-up juga tidak diberi indikator.

**AC-50a (filter tersimpan)** Diberikan pengguna mengubah rentang grafik ke 7 hari terakhir, ketika aplikasi dibuka ulang di perangkat yang sama, maka grafik langsung memakai rentang 7 hari terakhir tanpa diatur ulang, sementara bagian lain tetap memakai rentangnya masing-masing.

**AC-50b (urgensi input terakhir)** Diberikan pembacaan terakhir 30 jam lalu, ketika dashboard ditampilkan, maka waktu input terakhir berwarna hijau dengan ajakan "catat sekarang". Ketika pembacaan terakhir 48 jam lalu, maka berwarna merah dengan peringatan "perlu input!". Ketika kurang dari 24 jam, maka tampil netral.

**AC-28 (penggunaan memori)** Diberikan aplikasi berjalan normal, ketika penggunaan RAM seluruh container diukur dengan docker stats saat idle, saat membuat PDF, dan saat backup diunggah, maka container utama berada di bawah 15 MB saat idle (sasaran) dan tidak melebihi 60 MB saat puncak, sedangkan container pengunggah backup tidak melebihi 60 MB. Hasil pengukuran saat development menjadi dasar penetapan angka final.

**AC-29 (rilis tanpa build di server)** Diberikan kode di repositori, ketika opsi 1 skrip rilis dijalankan di mesin amd64, maka dihasilkan file tar.gz berisi image linux/arm64 dan file itu terkirim ke server. Ketika opsi 2 dijalankan di server, maka image dimuat dan aplikasi berjalan tanpa ada proses build di server.

## 12. Success Metrics

- Pencatatan meteran selesai ≤ 15 detik.
- Kesalahan prediksi tanggal sentuh buffer ≤ ±1 hari untuk horizon 7 hari.
- Sisa kWh tidak pernah di bawah buffer sebelum tanggal 1 berikutnya.

### Kriteria lolos paralel (aplikasi lama boleh dipensiunkan)

1. Jumlah baris dan total kWh/biaya hasil impor sama dengan sumber.
2. Pemakaian harian aplikasi baru cocok dengan perhitungan pemilik atau laporan PDF lama, dan prediksi tanggal sentuh buffer dapat dibandingkan dengan prediksi aplikasi lama.
3. Tujuh hari input paralel berjalan tanpa error.

## 13. Rencana Peralihan

1. Aplikasi baru dikembangkan dan diuji dengan data uji, sementara aplikasi lama tetap dipakai.
2. Saat dinyatakan siap, pemilik mengisi daftar denominasi lengkap di Pengaturan, lalu menjalankan impor dari snapshot SQLite aplikasi lama.
3. Paralel selama seminggu, input manual di kedua aplikasi.
4. Jika kriteria lolos terpenuhi, aplikasi lama dipensiunkan.

Rentang data lama: 1 Januari 2026 sampai tanggal snapshot saat impor.

## 14. Risiko

| Risiko | Dampak | Mitigasi |
| --- | --- | --- |
| Push tidak tepat waktu atau tertunda penghemat baterai Android | Pengingat telat | Banner dashboard sebagai cadangan |
| Push iOS hanya jalan bila PWA terpasang | Pengingat tidak masuk di iOS | Panduan instal, tombol aktivasi manual |
| Pengingat terkirim walau input offline belum tersinkron | Pengingat tidak relevan | Diterima, risiko kecil |
| Input di luar VPN tertahan di HP | Data terlambat | Antrean offline dan indikator |
| Data lama campuran (cost bisa tanpa admin) dan tanpa kode token | Harga efektif data lama sedikit lebih rendah dari kenyataan | Risiko diterima, penanda "belum lengkap" |
| Input bolong lebih dari 3 hari | Prediksi kurang akurat | Penanda hari estimasi, pesan data kurang |
| Satu file SQLite | Data hilang bila backup gagal | NFR-06, uji restore |
| PIN bocor | Kode token belum dipakai terlihat | Kode tersembunyi, rate limit, akses hanya lokal/VPN |
| Aplikasi tidak dijalankan melalui domain HTTPS | PWA dan push gagal | Pastikan HTTPS aktif sebelum dipakai |
| Impor salah commit | Data keliru | Pratinjau, atomik, backup sebelum impor, pemulihan manual |
| Rekomendasi muncul di hari isi besar tanggal 1 | Dianggap mengganggu | Diterima (bisa diabaikan) |

## 15. Keputusan Terbuka

- Tanggal snapshot impor (belum dapat ditentukan karena development belum dimulai).

## 16. Riwayat Versi

Perubahan setelah rilis 1.0 dicatat sebagai versi minor (1.1, 1.2, dan seterusnya).

| Versi | Tanggal | Perubahan |
| --- | --- | --- |
| 1.0 | 24 Okt 2026 | Rilis awal. Termasuk pola pembelian token (isi besar dan token tambahan), jenis token, dan target prediksi pukul 06:00 WIB. |
| 1.1 | 24 Okt 2026 | Fitur prediksi aplikasi lama (tanggal sentuh buffer, rata-rata kWh per hari, waktu input terakhir format relatif) dicatat sebagai baseline. Dashboard menampilkan rata-rata harian dan waktu input terakhir. Perbandingan prediksi dengan aplikasi lama masuk kriteria paralel. Daftar non-goal dirapikan. |
| 1.2 | 24 Okt 2026 | Ditambah NFR kualitas kode (SonarQube), keterbacaan oleh agentic coding, dan pengemasan Docker Compose. Ditambah acceptance criteria prediksi tanggal sentuh buffer (AC-20). |
| 1.3 | 24 Okt 2026 | Diputuskan: SonarQube memakai profil dan quality gate bawaan Sonar way, setiap file kode maksimal 500 baris, Docker Compose berisi beberapa container dengan HTTPS dihentikan di luar compose. |
| 1.4 | 24 Okt 2026 | Diputuskan: Vuetify 4 untuk Material 3, fpdf2 untuk PDF, PIN minimal 4 karakter dengan sesi maksimal 30 menit, token terkunci menjadi historis (hapus opsional setelah 50 pengisian), jam target isi besar dapat diatur (default 06:00 WIB). Ditambah pembanding tarif resmi (FR-33) dan endpoint /api/homepage (FR-110 sampai FR-113). |
| 1.5 | 24 Okt 2026 | Diputuskan: sesi PIN berakhir setelah 30 menit tanpa aktivitas; PIN melindungi input, ubah, dan hapus, sedangkan endpoint homepage hanya-baca tanpa PIN; estimated\_depletion adalah waktu mencapai buffer dan juga tampil di aplikasi; salinan backup diunggah ke Google Drive. |
| 1.6 | 24 Okt 2026 | Diputuskan: unggah backup ke Google Drive lewat container rclone (terenkripsi, OAuth Production); token historis yang dihapus tetap dihitung dalam rata-rata harga dan pemakaian. |
| 1.7 | 24 Okt 2026 | Diputuskan: backup harian 02:00 WIB dengan retensi 14 harian dan 8 mingguan; pembacaan tertaut token historis tidak dapat diedit atau dihapus; endpoint homepage mengembalikan null dan "N/A" saat data belum cukup. |
| 1.8 | 24 Okt 2026 | Ditambah pembanding interval pada daftar pembacaan (FR-40a), filter rentang tanggal per bagian dashboard yang tersimpan di browser (FR-50a), dan tiga tingkat urgensi waktu input terakhir (FR-50b), beserta AC-40a, AC-50a, AC-50b. |
| 1.9 | 24 Okt 2026 | Diputuskan: banner input FR-51 mengikuti tiga tingkat urgensi FR-50b; interval pertama sesudah baris top-up tidak diberi indikator; definisi Δ kWh dan kWh/hari interval pada FR-40a. |
| 1.10 | 24 Okt 2026 | Pilihan teknologi frontend dan backend dikeluarkan dari PRD dan dipilih di luar PRD. Pilihan pustaka pada baris 1.4 tidak berlaku lagi. Ditambah NFR-13 (penggunaan RAM sangat kecil) dan AC-28. |
| 1.11 | 24 Okt 2026 | Penegasan: pustaka komponen UI untuk Material 3 dipilih di luar PRD. Material 3 tetap menjadi tuntutan tampilan (NFR-03, NFR-08). |
| 1.12 | 24 Okt 2026 | Ditambah fakta RAM aplikasi lama dan server target (arm64, sisa RAM sekitar 512 MB), constraint build tidak di server dan image arm64, NFR-14, NFR-15, dan AC-29. |
| 1.13 | 24 Okt 2026 | Diputuskan: development dan build di WSL amd64, produksi di arm64; kode lewat git push dan pull, image tar.gz lewat scp; satu skrip dijalankan di dua tempat (build di amd64, deploy di arm64). Sasaran RAM seluruh container di bawah 15 MB saat idle. |
| 1.14 | 24 Okt 2026 | Diputuskan: RAM diukur dengan docker stats, batas puncak maksimal 60 MB; satu container utama berisi frontend, backend, dan engine database dengan file SQLite di-mount dari host (menggantikan keputusan beberapa container pada 1.3); container rclone hanya dijalankan saat dibutuhkan. |
| 1.15 | 24 Okt 2026 | Diputuskan: container pengunggah backup dijalankan setiap 2 hari sekali, sedangkan backup lokal tetap harian pukul 02:00 WIB. |
| 1.16 | 24 Okt 2026 | Diputuskan: batas RAM puncak dihitung per container (utama maksimal 60 MB, pengunggah backup maksimal 60 MB); angka RAM bersifat sasaran longgar dan dikonfirmasi lewat pengukuran saat development. |
