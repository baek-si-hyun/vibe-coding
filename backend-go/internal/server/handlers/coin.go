package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"investment-news-go/internal/coinquant"
	"investment-news-go/internal/httpx"
)

const (
	defaultCoinLimit       = 5
	maxCoinRecommendations = 5
	defaultCoinTradeValue  = 5_000_000_000.0
)

type coinQuantItem struct {
	Rank               int     `json:"rank"`
	Exchange           string  `json:"exchange,omitempty"`
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
}

type coinQuantResponse struct {
	GeneratedAt      string `json:"generated_at"`
	AsOf             int64  `json:"asOf"`
	Interval         string `json:"interval"`
	ScoreModel       string `json:"score_model"`
	Limit            int    `json:"limit"`
	UniverseCount    int    `json:"universe_count"`
	QualifiedCount   int    `json:"qualified_count,omitempty"`
	MinTradeValue24H float64 `json:"min_trade_value_24h"`
	Breadth24H       float64 `json:"breadth_24h"`
	Average          struct {
		Return24H     float64 `json:"return_24h,omitempty"`
		Volatility24H float64 `json:"volatility_24h,omitempty"`
		TradeValue24H float64 `json:"trade_value_24h,omitempty"`
	} `json:"average,omitempty"`
	Items []coinQuantItem `json:"items"`
}

func decodeCoinQuantResponse(payload map[string]any) (coinQuantResponse, error) {
	var result coinQuantResponse
	raw, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
}

func passesMergedStrictGate(item coinQuantItem) bool {
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

func passesMergedRelaxedGate(item coinQuantItem) bool {
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

func passesMergedSafetyGate(item coinQuantItem) bool {
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

func selectMergedRecommendations(items []coinQuantItem, limit int) []coinQuantItem {
	if len(items) == 0 {
		return nil
	}

	selected := make([]coinQuantItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	appendIfNeeded := func(item coinQuantItem) {
		key := item.Exchange + ":" + item.Symbol
		if _, ok := seen[key]; ok {
			return
		}
		selected = append(selected, item)
		seen[key] = struct{}{}
	}

	for _, item := range items {
		if !passesMergedStrictGate(item) {
			continue
		}
		appendIfNeeded(item)
		if len(selected) >= limit {
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
			if !passesMergedRelaxedGate(item) {
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
			if !passesMergedSafetyGate(item) {
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

func sortCoinItems(items []coinQuantItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalScore != items[j].TotalScore {
			return items[i].TotalScore > items[j].TotalScore
		}
		if items[i].ConvictionScore != items[j].ConvictionScore {
			return items[i].ConvictionScore > items[j].ConvictionScore
		}
		if items[i].LiquidityScore != items[j].LiquidityScore {
			return items[i].LiquidityScore > items[j].LiquidityScore
		}
		return items[i].TradeValue24H > items[j].TradeValue24H
	})
}

func (h *Handlers) CoinScreener(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	limit := defaultCoinLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > maxCoinRecommendations {
				parsed = maxCoinRecommendations
			}
			limit = parsed
		}
	}

	minTradeValue24H := defaultCoinTradeValue
	if raw := r.URL.Query().Get("min_trade_value_24h"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			minTradeValue24H = parsed
		}
	}

	perExchangeLimit := limit * 3
	if perExchangeLimit < 8 {
		perExchangeLimit = 8
	}

	type fetchResult struct {
		exchange string
		payload  coinQuantResponse
		err      error
	}

	results := make(chan fetchResult, 2)
	var wg sync.WaitGroup
	fetch := func(exchange string, run func() (map[string]any, error)) {
		defer wg.Done()
		payload, err := run()
		if err != nil {
			results <- fetchResult{exchange: exchange, err: err}
			return
		}
		decoded, err := decodeCoinQuantResponse(payload)
		results <- fetchResult{exchange: exchange, payload: decoded, err: err}
	}

	wg.Add(2)
	go fetch("bithumb", func() (map[string]any, error) {
		return h.app.Bithumb.GetQuantData(perExchangeLimit, minTradeValue24H)
	})
	go fetch("upbit", func() (map[string]any, error) {
		return h.app.Upbit.GetQuantData(perExchangeLimit, minTradeValue24H)
	})
	go func() {
		wg.Wait()
		close(results)
	}()

	merged := make([]coinQuantItem, 0, perExchangeLimit*2)
	warnings := make([]string, 0, 2)
	interval := "1h"
	asOf := time.Now().UnixMilli()
	universeCount := 0

	for result := range results {
		if result.err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", result.exchange, result.err))
			continue
		}
		if result.payload.Interval != "" {
			interval = result.payload.Interval
		}
		if result.payload.AsOf > asOf {
			asOf = result.payload.AsOf
		}
		universeCount += result.payload.UniverseCount
		for _, item := range result.payload.Items {
			item.Exchange = result.exchange
			merged = append(merged, item)
		}
	}

	if len(merged) == 0 {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":    "코인 퀀트 데이터를 계산하지 못했습니다.",
			"warnings": warnings,
		})
		return
	}

	sortCoinItems(merged)
	selected := selectMergedRecommendations(merged, limit)
	if len(selected) == 0 {
		selected = merged
		if len(selected) > limit {
			selected = selected[:limit]
		}
	}
	sortCoinItems(selected)
	for i := range selected {
		selected[i].Rank = i + 1
	}

	breadthPositive24H := 0
	avgReturn24H := 0.0
	avgVolatility24H := 0.0
	avgTradeValue24H := 0.0
	for _, item := range selected {
		if item.Return24H > 0 {
			breadthPositive24H++
		}
		avgReturn24H += item.Return24H
		avgVolatility24H += item.Volatility24H
		avgTradeValue24H += item.TradeValue24H
	}
	if len(selected) > 0 {
		avgReturn24H /= float64(len(selected))
		avgVolatility24H /= float64(len(selected))
		avgTradeValue24H /= float64(len(selected))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"generated_at":        time.Now().Format(time.RFC3339),
		"asOf":                asOf,
		"interval":            interval,
		"score_model":         "coin_quant_score_v2_merged",
		"limit":               limit,
		"universe_count":      universeCount,
		"qualified_count":     len(selected),
		"min_trade_value_24h": coinquant.Round2(minTradeValue24H),
		"breadth_24h":         coinquant.Round2(float64(breadthPositive24H) / float64(len(selected)) * 100),
		"average": map[string]any{
			"return_24h":      coinquant.Round2(avgReturn24H),
			"volatility_24h":  coinquant.Round2(avgVolatility24H),
			"trade_value_24h": coinquant.Round2(avgTradeValue24H),
		},
		"warnings": warnings,
		"exchanges": []string{
			"bithumb",
			"upbit",
		},
		"items": selected,
	})
}
