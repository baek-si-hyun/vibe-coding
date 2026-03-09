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
)

const (
	defaultRankLimit          = 30
	maxRankLimit              = 300
	defaultMinMarketCap int64 = 1_000_000_000_000 // 1조
	cacheTTL                  = 60 * time.Second
	scoreModelVersion         = "multifactor_v2"
)

type Service struct {
	cfg        config.Config
	cache      map[string]cacheEntry
	macroCache macroCacheEntry
	mu         sync.RWMutex
}

type cacheEntry struct {
	expiresAt time.Time
	result    rankResult
}

type rankResult struct {
	GeneratedAt   string     `json:"generated_at"`
	ScoreModel    string     `json:"score_model"`
	Market        string     `json:"market"`
	MinMarketCap  int64      `json:"min_market_cap"`
	UniverseCount int        `json:"universe_count"`
	AsOfMin       string     `json:"as_of_min"`
	AsOfMax       string     `json:"as_of_max"`
	Items         []RankItem `json:"items"`
}

type RankItem struct {
	Rank           int     `json:"rank"`
	Market         string  `json:"market"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	AsOf           string  `json:"as_of"`
	Close          float64 `json:"close"`
	MarketCap      int64   `json:"market_cap"`
	Turnover       float64 `json:"turnover"`
	TurnoverRatio  float64 `json:"turnover_ratio"`
	Return1D       float64 `json:"return_1d"`
	Return5D       float64 `json:"return_5d"`
	Return10D      float64 `json:"return_10d"`
	Return20D      float64 `json:"return_20d"`
	Return60D      float64 `json:"return_60d"`
	Return120D     float64 `json:"return_120d"`
	Volatility20D  float64 `json:"volatility_20d"`
	Volatility60D  float64 `json:"volatility_60d"`
	DownsideVol20D float64 `json:"downside_volatility_20d"`
	Drawdown60D    float64 `json:"drawdown_60d"`
	Drawdown120D   float64 `json:"drawdown_120d"`
	AvgTurnover20D float64 `json:"avg_turnover_ratio_20d"`
	MomentumScore  float64 `json:"momentum_score"`
	LiquidityScore float64 `json:"liquidity_score"`
	StabilityScore float64 `json:"stability_score"`
	TrendScore     float64 `json:"trend_score"`
	RiskAdjScore   float64 `json:"risk_adjusted_score"`
	TotalScore     float64 `json:"total_score"`

	rawShortMomentum      float64
	rawMomentum20         float64
	rawMomentum60         float64
	rawMomentum120        float64
	rawRiskAdjusted       float64
	rawTrendAlignment     float64
	rawTrendQuality       float64
	rawLiquidityNow       float64
	rawLiquidityAverage   float64
	rawLiquidityStability float64
	rawLiquiditySize      float64
	rawStability20        float64
	rawStability60        float64
	rawStabilityDownside  float64
	rawDrawdown60         float64
	rawDrawdown120        float64
}

func NewService(cfg config.Config) *Service {
	return &Service{
		cfg:   cfg,
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
	required := []string{"BAS_DD", "TDD_CLSPRC", "ACC_TRDVAL", "MKTCAP", "ISU_CD", "ISU_NM"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing column %s", col)
		}
	}

	closes := make([]float64, 0, 512)
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
	turnoverRatios20 := make([]float64, 0, len(turnoverTail20))
	if latestMktCap > 0 {
		avgTurnoverRatio20 = avgTurnover20 / float64(latestMktCap) * 100
		for _, value := range turnoverTail20 {
			turnoverRatios20 = append(turnoverRatios20, value/float64(latestMktCap)*100)
		}
	}
	liquidityStability := -coefficientOfVariation(turnoverRatios20)

	return &RankItem{
		Market:                market,
		Code:                  code,
		Name:                  name,
		AsOf:                  latestDate,
		Close:                 latestClose,
		MarketCap:             latestMktCap,
		Turnover:              latestTurnover,
		TurnoverRatio:         turnoverRatio,
		Return1D:              ret1,
		Return5D:              ret5,
		Return10D:             ret10,
		Return20D:             ret20,
		Return60D:             ret60,
		Return120D:            ret120,
		Volatility20D:         vol20,
		Volatility60D:         vol60,
		DownsideVol20D:        downVol20,
		Drawdown60D:           drawdown60,
		Drawdown120D:          drawdown120,
		AvgTurnover20D:        avgTurnoverRatio20,
		rawShortMomentum:      0.6*ret5 + 0.4*ret10,
		rawMomentum20:         ret20,
		rawMomentum60:         ret60,
		rawMomentum120:        ret120,
		rawRiskAdjusted:       riskAdjusted,
		rawTrendAlignment:     trendAlignment,
		rawTrendQuality:       trendQuality,
		rawLiquidityNow:       turnoverRatio,
		rawLiquidityAverage:   avgTurnoverRatio20,
		rawLiquidityStability: liquidityStability,
		rawLiquiditySize:      math.Log1p(float64(latestMktCap)),
		rawStability20:        -vol20,
		rawStability60:        -vol60,
		rawStabilityDownside:  -downVol20,
		rawDrawdown60:         drawdown60,
		rawDrawdown120:        drawdown120,
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
		it := &items[i]
		if it.AsOf < asOfMin {
			asOfMin = it.AsOf
		}
		if it.AsOf > asOfMax {
			asOfMax = it.AsOf
		}
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

	for i := range items {
		it := &items[i]
		it.TrendScore = round2(0.55*trendAlignmentScores[i] + 0.45*trendQualityScores[i])
		it.RiskAdjScore = round2(riskAdjustedScores[i])
		it.MomentumScore = round2(
			0.10*shortMomentumScores[i] +
				0.20*momentum20Scores[i] +
				0.22*momentum60Scores[i] +
				0.15*momentum120Scores[i] +
				0.18*riskAdjustedScores[i] +
				0.08*trendAlignmentScores[i] +
				0.07*trendQualityScores[i],
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
		total := 0.52*it.MomentumScore + 0.18*it.LiquidityScore + 0.30*it.StabilityScore
		it.TotalScore = round2(total)
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

	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalScore != items[j].TotalScore {
			return items[i].TotalScore > items[j].TotalScore
		}
		if items[i].MomentumScore != items[j].MomentumScore {
			return items[i].MomentumScore > items[j].MomentumScore
		}
		if items[i].StabilityScore != items[j].StabilityScore {
			return items[i].StabilityScore > items[j].StabilityScore
		}
		return items[i].MarketCap > items[j].MarketCap
	})
	for i := range items {
		items[i].Rank = i + 1
	}

	result := rankResult{
		GeneratedAt:   now.Format(time.RFC3339),
		ScoreModel:    scoreModelVersion,
		Market:        market,
		MinMarketCap:  minMarketCap,
		UniverseCount: len(items),
		AsOfMin:       asOfMin,
		AsOfMax:       asOfMax,
		Items:         items,
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
		"generated_at":   result.GeneratedAt,
		"score_model":    result.ScoreModel,
		"market":         result.Market,
		"min_market_cap": result.MinMarketCap,
		"limit":          limit,
		"universe_count": result.UniverseCount,
		"as_of_min":      result.AsOfMin,
		"as_of_max":      result.AsOfMax,
		"items":          items,
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
	sum20 := 0.0
	sum60 := 0.0
	for _, it := range items {
		if it.Market == "KOSPI" {
			kospiCount++
		} else if it.Market == "KOSDAQ" {
			kosdaqCount++
		}
		sum20 += it.Return20D
		sum60 += it.Return60D
	}
	avg20 := 0.0
	avg60 := 0.0
	if len(items) > 0 {
		avg20 = sum20 / float64(len(items))
		avg60 = sum60 / float64(len(items))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 퀀트 랭킹 리포트 (%s)\n\n", result.GeneratedAt))
	b.WriteString(fmt.Sprintf("- 모델 버전: `%s`\n", result.ScoreModel))
	b.WriteString(fmt.Sprintf("- 시장: `%s`\n", result.Market))
	b.WriteString(fmt.Sprintf("- 분석 대상: %d개 종목\n", result.UniverseCount))
	b.WriteString(fmt.Sprintf("- 시가총액 하한: %s\n", formatMarketCap(result.MinMarketCap)))
	b.WriteString(fmt.Sprintf("- 데이터 기준일 범위: %s ~ %s\n", result.AsOfMin, result.AsOfMax))
	b.WriteString(fmt.Sprintf("- 상위 %d 평균 수익률: 20일 %.2f%% / 60일 %.2f%%\n", len(items), avg20, avg60))
	b.WriteString(fmt.Sprintf("- 상위 %d 시장 분포: KOSPI %d / KOSDAQ %d\n\n", len(items), kospiCount, kosdaqCount))
	b.WriteString("## 상위 종목\n\n")
	b.WriteString("|순위|종목|시장|총점|20일|60일|120일|시총|\n")
	b.WriteString("|---:|---|---|---:|---:|---:|---:|---:|\n")
	for _, it := range items {
		b.WriteString(
			fmt.Sprintf(
				"|%d|%s(%s)|%s|%.2f|%.2f%%|%.2f%%|%.2f%%|%s|\n",
				it.Rank,
				it.Name,
				it.Code,
				it.Market,
				it.TotalScore,
				it.Return20D,
				it.Return60D,
				it.Return120D,
				formatMarketCap(it.MarketCap),
			),
		)
	}

	return map[string]any{
		"generated_at":   result.GeneratedAt,
		"score_model":    result.ScoreModel,
		"market":         result.Market,
		"min_market_cap": result.MinMarketCap,
		"limit":          len(items),
		"universe_count": result.UniverseCount,
		"as_of_min":      result.AsOfMin,
		"as_of_max":      result.AsOfMax,
		"average": map[string]any{
			"return_20d": round2(avg20),
			"return_60d": round2(avg60),
		},
		"distribution": map[string]any{
			"kospi":  kospiCount,
			"kosdaq": kosdaqCount,
		},
		"items":           items,
		"report_markdown": b.String(),
	}, nil
}
