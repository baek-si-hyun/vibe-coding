"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  buildLSTMMeta,
  buildNarrative,
  buildQuantSignal,
  buildRisks,
  buildStrengths,
  clamp,
  extractStockLine,
  formatDateCompact,
  formatMarketCap,
  formatNumber,
  formatOptionalPct,
  formatOptionalSignedPct,
  formatPct,
  formatPrice,
  formatSignedPct,
  sanitizeMarket,
  type QuantSignal as Signal,
  type ReportResponse,
  weightFromScore,
} from "@/components/quant-report/shared";
import {
  EmptyNotice,
  MetricCard,
  PageHero,
  Panel,
  ReportShell,
  StatusPill,
  type Tone,
  cx,
} from "@/components/reports/ReportUI";

const MIN_MARKET_CAP = 1_000_000_000_000;
const DETAIL_LIMIT = 300;

const buttonClass =
  "inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50";
const primaryButtonClass =
  "inline-flex items-center justify-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700";

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
  const lstmMeta = useMemo(() => buildLSTMMeta(reportData), [reportData]);

  const target = useMemo(() => {
    if (!code) return null;
    const exact = items.find(
      (item) => item.code.toUpperCase() === code && (requestedMarket === "ALL" || item.market === requestedMarket)
    );
    if (exact) return exact;
    return items.find((item) => item.code.toUpperCase() === code) ?? null;
  }, [code, items, requestedMarket]);

  const signal = useMemo(() => (target ? buildQuantSignal(target) : null), [target]);
  const weight = useMemo(() => (target ? weightFromScore(target.total_score, target.rank) : 0), [target]);
  const strengths = useMemo(() => (target ? buildStrengths(target) : []), [target]);
  const risks = useMemo(() => (target ? buildRisks(target) : []), [target]);
  const markdownLine = useMemo(() => extractStockLine(reportData?.report_markdown || "", code), [code, reportData?.report_markdown]);
  const peers = useMemo(() => {
    if (!target) return [];
    return items.filter((item) => item.market === target.market).slice(0, 8);
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
            ...(Number.isFinite(target.lstm_score)
              ? [{ label: "LSTM", value: target.lstm_score ?? 0, colorClass: "bg-fuchsia-500" }]
              : []),
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

  const summaryMetrics: Array<{
    label: string;
    value: string;
    description: string;
    tone: Tone;
  }> = target
    ? [
        {
          label: "신호 / 비중",
          value: signal?.label ?? "-",
          description: `권장 비중 ${weight.toFixed(1)}%`,
          tone: signal?.tone ?? "slate",
        },
        {
          label: "순위 / 총점",
          value: `#${target.rank}`,
          description: `${target.total_score.toFixed(2)}점 · ${target.market}`,
          tone: "cyan" as const,
        },
        {
          label: "기준일 / 종가",
          value: formatDateCompact(target.as_of),
          description: formatPrice(target.close),
          tone: "slate" as const,
        },
        {
          label: "시가총액 / 턴오버",
          value: formatMarketCap(target.market_cap),
          description: `거래대금비율 ${formatPct(target.turnover_ratio)}`,
          tone: "slate" as const,
        },
        {
          label: "LSTM 내일 전망",
          value: formatOptionalSignedPct(target.lstm_pred_return_1d ?? target.lstm_pred_return_5d),
          description: `점수 ${Number.isFinite(target.lstm_score) ? (target.lstm_score ?? 0).toFixed(2) : "-"} · 상승확률 ${formatOptionalPct(target.lstm_prob_up)}`,
          tone: Number.isFinite(target.lstm_score)
            ? ((target.lstm_pred_return_1d ?? target.lstm_pred_return_5d ?? 0) >= 0 ? "violet" : "amber")
            : "slate",
        },
        {
          label: "20일 변동성",
          value: formatPct(target.volatility_20d),
          description: `60일 변동성 ${formatOptionalPct(target.volatility_60d)}`,
          tone: target.volatility_20d >= 4.5 ? "amber" : "emerald",
        },
      ]
    : [];

  return (
    <ReportShell>
      <PageHero
        eyebrow="Quant Detail"
        title={target?.name || requestedName || code || "종목 상세"}
        description="퀀트 추천 리스트에서 선택한 종목의 점수 분해, 수익률, LSTM 신호, 비교 종목 정보를 같은 카드 레이아웃으로 정리했습니다."
        badges={
          <>
            {code ? <StatusPill tone="slate">{code}</StatusPill> : null}
            <StatusPill tone="cyan">{requestedMarket}</StatusPill>
            <StatusPill tone="slate">{reportData?.score_model || "multifactor"}</StatusPill>
            <StatusPill tone={lstmMeta.enabled ? "violet" : "slate"}>
              {lstmMeta.enabled
                ? `LSTM ${lstmMeta.modelVersion || "active"} · ${lstmMeta.weightPct.toFixed(1)}%`
                : "LSTM 미적용"}
            </StatusPill>
          </>
        }
        actions={
          <>
            <button type="button" onClick={backToList} className={buttonClass}>
              추천 리스트로 돌아가기
            </button>
            <button type="button" onClick={() => void fetchDetail()} className={primaryButtonClass}>
              리포트 새로고침
            </button>
          </>
        }
      />

      {loading && !target ? <EmptyNotice tone="cyan" message="상세 리포트를 불러오는 중입니다." /> : null}
      {error ? <EmptyNotice tone="rose" message={error} /> : null}
      {!loading && !error && !target ? (
        <EmptyNotice tone="amber" message={`현재 리포트 범위에서 ${code || "해당 종목"}을 찾지 못했습니다. (시장: ${requestedMarket})`} />
      ) : null}

      {target ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {summaryMetrics.map((metric) => (
              <MetricCard
                key={metric.label}
                label={metric.label}
                value={metric.value}
                description={metric.description}
                tone={metric.tone}
              />
            ))}
          </div>

          <div className="grid gap-4 xl:grid-cols-[1.2fr,1fr]">
            <Panel
              title="투자 요약"
              description="현재 종목을 한 문장으로 읽을 수 있게 정리한 카드입니다."
              badge={<StatusPill tone={signal?.tone ?? "slate"}>{signal?.label ?? "-"}</StatusPill>}
            >
              <p className="text-sm leading-6 text-slate-700">{buildNarrative(target, signal as Signal, weight)}</p>
              {markdownLine ? (
                <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">원문 마크다운 행</div>
                  <code className="mt-2 block break-all text-xs text-slate-700">{markdownLine}</code>
                </div>
              ) : null}
            </Panel>

            <Panel
              title="점수 분해"
              description="각 점수가 총점에 어떤 비중으로 기여하는지 막대로 보여줍니다."
              badge={<StatusPill tone="slate">{reportData?.generated_at ? "LIVE" : "대기"}</StatusPill>}
            >
              <div className="space-y-3">
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
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <MetricCard
                  label="LSTM 모델"
                  value={lstmMeta.enabled ? lstmMeta.modelVersion || "active" : "-"}
                  description={
                    lstmMeta.enabled
                      ? `가중치 ${lstmMeta.weightPct.toFixed(1)}% · 기준일 ${formatDateCompact(lstmMeta.predictionAsOf)}`
                      : "보조모델 미적용"
                  }
                  tone={lstmMeta.enabled ? "violet" : "slate"}
                />
                <MetricCard
                  label="LSTM 신뢰도"
                  value={formatOptionalPct(target.lstm_confidence)}
                  description={lstmMeta.enabled ? `유니버스 적용 ${formatNumber(lstmMeta.appliedCount)}개` : "LSTM 데이터 없음"}
                  tone={lstmMeta.enabled ? "violet" : "slate"}
                />
              </div>
            </Panel>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Panel title="수익률과 리스크" description="단기와 중기 흐름, 변동성, 낙폭을 함께 확인합니다.">
              <div className="grid gap-3 sm:grid-cols-2">
                <MetricCard label="1일" value={formatSignedPct(target.return_1d)} description="단기 반응" tone={target.return_1d >= 0 ? "emerald" : "rose"} />
                <MetricCard label="5일" value={formatOptionalSignedPct(target.return_5d)} description="단기 구간" tone={(target.return_5d ?? 0) >= 0 ? "cyan" : "amber"} />
                <MetricCard label="20일" value={formatSignedPct(target.return_20d)} description="중기 추세" tone={target.return_20d >= 0 ? "emerald" : "rose"} />
                <MetricCard label="60일" value={formatSignedPct(target.return_60d)} description="중장기 추세" tone={target.return_60d >= 0 ? "cyan" : "amber"} />
                <MetricCard label="120일" value={formatSignedPct(target.return_120d)} description="장기 흐름" tone={target.return_120d >= 0 ? "emerald" : "rose"} />
                <MetricCard label="60일 최대낙폭" value={formatOptionalSignedPct(target.drawdown_60d)} description="낙폭 관리" tone={(target.drawdown_60d ?? 0) >= -12 ? "emerald" : "rose"} />
              </div>
            </Panel>

            <Panel title="보조 지표" description="거래대금, 하방 변동성, LSTM 예측 정보를 묶었습니다.">
              <div className="grid gap-3 sm:grid-cols-2">
                <MetricCard label="20일 평균 거래대금비율" value={formatOptionalPct(target.avg_turnover_ratio_20d)} description="유동성 점검" tone="slate" />
                <MetricCard label="20일 하방변동성" value={formatOptionalPct(target.downside_volatility_20d)} description="하락 스트레스" tone="amber" />
                <MetricCard label="LSTM 5일 전망" value={formatOptionalSignedPct(target.lstm_pred_return_5d)} description="보조 시계열" tone="violet" />
                <MetricCard label="LSTM 20일 전망" value={formatOptionalSignedPct(target.lstm_pred_return_20d)} description="중기 예측" tone="violet" />
              </div>
            </Panel>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Panel title="유효 근거" description="현재 종목을 긍정적으로 볼 수 있는 요인입니다.">
              <div className="space-y-3">
                {strengths.map((line) => (
                  <div key={line} className="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700">
                    {line}
                  </div>
                ))}
              </div>
            </Panel>

            <Panel title="주의 포인트" description="실제 매매 전에 다시 체크해야 하는 리스크입니다.">
              <div className="space-y-3">
                {risks.map((line) => (
                  <div key={line} className="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700">
                    {line}
                  </div>
                ))}
              </div>
            </Panel>
          </div>

          <Panel title="동일 시장 비교" description="같은 시장 상위 종목과 현재 종목을 바로 비교합니다.">
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-[0.14em] text-slate-500">
                    <th className="px-3 py-3">순위</th>
                    <th className="px-3 py-3">종목</th>
                    <th className="px-3 py-3">총점</th>
                    <th className="px-3 py-3">LSTM</th>
                    <th className="px-3 py-3">20D</th>
                    <th className="px-3 py-3">60D</th>
                  </tr>
                </thead>
                <tbody>
                  {peers.map((item) => (
                    <tr
                      key={`${item.market}:${item.code}:${item.as_of}`}
                      className={cx(
                        "border-b border-slate-100 last:border-b-0",
                        item.code === target.code ? "bg-cyan-50" : ""
                      )}
                    >
                      <td className="px-3 py-3 font-semibold text-slate-700">#{item.rank}</td>
                      <td className="px-3 py-3">
                        <div className="font-semibold text-slate-900">{item.name}</div>
                        <div className="mt-1 text-xs text-slate-500">{item.code}</div>
                      </td>
                      <td className="px-3 py-3 font-semibold text-slate-800">{item.total_score.toFixed(2)}</td>
                      <td className={cx("px-3 py-3 font-semibold", (item.lstm_score ?? 0) >= 60 ? "text-fuchsia-700" : "text-slate-500")}>
                        {Number.isFinite(item.lstm_score) ? (item.lstm_score ?? 0).toFixed(2) : "-"}
                      </td>
                      <td className={cx("px-3 py-3 font-semibold", item.return_20d >= 0 ? "text-emerald-600" : "text-rose-600")}>
                        {formatSignedPct(item.return_20d)}
                      </td>
                      <td className={cx("px-3 py-3 font-semibold", item.return_60d >= 0 ? "text-emerald-600" : "text-rose-600")}>
                        {formatSignedPct(item.return_60d)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Panel>
        </>
      ) : null}
    </ReportShell>
  );
}
