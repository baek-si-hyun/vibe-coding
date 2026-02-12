import {
  NEWS_ITEMS_PER_SOURCE,
  NEWS_TIMEOUT_MS,
  NEWS_LOOKBACK_HOURS,
  NEWS_CONCURRENCY,
  NEWS_CACHE_TTL_MS,
  POSITIVE_KEYWORDS,
  NEGATIVE_KEYWORDS,
  GENERAL_NEWS_KEYWORDS,
  ALL_NEWS_ITEMS_PER_SOURCE,
} from "@/constants/api";
import { UI_LABELS, LIMITS, THRESHOLDS } from "@/constants/ui";
import { THEME_KEYWORDS } from "@/data/theme-keywords";
import { ISSUE_KEYWORDS, ISSUE_FALLBACK_TYPE } from "@/data/issue-keywords";
import { clamp } from "./momentum";
import type { NewsSource } from "@/types";
import type { Group } from "@/types";

export type NewsConfig = {
  enabledSources: NewsSource[];
  naver?: {
    clientId: string;
    clientSecret: string;
  };
  daum?: {
    apiKey: string;
  };
  newsapi?: {
    apiKey: string;
  };
};

export type ProviderSignal = {
  source: NewsSource;
  count: number;
  sentiment: number;
};

export type NewsSignal = {
  query: string;
  totalCount: number;
  volumeScore: number;
  sentimentScore: number;
  score: number;
  sources: NewsSource[];
  issueTypes?: string[];
};

export type NewsItem = {
  title: string;
  description: string;
  source: NewsSource;
  publishedAt: number;
};

type NewsCacheEntry = {
  fetchedAt: number;
  data: NewsItem[];
};

export function getNewsConfig(): NewsConfig {
  const enabledSources: NewsSource[] = [];
  const naverClientId =
    process.env.NAVER_CLIENT_ID?.trim() ??
    process.env.NEXT_PUBLIC_NAVER_API_CLIENT_ID?.trim();
  const naverClientSecret =
    process.env.NAVER_CLIENT_SECRET?.trim() ??
    process.env.NEXT_PUBLIC_NAVER_API_CLIENT_SECRET?.trim();
  const daumApiKey =
    process.env.KAKAO_REST_API_KEY?.trim() ??
    process.env.NEXT_PUBLIC_DAUM_API_KEY?.trim();
  const newsApiKey =
    process.env.NEWSAPI_KEY?.trim() ??
    process.env.NEXT_PUBLIC_NEWSAPI_API_KEY?.trim();

  const naver =
    naverClientId && naverClientSecret
      ? { clientId: naverClientId, clientSecret: naverClientSecret }
      : undefined;
  const daum = daumApiKey ? { apiKey: daumApiKey } : undefined;
  const newsapi = newsApiKey ? { apiKey: newsApiKey } : undefined;

  if (naver) enabledSources.push("naver");
  if (daum) enabledSources.push("daum");
  if (newsapi) enabledSources.push("newsapi");

  return {
    enabledSources,
    naver,
    daum,
    newsapi,
  };
}

export function buildNewsQuery(group: Group): string {
  const suffix =
    group.category === "theme" ? UI_LABELS.NEWS.THEME : UI_LABELS.NEWS.SECTOR;
  const query = `${group.name} ${suffix}`;
  return query.length > LIMITS.NEWS_QUERY_MAX_LENGTH
    ? query.slice(0, LIMITS.NEWS_QUERY_MAX_LENGTH)
    : query;
}

export function computeSentiment(texts: string[]): number {
  if (texts.length === 0) return THRESHOLDS.DEFAULT_SCORE;

  let positiveCount = 0;
  let negativeCount = 0;

  const lowerTexts = texts.map((text) => text.toLowerCase());

  for (const text of lowerTexts) {
    const hasPositive = POSITIVE_KEYWORDS.some((keyword) =>
      text.includes(keyword.toLowerCase()),
    );
    const hasNegative = NEGATIVE_KEYWORDS.some((keyword) =>
      text.includes(keyword.toLowerCase()),
    );

    if (hasPositive && !hasNegative) {
      positiveCount++;
    } else if (hasNegative && !hasPositive) {
      negativeCount++;
    }
  }

  if (positiveCount === 0 && negativeCount === 0) return THRESHOLDS.DEFAULT_SCORE;

  const total = positiveCount + negativeCount;
  return clamp(
    positiveCount / total,
    THRESHOLDS.MIN_NORMALIZED,
    THRESHOLDS.MAX_NORMALIZED,
  );
}

const LOOKBACK_MS = NEWS_LOOKBACK_HOURS * 60 * 60 * 1000;
const HTML_TAG_REGEX = /<[^>]*>/g;
const MULTI_SPACE_REGEX = /\s+/g;
const KEYWORD_SPLIT_REGEX = /[\\/·(),&]+/g;
const COMPACT_TEXT_REGEX = /[\\s\\/·(),&-]+/g;
const HANGUL_REGEX = /[ㄱ-ㅎ가-힣]/;
const ACRONYM_REGEX = /[A-Z0-9]+/g;
const ACRONYM_TEST_REGEX = /[A-Z0-9]+/;
const ASCII_ONLY_REGEX = /^[a-z0-9]+$/i;
const IGNORED_KEYWORDS = new Set([
  "테마",
  "섹터",
  "관련",
  "관련주",
  "주식",
]);
const SHORT_ASCII_ALLOWLIST = new Set(["ai", "ev", "k2", "k9", "l2"]);
const MIN_ASCII_KEYWORD_LENGTH = 3;
const RECENCY_MIN_WEIGHT = 0.2;
const CORE_MATCH_PENALTY = 0.6;
const DENSITY_BASE_WEIGHT = 0.55;
const DENSITY_STEP = 0.08;
const DENSITY_MAX_WEIGHT = 1;
const VOLUME_BASE_RATIO = 0.08;
const LETTER_NAME_MAP: Record<string, string> = {
  A: "에이",
  B: "비",
  C: "씨",
  D: "디",
  E: "이",
  F: "에프",
  G: "지",
  H: "에이치",
  I: "아이",
  J: "제이",
  K: "케이",
  L: "엘",
  M: "엠",
  N: "엔",
  O: "오",
  P: "피",
  Q: "큐",
  R: "알",
  S: "에스",
  T: "티",
  U: "유",
  V: "브이",
  W: "더블유",
  X: "엑스",
  Y: "와이",
  Z: "지",
};

const allNewsCache = new Map<string, NewsCacheEntry>();
const allNewsInFlight = new Map<string, Promise<NewsItem[]>>();

function stripHtml(value: string): string {
  return value.replace(HTML_TAG_REGEX, " ");
}

function normalizeText(value: string): string {
  return stripHtml(value)
    .replace(/&quot;|&apos;|&amp;|&lt;|&gt;/gi, " ")
    .replace(MULTI_SPACE_REGEX, " ")
    .trim()
    .toLowerCase();
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function matchesKeyword(text: string, keyword: string): boolean {
  if (!keyword) return false;
  if (ASCII_ONLY_REGEX.test(keyword)) {
    const regex = new RegExp(
      `(?:^|[^A-Za-z0-9])${escapeRegExp(keyword)}(?:$|[^A-Za-z0-9])`,
      "i",
    );
    return regex.test(text);
  }
  return text.includes(keyword);
}

function acronymToKorean(value: string): string | null {
  let result = "";
  for (const char of value) {
    if (/[0-9]/.test(char)) {
      result += char;
      continue;
    }
    const mapped = LETTER_NAME_MAP[char];
    if (!mapped) return null;
    result += mapped;
  }
  return result || null;
}

function expandAcronymVariant(value: string): string | null {
  const hasAcronym = ACRONYM_TEST_REGEX.test(value);
  if (!hasAcronym) return null;

  const allowSingle = HANGUL_REGEX.test(value);
  let replaced = false;
  const nextValue = value.replace(ACRONYM_REGEX, (match) => {
    if (!allowSingle && match.length < 2) {
      return match;
    }
    const converted = acronymToKorean(match);
    if (!converted) return match;
    replaced = true;
    return converted;
  });

  if (!replaced || nextValue === value) return null;
  return nextValue;
}

function expandKeywords(keyword: string): string[] {
  const trimmed = keyword.trim();
  if (!trimmed) return [];
  const parts = trimmed
    .split(KEYWORD_SPLIT_REGEX)
    .map((part) => part.trim())
    .filter(Boolean);
  const variants = new Set<string>([trimmed, ...parts]);
  for (const variant of Array.from(variants)) {
    const expanded = expandAcronymVariant(variant);
    if (expanded) {
      variants.add(expanded);
    }
  }
  return Array.from(variants);
}

function buildKeywordSet(baseKeywords: string[]): string[] {
  const keywords = new Set<string>();

  for (const keyword of baseKeywords) {
    for (const variant of expandKeywords(keyword)) {
      const normalized = normalizeText(variant);
      if (normalized.length < 2 || IGNORED_KEYWORDS.has(normalized)) {
        continue;
      }
      if (
        ASCII_ONLY_REGEX.test(normalized) &&
        normalized.length < MIN_ASCII_KEYWORD_LENGTH &&
        !SHORT_ASCII_ALLOWLIST.has(normalized)
      ) {
        continue;
      }
      keywords.add(normalized);
      const compact = normalized.replace(COMPACT_TEXT_REGEX, "");
      if (compact.length >= 2 && compact !== normalized) {
        keywords.add(compact);
      }
    }
  }

  return Array.from(keywords);
}

const ISSUE_KEYWORD_MAP = new Map<string, string[]>(
  Object.entries(ISSUE_KEYWORDS).map(([issue, keywords]) => [
    issue,
    buildKeywordSet(keywords),
  ]),
);

function resolveIssueTypes(text: string, compactText: string): string[] {
  const issueTypes: string[] = [];
  for (const [issue, keywords] of ISSUE_KEYWORD_MAP.entries()) {
    if (keywords.length === 0) continue;
    if (countKeywordHits(text, compactText, keywords) > 0) {
      issueTypes.push(issue);
    }
  }
  return issueTypes.length > 0 ? issueTypes : [ISSUE_FALLBACK_TYPE];
}

function buildGroupKeywords(group: Group): string[] {
  const themeKeywords = THEME_KEYWORDS[group.name] ?? [];
  return buildKeywordSet([
    group.name,
    ...themeKeywords,
    ...(group.newsKeywords ?? []),
    ...group.stocks.map((stock) => stock.name),
  ]);
}

function buildCoreKeywords(group: Group): string[] {
  const themeKeywords = THEME_KEYWORDS[group.name] ?? [];
  return buildKeywordSet([
    group.name,
    ...themeKeywords,
    ...(group.newsKeywords ?? []),
  ]);
}

function parseNewsDate(value?: string): number | null {
  if (!value) return null;
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? null : timestamp;
}

function buildAllNewsCacheKey(config: NewsConfig): string {
  const sources = [...config.enabledSources].sort().join(",");
  const keywords = GENERAL_NEWS_KEYWORDS.map((keyword) => keyword.trim())
    .filter(Boolean)
    .join(",");
  return `${sources}|${keywords}|${NEWS_LOOKBACK_HOURS}|${ALL_NEWS_ITEMS_PER_SOURCE}`;
}

function getRecencyWeight(publishedAt: number): number {
  if (!publishedAt) return 0;
  const age = Date.now() - publishedAt;
  if (age <= 0) return 1;
  const ratio = 1 - age / LOOKBACK_MS;
  return clamp(ratio, RECENCY_MIN_WEIGHT, 1);
}

function countKeywordHits(
  text: string,
  compactText: string,
  keywords: string[],
): number {
  let hits = 0;
  for (const keyword of keywords) {
    const isAscii = ASCII_ONLY_REGEX.test(keyword);
    if (matchesKeyword(text, keyword)) {
      hits++;
      continue;
    }
    if (isAscii) {
      if (keyword.length >= MIN_ASCII_KEYWORD_LENGTH && compactText.includes(keyword)) {
        hits++;
      }
      continue;
    }
    if (compactText.includes(keyword)) {
      hits++;
    }
  }
  return hits;
}

function computeWeightedSentiment(
  items: Array<{ text: string; weight: number }>,
): number {
  if (items.length === 0) return THRESHOLDS.DEFAULT_SCORE;

  let positiveWeight = 0;
  let negativeWeight = 0;

  for (const item of items) {
    if (!item.text) continue;
    const hasPositive = POSITIVE_KEYWORDS.some((keyword) =>
      item.text.includes(keyword.toLowerCase()),
    );
    const hasNegative = NEGATIVE_KEYWORDS.some((keyword) =>
      item.text.includes(keyword.toLowerCase()),
    );

    if (hasPositive && !hasNegative) {
      positiveWeight += item.weight;
    } else if (hasNegative && !hasPositive) {
      negativeWeight += item.weight;
    }
  }

  if (positiveWeight === 0 && negativeWeight === 0) {
    return THRESHOLDS.DEFAULT_SCORE;
  }

  const total = positiveWeight + negativeWeight;
  return clamp(
    positiveWeight / total,
    THRESHOLDS.MIN_NORMALIZED,
    THRESHOLDS.MAX_NORMALIZED,
  );
}

export async function fetchJson(
  url: string,
  options?: RequestInit,
): Promise<unknown> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), NEWS_TIMEOUT_MS);

  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
    });
    if (!response.ok) {
      return null;
    }
    return await response.json();
  } catch {
    return null;
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  fn: (item: T) => Promise<R>,
): Promise<R[]> {
  const results: R[] = [];
  for (let i = 0; i < items.length; i += concurrency) {
    const batch = items.slice(i, i + concurrency);
    const batchResults = await Promise.all(batch.map(fn));
    results.push(...batchResults);
  }
  return results;
}

export async function fetchNaverNews(
  query: string,
  config: NonNullable<NewsConfig["naver"]>,
): Promise<ProviderSignal | null> {
  const startTime = Date.now();
  const params = new URLSearchParams({
    query,
    display: String(NEWS_ITEMS_PER_SOURCE),
    sort: "date",
  });
  const data = await fetchJson(
    `https://openapi.naver.com/v1/search/news.json?${params.toString()}`,
    {
      headers: {
        "X-Naver-Client-Id": config.clientId,
        "X-Naver-Client-Secret": config.clientSecret,
      },
    },
  );
  const duration = Date.now() - startTime;
  
  if (!data) {
    return null;
  }

  const items = Array.isArray((data as { items?: unknown[] })?.items)
    ? (data as { items: Array<{ title?: string; description?: string }> }).items
    : [];
  const texts = items.map(
    (item) => `${item.title ?? ""} ${item.description ?? ""}`,
  );
  const result: ProviderSignal = {
    source: "naver" as NewsSource,
    count: texts.length,
    sentiment: computeSentiment(texts),
  };
  
  return result;
}

export async function fetchDaumNews(
  query: string,
  config: NonNullable<NewsConfig["daum"]>,
): Promise<ProviderSignal | null> {
  const startTime = Date.now();
  const params = new URLSearchParams({
    query,
    size: String(NEWS_ITEMS_PER_SOURCE),
    sort: "recency",
  });
  const data = await fetchJson(
    `https://dapi.kakao.com/v2/search/news?${params.toString()}`,
    {
      headers: {
        Authorization: `KakaoAK ${config.apiKey}`,
      },
    },
  );
  const duration = Date.now() - startTime;
  
  if (!data) {
    return null;
  }

  const items = Array.isArray((data as { documents?: unknown[] })?.documents)
    ? (data as {
        documents: Array<{ title?: string; contents?: string }>;
      }).documents
    : [];
  const texts = items.map(
    (item) => `${item.title ?? ""} ${item.contents ?? ""}`,
  );
  const result: ProviderSignal = {
    source: "daum" as NewsSource,
    count: texts.length,
    sentiment: computeSentiment(texts),
  };
  
  return result;
}

export async function fetchNewsApiNews(
  query: string,
  config: NonNullable<NewsConfig["newsapi"]>,
): Promise<ProviderSignal | null> {
  const startTime = Date.now();
  const params = new URLSearchParams({
    q: query,
    pageSize: String(NEWS_ITEMS_PER_SOURCE),
    sortBy: "publishedAt",
    language: "ko",
  });
  const data = await fetchJson(
    `https://newsapi.org/v2/everything?${params.toString()}`,
    {
      headers: {
        "X-Api-Key": config.apiKey,
      },
    },
  );
  const duration = Date.now() - startTime;
  
  if (!data) {
    return null;
  }

  const items = Array.isArray((data as { articles?: unknown[] })?.articles)
    ? (data as { articles: Array<{ title?: string; description?: string }> })
        .articles
    : [];
  const texts = items.map(
    (item) => `${item.title ?? ""} ${item.description ?? ""}`,
  );
  const result: ProviderSignal = {
    source: "newsapi" as NewsSource,
    count: texts.length,
    sentiment: computeSentiment(texts),
  };
  
  return result;
}

export async function fetchNewsSignal(
  query: string,
  config: NewsConfig,
): Promise<NewsSignal | null> {
  const tasks: Array<Promise<ProviderSignal | null>> = [];
  if (config.naver) {
    tasks.push(fetchNaverNews(query, config.naver));
  }
  if (config.daum) {
    tasks.push(fetchDaumNews(query, config.daum));
  }
  if (config.newsapi) {
    tasks.push(fetchNewsApiNews(query, config.newsapi));
  }

  const results = (await Promise.all(tasks)).filter(
    Boolean,
  ) as ProviderSignal[];
  if (results.length === 0) {
    return null;
  }

  const totalCount = results.reduce((sum, result) => sum + result.count, 0);
  if (totalCount === 0) {
    return null;
  }

  const sentimentScore =
    results.reduce((sum, result) => sum + result.sentiment * result.count, 0) /
    totalCount;
  const volumeScore = clamp(
    totalCount / (NEWS_ITEMS_PER_SOURCE * results.length),
    THRESHOLDS.MIN_NORMALIZED,
    THRESHOLDS.MAX_NORMALIZED,
  );
  const score = clamp(
    THRESHOLDS.DEFAULT_SCORE +
      THRESHOLDS.DEFAULT_SCORE * sentimentScore * volumeScore,
    THRESHOLDS.MIN_NORMALIZED,
    THRESHOLDS.MAX_NORMALIZED,
  );

  const sources = results.map((result) => result.source);

  return {
    query,
    totalCount,
    volumeScore,
    sentimentScore,
    score,
    sources,
    issueTypes: [],
  };
}

// 전체 뉴스를 가져오는 함수
export async function fetchAllNews(config: NewsConfig): Promise<NewsItem[]> {
  if (config.enabledSources.length === 0) return [];

  const cacheKey = buildAllNewsCacheKey(config);
  const cached = allNewsCache.get(cacheKey);
  const now = Date.now();
  if (cached && now - cached.fetchedAt < NEWS_CACHE_TTL_MS) {
    return cached.data;
  }

  const inFlight = allNewsInFlight.get(cacheKey);
  if (inFlight) {
    return inFlight;
  }

  const task = fetchAllNewsUncached(config)
    .then((data) => {
      allNewsCache.set(cacheKey, { data, fetchedAt: Date.now() });
      return data;
    })
    .finally(() => {
      allNewsInFlight.delete(cacheKey);
    });

  allNewsInFlight.set(cacheKey, task);
  return task;
}

async function fetchAllNewsUncached(config: NewsConfig): Promise<NewsItem[]> {
  const allNews: NewsItem[] = [];
  const tasks: Array<() => Promise<NewsItem[]>> = [];
  const keywords = Array.from(
    new Set(GENERAL_NEWS_KEYWORDS.map((keyword) => keyword.trim()).filter(Boolean)),
  );

  for (const keyword of keywords) {
    const naver = config.naver;
    if (naver) {
      tasks.push(() => fetchNaverNewsItems(keyword, naver));
    }
    const daum = config.daum;
    if (daum) {
      tasks.push(() => fetchDaumNewsItems(keyword, daum));
    }
    const newsapi = config.newsapi;
    if (newsapi) {
      tasks.push(() => fetchNewsApiNewsItems(keyword, newsapi));
    }
  }

  const results = await mapWithConcurrency(
    tasks,
    NEWS_CONCURRENCY,
    async (taskFn) => {
      try {
        return await taskFn();
      } catch {
        return [];
      }
    },
  );

  for (const newsItems of results) {
    allNews.push(...newsItems);
  }

  // 중복 제거 (정규화된 제목 기준)
  const deduped = new Map<string, NewsItem>();
  for (const item of allNews) {
    const key = normalizeText(item.title);
    if (!key) continue;
    if (!deduped.has(key)) {
      deduped.set(key, item);
    }
  }
  const uniqueNews = Array.from(deduped.values());

  const cutoff = Date.now() - LOOKBACK_MS;
  const recentNews = uniqueNews.filter((item) => item.publishedAt >= cutoff);

  return recentNews;
}

// 네이버 뉴스 아이템 가져오기
async function fetchNaverNewsItems(
  query: string,
  config: NonNullable<NewsConfig["naver"]>,
): Promise<NewsItem[]> {
  const params = new URLSearchParams({
    query,
    display: String(ALL_NEWS_ITEMS_PER_SOURCE),
    sort: "date",
  });
  const data = await fetchJson(
    `https://openapi.naver.com/v1/search/news.json?${params.toString()}`,
    {
      headers: {
        "X-Naver-Client-Id": config.clientId,
        "X-Naver-Client-Secret": config.clientSecret,
      },
    },
  );
  if (!data) return [];

  const items = Array.isArray((data as { items?: unknown[] })?.items)
    ? (data as {
        items: Array<{ title?: string; description?: string; pubDate?: string }>;
      }).items
    : [];
  return items.map((item) => ({
    title: item.title ?? "",
    description: item.description ?? "",
    source: "naver" as NewsSource,
    publishedAt: parseNewsDate(item.pubDate) ?? 0,
  }));
}

// 다음 뉴스 아이템 가져오기
async function fetchDaumNewsItems(
  query: string,
  config: NonNullable<NewsConfig["daum"]>,
): Promise<NewsItem[]> {
  const params = new URLSearchParams({
    query,
    size: String(ALL_NEWS_ITEMS_PER_SOURCE),
    sort: "recency",
  });
  const data = await fetchJson(
    `https://dapi.kakao.com/v2/search/news?${params.toString()}`,
    {
      headers: {
        Authorization: `KakaoAK ${config.apiKey}`,
      },
    },
  );
  if (!data) return [];

  const items = Array.isArray((data as { documents?: unknown[] })?.documents)
    ? (data as {
        documents: Array<{ title?: string; contents?: string; datetime?: string }>;
      }).documents
    : [];
  return items.map((item) => ({
    title: item.title ?? "",
    description: item.contents ?? "",
    source: "daum" as NewsSource,
    publishedAt: parseNewsDate(item.datetime) ?? 0,
  }));
}

// NewsAPI 뉴스 아이템 가져오기
async function fetchNewsApiNewsItems(
  query: string,
  config: NonNullable<NewsConfig["newsapi"]>,
): Promise<NewsItem[]> {
  const params = new URLSearchParams({
    q: query,
    pageSize: String(ALL_NEWS_ITEMS_PER_SOURCE),
    sortBy: "publishedAt",
    language: "ko",
  });
  const data = await fetchJson(
    `https://newsapi.org/v2/everything?${params.toString()}`,
    {
      headers: {
        "X-Api-Key": config.apiKey,
      },
    },
  );
  if (!data) return [];

  const items = Array.isArray((data as { articles?: unknown[] })?.articles)
    ? (data as {
        articles: Array<{ title?: string; description?: string; publishedAt?: string }>;
      }).articles
    : [];
  return items.map((item) => ({
    title: item.title ?? "",
    description: item.description ?? "",
    source: "newsapi" as NewsSource,
    publishedAt: parseNewsDate(item.publishedAt) ?? 0,
  }));
}

// 전체 뉴스에서 그룹별 신호 생성
export async function buildNewsSignals(
  groups: Array<Group>,
  config: NewsConfig,
): Promise<Map<string, NewsSignal>> {
  const signals = new Map<string, NewsSignal>();
  if (groups.length === 0 || config.enabledSources.length === 0) {
    return signals;
  }

  // 전체 뉴스 가져오기
  const allNews = await fetchAllNews(config);
  if (allNews.length === 0) {
    return signals;
  }

  const normalizedNews = allNews.map((news) => {
    const text = normalizeText(`${news.title} ${news.description}`);
    return {
      ...news,
      text,
      compactText: text.replace(COMPACT_TEXT_REGEX, ""),
    };
  });
  const totalWeight = normalizedNews.reduce(
    (sum, news) => sum + getRecencyWeight(news.publishedAt),
    0,
  );

  const keywordMap = new Map<
    string,
    { keywords: string[]; coreKeywords: string[] }
  >();

  for (const group of groups) {
    keywordMap.set(group.id, {
      keywords: buildGroupKeywords(group),
      coreKeywords: buildCoreKeywords(group),
    });
  }

  // 각 그룹별로 뉴스에서 언급 빈도 계산
  for (const group of groups) {
    const entry = keywordMap.get(group.id);
    const keywords = entry?.keywords ?? [];
    const coreKeywords = entry?.coreKeywords ?? [];
    if (keywords.length === 0) continue;

    const matchedNews = normalizedNews
      .map((news) => {
        const hitCount = countKeywordHits(
          news.text,
          news.compactText,
          keywords,
        );
        if (hitCount === 0) return null;
        const coreHit =
          coreKeywords.length === 0
            ? true
            : countKeywordHits(news.text, news.compactText, coreKeywords) > 0;
        const recencyWeight = getRecencyWeight(news.publishedAt);
        if (recencyWeight === 0) return null;
        const densityWeight = clamp(
          DENSITY_BASE_WEIGHT + DENSITY_STEP * hitCount,
          DENSITY_BASE_WEIGHT,
          DENSITY_MAX_WEIGHT,
        );
        const coreWeight = coreHit ? 1 : CORE_MATCH_PENALTY;
        const issueTypes = resolveIssueTypes(news.text, news.compactText);
        return {
          ...news,
          weight: recencyWeight * densityWeight * coreWeight,
          issueTypes,
        };
      })
      .filter(Boolean) as Array<
      NewsItem & {
        text: string;
        compactText: string;
        weight: number;
        issueTypes: string[];
      }
    >;

    if (matchedNews.length === 0) {
      continue;
    }

    // 감성 분석 (가중치 적용)
    const sentimentScore = computeWeightedSentiment(
      matchedNews.map((news) => ({ text: news.text, weight: news.weight })),
    );

    const matchWeightSum = matchedNews.reduce(
      (sum, news) => sum + news.weight,
      0,
    );

    // 볼륨 점수 계산 (전체 뉴스 대비 가중 비율)
    const volumeScore = clamp(
      matchWeightSum / (Math.max(totalWeight, 1) * VOLUME_BASE_RATIO),
      THRESHOLDS.MIN_NORMALIZED,
      THRESHOLDS.MAX_NORMALIZED,
    );

    // 최종 점수 계산
    const score = clamp(
      THRESHOLDS.DEFAULT_SCORE +
        THRESHOLDS.DEFAULT_SCORE * sentimentScore * volumeScore,
      THRESHOLDS.MIN_NORMALIZED,
      THRESHOLDS.MAX_NORMALIZED,
    );

    // 소스 추출
    const sources = Array.from(
      new Set(matchedNews.map((news) => news.source)),
    ) as NewsSource[];

    const issueTypeStats = new Map<string, { weightSum: number }>();
    for (const item of matchedNews) {
      for (const issueType of item.issueTypes) {
        const current = issueTypeStats.get(issueType) ?? { weightSum: 0 };
        issueTypeStats.set(issueType, {
          weightSum: current.weightSum + item.weight,
        });
      }
    }
    const issueTypes = Array.from(issueTypeStats.entries())
      .sort((a, b) => b[1].weightSum - a[1].weightSum)
      .slice(0, 3)
      .map(([issueType]) => issueType);

    const signal: NewsSignal = {
      query: group.name,
      totalCount: matchedNews.length,
      volumeScore,
      sentimentScore,
      score,
      sources,
      issueTypes,
    };

    signals.set(group.id, signal);
  }

  return signals;
}
