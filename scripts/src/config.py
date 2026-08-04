from typing import List

# ==========================================
# KONFIGURASI GLOBAL
# ==========================================

URUTAN_BULAN: List[str] = [
    'JANUARI', 'FEBRUARI', 'MARET', 'APRIL', 'MEI', 'JUNI',
    'JULI', 'AGUSTUS', 'SEPTEMBER', 'OKTOBER', 'NOVEMBER', 'DESEMBER'
]

DEFAULT_KOLOM_TETAP: List[str] = ['ID_TRX_BUNGA', 'ID_KANTOR', 'ID_PINJAMAN', 'JENIS_PINJAMAN']

NAMA_KOLOM_WAKTU: str = 'BULAN'
NAMA_KOLOM_ANGKA: str = 'NILAI_BUNGA'
BATAS_ANOMALI_PERSEN_DEFAULT: float = -10.0
