package news

type NewsItem struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	Description  string `json:"description"`
	PubDate      string `json:"pubDate"`
	PublishedAt  string `json:"publishedAt,omitempty"`
	RawPubDate   string `json:"rawPubDate,omitempty"`
	QualityTier  string `json:"qualityTier,omitempty"`
	QualityScore int    `json:"qualityScore,omitempty"`
	QualityFlags string `json:"qualityFlags,omitempty"`
	Keyword      string `json:"keyword,omitempty"`
	Press        string `json:"press,omitempty"`
}

type FetchResult struct {
	Items       []NewsItem `json:"items"`
	Total       int        `json:"total"`
	Error       string     `json:"error,omitempty"`
	RateLimited bool       `json:"rate_limited,omitempty"`
}

type CrawlProgress struct {
	CompletedKeywords []string `json:"completed_keywords"`
	TotalSaved        int      `json:"total_saved"`
	LastUpdated       string   `json:"last_updated,omitempty"`
}

type CrawlRunResult struct {
	TotalSaved   int     `json:"total_saved"`
	AddedThisRun int     `json:"added_this_run"`
	RateLimited  bool    `json:"rate_limited"`
	ElapsedSec   float64 `json:"elapsed_sec,omitempty"`
	Message      string  `json:"message,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type SavedFileInfo struct {
	Filename string `json:"filename"`
	SavedAt  string `json:"savedAt"`
	Count    int    `json:"count"`
}
