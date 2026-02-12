"""
KRX API 서비스
"""
from datetime import datetime, timedelta
import httpx
from config import KRX_API_KEY, API_ENDPOINTS


class KRXService:
    """KRX API 서비스 클래스"""
    
    @staticmethod
    def get_endpoints():
        """사용 가능한 API 엔드포인트 목록 반환"""
        return {
            "endpoints": API_ENDPOINTS,
            "available_apis": list(API_ENDPOINTS.keys())
        }
    
    @staticmethod
    def fetch_data(api_id: str, date: str = None):
        """
        KRX API 데이터 조회
        
        Args:
            api_id: API ID
            date: 날짜 (YYYYMMDD 형식, None이면 어제 날짜)
        
        Returns:
            API 응답 데이터
        
        Raises:
            ValueError: API ID가 유효하지 않거나 날짜 형식이 잘못된 경우
            RuntimeError: API 키가 설정되지 않았거나 API 호출 실패
        """
        # API ID 검증
        if api_id not in API_ENDPOINTS:
            raise ValueError(f"알 수 없는 API: {api_id}")
        
        # 날짜 처리
        if not date:
            yesterday = datetime.now() - timedelta(days=1)
            date = yesterday.strftime("%Y%m%d")
        
        # 날짜 형식 검증
        try:
            datetime.strptime(date, "%Y%m%d")
        except ValueError:
            raise ValueError(f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})")
        
        # API 키 확인
        if not KRX_API_KEY:
            raise RuntimeError("API 키가 설정되지 않았습니다. .env 파일에 KRX_API_KEY를 설정해주세요.")
        
        # KRX API 호출
        endpoint = API_ENDPOINTS[api_id]
        headers = {"AUTH_KEY": KRX_API_KEY}
        params = {"basDd": date}
        
        try:
            with httpx.Client(timeout=30.0, follow_redirects=True) as client:
                response = client.get(endpoint["url"], headers=headers, params=params)
                response.raise_for_status()
                data = response.json()
            
            # 응답 데이터 파싱
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
