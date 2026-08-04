import pandas as pd
from typing import Tuple, List
from src.config import URUTAN_BULAN, DEFAULT_KOLOM_TETAP, NAMA_KOLOM_WAKTU, NAMA_KOLOM_ANGKA

def clean_currency_series(series: pd.Series) -> pd.Series:
    """
    Membersihkan format angka/mata uang secara presisi & aman dari bug 10x desimal:
    - Jika series sudah berupa tipe numerik (int/float), langsung return nilai numerik.
    - Menangani string berformat Indonesia (504.428.654 atau 504.428.654,50).
    - Mencegah string float (contoh: '50442865.4') berubah jadi 10x lebih besar ('504428654').
    """
    if pd.api.types.is_numeric_dtype(series):
        return pd.to_numeric(series, errors='coerce').fillna(0)
    
    def parse_val(val):
        if pd.isna(val):
            return 0.0
        if isinstance(val, (int, float)):
            return float(val)
        
        val_str = str(val).replace('Rp', '').replace('rp', '').replace('RP', '').strip()
        if not val_str or val_str.lower() in ['nan', 'none', 'null', '']:
            return 0.0
        
        if ',' in val_str and '.' in val_str:
            val_str = val_str.replace('.', '').replace(',', '.')
        elif ',' in val_str:
            val_str = val_str.replace(',', '.')
        elif '.' in val_str:
            parts = val_str.split('.')
            if len(parts) > 2:
                val_str = val_str.replace('.', '')
            elif len(parts) == 2:
                if len(parts[1]) == 3 and len(parts[0]) <= 3:
                    val_str = val_str.replace('.', '')
                else:
                    pass
        
        try:
            return float(val_str)
        except ValueError:
            return 0.0

    return series.apply(parse_val)

def detect_columns(df: pd.DataFrame) -> Tuple[List[str], List[str]]:
    """Mendeteksi secara otomatis mana kolom identitas dan mana kolom bulan."""
    df_cols = [str(c).strip() for c in df.columns]
    month_cols = [c for c in df_cols if c.upper() in URUTAN_BULAN]
    id_cols = [c for c in df_cols if c not in month_cols]
    
    if not month_cols:
        raise ValueError("Tidak ditemukan kolom bulan (JANUARI..DESEMBER) dalam file CSV.")
    if not id_cols:
        id_cols = [c for c in DEFAULT_KOLOM_TETAP if c in df_cols]
        
    return id_cols, month_cols

def load_and_clean_data(df: pd.DataFrame, var_name: str = NAMA_KOLOM_WAKTU, value_name: str = NAMA_KOLOM_ANGKA) -> Tuple[pd.DataFrame, List[str]]:
    """Melakukan data cleaning header, unpivot (wide -> long), dan normalisasi nilai."""
    df_clean = df.copy()
    df_clean.columns = df_clean.columns.str.strip()
    
    id_vars, month_cols = detect_columns(df_clean)
    
    df_long = pd.melt(df_clean, id_vars=id_vars, value_vars=month_cols, var_name=var_name, value_name=value_name)
    df_long[var_name] = df_long[var_name].astype(str).str.strip().str.upper()
    df_long[value_name] = clean_currency_series(df_long[value_name])
    
    return df_long, id_vars
