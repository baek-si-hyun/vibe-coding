package handlers

import (
	"net/http"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) QuantRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	q := r.URL.Query()
	result, err := h.app.Quant.Rank(
		q.Get("market"),
		httpx.ToInt(q.Get("limit"), 30),
		h.quantMinCap(q, minQuantMarketCap),
	)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) QuantReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	q := r.URL.Query()
	result, err := h.app.Quant.Report(
		q.Get("market"),
		httpx.ToInt(q.Get("limit"), 20),
		h.quantMinCap(q, minQuantMarketCap),
	)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) QuantMacro(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	result, err := h.app.Quant.Macro()
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
