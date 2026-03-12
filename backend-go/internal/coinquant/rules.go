package coinquant

import "math"

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func Round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func ConvictionScore(
	totalScore float64,
	momentumScore float64,
	liquidityScore float64,
	stabilityScore float64,
	trendScore float64,
	breakoutScore float64,
	return1H float64,
	return24H float64,
	volatility24H float64,
	drawdown24H float64,
	tradeValueRatio24H float64,
) float64 {
	score := 0.24*totalScore +
		0.18*momentumScore +
		0.18*liquidityScore +
		0.16*stabilityScore +
		0.14*trendScore +
		0.10*breakoutScore

	if return24H < 0 {
		score -= math.Min(18, math.Abs(return24H)*2.6)
	}
	if return24H > 14 {
		score -= math.Min(12, (return24H-14)*1.4)
	}
	if return1H < -1.5 {
		score -= math.Min(10, math.Abs(return1H+1.5)*2.8)
	}
	if return1H > 5.5 {
		score -= math.Min(8, (return1H-5.5)*2.4)
	}
	if tradeValueRatio24H < 1.0 {
		score -= math.Min(12, (1.0-tradeValueRatio24H)*42)
	}
	if volatility24H > 7.0 {
		score -= math.Min(16, (volatility24H-7.0)*2.7)
	}
	if drawdown24H < -10 {
		score -= math.Min(14, math.Abs(drawdown24H+10)*1.5)
	}

	return Round2(clamp(score, 0, 100))
}

func PassesStrictGate(
	totalScore float64,
	convictionScore float64,
	momentumScore float64,
	liquidityScore float64,
	stabilityScore float64,
	trendScore float64,
	breakoutScore float64,
	return1H float64,
	return24H float64,
	volatility24H float64,
	drawdown24H float64,
	tradeValueRatio24H float64,
) bool {
	return totalScore >= 76 &&
		convictionScore >= 72 &&
		momentumScore >= 64 &&
		liquidityScore >= 60 &&
		stabilityScore >= 58 &&
		trendScore >= 58 &&
		breakoutScore >= 52 &&
		return24H >= 1.0 &&
		return24H <= 16.0 &&
		return1H >= -1.4 &&
		return1H <= 4.8 &&
		volatility24H <= 7.5 &&
		drawdown24H >= -12.0 &&
		tradeValueRatio24H >= 1.05
}

func PassesRelaxedGate(
	totalScore float64,
	convictionScore float64,
	momentumScore float64,
	liquidityScore float64,
	stabilityScore float64,
	trendScore float64,
	breakoutScore float64,
	return1H float64,
	return24H float64,
	volatility24H float64,
	drawdown24H float64,
	tradeValueRatio24H float64,
) bool {
	return totalScore >= 70 &&
		convictionScore >= 66 &&
		momentumScore >= 60 &&
		liquidityScore >= 56 &&
		stabilityScore >= 54 &&
		trendScore >= 54 &&
		breakoutScore >= 48 &&
		return24H >= 0.2 &&
		return24H <= 18.0 &&
		return1H >= -2.0 &&
		return1H <= 5.4 &&
		volatility24H <= 8.6 &&
		drawdown24H >= -14.0 &&
		tradeValueRatio24H >= 0.95
}

func PassesSafetyGate(
	totalScore float64,
	convictionScore float64,
	momentumScore float64,
	liquidityScore float64,
	stabilityScore float64,
	trendScore float64,
	return1H float64,
	return24H float64,
	volatility24H float64,
	drawdown24H float64,
	tradeValueRatio24H float64,
) bool {
	return totalScore >= 64 &&
		convictionScore >= 60 &&
		momentumScore >= 54 &&
		liquidityScore >= 50 &&
		stabilityScore >= 48 &&
		trendScore >= 48 &&
		return24H >= -0.8 &&
		return24H <= 20.0 &&
		return1H >= -2.8 &&
		return1H <= 6.2 &&
		volatility24H <= 9.6 &&
		drawdown24H >= -16.0 &&
		tradeValueRatio24H >= 0.85
}
