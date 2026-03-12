package news

import (
	"os"
	"path/filepath"
	"testing"

	"investment-news-go/internal/config"
)

func TestAppendRowsUpgradesExistingNewsRecord(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService(config.Config{DataDir: tempDir})

	legacyCSV := "title,link,description,pubDate,keyword\n" +
		"테스트 기사,https://example.com/news/1,요약,2026-03-10,반도체\n"
	if err := os.WriteFile(filepath.Join(tempDir, mergedFilename), []byte(legacyCSV), 0o644); err != nil {
		t.Fatalf("failed to write legacy csv: %v", err)
	}

	written, err := service.appendRows([]NewsItem{
		{
			Title:       "테스트 기사",
			Link:        "https://example.com/news/1",
			Description: "업데이트된 요약",
			PubDate:     "2026-03-10",
			PublishedAt: "2026-03-10T16:20:00+09:00",
			RawPubDate:  "2026-03-10T16:20:00+09:00",
			Keyword:     "반도체",
			Press:       "example",
		},
	})
	if err != nil {
		t.Fatalf("appendRows returned error: %v", err)
	}
	if written != 1 {
		t.Fatalf("expected 1 changed row, got %d", written)
	}

	items := service.readCSVItems(filepath.Join(tempDir, mergedFilename))
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].PublishedAt != "2026-03-10T16:20:00+09:00" {
		t.Fatalf("expected precise publishedAt to be saved, got %s", items[0].PublishedAt)
	}
	if items[0].Press != "example" {
		t.Fatalf("expected press to be saved, got %s", items[0].Press)
	}
	if items[0].Description != "업데이트된 요약" {
		t.Fatalf("expected description upgrade, got %s", items[0].Description)
	}
}
