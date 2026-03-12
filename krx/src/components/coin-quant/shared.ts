export type CoinQuantItem = {
  rank: number;
  symbol: string;
  price: number;
  candleTime: number;
  return_1h: number;
  return_24h: number;
  volatility_24h: number;
  drawdown_24h: number;
  trade_value_24h: number;
  trade_value_ratio_24h: number;
  momentum_score: number;
  liquidity_score: number;
  stability_score: number;
  trend_score: number;
  breakout_score: number;
  total_score: number;
};

export type CoinQuantResponse = {
  generated_at: string;
  asOf: number;
  interval: string;
  limit: number;
  universe_count: number;
  min_trade_value_24h: number;
  breadth_24h: number;
  average?: {
    return_24h?: number;
    volatility_24h?: number;
    trade_value_24h?: number;
  };
  items: CoinQuantItem[];
};

export type CoinCrisisLevel = "안정" | "주의" | "위험" | "극위험";
export type CoinAlertTone = "danger" | "warn" | "safe" | "info";
export type CoinScenarioTone = "positive" | "neutral" | "negative";

export type CoinScenario = {
  title: string;
  probability: number;
  description: string;
  implication: string;
  tone: CoinScenarioTone;
};

export type CoinAlertItem = {
  title: string;
  detail: string;
  tone: CoinAlertTone;
};

export type LiveTicker = {
  price?: number;
  return24H?: number;
  tradeValue24H?: number;
  timestamp?: number;
};

export type CoinDerived = {
  avg1h: number;
  avg24h: number;
  avgVol24h: number;
  avgTradeValue: number;
  avgLiquidity: number;
  avgMomentum: number;
  breadth: number;
  topScore: CoinQuantItem | null;
  topWinner: CoinQuantItem | null;
  topFlow: CoinQuantItem | null;
  riskScore: number;
  riskLevel: CoinCrisisLevel;
  scenarios: CoinScenario[];
};

const KOREAN_NUMBER_FORMATTER = new Intl.NumberFormat("ko-KR");
const KOREAN_PRICE_FORMATTER = new Intl.NumberFormat("ko-KR", { maximumFractionDigits: 0 });
const KOREAN_COMPACT_FORMATTER = new Intl.NumberFormat("ko-KR", {
  notation: "compact",
  maximumFractionDigits: 1,
});
const KST_DATE_TIME_FORMATTER = new Intl.DateTimeFormat("ko-KR", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "Asia/Seoul",
});

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function safeNumber(value?: number): number {
  return Number.isFinite(value) ? (value ?? 0) : 0;
}

export function formatSignedPct(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  const numeric = value ?? 0;
  return `${numeric > 0 ? "+" : ""}${numeric.toFixed(2)}%`;
}

export function formatPct(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${(value ?? 0).toFixed(2)}%`;
}

export function formatPrice(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  return `${KOREAN_PRICE_FORMATTER.format(value ?? 0)}원`;
}

export function formatTradeValue(value?: number): string {
  if (!Number.isFinite(value) || (value ?? 0) <= 0) return "-";
  const raw = value ?? 0;
  if (raw >= 1_0000_0000_0000) return `${(raw / 1_0000_0000_0000).toFixed(2)}조`;
  if (raw >= 1_0000_0000) return `${(raw / 1_0000_0000).toFixed(0)}억`;
  return KOREAN_COMPACT_FORMATTER.format(raw);
}

export function formatRatioX(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  return `x${(value ?? 0).toFixed(2)}`;
}

export function formatFixed(value?: number, digits = 2): string {
  if (!Number.isFinite(value)) return "-";
  return (value ?? 0).toFixed(digits);
}

export function formatNumber(value?: number): string {
  if (!Number.isFinite(value)) return "-";
  return KOREAN_NUMBER_FORMATTER.format(value ?? 0);
}

export function formatDateTime(value?: number | string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return KST_DATE_TIME_FORMATTER.format(date);
}

export function riskLevel(score: number): CoinCrisisLevel {
  if (score < 30) return "안정";
  if (score < 50) return "주의";
  if (score < 70) return "위험";
  return "극위험";
}

export function toneFromRisk(level: CoinCrisisLevel): "emerald" | "cyan" | "amber" | "rose" {
  if (level === "안정") return "emerald";
  if (level === "주의") return "cyan";
  if (level === "위험") return "amber";
  return "rose";
}

export function toneFromAlert(tone: CoinAlertTone): "rose" | "amber" | "emerald" | "cyan" {
  if (tone === "danger") return "rose";
  if (tone === "warn") return "amber";
  if (tone === "safe") return "emerald";
  return "cyan";
}

export function toneFromScenario(tone: CoinScenarioTone): "emerald" | "amber" | "rose" {
  if (tone === "positive") return "emerald";
  if (tone === "neutral") return "amber";
  return "rose";
}

export function buildCoinSignal(item: CoinQuantItem): { label: string; tone: "emerald" | "amber" | "cyan" | "rose" } {
  const totalScore = safeNumber(item.total_score);
  const return24h = safeNumber(item.return_24h);
  const tradeValueRatio24H = safeNumber(item.trade_value_ratio_24h);

  if (totalScore >= 75 && return24h > 0 && tradeValueRatio24H >= 1.2) {
    return { label: "공격 가능", tone: "emerald" };
  }
  if (totalScore >= 65 && return24h > 0) {
    return { label: "우선 감시", tone: "cyan" };
  }
  if (totalScore >= 58) {
    return { label: "관찰 유지", tone: "amber" };
  }
  return { label: "보류", tone: "rose" };
}

export function weightFromScore(score: number, rank: number): number {
  const base = score >= 75 ? 16 : score >= 68 ? 12 : score >= 60 ? 8 : 5;
  const rankPenalty = Math.min((rank - 1) * 0.7, 4.5);
  return clamp(base - rankPenalty, 3, 18);
}

export function buildCoinScenarios(
  riskScore: number,
  avg24h: number,
  breadth: number
): CoinScenario[] {
  if (riskScore >= 65) {
    return [
      {
        title: "방어 우위",
        probability: 52,
        description: "breadth 둔화와 변동성 상승이 겹쳐 눌림 재개 가능성이 큽니다.",
        implication: "거래대금이 붙는 메이저만 소액 관찰하는 접근이 유효합니다.",
        tone: "negative",
      },
      {
        title: "선별 반등",
        probability: 31,
        description: "BTC 등 상위 유동성 코인만 지지받고 알트 확산은 제한될 수 있습니다.",
        implication: "랭킹 상위이면서 거래대금 ratio가 높은 종목만 남기는 편이 안전합니다.",
        tone: "neutral",
      },
      {
        title: "광범위 회복",
        probability: 17,
        description: "breadth가 재확대되면 숏 커버성 회복이 나올 수 있습니다.",
        implication: "24H breadth가 55%를 넘길 때까지는 보수적으로 봐야 합니다.",
        tone: "positive",
      },
    ];
  }

  if (avg24h >= 0 && breadth >= 55) {
    return [
      {
        title: "추세 지속",
        probability: 48,
        description: "거래대금이 붙는 코인 중심의 추세 지속 가능성이 높습니다.",
        implication: "총점 상위와 거래대금 ratio 상위가 겹치는 코인을 우선 봅니다.",
        tone: "positive",
      },
      {
        title: "순환 장세",
        probability: 34,
        description: "메이저와 알트 간 리더 교체가 빠르게 발생할 수 있습니다.",
        implication: "1H 반응보다 24H 흐름과 자금 유입을 같이 봐야 합니다.",
        tone: "neutral",
      },
      {
        title: "과열 되돌림",
        probability: 18,
        description: "단기 급등 코인은 1시간 눌림이 먼저 나올 수 있습니다.",
        implication: "24H 급등폭이 큰 종목은 추격 비중을 줄이는 편이 낫습니다.",
        tone: "negative",
      },
    ];
  }

  return [
    {
      title: "박스 순환",
      probability: 44,
      description: "방향성보다 순환이 우세해 상단 랭킹이 자주 바뀔 수 있습니다.",
      implication: "고정 보유보다 짧은 점검 주기가 중요합니다.",
      tone: "neutral",
    },
    {
      title: "상방 재확산",
      probability: 30,
      description: "breadth가 소폭 개선되면 모멘텀 상위 코인이 다시 리드할 수 있습니다.",
      implication: "24H 거래대금 증가율이 높은 코인을 먼저 점검합니다.",
      tone: "positive",
    },
    {
      title: "하방 압박",
      probability: 26,
      description: "변동성이 다시 커지면 랭킹 상단도 빠르게 밀릴 수 있습니다.",
      implication: "drawdown과 변동성이 큰 코인은 제외하는 편이 안전합니다.",
      tone: "negative",
    },
  ];
}

export function deriveCoinMarket(rows: CoinQuantItem[], breadth24h = 0): CoinDerived {
  const avg1h = rows.length ? rows.reduce((sum, item) => sum + safeNumber(item.return_1h), 0) / rows.length : 0;
  const avg24h = rows.length ? rows.reduce((sum, item) => sum + safeNumber(item.return_24h), 0) / rows.length : 0;
  const avgVol24h = rows.length
    ? rows.reduce((sum, item) => sum + safeNumber(item.volatility_24h), 0) / rows.length
    : 0;
  const avgTradeValue = rows.length
    ? rows.reduce((sum, item) => sum + safeNumber(item.trade_value_24h), 0) / rows.length
    : 0;
  const avgLiquidity = rows.length
    ? rows.reduce((sum, item) => sum + safeNumber(item.liquidity_score), 0) / rows.length
    : 0;
  const avgMomentum = rows.length
    ? rows.reduce((sum, item) => sum + safeNumber(item.momentum_score), 0) / rows.length
    : 0;
  const breadth = breadth24h ?? 0;
  const riskScore = clamp(
    (avgVol24h / 8) * 38 + Math.max(0, 55 - breadth) * 1.1 + Math.max(0, -avg24h) * 4.5,
    0,
    100
  );

  return {
    avg1h,
    avg24h,
    avgVol24h,
    avgTradeValue,
    avgLiquidity,
    avgMomentum,
    breadth,
    topScore: rows[0] ?? null,
    topWinner: [...rows].sort((a, b) => safeNumber(b.return_24h) - safeNumber(a.return_24h))[0] ?? null,
    topFlow:
      [...rows].sort(
        (a, b) => safeNumber(b.trade_value_ratio_24h) - safeNumber(a.trade_value_ratio_24h)
      )[0] ?? null,
    riskScore,
    riskLevel: riskLevel(riskScore),
    scenarios: buildCoinScenarios(riskScore, avg24h, breadth),
  };
}
