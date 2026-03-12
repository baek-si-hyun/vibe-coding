package upbit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"investment-news-go/internal/config"
)

const (
	baseURL               = "https://api.upbit.com/v1"
	defaultQuantLimit     = 30
	maxQuantLimit         = 100
	defaultMinTradeValue  = 5_000_000_000.0
	quantInterval         = "1h"
	quantCandleUnit       = 60
	quantCacheTTL         = 30 * time.Second
	quantConcurrency      = 4
	quantCandidateMin     = 30
	quantCandidateMax     = 60
	quantCandles24H       = 24
	quantMinCandleRequest = 200
	quantMinRequired      = quantCandles24H * 2
)

type Candle struct {
	Timestamp  int64
	Open       float64
	Close      float64
	High       float64
	Low        float64
	Volume     float64
	TradeValue float64
}

type tickerItem struct {
	Market           string  `json:"market"`
	TradePrice       float64 `json:"trade_price"`
	AccTradePrice24H float64 `json:"acc_trade_price_24h"`
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

func (s *Service) fetchTickersKRW() ([]tickerItem, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/ticker/all?quote_currencies=KRW", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ticker error: %s", strings.TrimSpace(string(body)))
	}

	var payload []tickerItem
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]tickerItem, 0, len(payload))
	for _, item := range payload {
		if !strings.HasPrefix(item.Market, "KRW-") || item.TradePrice <= 0 || item.AccTradePrice24H <= 0 {
			continue
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].AccTradePrice24H > out[j].AccTradePrice24H
	})

	return out, nil
}

func (s *Service) fetchCandles(market string) ([]Candle, error) {
	collected := make([]Candle, 0, quantMinRequired)
	var to time.Time

	for len(collected) < quantMinRequired {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/candles/minutes/%d", baseURL, quantCandleUnit), nil)
		if err != nil {
			return nil, err
		}

		query := req.URL.Query()
		query.Set("market", market)
		query.Set("count", fmt.Sprintf("%d", quantMinCandleRequest))
		if !to.IsZero() {
			query.Set("to", to.Format(time.RFC3339))
		}
		req.URL.RawQuery = query.Encode()
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("candles error: %s", strings.TrimSpace(string(body)))
		}

		var payload []struct {
			CandleDateTimeUTC string  `json:"candle_date_time_utc"`
			OpeningPrice      float64 `json:"opening_price"`
			HighPrice         float64 `json:"high_price"`
			LowPrice          float64 `json:"low_price"`
			TradePrice        float64 `json:"trade_price"`
			Timestamp         int64   `json:"timestamp"`
			TradePriceAcc     float64 `json:"candle_acc_trade_price"`
			TradeVolumeAcc    float64 `json:"candle_acc_trade_volume"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		if len(payload) == 0 {
			break
		}

		for _, raw := range payload {
			if raw.TradePrice <= 0 || raw.OpeningPrice <= 0 || raw.HighPrice <= 0 || raw.LowPrice <= 0 || raw.TradeVolumeAcc < 0 || raw.TradePriceAcc < 0 {
				continue
			}
			collected = append(collected, Candle{
				Timestamp:  raw.Timestamp,
				Open:       raw.OpeningPrice,
				Close:      raw.TradePrice,
				High:       raw.HighPrice,
				Low:        raw.LowPrice,
				Volume:     raw.TradeVolumeAcc,
				TradeValue: raw.TradePriceAcc,
			})
		}

		oldest := payload[len(payload)-1]
		parsed, err := time.Parse("2006-01-02T15:04:05", oldest.CandleDateTimeUTC)
		if err != nil {
			break
		}
		to = parsed.Add(-time.Second).UTC()

		if len(payload) < quantMinCandleRequest {
			break
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Timestamp < collected[j].Timestamp
	})

	if len(collected) > quantMinRequired {
		collected = collected[len(collected)-quantMinRequired:]
	}

	return collected, nil
}
