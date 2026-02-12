export const MOMENTUM_THRESHOLD = 70;
export const TURNOVER_SPIKE_THRESHOLD = 2;
export const TOP_CAP_RATIO_THRESHOLD = 0.67;
export const NEWS_WEIGHT = 0.12;
export const NEWS_ITEMS_PER_SOURCE = 8;
export const NEWS_CONCURRENCY = 3;
export const NEWS_TIMEOUT_MS = 4500;
export const NEWS_LOOKBACK_HOURS = 24;
export const NEWS_CACHE_TTL_MS = 60_000;
export const GENERAL_NEWS_KEYWORDS = [
  "주식",
  "증시",
  "코스피",
  "코스닥",
  "증권",
  "경제",
  "산업",
  "금융",
  "미국",
  "실적",
  "정책",
  "규제",
];
export const ALL_NEWS_ITEMS_PER_SOURCE = 50;

export const ISSUE_WEIGHTS = {
  NEWS: 0.5,
  PARTICIPATION: 0.3,
  TURNOVER: 0.2,
} as const;
export const ISSUE_SCORE_THRESHOLD = 60;
export const ISSUE_NEWS_COUNT_THRESHOLD = 2;
export const ISSUE_PARTICIPATION_THRESHOLD = 0.6;
export const ISSUE_TURNOVER_SPIKE_THRESHOLD = 1.2;
export const MARKET_DATA_CACHE_TTL_MS = 30_000;
export const MARKET_DATA_CONCURRENCY = 4;
export const MARKET_DATA_LOOKBACK_DAYS = 30;

export const POSITIVE_KEYWORDS = [
  "수혜",
  "호재",
  "기대",
  "상승",
  "급등",
  "강세",
  "돌파",
  "최고",
  "신고가",
  "흑자",
  "실적개선",
  "실적 개선",
  "수주",
  "계약",
  "투자",
  "증설",
  "성장",
  "확대",
  "턴어라운드",
  "improvement",
  "record high",
] as const;

export const NEGATIVE_KEYWORDS = [
  "악재",
  "하락",
  "급락",
  "약세",
  "부진",
  "리스크",
  "규제",
  "경고",
  "적자",
  "손실",
  "실적부진",
  "실적 부진",
  "소송",
  "사고",
  "리콜",
  "파산",
  "철회",
  "하향",
  "신저가",
  "중단",
  "delay",
  "lawsuit",
] as const;
