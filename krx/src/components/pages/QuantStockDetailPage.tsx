"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";

type MarketFilter = "ALL" | "KOSPI" | "KOSDAQ";

type QuantItem = {
  rank: number;
  market: string;
  code: string;
  name: string;
  as_of: string;
  close: number;
  market_cap: number;
  turnover: number;
  turnover_ratio: number;
  return_1d: number;
  return_5d?: number;
  return_10d?: number;
  return_20d: number;
  return_60d: number;
  return_120d: number;
  volatility_20d: number;
  volatility_60d?: number;
  downside_volatility_20d?: number;
  drawdown_60d?: number;
  drawdown_120d?: number;
  avg_turnover_ratio_20d?: number;
  momentum_score: number;
  liquidity_score: number;
  stability_score: number;
  trend_score?: number;
  risk_adjusted_score?: number;
  total_score: number;
};

type ReportResponse = {
  generated_at: string;
  market: string;
  min_market_cap: number;
  limit: number;
  universe_count: number;
  as_of_min: string;
  as_of_max: string;
  items?: QuantItem[];
  report_markdown?: string;
};

type Signal = {
  label: string;
  toneClass: string;
};

const MIN_MARKET_CAP = 1_000_000_000_000;
const DETAIL_LIMIT = 300;

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function sanitizeMarket(raw: string): MarketFilter {
  const value = raw.trim().toUpperCase();
  if (value === "KOSPI" || value === "KOSDAQ" || value === "ALL") return value;
  return "ALL";
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return value.toLocaleString("ko-KR");
}

function formatSignedPct(value: number): string {
  if (!Number.isFinite(value)) return "-";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}%`;
}

function formatPct(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${value.toFixed(2)}%`;
}

function formatOptionalPct(value?: number): string {
  return Number.isFinite(value) ? formatPct(value ?? 0) : "-";
}

function formatOptionalSignedPct(value?: number): string {
  return Number.isFinite(value) ? formatSignedPct(value ?? 0) : "-";
}

function formatMarketCap(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  if (value >= 1_0000_0000_0000) return `${(value / 1_0000_0000_0000).toFixed(2)}조`;
  if (value >= 1_0000_0000) return `${(value / 1_0000_0000).toFixed(0)}억`;
  return formatNumber(value);
}

function formatPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return `${Math.round(value).toLocaleString("ko-KR")}원`;
}

function formatDateCompact(raw: string): string {
  const value = raw.trim();
  if (value.length !== 8) return value || "-";
  return `${value.slice(0, 4)}.${value.slice(4, 6)}.${value.slice(6, 8)}`;
}

function weightFromScore(score: number, rank: number): number {
  const scoreWeight = clamp((score - 55) * 0.35, 0, 15);
  const rankPenalty = clamp((rank - 1) * 0.4, 0, 7);
  return clamp(scoreWeight - rankPenalty + 3, 1, 15);
}

function buildSignal(item: QuantItem): Signal {
  if (item.total_score >= 85) return { label: "STRONG BUY", toneClass: "text-emerald-700 bg-emerald-100 border-emerald-200" };
  if (item.total_score >= 75) return { label: "BUY", toneClass: "text-cyan-700 bg-cyan-100 border-cyan-200" };
  if (item.total_score >= 65) return { label: "WATCH", toneClass: "text-amber-700 bg-amber-100 border-amber-200" };
  if (item.return_20d < -10 || item.total_score < 45) {
    return { label: "REDUCE", toneClass: "text-rose-700 bg-rose-100 border-rose-200" };
  }
  return { label: "HOLD", toneClass: "text-slate-700 bg-slate-100 border-slate-200" };
}

function extractStockLine(markdown: string, code: string): string {
  if (!markdown || !code) return "";
  const lines = markdown.split("\n");
  const target = `(${code})`;
  for (const line of lines) {
    const trimmed = line.trim();
    if (/^\|\d+\|/.test(trimmed) && trimmed.includes(target)) {
      return trimmed;
    }
  }
  return "";
}

function buildStrengths(item: QuantItem): string[] {
  const strengths: string[] = [];
  if (item.total_score >= 75) strengths.push(`총점 ${item.total_score.toFixed(2)}로 상위권입니다.`);
  if (item.return_20d > 0 && item.return_60d > 0) strengths.push("20일/60일 수익률이 모두 플러스입니다.");
  if (item.momentum_score >= 70) strengths.push(`모멘텀 점수 ${item.momentum_score.toFixed(1)}로 추세 우위입니다.`);
  if (item.stability_score >= 60) strengths.push(`안정성 점수 ${item.stability_score.toFixed(1)}로 변동성 부담이 낮은 편입니다.`);
  if (strengths.length === 0) strengths.push("점수는 중립 구간이며 추가 확인이 필요합니다.");
  return strengths.slice(0, 3);
}

function buildRisks(item: QuantItem): string[] {
  const risks: string[] = [];
  if (item.return_1d <= -2) risks.push(`1일 수익률이 ${formatSignedPct(item.return_1d)}로 단기 충격이 있습니다.`);
  if (item.return_20d < 0) risks.push(`20일 수익률 ${formatSignedPct(item.return_20d)}로 중기 추세가 약합니다.`);
  if (item.volatility_20d >= 4.5) risks.push(`20일 변동성 ${formatPct(item.volatility_20d)}로 변동폭이 큽니다.`);
  if (item.liquidity_score < 45) risks.push(`유동성 점수 ${item.liquidity_score.toFixed(1)}로 체결 리스크가 존재합니다.`);
  if (risks.length === 0) risks.push("지표상 큰 위험 신호는 제한적입니다. 손절 규칙만 고정 유지하세요.");
  return risks.slice(0, 3);
}

function buildNarrative(item: QuantItem, signal: Signal, weight: number): string {
  return `${item.name}(${item.code})는 ${signal.label} 구간이며 권장 비중은 ${weight.toFixed(
    1
  )}% 수준입니다. 20일 ${formatSignedPct(item.return_20d)}, 60일 ${formatSignedPct(
    item.return_60d
  )}, 변동성 ${formatPct(item.volatility_20d)} 기준으로 추세/리스크를 동시에 관리해야 합니다.`;
}

export default function QuantStockDetailPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const code = (searchParams.get("code") || "").trim().toUpperCase();
  const requestedName = (searchParams.get("name") || "").trim();
  const requestedMarket = sanitizeMarket(searchParams.get("market") || "ALL");

  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [reportData, setReportData] = useState<ReportResponse | null>(null);

  const fetchDetail = useCallback(async () => {
    if (!code) {
      setError("종목 코드가 비어 있습니다.");
      setReportData(null);
      return;
    }

    setLoading(true);
    setError("");

    try {
      const params = new URLSearchParams({
        market: requestedMarket,
        limit: String(DETAIL_LIMIT),
        min_market_cap: String(MIN_MARKET_CAP),
      });
      const res = await fetch(`/api/quant/report?${params.toString()}`, {
        method: "GET",
        cache: "no-store",
      });
      const json = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(json.error || "상세 리포트를 불러오지 못했습니다.");
      }
      setReportData(json as ReportResponse);
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "상세 리포트 조회 중 오류가 발생했습니다.");
    } finally {
      setLoading(false);
    }
  }, [code, requestedMarket]);

  useEffect(() => {
    void fetchDetail();
  }, [fetchDetail]);

  const items = useMemo(() => reportData?.items ?? [], [reportData?.items]);

  const target = useMemo(() => {
    if (!code) return null;
    const exact = items.find(
      (item) => item.code.toUpperCase() === code && (requestedMarket === "ALL" || item.market === requestedMarket)
    );
    if (exact) return exact;
    return items.find((item) => item.code.toUpperCase() === code) ?? null;
  }, [code, items, requestedMarket]);

  const signal = useMemo(() => (target ? buildSignal(target) : null), [target]);
  const weight = useMemo(() => (target ? weightFromScore(target.total_score, target.rank) : 0), [target]);
  const strengths = useMemo(() => (target ? buildStrengths(target) : []), [target]);
  const risks = useMemo(() => (target ? buildRisks(target) : []), [target]);
  const markdownLine = useMemo(
    () => extractStockLine(reportData?.report_markdown || "", code),
    [code, reportData?.report_markdown]
  );
  const peers = useMemo(() => {
    if (!target) return [];
    return items
      .filter((item) => item.market === target.market)
      .slice(0, 8);
  }, [items, target]);
  const scoreBars = useMemo(
    () =>
      target
        ? [
            { label: "총점", value: target.total_score, colorClass: "bg-cyan-500" },
            { label: "모멘텀", value: target.momentum_score, colorClass: "bg-emerald-500" },
            { label: "추세 품질", value: target.trend_score ?? target.momentum_score, colorClass: "bg-sky-500" },
            { label: "위험조정", value: target.risk_adjusted_score ?? target.momentum_score, colorClass: "bg-violet-500" },
            { label: "유동성", value: target.liquidity_score, colorClass: "bg-indigo-500" },
            { label: "안정성", value: target.stability_score, colorClass: "bg-amber-500" },
          ]
        : [],
    [target]
  );

  const backToList = useCallback(() => {
    if (window.history.length > 1) {
      router.back();
      return;
    }
    router.push("/?tab=QUANT");
  }, [router]);

  return (
    <div className="min-h-screen bg-slate-50 px-4 py-8 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-6xl space-y-4">
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-700">Quant Detail Report</div>
              <h1 className="mt-1 text-2xl font-extrabold text-slate-900">
                {target?.name || requestedName || code || "종목 상세"}
                {code ? <span className="ml-2 text-sm font-semibold text-slate-500">{code}</span> : null}
              </h1>
              <div className="mt-1 text-sm text-slate-600">
                KOSPI/KOSDAQ 시총 1조 이상 유니버스 기준 종목별 상세 리포트
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={backToList}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              >
                추천 리스트로 돌아가기
              </button>
              <button
                type="button"
                onClick={() => void fetchDetail()}
                className="rounded-lg border border-cyan-300 bg-cyan-50 px-4 py-2 text-sm font-semibold text-cyan-800 hover:bg-cyan-100"
              >
                리포트 새로고침
              </button>
            </div>
          </div>
        </div>

        {loading ? (
          <div className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-500">상세 리포트를 불러오는 중입니다...</div>
        ) : null}

        {error ? (
          <div className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm font-medium text-rose-700">{error}</div>
        ) : null}

        {!loading && !error && !target ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
            현재 리포트 범위에서 {code || "해당 종목"}을 찾지 못했습니다. (시장: {requestedMarket})
          </div>
        ) : null}

        {target ? (
          <>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">신호/비중</div>
                <div className={`mt-2 inline-flex items-center rounded-md border px-2.5 py-1 text-xs font-bold ${signal?.toneClass || ""}`}>
                  {signal?.label}
                </div>
                <div className="mt-2 text-2xl font-bold text-slate-900">{weight.toFixed(1)}%</div>
                <div className="text-xs text-slate-500">추천 비중</div>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">순위/총점</div>
                <div className="mt-2 text-2xl font-bold text-slate-900">#{target.rank}</div>
                <div className="text-sm font-semibold text-cyan-700">{target.total_score.toFixed(2)}점</div>
                <div className="text-xs text-slate-500">{target.market}</div>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">기준일/종가</div>
                <div className="mt-2 text-lg font-bold text-slate-900">{formatDateCompact(target.as_of)}</div>
                <div className="text-sm font-semibold text-slate-700">{formatPrice(target.close)}</div>
                <div className="text-xs text-slate-500">보고서 생성 {reportData?.generated_at || "-"}</div>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">시가총액/거래대금비율</div>
                <div className="mt-2 text-lg font-bold text-slate-900">{formatMarketCap(target.market_cap)}</div>
                <div className="text-sm font-semibold text-slate-700">{formatPct(target.turnover_ratio)}</div>
                <div className="text-xs text-slate-500">턴오버 비율</div>
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
                <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-slate-700">수익률/변동성</h2>
                <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">1일</div>
                    <div className={`mt-1 text-lg font-bold ${target.return_1d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatSignedPct(target.return_1d)}
                    </div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">20일</div>
                    <div className={`mt-1 text-lg font-bold ${target.return_20d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatSignedPct(target.return_20d)}
                    </div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">60일</div>
                    <div className={`mt-1 text-lg font-bold ${target.return_60d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatSignedPct(target.return_60d)}
                    </div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">120일</div>
                    <div className={`mt-1 text-lg font-bold ${target.return_120d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatSignedPct(target.return_120d)}
                    </div>
                  </div>
                </div>
                <div className="mt-3 rounded-lg bg-slate-50 p-3 text-sm">
                  <div className="text-xs text-slate-500">20일 변동성</div>
                  <div className={`mt-1 text-lg font-bold ${target.volatility_20d >= 4.5 ? "text-amber-600" : "text-cyan-700"}`}>
                    {formatPct(target.volatility_20d)}
                  </div>
                </div>
                <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">5일</div>
                    <div className={`mt-1 text-lg font-bold ${(target.return_5d ?? 0) >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatOptionalSignedPct(target.return_5d)}
                    </div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">10일</div>
                    <div className={`mt-1 text-lg font-bold ${(target.return_10d ?? 0) >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatOptionalSignedPct(target.return_10d)}
                    </div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">60일 변동성</div>
                    <div className="mt-1 text-lg font-bold text-slate-800">{formatOptionalPct(target.volatility_60d)}</div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">60일 최대낙폭</div>
                    <div className={`mt-1 text-lg font-bold ${(target.drawdown_60d ?? 0) >= -12 ? "text-emerald-600" : "text-rose-600"}`}>
                      {formatOptionalSignedPct(target.drawdown_60d)}
                    </div>
                  </div>
                </div>
              </div>

              <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
                <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-slate-700">점수 분해</h2>
                <div className="mt-3 space-y-3">
                  {scoreBars.map((bar) => (
                    <div key={bar.label}>
                      <div className="mb-1 flex items-center justify-between text-xs text-slate-600">
                        <span>{bar.label}</span>
                        <span className="font-semibold">{bar.value.toFixed(2)}</span>
                      </div>
                      <div className="h-2 overflow-hidden rounded-full bg-slate-200">
                        <div className={`h-full ${bar.colorClass}`} style={{ width: `${clamp(bar.value, 0, 100)}%` }} />
                      </div>
                    </div>
                  ))}
                </div>
                <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">20일 평균 거래대금비율</div>
                    <div className="mt-1 text-lg font-bold text-slate-800">{formatOptionalPct(target.avg_turnover_ratio_20d)}</div>
                  </div>
                  <div className="rounded-lg bg-slate-50 p-3">
                    <div className="text-xs text-slate-500">20일 하방변동성</div>
                    <div className="mt-1 text-lg font-bold text-slate-800">{formatOptionalPct(target.downside_volatility_20d)}</div>
                  </div>
                </div>
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-2xl border border-emerald-200 bg-emerald-50 p-5">
                <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-emerald-800">유효 근거</h2>
                <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-emerald-900">
                  {strengths.map((line) => (
                    <li key={line}>{line}</li>
                  ))}
                </ul>
              </div>
              <div className="rounded-2xl border border-rose-200 bg-rose-50 p-5">
                <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-rose-800">주의 포인트</h2>
                <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-rose-900">
                  {risks.map((line) => (
                    <li key={line}>{line}</li>
                  ))}
                </ul>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-slate-700">종목 리포트</h2>
              <p className="mt-2 text-sm leading-6 text-slate-700">{buildNarrative(target, signal as Signal, weight)}</p>
              {markdownLine ? (
                <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
                  <div className="mb-1 font-semibold text-slate-500">원문 마크다운 행</div>
                  <code className="break-all">{markdownLine}</code>
                </div>
              ) : null}
            </div>

            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <h2 className="text-sm font-extrabold uppercase tracking-[0.14em] text-slate-700">동일 시장 상위 비교</h2>
              <div className="mt-3 overflow-x-auto">
                <table className="min-w-full border-collapse text-sm">
                  <thead>
                    <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-[0.12em] text-slate-500">
                      <th className="px-2 py-2">순위</th>
                      <th className="px-2 py-2">종목</th>
                      <th className="px-2 py-2">총점</th>
                      <th className="px-2 py-2">20D</th>
                      <th className="px-2 py-2">60D</th>
                    </tr>
                  </thead>
                  <tbody>
                    {peers.map((item) => (
                      <tr
                        key={`${item.market}:${item.code}:${item.as_of}`}
                        className={item.code === target.code ? "bg-cyan-50" : "border-b border-slate-100"}
                      >
                        <td className="px-2 py-2 font-semibold text-slate-700">#{item.rank}</td>
                        <td className="px-2 py-2">
                          <div className="font-semibold text-slate-900">{item.name}</div>
                          <div className="text-xs text-slate-500">{item.code}</div>
                        </td>
                        <td className="px-2 py-2 font-semibold text-slate-800">{item.total_score.toFixed(2)}</td>
                        <td className={`px-2 py-2 font-semibold ${item.return_20d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                          {formatSignedPct(item.return_20d)}
                        </td>
                        <td className={`px-2 py-2 font-semibold ${item.return_60d >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                          {formatSignedPct(item.return_60d)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-slate-100 p-4 text-xs text-slate-600">
              본 상세 페이지는 `/api/quant/report`의 최신 응답에서 선택 종목을 찾아 렌더링합니다. 기준 유니버스는 코스피/코스닥 시가총액 1조 이상입니다.
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}
