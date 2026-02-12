import {
  MOMENTUM_THRESHOLD,
  TURNOVER_SPIKE_THRESHOLD,
  TOP_CAP_RATIO_THRESHOLD,
} from "@/constants/api";
import { THRESHOLDS, UI_LABELS, WEIGHTS } from "@/constants/ui";
import type { Group } from "@/types";

export function toMomentumLabel(score: number): string {
  if (score >= THRESHOLDS.MOMENTUM_STRONG) return UI_LABELS.MOMENTUM.STRONG;
  if (score >= THRESHOLDS.MOMENTUM_RISING) return UI_LABELS.MOMENTUM.RISING;
  if (score >= THRESHOLDS.MOMENTUM_INTEREST) return UI_LABELS.MOMENTUM.INTEREST;
  return UI_LABELS.MOMENTUM.WATCHING;
}

export function normalize(value: number, min: number, max: number): number {
  if (max === min) return THRESHOLDS.DEFAULT_SCORE;
  return clamp(
    (value - min) / (max - min),
    THRESHOLDS.MIN_NORMALIZED,
    THRESHOLDS.MAX_NORMALIZED,
  );
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function calculateShortAlert(group: Group): boolean {
  return (
    group.change1d >= THRESHOLDS.SHORT_ALERT_CHANGE1D &&
    group.change5d >= THRESHOLDS.SHORT_ALERT_CHANGE5D &&
    group.turnoverSpike >= TURNOVER_SPIKE_THRESHOLD &&
    group.topCapRatio >= TOP_CAP_RATIO_THRESHOLD
  );
}

export function calculateMomentumScore(
  group: Group,
  rs5d: number,
  rs20d: number,
  turnoverScore: number,
  newsScore: number = THRESHOLDS.DEFAULT_SCORE,
  newsWeight: number = 0,
): number {
  const baseScore =
    WEIGHTS.RS5D * rs5d +
    WEIGHTS.RS20D * rs20d +
    WEIGHTS.TURNOVER * turnoverScore +
    WEIGHTS.BREADTH * group.breadthRatio +
    WEIGHTS.TOP_CAP * group.topCapRatio +
    WEIGHTS.FLOW * group.flowScore;

  let score = Math.round(
    THRESHOLDS.MAX_SCORE *
      clamp(
        (1 - newsWeight) * baseScore + newsWeight * newsScore,
        THRESHOLDS.MIN_NORMALIZED,
        THRESHOLDS.MAX_NORMALIZED,
      ),
  );

  // 보너스 점수
  if (
    group.turnoverSpike >= TURNOVER_SPIKE_THRESHOLD &&
    group.topCapRatio >= TOP_CAP_RATIO_THRESHOLD
  ) {
    score = Math.min(THRESHOLDS.MAX_SCORE, score + THRESHOLDS.BONUS_SCORE);
  }

  return score;
}

export function filterByMomentumThreshold(
  groups: Array<{ momentumScore: number }>,
  threshold: number = MOMENTUM_THRESHOLD,
): Array<{ momentumScore: number }> {
  return groups.filter((group) => group.momentumScore >= threshold);
}
