package bithumb

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"investment-news-go/internal/coinquant"
)

const (
	defaultQuantLimit       = 30
	maxQuantLimit           = 100
	defaultMinTradeValue24H = 5_000_000_000.0
	quantInterval           = "1h"
	quantCandles24H         = 24
	quantMinRequiredCandles = quantCandles24H * 2
)

type QuantItem struct {
	Rank               int     `json:"rank"`
	Symbol             string  `json:"symbol"`
	Price              float64 `json:"price"`
	CandleTime         int64   `json:"candleTime"`
	Return1H           float64 `json:"return_1h"`
	Return24H          float64 `json:"return_24h"`
	Volatility24H      float64 `json:"volatility_24h"`
	Drawdown24H        float64 `json:"drawdown_24h"`
	TradeValue24H      float64 `json:"trade_value_24h"`
	TradeValueRatio24H float64 `json:"trade_value_ratio_24h"`
	MomentumScore      float64 `json:"momentum_score"`
	LiquidityScore     float64 `json:"liquidity_score"`
	StabilityScore     float64 `json:"stability_score"`
	TrendScore         float64 `json:"trend_score"`
	BreakoutScore      float64 `json:"breakout_score"`
	ConvictionScore    float64 `json:"conviction_score"`
	TotalScore         float64 `json:"total_score"`

	rawMomentumShort float64
	rawLiquidityNow  float64
	rawLiquidityFlow float64
	rawStability     float64
	rawDrawdown      float64
	rawTrend         float64
	rawBreakout      float64
	rawStretch       float64
}

func pctChange(curr, prev float64) float64 {
	if prev == 0 {
		return 0
	}
	return (curr/prev - 1) * 100
}

func meanFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range vals {
		total += v
	}
	return total / float64(len(vals))
}

func stddevFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	avg := meanFloat(vals)
	variance := 0.0
	for _, v := range vals {
		diff := v - avg
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(vals)))
}

func tailFloat(vals []float64, count int) []float64 {
	if count <= 0 || len(vals) == 0 {
		return nil
	}
	if len(vals) <= count {
		return vals
	}
	return vals[len(vals)-count:]
}

func ema(vals []float64, window int) float64 {
	if len(vals) == 0 {
		return 0
	}
	alpha := 2.0 / float64(window+1)
	value := vals[0]
	for i := 1; i < len(vals); i++ {
		value = alpha*vals[i] + (1-alpha)*value
	}
	return value
}

func maxDrawdownFloat(closes []float64, window int) float64 {
	segment := tailFloat(closes, window)
	if len(segment) == 0 {
		return 0
	}
	peak := segment[0]
	maxDD := 0.0
	for _, close := range segment {
		if close > peak {
			peak = close
		}
		if peak <= 0 {
			continue
		}
		drawdown := (close/peak - 1) * 100
		if drawdown < maxDD {
			maxDD = drawdown
		}
	}
	return maxDD
}

func percentileScores(values []float64) []float64 {
	out := make([]float64, len(values))
	type pair struct {
		idx int
		val float64
	}
	valid := make([]pair, 0, len(values))
	for idx, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		valid = append(valid, pair{idx: idx, val: value})
	}
	if len(valid) == 0 {
		return out
	}
	if len(valid) == 1 {
		out[valid[0].idx] = 50
		return out
	}

	sort.Slice(valid, func(i, j int) bool {
		if valid[i].val == valid[j].val {
			return valid[i].idx < valid[j].idx
		}
		return valid[i].val < valid[j].val
	})

	for start := 0; start < len(valid); {
		end := start + 1
		for end < len(valid) && valid[end].val == valid[start].val {
			end++
		}
		score := float64(start+end-1) / 2 / float64(len(valid)-1) * 100
		for i := start; i < end; i++ {
			out[valid[i].idx] = score
		}
		start = end
	}

	return out
}

func buildQuantItem(symbol string, candles []Candle, minTradeValue24H float64) *QuantItem {
	if len(candles) < quantMinRequiredCandles {
		return nil
	}

	closes := make([]float64, len(candles))
	highs := make([]float64, len(candles))
	tradeValues := make([]float64, len(candles))
	returns1H := make([]float64, 0, quantCandles24H)
	for i, candle := range candles {
		closes[i] = candle.Close
		highs[i] = candle.High
		tradeValues[i] = candle.Close * candle.Volume
		if i > 0 && candles[i-1].Close > 0 {
			returns1H = append(returns1H, pctChange(candle.Close, candles[i-1].Close))
		}
	}

	n := len(closes)
	latest := candles[n-1]
	price := latest.Close
	if price <= 0 {
		return nil
	}

	value24h := 0.0
	for _, value := range tailFloat(tradeValues, quantCandles24H) {
		value24h += value
	}
	if value24h < minTradeValue24H {
		return nil
	}

	valuePrev24h := 0.0
	for _, value := range tailFloat(tradeValues[:n-quantCandles24H], quantCandles24H) {
		valuePrev24h += value
	}
	valueRatio24H := 0.0
	if valuePrev24h > 0 {
		valueRatio24H = value24h / valuePrev24h
	}

	return1h := pctChange(closes[n-1], closes[n-2])
	return24h := pctChange(closes[n-1], closes[n-1-quantCandles24H])
	vol24h := stddevFloat(tailFloat(returns1H, quantCandles24H))
	drawdown24h := maxDrawdownFloat(closes, quantCandles24H)

	ema24h := ema(tailFloat(closes, quantCandles24H), quantCandles24H)
	trend := 0.60*pctChange(price, ema24h) + 0.40*return24h

	highest24h := 0.0
	for _, high := range tailFloat(highs, quantCandles24H) {
		if high > highest24h {
			highest24h = high
		}
	}
	breakout := -math.Abs(pctChange(price, highest24h))
	stretch := -math.Abs(pctChange(price, ema24h))

	if return24h < -12 || drawdown24h < -18 || vol24h > 12 {
		return nil
	}

	return &QuantItem{
		Symbol:             symbol,
		Price:              price,
		CandleTime:         latest.Timestamp,
		Return1H:           return1h,
		Return24H:          return24h,
		Volatility24H:      vol24h,
		Drawdown24H:        drawdown24h,
		TradeValue24H:      value24h,
		TradeValueRatio24H: valueRatio24H,
		rawMomentumShort:   0.35*return1h + 0.65*return24h,
		rawLiquidityNow:    math.Log1p(value24h),
		rawLiquidityFlow:   valueRatio24H,
		rawStability:       -vol24h,
		rawDrawdown:        drawdown24h,
		rawTrend:           trend,
		rawBreakout:        breakout,
		rawStretch:         stretch,
	}
}

func cacheKey(limit int, minTradeValue24H float64) string {
	return strconv.Itoa(limit) + "|" + strconv.FormatFloat(minTradeValue24H, 'f', 0, 64)
}

func (s *Service) GetQuantData(limit int, minTradeValue24H float64) (map[string]any, error) {
	if limit <= 0 {
		limit = defaultQuantLimit
	}
	if limit > maxQuantLimit {
		limit = maxQuantLimit
	}
	if minTradeValue24H <= 0 {
		minTradeValue24H = defaultMinTradeValue24H
	}

	key := cacheKey(limit, minTradeValue24H)
	now := time.Now()

	s.mu.RLock()
	cached, ok := s.quantCache[key]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) {
		return cached.result, nil
	}

	symbols, err := s.fetchSymbols()
	if err != nil {
		return nil, fmt.Errorf("심볼 조회 실패: %w", err)
	}

	items := make([]QuantItem, 0, len(symbols))
	sem := make(chan struct{}, concurrency)
	results := make(chan *QuantItem, len(symbols))
	var wg sync.WaitGroup
	for _, symbol := range symbols {
		symbol := symbol
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			candles, fetchErr := s.fetchCandles(symbol, quantInterval)
			if fetchErr != nil {
				return
			}
			if item := buildQuantItem(symbol, candles, minTradeValue24H); item != nil {
				results <- item
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	for item := range results {
		items = append(items, *item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("조건을 만족하는 코인이 없습니다")
	}

	momentumShortScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawMomentumShort }))
	liquidityNowScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawLiquidityNow }))
	liquidityFlowScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawLiquidityFlow }))
	stabilityScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawStability }))
	drawdownScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawDrawdown }))
	trendScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawTrend }))
	breakoutScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawBreakout }))
	stretchScores := percentileScores(extractValues(items, func(item QuantItem) float64 { return item.rawStretch }))

	for i := range items {
		it := &items[i]
		it.MomentumScore = round2(0.70*momentumShortScores[i] + 0.30*trendScores[i])
		it.LiquidityScore = round2(0.60*liquidityNowScores[i] + 0.40*liquidityFlowScores[i])
		it.StabilityScore = round2(0.65*stabilityScores[i] + 0.35*drawdownScores[i])
		it.TrendScore = round2(0.60*trendScores[i] + 0.40*stretchScores[i])
		it.BreakoutScore = round2(0.55*breakoutScores[i] + 0.45*trendScores[i])
		it.TotalScore = round2(
			0.32*it.MomentumScore +
				0.22*it.LiquidityScore +
				0.20*it.StabilityScore +
				0.16*it.BreakoutScore +
				0.10*it.TrendScore,
		)
		it.Return1H = round2(it.Return1H)
		it.Return24H = round2(it.Return24H)
		it.Volatility24H = round2(it.Volatility24H)
		it.Drawdown24H = round2(it.Drawdown24H)
		it.TradeValue24H = math.Round(it.TradeValue24H)
		it.TradeValueRatio24H = round2(it.TradeValueRatio24H)
		it.ConvictionScore = coinquant.ConvictionScore(
			it.TotalScore,
			it.MomentumScore,
			it.LiquidityScore,
			it.StabilityScore,
			it.TrendScore,
			it.BreakoutScore,
			it.Return1H,
			it.Return24H,
			it.Volatility24H,
			it.Drawdown24H,
			it.TradeValueRatio24H,
		)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalScore != items[j].TotalScore {
			return items[i].TotalScore > items[j].TotalScore
		}
		if items[i].ConvictionScore != items[j].ConvictionScore {
			return items[i].ConvictionScore > items[j].ConvictionScore
		}
		if items[i].MomentumScore != items[j].MomentumScore {
			return items[i].MomentumScore > items[j].MomentumScore
		}
		if items[i].LiquidityScore != items[j].LiquidityScore {
			return items[i].LiquidityScore > items[j].LiquidityScore
		}
		return items[i].TradeValue24H > items[j].TradeValue24H
	})

	universeCount := len(items)
	items = selectRecommendations(items, limit)
	if len(items) == 0 {
		return nil, fmt.Errorf("신중한 기준을 만족하는 빗썸 추천 코인이 없습니다")
	}
	for i := range items {
		items[i].Rank = i + 1
	}

	breadthPositive24H := 0
	avgReturn24H := 0.0
	avgVolatility24H := 0.0
	avgTradeValue24H := 0.0
	for _, item := range items {
		if item.Return24H > 0 {
			breadthPositive24H++
		}
		avgReturn24H += item.Return24H
		avgVolatility24H += item.Volatility24H
		avgTradeValue24H += item.TradeValue24H
	}
	if len(items) > 0 {
		avgReturn24H /= float64(len(items))
		avgVolatility24H /= float64(len(items))
		avgTradeValue24H /= float64(len(items))
	}

	result := map[string]any{
		"generated_at":        now.Format(time.RFC3339),
		"asOf":                now.UnixMilli(),
		"interval":            quantInterval,
		"score_model":         "coin_quant_score_v1",
		"limit":               limit,
		"universe_count":      universeCount,
		"qualified_count":     len(items),
		"min_trade_value_24h": math.Round(minTradeValue24H),
		"average": map[string]any{
			"return_24h":      round2(avgReturn24H),
			"volatility_24h":  round2(avgVolatility24H),
			"trade_value_24h": math.Round(avgTradeValue24H),
		},
		"breadth_24h": round2(float64(breadthPositive24H) / float64(len(items)) * 100),
		"items":       items,
	}

	s.mu.Lock()
	s.quantCache[key] = cacheEntry{
		expiresAt: now.Add(quantCacheTTL),
		result:    result,
	}
	s.mu.Unlock()

	return result, nil
}

func extractValues(items []QuantItem, selector func(QuantItem) float64) []float64 {
	values := make([]float64, len(items))
	for i, item := range items {
		values[i] = selector(item)
	}
	return values
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func passesStrictRecommendationGate(item QuantItem) bool {
	return coinquant.PassesStrictGate(
		item.TotalScore,
		item.ConvictionScore,
		item.MomentumScore,
		item.LiquidityScore,
		item.StabilityScore,
		item.TrendScore,
		item.BreakoutScore,
		item.Return1H,
		item.Return24H,
		item.Volatility24H,
		item.Drawdown24H,
		item.TradeValueRatio24H,
	)
}

func passesRelaxedRecommendationGate(item QuantItem) bool {
	return coinquant.PassesRelaxedGate(
		item.TotalScore,
		item.ConvictionScore,
		item.MomentumScore,
		item.LiquidityScore,
		item.StabilityScore,
		item.TrendScore,
		item.BreakoutScore,
		item.Return1H,
		item.Return24H,
		item.Volatility24H,
		item.Drawdown24H,
		item.TradeValueRatio24H,
	)
}

func passesSafetyRecommendationGate(item QuantItem) bool {
	return coinquant.PassesSafetyGate(
		item.TotalScore,
		item.ConvictionScore,
		item.MomentumScore,
		item.LiquidityScore,
		item.StabilityScore,
		item.TrendScore,
		item.Return1H,
		item.Return24H,
		item.Volatility24H,
		item.Drawdown24H,
		item.TradeValueRatio24H,
	)
}

func selectRecommendations(items []QuantItem, limit int) []QuantItem {
	if len(items) == 0 {
		return nil
	}

	selected := make([]QuantItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	appendIfNeeded := func(item QuantItem) {
		if _, ok := seen[item.Symbol]; ok {
			return
		}
		selected = append(selected, item)
		seen[item.Symbol] = struct{}{}
	}

	strictTarget := limit
	if strictTarget > len(items) {
		strictTarget = len(items)
	}
	for _, item := range items {
		if !passesStrictRecommendationGate(item) {
			continue
		}
		appendIfNeeded(item)
		if len(selected) >= strictTarget {
			break
		}
	}

	relaxedTarget := limit
	if relaxedTarget < 3 {
		relaxedTarget = 3
	}
	if relaxedTarget > len(items) {
		relaxedTarget = len(items)
	}
	if len(selected) < relaxedTarget {
		for _, item := range items {
			if !passesRelaxedRecommendationGate(item) {
				continue
			}
			appendIfNeeded(item)
			if len(selected) >= relaxedTarget {
				break
			}
		}
	}

	safetyTarget := limit
	if safetyTarget < 2 {
		safetyTarget = 2
	}
	if safetyTarget > len(items) {
		safetyTarget = len(items)
	}
	if len(selected) < safetyTarget {
		for _, item := range items {
			if !passesSafetyRecommendationGate(item) {
				continue
			}
			appendIfNeeded(item)
			if len(selected) >= safetyTarget {
				break
			}
		}
	}

	if len(selected) > limit {
		return selected[:limit]
	}
	return selected
}
