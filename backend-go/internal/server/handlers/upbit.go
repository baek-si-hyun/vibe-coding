package handlers

import (
	"net/http"
	"strconv"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) UpbitScreener(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	minTradeValue24H := 0.0
	if raw := r.URL.Query().Get("min_trade_value_24h"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			minTradeValue24H = parsed
		}
	}

	result, err := h.app.Upbit.GetQuantData(limit, minTradeValue24H)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "서버 오류가 발생했습니다.",
			"message": err.Error(),
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}
