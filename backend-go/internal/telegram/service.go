package telegram

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"investment-news-go/internal/config"
)

type Item struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	PubDate     string `json:"pubDate"`
	Keyword     string `json:"keyword,omitempty"`
}

type Service struct {
	cfg config.Config
}

func NewService(cfg config.Config) *Service {
	return &Service{cfg: cfg}
}

func normalizeHeaderCell(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "\ufeff")
}

func (s *Service) getCSVFiles() []string {
	dir := s.cfg.TelegramDataDir
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return []string{}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	files := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Slice(files, func(i, j int) bool {
		iStat, iErr := os.Stat(files[i])
		jStat, jErr := os.Stat(files[j])
		if iErr != nil || jErr != nil {
			return files[i] > files[j]
		}
		return iStat.ModTime().After(jStat.ModTime())
	})
	return files
}

func (s *Service) ChatRooms() []string {
	files := s.getCSVFiles()
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), filepath.Ext(f)))
	}
	return out
}

func readItemsFromCSV(path string) []Item {
	f, err := os.Open(path)
	if err != nil {
		return []Item{}
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return []Item{}
	}
	for i := range header {
		header[i] = normalizeHeaderCell(header[i])
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[h] = i
	}
	get := func(row []string, key string) string {
		i, ok := idx[key]
		if !ok || i >= len(row) {
			return ""
		}
		return row[i]
	}

	items := make([]Item, 0)
	for {
		row, rowErr := r.Read()
		if errors.Is(rowErr, io.EOF) {
			break
		}
		if rowErr != nil {
			continue
		}
		items = append(items, Item{
			Title:       get(row, "title"),
			Link:        get(row, "link"),
			Description: get(row, "description"),
			PubDate:     get(row, "pubDate"),
			Keyword:     get(row, "keyword"),
		})
	}
	return items
}

func (s *Service) Items(page, limit int, q, chatFilter string) map[string]any {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	files := s.getCSVFiles()
	if len(files) == 0 {
		return map[string]any{
			"items":   []Item{},
			"total":   0,
			"page":    1,
			"limit":   50,
			"hasMore": false,
		}
	}

	qLower := strings.ToLower(strings.TrimSpace(q))
	chatFilter = strings.TrimSpace(chatFilter)

	allItems := make([]Item, 0)
	for _, f := range files {
		items := readItemsFromCSV(f)
		for _, it := range items {
			if chatFilter != "" && strings.TrimSpace(it.Keyword) != chatFilter {
				continue
			}
			if qLower != "" {
				title := strings.ToLower(it.Title)
				desc := strings.ToLower(it.Description)
				if !strings.Contains(title, qLower) && !strings.Contains(desc, qLower) {
					continue
				}
			}
			allItems = append(allItems, it)
		}
	}

	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].PubDate > allItems[j].PubDate
	})

	total := len(allItems)
	start := (page - 1) * limit
	end := start + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	items := allItems[start:end]

	return map[string]any{
		"items":   items,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"hasMore": total > end,
		"savedAt": time.Now().Format(time.RFC3339),
	}
}
