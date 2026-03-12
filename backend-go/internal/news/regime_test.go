package news

import "testing"

func TestBuildMarketNewsRegimeRiskOn(t *testing.T) {
	window := TradingNewsWindow{
		TargetTradingDate: "20260311",
		WindowStart:       "2026-03-10T20:00:00+09:00",
		WindowEnd:         "2026-03-11T08:00:00+09:00",
		TimePrecision:     "mixed",
		Items: []WindowNewsItem{
			{
				NewsItem: NewsItem{
					Title:       "금리 인하 기대에 코스피 강세, 외국인 순매수 확대",
					Description: "물가 둔화와 함께 랠리 기대가 커졌다",
				},
				WindowBucket: "pre_market",
			},
			{
				NewsItem: NewsItem{
					Title:       "반도체 수주 증가와 실적 개선 기대",
					Description: "가이던스 상향과 성장 회복 기대",
				},
				WindowBucket: "post_close",
			},
		},
	}

	regime := buildMarketNewsRegime(window)
	if regime.RiskOnProb <= regime.RiskOffProb {
		t.Fatalf("expected risk-on dominance, got on=%.4f off=%.4f", regime.RiskOnProb, regime.RiskOffProb)
	}
	if regime.Bias != "risk_on" {
		t.Fatalf("expected risk_on bias, got %s", regime.Bias)
	}
	if regime.Confidence <= 0.30 {
		t.Fatalf("expected meaningful confidence, got %.4f", regime.Confidence)
	}
}

func TestBuildMarketNewsRegimeRiskOff(t *testing.T) {
	window := TradingNewsWindow{
		TargetTradingDate: "20260311",
		WindowStart:       "2026-03-10T20:00:00+09:00",
		WindowEnd:         "2026-03-11T08:00:00+09:00",
		TimePrecision:     "mixed",
		Items: []WindowNewsItem{
			{
				NewsItem: NewsItem{
					Title:       "중동 공습과 유가 급등, 달러 강세 우려",
					Description: "미사일 공격과 제재 확대로 risk-off 심리 확대",
				},
				WindowBucket: "pre_market",
			},
			{
				NewsItem: NewsItem{
					Title:       "실적 부진과 경기 둔화 우려로 증시 하락 전망",
					Description: "하향 조정과 손실 확대 가능성",
				},
				WindowBucket: "post_close",
			},
		},
	}

	regime := buildMarketNewsRegime(window)
	if regime.RiskOffProb <= regime.RiskOnProb {
		t.Fatalf("expected risk-off dominance, got on=%.4f off=%.4f", regime.RiskOnProb, regime.RiskOffProb)
	}
	if regime.Bias != "risk_off" {
		t.Fatalf("expected risk_off bias, got %s", regime.Bias)
	}
	if regime.SentimentScore >= 0 {
		t.Fatalf("expected negative sentiment score, got %.4f", regime.SentimentScore)
	}
}

func TestBuildMarketNewsRegimeNeutralWhenEmpty(t *testing.T) {
	regime := buildMarketNewsRegime(TradingNewsWindow{})
	if regime.Bias != "neutral" {
		t.Fatalf("expected neutral bias, got %s", regime.Bias)
	}
	if regime.RiskOnProb+regime.RiskOffProb+regime.NeutralProb < 0.99 {
		t.Fatalf("expected probabilities to sum near 1, got %.4f", regime.RiskOnProb+regime.RiskOffProb+regime.NeutralProb)
	}
}
