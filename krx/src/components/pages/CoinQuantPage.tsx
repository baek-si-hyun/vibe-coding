"use client";

import BithumbMarketScreener from "@/components/BithumbMarketScreener";
import UpbitMarketScreener from "@/components/UpbitMarketScreener";
import {
  useCoinQuantStore,
  type CoinQuantScreenKey,
} from "@/stores/coinQuantStore";

const switchButtonClass =
  "inline-flex items-center justify-center rounded-lg border px-4 py-2 text-sm font-medium transition";

export default function CoinQuantPage() {
  const activeExchange = useCoinQuantStore((state) => state.activeExchange);
  const setActiveExchange = useCoinQuantStore((state) => state.setActiveExchange);

  const renderExchange = () => {
    switch (activeExchange) {
      case "upbit":
        return <UpbitMarketScreener />;
      case "bithumb":
      default:
        return <BithumbMarketScreener />;
    }
  };

  const exchangeButtons: Array<{ key: CoinQuantScreenKey; label: string; note: string }> = [
    { key: "bithumb", label: "빗썸", note: "KRW 유동성 중심" },
    { key: "upbit", label: "업비트", note: "REST 스냅샷 기준" },
  ];

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-slate-200 bg-white p-3 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Coin Quant</div>
            <div className="mt-1 text-sm text-slate-600">
              상단 탭은 하나로 통합하고, 거래소 전환은 이 안에서 처리합니다. 두 거래소 모두 총점 기반 추천으로 동일한 방식으로 비교합니다.
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {exchangeButtons.map((button) => {
              const selected = activeExchange === button.key;
              return (
                <button
                  key={button.key}
                  type="button"
                  onClick={() => setActiveExchange(button.key)}
                  className={`${switchButtonClass} ${
                    selected
                      ? "border-blue-600 bg-blue-600 text-white"
                      : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
                  }`}
                  aria-pressed={selected}
                >
                  {button.label}
                  <span className={`ml-2 text-xs ${selected ? "text-blue-100" : "text-slate-400"}`}>{button.note}</span>
                </button>
              );
            })}
          </div>
        </div>
      </div>

      {renderExchange()}
    </div>
  );
}
