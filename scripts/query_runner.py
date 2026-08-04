import sys
import os
import json
import pandas as pd

# Add current directory to path to find helper modules
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from src.cleaner import load_and_clean_data
from src.utils import read_csv_robust

def run_query(file_path: str, pandas_code: str):
    try:
        # Load data
        if file_path.endswith('.xlsx') or file_path.endswith('.xls'):
            df_raw = pd.read_excel(file_path)
        else:
            with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                df_raw = read_csv_robust(f)
            
        df_bersih, id_vars = load_and_clean_data(df_raw)
        
        # Restricted execution context for basic sandboxing
        local_vars = {
            'df': df_bersih,
            'pd': pd
        }
        
        # Builtins restriction
        safe_builtins = {
            "abs": abs, "all": all, "any": any, "bool": bool,
            "dict": dict, "float": float, "int": int, "isinstance": isinstance,
            "len": len, "list": list, "max": max, "min": min,
            "range": range, "round": round, "str": str, "sum": sum,
            "tuple": tuple, "type": type
        }
        
        # Execute code
        # The AI-generated code must save the final answer into the variable 'result'
        exec(pandas_code, {"__builtins__": safe_builtins}, local_vars)
        
        # Retrieve result
        result = local_vars.get('result', None)
        
        # Normalize types for JSON serialization
        if isinstance(result, (pd.Series, pd.DataFrame)) and result.empty:
            return {"status": "success", "result": "TIDAK_ADA_DATA_MATCH", "message": "Tidak ada baris data yang cocok dengan kriteria filter."}

        if hasattr(result, 'item'):
            result = result.item()
            if pd.isna(result):
                result = "DATA_KOSONG_ATAU_NAN"
        elif isinstance(result, pd.Series):
            result = result.fillna(0).to_dict()
        elif isinstance(result, pd.DataFrame):
            result = result.fillna(0).to_dict(orient='records')
            
        return {"status": "success", "result": result}
        
    except Exception as e:
        return {"status": "error", "message": str(e)}
        
def get_df_info(file_path: str):
    try:
        if file_path.endswith('.xlsx') or file_path.endswith('.xls'):
            df_raw = pd.read_excel(file_path)
        else:
            with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                df_raw = read_csv_robust(f)
            
        df_bersih, id_vars = load_and_clean_data(df_raw)
        
        dtypes = {col: str(dtype) for col, dtype in df_bersih.dtypes.items()}
        head = df_bersih.head(3).to_dict(orient='records')
        
        unique_samples = {}
        for col in df_bersih.columns:
            uniques = df_bersih[col].dropna().unique()
            if len(uniques) > 0:
                unique_samples[col] = [str(x) for x in uniques[:10]]
        
        return {"status": "success", "dtypes": dtypes, "head": head, "unique_samples": unique_samples}
    except Exception as e:
        return {"status": "error", "message": str(e)}

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(json.dumps({"status": "error", "message": "Usage: python query_runner.py <file_path> <pandas_code|--info>"}))
        sys.exit(1)
        
    file_path = sys.argv[1]
    pandas_code = sys.argv[2]
    
    if pandas_code == "--info":
        output = get_df_info(file_path)
    else:
        output = run_query(file_path, pandas_code)
        
    print(json.dumps(output))
