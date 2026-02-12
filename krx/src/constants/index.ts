import type { Market, Category, NewsSource, SortOption, NewsSortOption } from "@/types";

export const MARKET_LABELS: Record<Market, string> = {
  KOSPI: "코스피",
  KOSDAQ: "코스닥",
};

export const CATEGORY_LABELS: Record<Category, string> = {
  sector: "섹터",
  theme: "테마",
};

export const NEWS_SOURCE_LABELS: Record<NewsSource, string> = {
  naver: "네이버",
  daum: "다음",
  newsapi: "NewsAPI",
};

export const MARKETS: Array<{ id: Market; label: string }> = [
  { id: "KOSPI", label: MARKET_LABELS.KOSPI },
  { id: "KOSDAQ", label: MARKET_LABELS.KOSDAQ },
];

export const CATEGORIES: Array<{ id: Category; label: string }> = [
  { id: "sector", label: CATEGORY_LABELS.sector },
  { id: "theme", label: CATEGORY_LABELS.theme },
];

export const SORT_OPTIONS: Array<{ value: SortOption; label: string }> = [
  { value: "score", label: "모멘텀 점수" },
  { value: "change1d", label: "1일 등락률" },
  { value: "change5d", label: "5일 등락률" },
  { value: "change20d", label: "20일 등락률" },
  { value: "turnover", label: "거래대금" },
];

export const NEWS_SORT_OPTIONS: Array<{
  value: NewsSortOption;
  label: string;
}> = [
  { value: "issueScore", label: "이슈 점수" },
  { value: "newsCount", label: "언급 기사 수" },
];

export const DEFAULT_MARKET: Market = "KOSPI";
export const DEFAULT_CATEGORY: Category = "sector";
export const DEFAULT_SORT: SortOption = "score";
export const DEFAULT_NEWS_SORT: NewsSortOption = "issueScore";
