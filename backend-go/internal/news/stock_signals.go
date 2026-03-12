package news

import (
	"math"
	"sort"
	"strings"
)

type StockNewsSignalsSnapshot struct {
	TargetTradingDate string                     `json:"target_trading_date"`
	WindowStart       string                     `json:"window_start"`
	WindowEnd         string                     `json:"window_end"`
	TimePrecision     string                     `json:"time_precision"`
	CoverageNote      string                     `json:"coverage_note,omitempty"`
	ItemCount         int                        `json:"item_count"`
	SignalCount       int                        `json:"signal_count"`
	Signals           map[string]StockNewsSignal `json:"signals,omitempty"`
}

type StockNewsSignal struct {
	StockKey      string              `json:"stock_key"`
	StockName     string              `json:"stock_name"`
	ArticleCount  int                 `json:"article_count"`
	PositiveCount int                 `json:"positive_count"`
	NegativeCount int                 `json:"negative_count"`
	NeutralCount  int                 `json:"neutral_count"`
	PositiveScore float64             `json:"positive_score"`
	NegativeScore float64             `json:"negative_score"`
	Sentiment     float64             `json:"sentiment"`
	Buzz          float64             `json:"buzz"`
	Score         float64             `json:"score"`
	Bias          string              `json:"bias"`
	MatchedTerms  []string            `json:"matched_terms,omitempty"`
	TopHeadlines  []StockNewsHeadline `json:"top_headlines,omitempty"`
}

type StockNewsHeadline struct {
	Title        string   `json:"title"`
	Direction    string   `json:"direction"`
	Score        float64  `json:"score"`
	WindowBucket string   `json:"window_bucket"`
	MatchedTerms []string `json:"matched_terms,omitempty"`
}

type stockNewsKeywordGroup struct {
	Label     string
	Direction string
	Weight    float64
	Keywords  []string
}

type stockSignalAccum struct {
	stockKey      string
	stockName     string
	articleCount  int
	positiveCount int
	negativeCount int
	neutralCount  int
	positiveScore float64
	negativeScore float64
	terms         map[string]struct{}
	headlines     []StockNewsHeadline
}

var stockNewsKeywordGroups = []stockNewsKeywordGroup{
	{
		Label:     "실적/수주 호재",
		Direction: "positive",
		Weight:    1.45,
		Keywords: []string{
			"수주", "공급계약", "계약 체결", "대규모 공급", "실적 개선", "흑자전환", "어닝 서프라이즈", "가이던스 상향",
			"목표가 상향", "투자의견 상향", "매수", "호실적", "record high", "beat", "upgrade",
		},
	},
	{
		Label:     "정책/주주환원 호재",
		Direction: "positive",
		Weight:    1.15,
		Keywords: []string{
			"자사주", "배당", "배당 확대", "소각", "주주환원", "분리과세", "환원 정책", "배당 매력",
		},
	},
	{
		Label:     "사업 진전",
		Direction: "positive",
		Weight:    1.10,
		Keywords: []string{
			"승인", "허가", "통과", "출시", "상용화", "증설", "협력", "파트너십", "mou", "합작", "신제품", "신사업", "임상 성공",
		},
	},
	{
		Label:     "희석/자금조달 악재",
		Direction: "negative",
		Weight:    1.70,
		Keywords: []string{
			"유상증자", "전환사채", "교환사채", "신주인수권부사채", "cb 발행", "bw 발행", "오버행", "지분 매각",
		},
	},
	{
		Label:     "실적 악화",
		Direction: "negative",
		Weight:    1.45,
		Keywords: []string{
			"적자", "영업손실", "순손실", "실적 부진", "실적 악화", "어닝 쇼크", "가이던스 하향", "목표가 하향", "투자의견 하향", "miss", "downgrade",
		},
	},
	{
		Label:     "사건/리스크",
		Direction: "negative",
		Weight:    1.35,
		Keywords: []string{
			"압수수색", "횡령", "배임", "소송", "리콜", "중단", "지연", "불발", "계약 해지", "상장폐지", "관리종목", "감사의견", "해킹", "제재",
		},
	},
}

func (s *Service) BuildStockNewsSignals(targetTradingDate string) (StockNewsSignalsSnapshot, error) {
	window, err := s.LoadTradingNewsWindow(targetTradingDate)
	if err != nil {
		return StockNewsSignalsSnapshot{}, err
	}
	return buildStockNewsSignals(window), nil
}

func (s StockNewsSignalsSnapshot) Lookup(stockName string) (StockNewsSignal, bool) {
	if len(s.Signals) == 0 {
		return StockNewsSignal{}, false
	}
	key := normalizeStockSignalKey(stockName)
	if key == "" {
		return StockNewsSignal{}, false
	}
	signal, ok := s.Signals[key]
	return signal, ok
}

func buildStockNewsSignals(window TradingNewsWindow) StockNewsSignalsSnapshot {
	result := StockNewsSignalsSnapshot{
		TargetTradingDate: window.TargetTradingDate,
		WindowStart:       window.WindowStart,
		WindowEnd:         window.WindowEnd,
		TimePrecision:     window.TimePrecision,
		CoverageNote:      window.CoverageNote,
		ItemCount:         window.ItemCount,
		Signals:           map[string]StockNewsSignal{},
	}
	if len(window.Items) == 0 {
		return result
	}

	accums := map[string]*stockSignalAccum{}

	for _, item := range window.Items {
		stockName := strings.TrimSpace(item.Keyword)
		stockKey := normalizeStockSignalKey(stockName)
		if stockKey == "" {
			continue
		}

		text := normalizeRegimeText(item.Title + " " + item.Description)
		if text == "" {
			continue
		}

		accum := accums[stockKey]
		if accum == nil {
			accum = &stockSignalAccum{
				stockKey:  stockKey,
				stockName: stockName,
				terms:     map[string]struct{}{},
				headlines: make([]StockNewsHeadline, 0, 6),
			}
			accums[stockKey] = accum
		}

		accum.articleCount++
		articleWeight := regimeBucketWeight(item.WindowBucket) * stockQualityWeight(item.NewsItem)
		positiveScore := 0.0
		negativeScore := 0.0
		matchedTerms := make([]string, 0, 8)

		for _, group := range stockNewsKeywordGroups {
			matched := matchRegimeKeywords(text, group.Keywords)
			if len(matched) == 0 {
				continue
			}
			score := float64(len(matched)) * group.Weight * articleWeight
			for _, term := range matched {
				accum.terms[term] = struct{}{}
				matchedTerms = append(matchedTerms, term)
			}
			if group.Direction == "positive" {
				positiveScore += score
				accum.positiveScore += score
			} else {
				negativeScore += score
				accum.negativeScore += score
			}
		}

		direction := "neutral"
		headlineScore := positiveScore - negativeScore
		switch {
		case headlineScore > 0:
			direction = "positive"
			accum.positiveCount++
		case headlineScore < 0:
			direction = "negative"
			accum.negativeCount++
		default:
			accum.neutralCount++
		}

		if positiveScore != 0 || negativeScore != 0 {
			accum.headlines = append(accum.headlines, StockNewsHeadline{
				Title:        item.Title,
				Direction:    direction,
				Score:        roundTo(headlineScore, 4),
				WindowBucket: item.WindowBucket,
				MatchedTerms: uniqueSortedTerms(matchedTerms),
			})
		}
	}

	for key, accum := range accums {
		if accum.articleCount == 0 {
			continue
		}
		directionalTotal := accum.positiveScore + accum.negativeScore
		directionalShare := 0.0
		if accum.articleCount > 0 {
			directionalShare = float64(accum.positiveCount+accum.negativeCount) / float64(accum.articleCount)
		}
		sentiment := 0.0
		if directionalTotal > 0 {
			sentiment = (accum.positiveScore - accum.negativeScore) / directionalTotal
		}
		buzz := clamp01(math.Log1p(float64(accum.articleCount)) / math.Log1p(7))
		score := clamp(50+38*sentiment*math.Max(directionalShare, 0.20)+8*(buzz-0.35)*directionalShare, 0, 100)

		bias := "neutral"
		if score >= 56 && sentiment >= 0.12 {
			bias = "positive"
		} else if score <= 44 && sentiment <= -0.12 {
			bias = "negative"
		}

		sort.Slice(accum.headlines, func(i, j int) bool {
			left := math.Abs(accum.headlines[i].Score)
			right := math.Abs(accum.headlines[j].Score)
			if left == right {
				return accum.headlines[i].Title < accum.headlines[j].Title
			}
			return left > right
		})
		if len(accum.headlines) > 3 {
			accum.headlines = accum.headlines[:3]
		}

		result.Signals[key] = StockNewsSignal{
			StockKey:      accum.stockKey,
			StockName:     accum.stockName,
			ArticleCount:  accum.articleCount,
			PositiveCount: accum.positiveCount,
			NegativeCount: accum.negativeCount,
			NeutralCount:  accum.neutralCount,
			PositiveScore: roundTo(accum.positiveScore, 4),
			NegativeScore: roundTo(accum.negativeScore, 4),
			Sentiment:     roundTo(sentiment, 4),
			Buzz:          roundTo(buzz, 4),
			Score:         roundTo(score, 4),
			Bias:          bias,
			MatchedTerms:  uniqueSortedMapTerms(accum.terms),
			TopHeadlines:  accum.headlines,
		}
	}

	result.SignalCount = len(result.Signals)
	return result
}

func normalizeStockSignalKey(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= '가' && r <= '힣':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func stockQualityWeight(item NewsItem) float64 {
	score := item.QualityScore
	if score <= 0 {
		score = 60
	}
	return 0.72 + 0.28*clamp01(float64(score)/100)
}
