import { notFound } from "next/navigation";
import type { Market, Category } from "@/types";
import KrxMomentumScreener from "@/components/KrxMomentumScreener";

type Props = {
  params: Promise<{
    market: string;
    category: string;
  }>;
  searchParams: Promise<{
    [key: string]: string | string[] | undefined;
  }>;
};

function parseMarket(value: string): Market | null {
  const upper = value.toUpperCase();
  return upper === "KOSPI" || upper === "KOSDAQ" ? upper : null;
}

function parseCategory(value: string): Category | null {
  return value === "sector" || value === "theme" ? value : null;
}

export async function generateStaticParams() {
  return [
    { market: "kospi", category: "sector" },
    { market: "kospi", category: "theme" },
    { market: "kosdaq", category: "sector" },
    { market: "kosdaq", category: "theme" },
  ];
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ market: string; category: string }>;
}) {
  const { market, category } = await params;
  const marketValue = parseMarket(market);
  const categoryValue = parseCategory(category);

  if (!marketValue || !categoryValue) {
    return {
      title: "모멘텀 스크리너",
    };
  }

  const { MARKET_LABELS, CATEGORY_LABELS } = await import("@/constants");
  const title = `${MARKET_LABELS[marketValue]} ${CATEGORY_LABELS[categoryValue]} 모멘텀`;

  return {
    title,
    description: `${title} - 코스피·코스닥의 섹터/테마 모멘텀과 시가총액 상위 종목을 정리합니다.`,
  };
}

export default async function MarketCategoryPage({ params }: Props) {
  const { market, category } = await params;
  const marketValue = parseMarket(market);
  const categoryValue = parseCategory(category);

  if (!marketValue || !categoryValue) {
    notFound();
  }

  return (
    <KrxMomentumScreener
      initialMarket={marketValue}
      initialCategory={categoryValue}
    />
  );
}
