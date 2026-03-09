package krx

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"investment-news-go/internal/config"
)

const (
	oneTrillion     int64 = 1_000_000_000_000
	defaultDelaySec       = 2.0
)

var (
	mktCapCols    = []string{"MKTCAP", "mkp", "시가총액"}
	nameCols      = []string{"ISU_NM", "isuNm", "itmsNm", "isuKorNm", "종목명"}
	codeCols      = []string{"ISU_CD", "ISU_SRT_CD", "srtnCd", "단축코드", "종목코드"}
	dateCols      = []string{"BAS_DD", "basDd", "날짜"}
	codeMatchCols = []string{"ISU_CD", "ISU_SRT_CD", "srtnCd"}

	preferredAPIOrder = []string{
		"kospi_daily",
		"kosdaq_daily",
		"kospi_basic",
		"kosdaq_basic",
		"smb_bond_daily",
		"bond_index_daily",
		"gold_daily",
		"etf_daily",
		"kosdaq_index_daily",
		"krx_index_daily",
		"bond_daily",
	}

	illegalFilenameRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
)

type Service struct {
	cfg          config.Config
	client       *http.Client
	dataDir      string
	progressFile string
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "http error"
	}
	body := strings.TrimSpace(e.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Sprintf("HTTP %d 에러 발생: %s", e.StatusCode, body)
}

type progressState struct {
	LastDate       string              `json:"last_date,omitempty"`
	TotalDatesDone int                 `json:"total_dates_done"`
	ByDate         map[string][]string `json:"by_date"`
	LastUpdated    string              `json:"last_updated,omitempty"`
}

func NewService(cfg config.Config) *Service {
	dataDir := cfg.DataRootDir
	return &Service{
		cfg:          cfg,
		client:       &http.Client{Timeout: 15 * time.Second},
		dataDir:      dataDir,
		progressFile: filepath.Join(dataDir, "krx_collect_progress.json"),
	}
}

func (s *Service) GetEndpoints() map[string]any {
	return map[string]any{
		"endpoints":      s.cfg.APIEndpoints,
		"available_apis": orderedAPIIDs(s.cfg.APIEndpoints),
	}
}

func orderedAPIIDs(m map[string]config.APIEndpoint) []string {
	out := make([]string, 0, len(m))
	seen := map[string]struct{}{}
	for _, id := range preferredAPIOrder {
		if _, ok := m[id]; ok {
			out = append(out, id)
			seen[id] = struct{}{}
		}
	}
	extra := make([]string, 0)
	for id := range m {
		if _, ok := seen[id]; !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	out = append(out, extra...)
	return out
}

func (s *Service) getAPIDir(apiID string) string {
	return filepath.Join(s.dataDir, apiID)
}

func parseYYYYMMDD(raw string) (time.Time, error) {
	return time.Parse("20060102", raw)
}

func normalizeDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty date")
	}
	t, err := parseYYYYMMDD(raw)
	if err != nil {
		return "", err
	}
	return t.Format("20060102"), nil
}

func (s *Service) FetchData(apiID, date string) (map[string]any, error) {
	ep, ok := s.cfg.APIEndpoints[apiID]
	if !ok {
		return nil, fmt.Errorf("알 수 없는 API: %s", apiID)
	}

	if strings.TrimSpace(date) == "" {
		date = time.Now().AddDate(0, 0, -1).Format("20060102")
	}
	normDate, err := normalizeDate(date)
	if err != nil {
		return nil, fmt.Errorf("날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: %s)", date)
	}
	if strings.TrimSpace(s.cfg.KRXAPIKey) == "" {
		return nil, errors.New("API 키가 설정되지 않았습니다. .env 파일에 KRX_API_KEY를 설정해주세요.")
	}

	rows, err := s.fetchDataRows(apiID, normDate)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"basDd":     normDate,
		"fetchedAt": time.Now().Format(time.RFC3339),
		"count":     len(rows),
		"data":      rows,
		"url":       ep.URL,
	}, nil
}

func (s *Service) fetchDataRows(apiID, date string) ([]map[string]any, error) {
	ep, ok := s.cfg.APIEndpoints[apiID]
	if !ok {
		return nil, fmt.Errorf("알 수 없는 API: %s", apiID)
	}
	u, err := url.Parse(ep.URL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("basDd", date)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("AUTH_KEY", s.cfg.KRXAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API 호출 실패: %s", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("API 호출 실패: %s", err.Error())
	}

	raw := payload["OutBlock_1"]
	return toRows(raw), nil
}

func toRows(raw any) []map[string]any {
	switch v := raw.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
		return rows
	case map[string]any:
		return []map[string]any{v}
	default:
		return []map[string]any{}
	}
}

func detectCol(keys []string, candidates []string) string {
	set := map[string]struct{}{}
	for _, k := range keys {
		set[k] = struct{}{}
	}
	for _, c := range candidates {
		if _, ok := set[c]; ok {
			return c
		}
	}
	return ""
}

func parseMktCap(v any) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0
		}
		return int64(f)
	default:
		s := strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(v)), ",", "")
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(f)
	}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeCode(v any) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" {
		return ""
	}
	if isDigits(s) {
		return fmt.Sprintf("%06s", s)
	}
	if len(s) > 6 {
		prefix := s[:len(s)-6]
		suffix := s[len(s)-6:]
		if isDigits(prefix) && isDigits(suffix) {
			return fmt.Sprintf("%06s", suffix)
		}
	}
	return s
}

func keysOfMap(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func filterDailyByMktCap(rows []map[string]any, minMktCap int64) ([]map[string]any, map[string]struct{}) {
	if len(rows) == 0 {
		return []map[string]any{}, map[string]struct{}{}
	}
	mktCol := detectCol(keysOfMap(rows[0]), mktCapCols)
	if mktCol == "" {
		mktCol = "MKTCAP"
	}
	filtered := make([]map[string]any, 0)
	codes := map[string]struct{}{}
	for _, r := range rows {
		mkt := parseMktCap(r[mktCol])
		if mkt >= minMktCap {
			filtered = append(filtered, r)
			for _, codeCol := range codeMatchCols {
				if _, ok := r[codeCol]; ok {
					c := normalizeCode(r[codeCol])
					if c != "" {
						codes[c] = struct{}{}
						break
					}
				}
			}
		}
	}
	return filtered, codes
}

func filterBasicByCodes(rows []map[string]any, dailyCodes map[string]struct{}) []map[string]any {
	if len(dailyCodes) == 0 {
		return []map[string]any{}
	}
	filtered := make([]map[string]any, 0)
	for _, r := range rows {
		for _, col := range []string{"ISU_SRT_CD", "ISU_CD"} {
			raw, ok := r[col]
			if !ok {
				continue
			}
			c := normalizeCode(raw)
			if c == "" {
				continue
			}
			if _, exists := dailyCodes[c]; exists {
				filtered = append(filtered, r)
				break
			}
			rawStr := fmt.Sprint(raw)
			if len(rawStr) > 6 {
				suffix := rawStr[len(rawStr)-6:]
				if isDigits(suffix) {
					if _, exists := dailyCodes[fmt.Sprintf("%06s", suffix)]; exists {
						filtered = append(filtered, r)
						break
					}
				}
			}
		}
	}
	return filtered
}

func safeFilename(name string) string {
	s := illegalFilenameRe.ReplaceAllString(strings.TrimSpace(name), "_")
	if s == "" {
		return "unknown"
	}
	return s
}

func normalizeHeaderCell(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "\ufeff")
}

func readCSV(path string) ([]string, []map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	for i := range header {
		header[i] = normalizeHeaderCell(header[i])
	}

	rows := make([]map[string]string, 0)
	for {
		rec, rowErr := r.Read()
		if errors.Is(rowErr, io.EOF) {
			break
		}
		if rowErr != nil {
			continue
		}
		row := map[string]string{}
		for i, h := range header {
			if i < len(rec) {
				row[h] = rec[i]
			} else {
				row[h] = ""
			}
		}
		rows = append(rows, row)
	}
	return header, rows, nil
}

func uniqueSortedStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func rowAnyToStringMap(row map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range row {
		if v == nil {
			out[k] = ""
			continue
		}
		if f, ok := v.(float64); ok {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				out[k] = ""
			} else {
				out[k] = strconv.FormatFloat(f, 'f', -1, 64)
			}
			continue
		}
		out[k] = fmt.Sprint(v)
	}
	return out
}

func pickDateValue(row map[string]string) string {
	if v := row["BAS_DD"]; v != "" {
		return v
	}
	if v := row["basDd"]; v != "" {
		return v
	}
	return row["날짜"]
}

func appendToStockCSV(rows []map[string]any, outDir string) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	cols := keysOfMap(rows[0])
	nameCol := detectCol(cols, nameCols)
	if nameCol == "" {
		nameCol = detectCol(cols, codeCols)
	}
	if nameCol == "" {
		return 0, nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, err
	}

	byStock := map[string][]map[string]any{}
	for _, r := range rows {
		key := strings.TrimSpace(fmt.Sprint(r[nameCol]))
		if key == "" {
			continue
		}
		byStock[key] = append(byStock[key], r)
	}

	dateCol := detectCol(cols, dateCols)
	if dateCol == "" {
		dateCol = "basDd"
	}
	allFieldnames := make([]string, 0)
	for _, r := range rows {
		for k := range r {
			allFieldnames = append(allFieldnames, k)
		}
	}
	allFieldnames = uniqueSortedStrings(allFieldnames)

	totalCount := 0
	for key, newRows := range byStock {
		filepath := filepath.Join(outDir, safeFilename(key)+".csv")
		fieldnames := append([]string{}, allFieldnames...)
		existingRows := []map[string]string{}

		if _, err := os.Stat(filepath); err == nil {
			existingHeader, existing, readErr := readCSV(filepath)
			if readErr == nil {
				existingRows = existing
				fieldnames = uniqueSortedStrings(append(fieldnames, existingHeader...))
			}
		}

		seen := map[string]struct{}{}
		for _, r := range existingRows {
			d := r[dateCol]
			if d == "" {
				d = r["날짜"]
			}
			if d != "" {
				seen[d] = struct{}{}
			}
		}

		for _, raw := range newRows {
			row := rowAnyToStringMap(raw)
			d := row["BAS_DD"]
			if d == "" {
				d = row["basDd"]
			}
			if d == "" {
				d = row["날짜"]
			}
			if d != "" {
				if _, exists := seen[d]; exists {
					continue
				}
				seen[d] = struct{}{}
			}
			normalized := map[string]string{}
			for _, f := range fieldnames {
				normalized[f] = row[f]
			}
			existingRows = append(existingRows, normalized)
		}

		sort.Slice(existingRows, func(i, j int) bool {
			return pickDateValue(existingRows[i]) < pickDateValue(existingRows[j])
		})

		if err := writeCSV(filepath, fieldnames, existingRows); err != nil {
			return totalCount, err
		}
		totalCount += len(newRows)
	}

	return totalCount, nil
}

func writeCSV(path string, fieldnames []string, rows []map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(fieldnames); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(fieldnames))
		for i, k := range fieldnames {
			rec[i] = row[k]
		}
		if err := w.Write(rec); err != nil {
			continue
		}
	}
	w.Flush()
	return w.Error()
}

func writeRowsToDateCSV(path string, rows []map[string]any) (int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		f, err := os.Create(path)
		if err != nil {
			return 0, err
		}
		f.Close()
		return 0, nil
	}

	fieldnames := []string{}
	for _, r := range rows {
		for k := range r {
			fieldnames = append(fieldnames, k)
		}
	}
	fieldnames = uniqueSortedStrings(fieldnames)

	outRows := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		outRows = append(outRows, rowAnyToStringMap(r))
	}
	if err := writeCSV(path, fieldnames, outRows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *Service) CollectAndSave(date string, apiIDs []string) (map[string]any, error) {
	if override := strings.TrimSpace(os.Getenv("KRX_DATE_OVERRIDE")); len(override) == 8 {
		if _, err := normalizeDate(override); err == nil {
			date = override
		}
	}
	if strings.TrimSpace(date) == "" {
		date = time.Now().AddDate(0, 0, -1).Format("20060102")
	} else {
		norm, err := normalizeDate(date)
		if err != nil {
			return nil, fmt.Errorf("날짜 형식 오류 (YYYYMMDD): %s", date)
		}
		date = norm
	}
	if strings.TrimSpace(s.cfg.KRXAPIKey) == "" {
		return nil, errors.New("KRX_OPENAPI_KEY(또는 KRX_API_KEY)가 설정되지 않았습니다.")
	}

	validatedAPIIDs, err := s.validateAndOrderAPIIDs(apiIDs)
	if err != nil {
		return nil, err
	}
	apiIDs = validatedAPIIDs

	results := map[string]any{}
	kospiCodes := map[string]struct{}{}
	kosdaqCodes := map[string]struct{}{}

	for _, apiID := range apiIDs {
		rows, err := s.fetchDataRows(apiID, date)
		if err != nil {
			results[apiID] = map[string]any{"error": err.Error()}
			continue
		}

		apiDir := s.getAPIDir(apiID)
		switch apiID {
		case "kospi_daily":
			filtered, codes := filterDailyByMktCap(rows, oneTrillion)
			kospiCodes = codes
			cnt, e := appendToStockCSV(filtered, apiDir)
			if e != nil {
				results[apiID] = map[string]any{"error": e.Error()}
			} else {
				results[apiID] = map[string]any{"path": apiDir, "count": cnt}
			}
		case "kosdaq_daily":
			filtered, codes := filterDailyByMktCap(rows, oneTrillion)
			kosdaqCodes = codes
			cnt, e := appendToStockCSV(filtered, apiDir)
			if e != nil {
				results[apiID] = map[string]any{"error": e.Error()}
			} else {
				results[apiID] = map[string]any{"path": apiDir, "count": cnt}
			}
		case "kospi_basic":
			filtered := filterBasicByCodes(rows, kospiCodes)
			filepath := filepath.Join(apiDir, date+".csv")
			cnt, e := writeRowsToDateCSV(filepath, filtered)
			if e != nil {
				results[apiID] = map[string]any{"error": e.Error()}
			} else {
				results[apiID] = map[string]any{"path": filepath, "count": cnt}
			}
		case "kosdaq_basic":
			filtered := filterBasicByCodes(rows, kosdaqCodes)
			filepath := filepath.Join(apiDir, date+".csv")
			cnt, e := writeRowsToDateCSV(filepath, filtered)
			if e != nil {
				results[apiID] = map[string]any{"error": e.Error()}
			} else {
				results[apiID] = map[string]any{"path": filepath, "count": cnt}
			}
		default:
			filepath := filepath.Join(apiDir, date+".csv")
			cnt, e := writeRowsToDateCSV(filepath, rows)
			if e != nil {
				results[apiID] = map[string]any{"error": e.Error()}
			} else {
				results[apiID] = map[string]any{"path": filepath, "count": cnt}
			}
		}
	}

	return map[string]any{
		"date":    date,
		"results": results,
	}, nil
}

func orderedForCollection(apiIDs []string) []string {
	in := map[string]struct{}{}
	for _, id := range apiIDs {
		in[id] = struct{}{}
	}
	out := make([]string, 0, len(apiIDs))
	seen := map[string]struct{}{}
	for _, id := range preferredAPIOrder {
		if _, ok := in[id]; ok {
			out = append(out, id)
			seen[id] = struct{}{}
		}
	}
	for _, id := range apiIDs {
		if _, ok := seen[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

func normalizeAPIIDList(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := set[id]; exists {
			continue
		}
		set[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func expandAPIIDDependencies(apiIDs []string) []string {
	if len(apiIDs) == 0 {
		return nil
	}

	out := normalizeAPIIDList(apiIDs)
	set := map[string]struct{}{}
	for _, id := range out {
		set[id] = struct{}{}
	}
	addIfMissing := func(id string) {
		if _, exists := set[id]; exists {
			return
		}
		set[id] = struct{}{}
		out = append(out, id)
	}

	// basic 종목정보는 같은 날짜의 daily 데이터(종목코드)가 있어야 필터링 가능하다.
	if _, ok := set["kospi_basic"]; ok {
		addIfMissing("kospi_daily")
	}
	if _, ok := set["kosdaq_basic"]; ok {
		addIfMissing("kosdaq_daily")
	}
	return out
}

func (s *Service) validateAndOrderAPIIDs(requested []string) ([]string, error) {
	cleaned := normalizeAPIIDList(requested)
	if len(cleaned) == 0 {
		return orderedAPIIDs(s.cfg.APIEndpoints), nil
	}
	expanded := expandAPIIDDependencies(cleaned)
	for _, id := range expanded {
		if _, ok := s.cfg.APIEndpoints[id]; !ok {
			return nil, fmt.Errorf("알 수 없는 API: %s", id)
		}
	}
	return orderedForCollection(expanded), nil
}

func sanitizeProgressPart(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]int{}
	for _, v := range a {
		set[v]++
	}
	for _, v := range b {
		set[v]--
	}
	for _, cnt := range set {
		if cnt != 0 {
			return false
		}
	}
	return true
}

func (s *Service) progressFileFor(apiIDs []string) string {
	all := orderedAPIIDs(s.cfg.APIEndpoints)
	normalized := normalizeAPIIDList(apiIDs)
	if len(normalized) == 0 || sameStringSet(normalized, all) {
		return s.progressFile
	}
	sort.Strings(normalized)
	parts := make([]string, 0, len(normalized))
	for _, id := range normalized {
		parts = append(parts, sanitizeProgressPart(id))
	}
	name := "krx_collect_progress__" + strings.Join(parts, "__") + ".json"
	return filepath.Join(s.dataDir, name)
}

func (s *Service) ListFiles(apiID string, limit, offset int) (map[string]any, error) {
	apiID = strings.TrimSpace(apiID)
	switch {
	case limit <= 0:
		limit = 200
	case limit > 2000:
		limit = 2000
	}
	if offset < 0 {
		offset = 0
	}

	type fileItem struct {
		APIID      string `json:"api_id"`
		APIName    string `json:"api_name"`
		FileName   string `json:"file_name"`
		Relative   string `json:"relative_path"`
		SizeBytes  int64  `json:"size_bytes"`
		ModifiedAt string `json:"modified_at"`
	}
	type apiSummary struct {
		APIID     string `json:"api_id"`
		APIName   string `json:"api_name"`
		FileCount int    `json:"file_count"`
	}

	selected := []string{}
	if apiID == "" || strings.EqualFold(apiID, "all") {
		selected = orderedAPIIDs(s.cfg.APIEndpoints)
		apiID = "all"
	} else {
		if _, ok := s.cfg.APIEndpoints[apiID]; !ok {
			return nil, fmt.Errorf("알 수 없는 API: %s", apiID)
		}
		selected = []string{apiID}
	}

	items := make([]fileItem, 0)
	summaries := make([]apiSummary, 0, len(selected))
	for _, id := range selected {
		ep := s.cfg.APIEndpoints[id]
		apiDir := s.getAPIDir(id)
		entries, err := os.ReadDir(apiDir)
		if err != nil {
			if os.IsNotExist(err) {
				summaries = append(summaries, apiSummary{
					APIID:     id,
					APIName:   ep.Name,
					FileCount: 0,
				})
				continue
			}
			return nil, err
		}

		fileCount := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			fileCount++
			items = append(items, fileItem{
				APIID:      id,
				APIName:    ep.Name,
				FileName:   entry.Name(),
				Relative:   filepath.ToSlash(filepath.Join(id, entry.Name())),
				SizeBytes:  info.Size(),
				ModifiedAt: info.ModTime().Format(time.RFC3339),
			})
		}

		summaries = append(summaries, apiSummary{
			APIID:     id,
			APIName:   ep.Name,
			FileCount: fileCount,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339, items[i].ModifiedAt)
		right, rightErr := time.Parse(time.RFC3339, items[j].ModifiedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.After(right)
		}
		if items[i].APIID != items[j].APIID {
			return items[i].APIID < items[j].APIID
		}
		return items[i].FileName < items[j].FileName
	})

	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	paged := items[offset:end]
	return map[string]any{
		"api_id":        apiID,
		"limit":         limit,
		"offset":        offset,
		"total":         total,
		"items":         paged,
		"api_summaries": summaries,
	}, nil
}

func defaultProgress() progressState {
	return progressState{
		LastDate:       "",
		TotalDatesDone: 0,
		ByDate:         map[string][]string{},
	}
}

func (s *Service) loadProgress(progressFile string) progressState {
	b, err := os.ReadFile(progressFile)
	if err != nil {
		return defaultProgress()
	}
	p := defaultProgress()
	if err := json.Unmarshal(b, &p); err != nil {
		return defaultProgress()
	}
	if p.ByDate == nil {
		p.ByDate = map[string][]string{}
	}
	return p
}

func (s *Service) saveProgress(progressFile string, p progressState) {
	_ = os.MkdirAll(s.dataDir, 0o755)
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = os.WriteFile(progressFile, b, 0o644)
}

func setFromSlice(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

func sliceFromSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func getCodesFromDailyCSV(apiDir, dateStr string) map[string]struct{} {
	codes := map[string]struct{}{}
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		return codes
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
			continue
		}
		path := filepath.Join(apiDir, e.Name())
		header, rows, readErr := readCSV(path)
		if readErr != nil {
			continue
		}
		dc := detectCol(header, dateCols)
		cc := detectCol(header, codeMatchCols)
		if dc == "" || cc == "" {
			continue
		}
		for _, row := range rows {
			if row[dc] == dateStr {
				c := normalizeCode(row[cc])
				if c != "" {
					codes[c] = struct{}{}
				}
				break
			}
		}
	}
	return codes
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusTooManyRequests
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "제한") || strings.Contains(msg, "limit")
}

func isUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusUnauthorized {
			return true
		}
		body := strings.ToLower(strings.TrimSpace(httpErr.Body))
		return strings.Contains(body, "unauthorized api call")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unauthorized api call") || strings.Contains(msg, " 401 ")
}

func isTemporaryCollectError(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode >= 500 {
			return true
		}
		body := strings.ToLower(strings.TrimSpace(httpErr.Body))
		return strings.Contains(body, "unable to provide service") ||
			strings.Contains(body, "temporarily unstable")
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "invalid character '<'")
}

func (s *Service) CollectBatchResume(delaySec float64, maxDates int, reset bool, requestedAPIIDs []string) (map[string]any, error) {
	endDate := time.Now()
	startDate := time.Date(2010, 1, 3, 0, 0, 0, 0, time.Local)

	apiIDs, err := s.validateAndOrderAPIIDs(requestedAPIIDs)
	if err != nil {
		return nil, err
	}
	progressFile := s.progressFileFor(apiIDs)

	progress := defaultProgress()
	_ = os.MkdirAll(s.dataDir, 0o755)
	if reset {
		_ = os.Remove(progressFile)
	} else {
		progress = s.loadProgress(progressFile)
	}

	if strings.TrimSpace(progress.LastDate) != "" {
		if last, parseErr := parseYYYYMMDD(progress.LastDate); parseErr == nil {
			startDate = last.AddDate(0, 0, 1)
		}
	}

	if startDate.After(endDate) {
		return map[string]any{
			"success":       true,
			"rate_limited":  false,
			"dates_done":    0,
			"total_dates":   progress.TotalDatesDone,
			"message":       "이미 최신 데이터까지 수집 완료.",
			"api_ids":       apiIDs,
			"progress_file": progressFile,
		}, nil
	}

	unauthorizedAPIs := map[string]struct{}{}
	datesDone := 0
	current := startDate

	for !current.After(endDate) {
		if maxDates > 0 && datesDone >= maxDates {
			break
		}
		if current.Weekday() == time.Saturday || current.Weekday() == time.Sunday {
			current = current.AddDate(0, 0, 1)
			continue
		}

		dateStr := current.Format("20060102")
		doneSet := setFromSlice(progress.ByDate[dateStr])
		kospiCodes := map[string]struct{}{}
		kosdaqCodes := map[string]struct{}{}

		for _, apiID := range apiIDs {
			if _, unauthorized := unauthorizedAPIs[apiID]; unauthorized {
				doneSet[apiID] = struct{}{}
				progress.ByDate[dateStr] = sliceFromSet(doneSet)
				continue
			}
			if _, done := doneSet[apiID]; done {
				if apiID == "kospi_daily" {
					kospiCodes = getCodesFromDailyCSV(s.getAPIDir(apiID), dateStr)
				} else if apiID == "kosdaq_daily" {
					kosdaqCodes = getCodesFromDailyCSV(s.getAPIDir(apiID), dateStr)
				}
				continue
			}

			rows, fetchErr := s.fetchDataRows(apiID, dateStr)
			if fetchErr != nil {
				if isUnauthorizedError(fetchErr) {
					unauthorizedAPIs[apiID] = struct{}{}
					doneSet[apiID] = struct{}{}
					progress.ByDate[dateStr] = sliceFromSet(doneSet)
					s.saveProgress(progressFile, progress)
					continue
				}
				if isRateLimitError(fetchErr) {
					progress.ByDate[dateStr] = sliceFromSet(doneSet)
					if datesDone > 0 {
						prev := current.AddDate(0, 0, -1).Format("20060102")
						if len(doneSet) < len(apiIDs) {
							progress.LastDate = prev
						}
						progress.TotalDatesDone += datesDone
					}
					s.saveProgress(progressFile, progress)
					return map[string]any{
						"success":       true,
						"rate_limited":  true,
						"dates_done":    datesDone,
						"total_dates":   progress.TotalDatesDone,
						"message":       fmt.Sprintf("호출 제한 도달. 이번에 %d일 수집. 다음 실행 시 이어서 진행.", datesDone),
						"api_ids":       apiIDs,
						"progress_file": progressFile,
					}, nil
				}
				if isTemporaryCollectError(fetchErr) {
					progress.ByDate[dateStr] = sliceFromSet(doneSet)
					if datesDone > 0 {
						prev := current.AddDate(0, 0, -1).Format("20060102")
						if len(doneSet) < len(apiIDs) {
							progress.LastDate = prev
						}
						progress.TotalDatesDone += datesDone
					}
					s.saveProgress(progressFile, progress)
					return map[string]any{
						"success":        true,
						"rate_limited":   true,
						"temporary_stop": true,
						"dates_done":     datesDone,
						"total_dates":    progress.TotalDatesDone,
						"message":        fmt.Sprintf("KRX 응답 지연/일시 장애로 중단. 이번에 %d일 수집. 다음 실행 시 이어서 진행.", datesDone),
						"api_ids":        apiIDs,
						"progress_file":  progressFile,
					}, nil
				}
				return nil, fetchErr
			}

			apiDir := s.getAPIDir(apiID)
			switch apiID {
			case "kospi_daily":
				filtered, codes := filterDailyByMktCap(rows, oneTrillion)
				kospiCodes = codes
				if _, writeErr := appendToStockCSV(filtered, apiDir); writeErr != nil {
					return nil, writeErr
				}
			case "kosdaq_daily":
				filtered, codes := filterDailyByMktCap(rows, oneTrillion)
				kosdaqCodes = codes
				if _, writeErr := appendToStockCSV(filtered, apiDir); writeErr != nil {
					return nil, writeErr
				}
			case "kospi_basic":
				filtered := filterBasicByCodes(rows, kospiCodes)
				if len(filtered) > 0 {
					if _, writeErr := writeRowsToDateCSV(filepath.Join(apiDir, dateStr+".csv"), filtered); writeErr != nil {
						return nil, writeErr
					}
				}
			case "kosdaq_basic":
				filtered := filterBasicByCodes(rows, kosdaqCodes)
				if len(filtered) > 0 {
					if _, writeErr := writeRowsToDateCSV(filepath.Join(apiDir, dateStr+".csv"), filtered); writeErr != nil {
						return nil, writeErr
					}
				}
			default:
				if _, writeErr := writeRowsToDateCSV(filepath.Join(apiDir, dateStr+".csv"), rows); writeErr != nil {
					return nil, writeErr
				}
			}

			doneSet[apiID] = struct{}{}
			progress.ByDate[dateStr] = sliceFromSet(doneSet)
			s.saveProgress(progressFile, progress)

			if delaySec > 0 {
				time.Sleep(time.Duration(delaySec * float64(time.Second)))
			}
		}

		if len(doneSet) == len(apiIDs) {
			progress.LastDate = dateStr
			progress.TotalDatesDone += 1
			delete(progress.ByDate, dateStr)
			datesDone += 1
		}
		progress.LastUpdated = time.Now().Format(time.RFC3339)
		s.saveProgress(progressFile, progress)
		current = current.AddDate(0, 0, 1)
	}

	skipped := make([]string, 0, len(unauthorizedAPIs))
	for apiID := range unauthorizedAPIs {
		skipped = append(skipped, apiID)
	}
	sort.Strings(skipped)
	msg := fmt.Sprintf("수집 완료. 이번에 %d일 추가.", datesDone)
	if len(skipped) > 0 {
		msg += fmt.Sprintf(" (권한 없음 API 건너뜀: %s)", strings.Join(skipped, ", "))
	}

	return map[string]any{
		"success":       true,
		"rate_limited":  false,
		"dates_done":    datesDone,
		"total_dates":   progress.TotalDatesDone,
		"message":       msg,
		"skipped_apis":  skipped,
		"api_ids":       apiIDs,
		"progress_file": progressFile,
	}, nil
}
