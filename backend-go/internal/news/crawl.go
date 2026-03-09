package news

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type keywordFetchResult struct {
	task        crawlTask
	items       []NewsItem
	rateLimited bool
	err         error
}

func (s *Service) fetchTask(task crawlTask, source string, maxResults, maxPages int) keywordFetchResult {
	result := s.FetchNewsAPI(source, task.Query, maxResults, defaultMinDate, maxPages)
	if result.Error != "" && len(result.Items) == 0 && !result.RateLimited {
		return keywordFetchResult{
			task: task,
			err:  errors.New(result.Error),
		}
	}
	return keywordFetchResult{
		task:        task,
		items:       result.Items,
		rateLimited: result.RateLimited,
	}
}

func (s *Service) runCrawlAPI(
	source string,
	keywords []string,
	workers int,
	reset bool,
	maxPages int,
	keywordsLimit int,
	checkpointEvery int,
) CrawlRunResult {
	if err := s.ensureOutputFile(); err != nil {
		return CrawlRunResult{Error: err.Error(), TotalSaved: 0, AddedThisRun: 0}
	}

	tasks := buildCrawlTasks(source, keywords)
	if keywordsLimit > 0 && keywordsLimit < len(tasks) {
		tasks = tasks[:keywordsLimit]
	}

	progress := s.loadProgress(source)
	if reset {
		progress = CrawlProgress{CompletedKeywords: []string{}, TotalSaved: 0}
		s.saveProgress(source, []string{}, 0)
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

	completedSet := map[string]struct{}{}
	for _, kw := range progress.CompletedKeywords {
		completedSet[kw] = struct{}{}
	}

	tasksToDo := make([]crawlTask, 0, len(tasks))
	for _, task := range tasks {
		if _, done := completedSet[task.ID]; !done {
			tasksToDo = append(tasksToDo, task)
		}
	}
	if len(tasksToDo) == 0 {
		return CrawlRunResult{
			TotalSaved:   progress.TotalSaved,
			AddedThisRun: 0,
			RateLimited:  false,
			Message:      "남은 키워드 없음. 완료.",
		}
	}

	existing := s.loadExistingLinks()
	seen := make(map[string]struct{}, len(existing))
	for link := range existing {
		seen[link] = struct{}{}
	}

	if workers < 1 {
		workers = 1
	}
	if workers > len(tasksToDo) {
		workers = len(tasksToDo)
	}
	if checkpointEvery < 1 {
		checkpointEvery = 100
	}

	t0 := time.Now()
	completedKeywords := append([]string{}, progress.CompletedKeywords...)
	totalSaved := progress.TotalSaved
	batch := make([]NewsItem, 0, checkpointEvery)
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
					res := s.fetchTask(task, source, 999999, maxPages)
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
		for _, task := range tasksToDo {
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

	for res := range results {
		if rateLimitedHit {
			continue
		}
		if res.err != nil {
			continue
		}

		if res.rateLimited {
			rateLimitedHit = true
			for _, it := range res.items {
				link := strings.TrimSpace(it.Link)
				if link == "" {
					continue
				}
				if _, exists := seen[link]; exists {
					continue
				}
				seen[link] = struct{}{}
				if strings.TrimSpace(it.Keyword) == "" {
					it.Keyword = assignMatchedKeywords(it, res.task.Keywords)
				}
				batch = append(batch, it)
			}
			if len(batch) > 0 {
				written, _ := s.appendRows(batch)
				totalSaved += written
				batch = batch[:0]
			}
			cancel()
			continue
		}

		completedKeywords = append(completedKeywords, res.task.ID)
		for _, it := range res.items {
			link := strings.TrimSpace(it.Link)
			if link == "" {
				continue
			}
			if _, exists := seen[link]; exists {
				continue
			}
			seen[link] = struct{}{}
			if strings.TrimSpace(it.Keyword) == "" {
				it.Keyword = assignMatchedKeywords(it, res.task.Keywords)
			}
			batch = append(batch, it)
			if len(batch) >= checkpointEvery {
				written, _ := s.appendRows(batch)
				totalSaved += written
				s.saveProgress(source, completedKeywords, totalSaved)
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		written, _ := s.appendRows(batch)
		totalSaved += written
	}
	s.saveProgress(source, completedKeywords, totalSaved)

	addedThisRun := totalSaved - progress.TotalSaved
	elapsed := time.Since(t0).Seconds()
	msg := fmt.Sprintf("이번 실행 %d건, 누적 %d건", addedThisRun, totalSaved)
	if rateLimitedHit {
		msg += " (호출 제한 도달, 저장됨. 다음 실행 시 이어서 진행)"
	}

	return CrawlRunResult{
		TotalSaved:   totalSaved,
		AddedThisRun: addedThisRun,
		RateLimited:  rateLimitedHit,
		ElapsedSec:   math.Round(elapsed*10) / 10,
		Message:      msg,
	}
}

func (s *Service) CrawlAPIResume(sources []string, reset bool) map[string]any {
	if len(sources) == 0 {
		sources = []string{"daum", "naver", "newsapi"}
	}

	keywords, err := s.loadKeywords()
	if err != nil {
		return map[string]any{
			"error": err.Error(),
		}
	}

	totalSaved := 0
	addedTotal := 0
	results := make([]map[string]any, 0)
	skipped := make([]map[string]any, 0)
	rateLimited := false
	errorsList := make([]string, 0)

	for _, source := range sources {
		if source != "naver" && source != "daum" && source != "newsapi" {
			continue
		}
		r := s.runCrawlAPI(source, keywords, 6, reset, 0, 0, 100)
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
			"total_saved":    r.TotalSaved,
			"added_this_run": r.AddedThisRun,
			"rate_limited":   r.RateLimited,
			"elapsed_sec":    r.ElapsedSec,
			"message":        r.Message,
		})
	}

	if len(results) == 0 && len(errorsList) > 0 {
		return map[string]any{
			"error":          strings.Join(errorsList, "; "),
			"total_saved":    0,
			"added_this_run": 0,
		}
	}

	items := s.readSavedNews("")
	sourceResults := make([]map[string]any, 0, len(results)+len(skipped))
	for _, r := range results {
		sourceResults = append(sourceResults, map[string]any{
			"source":       r["source"],
			"added":        r["added_this_run"],
			"total":        r["total_saved"],
			"rate_limited": r["rate_limited"],
		})
	}
	for _, sk := range skipped {
		sourceResults = append(sourceResults, map[string]any{
			"source":  sk["source"],
			"skipped": true,
			"reason":  sk["reason"],
		})
	}

	rateLimitedSources := make([]string, 0)
	continuedSources := make([]string, 0)
	resultSources := make([]string, 0, len(results))
	for _, r := range results {
		src, _ := r["source"].(string)
		resultSources = append(resultSources, src)
		rl, _ := r["rate_limited"].(bool)
		if rl {
			rateLimitedSources = append(rateLimitedSources, src)
		} else {
			continuedSources = append(continuedSources, src)
		}
	}

	msg := "완료"
	if len(results) > 0 {
		if last, ok := results[len(results)-1]["message"].(string); ok && last != "" {
			msg = last
		}
	}
	if len(rateLimitedSources) > 0 && len(continuedSources) > 0 {
		msg += fmt.Sprintf(" [호출 제한: %s → %s 계속 실행]",
			strings.Join(rateLimitedSources, ", "),
			strings.Join(continuedSources, ", "),
		)
	}

	return map[string]any{
		"success":        true,
		"total":          len(items),
		"added":          addedTotal,
		"keyword_count":  len(keywords),
		"keyword_scope":  "KOSPI/KOSDAQ 시총 1조 이상 종목명",
		"rate_limited":   rateLimited,
		"message":        msg,
		"sources":        resultSources,
		"source_results": sourceResults,
		"skipped":        skipped,
		"total_saved":    totalSaved,
	}
}
