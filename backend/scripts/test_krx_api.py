#!/usr/bin/env python3
"""
KRX API 연결 테스트: 응답 확인 및 CSV 저장 시 칼럼 누락 검증
"""
import csv
import sys
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(BACKEND_DIR))

from config import API_ENDPOINTS, KRX_API_KEY
from app.services.krx_service import KRXService


def test_fetch_and_save():
    from datetime import datetime, timedelta
    yesterday = (datetime.now() - timedelta(days=1)).strftime("%Y%m%d")
    test_date = "20240215"

    if not KRX_API_KEY:
        print("KRX_API_KEY가 설정되지 않았습니다. .env 또는 krx/.env.local 확인")
        return 1

    print("=" * 60)
    print("KRX API 응답 및 CSV 저장 테스트")
    print("=" * 60)

    result = KRXService.collect_and_save(date=test_date, api_ids=list(API_ENDPOINTS.keys()))
    print("\n[ 시총 1조 이상 필터 적용 ]")

    for api_id in API_ENDPOINTS:
        print(f"\n[ {api_id} ]")
        try:
            out = KRXService.fetch_data(api_id, test_date)
            rows = out.get("data", [])
            if not isinstance(rows, list):
                rows = [rows] if rows else []

            print(f"  API 원본 건수: {len(rows)}")

            if not rows:
                print(f"  -> 데이터 없음. 건너뜀.")
                continue

            all_keys = set()
            for r in rows:
                if isinstance(r, dict):
                    all_keys.update(r.keys())

            r = result.get("results", {}).get(api_id, {})
            if "error" in r:
                print(f"  저장 실패: {r['error']}")
                continue

            saved_path = r.get("path", "")
            count = r.get("count", 0)
            print(f"  저장 건수(시총 1조 이상): {count}건 -> {saved_path}")

            if api_id in ("kospi_daily", "kosdaq_daily"):
                api_dir = Path(BACKEND_DIR) / "lstm" / "data" / api_id
                sample_files = list(api_dir.glob("*.csv"))[:1]
                if sample_files:
                    with open(sample_files[0], encoding="utf-8-sig") as f:
                        reader = csv.DictReader(f)
                        csv_cols = reader.fieldnames or []
                        first_row = next(reader, {})
                    print(f"  CSV 샘플: {sample_files[0].name}")
                    print(f"  CSV 칼럼 수: {len(csv_cols)}")
                    if set(csv_cols) != all_keys:
                        missing = all_keys - set(csv_cols)
                        extra = set(csv_cols) - all_keys
                        if missing:
                            print(f"  칼럼 누락: {missing}")
                        if extra:
                            print(f"  추가 칼럼(기존): {extra}")
                    else:
                        print(f"  칼럼 일치: 모든 칼럼 저장됨")
            else:
                filepath = Path(BACKEND_DIR) / "lstm" / "data" / api_id / f"{test_date}.csv"
                if filepath.exists():
                    with open(filepath, encoding="utf-8-sig") as f:
                        reader = csv.DictReader(f)
                        csv_cols = reader.fieldnames or []
                    print(f"  CSV 칼럼 수: {len(csv_cols)}")
                    if set(csv_cols) != all_keys:
                        missing = all_keys - set(csv_cols)
                        if missing:
                            print(f"  칼럼 누락: {missing}")
                    else:
                        print(f"  칼럼 일치: 모든 칼럼 저장됨")

        except Exception as e:
            print(f"  오류: {e}")
            import traceback
            traceback.print_exc()

    print("\n" + "=" * 60)
    print("테스트 완료")
    return 0


if __name__ == "__main__":
    sys.exit(test_fetch_and_save())
