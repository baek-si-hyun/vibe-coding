export type MarketFilter = "ALL" | "KOSPI" | "KOSDAQ";

export type QuantItem = {
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
  next_day_score?: number;
  momentum_score: number;
  liquidity_score: number;
  stability_score: number;
  trend_score?: number;
  risk_adjusted_score?: number;
  lstm_score?: number;
  lstm_pred_return_1d?: number;
  lstm_pred_return_5d?: number;
  lstm_pred_return_20d?: number;
  lstm_prob_up?: number;
  lstm_confidence?: number;
  total_score: number;
};

export type RankResponse = {
  generated_at: string;
  score_model?: string;
  market: string;
  min_market_cap: number;
  limit: number;
  universe_count: number;
  as_of_min: string;
  as_of_max: string;
  lstm_enabled?: boolean;
  lstm_model_version?: string;
  lstm_weight?: number;
  lstm_prediction_as_of?: string;
  lstm_prediction_count?: number;
  lstm_applied_count?: number;
  items: QuantItem[];
};

export type ReportResponse = {
  generated_at?: string;
  score_model?: string;
  market?: string;
  min_market_cap?: number;
  limit?: number;
  universe_count?: number;
  as_of_min?: string;
  as_of_max?: string;
  lstm_enabled?: boolean;
  lstm_model_version?: string;
  lstm_weight?: number;
  lstm_prediction_as_of?: string;
  lstm_prediction_count?: number;
  lstm_applied_count?: number;
  items?: QuantItem[];
  report_markdown?: string;
};

export type MacroMetric = {
  label: string;
  value?: number;
  display: string;
  change_percent?: number;
  as_of?: string;
  provider?: string;
  status: string;
  note?: string;
};

export type MacroHeadline = {
  title: string;
  url?: string;
  source?: string;
  published_at?: string;
  keywords?: string[];
};

export type CrisisLevel = "안정" | "주의" | "위험" | "극위험";

export type MacroGeopolitical = {
  score: number;
  level: CrisisLevel;
  matched_keywords?: string[];
  headlines?: MacroHeadline[];
  providers?: string[];
  note?: string;
};

export type MacroResponse = {
  generated_at: string;
  metrics?: Record<string, MacroMetric>;
  geopolitical?: MacroGeopolitical;
  warnings?: string[];
};

export type ScenarioTone = "positive" | "neutral" | "negative";
export type AlertTone = "danger" | "warn" | "safe" | "info";
export type VerdictTone = "green" | "amber" | "red";
export type QuantSignalTone = "emerald" | "cyan" | "amber" | "rose" | "slate";

export type Scenario = {
  title: string;
  probability: number;
  description: string;
  implication: string;
  tone: ScenarioTone;
};

export type TacticalVerdict = {
  title: string;
  summary: string;
  tone: VerdictTone;
  pros: string[];
  cons: string[];
  conditions: string[];
};

export type QuantSignal = {
  label: string;
  tone: QuantSignalTone;
};

export type LSTMMeta = {
  enabled: boolean;
  modelVersion: string;
  weightPct: number;
  predictionAsOf: string;
  predictionCount: number;
  appliedCount: number;
};

export type QuantDerived = {
  avg1d: number;
  avg20d: number;
  avg60d: number;
  avg120d: number;
  avgVol20: number;
  avgTurnoverRatio: number;
  avgLSTMScore: number;
  avgLSTMPred1D: number;
  avgLSTMConfidence: number;
  visibleLSTMAppliedCount: number;
  kospiCount: number;
  kosdaqCount: number;
  positive20Ratio: number;
  topWinner: QuantItem | null;
  topLoser: QuantItem | null;
  topScore: QuantItem | null;
  riskScore: number;
  riskLevel: CrisisLevel;
  scenarios: Scenario[];
};

const KOREAN_NUMBER_FORMATTER = new Intl.NumberFormat("ko-KR");
const KOREAN_PRICE_FORMATTER = new Intl.NumberFormat("ko-KR", { maximumFractionDigits: 0 });
const KST_DATE_TIME_FORMATTER = new Intl.DateTimeFormat("ko-KR", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "Asia/Seoul",
});
const KST_CLOCK_FORMATTER = new Intl.DateTimeFormat("ko-KR", {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
  timeZone: "Asia/Seoul",
});

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function mean(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((acc, value) => acc + value, 0) / values.length;
}

export function median(values: number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
}

export function sanitizeMarket(raw: string): MarketFilter {
  const value = raw.trim().toUpperCase();
  if (value === "KOSPI" || value === "KOSDAQ" || value === "ALL") return value;
  return "ALL";
}

export function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return KOREAN_NUMBER_FORMATTER.format(value);
}

export function formatSignedPct(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${value > 0 ? "+" : ""}${value.toFixed(2)}%`;
}

export function formatPct(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${value.toFixed(2)}%`;
}

export function formatOptionalPct(value?: number): string {
  return Number.isFinite(value) ? formatPct(value ?? 0) : "-";
}

export function formatOptionalSignedPct(value?: number): string {
  return Number.isFinite(value) ? formatSignedPct(value ?? 0) : "-";
}

export function formatMarketCap(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  if (value >= 1_0000_0000_0000) return `${(value / 1_0000_0000_0000).toFixed(2)}조`;
  if (value >= 1_0000_0000) return `${(value / 1_0000_0000).toFixed(0)}억`;
  return formatNumber(value);
}

export function formatPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return `${KOREAN_PRICE_FORMATTER.format(Math.round(value))}원`;
}

export function formatDateTime(raw: string): string {
  if (!raw) return "-";
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  return KST_DATE_TIME_FORMATTER.format(date);
}

export function formatDateCompact(raw: string): string {
  const value = raw.trim();
  if (value.length !== 8) return value || "-";
  return `${value.slice(0, 4)}.${value.slice(4, 6)}.${value.slice(6, 8)}`;
}

export function formatChangeText(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${(value ?? 0) > 0 ? "+" : ""}${(value ?? 0).toFixed(2)}%`;
}

export function formatClock(date: Date): string {
  return KST_CLOCK_FORMATTER.format(date);
}

export function toneFromRisk(level: CrisisLevel): "emerald" | "cyan" | "amber" | "rose" {
  if (level === "안정") return "emerald";
  if (level === "주의") return "cyan";
  if (level === "위험") return "amber";
  return "rose";
}

export function toneFromScenario(tone: ScenarioTone): "emerald" | "amber" | "rose" {
  if (tone === "positive") return "emerald";
  if (tone === "neutral") return "amber";
  return "rose";
}

export function toneFromAlert(tone: AlertTone): "rose" | "amber" | "emerald" | "cyan" {
  if (tone === "danger") return "rose";
  if (tone === "warn") return "amber";
  if (tone === "safe") return "emerald";
  return "cyan";
}

export function toneFromVerdict(tone: VerdictTone): "emerald" | "amber" | "rose" {
  if (tone === "green") return "emerald";
  if (tone === "amber") return "amber";
  return "rose";
}

export function toneFromMacroMetric(
  key: string,
  metric?: MacroMetric
): "slate" | "cyan" | "amber" | "rose" {
  if (!metric || metric.status !== "ok") return "slate";
  const change = metric.change_percent ?? 0;
  switch (key) {
    case "wti":
    case "vix":
    case "usdkrw":
    case "us10y":
      return change > 0 ? "rose" : "cyan";
    case "gold":
      return change > 0 ? "amber" : "slate";
    default:
      return "slate";
  }
}

export function weightFromScore(score: number, rank: number): number {
  const scoreWeight = clamp((score - 55) * 0.35, 0, 15);
  const rankPenalty = clamp((rank - 1) * 0.4, 0, 7);
  return clamp(scoreWeight - rankPenalty + 3, 1, 15);
}

export function parseTopMarkdown(reportMarkdown: string, count: number): string[] {
  if (!reportMarkdown) return [];
  const lines = reportMarkdown.split("\n");
  const rows = lines.filter((line) => /^\|\d+\|/.test(line.trim()));
  return rows.slice(0, count).map((line) => {
    const cols = line.split("|").map((cell) => cell.trim());
    return `${cols[2] || "-"} · 총점 ${cols[5] || "-"}`;
  });
}

export function buildQuantSignal(item: QuantItem): QuantSignal {
  if (item.total_score >= 85) return { label: "STRONG BUY", tone: "emerald" };
  if (item.total_score >= 75) return { label: "BUY", tone: "cyan" };
  if (item.total_score >= 65) return { label: "WATCH", tone: "amber" };
  if (item.return_20d < -10 || item.total_score < 45) return { label: "REDUCE", tone: "rose" };
  return { label: "HOLD", tone: "slate" };
}

export function computeCrisisScore(items: QuantItem[]): number {
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

export function crisisLevel(score: number): CrisisLevel {
  if (score >= 80) return "극위험";
  if (score >= 60) return "위험";
  if (score >= 40) return "주의";
  return "안정";
}

export function scenarioSet(score: number): Scenario[] {
  const rebound = clamp(Math.round((100 - score) * 0.45), 10, 55);
  const tail = clamp(Math.round(score * 0.42), 12, 60);
  const base = clamp(100 - rebound - tail, 20, 70);
  const normalizeGap = 100 - rebound - tail - base;

  return [
    {
      title: "기술적 반등",
      probability: rebound + (normalizeGap > 0 ? normalizeGap : 0),
      description: "과매도 해소와 거래대금 회복이 동반될 때",
      implication: "낙폭과대보다는 총점 상위 종목만 선별하는 편이 안전합니다.",
      tone: "positive",
    },
    {
      title: "박스권 소화",
      probability: base,
      description: "상승과 하락 신호가 혼재된 중립 구간",
      implication: "리포트 상단 종목만 제한적으로 보는 것이 맞습니다.",
      tone: "neutral",
    },
    {
      title: "하방 재개",
      probability: tail,
      description: "리스크 심리 확대로 하락 압력이 다시 강해질 때",
      implication: "신규 비중보다 방어와 손절 규칙 유지가 우선입니다.",
      tone: "negative",
    },
  ];
}

export function buildLSTMMeta(source?: ReportResponse | RankResponse | null): LSTMMeta {
  return {
    enabled: Boolean(source?.lstm_enabled),
    modelVersion: source?.lstm_model_version || "",
    weightPct: (source?.lstm_weight ?? 0) * 100,
    predictionAsOf: source?.lstm_prediction_as_of || "",
    predictionCount: source?.lstm_prediction_count ?? 0,
    appliedCount: source?.lstm_applied_count ?? 0,
  };
}

export function deriveQuantRows(rows: QuantItem[]): QuantDerived {
  if (rows.length === 0) {
    return {
      avg1d: 0,
      avg20d: 0,
      avg60d: 0,
      avg120d: 0,
      avgVol20: 0,
      avgTurnoverRatio: 0,
      avgLSTMScore: 0,
      avgLSTMPred1D: 0,
      avgLSTMConfidence: 0,
      visibleLSTMAppliedCount: 0,
      kospiCount: 0,
      kosdaqCount: 0,
      positive20Ratio: 0,
      topWinner: null,
      topLoser: null,
      topScore: null,
      riskScore: 50,
      riskLevel: "주의",
      scenarios: scenarioSet(50),
    };
  }

  const avg1d = mean(rows.map((item) => item.return_1d));
  const avg20d = mean(rows.map((item) => item.return_20d));
  const avg60d = mean(rows.map((item) => item.return_60d));
  const avg120d = mean(rows.map((item) => item.return_120d));
  const avgVol20 = mean(rows.map((item) => item.volatility_20d));
  const avgTurnoverRatio = mean(rows.map((item) => item.turnover_ratio));
  const lstmRows = rows.filter((item) => Number.isFinite(item.lstm_score));
  const avgLSTMScore = mean(lstmRows.map((item) => item.lstm_score ?? 0));
  const avgLSTMPred1D = mean(
    lstmRows.map((item) => item.lstm_pred_return_1d ?? item.lstm_pred_return_5d ?? 0)
  );
  const avgLSTMConfidence = mean(lstmRows.map((item) => item.lstm_confidence ?? 0));
  const kospiCount = rows.filter((item) => item.market === "KOSPI").length;
  const kosdaqCount = rows.filter((item) => item.market === "KOSDAQ").length;
  const positive20Count = rows.filter((item) => item.return_20d > 0).length;
  const positive20Ratio = rows.length > 0 ? positive20Count / rows.length : 0;
  const sortedByDay = [...rows].sort((a, b) => b.return_1d - a.return_1d);
  const sortedByScore = [...rows].sort((a, b) => b.total_score - a.total_score);
  const riskScore = computeCrisisScore(rows);

  return {
    avg1d,
    avg20d,
    avg60d,
    avg120d,
    avgVol20,
    avgTurnoverRatio,
    avgLSTMScore,
    avgLSTMPred1D,
    avgLSTMConfidence,
    visibleLSTMAppliedCount: lstmRows.length,
    kospiCount,
    kosdaqCount,
    positive20Ratio,
    topWinner: sortedByDay[0] ?? null,
    topLoser: sortedByDay[sortedByDay.length - 1] ?? null,
    topScore: sortedByScore[0] ?? null,
    riskScore,
    riskLevel: crisisLevel(riskScore),
    scenarios: scenarioSet(riskScore),
  };
}

export function extractStockLine(markdown: string, code: string): string {
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

export function buildStrengths(item: QuantItem): string[] {
  const strengths: string[] = [];
  if (item.total_score >= 75) strengths.push(`총점 ${item.total_score.toFixed(2)}로 상위권입니다.`);
  if (item.return_20d > 0 && item.return_60d > 0) strengths.push("20일과 60일 수익률이 모두 플러스입니다.");
  if (item.momentum_score >= 70) strengths.push(`모멘텀 점수 ${item.momentum_score.toFixed(1)}로 추세 우위입니다.`);
  if (item.stability_score >= 60) strengths.push(`안정성 점수 ${item.stability_score.toFixed(1)}로 변동성 부담이 낮습니다.`);
  if (Number.isFinite(item.lstm_score) && (item.lstm_score ?? 0) >= 60) {
    strengths.push(`LSTM 보조점수 ${(item.lstm_score ?? 0).toFixed(1)}로 단기 신호가 우호적입니다.`);
  }
  if (Number.isFinite(item.lstm_pred_return_1d) && (item.lstm_pred_return_1d ?? 0) > 0) {
    strengths.push(`LSTM 다음날 예측 수익률 ${formatOptionalSignedPct(item.lstm_pred_return_1d)}입니다.`);
  }
  if (strengths.length === 0) strengths.push("점수는 중립 구간입니다. 추가 확인이 필요합니다.");
  return strengths.slice(0, 4);
}

export function buildRisks(item: QuantItem): string[] {
  const risks: string[] = [];
  if (item.return_1d <= -2) risks.push(`1일 수익률이 ${formatSignedPct(item.return_1d)}로 단기 충격이 있습니다.`);
  if (item.return_20d < 0) risks.push(`20일 수익률 ${formatSignedPct(item.return_20d)}로 중기 추세가 약합니다.`);
  if (item.volatility_20d >= 4.5) risks.push(`20일 변동성 ${formatPct(item.volatility_20d)}로 흔들림이 큽니다.`);
  if (item.liquidity_score < 45) risks.push(`유동성 점수 ${item.liquidity_score.toFixed(1)}로 체결 리스크가 있습니다.`);
  if (Number.isFinite(item.lstm_pred_return_1d) && (item.lstm_pred_return_1d ?? 0) < 0) {
    risks.push(`LSTM 다음날 예측 수익률이 ${formatOptionalSignedPct(item.lstm_pred_return_1d)}입니다.`);
  }
  if (Number.isFinite(item.lstm_confidence) && (item.lstm_confidence ?? 0) < 45) {
    risks.push(`LSTM 신뢰도 ${formatOptionalPct(item.lstm_confidence)}로 일관성이 높지 않습니다.`);
  }
  if (risks.length === 0) risks.push("지표상 큰 위험 신호는 제한적입니다. 손절 규칙만 유지하면 됩니다.");
  return risks.slice(0, 4);
}

export function buildNarrative(item: QuantItem, signal: QuantSignal, weight: number): string {
  const lstmSummary = Number.isFinite(item.lstm_score)
    ? ` LSTM 보조점수 ${(item.lstm_score ?? 0).toFixed(2)}, 다음날 예측 ${formatOptionalSignedPct(
        item.lstm_pred_return_1d ?? item.lstm_pred_return_5d
      )}, 상승확률 ${formatOptionalPct(item.lstm_prob_up)}가 함께 반영되었습니다.`
    : "";

  return `${item.name}(${item.code})는 ${signal.label} 구간이며 권장 비중은 ${weight.toFixed(
    1
  )}% 수준입니다. 20일 ${formatSignedPct(item.return_20d)}, 60일 ${formatSignedPct(
    item.return_60d
  )}, 변동성 ${formatPct(item.volatility_20d)} 기준으로 추세와 리스크를 동시에 관리해야 합니다.${lstmSummary}`;
}
