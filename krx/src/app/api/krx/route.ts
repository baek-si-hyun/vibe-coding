import { NextRequest, NextResponse } from "next/server";
import { type Category, type Market, type Group, type Stock } from "@/types";
import {
  ISSUE_SCORE_THRESHOLD,
  ISSUE_NEWS_COUNT_THRESHOLD,
} from "@/constants/api";
import { THRESHOLDS, LIMITS, CACHE } from "@/constants/ui";
import {
  parseMarket,
  parseCategory,
} from "@/utils/api-helpers";
import {
  getNewsConfig,
  buildNewsSignals,
} from "@/utils/news";
import { clamp } from "@/utils/momentum";
import type { NewsGroup, NewsSource } from "@/types";
import { THEME_KEYWORDS } from "@/data/theme-keywords";

// GroupResponse는 Group과 동일 (Group에 이미 모든 필드 포함)
type GroupResponse = Group;

type ScreenerResponse = {
  market: Market;
  category: Category;
  asOf: number;
  momentumThreshold: number;
  groups: GroupResponse[];
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

/**
 * theme-keywords.ts의 그룹 목록을 기반으로 최소한의 그룹 구조 생성
 * (뉴스 분석용)
 */
function createGroupsFromKeywords(market: Market, category: Category): Group[] {
  const groupNames = Object.keys(THEME_KEYWORDS);
  return groupNames.map((groupName) => ({
    id: `${market.toLowerCase()}-${category}-${groupName.toLowerCase().replace(/\s+/g, "-")}`,
    name: groupName,
    market,
    category,
    change1d: 0,
    change5d: 0,
    change20d: 0,
    turnover: 0,
    turnoverAvg20d: 0,
    foreignNet5d: 0,
    institutionNet5d: 0,
    momentumScore: 0,
    momentumLabel: "",
    marketCapTotal: 0,
    turnoverSpike: 0,
    breadthRatio: 0,
    topCapRatio: 0,
    flowScore: 0,
    shortAlert: false,
    stocks: [],
  }));
}

export async function GET(request: NextRequest) {
  const startTime = Date.now();
  const { searchParams } = new URL(request.url);
  const market = parseMarket(searchParams.get("market"));
  const category = parseCategory(searchParams.get("category"));
  
  const params = {
    market: searchParams.get("market"),
    category: searchParams.get("category"),
  };

  // 모든 시장: 뉴스 기반 이슈 분석만 사용
  const baseGroups = createGroupsFromKeywords(market, category);

  // Fetch news signals if configured
  const newsConfig = getNewsConfig();
  
  const newsSignals = await buildNewsSignals(baseGroups, newsConfig);

  // 모든 시장: 뉴스 기반 이슈 분석만 (groups는 빈 배열)
  const groups: Group[] = [];
  
  const enabledSources = newsConfig.enabledSources;
  const newsGroups: NewsGroup[] = baseGroups
    .map((group) => {
      const signal = newsSignals.get(group.id);
      if (!signal) return null;
      
      // 뉴스 점수만 사용
      const issueScoreRaw = clamp(
        signal.score,
        THRESHOLDS.MIN_NORMALIZED,
        THRESHOLDS.MAX_NORMALIZED,
      );
      
      const issueScore = Math.round(issueScoreRaw * 100);
      const newsScore = Math.round(signal.score * 100);
      
      // 뉴스 신호만 확인
      const meetsNewsSignal = signal.totalCount >= ISSUE_NEWS_COUNT_THRESHOLD;
      const isQualified = meetsNewsSignal && issueScore >= ISSUE_SCORE_THRESHOLD;
      
      if (!isQualified) return null;
      
      // leader stock은 빈 stocks 배열이므로 더미 데이터 생성
      const leader: Stock = {
        symbol: "",
        name: group.name,
        market: group.market,
        marketCap: 0,
        price: 0,
        change1d: 0,
        turnover: 0,
      };
      
      return {
        id: group.id,
        name: group.name,
        market: group.market,
        category: group.category,
        issueScore,
        newsScore,
        newsCount: signal.totalCount,
        sentimentScore: signal.sentimentScore,
        volumeScore: signal.volumeScore,
        sources: signal.sources,
        issueTypes: signal.issueTypes ?? [],
        leader,
      };
    })
    .filter(Boolean) as NewsGroup[];
    
  newsGroups.sort((a, b) => {
    if (b.issueScore !== a.issueScore) return b.issueScore - a.issueScore;
    return b.newsCount - a.newsCount;
  });

  const response: ScreenerResponse = {
    market,
    category,
    asOf: Date.now(),
    momentumThreshold: 0, // 코스피/코스닥은 모멘텀 점수 사용 안 함
    groups, // 코스피/코스닥은 빈 배열
    newsGroups,
    summary: {
      groupCount: groups.length,
      topGroup: groups[LIMITS.ARRAY_FIRST_INDEX]?.name,
      topScore: groups[LIMITS.ARRAY_FIRST_INDEX]?.momentumScore,
    },
    newsSummary: {
      groupCount: newsGroups.length,
      topGroup: newsGroups[LIMITS.ARRAY_FIRST_INDEX]?.name,
      topScore: newsGroups[LIMITS.ARRAY_FIRST_INDEX]?.issueScore,
    },
    news: {
      enabledSources,
    },
    note: `뉴스 기반 이슈 분석${enabledSources.length > 0 ? ` (${enabledSources.map((s) => s === "naver" ? "네이버" : s === "daum" ? "다음" : "NewsAPI").join(", ")})` : ""}`,
  };

  return NextResponse.json(response, {
    headers: {
      "cache-control": `public, max-age=${CACHE.MAX_AGE}, stale-while-revalidate=${CACHE.STALE_WHILE_REVALIDATE}`,
    },
  });
}
