package news

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var numberEntityRe = regexp.MustCompile(`&#\d+;`)

var csvFields = []string{"title", "link", "description", "pubDate", "publishedAt", "rawPubDate", "qualityTier", "qualityScore", "qualityFlags", "keyword", "press"}

func (s *Service) ensureOutputFile() error {
	return s.ensureCSVSchema()
}

func (s *Service) loadKeywordsFromFile() []string {
	path := s.keywordsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return append([]string{}, defaultKeywords...)
	}
	var payload struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return append([]string{}, defaultKeywords...)
	}
	if len(payload.Keywords) == 0 {
		return append([]string{}, defaultKeywords...)
	}
	return payload.Keywords
}

func (s *Service) loadExistingNewsIndex() map[string]NewsItem {
	index := map[string]NewsItem{}
	for _, item := range s.readCSVItems(s.mergedPath()) {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}
		index[link] = item
	}
	return index
}

func cleanText(text string) string {
	if text == "" {
		return ""
	}
	s := html.UnescapeString(text)
	s = numberEntityRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func (s *Service) appendRows(rows []NewsItem) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(s.cfg.DataDir, 0o755); err != nil {
		return 0, err
	}
	path := s.mergedPath()
	if err := s.ensureCSVSchema(); err != nil {
		return 0, err
	}

	existing := s.readCSVItems(path)
	index := make(map[string]int, len(existing))
	for i, item := range existing {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}
		index[link] = i
	}

	changes := 0
	for _, raw := range rows {
		item := normalizeNewsItem(raw)
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}
		if existingIndex, ok := index[link]; ok {
			merged := mergeStoredNewsItems(existing[existingIndex], item)
			if !newsItemsEqual(existing[existingIndex], merged) {
				existing[existingIndex] = merged
				changes++
			}
			continue
		}
		existing = append(existing, item)
		index[link] = len(existing) - 1
		changes++
	}

	if changes == 0 {
		return 0, nil
	}
	if err := writeNewsCSV(path, existing); err != nil {
		return 0, err
	}
	return changes, nil
}

func (s *Service) loadProgress(source string) CrawlProgress {
	path := s.progressPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return CrawlProgress{CompletedKeywords: []string{}, TotalSaved: 0}
	}
	var all map[string]CrawlProgress
	if err := json.Unmarshal(b, &all); err != nil {
		return CrawlProgress{CompletedKeywords: []string{}, TotalSaved: 0}
	}
	p, ok := all[source]
	if !ok {
		return CrawlProgress{CompletedKeywords: []string{}, TotalSaved: 0}
	}
	if p.CompletedKeywords == nil {
		p.CompletedKeywords = []string{}
	}
	return p
}

func (s *Service) saveProgress(source string, completedKeywords []string, totalSaved int) {
	path := s.progressPath()
	_ = os.MkdirAll(s.cfg.DataDir, 0o755)

	all := map[string]CrawlProgress{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &all)
	}

	all[source] = CrawlProgress{
		CompletedKeywords: completedKeywords,
		TotalSaved:        totalSaved,
		LastUpdated:       time.Now().Format(time.RFC3339),
	}

	b, err := json.Marshal(all)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func (s *Service) readCSVItems(path string) []NewsItem {
	rawItems := s.readCSVItemsRaw(path)
	items := make([]NewsItem, 0, len(rawItems))
	for _, item := range rawItems {
		items = append(items, normalizeNewsItem(item))
	}
	return items
}

func (s *Service) readCSVItemsRaw(path string) []NewsItem {
	items := []NewsItem{}
	f, err := os.Open(path)
	if err != nil {
		return items
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return items
	}
	index := map[string]int{}
	for i, h := range header {
		index[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}

	for {
		row, rowErr := r.Read()
		if errors.Is(rowErr, io.EOF) {
			break
		}
		if rowErr != nil {
			continue
		}
		get := func(key string) string {
			i, ok := index[key]
			if !ok || i >= len(row) {
				return ""
			}
			return row[i]
		}
		items = append(items, NewsItem{
			Title:        get("title"),
			Link:         get("link"),
			Description:  get("description"),
			PubDate:      get("pubDate"),
			PublishedAt:  get("publishedAt"),
			RawPubDate:   get("rawPubDate"),
			QualityTier:  get("qualityTier"),
			QualityScore: parseQualityScore(get("qualityScore")),
			QualityFlags: get("qualityFlags"),
			Keyword:      get("keyword"),
			Press:        get("press"),
		})
	}
	return items
}

func (s *Service) listSavedFiles() []SavedFileInfo {
	dataDir := s.cfg.DataDir
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		return []SavedFileInfo{}
	}

	out := []SavedFileInfo{}
	mergedPath := s.mergedPath()
	if stat, statErr := os.Stat(mergedPath); statErr == nil && !stat.IsDir() {
		count := len(s.readCSVItems(mergedPath))
		out = append(out, SavedFileInfo{
			Filename: mergedFilename,
			SavedAt:  stat.ModTime().Format(time.RFC3339),
			Count:    count,
		})
	}

	matches, _ := filepath.Glob(filepath.Join(dataDir, "news_*.csv"))
	sort.Slice(matches, func(i, j int) bool {
		iStat, iErr := os.Stat(matches[i])
		jStat, jErr := os.Stat(matches[j])
		if iErr != nil || jErr != nil {
			return matches[i] > matches[j]
		}
		return iStat.ModTime().After(jStat.ModTime())
	})
	for _, path := range matches {
		if filepath.Base(path) == mergedFilename {
			continue
		}
		stat, statErr := os.Stat(path)
		if statErr != nil || stat.IsDir() {
			continue
		}
		count := len(s.readCSVItems(path))
		out = append(out, SavedFileInfo{
			Filename: filepath.Base(path),
			SavedAt:  stat.ModTime().Format(time.RFC3339),
			Count:    count,
		})
	}
	return out
}

func (s *Service) getNewsFilepath(filename string) string {
	dataDir := s.cfg.DataDir
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		return ""
	}
	if strings.TrimSpace(filename) != "" {
		path := filepath.Join(dataDir, filename)
		if stat, statErr := os.Stat(path); statErr == nil && !stat.IsDir() {
			return path
		}
		return ""
	}
	mergedPath := s.mergedPath()
	if stat, statErr := os.Stat(mergedPath); statErr == nil && !stat.IsDir() {
		return mergedPath
	}

	matches, _ := filepath.Glob(filepath.Join(dataDir, "news_*.csv"))
	sort.Slice(matches, func(i, j int) bool {
		iStat, iErr := os.Stat(matches[i])
		jStat, jErr := os.Stat(matches[j])
		if iErr != nil || jErr != nil {
			return matches[i] > matches[j]
		}
		return iStat.ModTime().After(jStat.ModTime())
	})
	for _, path := range matches {
		if stat, statErr := os.Stat(path); statErr == nil && !stat.IsDir() {
			return path
		}
	}
	return ""
}

func (s *Service) readSavedNews(filename string) []NewsItem {
	path := s.getNewsFilepath(filename)
	if path == "" {
		return []NewsItem{}
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return []NewsItem{}
	}
	items, err := s.loadSavedNewsCached(path, info.ModTime())
	if err != nil {
		return []NewsItem{}
	}
	return items
}

func (s *Service) readSavedNewsPaginated(page, limit int, q, filename string) map[string]any {
	path := s.getNewsFilepath(filename)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	if path == "" {
		return map[string]any{
			"items":   []NewsItem{},
			"total":   0,
			"page":    page,
			"limit":   limit,
			"hasMore": false,
		}
	}

	qLower := strings.ToLower(strings.TrimSpace(q))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return map[string]any{
			"items":   []NewsItem{},
			"total":   0,
			"page":    page,
			"limit":   limit,
			"hasMore": false,
		}
	}

	var filtered []NewsItem
	if qLower == "" {
		filtered, err = s.loadSavedNewsSortedCached(path, info.ModTime())
		if err != nil {
			filtered = []NewsItem{}
		}
	} else {
		all, cacheErr := s.loadSavedNewsCached(path, info.ModTime())
		if cacheErr != nil {
			all = []NewsItem{}
		}
		filtered = make([]NewsItem, 0, len(all))
		for _, it := range all {
			title := strings.ToLower(it.Title)
			desc := strings.ToLower(it.Description)
			if !strings.Contains(title, qLower) && !strings.Contains(desc, qLower) {
				continue
			}
			filtered = append(filtered, it)
		}
		filtered = sortNewsItemsByRecency(filtered)
	}

	start := (page - 1) * limit
	end := start + limit
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	items := filtered[start:end]

	return map[string]any{
		"items":   items,
		"total":   len(filtered),
		"page":    page,
		"limit":   limit,
		"hasMore": len(filtered) > end,
	}
}

func (s *Service) ensureCSVSchema() error {
	if err := os.MkdirAll(s.cfg.DataDir, 0o755); err != nil {
		return err
	}
	path := s.mergedPath()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return writeNewsCSV(path, nil)
		}
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	reader := csv.NewReader(f)
	header, readErr := reader.Read()
	_ = f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	rawItems := s.readCSVItemsRaw(path)
	if sameCSVHeader(header, csvFields) {
		return nil
	}
	return writeNewsCSV(path, rawItems)
}

func writeNewsCSV(path string, rows []NewsItem) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(csvFields); err != nil {
		return err
	}
	for _, raw := range rows {
		item := normalizeNewsItem(raw)
		record := []string{
			cleanText(item.Title),
			strings.TrimSpace(item.Link),
			cleanText(item.Description),
			strings.TrimSpace(item.PubDate),
			strings.TrimSpace(item.PublishedAt),
			strings.TrimSpace(item.RawPubDate),
			strings.TrimSpace(item.QualityTier),
			strconv.Itoa(item.QualityScore),
			strings.TrimSpace(item.QualityFlags),
			strings.TrimSpace(item.Keyword),
			strings.TrimSpace(item.Press),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.Error()
}

func sameCSVHeader(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(strings.TrimPrefix(left[i], "\ufeff")) != right[i] {
			return false
		}
	}
	return true
}

func normalizeNewsItem(item NewsItem) NewsItem {
	return enrichNewsQuality(item)
}

func mergeStoredNewsItems(existing NewsItem, incoming NewsItem) NewsItem {
	base := normalizeNewsItem(existing)
	candidate := normalizeNewsItem(incoming)

	if candidate.Title != "" {
		base.Title = candidate.Title
	}
	if candidate.Description != "" {
		base.Description = candidate.Description
	}
	if candidate.Keyword != "" {
		base.Keyword = candidate.Keyword
	}
	if candidate.Press != "" {
		base.Press = candidate.Press
	}
	if candidate.PublishedAt != "" && betterPublishedAt(base.PublishedAt, candidate.PublishedAt) {
		base.PublishedAt = candidate.PublishedAt
	}
	if candidate.RawPubDate != "" && (base.RawPubDate == "" || hasDatetimePrecision(candidate.RawPubDate) && !hasDatetimePrecision(base.RawPubDate)) {
		base.RawPubDate = candidate.RawPubDate
	}
	if candidate.PubDate != "" {
		base.PubDate = candidate.PubDate
	}
	return base
}

func betterPublishedAt(existing string, candidate string) bool {
	existing = strings.TrimSpace(existing)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	if existing == "" {
		return true
	}
	if hasDatetimePrecision(candidate) && !hasDatetimePrecision(existing) {
		return true
	}
	return false
}

func newsItemsEqual(left NewsItem, right NewsItem) bool {
	a := normalizeNewsItem(left)
	b := normalizeNewsItem(right)
	return a.Title == b.Title &&
		a.Link == b.Link &&
		a.Description == b.Description &&
		a.PubDate == b.PubDate &&
		a.PublishedAt == b.PublishedAt &&
		a.RawPubDate == b.RawPubDate &&
		a.Keyword == b.Keyword &&
		a.Press == b.Press
}

func hasDatetimePrecision(raw string) bool {
	value := strings.TrimSpace(raw)
	return strings.Contains(value, "T") || strings.Contains(value, ":")
}

func newsSortKey(item NewsItem) string {
	normalized := normalizeNewsItem(item)
	if normalized.PublishedAt != "" {
		return normalized.PublishedAt
	}
	return normalized.PubDate
}
