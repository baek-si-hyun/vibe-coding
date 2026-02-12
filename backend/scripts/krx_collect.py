#!/usr/bin/env python3
import sys
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(BACKEND_DIR))

from config import API_ENDPOINTS, KRX_API_KEY
from app.services.krx_service import KRXService


def main():
    import argparse

    parser = argparse.ArgumentParser(description="KRX Open API 데이터 수집")
    parser.add_argument("--date", "-d", help="날짜 YYYYMMDD (기본: 어제)")
    parser.add_argument("--api", "-a", nargs="+", help="API ID (기본: 전체)")
    args = parser.parse_args()

    if not KRX_API_KEY:
        print("KRX_OPENAPI_KEY가 설정되지 않았습니다. .env 또는 krx/.env.local 확인")
        sys.exit(1)

    api_ids = args.api or list(API_ENDPOINTS.keys())
    print(f"KRX API 수집 대상: {api_ids}")

    try:
        out = KRXService.collect_and_save(date=args.date, api_ids=api_ids)
    except (ValueError, RuntimeError) as e:
        print(f"오류: {e}")
        sys.exit(1)

    print(f"기준일: {out['date']}")
    for api_id, r in out["results"].items():
        if "error" in r:
            print(f"  {api_id}: 실패 - {r['error']}")
        else:
            print(f"  {api_id}: {r['count']}건 -> {r['path']}")


if __name__ == "__main__":
    main()
