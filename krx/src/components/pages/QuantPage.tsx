"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

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
  return_20d: number;
  return_60d: number;
  return_120d: number;
  volatility_20d: number;
  momentum_score: number;
  liquidity_score: number;
  stability_score: number;
  total_score: number;
};

type RankResponse = {
  generated_at: string;
  market: string;
  min_market_cap: number;
  limit: number;
  universe_count: number;
  as_of_min: string;
  as_of_max: string;
  items: QuantItem[];
};

type ReportResponse = {
  report_markdown?: string;
};

type MacroMetric = {
  label: string;
  value?: number;
  display: string;
  change_percent?: number;
  as_of?: string;
  provider?: string;
  status: string;
  note?: string;
};

type MacroHeadline = {
  title: string;
  url?: string;
  source?: string;
  published_at?: string;
  keywords?: string[];
};

type MacroGeopolitical = {
  score: number;
  level: CrisisLevel;
  matched_keywords?: string[];
  headlines?: MacroHeadline[];
  providers?: string[];
  note?: string;
};

type MacroResponse = {
  generated_at: string;
  metrics?: Record<string, MacroMetric>;
  geopolitical?: MacroGeopolitical;
  warnings?: string[];
};

type CrisisLevel = "안정" | "주의" | "위험" | "극위험";

type Scenario = {
  title: string;
  probability: number;
  description: string;
  implication: string;
  tone: "positive" | "neutral" | "negative";
};

const MIN_MARKET_CAP = 1_000_000_000_000;

type VerdictTone = "green" | "amber" | "red";

type TacticalVerdict = {
  title: string;
  summary: string;
  tone: VerdictTone;
  pros: string[];
  cons: string[];
  conditions: string[];
};

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function mean(values: number[]): number {
  if (values.length === 0) return 0;
  const sum = values.reduce((acc, value) => acc + value, 0);
  return sum / values.length;
}

function median(values: number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
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

function formatMarketCap(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  if (value >= 1_0000_0000_0000) return `${(value / 1_0000_0000_0000).toFixed(2)}조`;
  if (value >= 1_0000_0000) return `${(value / 1_0000_0000).toFixed(0)}억`;
  return formatNumber(value);
}

function formatDateCompact(raw: string): string {
  const value = (raw || "").trim();
  if (value.length !== 8) return value || "-";
  return `${value.slice(0, 4)}.${value.slice(4, 6)}.${value.slice(6, 8)}`;
}

function formatChangeText(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  const sign = (value ?? 0) > 0 ? "+" : "";
  return `${sign}${(value ?? 0).toFixed(2)}%`;
}

function macroMetricClass(key: string, metric?: MacroMetric): string {
  if (!metric || metric.status !== "ok") return "bl";
  const change = metric.change_percent ?? 0;

  switch (key) {
    case "wti":
    case "vix":
    case "usdkrw":
      return change > 0 ? "dn" : "up";
    case "gold":
      return change > 0 ? "gol" : "bl";
    case "us10y":
      return change > 0 ? "warn" : "bl";
    default:
      return "bl";
  }
}

function weightFromScore(score: number, rank: number): number {
  const scoreWeight = clamp((score - 55) * 0.35, 0, 15);
  const rankPenalty = clamp((rank - 1) * 0.4, 0, 7);
  return clamp(scoreWeight - rankPenalty + 3, 1, 15);
}

function parseTopMarkdown(reportMarkdown: string, count: number): string[] {
  if (!reportMarkdown) return [];
  const lines = reportMarkdown.split("\n");
  const rows = lines.filter((line) => /^\|\d+\|/.test(line.trim()));
  return rows.slice(0, count).map((line) => {
    const cols = line.split("|").map((cell) => cell.trim());
    return `${cols[2] || "-"} · 총점 ${cols[5] || "-"}`;
  });
}

function buildSignal(item: QuantItem): { label: string; className: string } {
  if (item.total_score >= 85) return { label: "STRONG BUY", className: "sig sb" };
  if (item.total_score >= 75) return { label: "BUY", className: "sig sb" };
  if (item.total_score >= 65) return { label: "WATCH", className: "sig sw" };
  if (item.return_20d < -10 || item.total_score < 45) return { label: "REDUCE", className: "sig ss" };
  return { label: "HOLD", className: "sig sh" };
}

function computeCrisisScore(items: QuantItem[]): number {
  if (items.length === 0) return 50;

  const avg20 = mean(items.map((item) => item.return_20d));
  const avg1 = mean(items.map((item) => item.return_1d));
  const avgVol = mean(items.map((item) => item.volatility_20d));
  const medTurnoverRatio = median(items.map((item) => item.turnover_ratio));
  const positiveBreadth = items.filter((item) => item.return_20d > 0).length / items.length;
  const drawdown120 = Math.min(...items.map((item) => item.return_120d));

  const trendRisk = clamp((-avg20 / 12) * 32, 0, 32);
  const dayShockRisk = clamp((-avg1 / 3) * 18, 0, 18);
  const volRisk = clamp((avgVol / 5.5) * 22, 0, 22);
  const breadthRisk = clamp((0.55 - positiveBreadth) * 42, 0, 16);
  const liquidityRisk = clamp(((1.6 - medTurnoverRatio) / 1.6) * 12, 0, 12);
  const tailRisk = clamp((-drawdown120 / 45) * 10, 0, 10);

  return Math.round(clamp(trendRisk + dayShockRisk + volRisk + breadthRisk + liquidityRisk + tailRisk, 0, 100));
}

function crisisLevel(score: number): CrisisLevel {
  if (score >= 80) return "극위험";
  if (score >= 60) return "위험";
  if (score >= 40) return "주의";
  return "안정";
}

function scenarioSet(score: number): Scenario[] {
  const rebound = clamp(Math.round((100 - score) * 0.45), 10, 55);
  const tail = clamp(Math.round(score * 0.42), 12, 60);
  const base = clamp(100 - rebound - tail, 20, 70);
  const normalizeGap = 100 - rebound - tail - base;

  return [
    {
      title: "시나리오 A: 기술적 반등",
      probability: rebound + (normalizeGap > 0 ? normalizeGap : 0),
      description: "단기 과매도 해소와 거래대금 회복이 동반되는 경우",
      implication: "저점 분할매수 유효 · 변동성 확대는 지속",
      tone: "positive",
    },
    {
      title: "시나리오 B: 박스권 소화",
      probability: base,
      description: "상승·하락 요인이 상쇄되며 횡보가 이어지는 경우",
      implication: "추천 종목 중심 로테이션 선별",
      tone: "neutral",
    },
    {
      title: "시나리오 C: 하방 재개",
      probability: tail,
      description: "리스크 심리 확대로 하락 모멘텀이 재강화되는 경우",
      implication: "현금 비중 확대 · 손절 규칙 엄수",
      tone: "negative",
    },
  ];
}

function riskToneClass(level: CrisisLevel): string {
  if (level === "극위험") return "dn";
  if (level === "위험") return "warn";
  if (level === "주의") return "bl";
  return "up";
}

function formatClock(date: Date): string {
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  const ss = String(date.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

export default function QuantPage() {
  const router = useRouter();
  const [market, setMarket] = useState<MarketFilter>("ALL");
  const [limit, setLimit] = useState<number>(80);

  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  const [rankData, setRankData] = useState<RankResponse | null>(null);
  const [reportData, setReportData] = useState<ReportResponse | null>(null);
  const [macroData, setMacroData] = useState<MacroResponse | null>(null);

  const [autoRefresh, setAutoRefresh] = useState<boolean>(true);
  const [countdown, setCountdown] = useState<number>(60);
  const [lastUpdatedKST, setLastUpdatedKST] = useState<string>("-");
  const [clockKST, setClockKST] = useState<string>("--:--:--");
  const [dataPulse, setDataPulse] = useState<boolean>(false);
  const minMarketCap = MIN_MARKET_CAP;

  const fetchQuant = useCallback(async () => {
    setLoading(true);
    setError("");

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
      if (!rankRes.ok) {
        throw new Error(rankJson.error || "퀀트 랭킹 조회 실패");
      }

      const reportJson = await reportRes.json().catch(() => ({}));
      if (!reportRes.ok) {
        throw new Error(reportJson.error || "퀀트 리포트 조회 실패");
      }

      const macroJson = await macroRes.json().catch(() => ({}));

      setRankData(rankJson as RankResponse);
      setReportData(reportJson as ReportResponse);
      if (macroRes.ok) {
        setMacroData(macroJson as MacroResponse);
      } else {
        setMacroData({
          generated_at: new Date().toISOString(),
          metrics: {},
          geopolitical: {
            score: 0,
            level: "안정",
            note: typeof macroJson?.error === "string" ? macroJson.error : "매크로 데이터를 불러오지 못했습니다.",
          },
          warnings: [typeof macroJson?.error === "string" ? macroJson.error : "매크로 데이터를 불러오지 못했습니다."],
        });
      }
      setCountdown(60);
      setDataPulse(true);

      const now = new Date(new Date().toLocaleString("en-US", { timeZone: "Asia/Seoul" }));
      setLastUpdatedKST(formatClock(now));
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "퀀트 데이터 조회 중 오류가 발생했습니다.");
    } finally {
      setLoading(false);
    }
  }, [limit, market, minMarketCap]);

  useEffect(() => {
    void fetchQuant();
  }, [fetchQuant]);

  useEffect(() => {
    const timer = setInterval(() => {
      const now = new Date(new Date().toLocaleString("en-US", { timeZone: "Asia/Seoul" }));
      setClockKST(formatClock(now));
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!dataPulse) return;
    const timeout = setTimeout(() => setDataPulse(false), 550);
    return () => clearTimeout(timeout);
  }, [dataPulse]);

  useEffect(() => {
    if (!autoRefresh) return;

    const iv = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          void fetchQuant();
          return 60;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(iv);
  }, [autoRefresh, fetchQuant]);

  const rows = useMemo(() => rankData?.items ?? [], [rankData?.items]);

  const derived = useMemo(() => {
    if (rows.length === 0) {
      return {
        avg1d: 0,
        avg20d: 0,
        avg60d: 0,
        avg120d: 0,
        avgVol20: 0,
        avgTurnoverRatio: 0,
        kospiCount: 0,
        kosdaqCount: 0,
        positive20Count: 0,
        positive20Ratio: 0,
        topWinner: null as QuantItem | null,
        topLoser: null as QuantItem | null,
        topScore: null as QuantItem | null,
        riskScore: 50,
        riskLevel: "주의" as CrisisLevel,
        scenarios: scenarioSet(50),
      };
    }

    const avg1d = mean(rows.map((item) => item.return_1d));
    const avg20d = mean(rows.map((item) => item.return_20d));
    const avg60d = mean(rows.map((item) => item.return_60d));
    const avg120d = mean(rows.map((item) => item.return_120d));
    const avgVol20 = mean(rows.map((item) => item.volatility_20d));
    const avgTurnoverRatio = mean(rows.map((item) => item.turnover_ratio));

    const kospiCount = rows.filter((item) => item.market === "KOSPI").length;
    const kosdaqCount = rows.filter((item) => item.market === "KOSDAQ").length;

    const positive20Count = rows.filter((item) => item.return_20d > 0).length;
    const positive20Ratio = rows.length > 0 ? positive20Count / rows.length : 0;

    const sortedByDay = [...rows].sort((a, b) => b.return_1d - a.return_1d);
    const sortedByScore = [...rows].sort((a, b) => b.total_score - a.total_score);

    const riskScore = computeCrisisScore(rows);
    const riskLevel = crisisLevel(riskScore);

    return {
      avg1d,
      avg20d,
      avg60d,
      avg120d,
      avgVol20,
      avgTurnoverRatio,
      kospiCount,
      kosdaqCount,
      positive20Count,
      positive20Ratio,
      topWinner: sortedByDay[0] ?? null,
      topLoser: sortedByDay[sortedByDay.length - 1] ?? null,
      topScore: sortedByScore[0] ?? null,
      riskScore,
      riskLevel,
      scenarios: scenarioSet(riskScore),
    };
  }, [rows]);

  const macroMetrics = useMemo(() => macroData?.metrics ?? {}, [macroData?.metrics]);
  const geopolitical = useMemo(() => macroData?.geopolitical ?? null, [macroData?.geopolitical]);
  const headlineRiskScore = useMemo(
    () => Math.max(derived.riskScore, geopolitical?.score ?? 0),
    [derived.riskScore, geopolitical?.score]
  );
  const headlineRiskLevel = useMemo(
    () => crisisLevel(headlineRiskScore),
    [headlineRiskScore]
  );

  const topFromMarkdown = useMemo(
    () => parseTopMarkdown(reportData?.report_markdown || "", 5),
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
    const items: Array<{ title: string; detail: string; tone: "danger" | "warn" | "safe" | "info" }> = [];

    const geoHeadlines = geopolitical?.headlines ?? [];
    for (const headline of geoHeadlines.slice(0, 2)) {
      items.push({
        title: headline.title,
        detail: `${headline.source || "외부뉴스"}${headline.keywords?.length ? ` · ${headline.keywords.join(", ")}` : ""}`,
        tone: (geopolitical?.score ?? 0) >= 60 ? "danger" : "info",
      });
    }

    if (rows.length === 0) {
      return items;
    }

    if (derived.avg20d < -6) {
      items.push({
        title: "중기 모멘텀 급랭",
        detail: `추천 ${rows.length}개 평균 20일 수익률이 ${formatSignedPct(derived.avg20d)}입니다.`,
        tone: "danger",
      });
    } else if (derived.avg20d > 6) {
      items.push({
        title: "중기 모멘텀 우위",
        detail: `추천 ${rows.length}개 평균 20일 수익률이 ${formatSignedPct(derived.avg20d)}입니다.`,
        tone: "safe",
      });
    } else {
      items.push({
        title: "중기 모멘텀 혼조",
        detail: `추천 ${rows.length}개 평균 20일 수익률이 ${formatSignedPct(derived.avg20d)}입니다.`,
        tone: "warn",
      });
    }

    if (derived.avgVol20 > 4.5) {
      items.push({
        title: "변동성 경계 구간",
        detail: `평균 20일 변동성이 ${formatPct(derived.avgVol20)}로 높습니다.`,
        tone: "danger",
      });
    } else {
      items.push({
        title: "변동성 안정 구간",
        detail: `평균 20일 변동성이 ${formatPct(derived.avgVol20)}입니다.`,
        tone: "safe",
      });
    }

    if (derived.positive20Ratio < 0.4) {
      items.push({
        title: "상승 확산도 약함",
        detail: `20일 수익률 플러스 종목 비율 ${(derived.positive20Ratio * 100).toFixed(1)}%`,
        tone: "danger",
      });
    } else {
      items.push({
        title: "상승 확산도 유지",
        detail: `20일 수익률 플러스 종목 비율 ${(derived.positive20Ratio * 100).toFixed(1)}%`,
        tone: "info",
      });
    }

    if (derived.topWinner) {
      items.push({
        title: `오늘 상대강도 1위: ${derived.topWinner.name}`,
        detail: `${derived.topWinner.code} · 1일 ${formatSignedPct(derived.topWinner.return_1d)} · 총점 ${derived.topWinner.total_score.toFixed(2)}`,
        tone: "safe",
      });
    }

    if (derived.topLoser) {
      items.push({
        title: `오늘 상대약세 1위: ${derived.topLoser.name}`,
        detail: `${derived.topLoser.code} · 1일 ${formatSignedPct(derived.topLoser.return_1d)} · 총점 ${derived.topLoser.total_score.toFixed(2)}`,
        tone: "warn",
      });
    }

    return items.slice(0, 6);
  }, [derived, geopolitical, rows]);

  const tacticalVerdict = useMemo<TacticalVerdict>(() => {
    if (rows.length === 0) {
      return {
        title: "판단 보류",
        summary: "데이터가 부족하여 방어적 관망이 우선입니다.",
        tone: "amber",
        pros: ["현재 랭킹 데이터가 없어 추세 우위 판단 불가"],
        cons: ["근거 없는 진입은 손실 확률이 높습니다."],
        conditions: ["랭킹/리포트 데이터 갱신 후 판단"],
      };
    }

    const pressure = derived.avg20d < -5 || derived.avg60d < -10;
    const highStress = derived.riskScore >= 70 || derived.avgVol20 > 4.8;
    const rebound = derived.avg1d > 0.4 || derived.positive20Ratio > 0.58;

    if (highStress && pressure && !rebound) {
      return {
        title: "방어 포지션 유효",
        summary: "중기 하락 압력이 유지되어 방어적 운용과 현금 비중 확보가 우선입니다.",
        tone: "red",
        pros: [
          `위기지수 ${derived.riskScore}/100으로 고위험 구간`,
          `평균 20D ${formatSignedPct(derived.avg20d)}로 추세 약세`,
          `평균 변동성 ${formatPct(derived.avgVol20)}로 스트레스 확대`,
        ],
        cons: [
          `과매도 이후 기술적 반등 확률 ${(derived.scenarios[0]?.probability ?? 0)}%`,
          "급락 이후에는 숏/인버스 추격 진입 리스크가 큽니다.",
        ],
        conditions: [
          "평균 1D 수익률이 다시 -1% 이하로 하락할 때만 방어 강화",
          "상승 확산도(20D+) 40% 미만 유지 시에만 추가 방어",
        ],
      };
    }

    if (rebound && !highStress) {
      return {
        title: "완만한 리스크온 가능",
        summary: "단기 반등 신호가 있어 고점 추격보다 분할 진입 중심이 유효합니다.",
        tone: "green",
        pros: [
          `평균 1D ${formatSignedPct(derived.avg1d)}로 단기 매수세 확인`,
          `상승 확산도 ${(derived.positive20Ratio * 100).toFixed(1)}%로 시장 저변 개선`,
        ],
        cons: [
          `중기 모멘텀(20D ${formatSignedPct(derived.avg20d)})이 아직 완전 회복은 아님`,
          "변동성 장세에서는 손절 규칙 미준수 시 손실이 확대됩니다.",
        ],
        conditions: [
          "총점 70점 이상 + 20D/60D 동시 플러스 종목만 진입",
          "개별 종목 -5%~-7% 손절 기준 고정",
        ],
      };
    }

    return {
      title: "조건부 관망",
      summary: "추세와 반등 신호가 혼재되어 핵심 조건 충족 추천 종목만 제한적으로 선별해야 합니다.",
      tone: "amber",
      pros: [
        `시나리오 B 확률 ${(derived.scenarios[1]?.probability ?? 0)}%로 박스권 소화 가능`,
        `총점 1위 ${derived.topScore?.name ?? "-"} 중심의 국소적 강세 존재`,
      ],
      cons: [
        `리스크지수 ${derived.riskScore}로 급변동 리스크 잔존`,
        `평균 변동성 ${formatPct(derived.avgVol20)}로 추격 진입에 불리`,
      ],
      conditions: [
        "상승 확산도 50% 이상 회복 시 비중 확대",
        "위기지수 55 이하 하락 시 공격 비중 단계적 확대",
      ],
    };
  }, [derived, rows.length]);

  const crisisNarrative = useMemo(() => {
    if (rows.length === 0) {
      return "현재 랭킹 데이터가 비어 있어 위기 내러티브를 생성할 수 없습니다.";
    }

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

    return macroNarrative ? `${macroNarrative} | ${quantNarrative}` : quantNarrative;
  }, [derived, geopolitical, macroMetrics.usdkrw, macroMetrics.vix, macroMetrics.wti, rows.length]);

  const progressWidth = `${(countdown / 60) * 100}%`;
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

  const tickerText = useMemo(() => {
    const top = derived.topScore;
    const winner = derived.topWinner;
    const loser = derived.topLoser;
    const wti = macroMetrics.wti;
    const vix = macroMetrics.vix;
    const usdkrw = macroMetrics.usdkrw;

    return [
      `📌 시장: ${market} · 유니버스 ${formatNumber(rankData?.universe_count ?? 0)}개`,
      wti?.status === "ok" ? `⛽ WTI ${wti.display}` : "⛽ WTI 대기중",
      vix?.status === "ok" ? `😨 VIX ${vix.display}` : "😨 VIX 대기중",
      usdkrw?.status === "ok" ? `💱 USD/KRW ${usdkrw.display}` : "💱 USD/KRW 대기중",
      `⚡ 평균 20일: ${formatSignedPct(derived.avg20d)} · 평균 60일: ${formatSignedPct(derived.avg60d)}`,
      top ? `🏆 총점 1위 ${top.name}(${top.code}) ${top.total_score.toFixed(2)}` : "🏆 총점 1위 데이터 준비중",
      winner ? `🚀 일간 강세 ${winner.name} ${formatSignedPct(winner.return_1d)}` : "🚀 일간 강세 데이터 준비중",
      loser ? `📉 일간 약세 ${loser.name} ${formatSignedPct(loser.return_1d)}` : "📉 일간 약세 데이터 준비중",
      geopolitical ? `🌍 지정학 ${geopolitical.score}/100 (${geopolitical.level})` : "🌍 지정학 데이터 준비중",
      `🧭 종합 리스크 ${headlineRiskScore}/100 (${headlineRiskLevel})`,
    ].join("  |  ");
  }, [derived, geopolitical, headlineRiskLevel, headlineRiskScore, macroMetrics.usdkrw, macroMetrics.vix, macroMetrics.wti, market, rankData?.universe_count]);

  return (
    <div className="quant-war-v3">
      <div className="flash" data-show={loading ? "on" : "off"}>
        {loading ? "⟳ 업데이트 중" : "⟳ 준비완료"}
      </div>

      <div className="war-top">
        <div className="ticker-row">
          <span>🚨</span>
          <div className="ticker-wrap">
            <span className="ticker">{tickerText}</span>
          </div>
        </div>

        <div className="hrow">
          <div className="logo">
            <div className="logo-i">QX</div>
            <div>
              <div className="logo-t">퀀트 위기대응 리포트</div>
              <div className="logo-s">기존 저장 데이터 기반 자동 분석 · Asia/Seoul</div>
            </div>
          </div>

          <div className="hstats">
            <div className="hs">
              <div className={`hs-v ${riskToneClass(headlineRiskLevel)}`}>리스크 {headlineRiskScore}</div>
              <div className="hs-l">{headlineRiskLevel}</div>
            </div>
            <div className="hs">
              <div className={`hs-v ${derived.avg20d >= 0 ? "up" : "dn"}`}>{formatSignedPct(derived.avg20d)}</div>
              <div className="hs-l">추천 평균 20D</div>
            </div>
            <div className="hs">
              <div className="hs-v gol">{clockKST}</div>
              <div className="hs-l">KST</div>
            </div>
          </div>
        </div>
      </div>

      <div className="ubar">
        <span>
          <span className="ldot" />LIVE
        </span>
        <button
          type="button"
          className={`ubtn ${autoRefresh ? "bon" : "boff"}`}
          onClick={() => {
            setAutoRefresh((prev) => !prev);
            if (!autoRefresh) setCountdown(60);
          }}
        >
          {autoRefresh ? "자동갱신 ON (60초)" : "자동갱신 OFF"}
        </button>
        <button
          type="button"
          className="ubtn bman"
          onClick={() => {
            void fetchQuant();
          }}
        >
          ⟳ 수동갱신
        </button>
        <div className="pbg">
          <div className="pfill" style={{ width: progressWidth }} />
        </div>
        <span id="nxt">{autoRefresh ? `${countdown}초 후 갱신` : "자동 OFF"}</span>
        <span className="last-up">마지막: {lastUpdatedKST}</span>
      </div>

      <div className="wrap">
        <div className="sec">🌐 저장 데이터 + 외부 API 매크로 요약</div>

        <div className="cbar">
          <div className="cbar-icon">🚨</div>
          <div className="cbar-text">
            <strong>[실시간 판정]</strong> {crisisNarrative}
          </div>
          <div className={`cbar-verdict ${tacticalVerdict.tone}`}>
            <div className="cv-lbl">전술 판단</div>
            <div className="cv-main">{tacticalVerdict.title}</div>
            <div className="cv-sub">{headlineRiskLevel} · {headlineRiskScore}/100</div>
          </div>
        </div>

        <div className="control-bar">
          <div className="control-item">
            <label htmlFor="quant-market">시장</label>
            <select
              id="quant-market"
              value={market}
              onChange={(event) => setMarket(event.target.value as MarketFilter)}
            >
              <option value="ALL">ALL</option>
              <option value="KOSPI">KOSPI</option>
              <option value="KOSDAQ">KOSDAQ</option>
            </select>
          </div>
          <div className="control-item">
            <label htmlFor="quant-limit">랭킹 수</label>
            <input
              id="quant-limit"
              type="number"
              min={20}
              max={200}
              value={limit}
              onChange={(event) =>
                setLimit(
                  clamp(
                    Number.parseInt(event.target.value || "80", 10),
                    20,
                    200
                  )
                )
              }
            />
          </div>
          <div className="control-item">
            <label htmlFor="quant-mincap">최소 시총(원, 1조 고정)</label>
            <input
              id="quant-mincap"
              type="text"
              value={String(MIN_MARKET_CAP)}
              readOnly
            />
          </div>
          <div className="control-action">
            <button type="button" className="ubtn bman" onClick={() => void fetchQuant()}>
              리포트 갱신
            </button>
          </div>
        </div>

        <div className="mgrid">
          <div className="mc cr">
            <div className="ml">WTI 원유</div>
            <div className={`mv ${macroMetricClass("wti", macroMetrics.wti)}`}>{macroMetrics.wti?.display ?? "-"}</div>
            <div className={`mc2 ${macroMetricClass("wti", macroMetrics.wti)}`}>
              {macroMetrics.wti?.status === "ok" ? formatChangeText(macroMetrics.wti.change_percent) : macroMetrics.wti?.note ?? "FRED 연동 필요"}
            </div>
          </div>
          <div className="mc cr">
            <div className="ml">VIX 공포지수</div>
            <div className={`mv ${macroMetricClass("vix", macroMetrics.vix)}`}>{macroMetrics.vix?.display ?? "-"}</div>
            <div className={`mc2 ${macroMetricClass("vix", macroMetrics.vix)}`}>
              {macroMetrics.vix?.status === "ok" ? formatChangeText(macroMetrics.vix.change_percent) : macroMetrics.vix?.note ?? "CBOE/FRED 필요"}
            </div>
          </div>
          <div className="mc sf">
            <div className="ml">XAU/USD</div>
            <div className={`mv ${macroMetricClass("gold", macroMetrics.gold)}`}>{macroMetrics.gold?.display ?? "-"}</div>
            <div className={`mc2 ${macroMetricClass("gold", macroMetrics.gold)}`}>
              {macroMetrics.gold?.status === "ok" ? formatChangeText(macroMetrics.gold.change_percent) : macroMetrics.gold?.note ?? "Alpha/Polygon 필요"}
            </div>
          </div>
          <div className="mc wn">
            <div className="ml">USD/KRW 환율</div>
            <div className={`mv ${macroMetricClass("usdkrw", macroMetrics.usdkrw)}`}>{macroMetrics.usdkrw?.display ?? "-"}</div>
            <div className={`mc2 ${macroMetricClass("usdkrw", macroMetrics.usdkrw)}`}>
              {macroMetrics.usdkrw?.status === "ok" ? formatChangeText(macroMetrics.usdkrw.change_percent) : macroMetrics.usdkrw?.note ?? "Alpha/ECOS/exchangerate 필요"}
            </div>
          </div>
          <div className="mc wn">
            <div className="ml">미국채 10Y</div>
            <div className={`mv ${macroMetricClass("us10y", macroMetrics.us10y)}`}>{macroMetrics.us10y?.display ?? "-"}</div>
            <div className={`mc2 ${macroMetricClass("us10y", macroMetrics.us10y)}`}>
              {macroMetrics.us10y?.status === "ok" ? formatChangeText(macroMetrics.us10y.change_percent) : macroMetrics.us10y?.note ?? "FRED 연동 필요"}
            </div>
          </div>
          <div className="mc cr">
            <div className="ml">지정학 이벤트</div>
            <div className={`mv ${riskToneClass(geopolitical?.level ?? "주의")}`}>{geopolitical?.score ?? "-"}</div>
            <div className={`mc2 ${riskToneClass(geopolitical?.level ?? "주의")}`}>
              {geopolitical?.level ?? "GDELT/NewsAPI"}
            </div>
          </div>
          <div className="mc cr">
            <div className="ml">유니버스</div>
            <div className="mv">{formatNumber(rankData?.universe_count ?? 0)}</div>
            <div className="mc2">KOSPI/KOSDAQ 1조 이상 종목수</div>
          </div>
          <div className={`mc ${derived.avg20d < 0 ? "cr" : "sf"}`}>
            <div className="ml">평균 20D</div>
            <div className={`mv ${derived.avg20d >= 0 ? "up" : "dn"}`}>{formatSignedPct(derived.avg20d)}</div>
            <div className={`mc2 ${derived.avg20d >= 0 ? "up" : "dn"}`}>추천 {rows.length}개 기준</div>
          </div>
          <div className={`mc ${derived.avg60d < 0 ? "cr" : "sf"}`}>
            <div className="ml">평균 60D</div>
            <div className={`mv ${derived.avg60d >= 0 ? "up" : "dn"}`}>{formatSignedPct(derived.avg60d)}</div>
            <div className={`mc2 ${derived.avg60d >= 0 ? "up" : "dn"}`}>중기 추세</div>
          </div>
          <div className={`mc ${derived.avgVol20 > 4.5 ? "wn" : "sf"}`}>
            <div className="ml">평균 변동성(20D)</div>
            <div className={`mv ${derived.avgVol20 > 4.5 ? "warn" : "bl"}`}>{formatPct(derived.avgVol20)}</div>
            <div className="mc2">표준편차 기준</div>
          </div>
          <div className="mc pr">
            <div className="ml">상승 확산도</div>
            <div className="mv pu">{(derived.positive20Ratio * 100).toFixed(1)}%</div>
            <div className="mc2">20D 플러스 비율</div>
          </div>
          <div className="mc cr">
            <div className="ml">종합 위기지수</div>
            <div className={`mv ${riskToneClass(headlineRiskLevel)}`}>{headlineRiskScore}</div>
            <div className={`mc2 ${riskToneClass(headlineRiskLevel)}`}>{headlineRiskLevel}</div>
          </div>
          <div className="mc sf">
            <div className="ml">데이터 기준일</div>
            <div className="mv bl">{formatDateCompact(rankData?.as_of_max || "")}</div>
            <div className="mc2">{formatDateCompact(rankData?.as_of_min || "")}</div>
          </div>
        </div>

        <div className="main3">
          <div>
            <div className="panel">
              <div className="ph">
                <span className="pht">🎲 리스크 시나리오</span>
                <span className="bdg bn">자동 계산</span>
              </div>

              <div className="scenario-grid">
                {derived.scenarios.map((scenario) => (
                  <div
                    key={scenario.title}
                    className={`scenario-card ${
                      scenario.tone === "positive"
                        ? "sca"
                        : scenario.tone === "neutral"
                          ? "scb"
                          : "scc"
                    }`}
                  >
                    <div className={`sp ${scenario.tone === "positive" ? "up" : scenario.tone === "neutral" ? "warn" : "dn"}`}>
                      {scenario.probability}%
                    </div>
                    <div className="sn">{scenario.title}</div>
                    <div className="sd">{scenario.description}</div>
                    <div className="sc-i">{scenario.implication}</div>
                  </div>
                ))}
              </div>
            </div>

            <div className="panel mt14">
              <div className="ph">
                <span className="pht">🔎 전술 판단 박스</span>
                <span className={`bdg ${tacticalVerdict.tone === "green" ? "bn" : tacticalVerdict.tone === "amber" ? "bw" : "br"}`}>
                  {tacticalVerdict.title}
                </span>
              </div>
              <div className={`verdict-box ${tacticalVerdict.tone}`}>
                <div className="verdict-summary">{tacticalVerdict.summary}</div>
                <div className="verdict-cond">
                  {tacticalVerdict.conditions.map((condition, index) => (
                    <div key={`condition-${index}`}>• {condition}</div>
                  ))}
                </div>
              </div>
              <div className="pros-cons">
                <div className="pc-box">
                  <div className="pc-head up">✅ 유효 근거</div>
                  <div className="pc-body">
                    {tacticalVerdict.pros.map((item, index) => (
                      <div key={`pro-${index}`}>· {item}</div>
                    ))}
                  </div>
                </div>
                <div className="pc-box">
                  <div className="pc-head dn">❌ 주의 포인트</div>
                  <div className="pc-body">
                    {tacticalVerdict.cons.map((item, index) => (
                      <div key={`con-${index}`}>· {item}</div>
                    ))}
                  </div>
                </div>
              </div>
            </div>

            <div className="panel mt14">
              <div className="ph">
                <span className="pht">📊 퀀트 핵심지표</span>
                <span className="bdg bn">추천 {rows.length}</span>
              </div>
              <div className="qgrid">
                <div className="qi">
                  <div className="qi-l">모멘텀 평균 (20D)</div>
                  <div className={`qi-v ${derived.avg20d >= 0 ? "up" : "dn"} ${dataPulse ? "dp" : ""}`}>{formatSignedPct(derived.avg20d)}</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp((Math.abs(derived.avg20d) / 12) * 100, 0, 100)}%`, background: derived.avg20d >= 0 ? "var(--g)" : "var(--r)" }} />
                  </div>
                  <div className="qi-sub">리스크 계산 입력값</div>
                </div>
                <div className="qi">
                  <div className="qi-l">모멘텀 평균 (60D)</div>
                  <div className={`qi-v ${derived.avg60d >= 0 ? "up" : "dn"} ${dataPulse ? "dp" : ""}`}>{formatSignedPct(derived.avg60d)}</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp((Math.abs(derived.avg60d) / 22) * 100, 0, 100)}%`, background: derived.avg60d >= 0 ? "var(--g)" : "var(--o)" }} />
                  </div>
                  <div className="qi-sub">중기 추세 강도</div>
                </div>
                <div className="qi">
                  <div className="qi-l">변동성 평균 (20D)</div>
                  <div className={`qi-v ${derived.avgVol20 > 4.5 ? "warn" : "bl"} ${dataPulse ? "dp" : ""}`}>{formatPct(derived.avgVol20)}</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp((derived.avgVol20 / 6) * 100, 0, 100)}%`, background: derived.avgVol20 > 4.5 ? "var(--w)" : "var(--ac)" }} />
                  </div>
                  <div className="qi-sub">평균 표준편차</div>
                </div>
                <div className="qi">
                  <div className="qi-l">거래대금비율 평균</div>
                  <div className={`qi-v bl ${dataPulse ? "dp" : ""}`}>{formatPct(derived.avgTurnoverRatio)}</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp((derived.avgTurnoverRatio / 3) * 100, 0, 100)}%`, background: "var(--ac)" }} />
                  </div>
                  <div className="qi-sub">시장 유동성 강도</div>
                </div>
                <div className="qi">
                  <div className="qi-l">상승 종목 비율 (20D)</div>
                  <div className={`qi-v pu ${dataPulse ? "dp" : ""}`}>{(derived.positive20Ratio * 100).toFixed(1)}%</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp(derived.positive20Ratio * 100, 0, 100)}%`, background: "var(--purple)" }} />
                  </div>
                  <div className="qi-sub">시장 확산도</div>
                </div>
                <div className="qi">
                  <div className="qi-l">총점 1위</div>
                  <div className={`qi-v gol ${dataPulse ? "dp" : ""}`}>{derived.topScore?.name ?? "-"}</div>
                  <div className="qi-bar">
                    <div className="qi-fill" style={{ width: `${clamp(derived.topScore?.total_score ?? 0, 0, 100)}%`, background: "var(--gold)" }} />
                  </div>
                  <div className="qi-sub">{derived.topScore ? `${derived.topScore.code} · ${derived.topScore.total_score.toFixed(2)}` : "데이터 없음"}</div>
                </div>
              </div>
            </div>
          </div>

          <div>
            <div className="panel">
              <div className="ph">
                <span className="pht">🎯 추천 종목 테이블</span>
                <span className="bdg bw">추천용</span>
              </div>
              <table className="st">
                <thead>
                  <tr>
                    <th>종목</th>
                    <th>시그널</th>
                    <th>총점</th>
                    <th>추천 비중</th>
                    <th>20D</th>
                    <th>60D</th>
                    <th>시총</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(0, 12).map((row) => {
                    const signal = buildSignal(row);
                    const weight = weightFromScore(row.total_score, row.rank);
                    return (
                      <tr
                        key={`${row.market}:${row.code}:${row.as_of}`}
                        className="click-row"
                        role="button"
                        tabIndex={0}
                        aria-label={`${row.name} ${row.code} 상세보기`}
                        onClick={() => moveToDetail(row)}
                        onKeyDown={(event) => {
                          if (event.key === "Enter" || event.key === " ") {
                            event.preventDefault();
                            moveToDetail(row);
                          }
                        }}
                      >
                        <td>
                          <div className="sn2">{row.name}</div>
                          <div className="sc2">{row.code} · {row.market} · 클릭 시 상세보기</div>
                        </td>
                        <td>
                          <span className={signal.className}>{signal.label}</span>
                        </td>
                        <td>
                          <div className="pv">{row.total_score.toFixed(2)}</div>
                        </td>
                        <td>
                          <div className={`pv ${dataPulse ? "dp" : ""}`}>{weight.toFixed(1)}%</div>
                          <div className="wb">
                            <div className="wf" style={{ width: `${clamp(weight * 6.5, 10, 100)}%` }} />
                          </div>
                        </td>
                        <td className={row.return_20d >= 0 ? "up" : "dn"}>{formatSignedPct(row.return_20d)}</td>
                        <td className={row.return_60d >= 0 ? "up" : "dn"}>{formatSignedPct(row.return_60d)}</td>
                        <td>{formatMarketCap(row.market_cap)}</td>
                      </tr>
                    );
                  })}
                  {rows.length === 0 && (
                    <tr>
                      <td colSpan={7} className="empty-row">표시할 데이터가 없습니다.</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            <div className="panel mt14">
              <div className="ph">
                <span className="pht">📝 자동 리포트 요약</span>
                <span className="bdg bpu">Markdown</span>
              </div>
              <div className="sbox">
                {topFromMarkdown.length > 0 ? (
                  topFromMarkdown.map((line, index) => (
                    <div key={`${line}-${index}`} className="si">
                      <div className="snum">{index + 1}</div>
                      <div>
                        <div className="stit">추천 후보</div>
                        <div className="sdesc">{line}</div>
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="sdesc">리포트 요약 데이터가 없습니다.</div>
                )}
              </div>
            </div>
          </div>

          <div className="rcol">
            <div className="panel">
              <div className="ph">
                <span className="pht">⚡ 위기지수 게이지</span>
                <span className="bdg br">{headlineRiskLevel}</span>
              </div>
              <div className="rw">
                <div className="rl">
                  <div className="rn" style={{ left: `${headlineRiskScore}%` }} />
                </div>
                <div className="rls">
                  <span>안정</span>
                  <span>주의</span>
                  <span>위험</span>
                  <span>극위험</span>
                </div>
                <div className={`rs ${riskToneClass(headlineRiskLevel)}`}>{headlineRiskScore}</div>
                <div className={`rsl ${riskToneClass(headlineRiskLevel)}`}>{headlineRiskLevel} 구간</div>

                <div className="rrows">
                  <div className="rrow">
                    <span className="rname">지정학 충격</span>
                    <div className="rbg"><div className="rfill" style={{ width: `${clamp(geopolitical?.score ?? 0, 0, 100)}%`, background: "var(--r)" }} /></div>
                    <span className={`rpct ${riskToneClass(geopolitical?.level ?? "주의")}`}>{geopolitical?.score ?? 0}%</span>
                  </div>
                  <div className="rrow">
                    <span className="rname">20D 모멘텀 압력</span>
                    <div className="rbg"><div className="rfill" style={{ width: `${clamp((-derived.avg20d / 12) * 100, 0, 100)}%`, background: "var(--o)" }} /></div>
                    <span className="rpct warn">{formatSignedPct(derived.avg20d)}</span>
                  </div>
                  <div className="rrow">
                    <span className="rname">단기 충격(1D)</span>
                    <div className="rbg"><div className="rfill" style={{ width: `${clamp((-derived.avg1d / 3) * 100, 0, 100)}%`, background: "var(--r)" }} /></div>
                    <span className="rpct warn">{formatSignedPct(derived.avg1d)}</span>
                  </div>
                  <div className="rrow">
                    <span className="rname">변동성 스트레스</span>
                    <div className="rbg"><div className="rfill" style={{ width: `${clamp((derived.avgVol20 / 6) * 100, 0, 100)}%`, background: "var(--w)" }} /></div>
                    <span className="rpct warn">{formatPct(derived.avgVol20)}</span>
                  </div>
                  <div className="rrow">
                    <span className="rname">상승 확산도</span>
                    <div className="rbg"><div className="rfill" style={{ width: `${clamp(derived.positive20Ratio * 100, 0, 100)}%`, background: "var(--g)" }} /></div>
                    <span className="rpct up">{(derived.positive20Ratio * 100).toFixed(1)}%</span>
                  </div>
                </div>
              </div>
            </div>

            <div className="panel mt14">
              <div className="ph">
                <span className="pht">🔔 실시간 알람</span>
                <span className="bdg br">{alerts.length} ACTIVE</span>
              </div>
              <div className="alist">
                {alerts.map((alert, index) => (
                  <div key={`${alert.title}-${index}`} className="ali">
                    <div
                      className={`adot ${
                        alert.tone === "danger"
                          ? "dr"
                          : alert.tone === "warn"
                            ? "dw"
                            : alert.tone === "safe"
                              ? "dg"
                              : "db"
                      }`}
                    />
                    <div>
                      <div className="at2">{alert.title}</div>
                      <div className="ad2">{alert.detail}</div>
                      <div className="atime">{lastUpdatedKST} · 자동판정</div>
                    </div>
                  </div>
                ))}
                {alerts.length === 0 && <div className="empty-alert">알람 데이터가 없습니다.</div>}
              </div>
            </div>

            <div className="panel mt14">
              <div className="ph">
                <span className="pht">⏱️ 다음 자동 갱신</span>
                <span className="bdg bn">실시간</span>
              </div>
              <div className="cdown">
                <div className="cnum">{autoRefresh ? countdown : "OFF"}</div>
                <div className="clbl">{autoRefresh ? "초 후 자동 업데이트" : "자동 갱신 비활성화"}</div>
                <div className="mt12">
                  <button type="button" className="ubtn bman" onClick={() => void fetchQuant()}>
                    ⟳ 지금 갱신
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="botrow">
          <div className="panel">
            <div className="ph">
              <span className="pht">📋 실행 액션플랜</span>
              <span className="bdg bw">데이터 기반</span>
            </div>
            <div className="sbox">
              <div className="si">
                <div className="snum">1</div>
                <div>
                  <div className="stit">리스크 레벨 우선 확인</div>
                  <div className="sdesc">현재 위기지수는 {derived.riskScore}/100 ({derived.riskLevel})입니다. 위험 이상 구간에서는 신규 비중을 축소하고 기존 포지션 방어를 우선합니다.</div>
                </div>
              </div>
              <div className="si">
                <div className="snum">2</div>
                <div>
                  <div className="stit">추천 점수 + 추세 동시 충족 종목만 선별</div>
                  <div className="sdesc">총점 70 이상이면서 20일/60일 수익률이 동시에 플러스인 종목을 우선 검토하고, 단기 급등 추격은 피합니다.</div>
                </div>
              </div>
              <div className="si">
                <div className="snum">3</div>
                <div>
                  <div className="stit">손절 기준을 점수와 분리</div>
                  <div className="sdesc">점수가 높더라도 변동성 급등 구간에서는 손절 기준(-5%~-7%)을 고정해 감정 개입을 줄입니다.</div>
                </div>
              </div>
              <div className="si">
                <div className="snum">4</div>
                <div>
                  <div className="stit">현금 비중 유지</div>
                  <div className="sdesc">리스크가 &quot;위험&quot; 이상이면 현금/단기자금 비중을 최소 20% 이상 유지해 변동성 이벤트에 대응합니다.</div>
                </div>
              </div>
              <div className="si">
                <div className="snum">5</div>
                <div>
                  <div className="stit">갱신 주기 고정 운영</div>
                  <div className="sdesc">현재 60초 자동 갱신으로 동일 기준을 반복 점검하고, 장중에는 같은 규칙으로만 매매 의사결정을 내립니다.</div>
                </div>
              </div>
            </div>
          </div>

          <div className="panel">
            <div className="ph">
              <span className="pht">🧩 외부 API 연동 상태</span>
              <span className="bdg br">{macroData?.generated_at ? "LIVE" : "설정 필요"}</span>
            </div>
            <div className="abox">
              {macroStatusRows.map((row) => (
                <div key={row.key} className="ai">
                  <span className="al">{row.label}</span>
                  <span className="av">
                    {row.metric?.status === "ok"
                      ? `연동 완료 · ${row.metric.display}${row.metric.provider ? ` · ${row.metric.provider}` : ""}${row.metric.as_of ? ` · ${row.metric.as_of}` : ""}`
                      : row.metric?.note || "API 키 또는 제공자 설정이 필요합니다."}
                  </span>
                </div>
              ))}
              <div className="ai">
                <span className="al">지정학 이벤트 점수</span>
                <span className="av">
                  {(geopolitical?.providers?.length ?? 0) > 0
                    ? `연동 완료 · ${geopolitical?.providers?.join(", ")} · 점수 ${geopolitical?.score ?? 0}/100`
                    : geopolitical?.note || "NewsAPI/GDELT 설정이 필요합니다."}
                </span>
              </div>
              <div className="ai">
                <span className="al">국내 실시간 시세</span>
                <span className="av">현재 저장 데이터는 일봉 중심입니다. 실시간 시세는 `키움 OpenAPI+` 또는 `KIS 실시간 웹소켓`을 추가로 붙여야 합니다.</span>
              </div>
              <div className="ai">
                <span className="al">연동 경고</span>
                <span className="av">
                  {macroData?.warnings?.length ? macroData.warnings.join(" | ") : "경고 없음"}
                </span>
              </div>
              <div className="ai">
                <span className="al">뉴스 키워드</span>
                <span className="av">NewsAPI는 `backend-go/data/kospi_daily`, `backend-go/data/kosdaq_daily` 저장 CSV에서 계산한 국내 시총 1조 이상 종목명만 OR 쿼리 그룹으로 묶어 수집합니다.</span>
              </div>
              <div className="ai">
                <span className="al">현재 사용 데이터</span>
                <span className="av">`backend-go/data/kospi_daily`, `backend-go/data/kosdaq_daily` 저장 CSV의 최신 거래일 데이터와 `quant/macro` 최신 발행일 뉴스만 사용합니다.</span>
              </div>
            </div>
          </div>
        </div>

        {error && <div className="error-box">{error}</div>}
      </div>

      <style jsx>{`
        @import url('https://fonts.googleapis.com/css2?family=Space+Mono:wght@400;700&family=Noto+Sans+KR:wght@300;400;500;700;900&display=swap');

        .quant-war-v3 {
          --bg: #f8fafc;
          --panel: #ffffff;
          --panel2: #f1f5f9;
          --b1: #cbd5e1;
          --b2: #e2e8f0;
          --ac: #0891b2;
          --ac2: #0d9488;
          --r: #dc2626;
          --o: #ea580c;
          --w: #ca8a04;
          --g: #16a34a;
          --gold: #b45309;
          --purple: #7c3aed;
          --t: #0f172a;
          --t2: #475569;
          --t3: #64748b;

          background: var(--bg);
          color: var(--t);
          font-family: 'Noto Sans KR', sans-serif;
          border-radius: 14px;
          overflow: hidden;
          border: 1px solid #e2e8f0;
          position: relative;
        }

        .quant-war-v3::before {
          content: "";
          position: absolute;
          inset: 0;
          background-image: linear-gradient(rgba(8, 145, 178, 0.06) 1px, transparent 1px),
            linear-gradient(90deg, rgba(8, 145, 178, 0.06) 1px, transparent 1px);
          background-size: 36px 36px;
          pointer-events: none;
          z-index: 0;
        }

        .flash {
          position: fixed;
          top: 92px;
          right: 16px;
          background: rgba(22, 163, 74, 0.95);
          color: #fff;
          padding: 5px 10px;
          border-radius: 999px;
          font-size: 10px;
          font-weight: 700;
          font-family: 'Space Mono', monospace;
          opacity: 0;
          transform: translateY(-6px);
          transition: all 0.25s;
          z-index: 999;
        }

        .flash[data-show="on"] {
          opacity: 1;
          transform: translateY(0);
        }

        .war-top {
          position: sticky;
          top: 0;
          z-index: 30;
          background: linear-gradient(135deg, #ffffff, #f1f5f9);
          border-bottom: 2px solid var(--r);
          box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
        }

        .ticker-row {
          background: var(--r);
          color: #fff;
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 6px 12px;
          font-size: 11px;
          font-weight: 700;
        }

        .ticker-wrap {
          flex: 1;
          overflow: hidden;
          white-space: nowrap;
        }

        .ticker {
          display: inline-block;
          padding-left: 100%;
          animation: scroll 34s linear infinite;
        }

        @keyframes scroll {
          0% { transform: translateX(0); }
          100% { transform: translateX(-100%); }
        }

        .hrow {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 12px 14px;
          gap: 10px;
        }

        .logo {
          display: flex;
          align-items: center;
          gap: 10px;
        }

        .logo-i {
          width: 36px;
          height: 36px;
          border-radius: 8px;
          background: linear-gradient(135deg, var(--r), var(--o));
          display: flex;
          align-items: center;
          justify-content: center;
          font-family: 'Space Mono', monospace;
          font-weight: 700;
          color: #fff;
        }

        .logo-t {
          font-size: 14px;
          font-weight: 800;
          color: var(--t);
        }

        .logo-s {
          font-size: 10px;
          color: var(--t2);
        }

        .hstats {
          display: flex;
          align-items: center;
          gap: 16px;
        }

        .hs {
          text-align: right;
        }

        .hs-v {
          font-family: 'Space Mono', monospace;
          font-size: 13px;
          font-weight: 700;
        }

        .hs-l {
          font-size: 9px;
          color: var(--t2);
        }

        .ubar {
          position: sticky;
          top: 82px;
          z-index: 24;
          background: var(--panel2);
          border-bottom: 1px solid var(--b2);
          display: flex;
          align-items: center;
          gap: 10px;
          padding: 8px 12px;
          font-size: 11px;
          flex-wrap: wrap;
        }

        .ldot {
          display: inline-block;
          width: 8px;
          height: 8px;
          background: var(--g);
          border-radius: 50%;
          margin-right: 4px;
          animation: pulse 1.5s infinite;
        }

        @keyframes pulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.45; transform: scale(1.35); }
        }

        .ubtn {
          border: 1px solid;
          border-radius: 999px;
          padding: 5px 12px;
          font-size: 10px;
          font-weight: 700;
          cursor: pointer;
          font-family: 'Space Mono', monospace;
          transition: all 0.2s;
        }

        .ubtn:hover { transform: translateY(-1px); }

        .bon { background: rgba(0, 230, 122, 0.13); border-color: rgba(0, 230, 122, 0.45); color: var(--g); }
        .boff { background: rgba(255, 43, 79, 0.12); border-color: rgba(255, 43, 79, 0.45); color: var(--r); }
        .bman { background: rgba(0, 212, 255, 0.12); border-color: rgba(0, 212, 255, 0.45); color: var(--ac); }
        .bman:hover { background: rgba(0, 212, 255, 0.22); }

        .pbg {
          width: 150px;
          height: 4px;
          border-radius: 3px;
          overflow: hidden;
          background: #e2e8f0;
        }

        .pfill {
          height: 100%;
          background: linear-gradient(90deg, var(--ac), var(--ac2));
          transition: width 0.8s linear;
        }

        #nxt {
          color: var(--ac);
          font-family: 'Space Mono', monospace;
          font-size: 10px;
        }

        .last-up {
          margin-left: auto;
          color: var(--t2);
          font-size: 10px;
          font-family: 'Space Mono', monospace;
        }

        .wrap {
          position: relative;
          z-index: 1;
          width: 100%;
          max-width: none;
          margin: 0 auto;
          padding: 10px 12px 28px;
        }

        .sec {
          display: flex;
          align-items: center;
          gap: 8px;
          color: var(--t2);
          font-size: 10px;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: 2px;
          margin: 8px 0 10px;
        }

        .sec::after {
          content: "";
          height: 1px;
          flex: 1;
          background: var(--b2);
        }

        .cbar {
          display: grid;
          grid-template-columns: 34px minmax(0, 1fr) 158px;
          gap: 10px;
          align-items: center;
          border: 1px solid rgba(220, 38, 38, 0.35);
          background: linear-gradient(90deg, rgba(220, 38, 38, 0.06), rgba(234, 88, 12, 0.04));
          border-radius: 10px;
          padding: 10px 12px;
          margin-bottom: 10px;
        }

        .cbar-icon {
          width: 32px;
          height: 32px;
          border-radius: 8px;
          background: rgba(220, 38, 38, 0.12);
          border: 1px solid rgba(220, 38, 38, 0.35);
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 16px;
        }

        .cbar-text {
          font-size: 10px;
          color: var(--t2);
          line-height: 1.55;
        }

        .cbar-text strong {
          color: var(--w);
          margin-right: 5px;
        }

        .cbar-verdict {
          border: 1px solid;
          border-radius: 8px;
          padding: 7px 8px;
          text-align: center;
        }

        .cbar-verdict.green { border-color: rgba(0, 230, 122, 0.4); background: rgba(0, 230, 122, 0.1); }
        .cbar-verdict.amber { border-color: rgba(255, 191, 0, 0.45); background: rgba(255, 191, 0, 0.08); }
        .cbar-verdict.red { border-color: rgba(255, 43, 79, 0.45); background: rgba(255, 43, 79, 0.1); }

        .cv-lbl {
          font-size: 8px;
          color: var(--t2);
          text-transform: uppercase;
          letter-spacing: 1px;
        }

        .cv-main {
          margin-top: 2px;
          font-size: 14px;
          font-weight: 800;
        }

        .cv-sub {
          margin-top: 2px;
          font-size: 9px;
          color: var(--t3);
        }

        .mgrid {
          display: grid;
          grid-template-columns: repeat(12, minmax(0, 1fr));
          gap: 8px;
        }

        .control-bar {
          border: 1px solid var(--b2);
          border-radius: 10px;
          background: linear-gradient(135deg, rgba(8, 145, 178, 0.06), rgba(13, 148, 136, 0.04));
          padding: 10px;
          margin-bottom: 10px;
          display: grid;
          grid-template-columns: repeat(4, minmax(0, 1fr));
          gap: 10px;
        }

        .control-item {
          display: flex;
          flex-direction: column;
          gap: 4px;
        }

        .control-item label {
          font-size: 9px;
          color: var(--t2);
          text-transform: uppercase;
          letter-spacing: 1px;
        }

        .control-item select,
        .control-item input {
          border: 1px solid var(--b1);
          background: #ffffff;
          color: var(--t);
          border-radius: 7px;
          height: 34px;
          padding: 0 9px;
          font-size: 12px;
        }

        .control-item select:focus,
        .control-item input:focus {
          outline: none;
          border-color: var(--ac);
          box-shadow: 0 0 0 2px rgba(0, 212, 255, 0.14);
        }

        .control-action {
          display: flex;
          align-items: end;
          justify-content: flex-end;
        }

        .mc {
          border: 1px solid var(--b2);
          border-radius: 8px;
          padding: 9px;
          background: var(--panel);
          text-align: center;
        }

        .mc.cr { border-color: rgba(255, 43, 79, 0.45); background: rgba(255, 43, 79, 0.08); }
        .mc.wn { border-color: rgba(255, 191, 0, 0.45); background: rgba(255, 191, 0, 0.08); }
        .mc.sf { border-color: rgba(0, 230, 122, 0.35); background: rgba(0, 230, 122, 0.06); }
        .mc.pr { border-color: rgba(185, 128, 255, 0.5); background: rgba(185, 128, 255, 0.09); }

        .ml { font-size: 9px; color: var(--t2); margin-bottom: 4px; }
        .mv { font-family: 'Space Mono', monospace; font-size: 13px; font-weight: 700; }
        .mc2 { font-size: 8px; color: var(--t2); margin-top: 3px; }

        .main3 {
          display: grid;
          grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 360px;
          gap: 12px;
          margin-top: 12px;
        }

        .panel {
          border: 1px solid var(--b2);
          border-radius: 12px;
          background: var(--panel);
          overflow: hidden;
        }

        .ph {
          padding: 10px 14px;
          background: var(--panel2);
          border-bottom: 1px solid var(--b2);
          display: flex;
          align-items: center;
          justify-content: space-between;
        }

        .pht {
          font-size: 12px;
          font-weight: 700;
          color: var(--ac);
        }

        .bdg {
          padding: 2px 8px;
          border-radius: 999px;
          font-family: 'Space Mono', monospace;
          font-size: 9px;
          font-weight: 700;
          border: 1px solid;
        }

        .bn { background: rgba(0, 212, 255, 0.14); border-color: rgba(0, 212, 255, 0.35); color: var(--ac); }
        .bw { background: rgba(255, 191, 0, 0.14); border-color: rgba(255, 191, 0, 0.35); color: var(--w); }
        .br { background: rgba(255, 43, 79, 0.14); border-color: rgba(255, 43, 79, 0.35); color: var(--r); }
        .bpu { background: rgba(185, 128, 255, 0.15); border-color: rgba(185, 128, 255, 0.4); color: var(--purple); }

        .scenario-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 8px;
          padding: 12px;
        }

        .scenario-card {
          border-radius: 8px;
          border: 1px solid;
          padding: 10px;
        }

        .scenario-card.sca { border-color: rgba(0, 230, 122, 0.35); background: rgba(0, 230, 122, 0.08); }
        .scenario-card.scb { border-color: rgba(255, 191, 0, 0.35); background: rgba(255, 191, 0, 0.08); }
        .scenario-card.scc { border-color: rgba(255, 43, 79, 0.35); background: rgba(255, 43, 79, 0.08); }

        .sp {
          font-family: 'Space Mono', monospace;
          font-size: 18px;
          font-weight: 700;
          margin-bottom: 4px;
        }

        .sn { font-size: 12px; font-weight: 700; margin-bottom: 3px; }
        .sd { font-size: 10px; color: var(--t2); line-height: 1.5; }
        .sc-i { font-size: 10px; color: var(--t3); margin-top: 6px; }

        .verdict-box {
          margin: 12px;
          padding: 12px;
          border-radius: 8px;
          border: 1px solid;
        }

        .verdict-box.green { border-color: rgba(0, 230, 122, 0.4); background: rgba(0, 230, 122, 0.08); }
        .verdict-box.amber { border-color: rgba(255, 191, 0, 0.45); background: rgba(255, 191, 0, 0.08); }
        .verdict-box.red { border-color: rgba(255, 43, 79, 0.45); background: rgba(255, 43, 79, 0.1); }

        .verdict-summary {
          font-size: 11px;
          font-weight: 700;
          color: var(--t);
          margin-bottom: 8px;
        }

        .verdict-cond {
          font-size: 10px;
          color: var(--t2);
          line-height: 1.6;
        }

        .pros-cons {
          display: grid;
          grid-template-columns: repeat(2, minmax(0, 1fr));
          gap: 0;
          border-top: 1px solid var(--b2);
        }

        .pc-box {
          padding: 12px;
        }

        .pc-box:first-child {
          border-right: 1px solid var(--b2);
        }

        .pc-head {
          font-size: 10px;
          font-weight: 700;
          margin-bottom: 8px;
        }

        .pc-body {
          font-size: 10px;
          color: var(--t2);
          line-height: 1.6;
        }

        .qgrid {
          display: grid;
          grid-template-columns: repeat(2, minmax(0, 1fr));
          gap: 8px;
          padding: 12px;
        }

        .qi {
          background: #ffffff;
          border: 1px solid var(--b1);
          border-radius: 8px;
          padding: 10px;
        }

        .qi-l {
          font-size: 9px;
          color: var(--t2);
          text-transform: uppercase;
          margin-bottom: 6px;
          letter-spacing: 1px;
        }

        .qi-v {
          font-family: 'Space Mono', monospace;
          font-size: 18px;
          font-weight: 700;
          line-height: 1;
        }

        .qi-bar {
          margin-top: 7px;
          width: 100%;
          height: 4px;
          border-radius: 999px;
          background: #e2e8f0;
          overflow: hidden;
        }

        .qi-fill {
          height: 100%;
          border-radius: 999px;
          transition: width 0.5s ease;
        }

        .qi-sub {
          margin-top: 4px;
          font-size: 9px;
          color: var(--t2);
        }

        .st {
          width: 100%;
          border-collapse: collapse;
          font-size: 11px;
        }

        .st th {
          background: #f1f5f9;
          color: var(--t2);
          padding: 7px 10px;
          text-align: left;
          font-size: 8px;
          letter-spacing: 1px;
          text-transform: uppercase;
          border-bottom: 1px solid var(--b2);
        }

        .st td {
          padding: 9px 10px;
          border-bottom: 1px solid var(--b2);
        }

        .st tr:hover td { background: rgba(8, 145, 178, 0.06); }
        .st tr.click-row { cursor: pointer; }
        .st tr.click-row:focus-visible td {
          outline: 2px solid var(--ac);
          outline-offset: -2px;
          background: rgba(8, 145, 178, 0.08);
        }

        .sn2 { font-size: 12px; font-weight: 700; }
        .sc2 { font-size: 9px; color: var(--t2); }

        .sig {
          font-size: 9px;
          border-radius: 4px;
          padding: 2px 6px;
          border: 1px solid;
          font-family: 'Space Mono', monospace;
          font-weight: 700;
        }

        .sb { background: rgba(0, 230, 122, 0.14); color: var(--g); border-color: rgba(0, 230, 122, 0.35); }
        .sw { background: rgba(0, 212, 255, 0.12); color: var(--ac); border-color: rgba(0, 212, 255, 0.35); }
        .sh { background: rgba(255, 191, 0, 0.12); color: var(--w); border-color: rgba(255, 191, 0, 0.35); }
        .ss { background: rgba(255, 43, 79, 0.14); color: var(--r); border-color: rgba(255, 43, 79, 0.35); }

        .pv { font-family: 'Space Mono', monospace; font-size: 12px; font-weight: 700; }
        .wb {
          margin-top: 4px;
          width: 100%;
          height: 4px;
          border-radius: 999px;
          background: #e2e8f0;
          overflow: hidden;
        }

        .wf {
          height: 100%;
          border-radius: 999px;
          background: linear-gradient(90deg, var(--ac), var(--g));
          transition: width 0.4s ease;
        }

        .empty-row { color: var(--t2); text-align: center; }

        .rcol { display: flex; flex-direction: column; gap: 12px; }

        .rw { padding: 12px; }
        .rl {
          width: 100%;
          height: 14px;
          border-radius: 7px;
          background: linear-gradient(90deg, #00e67a, #ffbf00, #ff2b4f);
          position: relative;
          margin-bottom: 7px;
        }

        .rn {
          position: absolute;
          top: -5px;
          width: 4px;
          height: 24px;
          background: #0f172a;
          border-radius: 2px;
          transform: translateX(-50%);
          box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
          transition: left 0.8s ease;
        }

        .rls {
          display: flex;
          justify-content: space-between;
          font-size: 8px;
          color: var(--t2);
          margin-bottom: 8px;
        }

        .rs {
          text-align: center;
          font-family: 'Space Mono', monospace;
          font-size: 38px;
          font-weight: 700;
        }

        .rsl {
          text-align: center;
          font-size: 11px;
          margin-bottom: 10px;
          font-weight: 700;
        }

        .rrows { border-top: 1px solid var(--b2); padding-top: 9px; }

        .rrow {
          display: flex;
          align-items: center;
          gap: 7px;
          margin-bottom: 7px;
        }

        .rname { width: 90px; font-size: 9px; color: var(--t3); }
        .rbg {
          flex: 1;
          height: 3px;
          background: #e2e8f0;
          border-radius: 2px;
          overflow: hidden;
        }

        .rfill { height: 100%; border-radius: 2px; }
        .rpct { width: 52px; text-align: right; font-size: 8px; font-family: 'Space Mono', monospace; }

        .alist {
          max-height: 280px;
          overflow-y: auto;
        }

        .ali {
          display: flex;
          gap: 9px;
          padding: 10px 12px;
          border-bottom: 1px solid var(--b2);
        }

        .adot {
          width: 8px;
          height: 8px;
          margin-top: 3px;
          border-radius: 50%;
          flex-shrink: 0;
        }

        .dr { background: var(--r); box-shadow: 0 0 8px var(--r); }
        .dw { background: var(--w); box-shadow: 0 0 8px var(--w); }
        .dg { background: var(--g); box-shadow: 0 0 8px var(--g); }
        .db { background: var(--ac); box-shadow: 0 0 8px var(--ac); }

        .at2 { font-size: 11px; font-weight: 700; margin-bottom: 2px; }
        .ad2 { font-size: 9px; color: var(--t2); line-height: 1.5; }
        .atime { font-size: 8px; color: var(--t2); margin-top: 2px; font-family: 'Space Mono', monospace; }
        .empty-alert { color: var(--t2); font-size: 10px; padding: 12px; }

        .cdown { text-align: center; padding: 14px; }
        .cnum { font-family: 'Space Mono', monospace; font-size: 38px; font-weight: 700; color: var(--w); }
        .clbl { font-size: 11px; color: var(--t2); margin-top: 4px; }

        .botrow {
          display: grid;
          grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
          gap: 12px;
          margin-top: 12px;
        }

        .sbox,
        .abox {
          padding: 12px;
        }

        .si {
          display: flex;
          gap: 9px;
          padding: 8px 0;
          border-bottom: 1px solid var(--b2);
        }

        .si:last-child { border-bottom: none; }

        .snum {
          width: 20px;
          height: 20px;
          border-radius: 50%;
          border: 1px solid rgba(0, 212, 255, 0.35);
          background: rgba(0, 212, 255, 0.14);
          color: var(--ac);
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 10px;
          font-family: 'Space Mono', monospace;
          flex-shrink: 0;
        }

        .stit { font-size: 11px; font-weight: 700; margin-bottom: 2px; }
        .sdesc { font-size: 10px; color: var(--t2); line-height: 1.5; }

        .ai {
          display: flex;
          gap: 8px;
          padding: 8px 0;
          border-bottom: 1px solid var(--b2);
        }

        .ai:last-child { border-bottom: none; }

        .al {
          width: 130px;
          font-size: 10px;
          color: var(--t3);
          flex-shrink: 0;
        }

        .av {
          font-size: 10px;
          color: var(--t2);
          line-height: 1.45;
        }

        .error-box {
          margin-top: 12px;
          border: 1px solid rgba(220, 38, 38, 0.5);
          background: rgba(220, 38, 38, 0.08);
          color: #b91c1c;
          padding: 10px 12px;
          border-radius: 8px;
          font-size: 12px;
        }

        .dp {
          animation: datapulse 0.55s ease;
        }

        @keyframes datapulse {
          0% {
            filter: brightness(1.6);
          }
          100% {
            filter: brightness(1);
          }
        }

        .up { color: var(--g); }
        .dn { color: var(--r); }
        .warn { color: var(--w); }
        .bl { color: var(--ac); }
        .gol { color: var(--gold); }
        .pu { color: var(--purple); }

        .mt14 { margin-top: 12px; }
        .mt12 { margin-top: 12px; }

        @media (max-width: 1320px) {
          .cbar { grid-template-columns: 32px 1fr; }
          .cbar-verdict { grid-column: span 2; }
          .pros-cons { grid-template-columns: 1fr; }
          .pc-box:first-child { border-right: none; border-bottom: 1px solid var(--b2); }
          .control-bar { grid-template-columns: repeat(2, minmax(0, 1fr)); }
          .mgrid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
          .main3 { grid-template-columns: 1fr; }
          .botrow { grid-template-columns: 1fr; }
          .rcol { order: 3; }
        }

        @media (max-width: 760px) {
          .cbar { grid-template-columns: 1fr; }
          .cbar-icon { display: none; }
          .mgrid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
          .control-bar { grid-template-columns: 1fr; }
          .control-action { justify-content: stretch; }
          .control-action .ubtn { width: 100%; }
          .hrow {
            flex-direction: column;
            align-items: flex-start;
          }

          .hstats {
            width: 100%;
            justify-content: space-between;
          }

          .ubar {
            top: 122px;
          }

          .qgrid { grid-template-columns: 1fr; }
          .al { width: 102px; }
        }
      `}</style>
    </div>
  );
}
