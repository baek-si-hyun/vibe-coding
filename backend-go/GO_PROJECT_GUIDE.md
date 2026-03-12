# backend-go 안내서

이 문서는 `/Users/baek/project/투자관련/backend-go` 프로젝트를 처음 보는 사람, 그리고 Go를 처음 쓰는 사람을 위한 설명서입니다.

목표는 두 가지입니다.

1. 이 프로젝트의 파일들이 각각 무슨 역할을 하는지 이해한다.
2. Go 코드가 어떤 문법과 구조로 작성되는지 빠르게 익힌다.

## 1. 프로젝트 한눈에 보기

현재 백엔드는 전형적인 Go 서비스 구조에 가깝게 정리되어 있습니다.

```text
backend-go/
├── cmd/
│   ├── backend/
│   │   └── main.go
│   └── newsapi/
│       └── main.go
├── internal/
│   ├── app/
│   ├── bithumb/
│   ├── config/
│   ├── dart/
│   ├── httpx/
│   ├── krx/
│   ├── news/
│   ├── quant/
│   ├── server/
│   │   └── handlers/
│   └── telegram/
├── go.mod
├── run.sh
└── README.md
```

이 구조를 한 줄로 요약하면 아래와 같습니다.

```text
cmd -> config -> app -> server/router -> handlers -> domain services
```

즉:

- `cmd`: 프로그램 시작점
- `config`: 환경변수와 설정 로드
- `app`: 서비스 객체를 한 곳에 묶음
- `server/router`: HTTP 라우팅 구성
- `handlers`: 요청/응답 처리
- `internal/<domain>`: 실제 비즈니스 로직

## 2. Go를 처음 보는 사람을 위한 핵심 문법

이 프로젝트를 읽을 때 자주 보게 되는 문법만 먼저 정리합니다.

### 2.1 `package`

Go 파일은 항상 패키지로 시작합니다.

```go
package main
```

또는

```go
package quant
```

의미:

- 같은 디렉터리의 `.go` 파일들은 보통 같은 `package`를 사용합니다.
- `package main`은 실행 가능한 프로그램의 시작 패키지입니다.
- `package quant`, `package krx` 같은 것은 기능별 코드 묶음입니다.

### 2.2 `import`

다른 패키지를 사용하려면 `import` 합니다.

```go
import (
    "net/http"
    "investment-news-go/internal/app"
)
```

의미:

- `"net/http"`: Go 표준 라이브러리
- `"investment-news-go/internal/app"`: 이 프로젝트 내부 패키지

### 2.3 함수 `func`

Go 함수는 `func`로 정의합니다.

```go
func main() {
}
```

반환값이 있으면 뒤에 타입을 적습니다.

```go
func parseInt64(raw string) int64 {
}
```

여러 값을 반환할 수도 있습니다.

```go
func loadStockFromCSV(path string, market string, minMarketCap int64) (*RankItem, error) {
}
```

Go는 `값, 에러` 패턴을 많이 씁니다.

### 2.4 에러 처리

Go는 예외보다 명시적 에러 반환을 많이 사용합니다.

```go
result, err := doSomething()
if err != nil {
    return nil, err
}
```

이 프로젝트에서도 거의 모든 외부 API 호출, 파일 읽기, CSV 파싱이 이 방식입니다.

### 2.5 구조체 `struct`

여러 필드를 묶는 타입입니다.

```go
type Config struct {
    Host string
    Port string
}
```

이 프로젝트에서 `Config`, `Service`, `RankItem`, `MacroMetric` 같은 타입이 모두 `struct`입니다.

### 2.6 메서드와 리시버

구조체에 함수를 붙이면 메서드가 됩니다.

```go
func (s *Service) Rank(market string, limit int, minMarketCap int64) (map[string]any, error) {
}
```

의미:

- `s *Service`: `Service` 타입의 메서드
- `*Service`: 포인터 리시버
- 같은 서비스 객체의 상태나 설정을 공유할 때 자주 사용합니다.

### 2.7 포인터 `*`

Go는 값을 복사할 수도 있고, 포인터로 참조할 수도 있습니다.

```go
type App struct {
    KRX *krx.Service
}
```

포인터를 쓰는 이유:

- 큰 구조체 복사를 피함
- 같은 인스턴스를 여러 곳에서 공유
- 메서드에서 내부 상태 변경 가능

### 2.8 슬라이스 `[]T`

가장 많이 쓰는 동적 배열입니다.

```go
var items []RankItem
var keywords []string
```

`append`로 요소를 추가합니다.

```go
items = append(items, item)
```

### 2.9 맵 `map[K]V`

키-값 저장소입니다.

```go
map[string]any
map[string]string
map[string]struct{}
```

이 프로젝트에서는 JSON 비슷한 데이터를 임시로 다룰 때 `map[string]any`가 많습니다.

### 2.10 공개/비공개 규칙

Go는 첫 글자가 대문자면 외부 패키지에서 접근 가능합니다.

- `Config`, `Load`, `NewService`: 공개
- `loadDotEnvIfExists`, `parseInt64`: 비공개

이 규칙만 알아도 어느 함수가 패키지 외부용인지 빠르게 파악할 수 있습니다.

### 2.11 `if`, `for`

Go에는 `while`이 없고 반복은 거의 `for`로 처리합니다.

```go
if err != nil {
    return err
}

for _, item := range items {
}
```

### 2.12 JSON 태그

구조체를 JSON으로 주고받을 때 필드 이름을 지정합니다.

```go
type APIEndpoint struct {
    URL  string `json:"url"`
    Name string `json:"name"`
}
```

즉, Go 코드에서는 `URL`, JSON에서는 `url`로 보입니다.

### 2.13 파일명 관례

Go는 파일명 자체보다 `package`와 코드 내용이 더 중요합니다. 그래도 관례는 있습니다.

- `main.go`: 실행 시작점
- `service.go`: 도메인의 핵심 로직
- `router.go`: HTTP 라우팅
- `handlers.go`: HTTP 요청 처리
- `types.go`: 데이터 구조 정의
- `*_test.go`: 테스트 파일

현재 이 프로젝트는 테스트 파일이 거의 없고, 주로 서비스 코드 중심입니다.

## 3. Go 프로젝트 디렉터리 규칙

### 3.1 `go.mod`

Go 모듈의 기준 파일입니다.

현재 내용:

- 모듈명: `investment-news-go`
- Go 버전: `1.22`

이 파일이 있기 때문에 내부 import를 아래처럼 할 수 있습니다.

```go
import "investment-news-go/internal/quant"
```

### 3.2 `cmd`

실행 바이너리 시작점입니다.

- `cmd/backend`: 메인 백엔드 서버
- `cmd/newsapi`: 뉴스 전용 서버

Go에서는 하나의 저장소에 여러 실행 파일을 둘 때 `cmd/<app-name>/main.go` 구조를 많이 씁니다.

### 3.3 `internal`

`internal` 아래 패키지는 바깥 모듈에서 직접 import하기 어렵게 하는 Go 규칙이 있습니다.

의도:

- 외부 공개 라이브러리가 아니라, 이 프로젝트 내부 구현이라는 뜻
- 내부 구조를 비교적 자유롭게 바꿀 수 있음

이 프로젝트에는 아래 도메인들이 들어 있습니다.

- `krx`: KRX 수집/저장
- `dart`: DART 재무 데이터
- `news`: 뉴스 수집/저장
- `quant`: 퀀트 계산/매크로 계산
- `bithumb`: 가상자산 스크리너
- `telegram`: 텔레그램 CSV 조회

## 4. 이 프로젝트의 요청 흐름

브라우저나 프론트엔드가 API를 호출하면 대략 아래 흐름으로 움직입니다.

```text
HTTP 요청
-> cmd/backend/main.go
-> internal/config.Load()
-> internal/app.New()
-> internal/server.NewRouter()
-> internal/server/handlers/*.go
-> internal/<domain>/service.go
-> JSON 응답
```

예를 들어 퀀트 랭킹 요청은 이렇게 흐릅니다.

```text
GET /api/quant/rank
-> router.go 에서 QuantRank 핸들러 연결
-> handlers/quant.go
-> quant.Service.Rank()
-> CSV/저장 데이터 읽기 + 점수 계산
-> JSON 반환
```

## 5. 파일별 설명

아래 설명은 실제 존재하는 파일 기준입니다.

### 5.1 루트 파일

#### `go.mod`

프로젝트의 모듈 선언 파일입니다.

역할:

- 모듈 이름 정의: `investment-news-go`
- Go 버전 정의
- import 경로의 기준점 역할

처음 보는 사람은 이 파일을 보고 "이 프로젝트가 독립된 Go 모듈이구나"라고 이해하면 됩니다.

#### `run.sh`

백엔드 실행용 셸 스크립트입니다.

역할:

- 현재 디렉터리를 `backend-go`로 이동
- `.env`에서 포트 값을 읽음
- 이미 같은 포트에 서버가 떠 있으면 새로 실행하지 않음
- 문제 없으면 `go run ./cmd/backend` 실행

즉, 개발자가 편하게 서버를 띄우기 위한 진입 스크립트입니다.

#### `README.md`

프로젝트 사용법과 개요를 설명하는 기본 문서입니다.

역할:

- 실행 방법
- 데이터 저장 위치
- 주요 API 개요
- 현재 구조 설명

새로 합류한 사람은 `README.md`와 이 문서를 같이 보면 전체 맥락을 빠르게 잡을 수 있습니다.

### 5.2 실행 진입점

#### `cmd/backend/main.go`

메인 백엔드 서버의 시작 파일입니다.

역할:

- 설정 로드: `config.Load()`
- 앱 객체 생성: `app.New(cfg)`
- 라우터 생성: `server.NewRouter(application)`
- HTTP 서버 시작

중요한 점:

- 이 파일은 "얇게" 유지하는 것이 좋습니다.
- 실제 로직은 여기 넣지 않고, 조립만 담당합니다.

#### `cmd/newsapi/main.go`

뉴스 기능만 별도로 띄우는 실행 파일입니다.

역할:

- 설정 로드
- `news.Service` 생성
- `news.NewHandler(service)`로 뉴스 전용 핸들러 구성
- CORS 적용 후 서버 시작

이 파일이 있다는 것은 뉴스 기능을 독립 프로세스로도 운영할 수 있다는 뜻입니다.

### 5.3 공통 조립 계층

#### `internal/config/config.go`

환경변수와 경로를 읽어서 `Config` 구조체로 만드는 파일입니다.

핵심 타입:

- `APIEndpoint`
- `Config`

핵심 함수:

- `Load()`
- `EndpointKeys()`

하는 일:

- `.env` 탐색 및 로드
- 호스트, 포트, 데이터 경로 설정
- 각종 API 키 로드
- KRX API endpoint 목록 구성
- 백엔드 루트 경로 자동 추정

이 프로젝트에서 설정은 거의 전부 여기서 시작됩니다.

#### `internal/app/app.go`

서비스 객체들을 한데 모아두는 조립 레이어입니다.

핵심 타입:

- `App`

역할:

- `krx.Service`
- `dart.Service`
- `news.Service`
- `quant.Service`
- `bithumb.Service`
- `telegram.Service`

같은 도메인 서비스를 생성해서 하나의 구조체에 담습니다.

즉, 핸들러는 이 `App` 객체를 통해 필요한 서비스에 접근합니다.

### 5.4 HTTP 공통 유틸

#### `internal/httpx/json.go`

JSON 응답을 쓰는 공통 함수 파일입니다.

핵심 함수:

- `WriteJSON`

역할:

- `Content-Type` 설정
- HTTP 상태코드 설정
- JSON 인코딩

핸들러에서 중복되는 응답 코드를 줄입니다.

#### `internal/httpx/middleware.go`

HTTP 미들웨어 모음입니다.

핵심 함수:

- `WithCORS`

역할:

- 브라우저에서 다른 포트의 프론트엔드가 API 호출할 수 있게 CORS 헤더 추가

프론트와 백엔드를 로컬에서 따로 띄울 때 필수입니다.

#### `internal/httpx/request.go`

요청 파싱용 유틸 함수 모음입니다.

핵심 함수:

- `DecodeJSON`
- `ParseCommaList`
- `ToString`
- `ToFloat`
- `ToInt`
- `ToInt64`
- `ToBool`
- `ParseIntOrDefault`

역할:

- JSON body 읽기
- query/body 값을 문자열, 숫자, 불리언으로 안전하게 변환
- 잘못된 입력이 와도 기본값으로 처리

핸들러를 단순하게 유지하는 데 중요합니다.

### 5.5 서버 라우팅 계층

#### `internal/server/router.go`

전체 HTTP 라우트를 등록하는 파일입니다.

핵심 함수:

- `NewRouter`

현재 연결된 주요 엔드포인트:

- `/`
- `/api/news/`
- `/api/endpoints`
- `/api/files`
- `/api/collect`
- `/api/collect/resume`
- `/api/quant/rank`
- `/api/quant/report`
- `/api/quant/macro`
- `/api/dart/export/financials`
- `/api/bithumb/screener`
- `/api/telegram/chat-rooms`
- `/api/telegram/items`
- `/api/*` 동적 KRX fetch

이 파일을 보면 "어떤 URL이 어떤 핸들러로 연결되는지"를 한 번에 알 수 있습니다.

### 5.6 HTTP 핸들러 계층

#### `internal/server/handlers/handlers.go`

핸들러 공통 구조를 정의하는 파일입니다.

핵심 타입:

- `Handlers`

핵심 함수:

- `New`
- `quantMinCap`
- `apiIDsFromBodyOrQuery`

역할:

- `App` 객체를 보관
- 핸들러들이 공통으로 쓰는 파라미터 처리 로직 제공

#### `internal/server/handlers/health.go`

헬스체크 응답 파일입니다.

핵심 함수:

- `Health`

역할:

- 서버가 살아있는지 확인
- 기본 메타 정보 반환

#### `internal/server/handlers/krx.go`

KRX 관련 HTTP 요청을 처리합니다.

핵심 함수:

- `KRXEndpoints`
- `KRXFiles`
- `KRXCollect`
- `KRXCollectResume`
- `KRXDynamicFetch`

역할:

- 사용 가능한 KRX API endpoint 목록 조회
- 저장된 파일 목록 조회
- 수집 실행
- 중단된 수집 이어서 실행
- 일반 KRX API 프록시/동적 요청 처리

#### `internal/server/handlers/dart.go`

DART 관련 엔드포인트 처리 파일입니다.

핵심 함수:

- `DartExportFinancials`

역할:

- 시총 기준 종목들의 최근 연간/최근 분기 재무제표 CSV 내보내기 실행

#### `internal/server/handlers/quant.go`

퀀트 엔드포인트 처리 파일입니다.

핵심 함수:

- `QuantRank`
- `QuantReport`
- `QuantMacro`

역할:

- 종목 랭킹 계산
- 리포트 데이터 생성
- 거시지표 데이터 반환

#### `internal/server/handlers/bithumb.go`

빗썸 스크리너 요청 처리 파일입니다.

핵심 함수:

- `BithumbScreener`

역할:

- 거래량, 이동평균, 패턴 조건 기반 스크리너 결과 반환

#### `internal/server/handlers/telegram.go`

텔레그램 관련 조회 요청 처리 파일입니다.

핵심 함수:

- `TelegramChatRooms`
- `TelegramItems`

역할:

- CSV에 저장된 채팅방 목록 조회
- 메시지/아이템 조회 및 필터링

### 5.7 도메인 서비스: KRX

#### `internal/krx/service.go`

KRX 데이터 수집의 핵심 파일입니다.

핵심 타입:

- `Service`
- `HTTPError`
- `progressState`

핵심 기능:

- KRX API 호출
- 행 데이터 추출 및 정규화
- CSV 저장
- 시총 기준 종목 필터링
- 종목기본정보와 일별정보 연결
- 수집 진행 상태 저장/재개
- 배치 수집

이 파일은 현재 KRX 도메인의 핵심 엔진 역할을 합니다.

중요 포인트:

- `CollectAndSave`: 지정 날짜와 API 목록 수집
- `ListFiles`: 저장 파일 조회
- `CollectBatchResume`: 이어받기 수집

### 5.8 도메인 서비스: DART

#### `internal/dart/service.go`

DART Open API 통신의 핵심 파일입니다.

핵심 타입:

- `Service`
- `Company`
- `statusError`

핵심 기능:

- 기업 고유번호(corp code) 다운로드/캐시
- 재무제표 관련 API 호출
- 응답 상태 코드 판별
- DART 데이터와 KRX 종목 정보 연결

즉, "DART에서 데이터를 가져오는 기반 서비스"입니다.

#### `internal/dart/financial_export.go`

대형주 재무제표 CSV 내보내기 기능 파일입니다.

핵심 기능:

- 최근 연간 보고서 후보 생성
- 최근 분기 보고서 후보 생성
- KRX 대형주 목록 불러오기
- 회사별 최신 재무행 찾기
- 연간/분기 데이터를 CSV로 출력

중요 포인트:

- "코스피/코스닥 시총 1조 이상 기업" 같은 조건의 재무 CSV 생성 로직이 여기 있습니다.
- 연간과 분기 데이터를 함께 뽑기 위해 보고서 우선순위를 구성합니다.

### 5.9 도메인 서비스: 뉴스

#### `internal/news/service.go`

뉴스 서비스의 기본 설정 파일입니다.

핵심 타입:

- `Service`

역할:

- 데이터 디렉터리 경로 계산
- 키워드 파일 경로
- 병합 결과 파일 경로
- 진행 상태 파일 경로

즉, 뉴스 도메인의 공통 베이스 객체입니다.

#### `internal/news/types.go`

뉴스 도메인에서 쓰는 데이터 구조 정의 파일입니다.

핵심 타입:

- `NewsItem`
- `FetchResult`
- `CrawlProgress`
- `CrawlRunResult`
- `SavedFileInfo`

역할:

- 뉴스 한 건의 표준 구조 정의
- 수집 결과/진행률/저장 파일 메타 정보 표현

#### `internal/news/http.go`

뉴스 전용 HTTP 핸들러 생성 파일입니다.

핵심 함수:

- `NewHandler`

역할:

- `/api/news/*` 아래 뉴스 관련 서브 라우트를 구성

다른 도메인과 달리 뉴스는 자체 라우터를 별도 구성하는 형태입니다.

#### `internal/news/keywords.go`

뉴스 검색 키워드 처리 파일입니다.

핵심 타입:

- `crawlTask`

핵심 기능:

- 키워드 정규화
- ASCII 여부 판별
- 뉴스 API 질의 문자열 조합
- 키워드 묶음 분할
- 뉴스 결과와 매칭된 키워드 정리

최근 뉴스 API 질의 품질을 좌우하는 중요한 파일입니다.

#### `internal/news/news_api.go`

외부 뉴스 제공자와 직접 통신하는 파일입니다.

핵심 기능:

- 네이버 뉴스 호출
- 카카오 웹 검색 호출
- NewsAPI 호출
- 제공자별 응답을 공통 `NewsItem` 구조로 정규화
- 발행일 파싱
- 언론사 추출
- 날짜 필터링

즉, 외부 API 차이를 흡수하는 어댑터 역할입니다.

#### `internal/news/crawl.go`

뉴스 수집 오케스트레이션 파일입니다.

핵심 기능:

- 키워드별 요청 실행
- 소스별 배치 크롤링
- 저장 결과 집계
- 이어받기 크롤링

중요 포인트:

- 실제 수집 작업의 흐름 제어는 여기 있습니다.
- `CrawlAPIResume`가 중단 이후 재시작에 사용됩니다.

#### `internal/news/store.go`

뉴스 저장/읽기 파일입니다.

핵심 기능:

- 출력 파일 생성 보장
- 기존 링크 중복 제거
- CSV append 저장
- 진행 상태 저장
- 저장 파일 목록 조회
- 저장 뉴스 페이지 단위 조회

즉, 뉴스의 영속화 계층 역할을 합니다.

#### `internal/news/universe.go`

국내 대형주 키워드 유니버스를 만드는 파일입니다.

핵심 기능:

- 저장된 KRX CSV에서 최신 회사 스냅샷 읽기
- 시총 기준으로 국내 대형주 필터링
- 뉴스 검색용 회사명 키워드 생성

이 프로젝트가 "국내 코스피/코스닥 시총 1조 이상 기업" 중심으로 뉴스 수집을 맞추는 데 필요한 파일입니다.

### 5.10 도메인 서비스: Quant

#### `internal/quant/service.go`

퀀트 랭킹/리포트 계산의 핵심 파일입니다.

핵심 타입:

- `Service`
- `cacheEntry`
- `rankResult`
- `RankItem`

핵심 기능:

- CSV에서 종목 데이터 로드
- 최신 거래일만 필터링
- 중복 종목 제거
- 모멘텀/유동성/안정성 지표 계산
- 퍼센타일 점수화
- 종합 점수 계산
- 랭킹/리포트 JSON 생성

현재 이 파일은 퀀트 엔진의 중심입니다.

이 파일에서 자주 보이는 계산 함수:

- `pctChange`
- `stddev`
- `movingAverage`
- `downsideStddev`
- `maxDrawdown`
- `positiveRatio`
- `efficiencyRatio`
- `coefficientOfVariation`
- `percentileScores`

즉, 단순 CRUD가 아니라 계산 로직이 집중된 파일입니다.

#### `internal/quant/macro.go`

거시지표와 지정학 점수 계산 파일입니다.

핵심 타입:

- `MacroMetric`
- `MacroHeadline`
- `GeopoliticalSignal`
- `MacroResponse`

핵심 기능:

- WTI, 미국채 10년물, VIX, USD/KRW, 금 가격 수집
- FRED, CBOE, Polygon, Alpha Vantage, exchangerate.host 등 외부 API 연결
- NewsAPI/GDELT 기반 헤드라인 수집
- 지정학 키워드 매칭
- 지정학 점수 계산
- 헤드라인 최신순 정리

이 파일은 퀀트 화면의 매크로 섹션과 뉴스 기반 리스크 점수의 핵심입니다.

### 5.11 도메인 서비스: Bithumb

#### `internal/bithumb/service.go`

빗썸 마켓 데이터를 이용한 스크리너 파일입니다.

핵심 타입:

- `Candle`
- `Signals`
- `Item`
- `Service`

핵심 기능:

- 심볼 목록 조회
- 캔들 데이터 조회
- 거래량 급증 패턴 찾기
- 이동평균 반등 패턴 찾기
- 저항 돌파 패턴 찾기
- 모드별 스크리너 결과 생성

국내 주식 퀀트와 별개로, 가상자산 스크리닝 기능을 담당합니다.

### 5.12 도메인 서비스: Telegram

#### `internal/telegram/service.go`

텔레그램 CSV 데이터 조회 서비스 파일입니다.

핵심 타입:

- `Item`
- `Service`

핵심 기능:

- CSV 파일 목록 검색
- 채팅방 이름 추출
- CSV 행을 아이템 구조로 읽기
- 검색어, 채팅방, 페이지 기준 필터링

즉, 텔레그램 원본 CSV를 조회 API로 바꾸는 파일입니다.

## 6. 자주 보는 Go 코드 패턴을 이 프로젝트에 대입해 보기

### 6.1 생성자 패턴

Go에는 클래스 생성자가 없어서 보통 `New...` 함수를 씁니다.

예:

- `config.Load()`
- `app.New(cfg)`
- `krx.NewService(cfg)`
- `quant.NewService(cfg)`

의미:

- 객체를 만들 때 필요한 초기 설정을 한 곳에 모은다.

### 6.2 서비스 구조체 패턴

도메인마다 `Service` 구조체가 있고, 메서드로 기능을 제공합니다.

예:

```go
type Service struct {
    cfg config.Config
}
```

이 패턴의 장점:

- 설정을 한 번 넣고 재사용 가능
- 도메인 로직을 한 곳에 모을 수 있음

### 6.3 핸들러와 서비스 분리

이 프로젝트는 HTTP 처리와 계산 로직을 분리하고 있습니다.

- 핸들러: 요청 파라미터 읽기, 상태코드 결정, JSON 응답
- 서비스: 실제 계산과 외부 API 호출

이 분리는 유지보수에 중요합니다.

나쁜 예:

- 핸들러 안에서 CSV 읽고 계산하고 API까지 직접 호출

현재 구조의 좋은 점:

- 핸들러는 얇고, 도메인 로직은 서비스에 있음

## 7. 처음 보는 사람이 읽는 순서

처음 들어온 사람이면 아래 순서로 읽는 것이 가장 효율적입니다.

1. `go.mod`
2. `cmd/backend/main.go`
3. `internal/config/config.go`
4. `internal/app/app.go`
5. `internal/server/router.go`
6. `internal/server/handlers/quant.go`
7. `internal/quant/service.go`
8. 필요한 도메인별 `service.go`

이 순서로 보면 "서버가 어떻게 뜨고, 요청이 어디로 가고, 실제 계산은 어디서 하는지"가 가장 빨리 보입니다.

## 8. 이 프로젝트에서 자주 만지는 파일과 목적

작업 목적별로 보면 아래처럼 생각하면 됩니다.

### 새로운 API 엔드포인트를 추가하고 싶을 때

주로 수정하는 파일:

- `internal/server/router.go`
- `internal/server/handlers/*.go`
- `internal/<domain>/service.go`

### 환경변수나 경로를 추가하고 싶을 때

주로 수정하는 파일:

- `internal/config/config.go`

### 퀀트 계산식을 바꾸고 싶을 때

주로 수정하는 파일:

- `internal/quant/service.go`
- 필요하면 `internal/quant/macro.go`

### KRX 수집 로직을 바꾸고 싶을 때

주로 수정하는 파일:

- `internal/krx/service.go`

### DART 재무 CSV 형식을 바꾸고 싶을 때

주로 수정하는 파일:

- `internal/dart/financial_export.go`
- `internal/dart/service.go`

### 뉴스 키워드/공급자를 바꾸고 싶을 때

주로 수정하는 파일:

- `internal/news/keywords.go`
- `internal/news/news_api.go`
- `internal/news/crawl.go`
- `internal/news/universe.go`

## 9. 실행과 확인 방법

### 서버 실행

프로젝트 루트가 아니라 `backend-go` 기준으로 실행합니다.

```bash
cd /Users/baek/project/투자관련/backend-go
go run ./cmd/backend
```

또는

```bash
./run.sh
```

### 테스트 실행

```bash
go test ./...
```

### 코드 포맷

Go는 보통 `gofmt`를 기본으로 사용합니다.

```bash
gofmt -w ./cmd ./internal
```

### 가장 먼저 확인할 API

```bash
curl http://localhost:5002/
```

정상이라면 헬스체크 JSON이 나와야 합니다.

## 10. 초보자가 헷갈리기 쉬운 포인트

### `main.go`에 로직이 거의 없는 이유

Go 서비스는 `main.go`를 조립 전용으로 얇게 두는 경우가 많습니다. 실제 로직은 서비스 레이어에 넣는 것이 일반적입니다.

### `map[string]any`가 많은 이유

외부 API 응답이나 CSV 기반 임시 데이터를 유연하게 다루기 쉽기 때문입니다. 다만 타입 안정성은 약하므로, 규모가 커지면 점차 명시적 구조체로 바꾸는 것이 좋습니다.

### 왜 `internal`을 쓰는가

외부 공개용 라이브러리가 아니라 내부 서비스 코드이기 때문입니다. 의존성 경계를 강하게 만들 수 있습니다.

### 왜 `service.go` 파일이 큰가

Go 프로젝트 초기에 흔한 현상입니다. 기능이 늘면서 한 파일에 계산과 IO가 같이 모이기 쉽습니다. 지금 구조는 이미 상위 구조는 분리되었고, 다음 단계는 각 도메인 내부에서 파일을 더 나누는 것입니다.

## 11. 앞으로 더 개선할 수 있는 구조

현재 구조는 이미 많이 정리된 상태지만, 더 세분화하려면 아래 방향이 가능합니다.

### `internal/quant`

가능한 분리 예시:

- `loader.go`
- `factors.go`
- `ranking.go`
- `report.go`
- `cache.go`
- `macro.go`

### `internal/krx`

가능한 분리 예시:

- `client.go`
- `collector.go`
- `csv.go`
- `progress.go`
- `filter.go`

### `internal/dart`

가능한 분리 예시:

- `client.go`
- `corpcode.go`
- `financials.go`
- `export.go`

즉, 지금은 "프로젝트 레벨 구조"는 잘 잡혀 있고, 다음 최적화는 "도메인 내부 파일 분리" 단계라고 보면 됩니다.

## 12. 마지막 요약

이 프로젝트를 가장 짧게 설명하면 아래와 같습니다.

- `cmd`는 시작점이다.
- `config`는 환경설정 로더다.
- `app`은 서비스 조립기다.
- `router`는 URL 연결표다.
- `handlers`는 HTTP 입구다.
- 각 `internal/<domain>`은 실제 기능 구현부다.

Go를 처음 보는 사람은 아래 한 줄만 먼저 익히면 됩니다.

```text
main이 서버를 띄우고, router가 길을 정하고, handler가 요청을 받고, service가 실제 일을 한다.
```

이 문서 다음 단계로는 각 도메인별 상세 안내서가 가장 효과적입니다. 특히 `quant`, `krx`, `dart`는 별도 MD로 더 쪼개서 설명할 가치가 큽니다.
