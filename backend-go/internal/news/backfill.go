package news

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	defaultBackfillWorkers  = 2
	defaultBackfillMaxPages = 2
)

func (s *Service) BackfillRecentTradingDays(targetTradingDate string, tradingDays int, sources []string) map[string]any {
	if len(sources) == 0 {
		sources = []string{"daum", "naver"}
	}
	sources = normalizeSources(sources)
	if tradingDays <= 0 {
		tradingDays = 2
	}

	keywords, err := s.loadKeywords()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	minDate, coveredDates, resolvedTarget, rangeErr := s.resolveBackfillRange(targetTradingDate, tradingDays)
	if rangeErr != nil {
		return map[string]any{"error": rangeErr.Error()}
	}

	totalSaved := 0
	addedTotal := 0
	results := make([]map[string]any, 0)
	skipped := make([]map[string]any, 0)
	rateLimited := false
	errorsList := make([]string, 0)
	assignments := buildSourceKeywordAssignments(sources, keywords, resolvedTarget, s.cfg.NewsSourceKeywordCap)

	for _, source := range sources {
		assignedKeywords := assignments[source]
		if len(assignedKeywords) == 0 {
			skipped = append(skipped, map[string]any{
				"source": source,
				"reason": "할당된 키워드 없음",
			})
			continue
		}
		r := s.runBackfillAPI(source, assignedKeywords, defaultBackfillWorkers, defaultBackfillMaxPages, minDate)
		if r.Error != "" {
			errorsList = append(errorsList, fmt.Sprintf("%s: %s", source, r.Error))
			skipped = append(skipped, map[string]any{
				"source": source,
				"reason": r.Error,
			})
			continue
		}
		totalSaved = r.TotalSaved
		addedTotal += r.AddedThisRun
		rateLimited = rateLimited || r.RateLimited
		results = append(results, map[string]any{
			"source":         source,
			"keyword_count":  len(assignedKeywords),
			"total_saved":    r.TotalSaved,
			"added_this_run": r.AddedThisRun,
			"rate_limited":   r.RateLimited,
			"elapsed_sec":    r.ElapsedSec,
			"message":        r.Message,
		})
	}

	if len(results) == 0 && len(errorsList) > 0 {
		return map[string]any{
			"error":                 strings.Join(errorsList, "; "),
			"total_saved":           0,
			"added_this_run":        0,
			"target_trading_date":   resolvedTarget,
			"min_date":              minDate,
			"covered_trading_dates": coveredDates,
		}
	}

	items := s.readSavedNews("")
	sourceResults := make([]map[string]any, 0, len(results)+len(skipped))
	for _, r := range results {
		sourceResults = append(sourceResults, map[string]any{
			"source":        r["source"],
			"keyword_count": r["keyword_count"],
			"added":         r["added_this_run"],
			"total":         r["total_saved"],
			"rate_limited":  r["rate_limited"],
		})
	}
	for _, sk := range skipped {
		sourceResults = append(sourceResults, map[string]any{
			"source":  sk["source"],
			"skipped": true,
			"reason":  sk["reason"],
		})
	}

	msg := fmt.Sprintf(
		"최근 %d거래일 뉴스 백필 완료 (target=%s, min_date=%s, added=%d)",
		tradingDays,
		resolvedTarget,
		minDate,
		addedTotal,
	)
	if rateLimited {
		msg += " [일부 소스 호출 제한]"
	}

	return map[string]any{
		"success":               true,
		"mode":                  "backfill_recent_trading_days",
		"total":                 len(items),
		"added":                 addedTotal,
		"keyword_count":         len(keywords),
		"keyword_cap":           s.cfg.NewsSourceKeywordCap,
		"keyword_scope":         "KOSPI/KOSDAQ 시총 1조 이상 종목명",
		"rate_limited":          rateLimited,
		"message":               msg,
		"sources":               sources,
		"source_results":        sourceResults,
		"skipped":               skipped,
		"total_saved":           totalSaved,
		"target_trading_date":   resolvedTarget,
		"trading_days":          tradingDays,
		"min_date":              minDate,
		"covered_trading_dates": coveredDates,
	}
}

func (s *Service) runBackfillAPI(source string, keywords []string, workers int, maxPages int, minDate string) CrawlRunResult {
	if err := s.ensureOutputFile(); err != nil {
		return CrawlRunResult{Error: err.Error(), TotalSaved: 0, AddedThisRun: 0}
	}

	if source == "naver" && (s.cfg.NaverClientID == "" || s.cfg.NaverClientSecret == "") {
		return CrawlRunResult{Error: "NAVER_CLIENT_ID, NAVER_CLIENT_SECRET 필요", TotalSaved: 0, AddedThisRun: 0}
	}
	if source == "daum" && s.cfg.KakaoRestAPIKey == "" {
		return CrawlRunResult{Error: "KAKAO_REST_API_KEY 필요", TotalSaved: 0, AddedThisRun: 0}
	}
	if source == "newsapi" && s.cfg.NewsAPIKey == "" {
		return CrawlRunResult{Error: "NEWSAPI_KEY 필요", TotalSaved: 0, AddedThisRun: 0}
	}

	tasks := buildCrawlTasks(source, keywords)
	if len(tasks) == 0 {
		return CrawlRunResult{TotalSaved: len(s.readSavedNews("")), AddedThisRun: 0, Message: "백필 대상 키워드 없음"}
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if maxPages < 1 {
		maxPages = 1
	}

	t0 := time.Now()
	seen := s.loadExistingNewsIndex()
	batch := make([]NewsItem, 0, 100)
	changedTotal := 0
	rateLimitedHit := false

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan crawlTask)
	results := make(chan keywordFetchResult, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-jobs:
					if !ok {
						return
					}
					res := s.fetchTask(task, source, 999999, minDate, maxPages)
					select {
					case results <- res:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case jobs <- task:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		written, _ := s.appendRows(batch)
		changedTotal += written
		batch = batch[:0]
	}

	for res := range results {
		if res.err != nil {
			continue
		}
		if res.rateLimited {
			rateLimitedHit = true
		}
		for _, item := range res.items {
			normalized, keep := mergeFetchedNewsItem(seen, item, res.task.Keywords)
			if !keep {
				continue
			}
			batch = append(batch, normalized)
		}
		if len(batch) >= 100 {
			flushBatch()
		}
		if res.rateLimited {
			cancel()
		}
	}

	flushBatch()
	totalSaved := len(s.readSavedNews(""))
	msg := fmt.Sprintf("최근 뉴스 백필 %d건 변경", changedTotal)
	if rateLimitedHit {
		msg += " (일부 소스 호출 제한)"
	}

	return CrawlRunResult{
		TotalSaved:   totalSaved,
		AddedThisRun: changedTotal,
		RateLimited:  rateLimitedHit,
		ElapsedSec:   math.Round(time.Since(t0).Seconds()*10) / 10,
		Message:      msg,
	}
}

func mergeFetchedNewsItem(seen map[string]NewsItem, item NewsItem, taskKeywords []string) (NewsItem, bool) {
	normalized := normalizeNewsItem(item)
	link := strings.TrimSpace(normalized.Link)
	if link == "" {
		return NewsItem{}, false
	}
	if strings.TrimSpace(normalized.Keyword) == "" {
		normalized.Keyword = assignMatchedKeywords(normalized, taskKeywords)
	}

	if existing, ok := seen[link]; ok {
		merged := mergeStoredNewsItems(existing, normalized)
		if newsItemsEqual(existing, merged) {
			return NewsItem{}, false
		}
		seen[link] = merged
		return merged, true
	}
	seen[link] = normalized
	return normalized, true
}

func (s *Service) resolveBackfillRange(targetTradingDate string, tradingDays int) (string, []string, string, error) {
	tradingDates, err := s.loadTradingCalendarDates()
	if err != nil {
		return "", nil, "", err
	}
	target := normalizeTradingDate(targetTradingDate)
	if target == "" {
		window, resolveErr := s.resolveTradingNewsWindow("", seoulLocation())
		if resolveErr != nil {
			return "", nil, "", resolveErr
		}
		target = window.targetTradingDate
	}
	minDate, coveredDates, resolvedTarget, rangeErr := resolveRecentTradingBackfillRange(tradingDates, target, tradingDays, seoulLocation())
	if rangeErr != nil {
		return "", nil, "", rangeErr
	}
	return formatTradingDateForAPI(minDate), coveredDates, resolvedTarget, nil
}

func resolveRecentTradingBackfillRange(tradingDates []string, targetTradingDate string, tradingDays int, loc *time.Location) (string, []string, string, error) {
	if len(tradingDates) == 0 {
		return "", nil, "", fmt.Errorf("거래일 캘린더 데이터가 비어 있습니다")
	}
	if tradingDays < 1 {
		tradingDays = 1
	}

	target := normalizeTradingDate(targetTradingDate)
	if target == "" {
		return "", nil, "", fmt.Errorf("기준 거래일이 비어 있습니다")
	}

	extended := append([]string{}, tradingDates...)
	latest := extended[len(extended)-1]
	for latest < target {
		next := nextWeekdayTradingDate(latest, loc)
		if next == "" || next <= latest {
			break
		}
		extended = append(extended, next)
		latest = next
	}

	targetIndex := -1
	for i, value := range extended {
		if value == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return "", nil, "", fmt.Errorf("기준 거래일 %s 을 거래일 캘린더에서 찾지 못했습니다", target)
	}

	startTargetIndex := targetIndex - tradingDays + 1
	if startTargetIndex < 0 {
		startTargetIndex = 0
	}
	minIndex := startTargetIndex
	if minIndex > 0 {
		minIndex--
	}

	coveredDates := append([]string{}, extended[startTargetIndex:targetIndex+1]...)
	return extended[minIndex], coveredDates, target, nil
}

func formatTradingDateForAPI(date string) string {
	normalized := normalizeTradingDate(date)
	if normalized == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s", normalized[0:4], normalized[4:6], normalized[6:8])
}
