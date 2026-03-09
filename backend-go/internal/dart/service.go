package dart

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"investment-news-go/internal/config"
)

const (
	defaultMinMarketCap int64 = 1_000_000_000_000 // 1조
)

type Service struct {
	cfg           config.Config
	client        *http.Client
	dataRoot      string
	dartDir       string
	corpCacheFile string
}

type Company struct {
	Market    string `json:"market"`
	StockCode string `json:"stock_code"`
	Name      string `json:"name"`
	CorpCode  string `json:"corp_code"`
	LatestDay string `json:"latest_day"`
	MarketCap int64  `json:"market_cap"`
}

type statusError struct {
	Status  string
	Message string
}

func (e *statusError) Error() string {
	if e == nil {
		return "dart status error"
	}
	return fmt.Sprintf("DART 응답 오류(%s): %s", e.Status, strings.TrimSpace(e.Message))
}

func isNoDataStatus(code string) bool {
	return code == "013" || code == "014"
}

func isRateLimitStatus(code string) bool {
	return code == "020"
}

type corpCodeXML struct {
	List []corpCodeItem `xml:"list"`
}

type corpCodeItem struct {
	CorpCode   string `xml:"corp_code"`
	CorpName   string `xml:"corp_name"`
	StockCode  string `xml:"stock_code"`
	ModifyDate string `xml:"modify_date"`
}

func NewService(cfg config.Config) *Service {
	dartDir := filepath.Join(cfg.DataRootDir, "dart")
	return &Service{
		cfg: cfg,
		client: &http.Client{
			Timeout: 25 * time.Second,
			Transport: &http.Transport{
				ForceAttemptHTTP2: false,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS10,
					MaxVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_RSA_WITH_AES_128_CBC_SHA,
						tls.TLS_RSA_WITH_AES_256_CBC_SHA,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
						tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					},
					PreferServerCipherSuites: true,
				},
			},
		},
		dataRoot:      cfg.DataRootDir,
		dartDir:       dartDir,
		corpCacheFile: filepath.Join(dartDir, "corp_codes.csv"),
	}
}

func parseInt64(raw string) int64 {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err == nil {
		return v
	}
	f, ferr := strconv.ParseFloat(raw, 64)
	if ferr != nil {
		return 0
	}
	return int64(f)
}

func betterSnapshot(newOne, current Company) bool {
	if newOne.LatestDay > current.LatestDay {
		return true
	}
	if newOne.LatestDay < current.LatestDay {
		return false
	}
	if newOne.MarketCap > current.MarketCap {
		return true
	}
	if newOne.MarketCap < current.MarketCap {
		return false
	}
	return newOne.Name != "" && current.Name == ""
}

func rowAnyToStrings(row map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range row {
		if v == nil {
			out[k] = ""
			continue
		}
		switch t := v.(type) {
		case string:
			out[k] = t
		default:
			out[k] = fmt.Sprint(t)
		}
	}
	return out
}

func (s *Service) fetchDARTList(path string, q url.Values) ([]map[string]string, error) {
	key := strings.TrimSpace(s.cfg.DARTFSSAPIKey)
	if key == "" {
		return nil, errors.New("DART_FSS_API_KEY가 설정되지 않았습니다.")
	}
	base := strings.TrimRight(strings.TrimSpace(s.cfg.DARTAPIBaseURL), "/")
	if base == "" {
		base = "https://opendart.fss.or.kr/api"
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("crtfc_key", key)
	u := base + path + "?" + q.Encode()

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DART 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("DART HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Status  string           `json:"status"`
		Message string           `json:"message"`
		List    []map[string]any `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("DART 응답 파싱 실패: %w", err)
	}
	if strings.TrimSpace(payload.Status) != "000" {
		return nil, &statusError{Status: strings.TrimSpace(payload.Status), Message: payload.Message}
	}

	rows := make([]map[string]string, 0, len(payload.List))
	for _, item := range payload.List {
		rows = append(rows, rowAnyToStrings(item))
	}
	return rows, nil
}

func (s *Service) fetchSingleAcntAllRows(corpCode string, year int, reprtCode, fsDiv string) ([]map[string]string, error) {
	corpCode = strings.TrimSpace(corpCode)
	if corpCode == "" {
		return nil, errors.New("corp_code는 필수입니다.")
	}
	if year <= 0 {
		year = time.Now().Year() - 1
	}
	reprtCode = strings.TrimSpace(reprtCode)
	if reprtCode == "" {
		reprtCode = "11011"
	}
	fsDiv = strings.ToUpper(strings.TrimSpace(fsDiv))
	if fsDiv == "" {
		fsDiv = "CFS"
	}

	q := url.Values{}
	q.Set("corp_code", corpCode)
	q.Set("bsns_year", strconv.Itoa(year))
	q.Set("reprt_code", reprtCode)
	q.Set("fs_div", fsDiv)

	rows, err := s.fetchDARTList("/fnlttSinglAcntAll.json", q)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &statusError{Status: "013", Message: "조회된 데이타가 없습니다."}
	}
	return rows, nil
}

func (s *Service) enrichSingleAcntAllRows(c Company, fsDiv string, rows []map[string]string) []map[string]string {
	now := time.Now().Format(time.RFC3339)
	fsDiv = strings.ToUpper(strings.TrimSpace(fsDiv))
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		m := map[string]string{
			"mode":              "single_acnt_all",
			"market":            c.Market,
			"company_name":      c.Name,
			"corp_code":         c.CorpCode,
			"stock_code":        c.StockCode,
			"fs_div":            fsDiv,
			"rcept_no":          row["rcept_no"],
			"reprt_code":        row["reprt_code"],
			"bsns_year":         row["bsns_year"],
			"sj_div":            row["sj_div"],
			"sj_nm":             row["sj_nm"],
			"account_id":        row["account_id"],
			"account_nm":        row["account_nm"],
			"account_detail":    row["account_detail"],
			"thstrm_nm":         row["thstrm_nm"],
			"thstrm_amount":     row["thstrm_amount"],
			"thstrm_add_amount": row["thstrm_add_amount"],
			"frmtrm_nm":         row["frmtrm_nm"],
			"frmtrm_amount":     row["frmtrm_amount"],
			"frmtrm_q_nm":       row["frmtrm_q_nm"],
			"frmtrm_q_amount":   row["frmtrm_q_amount"],
			"frmtrm_add_amount": row["frmtrm_add_amount"],
			"bfefrmtrm_nm":      row["bfefrmtrm_nm"],
			"bfefrmtrm_amount":  row["bfefrmtrm_amount"],
			"ord":               row["ord"],
			"currency":          row["currency"],
			"fetched_at":        now,
		}
		out = append(out, m)
	}
	return out
}

func (s *Service) loadCorpMap() (map[string]corpCodeItem, error) {
	_ = os.MkdirAll(s.dartDir, 0o755)
	useCache := false
	if st, err := os.Stat(s.corpCacheFile); err == nil {
		if time.Since(st.ModTime()) < 24*time.Hour && st.Size() > 0 {
			useCache = true
		}
	}
	if useCache {
		m, err := readCorpCacheCSV(s.corpCacheFile)
		if err == nil && len(m) > 0 {
			return m, nil
		}
	}
	m, err := s.downloadCorpCodes()
	if err != nil {
		if fallback, ferr := readCorpCacheCSV(s.corpCacheFile); ferr == nil && len(fallback) > 0 {
			return fallback, nil
		}
		return nil, err
	}
	_ = writeCorpCacheCSV(s.corpCacheFile, m)
	return m, nil
}

func readCorpCacheCSV(path string) (map[string]corpCodeItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	req := []string{"corp_code", "corp_name", "stock_code", "modify_date"}
	for _, k := range req {
		if _, ok := idx[k]; !ok {
			return nil, fmt.Errorf("corp cache header missing: %s", k)
		}
	}

	out := map[string]corpCodeItem{}
	for {
		row, e := r.Read()
		if e != nil {
			break
		}
		stock := strings.TrimSpace(row[idx["stock_code"]])
		if stock == "" {
			continue
		}
		out[stock] = corpCodeItem{
			CorpCode:   strings.TrimSpace(row[idx["corp_code"]]),
			CorpName:   strings.TrimSpace(row[idx["corp_name"]]),
			StockCode:  stock,
			ModifyDate: strings.TrimSpace(row[idx["modify_date"]]),
		}
	}
	return out, nil
}

func writeCorpCacheCSV(path string, data map[string]corpCodeItem) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"corp_code", "corp_name", "stock_code", "modify_date"}); err != nil {
		return err
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := data[k]
		if err := w.Write([]string{v.CorpCode, v.CorpName, v.StockCode, v.ModifyDate}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func (s *Service) downloadCorpCodes() (map[string]corpCodeItem, error) {
	key := strings.TrimSpace(s.cfg.DARTFSSAPIKey)
	if key == "" {
		return nil, errors.New("DART_FSS_API_KEY가 설정되지 않았습니다.")
	}
	base := strings.TrimRight(strings.TrimSpace(s.cfg.DARTAPIBaseURL), "/")
	if base == "" {
		base = "https://opendart.fss.or.kr/api"
	}
	u := base + "/corpCode.xml?crtfc_key=" + url.QueryEscape(key)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("corpCode 다운로드 실패: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("corpCode HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("corpCode zip 파싱 실패: %w", err)
	}
	if len(zr.File) == 0 {
		return nil, errors.New("corpCode zip 파일이 비어있습니다.")
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	xmlBody, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var parsed corpCodeXML
	if err := xml.Unmarshal(xmlBody, &parsed); err != nil {
		return nil, fmt.Errorf("corpCode XML 파싱 실패: %w", err)
	}

	out := map[string]corpCodeItem{}
	for _, item := range parsed.List {
		stock := strings.TrimSpace(item.StockCode)
		if stock == "" {
			continue
		}
		item.CorpCode = strings.TrimSpace(item.CorpCode)
		item.CorpName = strings.TrimSpace(item.CorpName)
		item.StockCode = stock
		item.ModifyDate = strings.TrimSpace(item.ModifyDate)
		if existing, ok := out[stock]; ok {
			if item.ModifyDate <= existing.ModifyDate {
				continue
			}
		}
		out[stock] = item
	}
	if len(out) == 0 {
		return nil, errors.New("corpCode에서 상장사 stock_code 매핑을 찾을 수 없습니다.")
	}
	return out, nil
}
