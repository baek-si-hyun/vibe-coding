package handlers

import (
	"net/url"
	"strings"

	"investment-news-go/internal/app"
	"investment-news-go/internal/httpx"
)

const minQuantMarketCap int64 = 1_000_000_000_000

type Handlers struct {
	app *app.App
}

func New(application *app.App) *Handlers {
	return &Handlers{app: application}
}

func (h *Handlers) quantMinCap(q url.Values, fallback int64) int64 {
	minCap := httpx.ToInt64(q.Get("min_market_cap"), fallback)
	if q.Get("minMarketCap") != "" {
		minCap = httpx.ToInt64(q.Get("minMarketCap"), minCap)
	}
	if minCap < minQuantMarketCap {
		minCap = minQuantMarketCap
	}
	return minCap
}

func apiIDsFromBodyOrQuery(body map[string]any, q url.Values) []string {
	apiRaw := q.Get("api")
	if apiRaw == "" {
		switch value := body["apiIds"].(type) {
		case string:
			apiRaw = value
		case []any:
			items := make([]string, 0, len(value))
			for _, item := range value {
				items = append(items, strings.TrimSpace(httpx.ToString(item)))
			}
			apiRaw = strings.Join(items, ",")
		}
	}
	return httpx.ParseCommaList(apiRaw)
}
