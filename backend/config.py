"""설정 관리"""
import json
import os
from pathlib import Path
from dotenv import load_dotenv

# .env 파일 로드 (backend/.env, 프로젝트 루트 .env, krx/.env.local 순서)
CURRENT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = CURRENT_DIR.parent
KRX_ENV_PATH = PROJECT_ROOT / "krx" / ".env.local"

load_dotenv(CURRENT_DIR / ".env")
load_dotenv(PROJECT_ROOT / ".env")
load_dotenv(KRX_ENV_PATH)


def _load_endpoints() -> dict:
    """KRX API 엔드포인트 설정 로드"""
    raw = os.getenv("KRX_API_ENDPOINTS")
    if raw:
        try:
            parsed = json.loads(raw)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
    return {}


# 네이버 검색 API (뉴스)
NAVER_CLIENT_ID = os.getenv("NAVER_CLIENT_ID")
NAVER_CLIENT_SECRET = os.getenv("NAVER_CLIENT_SECRET")

# 카카오 REST API (웹 검색, daum 소스용)
KAKAO_REST_API_KEY = os.getenv("KAKAO_REST_API_KEY")

# KRX API 설정
KRX_API_KEY = os.getenv("KRX_API_KEY") or os.getenv("KRX_OPENAPI_KEY")
KRX_API_BASE_URL = os.getenv(
    "KRX_API_BASE_URL") or os.getenv("KRX_OPENAPI_BASE_URL")
API_ENDPOINTS = _load_endpoints()
