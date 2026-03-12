package news

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// NXT-inclusive Korea combined trading window.
	marketCloseHour     = 20
	marketCloseMinute   = 0
	preMarketOpenHour   = 8
	preMarketOpenMinute = 0
)

var krxCalendarDirs = []string{"kospi_daily", "kosdaq_daily"}

type WindowNewsItem struct {
	NewsItem
	PublishedAt   string `json:"published_at,omitempty"`
	DatePrecision string `json:"date_precision"`
	WindowBucket  string `json:"window_bucket"`
}

type WindowKeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

type TradingNewsWindow struct {
	TargetTradingDate          string               `json:"target_trading_date"`
	PreviousTradingDate        string               `json:"previous_trading_date"`
	LatestAvailableTradingDate string               `json:"latest_available_trading_date"`
	WindowStart                string               `json:"window_start"`
	WindowEnd                  string               `json:"window_end"`
	CalendarMode               string               `json:"calendar_mode"`
	TimePrecision              string               `json:"time_precision"`
	SourceFile                 string               `json:"source_file,omitempty"`
	SourceUpdatedAt            string               `json:"source_updated_at,omitempty"`
	CoverageNote               string               `json:"coverage_note,omitempty"`
	ItemCount                  int                  `json:"item_count"`
	PreviousDateCount          int                  `json:"previous_date_count"`
	TargetDateCount            int                  `json:"target_date_count"`
	KeywordCounts              []WindowKeywordCount `json:"keyword_counts"`
	Items                      []WindowNewsItem     `json:"items"`
}

type resolvedTradingWindow struct {
	targetTradingDate   string
	previousTradingDate string
	latestTradingDate   string
	windowStart         time.Time
	windowEnd           time.Time
	calendarMode        string
}

func (s *Service) LoadTradingNewsWindow(targetTradingDate string) (TradingNewsWindow, error) {
	loc := seoulLocation()
	window, err := s.resolveTradingNewsWindow(targetTradingDate, loc)
	if err != nil {
		return TradingNewsWindow{}, err
	}

	path := s.getNewsFilepath("")
	if path == "" {
		return TradingNewsWindow{}, fmt.Errorf("저장된 뉴스 CSV를 찾지 못했습니다")
	}
	info, err := os.Stat(path)
	if err != nil {
		return TradingNewsWindow{}, err
	}

	items, err := s.loadSavedNewsCached(path, info.ModTime())
	if err != nil {
		return TradingNewsWindow{}, err
	}

	result := TradingNewsWindow{
		TargetTradingDate:          window.targetTradingDate,
		PreviousTradingDate:        window.previousTradingDate,
		LatestAvailableTradingDate: window.latestTradingDate,
		WindowStart:                window.windowStart.Format(time.RFC3339),
		WindowEnd:                  window.windowEnd.Format(time.RFC3339),
		CalendarMode:               window.calendarMode,
		TimePrecision:              "date_only",
		SourceFile:                 path,
		SourceUpdatedAt:            info.ModTime().Format(time.RFC3339),
		CoverageNote:               "저장된 뉴스 CSV는 pubDate가 일자 단위라 전일 장마감 이후/당일 장전 시점을 정확히 자르지 못합니다. 전일/당일 날짜 버킷 기준으로 집계합니다.",
		Items:                      make([]WindowNewsItem, 0, 512),
	}

	keywordCounts := map[string]int{}
	hasTimestampPrecision := false
	hasDateOnlyPrecision := false

	for _, item := range items {
		if !meetsQualityTier(item, s.cfg.NewsQualityMinTier) {
			continue
		}
		publishedAt, precision, ok := parseStoredNewsTimestamp(item, loc)
		if !ok {
			continue
		}
		bucket, include := classifyWindowBucket(publishedAt, precision, window)
		if !include {
			continue
		}
		if precision != "date_only" {
			hasTimestampPrecision = true
		} else {
			hasDateOnlyPrecision = true
		}

		windowItem := WindowNewsItem{
			NewsItem:      item,
			PublishedAt:   publishedAt.Format(time.RFC3339),
			DatePrecision: precision,
			WindowBucket:  bucket,
		}
		result.Items = append(result.Items, windowItem)

		if bucket == "previous_trading_date" {
			result.PreviousDateCount++
		} else if bucket == "target_trading_date" {
			result.TargetDateCount++
		}

		keyword := strings.TrimSpace(item.Keyword)
		if keyword != "" {
			keywordCounts[keyword]++
		}
	}

	switch {
	case hasTimestampPrecision && hasDateOnlyPrecision:
		result.TimePrecision = "mixed"
		result.CoverageNote = "저장 뉴스에 timestamp가 있는 항목은 정확한 시간 윈도우를 적용하고, date-only 항목은 전일/당일 날짜 버킷 기준으로 집계합니다."
	case hasTimestampPrecision:
		result.TimePrecision = "datetime"
		result.CoverageNote = "저장 뉴스의 발행 시각(timestamp)을 기준으로 전일 장마감 후 ~ 당일 장전 윈도우를 정확하게 집계합니다."
	}

	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].PublishedAt == result.Items[j].PublishedAt {
			return result.Items[i].Title < result.Items[j].Title
		}
		return result.Items[i].PublishedAt > result.Items[j].PublishedAt
	})

	result.KeywordCounts = sortKeywordCounts(keywordCounts)
	result.ItemCount = len(result.Items)
	return result, nil
}

func (s *Service) loadSavedNewsCached(path string, modTime time.Time) ([]NewsItem, error) {
	s.mu.RLock()
	cached := s.cache
	s.mu.RUnlock()

	if cached.path == path && cached.modTime.Equal(modTime) {
		return cloneNewsItems(cached.items), nil
	}

	items := s.readCSVItems(path)
	cloned := cloneNewsItems(items)
	sorted := sortNewsItemsByRecency(cloned)

	s.mu.Lock()
	s.cache = storedNewsCache{
		path:    path,
		modTime: modTime,
		items:   cloned,
		sorted:  sorted,
	}
	s.mu.Unlock()

	return cloneNewsItems(cloned), nil
}

func (s *Service) loadSavedNewsSortedCached(path string, modTime time.Time) ([]NewsItem, error) {
	s.mu.RLock()
	cached := s.cache
	s.mu.RUnlock()

	if cached.path == path && cached.modTime.Equal(modTime) && len(cached.sorted) > 0 {
		return cloneNewsItems(cached.sorted), nil
	}

	items, err := s.loadSavedNewsCached(path, modTime)
	if err != nil {
		return nil, err
	}
	sorted := sortNewsItemsByRecency(items)

	s.mu.Lock()
	if s.cache.path == path && s.cache.modTime.Equal(modTime) {
		s.cache.sorted = cloneNewsItems(sorted)
	}
	s.mu.Unlock()

	return sorted, nil
}

func cloneNewsItems(items []NewsItem) []NewsItem {
	if len(items) == 0 {
		return []NewsItem{}
	}
	out := make([]NewsItem, len(items))
	copy(out, items)
	return out
}

func sortNewsItemsByRecency(items []NewsItem) []NewsItem {
	out := cloneNewsItems(items)
	sort.Slice(out, func(i, j int) bool {
		return newsSortKey(out[i]) > newsSortKey(out[j])
	})
	return out
}

func (s *Service) resolveTradingNewsWindow(targetTradingDate string, loc *time.Location) (resolvedTradingWindow, error) {
	tradingDates, err := s.loadTradingCalendarDates()
	if err != nil {
		return resolvedTradingWindow{}, err
	}
	if len(tradingDates) < 2 {
		return resolvedTradingWindow{}, fmt.Errorf("거래일 캘린더 데이터가 부족합니다")
	}

	latestTradingDate := tradingDates[len(tradingDates)-1]
	target := normalizeTradingDate(targetTradingDate)
	calendarMode := "explicit"
	if target == "" {
		target = nextWeekdayTradingDate(latestTradingDate, loc)
		calendarMode = "latest_plus_weekday"
	}
	if target == "" {
		return resolvedTradingWindow{}, fmt.Errorf("다음 거래일을 추론하지 못했습니다")
	}

	previous := ""
	for _, tradingDate := range tradingDates {
		if tradingDate >= target {
			break
		}
		previous = tradingDate
	}
	if previous == "" {
		return resolvedTradingWindow{}, fmt.Errorf("기준 거래일 %s 이전 거래일을 찾지 못했습니다", target)
	}

	windowStart, err := tradingDateTime(previous, marketCloseHour, marketCloseMinute, loc)
	if err != nil {
		return resolvedTradingWindow{}, err
	}
	windowEnd, err := tradingDateTime(target, preMarketOpenHour, preMarketOpenMinute, loc)
	if err != nil {
		return resolvedTradingWindow{}, err
	}

	return resolvedTradingWindow{
		targetTradingDate:   target,
		previousTradingDate: previous,
		latestTradingDate:   latestTradingDate,
		windowStart:         windowStart,
		windowEnd:           windowEnd,
		calendarMode:        calendarMode,
	}, nil
}

func (s *Service) loadTradingCalendarDates() ([]string, error) {
	calendarPath, err := findTradingCalendarCSV(s.cfg.DataRootDir)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(calendarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	dateIndex := -1
	for i, col := range header {
		if strings.TrimPrefix(strings.TrimSpace(col), "\ufeff") == "BAS_DD" {
			dateIndex = i
			break
		}
	}
	if dateIndex < 0 {
		return nil, fmt.Errorf("거래일 캘린더 CSV에 BAS_DD 컬럼이 없습니다")
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, 512)
	for {
		row, readErr := reader.Read()
		if readErr != nil {
			break
		}
		if dateIndex >= len(row) {
			continue
		}
		value := normalizeTradingDate(row[dateIndex])
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	sort.Strings(out)
	return out, nil
}

func findTradingCalendarCSV(dataRoot string) (string, error) {
	bestPath := ""
	bestSize := int64(-1)

	for _, dirName := range krxCalendarDirs {
		pattern := filepath.Join(dataRoot, dirName, "*.csv")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Strings(paths)
		for _, path := range paths {
			info, statErr := os.Stat(path)
			if statErr != nil || info.IsDir() {
				continue
			}
			if info.Size() > bestSize {
				bestPath = path
				bestSize = info.Size()
			}
		}
	}
	if bestPath == "" {
		return "", fmt.Errorf("거래일 캘린더용 KRX CSV를 찾지 못했습니다")
	}
	return bestPath, nil
}

func parseStoredNewsTimestamp(item NewsItem, loc *time.Location) (time.Time, string, bool) {
	for _, raw := range []string{item.PublishedAt, item.RawPubDate} {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
				return parsed.In(loc), "datetime", true
			}
		}
	}

	value := strings.TrimSpace(item.PubDate)
	if value == "" {
		return time.Time{}, "", false
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		return parsed, "date_only", true
	}
	return time.Time{}, "", false
}

func classifyWindowBucket(publishedAt time.Time, precision string, window resolvedTradingWindow) (string, bool) {
	if precision == "datetime" {
		if publishedAt.Before(window.windowStart) || publishedAt.After(window.windowEnd) {
			return "", false
		}
		if normalizeTradingDate(publishedAt.Format("2006-01-02")) == window.previousTradingDate {
			return "post_close", true
		}
		return "pre_market", true
	}

	dateKey := normalizeTradingDate(publishedAt.Format("2006-01-02"))
	switch dateKey {
	case window.previousTradingDate:
		return "previous_trading_date", true
	case window.targetTradingDate:
		return "target_trading_date", true
	default:
		return "", false
	}
}

func sortKeywordCounts(counts map[string]int) []WindowKeywordCount {
	if len(counts) == 0 {
		return []WindowKeywordCount{}
	}
	out := make([]WindowKeywordCount, 0, len(counts))
	for keyword, count := range counts {
		out = append(out, WindowKeywordCount{
			Keyword: keyword,
			Count:   count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Keyword < out[j].Keyword
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func seoulLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err == nil {
		return loc
	}
	return time.FixedZone("KST", 9*60*60)
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

func tradingDateTime(date string, hour, minute int, loc *time.Location) (time.Time, error) {
	normalized := normalizeTradingDate(date)
	if normalized == "" {
		return time.Time{}, fmt.Errorf("잘못된 거래일 형식: %s", date)
	}
	return time.Date(
		mustAtoi(normalized[0:4]),
		time.Month(mustAtoi(normalized[4:6])),
		mustAtoi(normalized[6:8]),
		hour,
		minute,
		0,
		0,
		loc,
	), nil
}

func nextWeekdayTradingDate(date string, loc *time.Location) string {
	base, err := tradingDateTime(date, 0, 0, loc)
	if err != nil {
		return ""
	}
	for i := 1; i <= 7; i++ {
		candidate := base.AddDate(0, 0, i)
		if candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
			continue
		}
		return candidate.Format("20060102")
	}
	return ""
}

func mustAtoi(raw string) int {
	value := 0
	for _, ch := range raw {
		value = value*10 + int(ch-'0')
	}
	return value
}
