"use client";

import CoinMarketScreener from "@/components/CoinMarketScreener";

export default function BithumbMarketScreener() {
  return (
    <CoinMarketScreener
      storeKey="bithumb"
      exchangeName="빗썸"
      apiBase="/api/bithumb/screener"
      title="빗썸 코인 퀀트 리포트"
      description="1시간봉 기준 총점과 세부 팩터를 주식 퀀트처럼 카드형 구조로 정리했습니다. 핵심은 모멘텀, 거래대금, breadth, 리스크를 한 화면에서 바로 판단하는 것입니다."
      tradeHref={(symbol) => `https://www.bithumb.com/trade/order/${symbol}_KRW`}
      tradeLabel="빗썸 열기"
    />
  );
}
