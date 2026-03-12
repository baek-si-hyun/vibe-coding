package news

import "testing"

func TestBuildSourceKeywordAssignmentsDoesNotOverlap(t *testing.T) {
	sources := []string{"daum", "naver"}
	keywords := []string{"삼성전자", "SK하이닉스", "현대차", "POSCO홀딩스", "한화에어로스페이스", "KB금융"}

	assignments := buildSourceKeywordAssignments(sources, keywords, "20260312", 0)
	if len(assignments["daum"]) == 0 || len(assignments["naver"]) == 0 {
		t.Fatalf("expected both sources to receive keywords: %+v", assignments)
	}

	seen := map[string]string{}
	for source, values := range assignments {
		for _, keyword := range values {
			if prev, exists := seen[keyword]; exists {
				t.Fatalf("keyword %q assigned to both %s and %s", keyword, prev, source)
			}
			seen[keyword] = source
		}
	}
}

func TestBuildSourceKeywordAssignmentsCapsDeterministically(t *testing.T) {
	sources := []string{"daum", "naver"}
	keywords := []string{
		"삼성전자", "SK하이닉스", "현대차", "POSCO홀딩스", "한화에어로스페이스", "KB금융",
		"HD현대중공업", "LG화학", "셀트리온", "기아", "NAVER", "카카오",
	}

	first := buildSourceKeywordAssignments(sources, keywords, "20260312", 2)
	second := buildSourceKeywordAssignments(sources, keywords, "20260312", 2)
	otherDay := buildSourceKeywordAssignments(sources, keywords, "20260313", 2)

	for _, source := range sources {
		if got := len(first[source]); got != 2 {
			t.Fatalf("expected cap 2 for %s, got %d", source, got)
		}
		if first[source][0] != second[source][0] || first[source][1] != second[source][1] {
			t.Fatalf("expected deterministic assignments for %s", source)
		}
	}

	sameAcrossDays := true
	for _, source := range sources {
		if first[source][0] != otherDay[source][0] || first[source][1] != otherDay[source][1] {
			sameAcrossDays = false
			break
		}
	}
	if sameAcrossDays {
		t.Fatalf("expected keyword window to rotate across days")
	}
}
