package news

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	pubDateRe = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
	pressRe   = regexp.MustCompile(`(?i)([a-z0-9-]+)\.(daum|naver|yonhap|hani|donga|chosun|joongang|mt|mk|hankyung|etnews|yna|khan|hani|sedaily)\.(net|co\.kr|com)`)
)

func (s *Service) fetchNaverNews(query string, display, start int, sort string) FetchResult {
	if s.cfg.NaverClientID == "" || s.cfg.NaverClientSecret == "" {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: "NAVER_CLIENT_ID/SECRET 미설정"}
	}

	if display < 1 {
		display = 1
	}
	if display > 100 {
		display = 100
	}
	if start < 1 {
		start = 1
	}
	if start > 1000 {
		start = 1000
	}
	if sort != "date" {
		sort = "sim"
	}

	q := url.Values{}
	q.Set("query", query)
	q.Set("display", fmt.Sprintf("%d", display))
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("sort", sort)

	req, err := http.NewRequest(http.MethodGet, "https://openapi.naver.com/v1/search/news.json?"+q.Encode(), nil)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	req.Header.Set("X-Naver-Client-Id", s.cfg.NaverClientID)
	req.Header.Set("X-Naver-Client-Secret", s.cfg.NaverClientSecret)

	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(body))
		errMsg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, msg)
		return FetchResult{
			Items:       []NewsItem{},
			Total:       0,
			Error:       errMsg,
			RateLimited: isRateLimited(resp.StatusCode, errMsg),
		}
	}

	var payload struct {
		Total int `json:"total"`
		Items []struct {
			Title       string `json:"title"`
			Link        string `json:"link"`
			Original    string `json:"originallink"`
			Description string `json:"description"`
			PubDate     string `json:"pubDate"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}

	items := make([]NewsItem, 0, len(payload.Items))
	for _, raw := range payload.Items {
		link := strings.TrimSpace(raw.Link)
		if link == "" {
			link = strings.TrimSpace(raw.Original)
		}
		press := extractPressFromURL(link)
		if press == "" {
			press = extractPressFromURL(raw.Original)
		}
		items = append(items, NewsItem{
			Title:       stripHTML(raw.Title),
			Link:        link,
			Description: stripHTML(raw.Description),
			Press:       press,
			PubDate:     parsePubDateNaver(raw.PubDate),
		})
	}

	return FetchResult{Items: items, Total: payload.Total}
}

func (s *Service) fetchKakaoWeb(query string, size, page int, sort string) FetchResult {
	if s.cfg.KakaoRestAPIKey == "" {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: "KAKAO_REST_API_KEY 미설정"}
	}

	if size < 1 {
		size = 1
	}
	if size > 50 {
		size = 50
	}
	if page < 1 {
		page = 1
	}
	if page > 50 {
		page = 50
	}
	if sort != "recency" {
		sort = "accuracy"
	}

	q := url.Values{}
	q.Set("query", query)
	q.Set("size", fmt.Sprintf("%d", size))
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("sort", sort)

	req, err := http.NewRequest(http.MethodGet, "https://dapi.kakao.com/v2/search/web?"+q.Encode(), nil)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	req.Header.Set("Authorization", "KakaoAK "+s.cfg.KakaoRestAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(body))
		errMsg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, msg)
		return FetchResult{
			Items:       []NewsItem{},
			Total:       0,
			Error:       errMsg,
			RateLimited: isRateLimited(resp.StatusCode, errMsg),
		}
	}

	var payload struct {
		Meta struct {
			TotalCount int `json:"total_count"`
		} `json:"meta"`
		Documents []struct {
			Title    string `json:"title"`
			URL      string `json:"url"`
			Contents string `json:"contents"`
			DateTime string `json:"datetime"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}

	items := make([]NewsItem, 0, len(payload.Documents))
	for _, raw := range payload.Documents {
		link := strings.TrimSpace(raw.URL)
		items = append(items, NewsItem{
			Title:       stripHTML(raw.Title),
			Link:        link,
			Description: stripHTML(raw.Contents),
			Press:       extractPressFromURL(link),
			PubDate:     parsePubDateKakao(raw.DateTime),
		})
	}

	return FetchResult{Items: items, Total: payload.Meta.TotalCount}
}

func (s *Service) fetchNewsAPIProvider(query string, pageSize, page int, minDate string) FetchResult {
	if s.cfg.NewsAPIKey == "" {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: "NEWSAPI_KEY 미설정"}
	}

	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("sortBy", "publishedAt")
	q.Set("searchIn", "title,description")
	if strings.TrimSpace(minDate) != "" {
		q.Set("from", strings.TrimSpace(minDate)+"T00:00:00Z")
	}

	req, err := http.NewRequest(http.MethodGet, "https://newsapi.org/v2/everything?"+q.Encode(), nil)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	req.Header.Set("X-Api-Key", s.cfg.NewsAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(body))
		errMsg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, msg)
		return FetchResult{
			Items:       []NewsItem{},
			Total:       0,
			Error:       errMsg,
			RateLimited: isRateLimited(resp.StatusCode, errMsg),
		}
	}

	var payload struct {
		Status       string `json:"status"`
		Code         string `json:"code"`
		Message      string `json:"message"`
		TotalResults int    `json:"totalResults"`
		Articles     []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return FetchResult{Items: []NewsItem{}, Total: 0, Error: err.Error()}
	}
	if strings.EqualFold(payload.Status, "error") {
		errMsg := strings.TrimSpace(payload.Message)
		if errMsg == "" {
			errMsg = payload.Code
		}
		return FetchResult{
			Items:       []NewsItem{},
			Total:       0,
			Error:       errMsg,
			RateLimited: isRateLimited(resp.StatusCode, errMsg),
		}
	}

	items := make([]NewsItem, 0, len(payload.Articles))
	for _, raw := range payload.Articles {
		items = append(items, NewsItem{
			Title:       strings.TrimSpace(raw.Title),
			Link:        strings.TrimSpace(raw.URL),
			Description: strings.TrimSpace(raw.Description),
			Press:       strings.TrimSpace(raw.Source.Name),
			PubDate:     parsePubDateISO(raw.PublishedAt),
		})
	}

	return FetchResult{Items: items, Total: payload.TotalResults}
}

func (s *Service) FetchNewsAPI(source, query string, maxResults int, minDate string, maxPages int) FetchResult {
	allItems := make([]NewsItem, 0)
	display := 100
	if source != "naver" {
		display = 10
	}
	perPage := 100
	if source == "daum" {
		perPage = 50
	}
	page := 0
	if maxResults <= 0 {
		maxResults = 999999
	}
	if minDate == "" {
		minDate = defaultMinDate
	}

	for {
		if maxPages > 0 && page >= maxPages {
			break
		}

		var result FetchResult
		if source == "naver" {
			start := page*display + 1
			if start > 1000 {
				break
			}
			remaining := 1000 - start + 1
			if remaining < display {
				display = remaining
			}
			result = s.fetchNaverNews(query, display, start, "date")
		} else if source == "daum" {
			if page >= 50 {
				break
			}
			result = s.fetchKakaoWeb(query, perPage, page+1, "recency")
		} else if source == "newsapi" {
			result = s.fetchNewsAPIProvider(query, perPage, page+1, minDate)
		} else {
			return FetchResult{Items: []NewsItem{}, Error: fmt.Sprintf("지원하지 않는 소스: %s", source)}
		}

		if result.Error != "" {
			if result.RateLimited {
				return FetchResult{
					Items:       allItems,
					Total:       len(allItems),
					RateLimited: true,
					Error:       result.Error,
				}
			}
			if page == 0 {
				return result
			}
			break
		}

		for _, it := range result.Items {
			if isValidDate(it.PubDate, minDate) {
				allItems = append(allItems, it)
			}
		}
		if len(allItems) >= maxResults {
			break
		}
		if len(result.Items) == 0 {
			break
		}
		page++
		time.Sleep(100 * time.Millisecond)
	}

	if len(allItems) > maxResults {
		allItems = allItems[:maxResults]
	}
	return FetchResult{Items: allItems, Total: len(allItems)}
}

func isRateLimited(statusCode int, errMsg string) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusForbidden {
		return true
	}
	msg := strings.ToLower(errMsg)
	return strings.Contains(msg, "limit") || strings.Contains(msg, "quota") || strings.Contains(errMsg, "한도")
}

func stripHTML(text string) string {
	if text == "" {
		return ""
	}
	return strings.TrimSpace(htmlTagRe.ReplaceAllString(text, ""))
}

func parsePubDateNaver(pubDate string) string {
	if strings.TrimSpace(pubDate) == "" {
		return ""
	}
	dt, err := mail.ParseDate(pubDate)
	if err != nil {
		return pubDate
	}
	return dt.Format("2006-01-02")
}

func parsePubDateKakao(dt string) string {
	m := pubDateRe.FindStringSubmatch(dt)
	if len(m) == 4 {
		return fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
	}
	return dt
}

func parsePubDateISO(dt string) string {
	if strings.TrimSpace(dt) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(dt))
	if err != nil {
		return dt
	}
	return parsed.Format("2006-01-02")
}

func extractPressFromURL(rawURL string) string {
	m := pressRe.FindStringSubmatch(rawURL)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func isValidDate(pubDate, minDate string) bool {
	if len(pubDate) < 10 {
		return false
	}
	return pubDate >= minDate
}
