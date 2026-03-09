package handlers

import (
	"net/http"
	"time"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]any{
			"error":   "요청한 리소스를 찾을 수 없습니다.",
			"message": "Not Found",
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"message":   "KRX Stock Info API Server",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
