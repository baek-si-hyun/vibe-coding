package handlers

import (
	"net/http"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) BithumbScreener(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	result, err := h.app.Bithumb.GetScreenerData(r.URL.Query().Get("mode"))
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "서버 오류가 발생했습니다.",
			"message": err.Error(),
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}
