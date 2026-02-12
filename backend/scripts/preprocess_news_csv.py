import csv
import html
import re
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
CSV_PATH = BACKEND_DIR / "lstm" / "data" / "news" / "news_merged.csv"
NEW_FIELDS = ["title", "link", "description", "pubDate", "keyword"]


def _clean_text(text: str) -> str:
    if not text:
        return ""
    cleaned = html.unescape(text)
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
                "keyword": (row.get("keyword") or "").strip(),
            })

    with open(CSV_PATH, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=NEW_FIELDS)
        w.writeheader()
        w.writerows(rows)

    print(f"전처리 완료: {len(rows)}건, 칼럼: {NEW_FIELDS}")


if __name__ == "__main__":
    main()
