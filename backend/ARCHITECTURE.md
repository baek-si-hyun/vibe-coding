# Flask 백엔드 아키텍처

## 구조

```
backend/
├── app/
│   ├── __init__.py              # Factory 패턴 (create_app)
│   ├── routes/                   # Blueprint 라우트
│   │   ├── __init__.py
│   │   ├── health.py             # 헬스 체크
│   │   ├── krx.py                # KRX API + POST /api/collect
│   │   ├── news.py               # 뉴스 API
│   │   ├── telegram.py           # 텔레그램 저장 데이터 API
│   │   └── bithumb.py            # 빗썸 API
│   ├── services/                 # 비즈니스 로직 레이어
│   │   ├── krx_service.py       # KRX 서비스
│   │   ├── news_service.py      # 뉴스 서비스
│   │   └── bithumb_service.py   # 빗썸 서비스
│   └── utils/                    # 유틸리티
│       └── errors.py             # 에러 핸들러
├── config.py                     # 설정 관리
├── news_crawler.py               # 크롤러 모듈
├── run.py                        # 실행 파일
└── run.sh                        # 실행 스크립트
```

## 아키텍처 패턴

### 1. Factory 패턴
- `app/__init__.py`의 `create_app()` 함수로 애플리케이션 생성
- 테스트와 설정 관리 용이

### 2. Blueprint 패턴
- 라우트를 기능별로 분리
- `health.py`: 헬스 체크
- `krx.py`: KRX API 엔드포인트
- `news.py`: 뉴스 API 엔드포인트

### 3. Service 레이어
- 비즈니스 로직을 라우트에서 분리
- `KRXService`: KRX API 비즈니스 로직
- `NewsService`: 뉴스 크롤링 비즈니스 로직

### 4. 중앙화된 에러 핸들러
- `app/utils/errors.py`에서 공통 에러 처리
- HTTP 상태 코드별 핸들러

## API 엔드포인트

### 헬스 체크
- `GET /` - 서버 상태 확인

### KRX API
- `GET /api/endpoints` - 사용 가능한 API 목록
- `GET /api/<api_id>` - KRX 데이터 조회
- `POST /api/collect` - KRX API 호출 후 CSV 저장 (lstm/data/krx/)

### 뉴스 API
- `GET/POST /api/news` - 뉴스 검색
- `GET/POST /api/news/naver` - 네이버 뉴스
- `GET/POST /api/news/daum` - 다음 뉴스
- `POST /api/news/crawl` - 뉴스 크롤링 및 저장

## 스크립트 (scripts/)

| 스크립트 | 용도 |
|----------|------|
| krx_collect.py | KRX API 수집 → lstm/data/krx/ CSV 저장 |
| crawl_daum_list.py | 뉴스 API 수집 → news_merged.csv |
| chat_export_to_csv.py | ChatExport HTML → telegram_chats CSV 변환 |
| add_keywords_to_csv.py | news/telegram CSV에 keyword 칼럼 추가 |
| preprocess_news_csv.py | news_merged.csv 전처리 (HTML 엔티티 제거) |
| fix_news_dates.py | news 날짜 형식 통일 (일회성) |
| fix_telegram_chats_csv.py | telegram_chats CSV 형식 수정 |

## 실행

```bash
cd backend && python run.py
# 또는 bash run.sh (venv 자동 생성)
```
