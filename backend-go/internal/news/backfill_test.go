package news

import "testing"

func TestResolveRecentTradingBackfillRange(t *testing.T) {
	loc := seoulLocation()
	minDate, coveredDates, target, err := resolveRecentTradingBackfillRange(
		[]string{"20260305", "20260306", "20260309", "20260310"},
		"20260311",
		3,
		loc,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if minDate != "20260306" {
		t.Fatalf("expected min date 20260306, got %s", minDate)
	}
	if target != "20260311" {
		t.Fatalf("expected target 20260311, got %s", target)
	}
	expectedCovered := []string{"20260309", "20260310", "20260311"}
	if len(coveredDates) != len(expectedCovered) {
		t.Fatalf("expected %d covered dates, got %d", len(expectedCovered), len(coveredDates))
	}
	for index, value := range expectedCovered {
		if coveredDates[index] != value {
			t.Fatalf("covered dates mismatch at %d: expected %s got %s", index, value, coveredDates[index])
		}
	}
}
