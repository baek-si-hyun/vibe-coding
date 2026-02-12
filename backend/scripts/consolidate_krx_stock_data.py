#!/usr/bin/env python3
"""
기존 KRX 데이터를 종목별 단일 CSV로 통합합니다.

지원 소스:
1. backend/lstm/data/stk_bydd_trd/data/<종목코드>/*.json
2. backend/lstm/data/ksq_bydd_trd/data/<종목코드>/*.json
3. backend/lstm/data: kospi_daily_*.csv, kosdaq_daily_*.csv (날짜별 플랫 CSV 또는 기존 형식)

출력: backend/lstm/data/kospi_daily/종목명.csv, backend/lstm/data/kosdaq_daily/종목명.csv

변환이 성공한 종목 폴더만 삭제합니다.
"""
import csv
import json
import re
import shutil
from collections import defaultdict
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
LSTM_DIR = BACKEND_DIR / "lstm"
DATA_DIR = LSTM_DIR / "data"
OLD_STK_DATA = LSTM_DIR / "data" / "stk_bydd_trd" / "data"
OLD_KSQ_DATA = LSTM_DIR / "data" / "ksq_bydd_trd" / "data"
OUT_STK_DIR = DATA_DIR / "kospi_daily"
OUT_KSQ_DIR = DATA_DIR / "kosdaq_daily"

NAME_COLS = ("ISU_NM", "isuNm", "itmsNm", "isuKorNm", "종목명", "name")
CODE_COLS = ("ISU_CD", "ISU_SRT_CD", "srtnCd", "단축코드", "종목코드", "code")
DATE_COLS = ("BAS_DD", "basDd", "TRD_DT", "날짜", "일자", "date")


def _safe_filename(name: str) -> str:
    illegal = r'[<>:"/\\|?*\x00-\x1f]'
    s = re.sub(illegal, "_", str(name).strip())
    return s or "unknown"


def _detect_col(cols: list[str], candidates: tuple) -> str | None:
    for c in candidates:
        if c in cols:
            return c
    return None


def _load_csv(path: Path) -> list[dict]:
    rows = []
    try:
        with open(path, encoding="utf-8-sig") as f:
            reader = csv.DictReader(f)
            for r in reader:
                if r:
                    rows.append(r)
    except Exception as e:
        print(f"  경고: {path} 읽기 실패 - {e}")
    return rows


def _load_json(path: Path) -> dict | None:
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception as e:
        print(f"  경고: {path} 읽기 실패 - {e}")
        return None


def _json_to_row(obj: dict) -> dict | None:
    data = obj.get("data") or obj.get("tradeData")
    if not data or not isinstance(data, dict):
        return None
    row = dict(data)
    if "name" in obj and "ISU_NM" not in row:
        row["ISU_NM"] = obj["name"]
    if "code" in obj and "ISU_CD" not in row:
        row["ISU_CD"] = obj["code"]
    if "date" in obj and "BAS_DD" not in row:
        row["BAS_DD"] = obj["date"]
    return row


def _merge_rows(rows: list[dict]) -> list[dict]:
    seen = set()
    out = []
    date_col = None
    for r in rows:
        if not isinstance(r, dict):
            continue
        key = None
        for c in DATE_COLS:
            if c in r and r.get(c):
                key = r.get(c)
                date_col = c
                break
        if key is None:
            key = id(r)
        if key in seen:
            continue
        seen.add(key)
        out.append(r)
    if date_col:
        out.sort(key=lambda x: x.get(date_col, ""))
    return out


def _save_csv(rows: list[dict], out_path: Path) -> bool:
    if not rows:
        return False
    fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
        w.writeheader()
        for r in rows:
            if isinstance(r, dict):
                w.writerow({k: r.get(k, "") for k in fieldnames})
    return True


def consolidate_from_json_folders(delete_after: bool = True) -> tuple[int, int]:
    """JSON 종목 폴더 통합. 성공 시 해당 폴더 삭제."""
    saved_stk, saved_ksq = 0, 0

    for src_dir, out_dir in [(OLD_STK_DATA, OUT_STK_DIR), (OLD_KSQ_DATA, OUT_KSQ_DIR)]:
        if not src_dir.exists():
            continue
        is_stk = out_dir == OUT_STK_DIR

        for code_dir in src_dir.iterdir():
            if not code_dir.is_dir():
                continue
            files = list(code_dir.glob("*.json"))
            if not files:
                continue

            all_rows = []
            for f in files:
                obj = _load_json(f)
                if not obj:
                    continue
                row = _json_to_row(obj)
                if row:
                    all_rows.append(row)

            if not all_rows:
                continue

            merged = _merge_rows(all_rows)
            if not merged:
                continue

            cols = list(merged[0].keys())
            name_col = _detect_col(cols, NAME_COLS) or _detect_col(cols, CODE_COLS)
            if name_col and merged[0].get(name_col):
                filename = _safe_filename(merged[0][name_col]) + ".csv"
            else:
                filename = _safe_filename(code_dir.name) + ".csv"

            out_path = out_dir / filename
            if out_path.exists():
                existing = _load_csv(out_path)
                merged = _merge_rows(existing + merged)

            if _save_csv(merged, out_path):
                if is_stk:
                    saved_stk += 1
                else:
                    saved_ksq += 1
                if delete_after:
                    try:
                        shutil.rmtree(code_dir)
                    except OSError as e:
                        print(f"  경고: {code_dir} 삭제 실패 - {e}")

    return saved_stk, saved_ksq


def consolidate_from_krx_dir() -> tuple[int, int]:
    """lstm/data 내 kospi_daily_*.csv, kosdaq_daily_*.csv 통합 (플랫 파일)"""
    if not DATA_DIR.exists():
        return 0, 0

    stk_by_name: dict[str, list[dict]] = defaultdict(list)
    ksq_by_name: dict[str, list[dict]] = defaultdict(list)

    for p in DATA_DIR.glob("*.csv"):
        name = p.stem.lower()
        if not name.startswith(("kospi_daily_", "kosdaq_daily_")):
            continue
        market = "stk" if "kospi" in name else "ksq"
        rows = _load_csv(p)
        if not rows:
            continue

        cols = list(rows[0].keys())
        name_col = _detect_col(cols, NAME_COLS) or _detect_col(cols, CODE_COLS)
        if not name_col:
            print(f"  건너뜀 (종목 식별 불가): {p.name}")
            continue

        for r in rows:
            key = (r.get(name_col) or "").strip()
            if not key:
                continue
            if market == "stk":
                stk_by_name[key].append(r)
            else:
                ksq_by_name[key].append(r)

    saved_stk = 0
    for name, rows in stk_by_name.items():
        merged = _merge_rows(rows)
        if merged and _save_csv(merged, OUT_STK_DIR / f"{_safe_filename(name)}.csv"):
            saved_stk += 1
    saved_ksq = 0
    for name, rows in ksq_by_name.items():
        merged = _merge_rows(rows)
        if merged and _save_csv(merged, OUT_KSQ_DIR / f"{_safe_filename(name)}.csv"):
            saved_ksq += 1

    return saved_stk, saved_ksq


def main():
    import argparse
    parser = argparse.ArgumentParser(description="KRX 종목별 CSV 통합")
    parser.add_argument("--no-delete", action="store_true", help="변환 후 소스 폴더 삭제하지 않음")
    args = parser.parse_args()

    print("KRX 종목별 CSV 통합")
    print("-" * 40)

    stk1, ksq1 = consolidate_from_json_folders(delete_after=not args.no_delete)
    stk2, ksq2 = consolidate_from_krx_dir()

    total_stk = stk1 + stk2
    total_ksq = ksq1 + ksq2

    print(f"kospi_daily: {total_stk}개 파일 -> {OUT_STK_DIR}")
    print(f"kosdaq_daily: {total_ksq}개 파일 -> {OUT_KSQ_DIR}")
    if not args.no_delete and (stk1 or ksq1):
        print("(변환 완료된 종목 폴더 삭제됨)")

    if total_stk == 0 and total_ksq == 0:
        print("\n처리할 소스 데이터가 없습니다.")
        print("  - backend/lstm/data/stk_bydd_trd/data/<종목코드>/*.json")
        print("  - backend/lstm/data/ksq_bydd_trd/data/<종목코드>/*.json")
        print("  - backend/lstm/data/kospi_daily_*.csv, kosdaq_daily_*.csv (플랫 형식)")


if __name__ == "__main__":
    main()
