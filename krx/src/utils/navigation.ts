import type { Market, Category } from "@/types";

/**
 * 시장과 카테고리를 URL 경로로 변환
 */
export function buildMarketCategoryPath(
  market: Market,
  category: Category,
): string {
  return `/${market.toLowerCase()}/${category}`;
}

/**
 * URL 경로에서 시장과 카테고리 파싱
 */
export function parseMarketCategoryPath(
  path: string,
): { market: Market | null; category: Category | null } {
  const parts = path.split("/").filter(Boolean);
  if (parts.length < 2) {
    return { market: null, category: null };
  }

  const marketStr = parts[0]?.toUpperCase();
  const categoryStr = parts[1];

  const market =
    marketStr === "KOSPI" || marketStr === "KOSDAQ" ? marketStr : null;
  const category = categoryStr === "sector" || categoryStr === "theme" ? categoryStr : null;

  return { market, category };
}
