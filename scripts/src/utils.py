import pandas as pd
import re

def format_rupiah(val: float) -> str:
    """Format angka numerik ke format Rupiah Indonesia (contoh: Rp 504.428.654)."""
    if pd.isna(val):
        return "Rp 0"
    return f"Rp {val:,.0f}".replace(',', '.')

def format_persen(val: float, with_sign: bool = False) -> str:
    """Format angka numerik ke format persen (contoh: 12.34% atau +12.34%)."""
    if pd.isna(val):
        return "0.00%"
    sign = "+" if with_sign and val > 0 else ""
    return f"{sign}{val:.2f}%"

def format_singkat_indonesia(val: float) -> str:
    """Format angka numerik ke singkatan Bahasa Indonesia (Rb, Jt, M, T)."""
    if pd.isna(val) or val == 0:
        return "0"
    abs_val = abs(val)
    sign = "-" if val < 0 else ""
    if abs_val >= 1e12:
        res = f"{abs_val / 1e12:.2f}".rstrip('0').rstrip('.')
        return f"{sign}{res} T"
    elif abs_val >= 1e9:
        res = f"{abs_val / 1e9:.2f}".rstrip('0').rstrip('.')
        return f"{sign}{res} M"
    elif abs_val >= 1e6:
        res = f"{abs_val / 1e6:.2f}".rstrip('0').rstrip('.')
        return f"{sign}{res} Jt"
    elif abs_val >= 1e3:
        res = f"{abs_val / 1e3:.2f}".rstrip('0').rstrip('.')
        return f"{sign}{res} Rb"
    else:
        return f"{sign}{abs_val:g}"

def read_csv_robust(uploaded_file) -> pd.DataFrame:
    """Membaca file CSV secara toleran dengan otomatis mendeteksi pemisah (; atau ,)."""
    try:
        df = pd.read_csv(uploaded_file, sep=';')
        if len(df.columns) == 1:
            uploaded_file.seek(0)
            df = pd.read_csv(uploaded_file, sep=',')
    except Exception:
        uploaded_file.seek(0)
        df = pd.read_csv(uploaded_file, sep=None, engine='python')
    return df

def natural_sort_key(val):
    """
    Kunci pengurutan alami (Natural Sorting Key) presisi tinggi untuk kode cabang/string.
    Mengurutkan angka secara numerik ('1', '2', ..., '10', '11') maupun string berpola ('Cabang 1', 'Cabang 2', ..., 'Cabang 10').
    """
    s = str(val).strip()
    return tuple(int(text) if text.isdigit() else text.lower() for text in re.split(r'(\d+)', s))
