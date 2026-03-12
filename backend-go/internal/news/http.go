package news

import (
	"encoding/json"
	"net/http"

	"investment-news-go/internal/httpx"
)

func NewHandler(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/news/crawl/resume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
			return
		}

		var body struct {
			Sources []string `json:"sources"`
			Reset   bool     `json:"reset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		result := service.CrawlAPIResume(body.Sources, body.Reset)
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": errMsg})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/api/news/backfill", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
			return
		}

		var body struct {
			Sources           []string `json:"sources"`
			TradingDays       int      `json:"tradingDays"`
			TargetTradingDate string   `json:"targetTradingDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		result := service.BackfillRecentTradingDays(body.TargetTradingDate, body.TradingDays, body.Sources)
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": errMsg})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/api/news/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"files": service.listSavedFiles(),
		})
	})

	mux.HandleFunc("/api/news/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
			return
		}
		q := r.URL.Query()
		page := httpx.ParseIntOrDefault(q.Get("page"), 1)
		limit := httpx.ParseIntOrDefault(q.Get("limit"), defaultPageSize)
		if limit > 100 {
			limit = 100
		}
		filename := q.Get("file")
		search := q.Get("q")
		if search == "" {
			search = q.Get("search")
		}

		result := service.readSavedNewsPaginated(page, limit, search, filename)
		httpx.WriteJSON(w, http.StatusOK, result)
	})

	return mux
}
