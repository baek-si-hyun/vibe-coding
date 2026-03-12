package quant

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"investment-news-go/internal/config"
	"investment-news-go/internal/news"
)

const (
	defaultRankLimit            = 30
	maxRankLimit                = 300
	defaultMinMarketCap   int64 = 1_000_000_000_000 // 1조
	cacheTTL                    = 60 * time.Second
	baseScoreModelVersion       = "nextday_focus_v7"
	stockNewsScoreWeight        = 0.10
	nxtScoreWeight              = 0.08
)

type Service struct {
	cfg        config.Config
	news       *news.Service
	cache      map[string]cacheEntry
	macroCache macroCacheEntry
	lstmCache  lstmPredictionCache
	lstmTuning lstmTuningCache
	nxtCache   nxtSnapshotCache
	mu         sync.RWMutex
}

type cacheEntry struct {
	expiresAt time.Time
	result    rankResult
}

type rankResult struct {
	GeneratedAt           string     `json:"generated_at"`
	ScoreModel            string     `json:"score_model"`
	Market                string     `json:"market"`
	MinMarketCap          int64      `json:"min_market_cap"`
	UniverseCount         int        `json:"universe_count"`
	AsOfMin               string     `json:"as_of_min"`
	AsOfMax               string     `json:"as_of_max"`
	Items                 []RankItem `json:"items"`
	NXTEnabled            bool       `json:"nxt_enabled"`
	NXTTradingDate        string     `json:"nxt_trading_date,omitempty"`
	NXTSetTime            string     `json:"nxt_set_time,omitempty"`
	NXTQuoteCount         int        `json:"nxt_quote_count,omitempty"`
	NXTAppliedCount       int        `json:"nxt_applied_count,omitempty"`
	LSTMEnabled           bool       `json:"lstm_enabled"`
	LSTMModelVersion      string     `json:"lstm_model_version,omitempty"`
	LSTMWeight            float64    `json:"lstm_weight,omitempty"`
	LSTMPredictionAsOf    string     `json:"lstm_prediction_as_of,omitempty"`
	LSTMPredictionCount   int        `json:"lstm_prediction_count,omitempty"`
	LSTMAppliedCount      int        `json:"lstm_applied_count,omitempty"`
	LSTMTuningMode        string     `json:"lstm_tuning_mode,omitempty"`
	LSTMTuningWeightMult  float64    `json:"lstm_tuning_weight_multiplier,omitempty"`
	LSTMTuningRecentHit   float64    `json:"lstm_tuning_recent_hit_rate,omitempty"`
	LSTMTuningRecentTopK  float64    `json:"lstm_tuning_recent_topk_hit_rate,omitempty"`
	LSTMTuningEvalCount   int        `json:"lstm_tuning_recent_evaluated_count,omitempty"`
	NewsStockAppliedCount int        `json:"news_stock_applied_count,omitempty"`
	NewsTargetTradingDate string     `json:"news_target_trading_date,omitempty"`
	NewsRiskOnProb        float64    `json:"news_risk_on_prob,omitempty"`
	NewsRiskOffProb       float64    `json:"news_risk_off_prob,omitempty"`
	NewsNeutralProb       float64    `json:"news_neutral_prob,omitempty"`
	NewsRegimeConfidence  float64    `json:"news_regime_confidence,omitempty"`
	NewsRegimeBias        string     `json:"news_regime_bias,omitempty"`
	NewsTimePrecision     string     `json:"news_time_precision,omitempty"`
}

type RankItem struct {
	Rank               int     `json:"rank"`
	Market             string  `json:"market"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	AsOf               string  `json:"as_of"`
	Close              float64 `json:"close"`
	MarketCap          int64   `json:"market_cap"`
	Turnover           float64 `json:"turnover"`
	TurnoverRatio      float64 `json:"turnover_ratio"`
	Return1D           float64 `json:"return_1d"`
	Return5D           float64 `json:"return_5d"`
	Return10D          float64 `json:"return_10d"`
	Return20D          float64 `json:"return_20d"`
	Return60D          float64 `json:"return_60d"`
	Return120D         float64 `json:"return_120d"`
	Volatility20D      float64 `json:"volatility_20d"`
	Volatility60D      float64 `json:"volatility_60d"`
	DownsideVol20D     float64 `json:"downside_volatility_20d"`
	Drawdown60D        float64 `json:"drawdown_60d"`
	Drawdown120D       float64 `json:"drawdown_120d"`
	AvgTurnover20D     float64 `json:"avg_turnover_ratio_20d"`
	NextDayScore       float64 `json:"next_day_score"`
	MomentumScore      float64 `json:"momentum_score"`
	LiquidityScore     float64 `json:"liquidity_score"`
	StabilityScore     float64 `json:"stability_score"`
	TrendScore         float64 `json:"trend_score"`
	RiskAdjScore       float64 `json:"risk_adjusted_score"`
	NewsScore          float64 `json:"news_score,omitempty"`
	NewsSentiment      float64 `json:"news_sentiment,omitempty"`
	NewsBuzz           float64 `json:"news_buzz,omitempty"`
	NewsBias           string  `json:"news_bias,omitempty"`
	NewsArticleCount   int     `json:"news_article_count,omitempty"`
	NewsPositiveCount  int     `json:"news_positive_count,omitempty"`
	NewsNegativeCount  int     `json:"news_negative_count,omitempty"`
	NXTPrice           float64 `json:"nxt_price,omitempty"`
	NXTReturn          float64 `json:"nxt_return,omitempty"`
	NXTIntradayReturn  float64 `json:"nxt_intraday_return,omitempty"`
	NXTTradeValueRatio float64 `json:"nxt_trade_value_ratio,omitempty"`
	NXTCloseStrength   float64 `json:"nxt_close_strength,omitempty"`
	NXTScore           float64 `json:"nxt_score,omitempty"`
	LSTMScore          float64 `json:"lstm_score,omitempty"`
	LSTMPredReturn1D   float64 `json:"lstm_pred_return_1d,omitempty"`
	LSTMPredReturn5D   float64 `json:"lstm_pred_return_5d,omitempty"`
	LSTMPredReturn20D  float64 `json:"lstm_pred_return_20d,omitempty"`
	LSTMProbUp         float64 `json:"lstm_prob_up,omitempty"`
	LSTMConfidence     float64 `json:"lstm_confidence,omitempty"`
	TotalScore         float64 `json:"total_score"`

	rawShortMomentum         float64
	rawMomentum20            float64
	rawMomentum60            float64
	rawMomentum120           float64
	rawRiskAdjusted          float64
	rawTrendAlignment        float64
	rawTrendQuality          float64
	rawLiquidityNow          float64
	rawLiquidityAverage      float64
	rawLiquidityStability    float64
	rawLiquiditySize         float64
	rawStability20           float64
	rawStability60           float64
	rawStabilityDownside     float64
	rawDrawdown60            float64
	rawDrawdown120           float64
	rawNextDayMomentum       float64
	rawCloseStrength         float64
	rawVolumeImpulse         float64
	rawTurnoverAcceleration  float64
	rawBreakoutPressure      float64
	rawFreshBreakout         float64
	rawIntradayFollowThrough float64
	rawCompressionSetup      float64
	rawStretchControl        float64
	rawBounceQuality         float64
	rawNewsScore             float64
	rawNXTChangeRate         float64
	rawNXTIntradayReturn     float64
	rawNXTTradeValueRatio    float64
	rawNXTTradeValueImpulse  float64
	rawNXTCloseStrength      float64
	rawLSTMReturn1D          float64
	rawLSTMReturn5D          float64
	rawLSTMReturn20D         float64
	rawLSTMProbability       float64
	rawLSTMConfidence        float64
	rawLSTMValidationAcc     float64
	rawLSTMBrierQuality      float64
	hasStockNews             bool
	hasNXT                   bool
	hasLSTM                  bool
}

func NewService(cfg config.Config) *Service {
	return &Service{
		cfg:   cfg,
		news:  news.NewService(cfg),
		cache: map[string]cacheEntry{},
	}
}

func sanitizeMarket(raw string) string {
	v := strings.ToUpper(strings.TrimSpace(raw))
	switch v {
	case "KOSPI", "KOSDAQ", "ALL":
		return v
	default:
		return "ALL"
	}
}

func sanitizeMinMarketCap(v int64) int64 {
	if v < defaultMinMarketCap {
		return defaultMinMarketCap
	}
	return v
}

func normalizeScore(v, minV, maxV float64) float64 {
	if !isFinite(v) {
		return 0
	}
	if !isFinite(minV) || !isFinite(maxV) || maxV <= minV {
		return 50
	}
	score := (v - minV) / (maxV - minV) * 100
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func parseFloat(raw string) float64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseInt64(raw string) int64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(raw, 64)
		if ferr != nil {
			return 0
		}
		return int64(f)
	}
	return v
}

func pctChange(curr, prev float64) float64 {
	if prev == 0 {
		return 0
	}
	return (curr/prev - 1) * 100
}

func stddev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))

	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(vals))
	return math.Sqrt(variance)
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range vals {
		total += v
	}
	return total / float64(len(vals))
}

func tailSlice(vals []float64, count int) []float64 {
	if count <= 0 || len(vals) == 0 {
		return nil
	}
	if len(vals) <= count {
		return vals
	}
	return vals[len(vals)-count:]
}

func dailyReturnsWindow(closes []float64, window int) []float64 {
	if len(closes) < 2 {
		return nil
	}
	start := len(closes) - window - 1
	if start < 0 {
		start = 0
	}
	out := make([]float64, 0, window)
	for i := start + 1; i < len(closes); i++ {
		prev := closes[i-1]
		if prev <= 0 {
			continue
		}
		out = append(out, (closes[i]/prev-1)*100)
	}
	return out
}

func movingAverage(closes []float64, window int) float64 {
	return mean(tailSlice(closes, window))
}

func downsideStddev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	downs := make([]float64, 0, len(vals))
	for _, v := range vals {
		if v < 0 {
			downs = append(downs, v)
		} else {
			downs = append(downs, 0)
		}
	}
	return stddev(downs)
}

func maxDrawdown(closes []float64, window int) float64 {
	segment := tailSlice(closes, window)
	if len(segment) == 0 {
		return 0
	}
	peak := segment[0]
	maxDD := 0.0
	for _, closePx := range segment {
		if closePx > peak {
			peak = closePx
		}
		if peak <= 0 {
			continue
		}
		drawdown := (closePx/peak - 1) * 100
		if drawdown < maxDD {
			maxDD = drawdown
		}
	}
	return maxDD
}

func positiveRatio(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	positive := 0
	for _, v := range vals {
		if v > 0 {
			positive++
		}
	}
	return float64(positive) / float64(len(vals)) * 100
}

func efficiencyRatio(closes []float64, window int) float64 {
	segment := tailSlice(closes, window+1)
	if len(segment) < 2 {
		return 0
	}
	netMove := math.Abs(segment[len(segment)-1] - segment[0])
	path := 0.0
	for i := 1; i < len(segment); i++ {
		path += math.Abs(segment[i] - segment[i-1])
	}
	if path == 0 {
		return 0
	}
	return netMove / path * 100
}

func coefficientOfVariation(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := mean(vals)
	if m == 0 {
		return 0
	}
	return stddev(vals) / math.Abs(m)
}

func maxFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	maxV := vals[0]
	for _, v := range vals[1:] {
		if v > maxV {
			maxV = v
		}
	}
	return maxV
}

func minFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	minV := vals[0]
	for _, v := range vals[1:] {
		if v < minV {
			minV = v
		}
	}
	return minV
}

func safeRatio(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

type lstmGateThresholds struct {
	MinProbUp     float64
	MinConfidence float64
	MinPredReturn float64
}

func effectiveLSTMWeight(base float64, tuning lstmTuningProfile) float64 {
	if !tuning.Enabled {
		return base
	}
	return clampFloat(base*tuning.WeightMultiplier, 0, 0.35)
}

func resolveLSTMThresholds(mode string, regime marketRegime, tuning lstmTuningProfile) lstmGateThresholds {
	thresholds := lstmGateThresholds{}
	switch mode {
	case "strict":
		thresholds = lstmGateThresholds{MinProbUp: 56, MinConfidence: 52, MinPredReturn: 0.10}
		if regime.RiskOff {
			thresholds = lstmGateThresholds{MinProbUp: 58, MinConfidence: 56, MinPredReturn: 0.18}
		}
		if regime.NewsHighRiskOff {
			thresholds.MinProbUp += 2
			thresholds.MinConfidence += 1
			thresholds.MinPredReturn += 0.04
		}
		if regime.NewsHighRiskOn && !regime.RiskOff {
			thresholds.MinProbUp -= 2
		}
		if tuning.Enabled {
			thresholds.MinProbUp += tuning.StrictProbUpDelta
			thresholds.MinConfidence += tuning.StrictConfidenceDelta
			thresholds.MinPredReturn += tuning.StrictPredReturnDelta
		}
	case "relaxed":
		thresholds = lstmGateThresholds{MinProbUp: 52, MinConfidence: 48, MinPredReturn: 0.00}
		if regime.RiskOff {
			thresholds = lstmGateThresholds{MinProbUp: 54, MinConfidence: 50, MinPredReturn: 0.05}
		}
		if tuning.Enabled {
			thresholds.MinProbUp += tuning.RelaxedProbUpDelta
			thresholds.MinConfidence += tuning.RelaxedConfidenceDelta
			thresholds.MinPredReturn += tuning.RelaxedPredReturnDelta
		}
	case "safety":
		thresholds = lstmGateThresholds{MinProbUp: 50, MinConfidence: 46, MinPredReturn: 0.00}
		if regime.RiskOff {
			thresholds = lstmGateThresholds{MinProbUp: 52, MinConfidence: 48, MinPredReturn: 0.05}
		}
		if tuning.Enabled {
			thresholds.MinProbUp += tuning.SafetyProbUpDelta
			thresholds.MinConfidence += tuning.SafetyConfidenceDelta
			thresholds.MinPredReturn += tuning.SafetyPredReturnDelta
		}
	}
	thresholds.MinProbUp = clampFloat(thresholds.MinProbUp, 42, 72)
	thresholds.MinConfidence = clampFloat(thresholds.MinConfidence, 38, 72)
	thresholds.MinPredReturn = clampFloat(thresholds.MinPredReturn, -0.05, 0.40)
	return thresholds
}

type marketRegime struct {
	AvgReturn1D       float64
	AvgReturn5D       float64
	Breadth1D         float64
	Breadth5D         float64
	Breadth20D        float64
	NewsRiskOnProb    float64
	NewsRiskOffProb   float64
	NewsNeutralProb   float64
	NewsConfidence    float64
	NewsBias          string
	NewsTimePrecision string
	NewsHighRiskOff   bool
	NewsHighRiskOn    bool
	RiskOff           bool
}

func detectMarketRegime(items []RankItem, newsRegime *news.MarketNewsRegime) marketRegime {
	if len(items) == 0 {
		return marketRegime{}
	}

	sumReturn1D := 0.0
	sumReturn5D := 0.0
	positive1D := 0
	positive5D := 0
	positive20D := 0
	for _, item := range items {
		sumReturn1D += item.Return1D
		sumReturn5D += item.Return5D
		if item.Return1D > 0 {
			positive1D++
		}
		if item.Return5D > 0 {
			positive5D++
		}
		if item.Return20D > 0 {
			positive20D++
		}
	}

	avgReturn1D := sumReturn1D / float64(len(items))
	avgReturn5D := sumReturn5D / float64(len(items))
	breadth1D := float64(positive1D) / float64(len(items)) * 100
	breadth5D := float64(positive5D) / float64(len(items)) * 100
	breadth20D := float64(positive20D) / float64(len(items)) * 100
	newsRiskOnProb := 0.0
	newsRiskOffProb := 0.0
	newsNeutralProb := 0.0
	newsConfidence := 0.0
	newsBias := ""
	newsTimePrecision := ""
	newsHighRiskOff := false
	newsHighRiskOn := false
	if newsRegime != nil {
		newsRiskOnProb = newsRegime.RiskOnProb * 100
		newsRiskOffProb = newsRegime.RiskOffProb * 100
		newsNeutralProb = newsRegime.NeutralProb * 100
		newsConfidence = newsRegime.Confidence * 100
		newsBias = strings.TrimSpace(newsRegime.Bias)
		newsTimePrecision = strings.TrimSpace(newsRegime.TimePrecision)
		newsHighRiskOff = newsRegime.RiskOffProb >= 0.56 && newsRegime.Confidence >= 0.28
		newsHighRiskOn = newsRegime.RiskOnProb >= 0.56 && newsRegime.Confidence >= 0.28
	}
	riskOff := avgReturn1D < -0.25 || avgReturn5D < -1.0 || breadth1D < 45 || breadth5D < 42 || breadth20D < 38 || newsHighRiskOff

	return marketRegime{
		AvgReturn1D:       avgReturn1D,
		AvgReturn5D:       avgReturn5D,
		Breadth1D:         breadth1D,
		Breadth5D:         breadth5D,
		Breadth20D:        breadth20D,
		NewsRiskOnProb:    newsRiskOnProb,
		NewsRiskOffProb:   newsRiskOffProb,
		NewsNeutralProb:   newsNeutralProb,
		NewsConfidence:    newsConfidence,
		NewsBias:          newsBias,
		NewsTimePrecision: newsTimePrecision,
		NewsHighRiskOff:   newsHighRiskOff,
		NewsHighRiskOn:    newsHighRiskOn,
		RiskOff:           riskOff,
	}
}

func passesRecommendationGate(item RankItem, regime marketRegime, tuning lstmTuningProfile) bool {
	minTotalScore := 60.0
	minNextDayScore := 62.0
	minMomentumScore := 56.0
	minTurnoverNow := 0.04
	minTurnoverAvg := 0.03
	minCloseStrength := 58.0
	minTurnoverAccel := 0.95
	minFollowThrough := 0.1
	minFreshBreakout := -3.0
	maxVolatility20 := 5.4
	maxDrawdown60 := -22.0
	maxPositiveReturn1D := 5.4
	minNegativeReturn1D := -3.0
	minStabilityScore := 48.0

	if regime.RiskOff {
		minTotalScore = 68.0
		minNextDayScore = 70.0
		minMomentumScore = 62.0
		minTurnoverNow = 0.05
		minTurnoverAvg = 0.04
		minCloseStrength = 66.0
		minTurnoverAccel = 1.15
		minFollowThrough = 0.45
		minFreshBreakout = -0.8
		maxVolatility20 = 4.8
		maxDrawdown60 = -18.0
		maxPositiveReturn1D = 3.8
		minNegativeReturn1D = -2.2
		minStabilityScore = 55.0
	}

	if regime.NewsHighRiskOff {
		minTotalScore += 2
		minNextDayScore += 2
		minMomentumScore += 1
		minCloseStrength += 2
		minTurnoverAccel += 0.05
		maxVolatility20 -= 0.3
		maxPositiveReturn1D -= 0.5
		minNegativeReturn1D += 0.4
		minStabilityScore += 2
	}

	if regime.NewsHighRiskOn && !regime.RiskOff {
		minNextDayScore -= 2
		minMomentumScore -= 1
		minCloseStrength -= 1
		minTurnoverAccel -= 0.05
		maxPositiveReturn1D += 0.6
	}

	if item.TotalScore < minTotalScore || item.MomentumScore < minMomentumScore {
		return false
	}
	if item.NextDayScore < minNextDayScore {
		return false
	}
	if item.TurnoverRatio < minTurnoverNow || item.AvgTurnover20D < minTurnoverAvg {
		return false
	}
	if item.rawCloseStrength < minCloseStrength {
		return false
	}
	if item.rawTurnoverAcceleration < minTurnoverAccel || item.rawIntradayFollowThrough < minFollowThrough {
		return false
	}
	if item.rawFreshBreakout < minFreshBreakout {
		return false
	}
	if item.Volatility20D > maxVolatility20 {
		return false
	}
	if item.StabilityScore < minStabilityScore {
		return false
	}
	if item.Drawdown60D < maxDrawdown60 || item.Return20D < -10 || item.Return10D < -5 {
		return false
	}
	if item.Return5D < -0.5 {
		return false
	}
	if item.Return1D < minNegativeReturn1D || item.Return1D > maxPositiveReturn1D {
		return false
	}
	if item.rawStretchControl < -0.8 {
		return false
	}
	if !passesStockNewsGate(item, regime, "strict") {
		return false
	}
	if !passesNXTGate(item, regime, "strict") {
		return false
	}

	if item.hasLSTM {
		thresholds := resolveLSTMThresholds("strict", regime, tuning)
		if item.LSTMPredReturn1D < thresholds.MinPredReturn || item.LSTMProbUp < thresholds.MinProbUp || item.LSTMConfidence < thresholds.MinConfidence {
			return false
		}
	} else {
		if item.TrendScore < 54 || item.Return10D < -1 {
			return false
		}
		if regime.RiskOff && item.TrendScore < 58 {
			return false
		}
		if item.Return1D <= 0 && (item.rawCloseStrength < minCloseStrength+6 || item.rawTurnoverAcceleration < minTurnoverAccel+0.1) {
			return false
		}
	}

	return true
}

func filterRecommendedItems(items []RankItem, regime marketRegime, tuning lstmTuningProfile) []RankItem {
	if len(items) == 0 {
		return items
	}

	filtered := make([]RankItem, 0, len(items))
	for _, item := range items {
		if passesRecommendationGate(item, regime, tuning) {
			filtered = append(filtered, item)
		}
	}

	targetMinCount := 6
	maxRecommendedCount := 8
	if regime.RiskOff {
		targetMinCount = 4
		maxRecommendedCount = 6
	}
	if len(items) < targetMinCount {
		targetMinCount = len(items)
	}
	if len(filtered) >= targetMinCount {
		return filtered
	}

	seen := make(map[string]struct{}, len(filtered))
	for _, item := range filtered {
		seen[item.Market+":"+item.Code] = struct{}{}
	}

	appendFallback := func(candidate RankItem) {
		key := candidate.Market + ":" + candidate.Code
		if _, ok := seen[key]; ok {
			return
		}
		filtered = append(filtered, candidate)
		seen[key] = struct{}{}
	}

	for _, item := range items {
		if !passesRelaxedRecommendationGate(item, regime, tuning) {
			continue
		}
		appendFallback(item)
		if len(filtered) >= maxRecommendedCount {
			return filtered
		}
	}

	safetyCap := 4
	if regime.RiskOff {
		safetyCap = 3
	}
	if len(filtered) >= safetyCap {
		return filtered
	}

	for _, item := range items {
		if !passesSafetyFallbackGate(item, regime, tuning) {
			continue
		}
		appendFallback(item)
		if len(filtered) >= safetyCap {
			return filtered
		}
	}

	emergencyCap := 4
	if regime.RiskOff {
		emergencyCap = 3
	}
	for _, item := range items {
		if !passesEmergencyFallbackGate(item, regime) {
			continue
		}
		appendFallback(item)
		if len(filtered) >= emergencyCap {
			return filtered
		}
	}

	return filtered
}

func countLSTMBackedItems(items []RankItem) int {
	count := 0
	for _, item := range items {
		if item.hasLSTM {
			count++
		}
	}
	return count
}

func preferLSTMBackedRecommendations(current []RankItem, ranked []RankItem, regime marketRegime, tuning lstmTuningProfile, minimum int) []RankItem {
	if minimum <= 0 || countLSTMBackedItems(current) >= minimum {
		return current
	}

	lstmOnly := make([]RankItem, 0, len(ranked))
	for _, item := range ranked {
		if item.hasLSTM {
			lstmOnly = append(lstmOnly, item)
		}
	}
	if len(lstmOnly) < minimum {
		return current
	}

	preferred := filterRecommendedItems(lstmOnly, regime, tuning)
	if countLSTMBackedItems(preferred) >= minimum {
		return preferred
	}

	return lstmOnly
}

func passesRelaxedRecommendationGate(item RankItem, regime marketRegime, tuning lstmTuningProfile) bool {
	minTotalScore := 54.0
	minNextDayScore := 54.0
	minMomentumScore := 50.0
	minTurnoverNow := 0.025
	minTurnoverAvg := 0.018
	minCloseStrength := 52.0
	minTurnoverAccel := 0.85
	minFreshBreakout := -5.0
	maxVolatility20 := 6.5
	maxDrawdown60 := -26.0
	maxPositiveReturn1D := 6.5
	minNegativeReturn1D := -4.2
	minStabilityScore := 42.0

	if regime.RiskOff {
		minTotalScore = 62.0
		minNextDayScore = 63.0
		minMomentumScore = 58.0
		minTurnoverNow = 0.03
		minTurnoverAvg = 0.025
		minCloseStrength = 60.0
		minTurnoverAccel = 1.05
		minFreshBreakout = -2.2
		maxVolatility20 = 5.8
		maxDrawdown60 = -24.0
		maxPositiveReturn1D = 4.8
		minNegativeReturn1D = -3.0
		minStabilityScore = 50.0
	}

	if regime.NewsHighRiskOff {
		minTotalScore += 2
		minNextDayScore += 2
		minCloseStrength += 2
		minTurnoverAccel += 0.05
		maxVolatility20 -= 0.3
		maxPositiveReturn1D -= 0.5
		minStabilityScore += 2
	}

	if regime.NewsHighRiskOn && !regime.RiskOff {
		minNextDayScore -= 1
		minMomentumScore -= 1
		minCloseStrength -= 1
		minTurnoverAccel -= 0.05
	}

	if item.TotalScore < minTotalScore || item.NextDayScore < minNextDayScore || item.MomentumScore < minMomentumScore {
		return false
	}
	if item.TurnoverRatio < minTurnoverNow || item.AvgTurnover20D < minTurnoverAvg {
		return false
	}
	if item.rawCloseStrength < minCloseStrength {
		return false
	}
	if item.rawTurnoverAcceleration < minTurnoverAccel || item.rawFreshBreakout < minFreshBreakout {
		return false
	}
	if item.Volatility20D > maxVolatility20 {
		return false
	}
	if item.StabilityScore < minStabilityScore {
		return false
	}
	if item.Drawdown60D < maxDrawdown60 || item.Return20D < -14 || item.Return60D < -30 {
		return false
	}
	if item.Return5D < -3.0 || item.Return10D < -6 {
		return false
	}
	if item.Return1D < minNegativeReturn1D || item.Return1D > maxPositiveReturn1D {
		return false
	}
	if item.rawStretchControl < -1.4 {
		return false
	}
	if !passesStockNewsGate(item, regime, "relaxed") {
		return false
	}
	if !passesNXTGate(item, regime, "relaxed") {
		return false
	}

	if item.hasLSTM {
		thresholds := resolveLSTMThresholds("relaxed", regime, tuning)
		if item.LSTMProbUp < thresholds.MinProbUp || item.LSTMConfidence < thresholds.MinConfidence || item.LSTMPredReturn1D < thresholds.MinPredReturn {
			return false
		}
	} else if item.TrendScore < 50 {
		return false
	}

	return true
}

func passesSafetyFallbackGate(item RankItem, regime marketRegime, tuning lstmTuningProfile) bool {
	if item.TotalScore < 58 || item.NextDayScore < 58 {
		return false
	}
	maxVolatility20 := 6.8
	if regime.NewsHighRiskOff {
		maxVolatility20 = 6.2
	}
	if item.Volatility20D > maxVolatility20 {
		return false
	}
	if item.Drawdown60D < -28 || item.Return20D < -15 || item.Return10D < -6 {
		return false
	}
	if item.Return5D < -1 || item.Return1D < -3.5 || item.Return1D > 5.5 {
		return false
	}
	if item.TurnoverRatio < 0.02 || item.rawCloseStrength < 55 || item.rawTurnoverAcceleration < 0.85 {
		return false
	}
	if regime.NewsHighRiskOff && item.StabilityScore < 50 {
		return false
	}
	if !passesStockNewsGate(item, regime, "safety") {
		return false
	}
	if !passesNXTGate(item, regime, "safety") {
		return false
	}
	if item.hasLSTM {
		thresholds := resolveLSTMThresholds("safety", regime, tuning)
		if item.LSTMProbUp < thresholds.MinProbUp || item.LSTMConfidence < thresholds.MinConfidence || item.LSTMPredReturn1D < thresholds.MinPredReturn {
			return false
		}
	}
	return true
}

func passesEmergencyFallbackGate(item RankItem, regime marketRegime) bool {
	if item.TotalScore < 50 || item.NextDayScore < 52 {
		return false
	}
	if item.TurnoverRatio < 0.015 || item.rawCloseStrength < 45 {
		return false
	}
	if item.Volatility20D > 8.0 || item.Drawdown60D < -35 {
		return false
	}
	if item.Return1D < -5.0 || item.Return1D > 7.5 || item.Return10D < -9 {
		return false
	}
	if item.hasLSTM && item.LSTMProbUp < 48 {
		return false
	}
	if item.hasStockNews && item.NewsBias == "negative" {
		if item.NewsScore <= 24 {
			return false
		}
		if item.NewsNegativeCount >= 3 && item.NewsSentiment <= -45 {
			return false
		}
	}
	if regime.NewsHighRiskOff && item.StabilityScore < 44 {
		return false
	}
	if !passesNXTGate(item, regime, "emergency") {
		return false
	}
	return true
}

func passesStockNewsGate(item RankItem, regime marketRegime, mode string) bool {
	if !item.hasStockNews {
		return true
	}

	maxScore := 100.0
	maxNegativeSentiment := 0.0
	minNegativeArticles := 0

	switch mode {
	case "strict":
		maxScore = 34
		maxNegativeSentiment = -24
		minNegativeArticles = 2
		if regime.RiskOff {
			maxScore = 40
			maxNegativeSentiment = -18
		}
		if regime.NewsHighRiskOff {
			maxScore += 3
			maxNegativeSentiment += 4
		}
	case "relaxed":
		maxScore = 28
		maxNegativeSentiment = -38
		minNegativeArticles = 2
		if regime.RiskOff {
			maxScore = 34
			maxNegativeSentiment = -30
		}
	case "safety":
		maxScore = 22
		maxNegativeSentiment = -48
		minNegativeArticles = 3
	}

	if item.NewsBias == "negative" && item.NewsScore <= maxScore {
		return false
	}
	if item.NewsBias == "negative" && item.NewsNegativeCount >= minNegativeArticles && item.NewsSentiment <= maxNegativeSentiment {
		return false
	}
	return true
}

func passesNXTGate(item RankItem, regime marketRegime, mode string) bool {
	if !item.hasNXT {
		return true
	}

	minScore := 0.0
	minCloseStrength := 0.0
	minReturn := -999.0
	minImpulse := 0.0
	switch mode {
	case "strict":
		minScore = 43
		minCloseStrength = 44
		minReturn = -1.8
		minImpulse = 0.55
	case "relaxed":
		minScore = 38
		minCloseStrength = 40
		minReturn = -2.2
		minImpulse = 0.45
	case "safety":
		minScore = 34
		minCloseStrength = 38
		minReturn = -2.6
		minImpulse = 0.35
	case "emergency":
		minScore = 26
		minCloseStrength = 32
		minReturn = -3.8
		minImpulse = 0
	default:
		return true
	}

	if regime.NewsHighRiskOff {
		minScore += 2
		minCloseStrength += 3
		minReturn += 0.4
	}
	if regime.NewsHighRiskOn && !regime.RiskOff {
		minScore -= 1
		minCloseStrength -= 1
	}

	if item.NXTScore < minScore {
		return false
	}
	if item.rawNXTCloseStrength < minCloseStrength {
		return false
	}
	if item.rawNXTChangeRate < minReturn {
		return false
	}
	if minImpulse > 0 && item.rawNXTTradeValueImpulse < minImpulse && item.rawNXTChangeRate < 0 {
		return false
	}
	return true
}

func percentileScores(values []float64) []float64 {
	out := make([]float64, len(values))
	type pair struct {
		idx int
		val float64
	}
	valid := make([]pair, 0, len(values))
	for idx, value := range values {
		if !isFinite(value) {
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
		avgRank := float64(start+end-1) / 2
		score := avgRank / float64(len(valid)-1) * 100
		for i := start; i < end; i++ {
			out[valid[i].idx] = score
		}
		start = end
	}

	return out
}

func headerIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")] = i
	}
	return idx
}

func getCell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func loadStockFromCSV(path string, market string, minMarketCap int64) (*RankItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idx := headerIndex(header)
	required := []string{"BAS_DD", "TDD_CLSPRC", "TDD_OPNPRC", "TDD_HGPRC", "TDD_LWPRC", "ACC_TRDVAL", "MKTCAP", "ISU_CD", "ISU_NM"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing column %s", col)
		}
	}

	closes := make([]float64, 0, 512)
	opens := make([]float64, 0, 512)
	highs := make([]float64, 0, 512)
	lows := make([]float64, 0, 512)
	turnovers := make([]float64, 0, 512)

	var (
		latestDate     string
		latestClose    float64
		latestTurnover float64
		latestMktCap   int64
		code           string
		name           string
	)

	for {
		row, readErr := r.Read()
		if readErr != nil {
			break
		}
		closePx := parseFloat(getCell(row, idx["TDD_CLSPRC"]))
		if closePx <= 0 {
			continue
		}
		d := strings.TrimSpace(getCell(row, idx["BAS_DD"]))
		if d == "" {
			continue
		}
		closes = append(closes, closePx)
		opens = append(opens, parseFloat(getCell(row, idx["TDD_OPNPRC"])))
		highs = append(highs, parseFloat(getCell(row, idx["TDD_HGPRC"])))
		lows = append(lows, parseFloat(getCell(row, idx["TDD_LWPRC"])))
		turnovers = append(turnovers, parseFloat(getCell(row, idx["ACC_TRDVAL"])))

		latestDate = d
		latestClose = closePx
		latestTurnover = turnovers[len(turnovers)-1]
		latestMktCap = parseInt64(getCell(row, idx["MKTCAP"]))
		code = strings.TrimSpace(getCell(row, idx["ISU_CD"]))
		name = strings.TrimSpace(getCell(row, idx["ISU_NM"]))
	}

	n := len(closes)
	if n < 121 {
		return nil, fmt.Errorf("insufficient history")
	}
	if latestMktCap < minMarketCap {
		return nil, fmt.Errorf("below min market cap")
	}

	ret1 := pctChange(closes[n-1], closes[n-2])
	ret3 := pctChange(closes[n-1], closes[n-4])
	ret5 := pctChange(closes[n-1], closes[n-6])
	ret10 := pctChange(closes[n-1], closes[n-11])
	ret20 := pctChange(closes[n-1], closes[n-21])
	ret60 := pctChange(closes[n-1], closes[n-61])
	ret120 := pctChange(closes[n-1], closes[n-121])

	returns20 := dailyReturnsWindow(closes, 20)
	returns60 := dailyReturnsWindow(closes, 60)
	vol20 := stddev(returns20)
	vol60 := stddev(returns60)
	downVol20 := downsideStddev(returns20)
	drawdown60 := maxDrawdown(closes, 60)
	drawdown120 := maxDrawdown(closes, 120)
	sma20 := movingAverage(closes, 20)
	sma60 := movingAverage(closes, 60)
	sma120 := movingAverage(closes, 120)
	sma5 := movingAverage(closes, 5)
	maGap20 := pctChange(closes[n-1], sma20)
	maGap60 := pctChange(closes[n-1], sma60)
	maGap120 := pctChange(closes[n-1], sma120)
	trendAlignment := 0.4*maGap20 + 0.35*maGap60 + 0.25*maGap120
	upRatio20 := positiveRatio(returns20)
	upRatio60 := positiveRatio(returns60)
	trendQuality := 0.6*(0.6*upRatio20+0.4*upRatio60) + 0.4*efficiencyRatio(closes, 20)
	riskAdjusted := 0.55*(ret20/math.Max(vol20, 0.25)) + 0.45*(ret60/math.Max(vol60, 0.25))

	turnoverRatio := 0.0
	if latestMktCap > 0 {
		turnoverRatio = latestTurnover / float64(latestMktCap) * 100
	}
	turnoverTail20 := tailSlice(turnovers, 20)
	avgTurnover20 := mean(turnoverTail20)
	avgTurnoverRatio20 := 0.0
	avgTurnoverRatio20Prev := 0.0
	turnoverRatios20 := make([]float64, 0, len(turnoverTail20))
	turnoverRatiosAll := make([]float64, 0, len(turnovers))
	if latestMktCap > 0 {
		avgTurnoverRatio20 = avgTurnover20 / float64(latestMktCap) * 100
		for _, value := range turnoverTail20 {
			turnoverRatios20 = append(turnoverRatios20, value/float64(latestMktCap)*100)
		}
		for _, value := range turnovers {
			turnoverRatiosAll = append(turnoverRatiosAll, value/float64(latestMktCap)*100)
		}
		avgTurnoverRatio20Prev = mean(tailSlice(turnoverRatiosAll[:len(turnoverRatiosAll)-1], 20))
	}
	liquidityStability := -coefficientOfVariation(turnoverRatios20)
	avgTurnoverRatio5Prev := mean(tailSlice(turnoverRatiosAll[:len(turnoverRatiosAll)-1], 5))
	avgTurnoverRatio3Prev := mean(tailSlice(turnoverRatiosAll[:len(turnoverRatiosAll)-1], 3))

	latestHigh := highs[n-1]
	latestLow := lows[n-1]
	latestRange := 0.0
	if latestClose > 0 {
		latestRange = (latestHigh - latestLow) / latestClose * 100
	}
	prevClose := closes[n-2]
	gapFromPrevClose := pctChange(opens[n-1], prevClose)
	intradayReturn := pctChange(latestClose, opens[n-1])
	closeStrength := 50.0
	if latestHigh > latestLow {
		closeStrength = (latestClose - latestLow) / (latestHigh - latestLow) * 100
	}
	closeStrengthBias := (closeStrength - 50) / 10
	rangePercents := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		if closes[i] <= 0 {
			continue
		}
		rangePercents = append(rangePercents, (highs[i]-lows[i])/closes[i]*100)
	}
	avgRange20 := 0.0
	for i := maxInt(0, n-20); i < n; i++ {
		if closes[i] <= 0 {
			continue
		}
		rangePct := (highs[i] - lows[i]) / closes[i] * 100
		avgRange20 += rangePct
	}
	avgRange20 /= float64(len(tailSlice(closes, 20)))
	rangeRatio := safeRatio(latestRange, math.Max(avgRange20, 0.01))
	avgRange20Prev := mean(tailSlice(rangePercents[:len(rangePercents)-1], 20))
	avgRange5Prev := mean(tailSlice(rangePercents[:len(rangePercents)-1], 5))
	turnoverRatioVsAvg20 := safeRatio(turnoverRatio, math.Max(avgTurnoverRatio20Prev, 0.01))
	turnoverRatioVsAvg5 := safeRatio(turnoverRatio, math.Max(avgTurnoverRatio5Prev, 0.01))
	turnoverRatioVsAvg3 := safeRatio(turnoverRatio, math.Max(avgTurnoverRatio3Prev, 0.01))
	turnoverAcceleration := 0.55*turnoverRatioVsAvg5 + 0.30*turnoverRatioVsAvg20 + 0.15*turnoverRatioVsAvg3
	previousHigh20 := maxFloat(tailSlice(highs[:n-1], 20))
	previousHigh60 := maxFloat(tailSlice(highs[:n-1], 60))
	breakoutPressure := -math.Abs(pctChange(latestClose, math.Max(previousHigh20, 0.01)))
	freshBreakout := 0.65*pctChange(latestClose, math.Max(previousHigh20, 0.01)) +
		0.35*pctChange(latestClose, math.Max(previousHigh60, 0.01)) -
		0.20*math.Max(0, gapFromPrevClose-2.0)
	intradayFollowThrough := 0.45*intradayReturn +
		0.25*ret1 +
		0.20*closeStrengthBias -
		0.10*math.Max(0, gapFromPrevClose-1.5)
	compressionSetup := 0.60*(-safeRatio(avgRange5Prev, math.Max(avgRange20Prev, 0.01))) +
		0.25*closeStrengthBias +
		0.15*math.Min(turnoverAcceleration, 4)
	bounceQuality := 0.30*ret5 +
		0.20*ret10 +
		0.25*intradayReturn +
		0.15*closeStrengthBias +
		0.10*turnoverAcceleration -
		0.20*math.Abs(gapFromPrevClose)
	nextDayMomentum := 0.50*ret1 + 0.30*ret3 + 0.20*ret5
	stretchControl := -0.45*math.Max(0, ret1-4.0) -
		0.30*math.Max(0, pctChange(latestClose, sma5)-4.0) -
		0.25*math.Max(0, latestRange-5.0)
	volumeImpulse := 0.60*turnoverAcceleration + 0.40*rangeRatio

	return &RankItem{
		Market:                   market,
		Code:                     code,
		Name:                     name,
		AsOf:                     latestDate,
		Close:                    latestClose,
		MarketCap:                latestMktCap,
		Turnover:                 latestTurnover,
		TurnoverRatio:            turnoverRatio,
		Return1D:                 ret1,
		Return5D:                 ret5,
		Return10D:                ret10,
		Return20D:                ret20,
		Return60D:                ret60,
		Return120D:               ret120,
		Volatility20D:            vol20,
		Volatility60D:            vol60,
		DownsideVol20D:           downVol20,
		Drawdown60D:              drawdown60,
		Drawdown120D:             drawdown120,
		AvgTurnover20D:           avgTurnoverRatio20,
		rawShortMomentum:         0.6*ret5 + 0.4*ret10,
		rawMomentum20:            ret20,
		rawMomentum60:            ret60,
		rawMomentum120:           ret120,
		rawRiskAdjusted:          riskAdjusted,
		rawTrendAlignment:        trendAlignment,
		rawTrendQuality:          trendQuality,
		rawLiquidityNow:          turnoverRatio,
		rawLiquidityAverage:      avgTurnoverRatio20,
		rawLiquidityStability:    liquidityStability,
		rawLiquiditySize:         math.Log1p(float64(latestMktCap)),
		rawStability20:           -vol20,
		rawStability60:           -vol60,
		rawStabilityDownside:     -downVol20,
		rawDrawdown60:            drawdown60,
		rawDrawdown120:           drawdown120,
		rawNextDayMomentum:       nextDayMomentum,
		rawCloseStrength:         closeStrength,
		rawVolumeImpulse:         volumeImpulse,
		rawTurnoverAcceleration:  turnoverAcceleration,
		rawBreakoutPressure:      breakoutPressure,
		rawFreshBreakout:         freshBreakout,
		rawIntradayFollowThrough: intradayFollowThrough,
		rawCompressionSetup:      compressionSetup,
		rawStretchControl:        stretchControl,
		rawBounceQuality:         bounceQuality,
		rawLSTMReturn1D:          math.NaN(),
		rawNXTChangeRate:         math.NaN(),
		rawNXTIntradayReturn:     math.NaN(),
		rawNXTTradeValueRatio:    math.NaN(),
		rawNXTTradeValueImpulse:  math.NaN(),
		rawNXTCloseStrength:      math.NaN(),
		rawLSTMReturn5D:          math.NaN(),
		rawLSTMReturn20D:         math.NaN(),
		rawLSTMProbability:       math.NaN(),
		rawLSTMConfidence:        math.NaN(),
		rawLSTMValidationAcc:     math.NaN(),
		rawLSTMBrierQuality:      math.NaN(),
	}, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func directoryForMarket(dataRoot, market string) string {
	switch market {
	case "KOSPI":
		return filepath.Join(dataRoot, "kospi_daily")
	case "KOSDAQ":
		return filepath.Join(dataRoot, "kosdaq_daily")
	default:
		return ""
	}
}

func cacheKey(market string, minMarketCap int64) string {
	return fmt.Sprintf("%s|%d", market, minMarketCap)
}

func scoreModelVersion(snapshot lstmPredictionSnapshot, appliedCount int) string {
	if !snapshot.Enabled || appliedCount == 0 {
		return baseScoreModelVersion
	}
	version := strings.TrimSpace(snapshot.ModelVersion)
	if version == "" {
		version = defaultLSTMModelVersion
	}
	return baseScoreModelVersion + "+" + version
}

func extractFactorValues(items []RankItem, selector func(RankItem) float64) []float64 {
	values := make([]float64, len(items))
	for i, item := range items {
		values[i] = selector(item)
	}
	return values
}

func shouldReplaceDuplicate(existing, candidate RankItem) bool {
	if candidate.AsOf > existing.AsOf {
		return true
	}
	if candidate.AsOf < existing.AsOf {
		return false
	}
	if candidate.MarketCap > existing.MarketCap {
		return true
	}
	if candidate.MarketCap < existing.MarketCap {
		return false
	}
	if candidate.Turnover > existing.Turnover {
		return true
	}
	if candidate.Turnover < existing.Turnover {
		return false
	}
	return candidate.Name != "" && existing.Name == ""
}

func dedupeByMarketCode(items []RankItem) []RankItem {
	if len(items) <= 1 {
		return items
	}

	out := make([]RankItem, 0, len(items))
	indexByKey := make(map[string]int, len(items))

	for _, item := range items {
		market := strings.TrimSpace(item.Market)
		code := strings.TrimSpace(item.Code)
		name := strings.TrimSpace(item.Name)

		key := market + ":" + code
		if code == "" {
			key = market + ":" + name
		}

		if idx, exists := indexByKey[key]; exists {
			if shouldReplaceDuplicate(out[idx], item) {
				out[idx] = item
			}
			continue
		}

		indexByKey[key] = len(out)
		out = append(out, item)
	}

	return out
}

func filterToLatestDate(items []RankItem) []RankItem {
	if len(items) == 0 {
		return items
	}

	latestDate := ""
	for _, item := range items {
		if item.AsOf > latestDate {
			latestDate = item.AsOf
		}
	}
	if latestDate == "" {
		return []RankItem{}
	}

	filtered := make([]RankItem, 0, len(items))
	for _, item := range items {
		if item.AsOf != latestDate {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (s *Service) computeRanks(market string, minMarketCap int64) (rankResult, error) {
	market = sanitizeMarket(market)
	key := cacheKey(market, minMarketCap)
	now := time.Now()

	s.mu.RLock()
	cached, ok := s.cache[key]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) {
		return cached.result, nil
	}

	targetMarkets := []string{"KOSPI", "KOSDAQ"}
	if market == "KOSPI" || market == "KOSDAQ" {
		targetMarkets = []string{market}
	}

	items := make([]RankItem, 0, 1024)
	for _, m := range targetMarkets {
		dir := directoryForMarket(s.cfg.DataRootDir, m)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
				continue
			}
			item, loadErr := loadStockFromCSV(filepath.Join(dir, entry.Name()), m, minMarketCap)
			if loadErr != nil || item == nil {
				continue
			}
			items = append(items, *item)
		}
	}
	items = filterToLatestDate(items)
	items = dedupeByMarketCode(items)

	if len(items) == 0 {
		return rankResult{}, fmt.Errorf("퀀트 분석 대상 데이터가 없습니다. (market=%s, min_market_cap=%d)", market, minMarketCap)
	}

	asOfMin, asOfMax := "99999999", "00000000"
	for i := range items {
		if items[i].AsOf < asOfMin {
			asOfMin = items[i].AsOf
		}
		if items[i].AsOf > asOfMax {
			asOfMax = items[i].AsOf
		}
	}

	nxtSnapshot, nxtErr := s.loadNXTSnapshot(asOfMax)
	if nxtErr != nil {
		nxtSnapshot = nxtSnapshotState{}
	}

	lstmSnapshot, lstmErr := s.loadLSTMPredictions()
	if lstmErr != nil {
		lstmSnapshot = lstmPredictionSnapshot{Weight: s.cfg.LSTMWeight}
	}
	lstmTuning, lstmTuningErr := s.loadLSTMTuning()
	if lstmTuningErr == nil {
		lstmSnapshot.Weight = effectiveLSTMWeight(lstmSnapshot.Weight, lstmTuning)
	}
	lstmAppliedCount := 0
	nxtAppliedCount := 0
	newsStockAppliedCount := 0
	var newsRegime *news.MarketNewsRegime
	var stockNewsSignals *news.StockNewsSignalsSnapshot

	for i := range items {
		it := &items[i]
		if quote, ok := nxtSnapshot.find(it.Market, it.Code); ok {
			intradayReturn := pctChange(quote.CurrentPrice, quote.OpenPrice)
			tradeValueRatio := 0.0
			if it.MarketCap > 0 {
				tradeValueRatio = float64(quote.TradeValue) / float64(it.MarketCap) * 100
			}
			tradeValueImpulse := safeRatio(tradeValueRatio, math.Max(it.AvgTurnover20D, 0.01))
			closeStrength := 50.0
			if quote.HighPrice > quote.LowPrice {
				closeStrength = (quote.CurrentPrice - quote.LowPrice) / (quote.HighPrice - quote.LowPrice) * 100
			}
			it.NXTPrice = round2(quote.CurrentPrice)
			it.NXTReturn = round2(quote.ChangeRate)
			it.NXTIntradayReturn = round2(intradayReturn)
			it.NXTTradeValueRatio = round2(tradeValueRatio)
			it.NXTCloseStrength = round2(closeStrength)
			it.rawNXTChangeRate = quote.ChangeRate
			it.rawNXTIntradayReturn = intradayReturn
			it.rawNXTTradeValueRatio = tradeValueRatio
			it.rawNXTTradeValueImpulse = tradeValueImpulse
			it.rawNXTCloseStrength = closeStrength
			it.hasNXT = isFinite(quote.ChangeRate) && isFinite(intradayReturn) && isFinite(closeStrength)
			if it.hasNXT {
				nxtAppliedCount++
			}
		}
		if prediction, ok := lstmSnapshot.find(it.Market, it.Code, it.AsOf); ok {
			it.LSTMPredReturn1D = round2(prediction.PredReturn1D)
			it.LSTMPredReturn5D = round2(prediction.PredReturn5D)
			it.LSTMPredReturn20D = round2(prediction.PredReturn20D)
			it.LSTMProbUp = round2(prediction.ProbUp * 100)
			it.LSTMConfidence = round2(prediction.Confidence * 100)
			it.rawLSTMReturn1D = prediction.PredReturn1D
			it.rawLSTMReturn5D = prediction.PredReturn5D
			it.rawLSTMReturn20D = prediction.PredReturn20D
			it.rawLSTMProbability = prediction.ProbUp
			it.rawLSTMConfidence = prediction.Confidence
			it.rawLSTMValidationAcc = prediction.ValidationAccuracy1D
			it.rawLSTMBrierQuality = 1 - prediction.ValidationBrier1D
			it.hasLSTM = isFinite(prediction.PredReturn1D) && isFinite(prediction.ProbUp) && isFinite(prediction.Confidence)
			if it.hasLSTM {
				lstmAppliedCount++
			}
		}
	}

	newsService := s.news
	if newsService == nil {
		newsService = news.NewService(s.cfg)
		s.news = newsService
	}
	if regimeSnapshot, err := newsService.BuildMarketNewsRegime(""); err == nil {
		newsRegime = &regimeSnapshot
	}
	if stockSignalSnapshot, err := newsService.BuildStockNewsSignals(""); err == nil {
		stockNewsSignals = &stockSignalSnapshot
	}

	for i := range items {
		it := &items[i]
		if stockNewsSignals == nil {
			continue
		}
		signal, ok := stockNewsSignals.Lookup(it.Name)
		if !ok {
			continue
		}
		it.NewsScore = round2(signal.Score)
		it.NewsSentiment = round2(signal.Sentiment * 100)
		it.NewsBuzz = round2(signal.Buzz * 100)
		it.NewsBias = strings.TrimSpace(signal.Bias)
		it.NewsArticleCount = signal.ArticleCount
		it.NewsPositiveCount = signal.PositiveCount
		it.NewsNegativeCount = signal.NegativeCount
		it.rawNewsScore = signal.Score
		it.hasStockNews = true
		newsStockAppliedCount++
	}

	shortMomentumScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawShortMomentum }))
	momentum20Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawMomentum20 }))
	momentum60Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawMomentum60 }))
	momentum120Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawMomentum120 }))
	riskAdjustedScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawRiskAdjusted }))
	trendAlignmentScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawTrendAlignment }))
	trendQualityScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawTrendQuality }))
	liquidityNowScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLiquidityNow }))
	liquidityAverageScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLiquidityAverage }))
	liquidityStabilityScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLiquidityStability }))
	liquiditySizeScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLiquiditySize }))
	stability20Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawStability20 }))
	stability60Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawStability60 }))
	stabilityDownsideScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawStabilityDownside }))
	drawdown60Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawDrawdown60 }))
	drawdown120Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawDrawdown120 }))
	nextDayMomentumScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNextDayMomentum }))
	closeStrengthScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawCloseStrength }))
	volumeImpulseScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawVolumeImpulse }))
	turnoverAccelerationScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawTurnoverAcceleration }))
	breakoutPressureScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawBreakoutPressure }))
	freshBreakoutScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawFreshBreakout }))
	intradayFollowThroughScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawIntradayFollowThrough }))
	compressionSetupScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawCompressionSetup }))
	stretchControlScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawStretchControl }))
	bounceQualityScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawBounceQuality }))
	nxtChangeRateScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNXTChangeRate }))
	nxtIntradayScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNXTIntradayReturn }))
	nxtTradeValueRatioScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNXTTradeValueRatio }))
	nxtTradeValueImpulseScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNXTTradeValueImpulse }))
	nxtCloseStrengthScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawNXTCloseStrength }))
	lstmReturn1Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMReturn1D }))
	lstmReturn5Scores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMReturn5D }))
	lstmProbabilityScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMProbability }))
	lstmConfidenceScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMConfidence }))
	lstmValidationAccScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMValidationAcc }))
	lstmBrierQualityScores := percentileScores(extractFactorValues(items, func(item RankItem) float64 { return item.rawLSTMBrierQuality }))

	for i := range items {
		it := &items[i]
		it.NextDayScore = round2(
			0.17*intradayFollowThroughScores[i] +
				0.16*freshBreakoutScores[i] +
				0.14*turnoverAccelerationScores[i] +
				0.10*closeStrengthScores[i] +
				0.11*nextDayMomentumScores[i] +
				0.10*stretchControlScores[i] +
				0.08*compressionSetupScores[i] +
				0.07*bounceQualityScores[i] +
				0.04*volumeImpulseScores[i] +
				0.03*breakoutPressureScores[i],
		)
		it.TrendScore = round2(
			0.42*trendAlignmentScores[i] +
				0.30*trendQualityScores[i] +
				0.16*momentum20Scores[i] +
				0.08*momentum60Scores[i] +
				0.04*momentum120Scores[i],
		)
		it.RiskAdjScore = round2(riskAdjustedScores[i])
		it.MomentumScore = round2(
			0.26*nextDayMomentumScores[i] +
				0.20*shortMomentumScores[i] +
				0.14*freshBreakoutScores[i] +
				0.10*intradayFollowThroughScores[i] +
				0.10*trendAlignmentScores[i] +
				0.08*riskAdjustedScores[i] +
				0.07*trendQualityScores[i] +
				0.05*momentum20Scores[i],
		)
		it.LiquidityScore = round2(
			0.25*liquidityNowScores[i] +
				0.35*liquidityAverageScores[i] +
				0.20*liquidityStabilityScores[i] +
				0.20*liquiditySizeScores[i],
		)
		it.StabilityScore = round2(
			0.25*stability20Scores[i] +
				0.20*stability60Scores[i] +
				0.20*stabilityDownsideScores[i] +
				0.20*drawdown60Scores[i] +
				0.15*drawdown120Scores[i],
		)
		if it.hasNXT {
			it.NXTScore = round2(
				0.30*nxtChangeRateScores[i] +
					0.18*nxtIntradayScores[i] +
					0.14*nxtTradeValueRatioScores[i] +
					0.20*nxtTradeValueImpulseScores[i] +
					0.18*nxtCloseStrengthScores[i],
			)
		}
		if it.hasLSTM {
			it.LSTMScore = round2(
				0.40*lstmReturn1Scores[i] +
					0.05*lstmReturn5Scores[i] +
					0.28*lstmProbabilityScores[i] +
					0.09*lstmConfidenceScores[i] +
					0.10*lstmValidationAccScores[i] +
					0.08*lstmBrierQualityScores[i],
			)
		}
		baseTotal := 0.55*it.NextDayScore + 0.15*it.MomentumScore + 0.10*it.TrendScore + 0.10*it.LiquidityScore + 0.10*it.StabilityScore
		total := baseTotal
		if it.hasStockNews {
			total += stockNewsScoreWeight * (it.NewsScore - 50)
		}
		if it.hasNXT {
			total += nxtScoreWeight * (it.NXTScore - 50)
		}
		if it.hasLSTM && lstmSnapshot.Enabled && lstmSnapshot.Weight > 0 {
			total += lstmSnapshot.Weight * (it.LSTMScore - 50)
		}
		it.TotalScore = round2(total)
		it.NextDayScore = round2(it.NextDayScore)
		it.Return1D = round2(it.Return1D)
		it.Return5D = round2(it.Return5D)
		it.Return10D = round2(it.Return10D)
		it.Return20D = round2(it.Return20D)
		it.Return60D = round2(it.Return60D)
		it.Return120D = round2(it.Return120D)
		it.Volatility20D = round2(it.Volatility20D)
		it.Volatility60D = round2(it.Volatility60D)
		it.DownsideVol20D = round2(it.DownsideVol20D)
		it.Drawdown60D = round2(it.Drawdown60D)
		it.Drawdown120D = round2(it.Drawdown120D)
		it.TurnoverRatio = round2(it.TurnoverRatio)
		it.AvgTurnover20D = round2(it.AvgTurnover20D)
	}

	regime := detectMarketRegime(items, newsRegime)
	for i := range items {
		it := &items[i]
		if newsRegime != nil && newsRegime.Confidence >= 0.22 {
			riskOffPenalty := math.Max(0, newsRegime.RiskOffProb-0.5) * newsRegime.Confidence * 18
			riskOnBonus := math.Max(0, newsRegime.RiskOnProb-0.5) * newsRegime.Confidence * 12
			if regime.NewsHighRiskOff {
				exposure := 0.45*clampFloat((it.Volatility20D-3.3)/3.8, 0, 1) +
					0.30*clampFloat((56-it.TrendScore)/24, 0, 1) +
					0.25*clampFloat(-it.Return5D/6, 0, 1)
				it.TotalScore = round2(math.Max(0, it.TotalScore-riskOffPenalty*exposure))
			} else if regime.NewsHighRiskOn {
				impulse := 0.45*clampFloat((it.NextDayScore-55)/35, 0, 1) +
					0.30*clampFloat((it.MomentumScore-50)/35, 0, 1) +
					0.25*clampFloat((it.Return5D+2)/10, 0, 1)
				it.TotalScore = round2(it.TotalScore + riskOnBonus*impulse)
			}
		}
		if it.hasStockNews {
			switch {
			case it.NewsBias == "negative":
				penalty := clampFloat((52-it.NewsScore)/4.5, 0, 8)
				if it.NewsNegativeCount >= 2 {
					penalty += 2
				}
				it.TotalScore = round2(math.Max(0, it.TotalScore-penalty))
			case it.NewsBias == "positive":
				bonus := clampFloat((it.NewsScore-50)/7.5, 0, 4)
				if !regime.RiskOff {
					bonus += clampFloat((it.NewsBuzz-55)/20, 0, 1.5)
				}
				it.TotalScore = round2(it.TotalScore + bonus)
			}
		}
		if it.hasNXT {
			switch {
			case it.NXTReturn <= -2.2 && it.NXTCloseStrength <= 35:
				it.TotalScore = round2(math.Max(0, it.TotalScore-6))
			case it.NXTReturn <= -1.2 && it.NXTCloseStrength <= 42:
				it.TotalScore = round2(math.Max(0, it.TotalScore-3.5))
			case it.NXTReturn >= 1.2 && it.NXTCloseStrength >= 62 && it.rawNXTTradeValueImpulse >= 1.1:
				it.TotalScore = round2(it.TotalScore + 2.5)
			}
		}
		if !regime.RiskOff {
			continue
		}
		penalty := 0.0
		if it.Return1D <= 0 {
			penalty += 5
		}
		if it.Return5D <= 0 {
			penalty += 4
		}
		if it.rawCloseStrength < 60 {
			penalty += 3
		}
		if it.TrendScore < 58 {
			penalty += 3
		}
		if it.hasLSTM {
			if it.LSTMPredReturn1D <= 0 || it.LSTMProbUp < 57 || it.LSTMConfidence < 54 {
				penalty += 7
			}
		} else {
			penalty += 4
		}
		it.TotalScore = round2(math.Max(0, it.TotalScore-penalty))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalScore != items[j].TotalScore {
			return items[i].TotalScore > items[j].TotalScore
		}
		if items[i].NextDayScore != items[j].NextDayScore {
			return items[i].NextDayScore > items[j].NextDayScore
		}
		if items[i].LSTMProbUp != items[j].LSTMProbUp {
			return items[i].LSTMProbUp > items[j].LSTMProbUp
		}
		if items[i].NXTScore != items[j].NXTScore {
			return items[i].NXTScore > items[j].NXTScore
		}
		if items[i].NewsScore != items[j].NewsScore {
			return items[i].NewsScore > items[j].NewsScore
		}
		if items[i].MomentumScore != items[j].MomentumScore {
			return items[i].MomentumScore > items[j].MomentumScore
		}
		if items[i].rawCloseStrength != items[j].rawCloseStrength {
			return items[i].rawCloseStrength > items[j].rawCloseStrength
		}
		if items[i].StabilityScore != items[j].StabilityScore {
			return items[i].StabilityScore > items[j].StabilityScore
		}
		return items[i].MarketCap > items[j].MarketCap
	})

	universeCount := len(items)
	rankedItems := append([]RankItem(nil), items...)
	items = filterRecommendedItems(items, regime, lstmTuning)
	if lstmSnapshot.Enabled && !lstmSnapshot.CoverageSufficient {
		items = preferLSTMBackedRecommendations(items, rankedItems, regime, lstmTuning, 3)
	}
	for i := range items {
		items[i].Rank = i + 1
	}

	result := rankResult{
		GeneratedAt:         now.Format(time.RFC3339),
		ScoreModel:          scoreModelVersion(lstmSnapshot, lstmAppliedCount),
		Market:              market,
		MinMarketCap:        minMarketCap,
		UniverseCount:       universeCount,
		AsOfMin:             asOfMin,
		AsOfMax:             asOfMax,
		Items:               items,
		NXTEnabled:          nxtSnapshot.Enabled && nxtAppliedCount > 0,
		NXTTradingDate:      strings.TrimSpace(nxtSnapshot.TradingDate),
		NXTSetTime:          strings.TrimSpace(nxtSnapshot.SetTime),
		NXTQuoteCount:       nxtSnapshot.QuoteCount,
		NXTAppliedCount:     nxtAppliedCount,
		LSTMEnabled:         lstmSnapshot.Enabled && lstmAppliedCount > 0,
		LSTMModelVersion:    strings.TrimSpace(lstmSnapshot.ModelVersion),
		LSTMWeight:          round2(lstmSnapshot.Weight),
		LSTMPredictionAsOf:  strings.TrimSpace(lstmSnapshot.PredictionAsOf),
		LSTMPredictionCount: lstmSnapshot.ItemCount,
		LSTMAppliedCount:    lstmAppliedCount,
		LSTMTuningMode: func() string {
			if !lstmTuning.Enabled {
				return ""
			}
			return lstmTuning.Mode
		}(),
		LSTMTuningWeightMult: func() float64 {
			if !lstmTuning.Enabled {
				return 0
			}
			return round2(lstmTuning.WeightMultiplier)
		}(),
		LSTMTuningRecentHit: func() float64 {
			if !lstmTuning.Enabled {
				return 0
			}
			return round2(lstmTuning.Recent20.DirectionHitProbRate * 100)
		}(),
		LSTMTuningRecentTopK: func() float64 {
			if !lstmTuning.Enabled {
				return 0
			}
			return round2(lstmTuning.Recent20.TopKHitRate * 100)
		}(),
		LSTMTuningEvalCount: func() int {
			if !lstmTuning.Enabled {
				return 0
			}
			return lstmTuning.Recent20.EvaluatedCount
		}(),
		NewsStockAppliedCount: newsStockAppliedCount,
		NewsTargetTradingDate: func() string {
			if newsRegime == nil {
				return ""
			}
			return strings.TrimSpace(newsRegime.TargetTradingDate)
		}(),
		NewsRiskOnProb: func() float64 {
			if newsRegime == nil {
				return 0
			}
			return round2(newsRegime.RiskOnProb * 100)
		}(),
		NewsRiskOffProb: func() float64 {
			if newsRegime == nil {
				return 0
			}
			return round2(newsRegime.RiskOffProb * 100)
		}(),
		NewsNeutralProb: func() float64 {
			if newsRegime == nil {
				return 0
			}
			return round2(newsRegime.NeutralProb * 100)
		}(),
		NewsRegimeConfidence: func() float64 {
			if newsRegime == nil {
				return 0
			}
			return round2(newsRegime.Confidence * 100)
		}(),
		NewsRegimeBias: func() string {
			if newsRegime == nil {
				return ""
			}
			return strings.TrimSpace(newsRegime.Bias)
		}(),
		NewsTimePrecision: func() string {
			if newsRegime == nil {
				return ""
			}
			return strings.TrimSpace(newsRegime.TimePrecision)
		}(),
	}

	s.mu.Lock()
	s.cache[key] = cacheEntry{
		expiresAt: now.Add(cacheTTL),
		result:    result,
	}
	s.mu.Unlock()

	return result, nil
}

func (s *Service) Rank(market string, limit int, minMarketCap int64) (map[string]any, error) {
	if limit <= 0 {
		limit = defaultRankLimit
	}
	if limit > maxRankLimit {
		limit = maxRankLimit
	}
	minMarketCap = sanitizeMinMarketCap(minMarketCap)

	result, err := s.computeRanks(market, minMarketCap)
	if err != nil {
		return nil, err
	}

	items := result.Items
	if limit < len(items) {
		items = items[:limit]
	}

	return map[string]any{
		"generated_at":                       result.GeneratedAt,
		"score_model":                        result.ScoreModel,
		"market":                             result.Market,
		"min_market_cap":                     result.MinMarketCap,
		"limit":                              limit,
		"universe_count":                     result.UniverseCount,
		"as_of_min":                          result.AsOfMin,
		"as_of_max":                          result.AsOfMax,
		"nxt_enabled":                        result.NXTEnabled,
		"nxt_trading_date":                   result.NXTTradingDate,
		"nxt_set_time":                       result.NXTSetTime,
		"nxt_quote_count":                    result.NXTQuoteCount,
		"nxt_applied_count":                  result.NXTAppliedCount,
		"lstm_enabled":                       result.LSTMEnabled,
		"lstm_model_version":                 result.LSTMModelVersion,
		"lstm_weight":                        result.LSTMWeight,
		"lstm_prediction_as_of":              result.LSTMPredictionAsOf,
		"lstm_prediction_count":              result.LSTMPredictionCount,
		"lstm_applied_count":                 result.LSTMAppliedCount,
		"lstm_tuning_mode":                   result.LSTMTuningMode,
		"lstm_tuning_weight_multiplier":      result.LSTMTuningWeightMult,
		"lstm_tuning_recent_hit_rate":        result.LSTMTuningRecentHit,
		"lstm_tuning_recent_topk_hit_rate":   result.LSTMTuningRecentTopK,
		"lstm_tuning_recent_evaluated_count": result.LSTMTuningEvalCount,
		"news_stock_applied_count":           result.NewsStockAppliedCount,
		"news_target_trading_date":           result.NewsTargetTradingDate,
		"news_risk_on_prob":                  result.NewsRiskOnProb,
		"news_risk_off_prob":                 result.NewsRiskOffProb,
		"news_neutral_prob":                  result.NewsNeutralProb,
		"news_regime_confidence":             result.NewsRegimeConfidence,
		"news_regime_bias":                   result.NewsRegimeBias,
		"news_time_precision":                result.NewsTimePrecision,
		"items":                              items,
	}, nil
}

func formatMarketCap(v int64) string {
	if v >= 1_0000_0000_0000 {
		return fmt.Sprintf("%.1f조", float64(v)/1_0000_0000_0000)
	}
	if v >= 1_0000_0000 {
		return fmt.Sprintf("%.0f억", float64(v)/1_0000_0000)
	}
	return fmt.Sprintf("%d", v)
}

func (s *Service) Report(market string, limit int, minMarketCap int64) (map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	minMarketCap = sanitizeMinMarketCap(minMarketCap)

	result, err := s.computeRanks(market, minMarketCap)
	if err != nil {
		return nil, err
	}
	items := result.Items
	if limit < len(items) {
		items = items[:limit]
	}

	kospiCount := 0
	kosdaqCount := 0
	sum1 := 0.0
	sum5 := 0.0
	sum20 := 0.0
	sumNextDay := 0.0
	sumNXTScore := 0.0
	sumProbUp := 0.0
	sumNewsScore := 0.0
	nxtScoreCount := 0
	probUpCount := 0
	newsScoreCount := 0
	for _, it := range items {
		if it.Market == "KOSPI" {
			kospiCount++
		} else if it.Market == "KOSDAQ" {
			kosdaqCount++
		}
		sum1 += it.Return1D
		sum5 += it.Return5D
		sum20 += it.Return20D
		sumNextDay += it.NextDayScore
		if it.NXTScore > 0 {
			sumNXTScore += it.NXTScore
			nxtScoreCount++
		}
		if it.LSTMProbUp > 0 {
			sumProbUp += it.LSTMProbUp
			probUpCount++
		}
		if it.NewsScore > 0 {
			sumNewsScore += it.NewsScore
			newsScoreCount++
		}
	}
	avg1 := 0.0
	avg5 := 0.0
	avg20 := 0.0
	avgNextDay := 0.0
	avgNXTScore := 0.0
	avgProbUp := 0.0
	avgNewsScore := 0.0
	if len(items) > 0 {
		avg1 = sum1 / float64(len(items))
		avg5 = sum5 / float64(len(items))
		avg20 = sum20 / float64(len(items))
		avgNextDay = sumNextDay / float64(len(items))
	}
	if probUpCount > 0 {
		avgProbUp = sumProbUp / float64(probUpCount)
	}
	if nxtScoreCount > 0 {
		avgNXTScore = sumNXTScore / float64(nxtScoreCount)
	}
	if newsScoreCount > 0 {
		avgNewsScore = sumNewsScore / float64(newsScoreCount)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 퀀트 랭킹 리포트 (%s)\n\n", result.GeneratedAt))
	b.WriteString(fmt.Sprintf("- 모델 버전: `%s`\n", result.ScoreModel))
	b.WriteString(fmt.Sprintf("- 시장: `%s`\n", result.Market))
	b.WriteString(fmt.Sprintf("- 분석 대상: %d개 종목\n", result.UniverseCount))
	b.WriteString(fmt.Sprintf("- 시가총액 하한: %s\n", formatMarketCap(result.MinMarketCap)))
	b.WriteString(fmt.Sprintf("- 데이터 기준일 범위: %s ~ %s\n", result.AsOfMin, result.AsOfMax))
	if result.NXTEnabled {
		b.WriteString(fmt.Sprintf("- NXT 지연시세 적용: %s / 적용 %d개 / 원본 %d개\n", result.NXTTradingDate, result.NXTAppliedCount, result.NXTQuoteCount))
	}
	if result.LSTMEnabled {
		b.WriteString(fmt.Sprintf("- LSTM 보조 점수: `%s` / 가중치 %.2f%% / 적용 %d개\n", result.LSTMModelVersion, result.LSTMWeight*100, result.LSTMAppliedCount))
	}
	if result.LSTMTuningMode != "" {
		b.WriteString(fmt.Sprintf("- LSTM 튜닝: `%s` / weight x%.2f / 최근 적중률 %.2f%% / 최근 Top20 적중률 %.2f%% / 평가표본 %d개\n",
			result.LSTMTuningMode,
			result.LSTMTuningWeightMult,
			result.LSTMTuningRecentHit,
			result.LSTMTuningRecentTopK,
			result.LSTMTuningEvalCount,
		))
	}
	if result.NewsStockAppliedCount > 0 {
		b.WriteString(fmt.Sprintf("- 종목 뉴스 점수 적용: %d개 / 평균 %.2f\n", result.NewsStockAppliedCount, avgNewsScore))
	}
	if result.NewsTargetTradingDate != "" {
		newsBias := strings.TrimSpace(result.NewsRegimeBias)
		if newsBias == "" {
			newsBias = "neutral"
		}
		b.WriteString(fmt.Sprintf("- 뉴스 레짐: `%s` / Risk-On %.2f%% / Risk-Off %.2f%% / 신뢰도 %.2f%% / 목표 거래일 %s\n",
			newsBias,
			result.NewsRiskOnProb,
			result.NewsRiskOffProb,
			result.NewsRegimeConfidence,
			result.NewsTargetTradingDate,
		))
	}
	b.WriteString(fmt.Sprintf("- 상위 %d 평균 최근 흐름: 1일 %.2f%% / 5일 %.2f%% / 20일 %.2f%%\n", len(items), avg1, avg5, avg20))
	b.WriteString(fmt.Sprintf("- 상위 %d 평균 다음날 점수: %.2f", len(items), avgNextDay))
	if avgNXTScore > 0 {
		b.WriteString(fmt.Sprintf(" / NXT 평균 점수 %.2f", avgNXTScore))
	}
	if avgProbUp > 0 {
		b.WriteString(fmt.Sprintf(" / LSTM 평균 상승확률 %.2f%%", avgProbUp))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("- 상위 %d 시장 분포: KOSPI %d / KOSDAQ %d\n\n", len(items), kospiCount, kosdaqCount))
	b.WriteString("## 상위 종목\n\n")
	b.WriteString("|순위|종목|시장|총점|다음날점수|뉴스|1일|5일|LSTM확률|시총|\n")
	b.WriteString("|---:|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, it := range items {
		lstmProb := "-"
		if it.LSTMProbUp > 0 {
			lstmProb = fmt.Sprintf("%.2f%%", it.LSTMProbUp)
		}
		newsScore := "-"
		if it.NewsScore > 0 {
			newsScore = fmt.Sprintf("%.2f", it.NewsScore)
		}
		b.WriteString(
			fmt.Sprintf(
				"|%d|%s(%s)|%s|%.2f|%.2f|%s|%.2f%%|%.2f%%|%s|%s|\n",
				it.Rank,
				it.Name,
				it.Code,
				it.Market,
				it.TotalScore,
				it.NextDayScore,
				newsScore,
				it.Return1D,
				it.Return5D,
				lstmProb,
				formatMarketCap(it.MarketCap),
			),
		)
	}

	return map[string]any{
		"generated_at":                       result.GeneratedAt,
		"score_model":                        result.ScoreModel,
		"market":                             result.Market,
		"min_market_cap":                     result.MinMarketCap,
		"limit":                              len(items),
		"universe_count":                     result.UniverseCount,
		"as_of_min":                          result.AsOfMin,
		"as_of_max":                          result.AsOfMax,
		"nxt_enabled":                        result.NXTEnabled,
		"nxt_trading_date":                   result.NXTTradingDate,
		"nxt_set_time":                       result.NXTSetTime,
		"nxt_quote_count":                    result.NXTQuoteCount,
		"nxt_applied_count":                  result.NXTAppliedCount,
		"lstm_enabled":                       result.LSTMEnabled,
		"lstm_model_version":                 result.LSTMModelVersion,
		"lstm_weight":                        result.LSTMWeight,
		"lstm_prediction_as_of":              result.LSTMPredictionAsOf,
		"lstm_prediction_count":              result.LSTMPredictionCount,
		"lstm_applied_count":                 result.LSTMAppliedCount,
		"lstm_tuning_mode":                   result.LSTMTuningMode,
		"lstm_tuning_weight_multiplier":      result.LSTMTuningWeightMult,
		"lstm_tuning_recent_hit_rate":        result.LSTMTuningRecentHit,
		"lstm_tuning_recent_topk_hit_rate":   result.LSTMTuningRecentTopK,
		"lstm_tuning_recent_evaluated_count": result.LSTMTuningEvalCount,
		"news_stock_applied_count":           result.NewsStockAppliedCount,
		"news_target_trading_date":           result.NewsTargetTradingDate,
		"news_risk_on_prob":                  result.NewsRiskOnProb,
		"news_risk_off_prob":                 result.NewsRiskOffProb,
		"news_neutral_prob":                  result.NewsNeutralProb,
		"news_regime_confidence":             result.NewsRegimeConfidence,
		"news_regime_bias":                   result.NewsRegimeBias,
		"news_time_precision":                result.NewsTimePrecision,
		"average": map[string]any{
			"return_1d":      round2(avg1),
			"return_5d":      round2(avg5),
			"return_20d":     round2(avg20),
			"next_day_score": round2(avgNextDay),
			"nxt_score":      round2(avgNXTScore),
			"lstm_prob_up":   round2(avgProbUp),
			"news_score":     round2(avgNewsScore),
		},
		"distribution": map[string]any{
			"kospi":  kospiCount,
			"kosdaq": kosdaqCount,
		},
		"items":           items,
		"report_markdown": b.String(),
	}, nil
}
