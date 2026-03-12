package nxt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"investment-news-go/internal/config"
)

const (
	defaultBaseURL       = "https://www.nextrade.co.kr"
	delayedListPath      = "/brdinfoTime/brdinfoTimeList.do"
	defaultPageUnit      = 1000
	defaultClientTimeout = 20 * time.Second
)

type Quote struct {
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

type Snapshot struct {
	TradingDate     string         `json:"trading_date"`
	SetTime         string         `json:"set_time,omitempty"`
	FetchedAt       string         `json:"fetched_at"`
	SourceURL       string         `json:"source_url"`
	QuoteCount      int            `json:"quote_count"`
	MarketCounts    map[string]int `json:"market_counts,omitempty"`
	TotalTradeValue int64          `json:"total_trade_value"`
	TotalVolume     int64          `json:"total_volume"`
	Items           []Quote        `json:"items"`
}

type Service struct {
	cfg         config.Config
	client      *http.Client
	baseURL     string
	dataDir     string
	snapshotDir string
	latestPath  string
}

func NewService(cfg config.Config) *Service {
	dataDir := filepath.Join(cfg.DataRootDir, "nxt")
	return &Service{
		cfg:         cfg,
		client:      &http.Client{Timeout: defaultClientTimeout},
		baseURL:     defaultBaseURL,
		dataDir:     dataDir,
		snapshotDir: filepath.Join(dataDir, "snapshots"),
		latestPath:  filepath.Join(dataDir, "nxt_snapshot_latest.json"),
	}
}

func (s *Service) CollectAndSave(tradingDate string) (Snapshot, error) {
	snapshot, err := s.FetchSnapshot(tradingDate)
	if err != nil {
		return Snapshot{}, err
	}
	if err := s.saveSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Service) FetchSnapshot(tradingDate string) (Snapshot, error) {
	targetDate := normalizeTradingDate(tradingDate)
	page := 1
	totalPages := 1
	setTime := ""
	items := make([]Quote, 0, 1024)
	sourceURL := strings.TrimRight(s.baseURL, "/") + delayedListPath

	for page <= totalPages {
		payload, err := s.fetchPage(targetDate, page)
		if err != nil {
			return Snapshot{}, err
		}
		if setTime == "" {
			setTime = strings.TrimSpace(toString(payload["setTime"]))
		}
		if parsedTotal := toInt(payload["total"]); parsedTotal > totalPages {
			totalPages = parsedTotal
		}
		items = append(items, parseQuotes(payload["brdinfoTimeList"])...)
		page++
	}

	if len(items) == 0 {
		return Snapshot{}, fmt.Errorf("NXT 지연 시세 데이터가 비어 있습니다")
	}

	resolvedDate := targetDate
	if resolvedDate == "" {
		resolvedDate = items[0].TradingDate
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Market != items[j].Market {
			return items[i].Market < items[j].Market
		}
		if items[i].TradeValue != items[j].TradeValue {
			return items[i].TradeValue > items[j].TradeValue
		}
		if items[i].Code != items[j].Code {
			return items[i].Code < items[j].Code
		}
		return items[i].Name < items[j].Name
	})

	marketCounts := make(map[string]int, 4)
	var totalTradeValue int64
	var totalVolume int64
	for idx := range items {
		if items[idx].TradingDate == "" {
			items[idx].TradingDate = resolvedDate
		}
		marketCounts[items[idx].Market]++
		totalTradeValue += items[idx].TradeValue
		totalVolume += items[idx].Volume
	}

	return Snapshot{
		TradingDate:     resolvedDate,
		SetTime:         setTime,
		FetchedAt:       time.Now().Format(time.RFC3339),
		SourceURL:       sourceURL,
		QuoteCount:      len(items),
		MarketCounts:    marketCounts,
		TotalTradeValue: totalTradeValue,
		TotalVolume:     totalVolume,
		Items:           items,
	}, nil
}

func (s *Service) LoadSnapshot(tradingDate string) (Snapshot, error) {
	path := s.latestPath
	if normalized := normalizeTradingDate(tradingDate); normalized != "" {
		path = s.snapshotPath(normalized)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Service) LatestSnapshotPath() string {
	if s == nil {
		return ""
	}
	return s.latestPath
}

func (s *Service) SnapshotPath(tradingDate string) string {
	if s == nil {
		return ""
	}
	return s.snapshotPath(tradingDate)
}

func (s *Service) fetchPage(tradingDate string, page int) (map[string]any, error) {
	form := url.Values{}
	form.Set("pageIndex", strconv.Itoa(page))
	form.Set("pageUnit", strconv.Itoa(defaultPageUnit))
	if tradingDate != "" {
		form.Set("scAggDd", tradingDate)
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.baseURL, "/")+delayedListPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("NXT API 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("NXT API HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("NXT API 응답 파싱 실패: %w", err)
	}
	return payload, nil
}

func (s *Service) saveSnapshot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.TradingDate) == "" {
		return fmt.Errorf("snapshot trading date is empty")
	}
	if err := os.MkdirAll(s.snapshotDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.snapshotPath(snapshot.TradingDate), content, 0o644); err != nil {
		return err
	}
	return os.WriteFile(s.latestPath, content, 0o644)
}

func (s *Service) snapshotPath(tradingDate string) string {
	return filepath.Join(s.snapshotDir, "nxt_snapshot_"+normalizeTradingDate(tradingDate)+".json")
}

func parseQuotes(raw any) []Quote {
	rows, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Quote, 0, len(rows))
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		code := normalizeShortCode(toString(row["isuSrdCd"]))
		if code == "" {
			code = normalizeShortCode(toString(row["isuCd"]))
		}
		if code == "" {
			continue
		}
		out = append(out, Quote{
			TradingDate:  normalizeTradingDate(toString(row["aggDd"])),
			Market:       normalizeMarket(toString(row["mktNm"])),
			Code:         code,
			ShortCode:    strings.TrimSpace(toString(row["isuSrdCd"])),
			IssueCode:    strings.TrimSpace(toString(row["isuCd"])),
			Name:         strings.TrimSpace(toString(row["isuAbwdNm"])),
			SnapshotAt:   normalizeClock(toString(row["creTime"])),
			BasePrice:    toFloat(row["basePrc"]),
			CurrentPrice: toFloat(row["curPrc"]),
			ChangePrice:  toFloat(row["contrastPrc"]),
			ChangeRate:   toFloat(row["upDownRate"]),
			OpenPrice:    toFloat(row["oppr"]),
			HighPrice:    toFloat(row["hgpr"]),
			LowPrice:     toFloat(row["lwpr"]),
			Volume:       toInt64(row["accTdQty"]),
			TradeValue:   toInt64(row["accTrval"]),
		})
	}
	return out
}

func normalizeTradingDate(raw string) string {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "-", ""))
	if len(value) != 8 {
		return ""
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value
}

func normalizeShortCode(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(strings.ToUpper(value), "A")
	if len(value) < 6 {
		value = strings.Repeat("0", 6-len(value)) + value
	}
	if len(value) != 6 {
		return ""
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value
}

func normalizeMarket(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "KOSPI", "KOSDAQ":
		return value
	default:
		return value
	}
}

func normalizeClock(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) < 4 {
		return ""
	}
	if len(value) >= 6 {
		return value[:2] + ":" + value[2:4] + ":" + value[4:6]
	}
	return value[:2] + ":" + value[2:4]
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func toFloat(v any) float64 {
	raw := strings.ReplaceAll(toString(v), ",", "")
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func toInt(v any) int {
	raw := strings.ReplaceAll(toString(v), ",", "")
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		fallback, ferr := strconv.ParseFloat(raw, 64)
		if ferr != nil {
			return 0
		}
		return int(fallback)
	}
	return parsed
}

func toInt64(v any) int64 {
	raw := strings.ReplaceAll(toString(v), ",", "")
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fallback, ferr := strconv.ParseFloat(raw, 64)
		if ferr != nil {
			return 0
		}
		return int64(fallback)
	}
	return parsed
}

func truncateBody(body []byte) string {
	value := strings.TrimSpace(string(body))
	if len(value) > 400 {
		return value[:400]
	}
	return value
}
