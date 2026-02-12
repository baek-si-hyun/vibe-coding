"""
news_merged.csv 전처리
- 언론사(press) 칼럼 제거
- 제목(title), 내용(description)에서 HTML 엔티티(&quot;, &amp; 등) 제거
"""
import csv
import html
import re
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
CSV_PATH = BACKEND_DIR / "lstm" / "data" / "news" / "news_merged.csv"
NEW_FIELDS = ["title", "link", "description", "pubDate"]


def _clean_text(text: str) -> str:
    """HTML 엔티티 디코딩 및 이상 문자 제거"""
    if not text:
        return ""
    # html.unescape: &quot; -> ", &amp; -> &, &lt; -> <, &gt; -> >, &#39; -> '
    cleaned = html.unescape(text)
    # 추가로 남을 수 있는 엔티티 패턴 제거 (&#숫자; 등)
    cleaned = re.sub(r"&#\d+;", "", cleaned)
    return cleaned.strip()


def main():
    if not CSV_PATH.exists():
        print(f"파일 없음: {CSV_PATH}")
        return

    rows = []
    with open(CSV_PATH, "r", encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        old_fields = reader.fieldnames or []
        for row in reader:
            title = _clean_text(row.get("title", ""))
            description = _clean_text(row.get("description", ""))
            rows.append({
                "title": title,
                "link": (row.get("link") or "").strip(),
                "description": description,
                "pubDate": (row.get("pubDate") or "").strip(),
            })

    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=NEW_FIELDS)
        w.writeheader()
        w.writerows(rows)

    print(f"전처리 완료: {len(rows)}건, 칼럼: {NEW_FIELDS}")


if __name__ == "__main__":
    main()
