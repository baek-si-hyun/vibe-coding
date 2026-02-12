import type { Market, Category } from "@/types";

export function buildMarketCategoryPath(
  market: Market,
  category: Category,
): string {
  return `/${market.toLowerCase()}/${category}`;
}

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
