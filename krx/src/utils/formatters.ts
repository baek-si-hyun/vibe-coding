import { FORMATTING, UI_LABELS } from "@/constants/ui";

// Formatter instances - created once at module level for performance
const percentFormatter = new Intl.NumberFormat("ko-KR", {
  maximumFractionDigits: 1,
  minimumFractionDigits: 1,
});

const ratioFormatter = new Intl.NumberFormat("ko-KR", {
  maximumFractionDigits: 1,
});

const priceFormatter = new Intl.NumberFormat("ko-KR", {
  maximumFractionDigits: 0,
});

export const formatChange = (value: number): string => {
  const sign = value > 0 ? "+" : "";
  return `${sign}${percentFormatter.format(value)}%`;
};

export const formatMoney = (value: number): string => {
  const jo = Math.floor(value / FORMATTING.JO_DIVISOR);
  const eok = Math.floor((value % FORMATTING.JO_DIVISOR) / FORMATTING.EOK_DIVISOR);
  if (jo > 0) {
    return eok > 0 ? `${jo}조 ${eok}억` : `${jo}조`;
  }
  return `${Math.round(value / FORMATTING.EOK_DIVISOR)}억`;
};

export const formatTurnoverSpike = (value: number): string =>
  `x${ratioFormatter.format(value)}`;

export const formatRatioPercent = (value: number): string =>
  `${Math.round(value * FORMATTING.PERCENT_MULTIPLIER)}%`;

export const formatPrice = (value: number): string =>
  `${priceFormatter.format(value)}${UI_LABELS.UNITS.WON}`;
