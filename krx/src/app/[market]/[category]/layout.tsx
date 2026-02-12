import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "모멘텀 스크리너",
  description: "코스피·코스닥의 섹터/테마 모멘텀과 시가총액 상위 종목을 정리합니다.",
};

export default function MarketCategoryLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <>{children}</>;
}
