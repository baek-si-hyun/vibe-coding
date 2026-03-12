package nxt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"investment-news-go/internal/config"
)

func TestFetchSnapshotAndSave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != delayedListPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		page := r.Form.Get("pageIndex")
		if page == "" {
			page = "1"
		}
		payload := map[string]any{
			"total":   2,
			"records": 3,
			"setTime": "2026-03-12 20:13",
		}
		switch page {
		case "1":
			payload["brdinfoTimeList"] = []map[string]any{
				{
					"aggDd":       "20260311",
					"mktNm":       "KOSPI",
					"isuCd":       "KR7005930003",
					"isuSrdCd":    "A005930",
					"isuAbwdNm":   "삼성전자",
					"creTime":     "200500",
					"basePrc":     187900,
					"curPrc":      189200,
					"contrastPrc": 1300,
					"upDownRate":  0.69,
					"oppr":        189800,
					"hgpr":        194800,
					"lwpr":        185100,
					"accTdQty":    23871819,
					"accTrval":    4567888000000,
				},
				{
					"aggDd":       "20260311",
					"mktNm":       "KOSDAQ",
					"isuCd":       "KR7035420009",
					"isuSrdCd":    "A035420",
					"isuAbwdNm":   "NAVER테스트",
					"creTime":     "200500",
					"basePrc":     100000,
					"curPrc":      101500,
					"contrastPrc": 1500,
					"upDownRate":  1.50,
					"oppr":        100100,
					"hgpr":        102000,
					"lwpr":        99800,
					"accTdQty":    120000,
					"accTrval":    12180000000,
				},
			}
		default:
			payload["brdinfoTimeList"] = []map[string]any{
				{
					"aggDd":       "20260311",
					"mktNm":       "KOSPI",
					"isuCd":       "KR7000660001",
					"isuSrdCd":    "A000660",
					"isuAbwdNm":   "SK하이닉스",
					"creTime":     "200500",
					"basePrc":     955000,
					"curPrc":      935000,
					"contrastPrc": -20000,
					"upDownRate":  -2.09,
					"oppr":        954000,
					"hgpr":        963000,
					"lwpr":        932000,
					"accTdQty":    1041571,
					"accTrval":    987968839000,
				},
			}
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	service := NewService(config.Config{DataRootDir: tempDir})
	service.baseURL = server.URL
	service.latestPath = filepath.Join(tempDir, "nxt", "nxt_snapshot_latest.json")
	service.dataDir = filepath.Join(tempDir, "nxt")
	service.snapshotDir = filepath.Join(tempDir, "nxt", "snapshots")

	snapshot, err := service.CollectAndSave("20260311")
	if err != nil {
		t.Fatalf("CollectAndSave returned error: %v", err)
	}
	if snapshot.TradingDate != "20260311" {
		t.Fatalf("unexpected trading date: %s", snapshot.TradingDate)
	}
	if snapshot.QuoteCount != 3 {
		t.Fatalf("expected 3 quotes, got %d", snapshot.QuoteCount)
	}
	if snapshot.MarketCounts["KOSPI"] != 2 || snapshot.MarketCounts["KOSDAQ"] != 1 {
		t.Fatalf("unexpected market counts: %+v", snapshot.MarketCounts)
	}

	if _, err := os.Stat(service.LatestSnapshotPath()); err != nil {
		t.Fatalf("expected latest snapshot file: %v", err)
	}
	if _, err := os.Stat(service.SnapshotPath("20260311")); err != nil {
		t.Fatalf("expected dated snapshot file: %v", err)
	}

	loaded, err := service.LoadSnapshot("20260311")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if loaded.QuoteCount != 3 {
		t.Fatalf("expected loaded quote count 3, got %d", loaded.QuoteCount)
	}
	if loaded.Items[0].Code == "" {
		t.Fatalf("expected normalized code in saved snapshot")
	}
}
