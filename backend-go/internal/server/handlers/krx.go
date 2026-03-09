package handlers

import (
	"net/http"
	"strings"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) KRXEndpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.app.KRX.GetEndpoints())
}

func (h *Handlers) KRXFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	q := r.URL.Query()
	apiID := strings.TrimSpace(q.Get("api"))
	limit := httpx.ToInt(q.Get("limit"), 200)
	offset := httpx.ToInt(q.Get("offset"), 0)

	result, err := h.app.KRX.ListFiles(apiID, limit, offset)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "알 수 없는 API") {
			status = http.StatusBadRequest
		}
		httpx.WriteJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) KRXCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	body := map[string]any{}
	httpx.DecodeJSON(r, &body)

	date := r.URL.Query().Get("date")
	if date == "" {
		if value, ok := body["date"].(string); ok {
			date = value
		}
	}

	result, err := h.app.KRX.CollectAndSave(date, apiIDsFromBodyOrQuery(body, r.URL.Query()))
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "날짜 형식") {
			status = http.StatusBadRequest
		}
		httpx.WriteJSON(w, status, map[string]any{"error": err.Error()})
		return
	}

	out := map[string]any{"success": true}
	for key, value := range result {
		out[key] = value
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handlers) KRXCollectResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	body := map[string]any{}
	httpx.DecodeJSON(r, &body)

	delay := httpx.ToFloat(body["delay"], 2.0)
	maxDates := httpx.ToInt(body["maxDates"], 0)
	reset := httpx.ToBool(body["reset"])
	apiIDs := apiIDsFromBodyOrQuery(body, r.URL.Query())

	maxDatesCap := 8
	if len(apiIDs) == 0 || len(apiIDs) > 1 {
		maxDatesCap = 2
	}
	if maxDates <= 0 {
		maxDates = maxDatesCap
	} else if maxDates > maxDatesCap {
		maxDates = maxDatesCap
	}

	result, err := h.app.KRX.CollectBatchResume(delay, maxDates, reset, apiIDs)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "API 키") ||
			strings.Contains(err.Error(), "KRX_OPENAPI_KEY") ||
			strings.Contains(err.Error(), "알 수 없는 API") {
			status = http.StatusBadRequest
		}
		httpx.WriteJSON(w, status, map[string]any{"error": err.Error(), "message": err.Error()})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) KRXDynamicFetch(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/api/")
	if tail == "" || strings.Contains(tail, "/") {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]any{
			"error":   "요청한 리소스를 찾을 수 없습니다.",
			"message": "Not Found",
		})
		return
	}
	if r.Method != http.MethodGet {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	result, err := h.app.KRX.FetchData(tail, r.URL.Query().Get("date"))
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "날짜 형식") || strings.Contains(err.Error(), "알 수 없는 API") {
			status = http.StatusBadRequest
		}
		httpx.WriteJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
