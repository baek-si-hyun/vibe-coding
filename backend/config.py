import json
import os
from pathlib import Path
from dotenv import load_dotenv

CURRENT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = CURRENT_DIR.parent
KRX_ENV_PATH = PROJECT_ROOT / "krx" / ".env.local"

load_dotenv(CURRENT_DIR / ".env")
load_dotenv(PROJECT_ROOT / ".env")
load_dotenv(KRX_ENV_PATH)


def _load_endpoints() -> dict:
    raw = os.getenv("KRX_API_ENDPOINTS")
    if raw:
        try:
            parsed = json.loads(raw)
            if isinstance(parsed, dict) and parsed:
                return parsed
        except json.JSONDecodeError:
            pass
    base = os.getenv("KRX_API_BASE_URL") or os.getenv("KRX_OPENAPI_BASE_URL") or "https://data.krx.co.kr/svc/apis"
    base = base.rstrip("/")
    return {
        "kospi_daily": {"url": f"{base}/sto/stk_bydd_trd", "name": "유가증권 일별매매정보"},
        "kosdaq_daily": {"url": f"{base}/sto/ksq_bydd_trd", "name": "코스닥 일별매매정보"},
        "kospi_basic": {"url": f"{base}/sto/stk_isu_base_info", "name": "유가증권 종목기본정보"},
        "kosdaq_basic": {"url": f"{base}/sto/ksq_isu_base_info", "name": "코스닥 종목기본정보"},
    }


NAVER_CLIENT_ID = os.getenv("NAVER_CLIENT_ID")
NAVER_CLIENT_SECRET = os.getenv("NAVER_CLIENT_SECRET")
KAKAO_REST_API_KEY = os.getenv("KAKAO_REST_API_KEY")
KRX_API_KEY = os.getenv("KRX_API_KEY") or os.getenv("KRX_OPENAPI_KEY")
KRX_API_BASE_URL = os.getenv(
    "KRX_API_BASE_URL") or os.getenv("KRX_OPENAPI_BASE_URL")
API_ENDPOINTS = _load_endpoints()
