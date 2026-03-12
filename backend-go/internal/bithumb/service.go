package bithumb

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"investment-news-go/internal/config"
)

const (
	baseURL                 = "https://api.bithumb.com"
	paymentCurrency         = "KRW"
	concurrency             = 8
	maxResults              = 50
	quantCacheTTL           = 30 * time.Second
	volumeSpikeRatio        = 5.0
	patternVolumeWindow     = 20
	patternVolumeSpikeRatio = 3.0
	patternResWindow        = 20
	patternLookback         = 48
)

type Candle struct {
	Timestamp int64
	Open      float64
	Close     float64
	High      float64
	Low       float64
	Volume    float64
}

type Signals struct {
	SpikeRatio   float64 `json:"spikeRatio"`
	SpikeTime    int64   `json:"spikeTime"`
	ResBreakTime int64   `json:"resBreakTime"`
	MA7Time      *int64  `json:"ma7Time,omitempty"`
	MA20Time     *int64  `json:"ma20Time,omitempty"`
}

type Item struct {
	Symbol       string   `json:"symbol"`
	Price        float64  `json:"price"`
	CandleTime   int64    `json:"candleTime"`
	Volume       float64  `json:"volume,omitempty"`
	PrevVolume   float64  `json:"prevVolume,omitempty"`
	Ratio        float64  `json:"ratio,omitempty"`
	MA           float64  `json:"ma,omitempty"`
	DeviationPct float64  `json:"deviationPct,omitempty"`
	SignalTime   int64    `json:"signalTime,omitempty"`
	Signals      *Signals `json:"signals,omitempty"`
}

type cacheEntry struct {
	expiresAt time.Time
	result    map[string]any
}

type Service struct {
	cfg        config.Config
	client     *http.Client
	quantCache map[string]cacheEntry
	mu         sync.RWMutex
}

func NewService(cfg config.Config) *Service {
	return &Service{
		cfg:        cfg,
		client:     &http.Client{Timeout: 30 * time.Second},
		quantCache: map[string]cacheEntry{},
	}
}

func (s *Service) fetchSymbols() ([]string, error) {
	resp, err := s.client.Get(fmt.Sprintf("%s/public/ticker/ALL_%s", baseURL, paymentCurrency))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ticker error: %s", string(body))
	}

	var payload struct {
		Status string                 `json:"status"`
		Data   map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "0000" || len(payload.Data) == 0 {
		return nil, fmt.Errorf("ticker response error")
	}
	out := make([]string, 0, len(payload.Data))
	for k := range payload.Data {
		if k == "date" {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func parseFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		s := fmt.Sprint(v)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
}

func (s *Service) fetchCandles(symbol, interval string) ([]Candle, error) {
	resp, err := s.client.Get(fmt.Sprintf("%s/public/candlestick/%s_%s/%s", baseURL, symbol, paymentCurrency, interval))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("candlestick error: %s", string(body))
	}

	var payload struct {
		Status string  `json:"status"`
		Data   [][]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "0000" || payload.Data == nil {
		return nil, fmt.Errorf("candlestick response error")
	}

	candles := make([]Candle, 0, len(payload.Data))
	for _, raw := range payload.Data {
		if len(raw) < 6 {
			continue
		}
		ts, ok := parseFloat(raw[0])
		if !ok {
			continue
		}
		op, ok1 := parseFloat(raw[1])
		cl, ok2 := parseFloat(raw[2])
		hi, ok3 := parseFloat(raw[3])
		lo, ok4 := parseFloat(raw[4])
		vol, ok5 := parseFloat(raw[5])
		if !(ok1 && ok2 && ok3 && ok4 && ok5) {
			continue
		}
		if op < 0 || cl < 0 || hi < 0 || lo < 0 || vol < 0 {
			continue
		}
		candles = append(candles, Candle{
			Timestamp: int64(ts),
			Open:      op,
			Close:     cl,
			High:      hi,
			Low:       lo,
			Volume:    vol,
		})
	}
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Timestamp < candles[j].Timestamp
	})
	return candles, nil
}

func mapWithConcurrency(symbols []string, fn func(symbol string) *Item) []*Item {
	if len(symbols) == 0 {
		return []*Item{}
	}
	sem := make(chan struct{}, concurrency)
	out := make([]*Item, 0, len(symbols))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, symbol := range symbols {
		symbol := symbol
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			item := fn(symbol)
			if item == nil {
				return
			}
			mu.Lock()
			out = append(out, item)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (s *Service) buildVolumeItems(symbols []string) []Item {
	raw := mapWithConcurrency(symbols, func(symbol string) *Item {
		candles, err := s.fetchCandles(symbol, "5m")
		if err != nil || len(candles) < 2 {
			return nil
		}
		latest := candles[len(candles)-1]
		prev := candles[len(candles)-2]
		if prev.Volume <= 0 {
			return nil
		}
		ratio := latest.Volume / prev.Volume
		if ratio < volumeSpikeRatio {
			return nil
		}
		return &Item{
			Symbol:     symbol,
			Price:      latest.Close,
			CandleTime: latest.Timestamp,
			Volume:     latest.Volume,
			PrevVolume: prev.Volume,
			Ratio:      ratio,
		}
	})
	items := make([]Item, 0, len(raw))
	for _, it := range raw {
		items = append(items, *it)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Ratio > items[j].Ratio
	})
	if len(items) > maxResults {
		items = items[:maxResults]
	}
	return items
}

func (s *Service) buildMAItems(symbols []string, period int) []Item {
	raw := mapWithConcurrency(symbols, func(symbol string) *Item {
		candles, err := s.fetchCandles(symbol, "5m")
		if err != nil || len(candles) < period {
			return nil
		}
		current := candles[len(candles)-1]
		window := candles[len(candles)-period:]
		sum := 0.0
		for _, c := range window {
			sum += c.Close
		}
		ma := sum / float64(period)
		if ma <= 0 {
			return nil
		}
		bodyLow := math.Min(current.Open, current.Close)
		deviation := ((current.Close - ma) / ma) * 100
		wickTouched := current.Low <= ma && bodyLow > ma
		bullish := current.Close > current.Open
		if !(wickTouched && bullish) {
			return nil
		}
		return &Item{
			Symbol:       symbol,
			Price:        current.Close,
			CandleTime:   current.Timestamp,
			MA:           ma,
			DeviationPct: deviation,
		}
	})
	items := make([]Item, 0, len(raw))
	for _, it := range raw {
		items = append(items, *it)
	}
	sort.Slice(items, func(i, j int) bool {
		return math.Abs(items[i].DeviationPct) < math.Abs(items[j].DeviationPct)
	})
	if len(items) > maxResults {
		items = items[:maxResults]
	}
	return items
}

func rollingAverage(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN()
	}
	sum := 0.0
	for i := 0; i < len(values); i++ {
		sum += values[i]
		if i >= window {
			sum -= values[i-window]
		}
		if i >= window-1 {
			out[i] = sum / float64(window)
		}
	}
	return out
}

func findLastVolumeSpike(volumes []float64, startIndex int) (int, float64, bool) {
	prefix := make([]float64, len(volumes)+1)
	for i := 0; i < len(volumes); i++ {
		prefix[i+1] = prefix[i] + volumes[i]
	}
	lastIdx := -1
	lastRatio := 0.0
	for i := max(startIndex, patternVolumeWindow); i < len(volumes); i++ {
		avg := (prefix[i] - prefix[i-patternVolumeWindow]) / float64(patternVolumeWindow)
		if avg <= 0 {
			continue
		}
		ratio := volumes[i] / avg
		if ratio >= patternVolumeSpikeRatio {
			lastIdx = i
			lastRatio = ratio
		}
	}
	return lastIdx, lastRatio, lastIdx >= 0
}

func findLastResistanceBreak(candles []Candle, startIndex int) int {
	lastIdx := -1
	for i := max(startIndex, patternResWindow); i < len(candles); i++ {
		prevHigh := candles[i-patternResWindow].High
		for j := i - patternResWindow + 1; j < i; j++ {
			if candles[j].High > prevHigh {
				prevHigh = candles[j].High
			}
		}
		if candles[i].Close >= prevHigh {
			lastIdx = i
		}
	}
	return lastIdx
}

func findLastMABounce(candles []Candle, ma []float64, startIndex int) int {
	lastIdx := -1
	for i := startIndex; i < len(candles); i++ {
		maVal := ma[i]
		if math.IsNaN(maVal) {
			continue
		}
		c := candles[i]
		bodyLow := math.Min(c.Open, c.Close)
		bullish := c.Close > c.Open
		if bullish && c.Low <= maVal && bodyLow > maVal {
			lastIdx = i
		}
	}
	return lastIdx
}

func (s *Service) buildPatternItems(symbols []string) []Item {
	raw := mapWithConcurrency(symbols, func(symbol string) *Item {
		candles, err := s.fetchCandles(symbol, "5m")
		if err != nil {
			return nil
		}
		minLength := max(patternResWindow, max(patternVolumeWindow, 20))
		if len(candles) < minLength {
			return nil
		}
		startIndex := max(0, len(candles)-patternLookback)
		closes := make([]float64, len(candles))
		volumes := make([]float64, len(candles))
		for i, c := range candles {
			closes[i] = c.Close
			volumes[i] = c.Volume
		}
		ma7 := rollingAverage(closes, 7)
		ma20 := rollingAverage(closes, 20)
		spikeIdx, spikeRatio, spikeOk := findLastVolumeSpike(volumes, startIndex)
		resIdx := findLastResistanceBreak(candles, startIndex)
		ma7Idx := findLastMABounce(candles, ma7, startIndex)
		ma20Idx := findLastMABounce(candles, ma20, startIndex)
		if !spikeOk || resIdx < 0 || (ma7Idx < 0 && ma20Idx < 0) {
			return nil
		}
		latest := candles[len(candles)-1]
		ma7Time := int64(0)
		ma20Time := int64(0)
		if ma7Idx >= 0 {
			ma7Time = candles[ma7Idx].Timestamp
		}
		if ma20Idx >= 0 {
			ma20Time = candles[ma20Idx].Timestamp
		}
		signalTime := maxInt64(candles[spikeIdx].Timestamp, maxInt64(candles[resIdx].Timestamp, maxInt64(ma7Time, ma20Time)))
		var ma7Ptr *int64
		var ma20Ptr *int64
		if ma7Idx >= 0 {
			v := candles[ma7Idx].Timestamp
			ma7Ptr = &v
		}
		if ma20Idx >= 0 {
			v := candles[ma20Idx].Timestamp
			ma20Ptr = &v
		}
		return &Item{
			Symbol:     symbol,
			Price:      latest.Close,
			CandleTime: latest.Timestamp,
			SignalTime: signalTime,
			Signals: &Signals{
				SpikeRatio:   spikeRatio,
				SpikeTime:    candles[spikeIdx].Timestamp,
				ResBreakTime: candles[resIdx].Timestamp,
				MA7Time:      ma7Ptr,
				MA20Time:     ma20Ptr,
			},
		}
	})
	items := make([]Item, 0, len(raw))
	for _, it := range raw {
		items = append(items, *it)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SignalTime > items[j].SignalTime
	})
	if len(items) > maxResults {
		items = items[:maxResults]
	}
	return items
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Service) GetScreenerData(mode string) (map[string]any, error) {
	symbols, err := s.fetchSymbols()
	if err != nil {
		return nil, fmt.Errorf("심볼 조회 실패: %s", err.Error())
	}

	if mode != "volume" && mode != "ma7" && mode != "ma20" && mode != "pattern" {
		mode = "volume"
	}

	items := []Item{}
	switch mode {
	case "volume":
		items = s.buildVolumeItems(symbols)
	case "ma7":
		items = s.buildMAItems(symbols, 7)
	case "ma20":
		items = s.buildMAItems(symbols, 20)
	case "pattern":
		items = s.buildPatternItems(symbols)
	}

	return map[string]any{
		"mode":  mode,
		"asOf":  time.Now().UnixMilli(),
		"items": items,
	}, nil
}
