"use client";

import { useCallback, useEffect, useMemo } from "react";
import {
  buildCoinSignal,
  deriveCoinMarket,
  formatDateTime,
  formatFixed,
  formatNumber,
  formatPct,
  formatPrice,
  formatRatioX,
  formatSignedPct,
  formatTradeValue,
  safeNumber,
  toneFromAlert,
  toneFromRisk,
  toneFromScenario,
  type CoinAlertItem as AlertItem,
  type CoinQuantResponse as QuantResponse,
  weightFromScore,
} from "@/components/coin-quant/shared";
import {
  EmptyNotice,
  KSTClockPill,
  MetricCard,
  PageHero,
  Panel,
  ReportShell,
  StatusPill,
  cx,
} from "@/components/reports/ReportUI";
import {
  useCoinQuantStore,
  type CoinQuantScreenKey,
} from "@/stores/coinQuantStore";

type CoinMarketScreenerProps = {
  storeKey: CoinQuantScreenKey;
  exchangeName: string;
  apiBase: string;
  title: string;
  description: string;
  tradeHref: (symbol: string) => string;
  tradeLabel: string;
};

const FETCH_RETRY_COOLDOWN_SECONDS = 60;
const MAX_RECOMMENDATIONS = 3;

const buttonClass =
  "inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50";
const primaryButtonClass =
  "inline-flex items-center justify-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700";
const inputClass =
  "w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100";

export default function CoinMarketScreener({
  storeKey,
  exchangeName,
  apiBase,
  title,
  description,
  tradeHref,
  tradeLabel,
}: CoinMarketScreenerProps) {
  const screen = useCoinQuantStore((state) => state.screens[storeKey]);
  const updateScreen = useCoinQuantStore((state) => state.updateScreen);
  const {
    limit,
    minTradeValueInput,
    appliedMinTradeValue,
    data,
    loading,
    error,
    lastAttemptedAt,
    dataKey,
  } = screen;
  const requestKey = `${limit}:${appliedMinTradeValue}`;

  const fetchQuant = useCallback(async () => {
    updateScreen(storeKey, {
      loading: true,
      error: "",
    });
    try {
      const params = new URLSearchParams({
        limit: String(limit),
        min_trade_value_24h: String(appliedMinTradeValue),
      });
      const res = await fetch(`${apiBase}?${params.toString()}`, {
        method: "GET",
        cache: "no-store",
      });
      const payload = (await res.json().catch(() => ({}))) as QuantResponse & { error?: string };
      if (!res.ok) {
        throw new Error(payload.error || `API error ${res.status}`);
      }
      updateScreen(storeKey, {
        data: payload,
        loading: false,
        error: "",
        lastAttemptedAt: Date.now(),
        lastFetchedAt: Date.now(),
        dataKey: requestKey,
      });
    } catch (loadError) {
      updateScreen(storeKey, {
        loading: false,
        error: loadError instanceof Error ? loadError.message : "코인 퀀트 데이터를 불러오지 못했습니다.",
        lastAttemptedAt: Date.now(),
        dataKey: requestKey,
      });
    }
  }, [apiBase, appliedMinTradeValue, limit, requestKey, storeKey, updateScreen]);

  useEffect(() => {
    const hasRecentAttempt =
      dataKey === requestKey &&
      lastAttemptedAt !== null &&
      Date.now() - lastAttemptedAt < FETCH_RETRY_COOLDOWN_SECONDS * 1000;
    if (loading || hasRecentAttempt) {
      return;
    }
    void fetchQuant();
  }, [dataKey, fetchQuant, lastAttemptedAt, loading, requestKey]);

  const baseRows = useMemo(() => data?.items ?? [], [data?.items]);
  const rows = baseRows;

  const derived = useMemo(() => deriveCoinMarket(rows, data?.breadth_24h ?? 0), [data?.breadth_24h, rows]);

  const tacticalVerdict = useMemo(() => {
    if (derived.riskScore >= 65) {
      return {
        title: "공격 축소",
        tone: "rose" as const,
        summary: "변동성 대비 breadth가 약해서 추세 추격보다 방어가 우선입니다.",
        conditions: [
          "총점 70 이상 + 거래대금 ratio 1.2배 이상만 관찰",
          "24H 급등 코인 추격 비중 축소",
          "drawdown -15% 이하 코인 제외",
        ],
      };
    }

    if (derived.avg24h >= 0 && derived.breadth >= 55) {
      return {
        title: "상위 코인 중심 매매",
        tone: "emerald" as const,
        summary: "유동성과 breadth가 받쳐주는 구간이라 상단 랭킹의 신뢰도가 상대적으로 높습니다.",
        conditions: [
          "총점 68 이상 + 24H 플러스 코인 우선",
          "거래대금 증가율 상위 동시 확인",
          "1H 급등보다 24H 지속성 우선",
        ],
      };
    }

    return {
      title: "선별 접근",
      tone: "amber" as const,
      summary: "순환 장세에 가까워 상단 랭킹도 빠르게 교체될 수 있습니다.",
      conditions: [
        "총점 65 이상만 감시",
        "24H 수익률보다 거래대금 flow 우선",
        "breadth 재확대 여부 확인",
      ],
    };
  }, [derived]);

  const alerts = useMemo<AlertItem[]>(() => {
    const list: AlertItem[] = [];

    if (derived.breadth < 45) {
      list.push({
        title: "breadth 약화",
        detail: `24H breadth ${formatPct(derived.breadth)}로 확산이 좋지 않습니다.`,
        tone: "danger",
      });
    }

    if (derived.avgVol24h > 6.5) {
      list.push({
        title: "변동성 경계",
        detail: `평균 24H 변동성 ${formatPct(derived.avgVol24h)}로 흔들림이 큽니다.`,
        tone: "warn",
      });
    }

    if (derived.topFlow) {
      list.push({
        title: "거래대금 집중",
        detail: `${derived.topFlow.symbol} 거래대금 ratio ${formatRatioX(derived.topFlow.trade_value_ratio_24h)}로 자금이 몰립니다.`,
        tone: "info",
      });
    }

    if (derived.topWinner) {
      list.push({
        title: "단기 리더",
        detail: `${derived.topWinner.symbol} 24H ${formatSignedPct(derived.topWinner.return_24h)}로 상대 강도가 가장 높습니다.`,
        tone: "safe",
      });
    }

    list.push({
      title: "REST 스냅샷 기준",
      detail: `${exchangeName} REST 응답 기준으로 점수를 계산하며, 정기 배치는 00:00와 07:50에만 실행됩니다.`,
      tone: "info",
    });

    return list.slice(0, 4);
  }, [derived, exchangeName]);

  const applyMinTradeValue = () => {
    const parsed = Number(minTradeValueInput.replaceAll(",", "").trim());
    if (Number.isFinite(parsed) && parsed > 0) {
      updateScreen(storeKey, {
        appliedMinTradeValue: parsed,
      });
    }
  };

  const summaryMetrics = [
    {
      label: "24H Breadth",
      value: formatPct(derived.breadth),
      description: "플러스 수익률 코인 비율",
      tone: derived.breadth >= 55 ? "emerald" : derived.breadth >= 45 ? "amber" : "rose",
    },
    {
      label: "평균 24H",
      value: formatSignedPct(derived.avg24h),
      description: "상위 랭킹 평균 수익률",
      tone: derived.avg24h >= 0 ? "emerald" : "rose",
    },
    {
      label: "평균 1H",
      value: formatSignedPct(derived.avg1h),
      description: "단기 방향성",
      tone: derived.avg1h >= 0 ? "cyan" : "amber",
    },
    {
      label: "평균 변동성",
      value: formatPct(derived.avgVol24h),
      description: "24시간 표준편차",
      tone: derived.avgVol24h > 6.5 ? "amber" : "slate",
    },
    {
      label: "평균 거래대금",
      value: formatTradeValue(derived.avgTradeValue),
      description: "상위 랭킹 평균",
      tone: "slate",
    },
    {
      label: "리스크 지수",
      value: formatFixed(derived.riskScore, 0),
      description: `${derived.riskLevel} 구간`,
      tone: toneFromRisk(derived.riskLevel),
    },
  ] as const;

  return (
    <ReportShell>
      <PageHero
        eyebrow="Coin Quant"
        title={title}
        description={description}
        badges={
          <>
            <StatusPill tone="amber">{data?.interval ?? "1h"}</StatusPill>
            <StatusPill tone="slate">유니버스 {formatNumber(data?.universe_count ?? 0)}개</StatusPill>
            <StatusPill tone={toneFromRisk(derived.riskLevel)}>리스크 {derived.riskLevel}</StatusPill>
            <StatusPill tone="slate">REST snapshot</StatusPill>
            <StatusPill tone="slate">배치 00:00 · 07:50</StatusPill>
          </>
        }
        actions={
          <button type="button" onClick={() => void fetchQuant()} className={primaryButtonClass}>
            데이터 새로고침
          </button>
        }
      />

      {error ? <EmptyNotice tone="rose" message={error} /> : null}

      <div className="grid gap-4 xl:grid-cols-[1.4fr,1fr]">
        <Panel
          title="조회 설정"
          description="표시 개수와 최소 거래대금 조건을 조절합니다. 자동 호출은 자정과 장전 10분 전 배치로만 처리합니다."
          badge={<KSTClockPill />}
        >
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <label className="space-y-2 text-sm text-slate-600">
              <span className="block text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">추천 수</span>
              <select
                className={inputClass}
                value={limit}
                onChange={(event) => updateScreen(storeKey, { limit: Number(event.target.value) })}
              >
                <option value={1}>1</option>
                <option value={2}>2</option>
                <option value={3}>3</option>
              </select>
            </label>
            <label className="space-y-2 text-sm text-slate-600 xl:col-span-2">
              <span className="block text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">최소 24H 거래대금</span>
              <input
                className={inputClass}
                type="text"
                value={minTradeValueInput}
                onChange={(event) => updateScreen(storeKey, { minTradeValueInput: event.target.value })}
              />
            </label>
            <div className="space-y-2 text-sm text-slate-600">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">기준 인터벌</div>
              <div className={cx(inputClass, "flex items-center bg-slate-50 text-slate-500")}>{data?.interval ?? "1h"}</div>
            </div>
          </div>

          <div className="mt-4 grid gap-3 lg:grid-cols-[1fr,auto] lg:items-center">
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">배치 실행 시각</div>
              <div className="mt-1 text-sm font-semibold text-slate-900">00:00 · 07:50 KST</div>
              <div className="mt-1 text-xs text-slate-500">
                마지막 스냅샷 {formatDateTime(data?.generated_at)} · 화면은 REST 결과만 수동 갱신합니다.
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <button type="button" className={buttonClass} onClick={applyMinTradeValue}>
                조건 적용
              </button>
              <button type="button" className={primaryButtonClass} onClick={() => void fetchQuant()}>
                지금 갱신
              </button>
            </div>
          </div>
        </Panel>

        <Panel
          title="시장 코멘트"
          description="현재 breadth, 변동성, 거래대금 흐름을 한 문장으로 정리합니다."
          badge={<StatusPill tone={tacticalVerdict.tone}>{tacticalVerdict.title}</StatusPill>}
        >
          <p className="text-sm leading-6 text-slate-700">{tacticalVerdict.summary}</p>
          <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
            <div className="flex flex-wrap gap-2">
              <StatusPill tone="amber">breadth {formatPct(derived.breadth)}</StatusPill>
              <StatusPill tone={derived.avg24h >= 0 ? "emerald" : "rose"}>평균 24H {formatSignedPct(derived.avg24h)}</StatusPill>
              <StatusPill tone="slate">기준 {formatDateTime(data?.asOf)}</StatusPill>
              <StatusPill tone="slate">탑 {derived.topScore?.symbol ?? "-"}</StatusPill>
              <StatusPill tone="slate">Flow {derived.topFlow?.symbol ?? "-"}</StatusPill>
              <StatusPill tone="slate">{exchangeName}</StatusPill>
            </div>
          </div>
        </Panel>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
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

      <div className="grid gap-4 xl:grid-cols-[1.7fr,1fr]">
        <div className="space-y-4">
          <Panel
            title="추천 코인"
            description="총점, 거래대금, 1H/24H 흐름을 같이 보여주는 추천 테이블입니다."
            badge={<StatusPill tone="slate">최대 {MAX_RECOMMENDATIONS}개</StatusPill>}
          >
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-[0.14em] text-slate-500">
                    <th className="px-3 py-3">코인</th>
                    <th className="px-3 py-3">시그널</th>
                    <th className="px-3 py-3">총점</th>
                    <th className="px-3 py-3">1H</th>
                    <th className="px-3 py-3">24H</th>
                    <th className="px-3 py-3">거래대금</th>
                    <th className="px-3 py-3">Flow</th>
                    <th className="px-3 py-3">매매</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(0, MAX_RECOMMENDATIONS).map((row) => {
                    const rank = Number.isFinite(row.rank) && row.rank > 0 ? row.rank : 999;
                    const totalScore = safeNumber(row.total_score);
                    const signal = buildCoinSignal(row);
                    const weight = weightFromScore(totalScore, rank);
                    return (
                      <tr key={row.symbol} className="border-b border-slate-100 last:border-b-0">
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">{row.symbol}</div>
                          <div className="mt-1 text-xs text-slate-500">
                            {formatPrice(row.price)} · 비중 {weight.toFixed(1)}%
                          </div>
                          <div className="mt-1 text-xs text-slate-500">{formatDateTime(row.candleTime)}</div>
                        </td>
                        <td className="px-3 py-3">
                          <StatusPill tone={signal.tone}>{signal.label}</StatusPill>
                        </td>
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">{formatFixed(totalScore, 2)}</div>
                          <div className="mt-1 text-xs text-slate-500">
                            Mom {formatFixed(row.momentum_score, 1)} · Brk {formatFixed(row.breakout_score, 1)}
                          </div>
                        </td>
                        <td className={cx("px-3 py-3 font-semibold", row.return_1h >= 0 ? "text-emerald-600" : "text-rose-600")}>
                          {formatSignedPct(row.return_1h)}
                        </td>
                        <td className={cx("px-3 py-3 font-semibold", row.return_24h >= 0 ? "text-emerald-600" : "text-rose-600")}>
                          {formatSignedPct(row.return_24h)}
                        </td>
                        <td className="px-3 py-3 text-slate-700">{formatTradeValue(row.trade_value_24h)}</td>
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">{formatRatioX(row.trade_value_ratio_24h)}</div>
                          <div className="mt-1 text-xs text-slate-500">
                            Vol {formatPct(row.volatility_24h)} · DD {formatPct(row.drawdown_24h)}
                          </div>
                        </td>
                        <td className="px-3 py-3">
                          <a href={tradeHref(row.symbol)} target="_blank" rel="noreferrer" className={primaryButtonClass}>
                            {tradeLabel}
                          </a>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            {!rows.length ? <EmptyNotice message="표시할 코인 데이터가 없습니다." /> : null}
          </Panel>

          <Panel
            title="실행 기준"
            description="현재 장세에서 코인 퀀트 랭킹을 어떻게 써야 하는지 간단히 정리했습니다."
            badge={<StatusPill tone={tacticalVerdict.tone}>Action Plan</StatusPill>}
          >
            <div className="space-y-3">
              {tacticalVerdict.conditions.map((condition, index) => (
                <div key={condition} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">운영 기준 {index + 1}</div>
                  <div className="mt-2 text-sm text-slate-700">{condition}</div>
                </div>
              ))}
            </div>
          </Panel>
        </div>

        <div className="space-y-4">
          <Panel
            title="전술 판단"
            description="현재 breadth와 자금 흐름을 기준으로 바로 참고할 수 있는 요약입니다."
            badge={<StatusPill tone={tacticalVerdict.tone}>{tacticalVerdict.title}</StatusPill>}
          >
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-700">
              {tacticalVerdict.summary}
            </div>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <MetricCard label="평균 모멘텀" value={formatFixed(derived.avgMomentum, 1)} description="랭킹 상단 평균" tone="cyan" />
              <MetricCard label="평균 유동성" value={formatFixed(derived.avgLiquidity, 1)} description="거래대금 기반 점수" tone="amber" />
            </div>
          </Panel>

          <Panel
            title="시장 시나리오"
            description="코인 유니버스를 세 가지 경로로 단순화한 확률 분포입니다."
            badge={<StatusPill tone={toneFromRisk(derived.riskLevel)}>{derived.riskLevel}</StatusPill>}
          >
            <div className="space-y-3">
              {derived.scenarios.map((scenario) => (
                <div key={scenario.title} className="rounded-lg border border-gray-200 bg-white p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-slate-900">{scenario.title}</div>
                      <div className="mt-1 text-sm text-slate-700">{scenario.description}</div>
                    </div>
                    <StatusPill tone={toneFromScenario(scenario.tone)}>{scenario.probability}%</StatusPill>
                  </div>
                  <div className="mt-3 text-sm text-slate-600">{scenario.implication}</div>
                </div>
              ))}
            </div>
          </Panel>

          <Panel
            title="실시간 알람"
            description="변동성이 큰 구간에 참고할 만한 짧은 신호만 남겼습니다."
            badge={<StatusPill tone="slate">{alerts.length}건</StatusPill>}
          >
            <div className="space-y-3">
              {alerts.length > 0 ? (
                alerts.map((alert, index) => (
                  <div key={`${alert.title}-${index}`} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-semibold text-slate-900">{alert.title}</div>
                        <div className="mt-1 text-sm text-slate-600">{alert.detail}</div>
                      </div>
                      <StatusPill tone={toneFromAlert(alert.tone)}>{alert.tone}</StatusPill>
                    </div>
                  </div>
                ))
              ) : (
                <EmptyNotice message="알람 데이터가 없습니다." />
              )}
            </div>
          </Panel>
        </div>
      </div>

      {loading && !rows.length ? <EmptyNotice tone="amber" message="코인 퀀트 데이터를 불러오는 중입니다." /> : null}
    </ReportShell>
  );
}
