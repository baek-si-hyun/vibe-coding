"use client";

import { useCallback, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  buildLSTMMeta,
  buildQuantSignal,
  clamp,
  crisisLevel,
  deriveQuantRows,
  formatChangeText,
  formatClock,
  formatDateTime,
  formatMarketCap,
  formatNumber,
  formatOptionalSignedPct,
  formatPct,
  formatSignedPct,
  parseTopMarkdown,
  toneFromAlert,
  toneFromMacroMetric,
  toneFromRisk,
  toneFromScenario,
  toneFromVerdict,
  type AlertTone,
  type MacroResponse,
  type MarketFilter,
  type QuantItem,
  type RankResponse,
  type ReportResponse,
  type TacticalVerdict,
  weightFromScore,
} from "@/components/quant-report/shared";
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
import { useQuantPageStore } from "@/stores/quantPageStore";

const MIN_MARKET_CAP = 1_000_000_000_000;
const MAX_RECOMMENDATIONS = 3;
const FETCH_RETRY_COOLDOWN_SECONDS = 60;

const buttonClass =
  "inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50";
const primaryButtonClass =
  "inline-flex items-center justify-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700";
const inputClass =
  "w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100";

export default function QuantPage() {
  const router = useRouter();
  const state = useQuantPageStore((store) => store.state);
  const updateState = useQuantPageStore((store) => store.updateState);
  const {
    market,
    limit,
    loading,
    error,
    rankData,
    reportData,
    macroData,
    lastUpdatedKST,
    dataPulse,
    lastAttemptedAt,
    dataKey,
  } = state;
  const minMarketCap = MIN_MARKET_CAP;
  const requestKey = `${market}:${limit}:${minMarketCap}`;

  const fetchQuant = useCallback(async () => {
    updateState({
      loading: true,
      error: "",
    });

    try {
      const params = new URLSearchParams({
        market,
        limit: String(limit),
        min_market_cap: String(minMarketCap),
      });

      const [rankRes, reportRes, macroRes] = await Promise.all([
        fetch(`/api/quant/rank?${params.toString()}`, { method: "GET", cache: "no-store" }),
        fetch(`/api/quant/report?${params.toString()}`, { method: "GET", cache: "no-store" }),
        fetch("/api/quant/macro", { method: "GET", cache: "no-store" }),
      ]);

      const rankJson = await rankRes.json().catch(() => ({}));
      if (!rankRes.ok) throw new Error(rankJson.error || "퀀트 랭킹 조회 실패");

      const reportJson = await reportRes.json().catch(() => ({}));
      if (!reportRes.ok) throw new Error(reportJson.error || "퀀트 리포트 조회 실패");

      const macroJson = await macroRes.json().catch(() => ({}));

      if (macroRes.ok) {
        updateState({
          rankData: rankJson as RankResponse,
          reportData: reportJson as ReportResponse,
          macroData: macroJson as MacroResponse,
          loading: false,
          error: "",
          dataPulse: true,
          lastUpdatedKST: formatClock(new Date()),
          lastAttemptedAt: Date.now(),
          lastFetchedAt: Date.now(),
          dataKey: requestKey,
        });
      } else {
        updateState({
          rankData: rankJson as RankResponse,
          reportData: reportJson as ReportResponse,
          macroData: {
            generated_at: new Date().toISOString(),
            metrics: {},
            geopolitical: {
              score: 0,
              level: "안정",
              note: typeof macroJson?.error === "string" ? macroJson.error : "매크로 데이터를 불러오지 못했습니다.",
            },
            warnings: [typeof macroJson?.error === "string" ? macroJson.error : "매크로 데이터를 불러오지 못했습니다."],
          },
          loading: false,
          error: "",
          dataPulse: true,
          lastUpdatedKST: formatClock(new Date()),
          lastAttemptedAt: Date.now(),
          lastFetchedAt: Date.now(),
          dataKey: requestKey,
        });
      }
    } catch (fetchError) {
      updateState({
        loading: false,
        error: fetchError instanceof Error ? fetchError.message : "퀀트 데이터 조회 중 오류가 발생했습니다.",
        lastAttemptedAt: Date.now(),
        dataKey: requestKey,
      });
    }
  }, [limit, market, minMarketCap, requestKey, updateState]);

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

  useEffect(() => {
    if (!dataPulse) return;
    const timeout = window.setTimeout(() => updateState({ dataPulse: false }), 500);
    return () => window.clearTimeout(timeout);
  }, [dataPulse, updateState]);

  const rows = useMemo(() => rankData?.items ?? [], [rankData?.items]);

  const lstmMeta = useMemo(() => buildLSTMMeta(rankData), [rankData]);

  const derived = useMemo(() => deriveQuantRows(rows), [rows]);

  const macroMetrics = useMemo(() => macroData?.metrics ?? {}, [macroData?.metrics]);
  const geopolitical = useMemo(() => macroData?.geopolitical ?? null, [macroData?.geopolitical]);
  const headlineRiskScore = useMemo(() => Math.max(derived.riskScore, geopolitical?.score ?? 0), [derived.riskScore, geopolitical?.score]);
  const headlineRiskLevel = useMemo(() => crisisLevel(headlineRiskScore), [headlineRiskScore]);
  const topFromMarkdown = useMemo(
    () => parseTopMarkdown(reportData?.report_markdown || "", MAX_RECOMMENDATIONS),
    [reportData?.report_markdown]
  );

  const macroStatusRows = useMemo(
    () => [
      { key: "wti", label: "WTI 원유", metric: macroMetrics.wti },
      { key: "vix", label: "VIX 공포지수", metric: macroMetrics.vix },
      { key: "usdkrw", label: "USD/KRW 환율", metric: macroMetrics.usdkrw },
      { key: "gold", label: "XAU/USD", metric: macroMetrics.gold },
      { key: "us10y", label: "미국채 10Y 금리", metric: macroMetrics.us10y },
    ],
    [macroMetrics]
  );

  const alerts = useMemo(() => {
    const items: Array<{ title: string; detail: string; tone: AlertTone }> = [];

    const geoHeadlines = geopolitical?.headlines ?? [];
    for (const headline of geoHeadlines.slice(0, 2)) {
      items.push({
        title: headline.title,
        detail: `${headline.source || "외부뉴스"}${headline.keywords?.length ? ` · ${headline.keywords.join(", ")}` : ""}`,
        tone: (geopolitical?.score ?? 0) >= 60 ? "danger" : "info",
      });
    }

    if (rows.length === 0) return items;

    items.push({
      title: derived.avg20d < -6 ? "중기 모멘텀 급랭" : derived.avg20d > 6 ? "중기 모멘텀 우위" : "중기 모멘텀 혼조",
      detail: `추천 ${rows.length}개 평균 20일 수익률 ${formatSignedPct(derived.avg20d)}`,
      tone: derived.avg20d < -6 ? "danger" : derived.avg20d > 6 ? "safe" : "warn",
    });

    items.push({
      title: derived.avgVol20 > 4.5 ? "변동성 경계 구간" : "변동성 안정 구간",
      detail: `평균 20일 변동성 ${formatPct(derived.avgVol20)}`,
      tone: derived.avgVol20 > 4.5 ? "danger" : "safe",
    });

    if (lstmMeta.enabled && derived.visibleLSTMAppliedCount > 0) {
      items.push({
        title: derived.avgLSTMPred1D >= 0 ? "LSTM 내일 기대수익 양호" : "LSTM 내일 기대수익 둔화",
        detail: `적용 ${derived.visibleLSTMAppliedCount}개 · 평균 1일 예측 ${formatSignedPct(derived.avgLSTMPred1D)} · 신뢰도 ${formatPct(derived.avgLSTMConfidence)}`,
        tone: derived.avgLSTMPred1D >= 0 ? "info" : "warn",
      });
    }

    items.push({
      title: derived.positive20Ratio < 0.4 ? "상승 확산도 약함" : "상승 확산도 유지",
      detail: `20일 플러스 비율 ${(derived.positive20Ratio * 100).toFixed(1)}%`,
      tone: derived.positive20Ratio < 0.4 ? "danger" : "info",
    });

    if (derived.topWinner) {
      items.push({
        title: `상대강도 1위: ${derived.topWinner.name}`,
        detail: `${derived.topWinner.code} · 1일 ${formatSignedPct(derived.topWinner.return_1d)} · 총점 ${derived.topWinner.total_score.toFixed(2)}`,
        tone: "safe",
      });
    }

    if (derived.topLoser) {
      items.push({
        title: `상대약세 1위: ${derived.topLoser.name}`,
        detail: `${derived.topLoser.code} · 1일 ${formatSignedPct(derived.topLoser.return_1d)} · 총점 ${derived.topLoser.total_score.toFixed(2)}`,
        tone: "warn",
      });
    }

    return items.slice(0, 6);
  }, [derived, geopolitical, lstmMeta.enabled, rows]);

  const tacticalVerdict = useMemo<TacticalVerdict>(() => {
    if (rows.length === 0) {
      return {
        title: "판단 보류",
        summary: "데이터가 부족해 방어적 관망이 우선입니다.",
        tone: "amber",
        pros: ["현재 랭킹 데이터가 없어 추세 우위 판단 불가"],
        cons: ["근거 없는 진입은 손실 확률이 높습니다."],
        conditions: ["랭킹 데이터 갱신 후 다시 판단"],
      };
    }

    const pressure = derived.avg20d < -5 || derived.avg60d < -10;
    const highStress = derived.riskScore >= 70 || derived.avgVol20 > 4.8;
    const rebound = derived.avg1d > 0.4 || derived.positive20Ratio > 0.58;

    if (highStress && pressure && !rebound) {
      return {
        title: "방어 포지션 유효",
        summary: "중기 하락 압력과 변동성 확대가 겹쳐 공격적 진입보다 방어가 우선입니다.",
        tone: "red",
        pros: [
          `위기지수 ${derived.riskScore}/100`,
          `평균 20D ${formatSignedPct(derived.avg20d)}`,
          `평균 변동성 ${formatPct(derived.avgVol20)}`,
        ],
        cons: [
          `기술적 반등 시나리오 ${(derived.scenarios[0]?.probability ?? 0)}%`,
          "급락 직후 숏 성격 대응은 오히려 손실이 커질 수 있습니다.",
        ],
        conditions: [
          "총점 상위라도 추세 동시 회복 전까지 비중 확대 보류",
          "상승 확산도 40% 미만 유지 시에만 추가 방어",
        ],
      };
    }

    if (rebound && !highStress) {
      return {
        title: "완만한 리스크온",
        summary: "단기 반등 신호가 있어 분할 접근 중심의 선별 매수가 가능한 구간입니다.",
        tone: "green",
        pros: [
          `평균 1D ${formatSignedPct(derived.avg1d)}`,
          `상승 확산도 ${(derived.positive20Ratio * 100).toFixed(1)}%`,
        ],
        cons: [
          `중기 모멘텀 20D ${formatSignedPct(derived.avg20d)}로 완전 회복은 아님`,
          "변동성 구간에서는 손절 규칙 미준수 시 손실 확대 가능",
        ],
        conditions: [
          "총점 70 이상 + 20D/60D 동시 플러스 종목 우선",
          "개별 종목 손절 기준은 -5%~-7% 고정",
        ],
      };
    }

    return {
      title: "조건부 관망",
      summary: "추세와 반등 신호가 혼재되어 상위 랭킹 종목만 제한적으로 봐야 합니다.",
      tone: "amber",
      pros: [
        `박스권 시나리오 ${(derived.scenarios[1]?.probability ?? 0)}%`,
        `총점 1위 ${derived.topScore?.name ?? "-"} 중심의 국소 강세`,
      ],
      cons: [
        `리스크지수 ${derived.riskScore}/100`,
        `평균 변동성 ${formatPct(derived.avgVol20)}`,
      ],
      conditions: [
        "상승 확산도 50% 이상 회복 시 비중 확대",
        "위기지수 55 이하 하락 시 공격 비중 단계적 확대",
      ],
    };
  }, [derived, rows.length]);

  const crisisNarrative = useMemo(() => {
    if (rows.length === 0) return "현재 랭킹 데이터가 비어 있어 종합 내러티브를 만들 수 없습니다.";

    const macroNarrative = [
      macroMetrics.wti?.status === "ok" ? `WTI ${macroMetrics.wti.display}` : "",
      macroMetrics.vix?.status === "ok" ? `VIX ${macroMetrics.vix.display}` : "",
      macroMetrics.usdkrw?.status === "ok" ? `USD/KRW ${macroMetrics.usdkrw.display}` : "",
      geopolitical ? `지정학 ${geopolitical.score}/100` : "",
    ]
      .filter(Boolean)
      .join(" · ");

    const quantNarrative = `추천 ${rows.length}개 평균 20D ${formatSignedPct(derived.avg20d)}, 평균 변동성 ${formatPct(
      derived.avgVol20
    )}, 상승 확산도 ${(derived.positive20Ratio * 100).toFixed(1)}%입니다.`;

    const lstmNarrative =
      lstmMeta.enabled && derived.visibleLSTMAppliedCount > 0
        ? `LSTM 평균 1D ${formatSignedPct(derived.avgLSTMPred1D)} · 적용 ${derived.visibleLSTMAppliedCount}개`
        : "";

    return [macroNarrative, quantNarrative, lstmNarrative].filter(Boolean).join(" | ");
  }, [derived, geopolitical, lstmMeta.enabled, macroMetrics.usdkrw, macroMetrics.vix, macroMetrics.wti, rows.length]);

  const moveToDetail = useCallback(
    (item: QuantItem) => {
      const params = new URLSearchParams({
        code: item.code,
        market: item.market,
        name: item.name,
        limit: String(limit),
        min_market_cap: String(minMarketCap),
      });
      router.push(`/quant/detail?${params.toString()}`);
    },
    [limit, minMarketCap, router]
  );

  const summaryMetrics = [
    {
      label: "종합 리스크",
      value: `${headlineRiskScore}`,
      description: `${headlineRiskLevel} · 수급/변동성/지정학 통합`,
      tone: toneFromRisk(headlineRiskLevel),
    },
    {
      label: "평균 20D",
      value: formatSignedPct(derived.avg20d),
      description: `추천 ${rows.length}개 기준`,
      tone: derived.avg20d >= 0 ? "emerald" : "rose",
    },
    {
      label: "평균 60D",
      value: formatSignedPct(derived.avg60d),
      description: "중기 추세 체력",
      tone: derived.avg60d >= 0 ? "cyan" : "amber",
    },
    {
      label: "상승 확산도",
      value: `${(derived.positive20Ratio * 100).toFixed(1)}%`,
      description: "20일 수익률이 플러스인 종목 비율",
      tone: derived.positive20Ratio >= 0.5 ? "emerald" : "amber",
    },
    {
      label: "평균 변동성",
      value: formatPct(derived.avgVol20),
      description: "20일 기준 표준편차",
      tone: derived.avgVol20 > 4.5 ? "amber" : "slate",
    },
    {
      label: "LSTM 평균 1D",
      value: lstmMeta.enabled ? formatSignedPct(derived.avgLSTMPred1D) : "미적용",
      description: lstmMeta.enabled
        ? `적용 ${derived.visibleLSTMAppliedCount}개 · 신뢰도 ${formatPct(derived.avgLSTMConfidence)}`
        : "보조모델 데이터 없음",
      tone: !lstmMeta.enabled ? "slate" : derived.avgLSTMPred1D >= 0 ? "violet" : "amber",
    },
  ] as const;

  return (
    <ReportShell>
      <PageHero
        eyebrow="Quant Report"
        title="주식 퀀트 리포트"
        description="기존 저장 일봉 데이터와 매크로 신호를 함께 요약해 추천 종목과 현재 시장 컨디션을 한 화면에서 보도록 정리했습니다."
        badges={
          <>
            <StatusPill tone="cyan">{market}</StatusPill>
            <StatusPill>{reportData?.score_model || rankData?.score_model || "multifactor"}</StatusPill>
            <StatusPill tone={lstmMeta.enabled ? "violet" : "slate"}>
              {lstmMeta.enabled
                ? `LSTM ${lstmMeta.modelVersion || "active"} · ${lstmMeta.weightPct.toFixed(1)}%`
                : "LSTM 미적용"}
            </StatusPill>
            <StatusPill tone={toneFromRisk(headlineRiskLevel)}>리스크 {headlineRiskLevel}</StatusPill>
            <StatusPill tone="slate">배치 00:00 · 07:50</StatusPill>
          </>
        }
        actions={
          <button type="button" onClick={() => void fetchQuant()} className={primaryButtonClass}>
            리포트 새로고침
          </button>
        }
      />

      {error ? <EmptyNotice tone="rose" message={error} /> : null}

      <div className="grid gap-4 xl:grid-cols-[1.5fr,1fr]">
        <Panel
          title="조회 설정"
          description="시장과 표시 개수를 관리합니다. 자동 실행은 자정과 장전 10분 전 배치로만 처리합니다."
          badge={<KSTClockPill />}
        >
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <label className="space-y-2 text-sm text-slate-600">
              <span className="block text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">시장</span>
              <select
                className={inputClass}
                value={market}
                onChange={(event) => updateState({ market: event.target.value as MarketFilter })}
              >
                <option value="ALL">ALL</option>
                <option value="KOSPI">KOSPI</option>
                <option value="KOSDAQ">KOSDAQ</option>
              </select>
            </label>
            <label className="space-y-2 text-sm text-slate-600">
              <span className="block text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">추천 수</span>
              <select
                className={inputClass}
                value={limit}
                onChange={(event) =>
                  updateState({
                    limit: clamp(Number.parseInt(event.target.value || "3", 10), 1, MAX_RECOMMENDATIONS),
                  })
                }
              >
                <option value={1}>1</option>
                <option value={2}>2</option>
                <option value={3}>3</option>
              </select>
            </label>
            <div className="space-y-2 text-sm text-slate-600">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">최소 시총</div>
              <div className={cx(inputClass, "flex items-center bg-slate-50 text-slate-500")}>
                {formatMarketCap(MIN_MARKET_CAP)}
              </div>
            </div>
            <div className="space-y-2 text-sm text-slate-600">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">마지막 갱신</div>
              <div className={cx(inputClass, "flex items-center bg-slate-50 text-slate-500")}>{lastUpdatedKST}</div>
            </div>
          </div>

          <div className="mt-4 grid gap-3 lg:grid-cols-[1fr,auto] lg:items-center">
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">배치 실행 시각</div>
              <div className="mt-1 text-sm font-semibold text-slate-900">00:00 · 07:50 KST</div>
              <div className="mt-1 text-xs text-slate-500">
                마지막 리포트 {formatDateTime(rankData?.generated_at || "")} · 요청 시 자동 수집은 비활성화되어 있습니다.
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <button type="button" className={primaryButtonClass} onClick={() => void fetchQuant()}>
                지금 갱신
              </button>
            </div>
          </div>
        </Panel>

        <Panel
          title="시장 코멘트"
          description="현재 추천 유니버스와 매크로 신호를 합쳐 한 줄로 정리한 요약입니다."
          badge={<StatusPill tone={toneFromVerdict(tacticalVerdict.tone)}>{tacticalVerdict.title}</StatusPill>}
        >
          <p className="text-sm leading-6 text-slate-700">{crisisNarrative}</p>
          <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
            <div className="text-sm font-semibold text-slate-900">{tacticalVerdict.summary}</div>
            <div className="mt-3 flex flex-wrap gap-2">
              <StatusPill tone={toneFromRisk(derived.riskLevel)}>위기지수 {derived.riskScore}</StatusPill>
              <StatusPill tone="cyan">
                유니버스 {formatNumber(rankData?.universe_count ?? 0)}개
              </StatusPill>
              <StatusPill tone="slate">
                KOSPI {formatNumber(derived.kospiCount)} · KOSDAQ {formatNumber(derived.kosdaqCount)}
              </StatusPill>
            </div>
          </div>
        </Panel>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {summaryMetrics.map((metric) => (
          <MetricCard
            key={metric.label}
            label={metric.label}
            value={<span className={cx(dataPulse ? "text-slate-900" : "")}>{metric.value}</span>}
            description={metric.description}
            tone={metric.tone}
          />
        ))}
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.7fr,1fr]">
        <div className="space-y-4">
          <Panel
            title="추천 종목"
            description="총점과 추세를 함께 보고 바로 상세 리포트로 이동할 수 있게 정리했습니다."
            badge={<StatusPill tone="slate">최대 {MAX_RECOMMENDATIONS}개</StatusPill>}
          >
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-[0.14em] text-slate-500">
                    <th className="px-3 py-3">종목</th>
                    <th className="px-3 py-3">신호</th>
                    <th className="px-3 py-3">총점</th>
                    <th className="px-3 py-3">LSTM</th>
                    <th className="px-3 py-3">20D</th>
                    <th className="px-3 py-3">60D</th>
                    <th className="px-3 py-3">시총</th>
                    <th className="px-3 py-3">상세</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(0, MAX_RECOMMENDATIONS).map((row) => {
                    const signal = buildQuantSignal(row);
                    const weight = weightFromScore(row.total_score, row.rank);
                    return (
                      <tr key={`${row.market}:${row.code}:${row.as_of}`} className="border-b border-slate-100 last:border-b-0">
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">{row.name}</div>
                          <div className="mt-1 text-xs text-slate-500">
                            {row.code} · {row.market} · 비중 {weight.toFixed(1)}%
                          </div>
                        </td>
                        <td className="px-3 py-3">
                          <StatusPill tone={signal.tone}>{signal.label}</StatusPill>
                        </td>
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">{row.total_score.toFixed(2)}</div>
                          <div className="mt-1 text-xs text-slate-500">Mom {row.momentum_score.toFixed(1)}</div>
                        </td>
                        <td className="px-3 py-3">
                          <div className="font-semibold text-slate-900">
                            {Number.isFinite(row.lstm_score) ? (row.lstm_score ?? 0).toFixed(2) : "-"}
                          </div>
                          <div className="mt-1 text-xs text-slate-500">
                            1D {formatOptionalSignedPct(row.lstm_pred_return_1d ?? row.lstm_pred_return_5d)}
                          </div>
                        </td>
                        <td className={cx("px-3 py-3 font-semibold", row.return_20d >= 0 ? "text-emerald-600" : "text-rose-600")}>
                          {formatSignedPct(row.return_20d)}
                        </td>
                        <td className={cx("px-3 py-3 font-semibold", row.return_60d >= 0 ? "text-emerald-600" : "text-rose-600")}>
                          {formatSignedPct(row.return_60d)}
                        </td>
                        <td className="px-3 py-3 text-slate-700">{formatMarketCap(row.market_cap)}</td>
                        <td className="px-3 py-3">
                          <button type="button" className={buttonClass} onClick={() => moveToDetail(row)}>
                            상세보기
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            {!rows.length ? <EmptyNotice message="표시할 추천 데이터가 없습니다." /> : null}
          </Panel>

          <Panel
            title="자동 리포트 요약"
            description="백엔드에서 생성한 마크다운 추천 항목을 카드형 목록으로 다시 정리했습니다."
            badge={<StatusPill tone="slate">Markdown</StatusPill>}
          >
            <div className="space-y-3">
              {topFromMarkdown.length > 0 ? (
                topFromMarkdown.map((line, index) => (
                  <div key={`${line}-${index}`} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                    <div className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">추천 후보 {index + 1}</div>
                    <div className="mt-2 text-sm font-medium text-slate-800">{line}</div>
                  </div>
                ))
              ) : (
                <EmptyNotice message="리포트 요약 데이터가 없습니다." />
              )}
            </div>
          </Panel>
        </div>

        <div className="space-y-4">
          <Panel
            title="전술 판단"
            description="현재 장세에서 어떤 기준으로 접근해야 하는지 간단히 정리했습니다."
            badge={<StatusPill tone={toneFromVerdict(tacticalVerdict.tone)}>{tacticalVerdict.title}</StatusPill>}
          >
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-700">
              {tacticalVerdict.summary}
            </div>
            <div className="mt-4 grid gap-3">
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
                <div className="text-sm font-semibold text-emerald-700">유효 근거</div>
                <ul className="mt-2 space-y-2 text-sm text-gray-700">
                  {tacticalVerdict.pros.map((item) => (
                    <li key={item}>• {item}</li>
                  ))}
                </ul>
              </div>
              <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
                <div className="text-sm font-semibold text-rose-700">주의 포인트</div>
                <ul className="mt-2 space-y-2 text-sm text-gray-700">
                  {tacticalVerdict.cons.map((item) => (
                    <li key={item}>• {item}</li>
                  ))}
                </ul>
              </div>
              <div className="rounded-lg border border-gray-200 bg-white p-4">
                <div className="text-sm font-semibold text-slate-900">운영 조건</div>
                <ul className="mt-2 space-y-2 text-sm text-slate-600">
                  {tacticalVerdict.conditions.map((item) => (
                    <li key={item}>• {item}</li>
                  ))}
                </ul>
              </div>
            </div>
          </Panel>

          <Panel
            title="리스크 시나리오"
            description="현재 장세를 세 가지 경로로 단순화해 보여줍니다."
            badge={<StatusPill tone={toneFromRisk(headlineRiskLevel)}>{headlineRiskLevel}</StatusPill>}
          >
            <div className="space-y-3">
              {derived.scenarios.map((scenario) => (
                <div
                  key={scenario.title}
                  className={cx(
                    "rounded-lg border border-gray-200 bg-white p-4"
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-slate-900">{scenario.title}</div>
                      <p className="mt-1 text-sm leading-6 text-slate-700">{scenario.description}</p>
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
            description="상태 변화가 큰 항목만 짧은 문장으로 표시합니다."
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

      <div className="grid gap-4 lg:grid-cols-2">
        <Panel
          title="매크로 체크"
          description="외부 API로 읽은 매크로 신호를 같은 카드 문법으로 맞췄습니다."
          badge={<StatusPill tone="slate">{macroData?.generated_at ? "LIVE" : "대기"}</StatusPill>}
        >
          <div className="grid gap-3 sm:grid-cols-2">
            {macroStatusRows.map((row) => (
              <MetricCard
                key={row.key}
                label={row.label}
                value={row.metric?.display ?? "-"}
                description={
                  row.metric?.status === "ok"
                    ? `${formatChangeText(row.metric.change_percent)}${row.metric.provider ? ` · ${row.metric.provider}` : ""}`
                    : row.metric?.note || "연동 정보 없음"
                }
                tone={toneFromMacroMetric(row.key, row.metric)}
              />
            ))}
            <MetricCard
              label="지정학 이벤트"
              value={`${geopolitical?.score ?? 0}/100`}
              description={
                geopolitical?.providers?.length
                  ? `${geopolitical.providers.join(", ")} · ${geopolitical.level}`
                  : geopolitical?.note || "지정학 데이터 없음"
              }
              tone={toneFromRisk(geopolitical?.level ?? "주의")}
            />
          </div>
          {macroData?.warnings?.length ? (
            <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
              {macroData.warnings.join(" | ")}
            </div>
          ) : null}
        </Panel>

        <Panel
          title="운영 메모"
          description="현재 리포트를 실제로 활용할 때 확인해야 하는 기준만 남겼습니다."
          badge={<StatusPill tone="cyan">Action Plan</StatusPill>}
        >
          <div className="space-y-3 text-sm text-slate-700">
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              1. 리스크 레벨이 {headlineRiskLevel} 이상이면 신규 비중보다 기존 포지션 방어가 먼저입니다.
            </div>
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              2. 총점 70 이상이면서 20D/60D가 동시에 플러스인 종목만 우선 검토하는 편이 안정적입니다.
            </div>
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              3. 손절 기준은 점수와 분리해 -5%~-7% 수준으로 고정해야 변동성 장세에서 흔들리지 않습니다.
            </div>
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              4. 현재 데이터는 저장된 일봉과 매크로 API 기준입니다. 실시간 체결 판단은 별도 시세 소스가 추가돼야 합니다.
            </div>
          </div>
        </Panel>
      </div>

      {loading && !rows.length ? <EmptyNotice tone="cyan" message="퀀트 데이터를 불러오는 중입니다." /> : null}
    </ReportShell>
  );
}
