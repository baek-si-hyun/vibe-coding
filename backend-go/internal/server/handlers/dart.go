package handlers

import (
	"net/http"
	"strings"

	"investment-news-go/internal/httpx"
)

func (h *Handlers) DartExportFinancials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "Method not allowed"})
		return
	}

	body := map[string]any{}
	httpx.DecodeJSON(r, &body)
	q := r.URL.Query()

	minCap := httpx.ToInt64(body["minMarketCap"], httpx.ToInt64(body["min_market_cap"], minQuantMarketCap))
	if q.Get("min_market_cap") != "" {
		minCap = httpx.ToInt64(q.Get("min_market_cap"), minCap)
	}
	if q.Get("minMarketCap") != "" {
		minCap = httpx.ToInt64(q.Get("minMarketCap"), minCap)
	}
	if minCap < minQuantMarketCap {
		minCap = minQuantMarketCap
	}

	fsDiv := strings.TrimSpace(httpx.ToString(body["fsDiv"]))
	if fsDiv == "" {
		fsDiv = strings.TrimSpace(httpx.ToString(body["fs_div"]))
	}
	if fsDiv == "" {
		fsDiv = strings.TrimSpace(q.Get("fs_div"))
	}
	if fsDiv == "" {
		fsDiv = strings.TrimSpace(q.Get("fsDiv"))
	}

	asOfDate := strings.TrimSpace(httpx.ToString(body["asOfDate"]))
	if asOfDate == "" {
		asOfDate = strings.TrimSpace(httpx.ToString(body["as_of_date"]))
	}
	if asOfDate == "" {
		asOfDate = strings.TrimSpace(q.Get("as_of_date"))
	}
	if asOfDate == "" {
		asOfDate = strings.TrimSpace(q.Get("asOfDate"))
	}

	maxCompanies := httpx.ToInt(body["maxCompanies"], httpx.ToInt(body["max_companies"], 0))
	if q.Get("max_companies") != "" {
		maxCompanies = httpx.ToInt(q.Get("max_companies"), maxCompanies)
	}
	if q.Get("maxCompanies") != "" {
		maxCompanies = httpx.ToInt(q.Get("maxCompanies"), maxCompanies)
	}

	delay := httpx.ToFloat(body["delay"], 0.15)
	if q.Get("delay") != "" {
		delay = httpx.ToFloat(q.Get("delay"), delay)
	}

	outputPath := strings.TrimSpace(httpx.ToString(body["outputPath"]))
	if outputPath == "" {
		outputPath = strings.TrimSpace(httpx.ToString(body["output_path"]))
	}
	if outputPath == "" {
		outputPath = strings.TrimSpace(q.Get("output_path"))
	}
	if outputPath == "" {
		outputPath = strings.TrimSpace(q.Get("outputPath"))
	}

	result, err := h.app.Dart.ExportLargeCapFinancialsCSV(minCap, fsDiv, asOfDate, maxCompanies, delay, outputPath)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "설정되지 않았") ||
			strings.Contains(err.Error(), "형식이 올바르지") ||
			strings.Contains(err.Error(), "알 수 없는 API") ||
			strings.Contains(err.Error(), "없습니다") {
			status = http.StatusBadRequest
		}
		httpx.WriteJSON(w, status, map[string]any{"error": err.Error()})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}
