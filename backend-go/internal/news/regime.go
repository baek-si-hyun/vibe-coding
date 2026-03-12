package news

import (
	"math"
	"sort"
	"strings"
)

type MarketNewsRegime struct {
	TargetTradingDate string               `json:"target_trading_date"`
	WindowStart       string               `json:"window_start"`
	WindowEnd         string               `json:"window_end"`
	TimePrecision     string               `json:"time_precision"`
	CoverageNote      string               `json:"coverage_note,omitempty"`
	ItemCount         int                  `json:"item_count"`
	RiskOnProb        float64              `json:"risk_on_prob"`
	RiskOffProb       float64              `json:"risk_off_prob"`
	NeutralProb       float64              `json:"neutral_prob"`
	Confidence        float64              `json:"confidence"`
	SentimentScore    float64              `json:"sentiment_score"`
	Bias              string               `json:"bias"`
	Drivers           []MarketRegimeDriver `json:"drivers"`
	Headlines         []RegimeHeadline     `json:"headlines"`
}

type MarketRegimeDriver struct {
	Label        string   `json:"label"`
	Direction    string   `json:"direction"`
	Score        float64  `json:"score"`
	Count        int      `json:"count"`
	MatchedTerms []string `json:"matched_terms,omitempty"`
}

type RegimeHeadline struct {
	Title        string   `json:"title"`
	Direction    string   `json:"direction"`
	Score        float64  `json:"score"`
	WindowBucket string   `json:"window_bucket"`
	MatchedTerms []string `json:"matched_terms,omitempty"`
}

type regimeKeywordGroup struct {
	Label     string
	Direction string
	Weight    float64
	Keywords  []string
}

type regimeDriverAccum struct {
	label     string
	direction string
	score     float64
	count     int
	terms     map[string]struct{}
}

var marketRegimeKeywordGroups = []regimeKeywordGroup{
	{
		Label:     "완화/부양",
		Direction: "risk_on",
		Weight:    1.45,
		Keywords: []string{
			"금리 인하", "인하 기대", "통화 완화", "부양책", "정책 지원", "유동성 공급", "유동성 확대", "stimulus", "easing", "rate cut",
		},
	},
	{
		Label:     "성장/실적",
		Direction: "risk_on",
		Weight:    1.25,
		Keywords: []string{
			"실적 개선", "흑자", "어닝 서프라이즈", "수주", "계약", "성장", "회복", "반등", "가이던스 상향", "매출 증가", "record high", "beat", "upgrade",
		},
	},
	{
		Label:     "시장 강세",
		Direction: "risk_on",
		Weight:    1.00,
		Keywords: []string{
			"상승", "강세", "돌파", "신고가", "매수세", "외국인 순매수", "랠리", "risk-on", "rally",
		},
	},
	{
		Label:     "물가/금리 안정",
		Direction: "risk_on",
		Weight:    1.20,
		Keywords: []string{
			"물가 둔화", "인플레이션 둔화", "유가 안정", "환율 안정", "국채금리 하락", "금리 하락", "달러 약세",
		},
	},
	{
		Label:     "지정학 충격",
		Direction: "risk_off",
		Weight:    1.55,
		Keywords: []string{
			"전쟁", "공습", "미사일", "봉쇄", "제재", "드론", "red sea", "middle east", "iran", "israel", "hormuz", "houthi", "attack", "strike", "missile", "war", "sanctions",
		},
	},
	{
		Label:     "물가/긴축",
		Direction: "risk_off",
		Weight:    1.35,
		Keywords: []string{
			"인플레이션", "물가 상승", "금리 인상", "긴축", "국채금리 상승", "달러 강세", "환율 급등", "유가 급등", "관세", "tariff",
		},
	},
	{
		Label:     "경기 둔화/악재",
		Direction: "risk_off",
		Weight:    1.20,
		Keywords: []string{
			"침체", "리세션", "경기 둔화", "실적 부진", "적자", "손실", "급락", "하락", "하향", "규제", "소송", "지연", "downgrade", "miss", "delay", "lawsuit",
		},
	},
}

func (s *Service) BuildMarketNewsRegime(targetTradingDate string) (MarketNewsRegime, error) {
	window, err := s.LoadTradingNewsWindow(targetTradingDate)
	if err != nil {
		return MarketNewsRegime{}, err
	}
	return buildMarketNewsRegime(window), nil
}

func buildMarketNewsRegime(window TradingNewsWindow) MarketNewsRegime {
	result := MarketNewsRegime{
		TargetTradingDate: window.TargetTradingDate,
		WindowStart:       window.WindowStart,
		WindowEnd:         window.WindowEnd,
		TimePrecision:     window.TimePrecision,
		CoverageNote:      window.CoverageNote,
		ItemCount:         window.ItemCount,
		Drivers:           make([]MarketRegimeDriver, 0, 8),
		Headlines:         make([]RegimeHeadline, 0, 8),
	}

	if len(window.Items) == 0 {
		result.RiskOnProb = 0.33
		result.RiskOffProb = 0.33
		result.NeutralProb = 0.34
		result.Confidence = 0.15
		result.SentimentScore = 0
		result.Bias = "neutral"
		return result
	}

	driverStats := map[string]*regimeDriverAccum{}
	headlines := make([]RegimeHeadline, 0, len(window.Items))

	riskOnWeight := 0.0
	riskOffWeight := 0.0

	for _, item := range window.Items {
		text := normalizeRegimeText(item.Title + " " + item.Description)
		if text == "" {
			continue
		}

		bucketWeight := regimeBucketWeight(item.WindowBucket)
		headlineOn := 0.0
		headlineOff := 0.0
		headlineTerms := make([]string, 0, 8)

		for _, group := range marketRegimeKeywordGroups {
			matched := matchRegimeKeywords(text, group.Keywords)
			if len(matched) == 0 {
				continue
			}

			score := float64(len(matched)) * group.Weight * bucketWeight
			accum := driverStats[group.Label]
			if accum == nil {
				accum = &regimeDriverAccum{
					label:     group.Label,
					direction: group.Direction,
					terms:     map[string]struct{}{},
				}
				driverStats[group.Label] = accum
			}
			accum.score += score
			accum.count++
			for _, term := range matched {
				accum.terms[term] = struct{}{}
				headlineTerms = append(headlineTerms, term)
			}

			if group.Direction == "risk_on" {
				headlineOn += score
				riskOnWeight += score
			} else {
				headlineOff += score
				riskOffWeight += score
			}
		}

		if headlineOn == 0 && headlineOff == 0 {
			continue
		}

		direction := "neutral"
		score := headlineOn - headlineOff
		if score > 0 {
			direction = "risk_on"
		} else if score < 0 {
			direction = "risk_off"
		}

		headlines = append(headlines, RegimeHeadline{
			Title:        item.Title,
			Direction:    direction,
			Score:        roundTo(score, 4),
			WindowBucket: item.WindowBucket,
			MatchedTerms: uniqueSortedTerms(headlineTerms),
		})
	}

	totalSignal := riskOnWeight + riskOffWeight
	net := 0.0
	if totalSignal > 0 {
		net = (riskOnWeight - riskOffWeight) / totalSignal
	}
	signalStrength := clamp01(totalSignal / math.Max(float64(window.ItemCount)*1.6, 6))
	neutral := clamp(0.50-0.28*signalStrength, 0.12, 0.55)
	activeBucket := 1 - neutral
	riskOn := clamp(0.5*activeBucket+0.5*activeBucket*net, 0.05, 0.90)
	riskOff := clamp(activeBucket-riskOn, 0.05, 0.90)

	precisionPenalty := 0.12
	if window.TimePrecision != "date_only" {
		precisionPenalty = 0.04
	}
	confidence := clamp(0.22+0.46*signalStrength+0.18*math.Abs(net)-precisionPenalty, 0.10, 0.92)

	driverList := make([]MarketRegimeDriver, 0, len(driverStats))
	for _, accum := range driverStats {
		driverList = append(driverList, MarketRegimeDriver{
			Label:        accum.label,
			Direction:    accum.direction,
			Score:        roundTo(accum.score, 4),
			Count:        accum.count,
			MatchedTerms: uniqueSortedMapTerms(accum.terms),
		})
	}
	sort.Slice(driverList, func(i, j int) bool {
		if driverList[i].Score == driverList[j].Score {
			return driverList[i].Label < driverList[j].Label
		}
		return driverList[i].Score > driverList[j].Score
	})
	if len(driverList) > 8 {
		driverList = driverList[:8]
	}

	sort.Slice(headlines, func(i, j int) bool {
		left := math.Abs(headlines[i].Score)
		right := math.Abs(headlines[j].Score)
		if left == right {
			return headlines[i].Title < headlines[j].Title
		}
		return left > right
	})
	if len(headlines) > 8 {
		headlines = headlines[:8]
	}

	totalProb := riskOn + riskOff + neutral
	if totalProb <= 0 {
		riskOn, riskOff, neutral = 0.33, 0.33, 0.34
	} else {
		riskOn /= totalProb
		riskOff /= totalProb
		neutral /= totalProb
	}

	bias := "neutral"
	if riskOn >= riskOff+0.10 && riskOn >= 0.45 {
		bias = "risk_on"
	} else if riskOff >= riskOn+0.10 && riskOff >= 0.45 {
		bias = "risk_off"
	}

	result.RiskOnProb = roundTo(riskOn, 4)
	result.RiskOffProb = roundTo(riskOff, 4)
	result.NeutralProb = roundTo(neutral, 4)
	result.Confidence = roundTo(confidence, 4)
	result.SentimentScore = roundTo(net, 4)
	result.Bias = bias
	result.Drivers = driverList
	result.Headlines = headlines
	return result
}

func normalizeRegimeText(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}

func matchRegimeKeywords(text string, keywords []string) []string {
	matched := make([]string, 0, len(keywords))
	seen := map[string]struct{}{}
	for _, keyword := range keywords {
		normalized := strings.ToLower(strings.TrimSpace(keyword))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		if strings.Contains(text, normalized) {
			seen[normalized] = struct{}{}
			matched = append(matched, normalized)
		}
	}
	sort.Strings(matched)
	return matched
}

func regimeBucketWeight(bucket string) float64 {
	switch strings.TrimSpace(bucket) {
	case "pre_market", "target_trading_date":
		return 1.15
	case "post_close", "previous_trading_date":
		return 1.00
	default:
		return 0.95
	}
}

func uniqueSortedTerms(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func uniqueSortedMapTerms(values map[string]struct{}) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, strings.TrimSpace(strings.ToLower(value)))
	}
	sort.Strings(out)
	return out
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func clamp01(v float64) float64 {
	return clamp(v, 0, 1)
}

func roundTo(v float64, digits int) float64 {
	pow := math.Pow10(digits)
	return math.Round(v*pow) / pow
}
