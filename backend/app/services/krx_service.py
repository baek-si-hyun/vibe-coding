import csv
import os
from datetime import datetime, timedelta
from pathlib import Path

import httpx

from config import API_ENDPOINTS, KRX_API_KEY

BACKEND_DIR = Path(__file__).resolve().parent.parent.parent
KRX_DATA_DIR = BACKEND_DIR / "lstm" / "data" / "krx"


class KRXService:
    @staticmethod
    def get_endpoints():
        return {
            "endpoints": API_ENDPOINTS,
            "available_apis": list(API_ENDPOINTS.keys())
        }
    
    @staticmethod
    def fetch_data(api_id: str, date: str = None):
        if api_id not in API_ENDPOINTS:
            raise ValueError(f"알 수 없는 API: {api_id}")

        if not date:
            yesterday = datetime.now() - timedelta(days=1)
            date = yesterday.strftime("%Y%m%d")

        try:
            datetime.strptime(date, "%Y%m%d")
        except ValueError:
            raise ValueError(f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})")

        if not KRX_API_KEY:
            raise RuntimeError("API 키가 설정되지 않았습니다. .env 파일에 KRX_API_KEY를 설정해주세요.")

        endpoint = API_ENDPOINTS[api_id]
        headers = {"AUTH_KEY": KRX_API_KEY}
        params = {"basDd": date}
        
        try:
            with httpx.Client(timeout=30.0, follow_redirects=True) as client:
                response = client.get(endpoint["url"], headers=headers, params=params)
                response.raise_for_status()
                data = response.json()
            stock_list = data.get("OutBlock_1", [])
            
            return {
                "basDd": date,
                "fetchedAt": datetime.now().isoformat(),
                "count": len(stock_list) if isinstance(stock_list, list) else 0,
                "data": stock_list
            }
        
        except httpx.HTTPStatusError as e:
            raise RuntimeError(f"HTTP {e.response.status_code} 에러 발생: {e.response.text[:500] if e.response.text else str(e)}")
        except Exception as e:
            raise RuntimeError(f"API 호출 실패: {str(e)}")

    @staticmethod
    def collect_and_save(date: str = None, api_ids: list = None) -> dict:
        date_override = os.getenv("KRX_DATE_OVERRIDE", "").strip()
        if date_override and len(date_override) == 8:
            try:
                datetime.strptime(date_override, "%Y%m%d")
                date = date_override
            except ValueError:
                pass
        if not date:
            yesterday = datetime.now() - timedelta(days=1)
            date = yesterday.strftime("%Y%m%d")
        else:
            try:
                datetime.strptime(date, "%Y%m%d")
            except ValueError:
                raise ValueError(f"날짜 형식 오류 (YYYYMMDD): {date}")

        if not KRX_API_KEY:
            raise RuntimeError("KRX_OPENAPI_KEY(또는 KRX_API_KEY)가 설정되지 않았습니다.")

        api_ids = api_ids or list(API_ENDPOINTS.keys())
        results = {}

        for api_id in api_ids:
            try:
                out = KRXService.fetch_data(api_id, date)
                rows = out.get("data", [])
                if not isinstance(rows, list):
                    rows = [rows] if rows else []

                KRX_DATA_DIR.mkdir(parents=True, exist_ok=True)
                filepath = KRX_DATA_DIR / f"{api_id}_{date}.csv"

                if rows:
                    fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                    with open(filepath, "w", encoding="utf-8", newline="") as f:
                        w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                        w.writeheader()
                        for r in rows:
                            if isinstance(r, dict):
                                w.writerow({k: r.get(k, "") for k in fieldnames})
                    results[api_id] = {"path": str(filepath), "count": len(rows)}
                else:
                    results[api_id] = {"path": str(filepath), "count": 0}
            except Exception as e:
                results[api_id] = {"error": str(e)}

        return {"date": date, "results": results}
