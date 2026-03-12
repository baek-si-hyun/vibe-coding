package news

import "testing"

func TestParseStoredNewsTimestampPrefersPublishedAt(t *testing.T) {
	loc := seoulLocation()
	parsed, precision, ok := parseStoredNewsTimestamp(NewsItem{
		PubDate:     "2026-03-11",
		PublishedAt: "2026-03-11T08:35:00+09:00",
	}, loc)
	if !ok {
		t.Fatal("expected timestamp to parse")
	}
	if precision != "datetime" {
		t.Fatalf("expected datetime precision, got %s", precision)
	}
	if parsed.Format("20060102 15:04") != "20260311 08:35" {
		t.Fatalf("unexpected parsed time: %s", parsed.Format("20060102 15:04"))
	}
}
