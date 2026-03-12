package news

import "testing"

func TestBuildStockNewsSignalsPositive(t *testing.T) {
	window := TradingNewsWindow{
		TargetTradingDate: "20260312",
		TimePrecision:     "datetime",
		Items: []WindowNewsItem{
			{
				NewsItem: NewsItem{
					Title:        "삼성전자, 대규모 공급계약 체결 및 실적 개선 기대",
					Description:  "가이던스 상향과 투자의견 상향이 이어졌다",
					Keyword:      "삼성전자",
					QualityScore: 100,
				},
				WindowBucket: "pre_market",
			},
			{
				NewsItem: NewsItem{
					Title:        "삼성전자 자사주 소각 검토, 주주환원 강화",
					Description:  "배당 확대 기대도 반영됐다",
					Keyword:      "삼성전자",
					QualityScore: 95,
				},
				WindowBucket: "post_close",
			},
		},
	}

	signals := buildStockNewsSignals(window)
	signal, ok := signals.Lookup("삼성전자")
	if !ok {
		t.Fatalf("expected stock signal for 삼성전자")
	}
	if signal.Score <= 56 {
		t.Fatalf("expected positive score, got %.4f", signal.Score)
	}
	if signal.Bias != "positive" {
		t.Fatalf("expected positive bias, got %s", signal.Bias)
	}
	if signal.PositiveCount == 0 {
		t.Fatalf("expected positive article count")
	}
}

func TestBuildStockNewsSignalsNegative(t *testing.T) {
	window := TradingNewsWindow{
		TargetTradingDate: "20260312",
		TimePrecision:     "datetime",
		Items: []WindowNewsItem{
			{
				NewsItem: NewsItem{
					Title:        "카카오뱅크 유상증자 검토설과 목표가 하향",
					Description:  "실적 부진 우려가 이어진다",
					Keyword:      "카카오뱅크",
					QualityScore: 100,
				},
				WindowBucket: "pre_market",
			},
			{
				NewsItem: NewsItem{
					Title:        "카카오뱅크 압수수색 보도에 투자심리 위축",
					Description:  "소송 리스크와 계약 지연 우려도 부각됐다",
					Keyword:      "카카오뱅크",
					QualityScore: 100,
				},
				WindowBucket: "target_trading_date",
			},
		},
	}

	signals := buildStockNewsSignals(window)
	signal, ok := signals.Lookup("카카오뱅크")
	if !ok {
		t.Fatalf("expected stock signal for 카카오뱅크")
	}
	if signal.Score >= 44 {
		t.Fatalf("expected negative score, got %.4f", signal.Score)
	}
	if signal.Bias != "negative" {
		t.Fatalf("expected negative bias, got %s", signal.Bias)
	}
	if signal.NegativeCount == 0 {
		t.Fatalf("expected negative article count")
	}
}

func TestStockNewsSignalLookupNormalizesName(t *testing.T) {
	snapshot := StockNewsSignalsSnapshot{
		Signals: map[string]StockNewsSignal{
			normalizeStockSignalKey("CJ CGV"): {StockName: "CJ CGV", Score: 61},
		},
	}

	signal, ok := snapshot.Lookup("CJC GV")
	if !ok {
		t.Fatalf("expected lookup to normalize stock name")
	}
	if signal.Score != 61 {
		t.Fatalf("unexpected signal score %.2f", signal.Score)
	}
}
