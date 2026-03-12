package news

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	newsQualityHigh   = "high"
	newsQualityMedium = "medium"
	newsQualityLow    = "low"
)

var (
	attachmentExtRe  = regexp.MustCompile(`(?i)\.(pdf|hwp|hwpx|docx?|xlsx?|pptx?|zip)(?:$|[?#])`)
	fileLikeTitleRe  = regexp.MustCompile(`(?i)^(?:file(?:[_-][a-z0-9_ -]+|\d*)|[\w.-]+\.(pdf|hwp|hwpx|docx?|xlsx?|pptx?|zip))$`)
	lowSignalTitleRe = regexp.MustCompile(`(?i)^(?:\d+|page\s*\d+|magazine|newsletter|newsroom|policy|정책|뉴스룸|매거진|download|filedown)$`)
)

var noisyLinkPatterns = []string{
	"download.php",
	"/download",
	"openpdf",
	"fileupload.do",
	"filedown.do",
	"file_util.do",
	"j_download.asp",
	"article-banner.php",
	"celebritylist.do",
	"/ebook/",
}

func normalizeQualityTier(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case newsQualityHigh:
		return newsQualityHigh
	case newsQualityMedium:
		return newsQualityMedium
	case newsQualityLow:
		return newsQualityLow
	default:
		return newsQualityLow
	}
}

func qualityTierRank(tier string) int {
	switch normalizeQualityTier(tier) {
	case newsQualityHigh:
		return 3
	case newsQualityMedium:
		return 2
	default:
		return 1
	}
}

func meetsQualityTier(item NewsItem, minTier string) bool {
	return qualityTierRank(item.QualityTier) >= qualityTierRank(minTier)
}

func qualityFlags(item NewsItem) []string {
	normalized := normalizeNewsItemLite(item)
	title := strings.TrimSpace(normalized.Title)
	link := strings.TrimSpace(strings.ToLower(normalized.Link))
	description := strings.TrimSpace(normalized.Description)

	flags := make([]string, 0, 8)
	add := func(flag string) {
		for _, existing := range flags {
			if existing == flag {
				return
			}
		}
		flags = append(flags, flag)
	}

	if link == "" {
		add("missing_link")
	}
	if title == "" {
		add("missing_title")
	}
	if attachmentExtRe.MatchString(link) {
		add("attachment_link")
	}
	if fileLikeTitleRe.MatchString(title) {
		add("filelike_title")
	}
	if lowSignalTitleRe.MatchString(title) {
		add("low_signal_title")
	}
	for _, pattern := range noisyLinkPatterns {
		if strings.Contains(link, pattern) {
			add("download_path")
			break
		}
	}
	if utf8.RuneCountInString(title) > 0 && utf8.RuneCountInString(title) < 4 {
		add("very_short_title")
	}
	if len(strings.Fields(title)) <= 1 && utf8.RuneCountInString(title) < 6 {
		add("thin_title")
	}
	if description == "" {
		add("missing_description")
	}

	sort.Strings(flags)
	return flags
}

func scoreNewsQuality(item NewsItem) (string, int, []string) {
	flags := qualityFlags(item)
	score := 100

	for _, flag := range flags {
		switch flag {
		case "missing_link":
			score -= 80
		case "missing_title":
			score -= 80
		case "attachment_link":
			score -= 55
		case "filelike_title":
			score -= 45
		case "download_path":
			score -= 40
		case "low_signal_title":
			score -= 35
		case "very_short_title":
			score -= 20
		case "thin_title":
			score -= 15
		case "missing_description":
			score -= 8
		}
	}

	if score < 0 {
		score = 0
	}

	tier := newsQualityHigh
	switch {
	case score < 45:
		tier = newsQualityLow
	case score < 75:
		tier = newsQualityMedium
	}
	return tier, score, flags
}

func enrichNewsQuality(item NewsItem) NewsItem {
	normalized := normalizeNewsItemLite(item)
	tier, score, flags := scoreNewsQuality(normalized)
	normalized.QualityTier = tier
	normalized.QualityScore = score
	normalized.QualityFlags = strings.Join(flags, ",")
	return normalized
}

func normalizeNewsItemLite(item NewsItem) NewsItem {
	item.Title = cleanText(item.Title)
	item.Link = strings.TrimSpace(item.Link)
	item.Description = cleanText(item.Description)
	item.PubDate = strings.TrimSpace(item.PubDate)
	item.PublishedAt = strings.TrimSpace(item.PublishedAt)
	item.RawPubDate = strings.TrimSpace(item.RawPubDate)
	item.Keyword = strings.TrimSpace(item.Keyword)
	item.Press = strings.TrimSpace(item.Press)
	if item.PubDate == "" {
		item.PubDate = extractDateOnly(item.PublishedAt)
	}
	if item.RawPubDate == "" {
		item.RawPubDate = item.PublishedAt
	}
	if item.QualityScore < 0 {
		item.QualityScore = 0
	}
	return item
}

func parseQualityScore(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func isHighQualityNewsItem(item NewsItem) bool {
	return normalizeQualityTier(enrichNewsQuality(item).QualityTier) == newsQualityHigh
}
