package news

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"investment-news-go/internal/config"
)

const (
	mergedFilename  = "news_merged.csv"
	keywordsFile    = "crawl_keywords.json"
	progressFile    = "crawl_list_progress.json"
	defaultMinDate  = "2010-01-01"
	defaultPageSize = 50
)

var defaultKeywords = []string{"주식", "코스피", "코스닥", "증시", "투자", "금융", "경제"}

type Service struct {
	cfg    config.Config
	client *http.Client
	mu     sync.RWMutex
	cache  storedNewsCache
}

type storedNewsCache struct {
	path    string
	modTime time.Time
	items   []NewsItem
	sorted  []NewsItem
}

func NewService(cfg config.Config) *Service {
	service := &Service{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	go service.prewarmSavedNewsCache()
	return service
}

func (s *Service) DataDir() string {
	return s.cfg.DataDir
}

func (s *Service) mergedPath() string {
	return filepath.Join(s.cfg.DataDir, mergedFilename)
}

func (s *Service) keywordsPath() string {
	return filepath.Join(s.cfg.DataDir, keywordsFile)
}

func (s *Service) progressPath() string {
	return filepath.Join(s.cfg.DataDir, progressFile)
}

func (s *Service) prewarmSavedNewsCache() {
	path := s.getNewsFilepath("")
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	_, _ = s.loadSavedNewsSortedCached(path, info.ModTime())
}
