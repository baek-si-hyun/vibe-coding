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
	"strings"
	"time"
)

var numberEntityRe = regexp.MustCompile(`&#\d+;`)

var csvFields = []string{"title", "link", "description", "pubDate", "keyword"}

func (s *Service) ensureOutputFile() error {
	if err := os.MkdirAll(s.cfg.DataDir, 0o755); err != nil {
		return err
	}
	path := s.mergedPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	return w.Write(csvFields)
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

func (s *Service) loadExistingLinks() map[string]struct{} {
	links := map[string]struct{}{}
	path := s.mergedPath()
	f, err := os.Open(path)
	if err != nil {
		return links
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return links
	}
	linkIdx := -1
	for i, h := range header {
		if strings.TrimSpace(h) == "link" {
			linkIdx = i
			break
		}
	}
	if linkIdx < 0 {
		return links
	}

	for {
		row, rowErr := r.Read()
		if errors.Is(rowErr, io.EOF) {
			break
		}
		if rowErr != nil {
			continue
		}
		if linkIdx >= len(row) {
			continue
		}
		link := strings.TrimSpace(row[linkIdx])
		if link != "" {
			links[link] = struct{}{}
		}
	}
	return links
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
	_, statErr := os.Stat(path)
	fileExists := statErr == nil

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if !fileExists {
		if err := w.Write(csvFields); err != nil {
			return 0, err
		}
	}
	count := 0
	for _, r := range rows {
		record := []string{
			cleanText(r.Title),
			strings.TrimSpace(r.Link),
			cleanText(r.Description),
			strings.TrimSpace(r.PubDate),
			strings.TrimSpace(r.Keyword),
		}
		if err := w.Write(record); err != nil {
			continue
		}
		count++
	}
	return count, nil
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
		index[h] = i
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
			Title:       get("title"),
			Link:        get("link"),
			Description: get("description"),
			PubDate:     get("pubDate"),
			Keyword:     get("keyword"),
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
	return s.readCSVItems(path)
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
	all := s.readCSVItems(path)
	filtered := make([]NewsItem, 0, len(all))
	for _, it := range all {
		if qLower != "" {
			title := strings.ToLower(it.Title)
			desc := strings.ToLower(it.Description)
			if !strings.Contains(title, qLower) && !strings.Contains(desc, qLower) {
				continue
			}
		}
		filtered = append(filtered, it)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].PubDate > filtered[j].PubDate
	})

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
