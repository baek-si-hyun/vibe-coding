package quant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type nxtQuoteEntry struct {
	TradingDate  string  `json:"trading_date"`
	Market       string  `json:"market"`
	Code         string  `json:"code"`
	ShortCode    string  `json:"short_code,omitempty"`
	IssueCode    string  `json:"issue_code,omitempty"`
	Name         string  `json:"name"`
	SnapshotAt   string  `json:"snapshot_at,omitempty"`
	BasePrice    float64 `json:"base_price"`
	CurrentPrice float64 `json:"current_price"`
	ChangePrice  float64 `json:"change_price"`
	ChangeRate   float64 `json:"change_rate"`
	OpenPrice    float64 `json:"open_price"`
	HighPrice    float64 `json:"high_price"`
	LowPrice     float64 `json:"low_price"`
	Volume       int64   `json:"volume"`
	TradeValue   int64   `json:"trade_value"`
}

type nxtSnapshotFile struct {
	TradingDate     string          `json:"trading_date"`
	SetTime         string          `json:"set_time,omitempty"`
	FetchedAt       string          `json:"fetched_at"`
	SourceURL       string          `json:"source_url"`
	QuoteCount      int             `json:"quote_count"`
	MarketCounts    map[string]int  `json:"market_counts,omitempty"`
	TotalTradeValue int64           `json:"total_trade_value"`
	TotalVolume     int64           `json:"total_volume"`
	Items           []nxtQuoteEntry `json:"items"`
}

type nxtSnapshotState struct {
	Enabled         bool
	TradingDate     string
	SetTime         string
	FetchedAt       string
	QuoteCount      int
	TotalTradeValue int64
	TotalVolume     int64
	Index           map[string]nxtQuoteEntry
}

type nxtSnapshotCache struct {
	path        string
	tradingDate string
	modTime     time.Time
	snapshot    nxtSnapshotState
}

func makeNXTKey(market, code string) string {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		return ""
	}
	normalizedMarket := normalizePredictionMarket(market)
	if normalizedMarket == "" {
		return normalizedCode
	}
	return normalizedMarket + ":" + normalizedCode
}

func (s *Service) loadNXTSnapshot(tradingDate string) (nxtSnapshotState, error) {
	tradingDate = strings.TrimSpace(tradingDate)
	if tradingDate == "" {
		return nxtSnapshotState{}, nil
	}
	path := filepath.Join(s.cfg.DataRootDir, "nxt", "snapshots", "nxt_snapshot_"+tradingDate+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nxtSnapshotState{}, nil
		}
		return nxtSnapshotState{}, err
	}

	s.mu.RLock()
	cached := s.nxtCache
	s.mu.RUnlock()
	if cached.path == path && cached.tradingDate == tradingDate && info.ModTime().Equal(cached.modTime) {
		return cached.snapshot, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nxtSnapshotState{}, err
	}

	var payload nxtSnapshotFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return nxtSnapshotState{}, err
	}

	index := make(map[string]nxtQuoteEntry, len(payload.Items)*2)
	for _, raw := range payload.Items {
		entry := nxtQuoteEntry{
			TradingDate:  strings.TrimSpace(raw.TradingDate),
			Market:       normalizePredictionMarket(raw.Market),
			Code:         strings.TrimSpace(raw.Code),
			ShortCode:    strings.TrimSpace(raw.ShortCode),
			IssueCode:    strings.TrimSpace(raw.IssueCode),
			Name:         strings.TrimSpace(raw.Name),
			SnapshotAt:   strings.TrimSpace(raw.SnapshotAt),
			BasePrice:    raw.BasePrice,
			CurrentPrice: raw.CurrentPrice,
			ChangePrice:  raw.ChangePrice,
			ChangeRate:   raw.ChangeRate,
			OpenPrice:    raw.OpenPrice,
			HighPrice:    raw.HighPrice,
			LowPrice:     raw.LowPrice,
			Volume:       raw.Volume,
			TradeValue:   raw.TradeValue,
		}
		if entry.Code == "" {
			continue
		}
		if existing, ok := index[makeNXTKey(entry.Market, entry.Code)]; !ok || entry.TradeValue > existing.TradeValue {
			index[makeNXTKey(entry.Market, entry.Code)] = entry
		}
		if existing, ok := index[makeNXTKey("", entry.Code)]; !ok || entry.TradeValue > existing.TradeValue {
			index[makeNXTKey("", entry.Code)] = entry
		}
	}

	snapshot := nxtSnapshotState{
		Enabled:         len(index) > 0,
		TradingDate:     strings.TrimSpace(payload.TradingDate),
		SetTime:         strings.TrimSpace(payload.SetTime),
		FetchedAt:       strings.TrimSpace(payload.FetchedAt),
		QuoteCount:      payload.QuoteCount,
		TotalTradeValue: payload.TotalTradeValue,
		TotalVolume:     payload.TotalVolume,
		Index:           index,
	}
	if snapshot.TradingDate == "" {
		snapshot.TradingDate = tradingDate
	}

	s.mu.Lock()
	s.nxtCache = nxtSnapshotCache{
		path:        path,
		tradingDate: tradingDate,
		modTime:     info.ModTime(),
		snapshot:    snapshot,
	}
	s.mu.Unlock()

	return snapshot, nil
}

func (snapshot nxtSnapshotState) find(market, code string) (nxtQuoteEntry, bool) {
	if !snapshot.Enabled || strings.TrimSpace(code) == "" {
		return nxtQuoteEntry{}, false
	}
	keys := []string{makeNXTKey(market, code), makeNXTKey("", code)}
	for _, key := range keys {
		if key == "" {
			continue
		}
		entry, ok := snapshot.Index[key]
		if ok {
			return entry, true
		}
	}
	return nxtQuoteEntry{}, false
}
