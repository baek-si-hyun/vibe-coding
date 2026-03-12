package news

import (
	"os"
	"path/filepath"
	"testing"

	"investment-news-go/internal/config"
)

func TestIsHighQualityNewsItemRejectsNoise(t *testing.T) {
	cases := []NewsItem{
		{Title: "", Link: "https://example.com/article/1", Description: "본문"},
		{Title: "file3", Link: "https://example.com/download.php?id=3", Description: "본문"},
		{Title: "FILE_000000000010365", Link: "https://example.com/cmm/fms/filedown.do?atchFileId=FILE_000000000010365", Description: "본문"},
		{Title: "20260212-newspaper.pdf", Link: "https://example.com/20260212-newspaper.pdf", Description: "본문"},
		{Title: "2", Link: "https://example.com/article/2", Description: "본문"},
	}
	for _, item := range cases {
		if isHighQualityNewsItem(item) {
			t.Fatalf("expected noisy item to be rejected: %#v", item)
		}
	}

	good := NewsItem{
		Title:       "삼성전자, AI 반도체 투자 확대 검토",
		Link:        "https://example.com/news/ai-semiconductor",
		Description: "삼성전자가 차세대 AI 반도체 투자 계획을 검토하고 있다는 보도다.",
	}
	if !isHighQualityNewsItem(good) {
		t.Fatalf("expected valid news item to pass quality filter")
	}

	medium := enrichNewsQuality(NewsItem{
		Title:       "download",
		Link:        "https://example.com/article/summary",
		Description: "짧은 설명",
	})
	if medium.QualityTier == newsQualityHigh {
		t.Fatalf("expected borderline item not to be high quality")
	}
}

func TestEnsureCSVSchemaAddsQualityMetadataWithoutPruning(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService(config.Config{DataDir: tempDir})

	content := "title,link,description,pubDate,publishedAt,rawPubDate,keyword,press\n" +
		"file3,https://example.com/download.php?id=3,본문,2026-03-11,2026-03-11T09:00:00+09:00,2026-03-11T09:00:00+09:00,삼성전자,\n" +
		"삼성전자 AI 반도체 투자 확대 검토,https://example.com/news/ai,삼성전자 투자 기사,2026-03-11,2026-03-11T08:55:00+09:00,2026-03-11T08:55:00+09:00,삼성전자,example\n"
	if err := os.WriteFile(filepath.Join(tempDir, mergedFilename), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to seed csv: %v", err)
	}

	if err := service.ensureCSVSchema(); err != nil {
		t.Fatalf("ensureCSVSchema returned error: %v", err)
	}

	rawItems := service.readCSVItemsRaw(filepath.Join(tempDir, mergedFilename))
	if len(rawItems) != 2 {
		t.Fatalf("expected both rows to be retained, got %d rows", len(rawItems))
	}
	if rawItems[0].QualityTier == "" || rawItems[1].QualityTier == "" {
		t.Fatalf("expected quality metadata to be filled for all rows")
	}
	if !meetsQualityTier(rawItems[1], newsQualityHigh) {
		t.Fatalf("expected normal article row to be high quality")
	}
	if meetsQualityTier(rawItems[0], newsQualityHigh) {
		t.Fatalf("expected file-like row not to be high quality")
	}
}
