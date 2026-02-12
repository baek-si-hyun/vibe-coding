// UI 관련 상수들
export const UI_LABELS = {
  DAYS: {
    ONE: "1일",
    FIVE: "5일",
    TWENTY: "20일",
  },
  UNITS: {
    POINT: "점",
    COUNT: "개",
    WON: "원",
  },
  FLOW_STATUS: {
    BOTH_BUYING: "외인·기관 동반 순매수",
    SINGLE_INFLOW: "단일 수급 유입",
    WATCHING: "수급 관망",
  },
  BADGES: {
    TURNOVER_SPIKE: "거래대금 급증",
    TOP_CAP_RATIO: "상위 시총 동반",
    SHORT_ALERT: "단기 알림",
  },
  DETAIL: {
    NO_SELECTION: "선택된 섹터/테마가 없습니다.",
    DETAIL_SUFFIX: " 상세",
    LEADER_STOCK: "대표 종목",
    LEADER_NOTE: "이슈 전략 기준 대표 종목만 표시합니다.",
    TOP_STOCKS: "시가총액 상위 종목",
    TRADING_AMOUNT: "거래대금",
    MARKET_CAP_TOTAL: "시가총액 합계",
    TURNOVER_SPIKE: "거래대금 급증",
    BREADTH_RATIO: "상승 확산도",
    TOP_CAP_RATIO: "상위 시총 동반",
    FLOW_STATUS: "수급 상태",
  },
  TABLE: {
    STOCK_NAME: "종목명",
    STOCK_CODE: "종목코드",
    MARKET_CAP: "시가총액",
    CURRENT_PRICE: "현재가",
    CHANGE_RATE: "등락률",
  },
  SELECTOR: {
    MARKET: "시장:",
    CATEGORY: "분류:",
  },
  SEARCH: {
    PLACEHOLDER: "섹터/테마 이름으로 검색...",
    TRADING: "거래",
    RESULTS: "결과",
  },
  VIEW: {
    MARKET: "시장 데이터",
    NEWS: "이슈 분석",
  },
  STATS: {
    RECENT_UPDATE: "최근 업데이트",
    MOMENTUM_THRESHOLD: "모멘텀 기준선",
    TOTAL_GROUPS: "총 그룹 수",
    DATA: "데이터",
    MOMENTUM_TOOLTIP: "모멘텀 점수 비교용 기준선입니다",
    NEWS_NOT_CONNECTED: "뉴스 미연동",
    DEMO: "데모",
    DEMO_NEWS: "데모+이슈",
  },
  HEADER: {
    BADGE_TEXT: "모멘텀 스크리너",
    TITLE: "코스피·코스닥 모멘텀",
    DESCRIPTION:
      "최근 주목받는 섹터와 테마를 모멘텀 점수로 정리하고, 시가총액 상위 종목을 함께 보여줍니다.",
  },
  TABS: {
    KOSPI: "코스피",
    KOSDAQ: "코스닥",
    BITHUMB: "빗썸",
    TELEGRAM: "텔레그램",
    NEWS: "뉴스 크롤링",
  },
  MOMENTUM: {
    LABEL: "모멘텀",
    STRONG: "강한 모멘텀",
    RISING: "상승 모멘텀",
    INTEREST: "관심 모멘텀",
    WATCHING: "관망",
  },
  NEWS: {
    THEME: "테마",
    SECTOR: "섹터",
    ISSUE: "시장 이슈",
    SCORE: "이슈 점수",
    NEWS_SCORE: "뉴스 점수",
    COUNT: "언급 기사",
    SENTIMENT: "긍정 비율",
    VOLUME: "이슈 강도",
    SOURCES: "뉴스 소스",
    ISSUE_TYPES: "이슈 분류",
  },
  A11Y: {
    LOADING: "로딩 중",
    SELECT_GROUP: "그룹 선택",
  },
  SEPARATOR: {
    BULLET: "•",
  },
  ERROR: {
    NOT_FOUND_TITLE: "404",
    NOT_FOUND_MESSAGE: "페이지를 찾을 수 없습니다.",
    GO_HOME: "홈으로 돌아가기",
  },
  LOADING: "불러오는 중...",
} as const;

// 매직 넘버 상수화
export const THRESHOLDS = {
  TURNOVER_SPIKE: 2,
  TOP_CAP_RATIO: 0.67,
  FLOW_SCORE_FULL: 1,
  FLOW_SCORE_HALF: 0.5,
  TOP_STOCKS_COUNT: 3,
  MOMENTUM_STRONG: 85,
  MOMENTUM_RISING: 75,
  MOMENTUM_INTEREST: 65,
  SHORT_ALERT_CHANGE1D: 2,
  SHORT_ALERT_CHANGE5D: 4,
  MAX_SCORE: 100,
  BONUS_SCORE: 5,
  TURNOVER_SCORE_DIVISOR: 3, // turnoverScore 계산용
  DEFAULT_SCORE: 0.5, // 기본 점수
  MAX_NORMALIZED: 1, // 정규화 최대값
  MIN_NORMALIZED: 0, // 정규화 최소값
} as const;

// 가중치 상수
export const WEIGHTS = {
  RS5D: 0.3,
  RS20D: 0.2,
  TURNOVER: 0.15,
  BREADTH: 0.15,
  TOP_CAP: 0.1,
  FLOW: 0.1,
} as const;

// 스타일 관련 상수
export const STYLES = {
  STICKY_TOP: 6, // sticky top-6
  STICKY_TOP_MULTIPLIER: 0.25, // rem 변환용
  CARD_SHADOW: "shadow-md",
  CARD_HOVER: "hover:shadow-lg",
} as const;

// 색상 관련 상수
export const COLORS = {
  POSITIVE: "text-red-600",
  NEGATIVE: "text-blue-600",
  NEUTRAL: "text-gray-900",
  SELECTED_RING: "ring-2 ring-blue-500",
  SELECTED_BG: "bg-blue-50",
  BADGE_BLUE: "bg-blue-500",
} as const;

// 기타 상수
export const LIMITS = {
  NEWS_QUERY_MAX_LENGTH: 60,
  NEWS_PAGE: 1, // 뉴스 API 페이지 번호
  ARRAY_FIRST_INDEX: 0, // 배열 첫 번째 인덱스
} as const;

// 포맷팅 관련 상수
export const FORMATTING = {
  JO_DIVISOR: 1_000_000_000_000, // 조 단위
  EOK_DIVISOR: 100_000_000, // 억 단위
  PERCENT_MULTIPLIER: 100, // 퍼센트 변환
} as const;

// 캐시 관련 상수
export const CACHE = {
  MAX_AGE: 30, // 초
  STALE_WHILE_REVALIDATE: 60, // 초
} as const;
