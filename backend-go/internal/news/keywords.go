package news

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	newsAPIBatchSize     = 6
	newsAPIQueryMaxChars = 280
)

var shortASCIIKeywordAllowlist = map[string]struct{}{
	"ai": {},
	"ev": {},
	"k2": {},
	"k9": {},
}

type crawlTask struct {
	ID       string
	Query    string
	Keywords []string
}

func normalizeKeywords(keywords []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keywords))

	for _, keyword := range keywords {
		normalized := normalizeKeyword(keyword)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}

	return out
}

func normalizeKeyword(keyword string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
	if normalized == "" {
		return ""
	}

	lower := strings.ToLower(normalized)
	if utf8.RuneCountInString(normalized) < 2 {
		return ""
	}
	if isASCIIKeyword(lower) && len(lower) < 3 {
		if _, ok := shortASCIIKeywordAllowlist[lower]; !ok {
			return ""
		}
	}

	return normalized
}

func isASCIIKeyword(value string) bool {
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '&':
		default:
			return false
		}
	}
	return value != ""
}

func buildCrawlTasks(source string, keywords []string) []crawlTask {
	normalized := normalizeKeywords(keywords)
	if source != "newsapi" {
		tasks := make([]crawlTask, 0, len(normalized))
		for _, keyword := range normalized {
			tasks = append(tasks, crawlTask{
				ID:       keyword,
				Query:    keyword,
				Keywords: []string{keyword},
			})
		}
		return tasks
	}

	tasks := make([]crawlTask, 0, len(normalized)/newsAPIBatchSize+1)
	batch := make([]string, 0, newsAPIBatchSize)
	currentLen := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		query := buildNewsAPIQuery(batch)
		tasks = append(tasks, crawlTask{
			ID:       fmt.Sprintf("newsapi:%s", strings.Join(batch, " | ")),
			Query:    query,
			Keywords: append([]string{}, batch...),
		})
		batch = batch[:0]
		currentLen = 0
	}

	for _, keyword := range normalized {
		quoted := quoteNewsAPIKeyword(keyword)
		addedLen := len(quoted)
		if len(batch) > 0 {
			addedLen += len(" OR ")
		}

		if len(batch) >= newsAPIBatchSize || currentLen+addedLen > newsAPIQueryMaxChars {
			flush()
		}

		batch = append(batch, keyword)
		currentLen += addedLen
	}

	flush()
	return tasks
}

func buildNewsAPIQuery(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	parts := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		parts = append(parts, quoteNewsAPIKeyword(keyword))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, " OR ")
}

func quoteNewsAPIKeyword(keyword string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(keyword, `"`, ""))
	if clean == "" {
		return ""
	}
	if strings.ContainsAny(clean, " -/·(),&") || utf8.RuneCountInString(clean) > 1 {
		return fmt.Sprintf(`"%s"`, clean)
	}
	return clean
}

func assignMatchedKeywords(item NewsItem, keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	text := strings.ToLower(strings.Join([]string{item.Title, item.Description}, " "))
	compact := compactKeywordText(text)

	matched := make([]string, 0, 3)
	for _, keyword := range keywords {
		lower := strings.ToLower(strings.TrimSpace(keyword))
		if lower == "" {
			continue
		}
		if strings.Contains(text, lower) || strings.Contains(compact, compactKeywordText(lower)) {
			matched = append(matched, keyword)
		}
		if len(matched) >= 3 {
			break
		}
	}
	if len(matched) == 0 {
		return keywords[0]
	}
	return strings.Join(matched, ",")
}

func compactKeywordText(value string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"-", "",
		"/", "",
		"·", "",
		",", "",
		"(", "",
		")", "",
		"&", "",
	)
	return replacer.Replace(strings.ToLower(value))
}
