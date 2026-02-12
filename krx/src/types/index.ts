export type Market = "KOSPI" | "KOSDAQ";
export type TabType = "KOSPI" | "KOSDAQ" | "BITHUMB" | "TELEGRAM" | "NEWS";
export type Category = "sector" | "theme";
export type NewsSource = "naver" | "daum" | "newsapi";

export type Stock = {
  symbol: string;
  name: string;
  market: Market;
  marketCap: number;
  price: number;
  change1d: number;
  turnover: number;
};

export type Group = {
  id: string;
  name: string;
  market: Market;
  category: Category;
  newsKeywords?: string[];
  change1d: number;
  change5d: number;
  change20d: number;
  turnover: number;
  turnoverAvg20d: number;
  foreignNet5d: number;
  institutionNet5d: number;
  momentumScore: number;
  momentumLabel: string;
  marketCapTotal: number;
  turnoverSpike: number;
  breadthRatio: number;
  topCapRatio: number;
  flowScore: number;
  shortAlert: boolean;
  stocks: Stock[];
};

export type NewsGroup = {
  id: string;
  name: string;
  market: Market;
  category: Category;
  issueScore: number;
  newsScore: number;
  newsCount: number;
  sentimentScore: number;
  volumeScore: number;
  sources: NewsSource[];
  issueTypes: string[];
  leader: Stock;
};

export type ScreenerResponse = {
  market: Market;
  category: Category;
  asOf: number;
  momentumThreshold: number;
  groups: Group[];
  newsGroups: NewsGroup[];
  summary: {
    groupCount: number;
    topGroup?: string;
    topScore?: number;
  };
  newsSummary: {
    groupCount: number;
    topGroup?: string;
    topScore?: number;
  };
  news: {
    enabledSources: NewsSource[];
  };
  note: string;
};

export type SortOption = "score" | "change1d" | "change5d" | "change20d" | "turnover";
export type NewsSortOption = "issueScore" | "newsCount";
