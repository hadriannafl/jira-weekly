# jira-weekly

Tool kecil untuk memantau task Jira yang di-assign ke kamu (To Do, In Progress, Done, Bug), membuat laporan weekly dan timesheet bulanan dalam Excel, plus dashboard web.

## Setup

1. Install Go 1.22 atau lebih baru: `winget install GoLang.Go`
2. Buat API token di https://id.atlassian.com/manage-profile/security/api-tokens (tombol **Create API token**, bukan yang "with scopes").
3. Salin `.env.example` ke `.env`, isi URL Jira, email, token dan data timesheet.
4. Unduh dependency:

   ```
   go mod tidy
   ```

## Weekly report

```
go run .                     # laporan minggu ini
go run . -week -1            # laporan minggu lalu
go run . -interval 15m       # mode monitoring, cek ulang tiap 15 menit
go run . -out D:\laporan     # simpan Excel di folder lain
```

Hasilnya `REPORT_WEEKLY_TASK_YYYYWW.xlsx` (tahun + nomor minggu ISO). Isinya satu sheet bernama tanggal Jumat minggu itu, misalnya "25 September 2026", dengan tiga bagian:

- **LAST WEEK**: task yang kamu ubah di Jira minggu lalu
- **CURRENT WEEK**: task yang kamu ubah minggu ini, ditambah semua yang masih In Progress
- **NEXT WEEK**: task yang masih To Do

Kolom Status berisi 100% untuk Done, 0% untuk To Do, dan nama status Jira (misalnya "QC") untuk sisanya. Angka persen progres dan kolom Note diisi manual.

## Timesheet

```
go run . -timesheet                  # periode berjalan (22 bulan lalu - 21 bulan ini)
go run . -timesheet -period 2026-09  # periode 22 Agustus - 21 September 2026
```

Hasilnya `Timesheet_<nama>_ <mulai> - <selesai>.xlsx` dengan kop surat, satu baris per hari, summary dan tanda tangan.

- Sabtu/Minggu otomatis **Off**, hari kerja **In**.
- **Task List** berisi tiket yang kamu ubah di Jira pada hari itu, **Notes** berisi status tiketnya sekarang.
- **Check In / Check Out** tidak ada di Jira, jadi diisi manual. Libur nasional dan cuti juga diubah manual jadi Off.
- Logo dan tanda tangan diambil dari folder `assets` (`logo.png`, `logo-corner.png`, `signature.png`). Kalau file-nya tidak ada, gambar dilewati.

Weekly dan timesheet membaca riwayat perubahan (changelog) tiket. Tiket yang sangat sering diubah bisa kehilangan riwayat lama, karena Jira hanya mengirim sebagian riwayat terakhir lewat pencarian.

## Dashboard web

```
go run . -web :8080
```

Buka http://localhost:8080. Tampilannya menyesuaikan layar HP maupun desktop dan mengikuti mode gelap/terang perangkat.

- **Weekly**: progres minggu ini, jumlah per kategori, dan daftar task. Di HP, kartu jumlah berfungsi sebagai tab kategori. Di desktop, keempat kategori tampil sebagai kolom. Ada pencarian, navigasi minggu, dan tombol download Weekly/Timesheet.
- **Riwayat** (`/history`): tiket berstatus Done yang pernah di-assign ke kamu, dikelompokkan per bulan selesai, dengan filter 30/90/365 hari dan pencarian.

Timesheet periode lain bisa diunduh lewat `/timesheet?period=2026-09`.

Setiap kartu tiket menampilkan **"dari ..."**, yaitu assignee tiket induk (untuk subtask). Kalau tiketnya bukan subtask atau induknya belum di-assign, yang ditampilkan adalah reporter.

**Pengingat** muncul sebagai banner di dashboard mulai pukul 09:00:

- Weekly Report setiap hari Jumat.
- Timesheet setiap tanggal 21. Kalau tanggal 21 jatuh di Sabtu/Minggu, pengingat muncul di hari Jumat sebelumnya.

Tombol "Sudah dikirim" menyembunyikan banner hari itu di browser tersebut.

Isi `WEB_PASSWORD` (dan `WEB_USER`) di `.env` supaya dashboard minta login. Ini penting kalau dashboard dibuka dari internet.

## Jalankan di server dengan Docker

1. Upload folder project ini ke server, termasuk folder `assets` (tanpa file `.xlsx`).
2. Di server, buat `.env` dari `.env.example` lalu isi.
3. Jalankan:

   ```
   docker compose up -d --build
   ```

Dashboard bisa dibuka di `http://IP-SERVER:8080`. Container juga membuat weekly report setiap 1 jam ke folder `./reports`. Zona waktu diset ke `Asia/Jakarta` supaya penentuan minggu sesuai WIB. Folder `assets` di-mount ke container, jadi logo dan tanda tangan tidak ikut masuk ke image. Untuk mengubah interval, tambahkan `command` di `docker-compose.yml`, contoh:

```yaml
    command: ["-web", ":8080", "-interval", "30m", "-out", "/app/reports"]
```

Cek log: `docker compose logs -f`

## Pengelompokan di dashboard

- Issue bertipe Bug masuk kolom **Bug**, apa pun statusnya.
- Sisanya dikelompokkan berdasarkan status category Jira, jadi status custom seperti "Code Review" atau "QC" tetap masuk In Progress.
