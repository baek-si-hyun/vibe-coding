# Backend Go

기존 Python/Flask 백엔드를 대체하는 Go 통합 서버입니다.

## Architecture

일반적인 Go 서비스 구조로 정리했습니다.

- `cmd/backend`: 통합 API 서버 엔트리포인트
- `cmd/newsapi`: 뉴스 전용 엔트리포인트
- `internal/config`: 환경변수/엔드포인트 설정 로드
- `internal/app`: 서비스 의존성 조합(bootstrap)
- `internal/server`: 라우터와 HTTP 핸들러 조합
- `internal/server/handlers`: 도메인별 HTTP 핸들러
- `internal/httpx`: 공통 HTTP 유틸(JSON 응답, CORS, 요청 파싱)
- `internal/news`, `internal/krx`, `internal/quant`, `internal/dart`, `internal/bithumb`, `internal/telegram`: 도메인 서비스

흐름은 `cmd -> app -> server/router -> handlers -> domain service` 입니다.  
설정(`config`)과 HTTP 공통 관심사(`httpx`)를 도메인 밖으로 분리해서 모듈 결합도를 줄였습니다.

## Endpoints

- `GET /` (health)
- `GET /api/endpoints`
- `GET /api/files`
- `GET /api/{api_id}`
- `POST /api/collect`
- `POST /api/collect/resume`
- `GET /api/bithumb/screener`
- `GET /api/telegram/chat-rooms`
- `GET /api/telegram/items`
- `POST /api/news/crawl/resume`
- `GET /api/news/files`
- `GET /api/news/items`
- `POST /api/dart/export/financials`

## LSTM-Assisted Quant Ranking

`/api/quant/rank` and `/api/quant/report` can optionally blend a precomputed LSTM prediction file into the existing next-day focused score. The integration is conservative by design:

- If the prediction file is missing, ranking stays identical to the existing multifactor model.
- If a prediction exists for a stock with the same `code` and `as_of`, the backend computes an `lstm_score` and blends it into `total_score`.
- The model now targets `pred_return_1d` and next-day `prob_up` first, while `pred_return_5d` / `pred_return_20d` stay as auxiliary context.
- The API response includes metadata such as `lstm_enabled`, `lstm_model_version`, `lstm_weight`, and per-item fields like `next_day_score`, `lstm_score`, `lstm_pred_return_1d`, `lstm_pred_return_5d`, `lstm_pred_return_20d`, `lstm_prob_up`, `lstm_confidence`.

Generate the prediction file from the project root:

```bash
bash lstm/run_batch_export.sh
```

On the first run, the script automatically:

- installs Python 3.12 with Homebrew if needed
- creates `lstm/venv`
- installs packages from `lstm/requirements.txt`

The default export path is `backend-go/data/quant/lstm_predictions_latest.json`.

Optional tuning example:

```bash
bash lstm/run_batch_export.sh \
  --min-market-cap 1000000000000 \
  --epochs 12
```

## Large-Cap Financial CSV Export

코스피/코스닥 시총 1조 이상 기업을 KRX API에서 조회한 뒤, 기업별로 `최근 연간(11011)` + `최근 분기(11013/11012/11014 중 최신)` 재무제표를 DART XBRL에서 찾아 단일 CSV로 저장합니다.

```bash
curl -X POST "http://localhost:5002/api/dart/export/financials" \
  -H "Content-Type: application/json" \
  -d '{
    "minMarketCap": 1000000000000,
    "fsDiv": "CFS",
    "delay": 0.15
  }'
```

요청 파라미터:

- `minMarketCap` / `min_market_cap` (기본: `1000000000000`)
- `fsDiv` / `fs_div` (`CFS` 또는 `OFS`, 기본: `CFS`)
- `asOfDate` / `as_of_date` (`YYYYMMDD`, 미입력 시 최근 14영업일 내 자동 탐색)
- `maxCompanies` / `max_companies` (테스트용 상한, 기본: 전체)
- `delay` (DART 호출 간 지연초, 기본: `0.15`)
- `outputPath` / `output_path` (미입력 시 `backend-go/data/dart/exports/*.csv`)

## Run

```bash
cd backend-go
bash run.sh
```

또는:

```bash
go run ./cmd/backend
```

기본 포트는 `5002`입니다.

## Data Directory

- 기본: `backend-go/data`
- 뉴스: `backend-go/data/news`
- 텔레그램 CSV: `backend-go/data/telegram_chats`
- LSTM 예측 파일: `backend-go/data/quant/lstm_predictions_latest.json`
- KRX 진행 상태: `backend-go/data/krx_collect_progress.json`
- KRX 단일/부분 API 수집 시 진행 상태: `backend-go/data/krx_collect_progress__<api_id...>.json`
- DART 대상 기업 스냅샷: `backend-go/data/dart/targets_latest.csv`
- DART 법인코드 캐시: `backend-go/data/dart/corp_codes.csv`
- DART Export CSV: `backend-go/data/dart/exports/*.csv`

기본 KRX 수집 API:

- `kospi_daily` (`/sto/stk_bydd_trd`)
- `kosdaq_daily` (`/sto/ksq_bydd_trd`)
- `kospi_basic` (`/sto/stk_isu_base_info`)
- `kosdaq_basic` (`/sto/ksq_isu_base_info`)
- `smb_bond_daily` (`/bon/smb_bydd_trd`)
- `bond_index_daily` (`/idx/bon_dd_trd`)
- `gold_daily` (`/gen/gold_bydd_trd`)
- `etf_daily` (`/etp/etf_bydd_trd`)
- `kosdaq_index_daily` (`/idx/kosdaq_dd_trd`)
- `krx_index_daily` (`/idx/krx_dd_trd`)
- `bond_daily` (`/bon/bnd_bydd_trd`)

## Environment

아래 파일을 자동 로드합니다(이미 설정된 환경변수는 덮어쓰지 않음):

1. `backend-go/.env`
2. `<project>/.env`
3. `<project>/krx/.env.local`

주요 환경변수:

- `NEWS_GO_HOST` (기본 `0.0.0.0`)
- `NEWS_GO_PORT` (기본 `5002`)
- `NEWS_GO_DIR` (기본 자동 탐색)
- `NEWS_DATA_ROOT_DIR` (기본 `<backend-go>/data`)
- `LSTM_PREDICTIONS_FILE` (기본 `<backend-go>/data/quant/lstm_predictions_latest.json`)
- `LSTM_WEIGHT` (기본 `0.12`, 최대 `0.35`)
- `KRX_API_KEY` 또는 `KRX_OPENAPI_KEY`
- `DART_FSS_API_KEY`
- `DART_API_BASE_URL` (옵션, 기본 `https://opendart.fss.or.kr/api`)
- `NAVER_CLIENT_ID`
- `NAVER_CLIENT_SECRET`
- `KAKAO_REST_API_KEY`

뉴스 소스별 필수 키:

- `daum`: `KAKAO_REST_API_KEY`
- `naver`: `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`
