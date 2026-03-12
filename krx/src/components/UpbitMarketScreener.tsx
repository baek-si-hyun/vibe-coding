"use client";

import CoinMarketScreener from "@/components/CoinMarketScreener";

export default function UpbitMarketScreener() {
  return (
    <CoinMarketScreener
      storeKey="upbit"
      exchangeName="업비트"
      apiBase="/api/upbit/screener"
      title="업비트 코인 퀀트 리포트"
      description="공식 Upbit REST API 기준 1시간봉 점수를 계산하고, 총점과 세부 팩터를 주식 퀀트처럼 비교할 수 있게 정리했습니다."
      tradeHref={(symbol) => `https://upbit.com/exchange?code=CRIX.UPBIT.KRW-${symbol}`}
      tradeLabel="업비트 열기"
    />
  );
}
