package dart

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"investment-news-go/internal/krx"
)

const (
	defaultFinancialFSDiv   = "CFS"
	financialKRXLookbackDay = 14
	financialWarningCap     = 200
)

var (
	financialMktCapCols = []string{"MKTCAP", "mkp", "시가총액"}
	financialNameCols   = []string{"ISU_NM", "ISU_ABBRV", "isuNm", "itmsNm", "isuKorNm", "종목명"}
	financialCodeCols   = []string{"ISU_SRT_CD", "srtnCd", "단축코드", "ISU_CD", "isuCd", "종목코드"}
	financialDateCols   = []string{"BAS_DD", "basDd", "날짜"}
)

type reportCandidate struct {
	Year      int
	ReprtCode string
}

func financialExportHeaders() []string {
	return []string{
		"mode",
		"report_type",
		"market_cap",
		"market_cap_date",
		"market",
		"company_name",
		"corp_code",
		"stock_code",
		"fs_div",
		"rcept_no",
		"reprt_code",
		"bsns_year",
		"sj_div",
		"sj_nm",
		"account_id",
		"account_nm",
		"account_detail",
		"thstrm_nm",
		"thstrm_amount",
		"thstrm_add_amount",
		"frmtrm_nm",
		"frmtrm_amount",
		"frmtrm_q_nm",
		"frmtrm_q_amount",
		"frmtrm_add_amount",
		"bfefrmtrm_nm",
		"bfefrmtrm_amount",
		"ord",
		"currency",
		"fetched_at",
	}
}

func normalizeFinancialFSDiv(raw string) string {
	fsDiv := strings.ToUpper(strings.TrimSpace(raw))
	if fsDiv == "OFS" || fsDiv == "CFS" {
		return fsDiv
	}
	return defaultFinancialFSDiv
}

func (s *Service) resolveFinancialExportPath(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = fmt.Sprintf("largecap_financials_%s.csv", time.Now().Format("20060102_150405"))
	}
	if !strings.HasSuffix(strings.ToLower(name), ".csv") {
		name += ".csv"
	}

	var path string
	if filepath.IsAbs(name) {
		path = filepath.Clean(name)
	} else {
		path = filepath.Join(s.dartDir, "exports", name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func buildAnnualReportCandidates(nowYear int) []reportCandidate {
	out := make([]reportCandidate, 0, 3)
	for y := nowYear; y >= nowYear-2; y-- {
		out = append(out, reportCandidate{Year: y, ReprtCode: "11011"})
	}
	return out
}

func buildQuarterReportCandidates(nowYear int) []reportCandidate {
	quarterCodes := []string{"11014", "11012", "11013"}
	out := make([]reportCandidate, 0, 9)
	for y := nowYear; y >= nowYear-2; y-- {
		for _, code := range quarterCodes {
			out = append(out, reportCandidate{Year: y, ReprtCode: code})
		}
	}
	return out
}

func stringifyReportCandidates(candidates []reportCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, fmt.Sprintf("%d:%s", c.Year, c.ReprtCode))
	}
	return out
}

func appendFinancialWarning(
	warnings *[]map[string]string,
	company Company,
	reportType string,
	candidate reportCandidate,
	msg string,
) {
	if warnings == nil || len(*warnings) >= financialWarningCap {
		return
	}
	entry := map[string]string{
		"market":       company.Market,
		"stock_code":   company.StockCode,
		"corp_code":    company.CorpCode,
		"company_name": company.Name,
		"report_type":  reportType,
		"message":      strings.TrimSpace(msg),
	}
	if candidate.Year > 0 {
		entry["candidate_year"] = strconv.Itoa(candidate.Year)
	}
	if strings.TrimSpace(candidate.ReprtCode) != "" {
		entry["candidate_reprt_code"] = strings.TrimSpace(candidate.ReprtCode)
	}
	*warnings = append(*warnings, entry)
}

func financialFieldValue(field string, company Company, reportType string, row map[string]string, fsDiv string) string {
	switch field {
	case "mode":
		return "largecap_financial_export"
	case "report_type":
		return reportType
	case "market_cap":
		return strconv.FormatInt(company.MarketCap, 10)
	case "market_cap_date":
		return company.LatestDay
	case "market":
		if strings.TrimSpace(row["market"]) != "" {
			return strings.TrimSpace(row["market"])
		}
		return company.Market
	case "company_name":
		if strings.TrimSpace(row["company_name"]) != "" {
			return strings.TrimSpace(row["company_name"])
		}
		return company.Name
	case "corp_code":
		if strings.TrimSpace(row["corp_code"]) != "" {
			return strings.TrimSpace(row["corp_code"])
		}
		return company.CorpCode
	case "stock_code":
		if strings.TrimSpace(row["stock_code"]) != "" {
			return strings.TrimSpace(row["stock_code"])
		}
		return company.StockCode
	case "fs_div":
		if strings.TrimSpace(row["fs_div"]) != "" {
			return strings.TrimSpace(row["fs_div"])
		}
		return fsDiv
	default:
		return row[field]
	}
}

func writeFinancialRowsCSV(
	w *csv.Writer,
	headers []string,
	company Company,
	reportType string,
	rows []map[string]string,
	fsDiv string,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	written := 0
	for _, row := range rows {
		record := make([]string, 0, len(headers))
		for _, h := range headers {
			record = append(record, financialFieldValue(h, company, reportType, row, fsDiv))
		}
		if err := w.Write(record); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

func (s *Service) findLatestFinancialRows(
	corpCode string,
	fsDiv string,
	candidates []reportCandidate,
	delaySec float64,
) ([]map[string]string, reportCandidate, error) {
	for _, candidate := range candidates {
		rows, err := s.fetchSingleAcntAllRows(corpCode, candidate.Year, candidate.ReprtCode, fsDiv)
		if delaySec > 0 {
			time.Sleep(time.Duration(delaySec * float64(time.Second)))
		}
		if err == nil {
			if len(rows) > 0 {
				return rows, candidate, nil
			}
			continue
		}
		var se *statusError
		if errors.As(err, &se) {
			if isNoDataStatus(se.Status) {
				continue
			}
			if isRateLimitStatus(se.Status) {
				return nil, candidate, err
			}
		}
		return nil, candidate, err
	}
	return nil, reportCandidate{}, nil
}

func extractRowsFromKRXResult(result map[string]any) []map[string]any {
	raw, ok := result["data"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func detectAnyCol(row map[string]any, candidates []string) string {
	if len(row) == 0 {
		return ""
	}
	for _, c := range candidates {
		if _, ok := row[c]; ok {
			return c
		}
	}
	lowerIndex := map[string]string{}
	for k := range row {
		lowerIndex[strings.ToLower(strings.TrimSpace(k))] = k
	}
	for _, c := range candidates {
		if key, ok := lowerIndex[strings.ToLower(strings.TrimSpace(c))]; ok {
			return key
		}
	}
	return ""
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func parseAnyInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	case string:
		return parseInt64(t)
	default:
		return parseInt64(anyToString(t))
	}
}

func normalizeStockCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	digits := make([]rune, 0, len(raw))
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, ch)
		}
	}
	if len(digits) >= 6 {
		return string(digits[len(digits)-6:])
	}
	return string(digits)
}

func normalizeMarketDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	digits := make([]rune, 0, len(raw))
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, ch)
		}
	}
	if len(digits) >= 8 {
		return string(digits[:8])
	}
	return string(digits)
}

func mergeCompaniesByKey(markets ...[]Company) []Company {
	outByKey := map[string]Company{}
	for _, list := range markets {
		for _, c := range list {
			key := c.Market + ":" + c.StockCode
			if prev, ok := outByKey[key]; !ok || betterSnapshot(c, prev) {
				outByKey[key] = c
			}
		}
	}
	out := make([]Company, 0, len(outByKey))
	for _, item := range outByKey {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MarketCap != out[j].MarketCap {
			return out[i].MarketCap > out[j].MarketCap
		}
		if out[i].Market != out[j].Market {
			return out[i].Market < out[j].Market
		}
		return out[i].StockCode < out[j].StockCode
	})
	return out
}

func extractLargeCapCompanies(rows []map[string]any, market string, minMarketCap int64) []Company {
	if len(rows) == 0 {
		return nil
	}

	codeCol := ""
	nameCol := ""
	capCol := ""
	dateCol := ""
	for _, row := range rows {
		if codeCol == "" {
			codeCol = detectAnyCol(row, financialCodeCols)
		}
		if nameCol == "" {
			nameCol = detectAnyCol(row, financialNameCols)
		}
		if capCol == "" {
			capCol = detectAnyCol(row, financialMktCapCols)
		}
		if dateCol == "" {
			dateCol = detectAnyCol(row, financialDateCols)
		}
		if codeCol != "" && capCol != "" {
			break
		}
	}
	if codeCol == "" || capCol == "" {
		return nil
	}

	byKey := map[string]Company{}
	for _, row := range rows {
		stockCode := normalizeStockCode(anyToString(row[codeCol]))
		if len(stockCode) != 6 {
			continue
		}
		marketCap := parseAnyInt64(row[capCol])
		if marketCap < minMarketCap {
			continue
		}
		item := Company{
			Market:    market,
			StockCode: stockCode,
			Name:      strings.TrimSpace(anyToString(row[nameCol])),
			LatestDay: normalizeMarketDate(anyToString(row[dateCol])),
			MarketCap: marketCap,
		}
		key := item.Market + ":" + item.StockCode
		if prev, ok := byKey[key]; !ok || betterSnapshot(item, prev) {
			byKey[key] = item
		}
	}

	out := make([]Company, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MarketCap != out[j].MarketCap {
			return out[i].MarketCap > out[j].MarketCap
		}
		return out[i].StockCode < out[j].StockCode
	})
	return out
}

func buildKRXDateCandidates(asOfDate string, lookbackDays int) ([]string, error) {
	asOfDate = strings.TrimSpace(asOfDate)
	if asOfDate != "" {
		if _, err := time.Parse("20060102", asOfDate); err != nil {
			return nil, fmt.Errorf("as_of_date 형식이 올바르지 않습니다. (YYYYMMDD): %s", asOfDate)
		}
		return []string{asOfDate}, nil
	}

	if lookbackDays <= 0 {
		lookbackDays = 1
	}
	out := make([]string, 0, lookbackDays)
	base := time.Now()
	for i := 1; i <= lookbackDays; i++ {
		out = append(out, base.AddDate(0, 0, -i).Format("20060102"))
	}
	return out, nil
}

func (s *Service) fetchLargeCapsForMarket(
	krxSvc *krx.Service,
	apiID string,
	market string,
	date string,
	minMarketCap int64,
) ([]Company, error) {
	result, err := krxSvc.FetchData(apiID, date)
	if err != nil {
		return nil, err
	}
	rows := extractRowsFromKRXResult(result)
	return extractLargeCapCompanies(rows, market, minMarketCap), nil
}

func (s *Service) loadLargeCapCompaniesFromKRX(minMarketCap int64, asOfDate string) ([]Company, string, error) {
	dates, err := buildKRXDateCandidates(asOfDate, financialKRXLookbackDay)
	if err != nil {
		return nil, "", err
	}

	krxSvc := krx.NewService(s.cfg)
	var lastErr error
	for _, date := range dates {
		kospiItems, kospiErr := s.fetchLargeCapsForMarket(krxSvc, "kospi_daily", "KOSPI", date, minMarketCap)
		if kospiErr != nil {
			lastErr = kospiErr
		}
		kosdaqItems, kosdaqErr := s.fetchLargeCapsForMarket(krxSvc, "kosdaq_daily", "KOSDAQ", date, minMarketCap)
		if kosdaqErr != nil {
			lastErr = kosdaqErr
		}

		merged := mergeCompaniesByKey(kospiItems, kosdaqItems)
		if len(merged) > 0 {
			return merged, date, nil
		}
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", errors.New("KRX에서 시총 기준 대상 기업을 찾지 못했습니다. 날짜 또는 API 키를 확인하세요.")
}

func (s *Service) ExportLargeCapFinancialsCSV(
	minMarketCap int64,
	fsDiv string,
	asOfDate string,
	maxCompanies int,
	delaySec float64,
	outputPath string,
) (map[string]any, error) {
	if minMarketCap <= 0 {
		minMarketCap = defaultMinMarketCap
	}
	if delaySec < 0 {
		delaySec = 0
	}
	fsDiv = normalizeFinancialFSDiv(fsDiv)

	targets, marketDate, err := s.loadLargeCapCompaniesFromKRX(minMarketCap, asOfDate)
	if err != nil {
		return nil, err
	}

	corpMap, err := s.loadCorpMap()
	if err != nil {
		return nil, err
	}

	missingCorpCode := 0
	mapped := make([]Company, 0, len(targets))
	for _, c := range targets {
		corp, ok := corpMap[c.StockCode]
		if !ok || strings.TrimSpace(corp.CorpCode) == "" {
			missingCorpCode++
			continue
		}
		c.CorpCode = strings.TrimSpace(corp.CorpCode)
		if strings.TrimSpace(c.Name) == "" {
			c.Name = strings.TrimSpace(corp.CorpName)
		}
		mapped = append(mapped, c)
	}
	if len(mapped) == 0 {
		return nil, errors.New("DART corp_code 매핑 가능한 기업이 없습니다.")
	}

	allTargetCount := len(mapped)
	if maxCompanies > 0 && maxCompanies < len(mapped) {
		mapped = mapped[:maxCompanies]
	}

	path, err := s.resolveFinancialExportPath(outputPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	headers := financialExportHeaders()
	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return nil, err
	}

	nowYear := time.Now().Year()
	annualCandidates := buildAnnualReportCandidates(nowYear)
	quarterCandidates := buildQuarterReportCandidates(nowYear)

	processedCompanies := 0
	annualMatchedCompanies := 0
	quarterMatchedCompanies := 0
	noDataCompanies := 0
	writtenRows := 0
	rateLimited := false
	warnings := make([]map[string]string, 0, 16)

	for _, company := range mapped {
		annualRows, annualPick, annualErr := s.findLatestFinancialRows(company.CorpCode, fsDiv, annualCandidates, delaySec)
		if annualErr != nil {
			var se *statusError
			if errors.As(annualErr, &se) && isRateLimitStatus(se.Status) {
				appendFinancialWarning(&warnings, company, "annual", annualPick, annualErr.Error())
				rateLimited = true
				break
			}
			appendFinancialWarning(&warnings, company, "annual", annualPick, annualErr.Error())
		}

		if len(annualRows) > 0 {
			n, writeErr := writeFinancialRowsCSV(w, headers, company, "annual", s.enrichSingleAcntAllRows(company, fsDiv, annualRows), fsDiv)
			if writeErr != nil {
				return nil, writeErr
			}
			writtenRows += n
			annualMatchedCompanies++
		}

		quarterRows, quarterPick, quarterErr := s.findLatestFinancialRows(company.CorpCode, fsDiv, quarterCandidates, delaySec)
		if quarterErr != nil {
			var se *statusError
			if errors.As(quarterErr, &se) && isRateLimitStatus(se.Status) {
				appendFinancialWarning(&warnings, company, "quarterly", quarterPick, quarterErr.Error())
				rateLimited = true
				break
			}
			appendFinancialWarning(&warnings, company, "quarterly", quarterPick, quarterErr.Error())
		}
		if len(quarterRows) > 0 {
			n, writeErr := writeFinancialRowsCSV(w, headers, company, "quarterly", s.enrichSingleAcntAllRows(company, fsDiv, quarterRows), fsDiv)
			if writeErr != nil {
				return nil, writeErr
			}
			writtenRows += n
			quarterMatchedCompanies++
		}
		if len(annualRows) == 0 && len(quarterRows) == 0 {
			noDataCompanies++
		}
		processedCompanies++
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return map[string]any{
		"success":                     true,
		"output_path":                 path,
		"market_date":                 marketDate,
		"requested_min_market_cap":    minMarketCap,
		"fs_div":                      fsDiv,
		"all_target_companies":        allTargetCount,
		"selected_target_companies":   len(mapped),
		"processed_companies":         processedCompanies,
		"missing_corp_code":           missingCorpCode,
		"annual_matched_companies":    annualMatchedCompanies,
		"quarterly_matched_companies": quarterMatchedCompanies,
		"no_data_companies":           noDataCompanies,
		"written_rows":                writtenRows,
		"rate_limited":                rateLimited,
		"annual_candidates":           stringifyReportCandidates(annualCandidates),
		"quarter_candidates":          stringifyReportCandidates(quarterCandidates),
		"warnings_count":              len(warnings),
		"warnings":                    warnings,
	}, nil
}
