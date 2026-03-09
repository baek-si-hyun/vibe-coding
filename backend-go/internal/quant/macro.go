package quant

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	macroCacheTTL     = 2 * time.Minute
	gdeltMinInterval  = 5 * time.Second
	cboeVIXHistoryURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_History.csv"
)

var (
	gdeltMu            sync.Mutex
	gdeltLastRequestAt time.Time
	gdeltLastHeadlines []MacroHeadline
)

var geopoliticalKeywords = []string{
	"Iran",
	"Israel",
	"Hormuz",
	"Houthi",
	"Middle East",
	"Red Sea",
	"oil",
	"missile",
	"war",
	"strike",
	"blockade",
	"drone",
	"sanctions",
	"중동",
	"이란",
	"이스라엘",
	"호르무즈",
	"후티",
	"미사일",
	"공습",
	"제재",
}

type macroCacheEntry struct {
	expiresAt time.Time
	result    MacroResponse
}

type MacroMetric struct {
	Label         string   `json:"label"`
	Value         *float64 `json:"value,omitempty"`
	Display       string   `json:"display"`
	ChangePercent *float64 `json:"change_percent,omitempty"`
	AsOf          string   `json:"as_of,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	Status        string   `json:"status"`
	Note          string   `json:"note,omitempty"`
}

type MacroHeadline struct {
	Title       string   `json:"title"`
	URL         string   `json:"url,omitempty"`
	Source      string   `json:"source,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

type GeopoliticalSignal struct {
	Score           int             `json:"score"`
	Level           string          `json:"level"`
	MatchedKeywords []string        `json:"matched_keywords,omitempty"`
	Headlines       []MacroHeadline `json:"headlines,omitempty"`
	Providers       []string        `json:"providers,omitempty"`
	Note            string          `json:"note,omitempty"`
}

type MacroResponse struct {
	GeneratedAt  string                 `json:"generated_at"`
	Metrics      map[string]MacroMetric `json:"metrics"`
	Geopolitical GeopoliticalSignal     `json:"geopolitical"`
	Warnings     []string               `json:"warnings,omitempty"`
}

func (s *Service) Macro() (MacroResponse, error) {
	now := time.Now()

	s.mu.RLock()
	if s.macroCache.expiresAt.After(now) {
		cached := s.macroCache.result
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	result := MacroResponse{
		GeneratedAt: now.Format(time.RFC3339),
		Metrics:     map[string]MacroMetric{},
		Warnings:    make([]string, 0, 8),
	}

	metricFetchers := []struct {
		key   string
		label string
		fn    func() (MacroMetric, error)
	}{
		{key: "wti", label: "WTI 원유", fn: s.fetchWTIMetric},
		{key: "vix", label: "VIX 공포지수", fn: s.fetchVIXMetric},
		{key: "gold", label: "XAU/USD", fn: s.fetchGoldMetric},
		{key: "usdkrw", label: "USD/KRW", fn: s.fetchUSDKRWMetric},
		{key: "us10y", label: "미국채 10Y", fn: s.fetchUS10YMetric},
	}

	for _, metricFetcher := range metricFetchers {
		metric, err := metricFetcher.fn()
		if err != nil {
			result.Metrics[metricFetcher.key] = unavailableMetric(metricFetcher.label, "", err.Error())
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", metricFetcher.label, err.Error()))
			continue
		}
		result.Metrics[metricFetcher.key] = metric
	}

	geopolitical, geoWarnings := s.fetchGeopoliticalSignal()
	result.Geopolitical = geopolitical
	result.Warnings = append(result.Warnings, geoWarnings...)

	s.mu.Lock()
	s.macroCache = macroCacheEntry{
		expiresAt: now.Add(macroCacheTTL),
		result:    result,
	}
	s.mu.Unlock()

	return result, nil
}

func unavailableMetric(label, provider, note string) MacroMetric {
	return MacroMetric{
		Label:    label,
		Display:  "-",
		Provider: provider,
		Status:   "unavailable",
		Note:     note,
	}
}

func numericMetric(label, display, provider, asOf string, value float64, change *float64) MacroMetric {
	valueCopy := value
	return MacroMetric{
		Label:         label,
		Value:         &valueCopy,
		Display:       display,
		ChangePercent: change,
		AsOf:          asOf,
		Provider:      provider,
		Status:        "ok",
	}
}

func (s *Service) fetchWTIMetric() (MacroMetric, error) {
	if strings.TrimSpace(s.cfg.FREDAPIKey) == "" {
		return MacroMetric{}, fmt.Errorf("FRED_API_KEY 미설정")
	}
	return s.fetchFREDSeriesMetric("DCOILWTICO", "WTI 원유", "FRED(DCOILWTICO)", func(value float64) string {
		return fmt.Sprintf("$%.2f", value)
	})
}

func (s *Service) fetchUS10YMetric() (MacroMetric, error) {
	if strings.TrimSpace(s.cfg.FREDAPIKey) == "" {
		return MacroMetric{}, fmt.Errorf("FRED_API_KEY 미설정")
	}
	return s.fetchFREDSeriesMetric("DGS10", "미국채 10Y", "FRED(DGS10)", func(value float64) string {
		return fmt.Sprintf("%.2f%%", value)
	})
}

func (s *Service) fetchVIXMetric() (MacroMetric, error) {
	if metric, err := s.fetchCBOEVIXMetric(); err == nil {
		return metric, nil
	}
	if strings.TrimSpace(s.cfg.TwelveDataAPIKey) != "" {
		if metric, err := s.fetchTwelveDataQuote([]string{"VIX", "VIX:CBOE"}, "VIX 공포지수"); err == nil {
			return metric, nil
		}
	}
	if strings.TrimSpace(s.cfg.PolygonAPIKey) != "" {
		if metric, err := s.fetchPolygonVIX(); err == nil {
			return metric, nil
		}
	}
	if strings.TrimSpace(s.cfg.FREDAPIKey) != "" {
		return s.fetchFREDSeriesMetric("VIXCLS", "VIX 공포지수", "FRED(VIXCLS)", func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		})
	}
	return MacroMetric{}, fmt.Errorf("CBOE 데이터 또는 TWELVE_DATA_API_KEY, POLYGON_API_KEY, FRED_API_KEY 필요")
}

func (s *Service) fetchCBOEVIXMetric() (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, cboeVIXHistoryURL, nil)
	if err != nil {
		return MacroMetric{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return MacroMetric{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MacroMetric{}, err
	}
	if resp.StatusCode >= 400 {
		return MacroMetric{}, fmt.Errorf("HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	reader := csv.NewReader(strings.NewReader(string(body)))
	rows, err := reader.ReadAll()
	if err != nil {
		return MacroMetric{}, err
	}
	if len(rows) < 2 {
		return MacroMetric{}, fmt.Errorf("CBOE 응답에 VIX 데이터가 없습니다")
	}

	dateIdx := -1
	closeIdx := -1
	for i, column := range rows[0] {
		switch strings.ToUpper(strings.TrimSpace(column)) {
		case "DATE":
			dateIdx = i
		case "CLOSE":
			closeIdx = i
		}
	}
	if dateIdx < 0 || closeIdx < 0 {
		return MacroMetric{}, fmt.Errorf("CBOE 응답 컬럼 형식이 예상과 다릅니다")
	}

	type vixPoint struct {
		date  string
		value float64
	}

	values := make([]vixPoint, 0, 2)
	for i := len(rows) - 1; i >= 1; i-- {
		row := rows[i]
		maxIdx := dateIdx
		if closeIdx > maxIdx {
			maxIdx = closeIdx
		}
		if len(row) <= maxIdx {
			continue
		}

		dateRaw := strings.TrimSpace(row[dateIdx])
		closeRaw := strings.TrimSpace(row[closeIdx])
		if dateRaw == "" || closeRaw == "" {
			continue
		}

		value, parseErr := strconv.ParseFloat(strings.ReplaceAll(closeRaw, ",", ""), 64)
		if parseErr != nil {
			continue
		}
		if parsedDate, dateErr := time.Parse("01/02/2006", dateRaw); dateErr == nil {
			dateRaw = parsedDate.Format("2006-01-02")
		}

		values = append(values, vixPoint{date: dateRaw, value: value})
		if len(values) >= 2 {
			break
		}
	}
	if len(values) == 0 {
		return MacroMetric{}, fmt.Errorf("CBOE 응답에서 유효한 VIX 가격을 찾지 못했습니다")
	}

	latest := values[0]
	var change *float64
	if len(values) > 1 && values[1].value != 0 {
		changeValue := ((latest.value / values[1].value) - 1) * 100
		change = &changeValue
	}

	return numericMetric("VIX 공포지수", fmt.Sprintf("%.2f", latest.value), "CBOE(VIX_History.csv)", latest.date, latest.value, change), nil
}

func (s *Service) fetchUSDKRWMetric() (MacroMetric, error) {
	var lastErr error

	if strings.TrimSpace(s.cfg.AlphaVantageAPIKey) != "" {
		if metric, err := s.fetchAlphaVantageFX("USD", "KRW", "USD/KRW", "Alpha Vantage FX", func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(s.cfg.PolygonAPIKey) != "" {
		if metric, err := s.fetchPolygonForexMetric("USD/KRW", "C:USDKRW", "Polygon(C:USDKRW)", func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(s.cfg.ExchangeRateAPIKey) != "" {
		if metric, err := s.fetchExchangeRateHostFX("USD", "KRW", "USD/KRW", func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(s.cfg.FREDAPIKey) != "" {
		if metric, err := s.fetchFREDSeriesMetric("DEXKOUS", "USD/KRW", "FRED(DEXKOUS)", func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return MacroMetric{}, lastErr
	}
	return MacroMetric{}, fmt.Errorf("ALPHA_VANTAGE_API_KEY 또는 POLYGON_API_KEY 또는 FRED_API_KEY 필요")
}

func (s *Service) fetchGoldMetric() (MacroMetric, error) {
	var lastErr error

	if strings.TrimSpace(s.cfg.AlphaVantageAPIKey) != "" {
		if metric, err := s.fetchAlphaVantageFX("XAU", "USD", "XAU/USD", "Alpha Vantage FX", func(value float64) string {
			return fmt.Sprintf("$%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(s.cfg.PolygonAPIKey) != "" {
		if metric, err := s.fetchPolygonForexMetric("XAU/USD", "C:XAUUSD", "Polygon(C:XAUUSD)", func(value float64) string {
			return fmt.Sprintf("$%.2f", value)
		}); err == nil {
			return metric, nil
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(s.cfg.MetalsAPIKey) != "" {
		return s.fetchMetalsFX("XAU", "USD", "XAU/USD", func(value float64) string {
			return fmt.Sprintf("$%.2f", value)
		})
	}
	if lastErr != nil {
		return MacroMetric{}, lastErr
	}
	return MacroMetric{}, fmt.Errorf("ALPHA_VANTAGE_API_KEY 또는 POLYGON_API_KEY 필요 (METALS_API_KEY 선택)")
}

func (s *Service) fetchFREDSeriesMetric(seriesID, label, provider string, formatter func(float64) string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("series_id", seriesID)
	params.Set("api_key", s.cfg.FREDAPIKey)
	params.Set("file_type", "json")
	params.Set("sort_order", "desc")
	params.Set("limit", "5")

	var payload struct {
		ErrorMessage string `json:"error_message"`
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := fetchJSON(client, "https://api.stlouisfed.org/fred/series/observations?"+params.Encode(), nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if strings.TrimSpace(payload.ErrorMessage) != "" {
		return MacroMetric{}, fmt.Errorf(strings.TrimSpace(payload.ErrorMessage))
	}

	values := make([]struct {
		date  string
		value float64
	}, 0, len(payload.Observations))
	for _, observation := range payload.Observations {
		value, err := strconv.ParseFloat(strings.TrimSpace(observation.Value), 64)
		if err != nil {
			continue
		}
		values = append(values, struct {
			date  string
			value float64
		}{date: observation.Date, value: value})
	}
	if len(values) == 0 {
		return MacroMetric{}, fmt.Errorf("FRED 응답에 유효한 데이터가 없습니다")
	}

	latest := values[0]
	var change *float64
	if len(values) > 1 && values[1].value != 0 {
		changeValue := ((latest.value / values[1].value) - 1) * 100
		change = &changeValue
	}

	return numericMetric(label, formatter(latest.value), provider, latest.date, latest.value, change), nil
}

func (s *Service) fetchTwelveDataQuote(symbols []string, label string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	var lastErr error

	for _, symbol := range symbols {
		params := url.Values{}
		params.Set("symbol", symbol)
		params.Set("apikey", s.cfg.TwelveDataAPIKey)

		payload := map[string]any{}
		if err := fetchJSON(client, "https://api.twelvedata.com/quote?"+params.Encode(), nil, &payload); err != nil {
			lastErr = err
			continue
		}
		if errMsg := extractGenericAPIError(payload); errMsg != "" {
			lastErr = fmt.Errorf(errMsg)
			continue
		}

		value, ok := anyToFloat(payload["close"])
		if !ok {
			value, ok = anyToFloat(payload["price"])
		}
		if !ok {
			lastErr = fmt.Errorf("Twelve Data 응답에 가격이 없습니다")
			continue
		}

		var change *float64
		if pct, ok := anyToFloat(payload["percent_change"]); ok {
			change = &pct
		}

		asOf := firstString(payload["datetime"], payload["timestamp"])
		return numericMetric(label, fmt.Sprintf("%.2f", value), "Twelve Data", asOf, value, change), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("Twelve Data 응답을 해석할 수 없습니다")
	}
	return MacroMetric{}, lastErr
}

func (s *Service) fetchPolygonVIX() (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("adjusted", "true")
	params.Set("apiKey", s.cfg.PolygonAPIKey)

	var payload struct {
		Results []struct {
			Open      float64 `json:"o"`
			Close     float64 `json:"c"`
			Timestamp int64   `json:"t"`
		} `json:"results"`
		Error  string `json:"error"`
		Status string `json:"status"`
	}
	if err := fetchJSON(client, "https://api.polygon.io/v2/aggs/ticker/I:VIX/prev?"+params.Encode(), nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if strings.TrimSpace(payload.Error) != "" {
		return MacroMetric{}, fmt.Errorf(strings.TrimSpace(payload.Error))
	}
	if len(payload.Results) == 0 {
		return MacroMetric{}, fmt.Errorf("Polygon 응답에 VIX 데이터가 없습니다")
	}

	latest := payload.Results[0]
	var change *float64
	if latest.Open != 0 {
		changeValue := ((latest.Close / latest.Open) - 1) * 100
		change = &changeValue
	}

	return numericMetric("VIX 공포지수", fmt.Sprintf("%.2f", latest.Close), "Polygon(I:VIX)", timeFromMillis(latest.Timestamp), latest.Close, change), nil
}

func (s *Service) fetchPolygonForexMetric(label, ticker, provider string, formatter func(float64) string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("adjusted", "true")
	params.Set("apiKey", s.cfg.PolygonAPIKey)

	var payload struct {
		Results []struct {
			Open      float64 `json:"o"`
			Close     float64 `json:"c"`
			Timestamp any     `json:"t"`
		} `json:"results"`
		Error string `json:"error"`
	}
	url := fmt.Sprintf("https://api.polygon.io/v2/aggs/ticker/%s/prev?%s", ticker, params.Encode())
	if err := fetchJSON(client, url, nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if strings.TrimSpace(payload.Error) != "" {
		return MacroMetric{}, fmt.Errorf(strings.TrimSpace(payload.Error))
	}
	if len(payload.Results) == 0 {
		return MacroMetric{}, fmt.Errorf("Polygon 응답에 %s 데이터가 없습니다", label)
	}

	latest := payload.Results[0]
	var change *float64
	if latest.Open != 0 {
		changeValue := ((latest.Close / latest.Open) - 1) * 100
		change = &changeValue
	}
	asOf := ""
	if ts, ok := anyToInt64(latest.Timestamp); ok {
		asOf = timeFromMillis(ts)
	}
	return numericMetric(label, formatter(latest.Close), provider, asOf, latest.Close, change), nil
}

func (s *Service) fetchAlphaVantageFX(from, to, label, provider string, formatter func(float64) string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("function", "CURRENCY_EXCHANGE_RATE")
	params.Set("from_currency", from)
	params.Set("to_currency", to)
	params.Set("apikey", s.cfg.AlphaVantageAPIKey)

	payload := map[string]any{}
	if err := fetchJSON(client, "https://www.alphavantage.co/query?"+params.Encode(), nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if errMsg := extractGenericAPIError(payload); errMsg != "" {
		return MacroMetric{}, fmt.Errorf(errMsg)
	}

	body, ok := payload["Realtime Currency Exchange Rate"].(map[string]any)
	if !ok {
		return MacroMetric{}, fmt.Errorf("Alpha Vantage 응답을 해석할 수 없습니다")
	}

	value, ok := anyToFloat(body["5. Exchange Rate"])
	if !ok {
		return MacroMetric{}, fmt.Errorf("Alpha Vantage 환율 값이 없습니다")
	}
	asOf := firstString(body["6. Last Refreshed"])

	return numericMetric(label, formatter(value), provider, asOf, value, nil), nil
}

func (s *Service) fetchMetalsFX(base, symbol, label string, formatter func(float64) string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("access_key", s.cfg.MetalsAPIKey)
	params.Set("base", base)
	params.Set("symbols", symbol)

	var payload struct {
		Success   bool               `json:"success"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
		Error     struct {
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := fetchJSON(client, "https://metals-api.com/api/latest?"+params.Encode(), nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if !payload.Success {
		return MacroMetric{}, fmt.Errorf(strings.TrimSpace(payload.Error.Info))
	}
	value, ok := payload.Rates[symbol]
	if !ok || value == 0 {
		return MacroMetric{}, fmt.Errorf("Metals API 응답에 %s 값이 없습니다", symbol)
	}

	return numericMetric(label, formatter(value), "Metals API", timeFromSeconds(payload.Timestamp), value, nil), nil
}

func (s *Service) fetchExchangeRateHostFX(base, symbol, label string, formatter func(float64) string) (MacroMetric, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("access_key", s.cfg.ExchangeRateAPIKey)
	params.Set("source", base)
	params.Set("currencies", symbol)

	var payload struct {
		Success bool               `json:"success"`
		Quotes  map[string]float64 `json:"quotes"`
		Error   struct {
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := fetchJSON(client, "https://api.exchangerate.host/live?"+params.Encode(), nil, &payload); err != nil {
		return MacroMetric{}, err
	}
	if !payload.Success {
		return MacroMetric{}, fmt.Errorf(strings.TrimSpace(payload.Error.Info))
	}
	key := base + symbol
	value, ok := payload.Quotes[key]
	if !ok || value == 0 {
		return MacroMetric{}, fmt.Errorf("exchangerate.host 응답에 %s 값이 없습니다", key)
	}

	return numericMetric(label, formatter(value), "exchangerate.host", "", value, nil), nil
}

func (s *Service) fetchGeopoliticalSignal() (GeopoliticalSignal, []string) {
	query := `("Middle East" OR Iran OR Israel OR Hormuz OR Houthi OR "Red Sea" OR 중동 OR 이란 OR 이스라엘 OR 호르무즈) AND (attack OR strike OR missile OR war OR blockade OR oil OR sanctions OR 공습 OR 미사일 OR 전쟁 OR 봉쇄)`
	headlines := make([]MacroHeadline, 0, 12)
	providers := make([]string, 0, 2)
	warnings := make([]string, 0, 2)

	if strings.TrimSpace(s.cfg.NewsAPIKey) != "" {
		items, err := s.fetchNewsAPIHeadlines(query)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("NewsAPI: %s", err.Error()))
		} else if len(items) > 0 {
			headlines = append(headlines, items...)
			providers = append(providers, "NewsAPI")
		}
	} else {
		warnings = append(warnings, "NewsAPI: NEWSAPI_KEY 미설정")
	}

	items, err := s.fetchGDELTHeadlines(query)
	if err != nil {
		// NewsAPI 데이터가 이미 있으면 GDELT 오류는 보조 소스 실패로 처리한다.
		if len(headlines) == 0 {
			warnings = append(warnings, fmt.Sprintf("GDELT: %s", err.Error()))
		}
	} else if len(items) > 0 {
		headlines = append(headlines, items...)
		providers = append(providers, "GDELT")
	}

	deduped := dedupeHeadlines(headlines)
	latestOnly := keepLatestPublishedHeadlines(deduped)
	matchedKeywords := collectMatchedKeywords(latestOnly)
	score := computeGeopoliticalScore(latestOnly, matchedKeywords)

	return GeopoliticalSignal{
		Score:           score,
		Level:           geopoliticalLevel(score),
		MatchedKeywords: matchedKeywords,
		Headlines:       latestOnly,
		Providers:       providers,
		Note:            strings.Join(warnings, " | "),
	}, warnings
}

func (s *Service) fetchNewsAPIHeadlines(query string) ([]MacroHeadline, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("q", query)
	params.Set("pageSize", "8")
	params.Set("page", "1")
	params.Set("sortBy", "publishedAt")
	params.Set("searchIn", "title,description")
	params.Set("apiKey", s.cfg.NewsAPIKey)

	req, err := http.NewRequest(http.MethodGet, "https://newsapi.org/v2/everything?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", s.cfg.NewsAPIKey)

	var payload struct {
		Status   string `json:"status"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Articles []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		} `json:"articles"`
	}
	if err := fetchJSONRequest(client, req, &payload); err != nil {
		return nil, err
	}
	if strings.EqualFold(payload.Status, "error") {
		return nil, fmt.Errorf(strings.TrimSpace(firstString(payload.Message, payload.Code)))
	}

	out := make([]MacroHeadline, 0, len(payload.Articles))
	for _, article := range payload.Articles {
		headline := MacroHeadline{
			Title:       strings.TrimSpace(article.Title),
			URL:         strings.TrimSpace(article.URL),
			Source:      strings.TrimSpace(article.Source.Name),
			PublishedAt: strings.TrimSpace(article.PublishedAt),
			Keywords:    matchedGeopoliticalKeywords(article.Title),
		}
		if headline.Title != "" {
			out = append(out, headline)
		}
	}
	return out, nil
}

func (s *Service) fetchGDELTHeadlines(query string) ([]MacroHeadline, error) {
	gdeltMu.Lock()
	defer gdeltMu.Unlock()

	if wait := gdeltMinInterval - time.Since(gdeltLastRequestAt); wait > 0 {
		time.Sleep(wait)
	}
	gdeltLastRequestAt = time.Now()

	client := &http.Client{Timeout: 12 * time.Second}
	params := url.Values{}
	params.Set("query", query)
	params.Set("mode", "ArtList")
	params.Set("maxrecords", "8")
	params.Set("format", "json")
	params.Set("sort", "HybridRel")
	params.Set("timespan", "3days")

	var payload struct {
		Articles []struct {
			URL      string `json:"url"`
			Title    string `json:"title"`
			Domain   string `json:"domain"`
			SeenDate string `json:"seendate"`
		} `json:"articles"`
	}
	call := func() ([]MacroHeadline, error) {
		payload.Articles = payload.Articles[:0]
		if err := fetchJSON(client, "https://api.gdeltproject.org/api/v2/doc/doc?"+params.Encode(), nil, &payload); err != nil {
			return nil, err
		}
		return toMacroHeadlines(payload.Articles), nil
	}

	out, err := call()
	if err != nil {
		if isGDELTRateLimit(err) {
			time.Sleep(gdeltMinInterval)
			gdeltLastRequestAt = time.Now()
			retryOut, retryErr := call()
			if retryErr == nil {
				gdeltLastHeadlines = cloneHeadlines(retryOut)
				return retryOut, nil
			}
			if len(gdeltLastHeadlines) > 0 {
				return cloneHeadlines(gdeltLastHeadlines), nil
			}
			return nil, retryErr
		}
		if len(gdeltLastHeadlines) > 0 {
			return cloneHeadlines(gdeltLastHeadlines), nil
		}
		return nil, err
	}
	gdeltLastHeadlines = cloneHeadlines(out)
	return out, nil
}

func toMacroHeadlines(items []struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Domain   string `json:"domain"`
	SeenDate string `json:"seendate"`
}) []MacroHeadline {
	out := make([]MacroHeadline, 0, len(items))
	for _, article := range items {
		headline := MacroHeadline{
			Title:       strings.TrimSpace(article.Title),
			URL:         strings.TrimSpace(article.URL),
			Source:      strings.TrimSpace(article.Domain),
			PublishedAt: parseGDELTSeenDate(article.SeenDate),
			Keywords:    matchedGeopoliticalKeywords(article.Title),
		}
		if headline.Title != "" {
			out = append(out, headline)
		}
	}
	return out
}

func cloneHeadlines(items []MacroHeadline) []MacroHeadline {
	if len(items) == 0 {
		return nil
	}
	out := make([]MacroHeadline, 0, len(items))
	for _, item := range items {
		cloned := item
		if len(item.Keywords) > 0 {
			cloned.Keywords = append([]string(nil), item.Keywords...)
		}
		out = append(out, cloned)
	}
	return out
}

func isGDELTRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 429") || strings.Contains(msg, "one every 5 seconds")
}

func fetchJSON(client *http.Client, rawURL string, headers map[string]string, target any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return fetchJSONRequest(client, req, target)
}

func fetchJSONRequest(client *http.Client, req *http.Request, target any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

func extractGenericAPIError(payload map[string]any) string {
	for _, key := range []string{"Error Message", "Information", "Note", "message"} {
		if value := firstString(payload[key]); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if status := firstString(payload["status"]); strings.EqualFold(status, "error") {
		return strings.TrimSpace(firstString(payload["message"], payload["code"]))
	}
	return ""
}

func anyToFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func anyToInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
		f, ferr := typed.Float64()
		return int64(f), ferr == nil
	case string:
		s := strings.TrimSpace(typed)
		if s == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
			return parsed, true
		}
		f, err := strconv.ParseFloat(s, 64)
		return int64(f), err == nil
	default:
		return 0, false
	}
}

func firstString(values ...any) string {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if strings.TrimSpace(typed.String()) != "" {
				return strings.TrimSpace(typed.String())
			}
		}
	}
	return ""
}

func timeFromMillis(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.UnixMilli(timestamp).Format(time.RFC3339)
}

func timeFromSeconds(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format(time.RFC3339)
}

func parseGDELTSeenDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := time.Parse("20060102T150405Z", trimmed)
	if err != nil {
		return trimmed
	}
	return parsed.Format(time.RFC3339)
}

func matchedGeopoliticalKeywords(text string) []string {
	lower := strings.ToLower(text)
	counts := make([]string, 0, 4)
	for _, keyword := range geopoliticalKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			counts = append(counts, keyword)
		}
		if len(counts) >= 4 {
			break
		}
	}
	return counts
}

func dedupeHeadlines(items []MacroHeadline) []MacroHeadline {
	seen := map[string]struct{}{}
	out := make([]MacroHeadline, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.URL))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(item.Title))
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func parseHeadlinePublishedAt(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func keepLatestPublishedHeadlines(items []MacroHeadline) []MacroHeadline {
	if len(items) == 0 {
		return items
	}

	latest := time.Time{}
	hasLatest := false
	for _, item := range items {
		parsed, ok := parseHeadlinePublishedAt(item.PublishedAt)
		if !ok {
			continue
		}
		if !hasLatest || parsed.After(latest) {
			latest = parsed
			hasLatest = true
		}
	}

	if !hasLatest {
		return []MacroHeadline{items[0]}
	}

	latestY, latestM, latestD := latest.Date()
	out := make([]MacroHeadline, 0, len(items))
	for _, item := range items {
		parsed, ok := parseHeadlinePublishedAt(item.PublishedAt)
		if !ok {
			continue
		}
		y, m, d := parsed.Date()
		if y == latestY && m == latestM && d == latestD {
			out = append(out, item)
		}
	}

	if len(out) == 0 {
		return []MacroHeadline{items[0]}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func collectMatchedKeywords(items []MacroHeadline) []string {
	counts := map[string]int{}
	for _, item := range items {
		for _, keyword := range item.Keywords {
			counts[keyword]++
		}
	}
	type entry struct {
		keyword string
		count   int
	}
	entries := make([]entry, 0, len(counts))
	for keyword, count := range counts {
		entries = append(entries, entry{keyword: keyword, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].keyword < entries[j].keyword
	})

	limit := 6
	if len(entries) < limit {
		limit = len(entries)
	}
	out := make([]string, 0, limit)
	for _, item := range entries {
		out = append(out, item.keyword)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func computeGeopoliticalScore(items []MacroHeadline, keywords []string) int {
	if len(items) == 0 {
		return 20
	}
	severeHits := 0
	for _, item := range items {
		text := strings.ToLower(item.Title)
		for _, keyword := range []string{"attack", "strike", "missile", "war", "blockade", "hormuz", "oil", "drone", "sanction", "공습", "전쟁", "미사일", "봉쇄"} {
			if strings.Contains(text, keyword) {
				severeHits++
			}
		}
	}

	score := 20 + len(items)*6 + len(keywords)*5 + severeHits*3
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}

func geopoliticalLevel(score int) string {
	switch {
	case score >= 80:
		return "극위험"
	case score >= 60:
		return "위험"
	case score >= 40:
		return "주의"
	default:
		return "안정"
	}
}
