package main

var sqlContekan = []string{
	`
-- Pertanyaan: "siapa nasabah dengan saldo terbanyak?"
SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo
FROM jurnal_transaksi jt JOIN rekening r ON jt.id_rekening = r.id_rekening JOIN nasabah n ON r.id_nasabah = n.id_nasabah
WHERE r.id_status_rekening = 1
GROUP BY n.id_nasabah, n.nama_lengkap ORDER BY total_saldo DESC LIMIT 1;
`,
	`
-- Pertanyaan: "tampilkan mutasi rekening 110000001"
SELECT jt.id_rekening, t.waktu_transaksi, t.deskripsi,
CASE WHEN jt.tipe_dk = 'DEBIT' THEN jt.jumlah ELSE 0 END AS debit,
CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE 0 END AS kredit,
SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) OVER (PARTITION BY jt.id_rekening ORDER BY t.waktu_transaksi, t.id_transaksi) AS saldo_akhir
FROM jurnal_transaksi jt JOIN transaksi t ON t.id_transaksi = jt.id_transaksi
WHERE jt.id_rekening = '110000001'
ORDER BY jt.id_rekening, t.waktu_transaksi, t.id_transaksi;
`,
	`
-- Pertanyaan: "siapa nasabah dengan saldo TABUNGAN terbesar?"
SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_tabungan
FROM jurnal_transaksi jt JOIN rekening r ON jt.id_rekening = r.id_rekening JOIN nasabah n ON r.id_nasabah = n.id_nasabah JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
WHERE r.id_status_rekening = 1 AND mjr.nama_jenis = 'tabungan'
GROUP BY n.id_nasabah, n.nama_lengkap ORDER BY total_saldo_tabungan DESC LIMIT 1;
`,
	`
-- Pertanyaan: "cari nasabah yang namanya ada 'jaya'"
SELECT id_nasabah, nama_lengkap, alamat FROM nasabah WHERE nama_lengkap ILIKE '%jaya%';
`,
	`
-- Pertanyaan: "tampilkan semua transaksi bulan lalu"
SELECT t.waktu_transaksi, t.deskripsi
FROM transaksi t
WHERE t.waktu_transaksi >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '1 month')
  AND t.waktu_transaksi < DATE_TRUNC('month', CURRENT_DATE)
ORDER BY t.waktu_transaksi DESC;
`,
	`
-- Pertanyaan: "berapa jumlah transaksi Budi Santoso (CIF00001)?"
SELECT COUNT(DISTINCT t.id_transaksi) AS jumlah_transaksi
FROM transaksi t JOIN jurnal_transaksi jt ON t.id_transaksi = jt.id_transaksi JOIN rekening r ON jt.id_rekening = r.id_rekening
WHERE r.id_nasabah = 'CIF00001';
`,
}
