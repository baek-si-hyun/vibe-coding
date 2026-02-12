#!/usr/bin/env python3
"""코스피/코스닥 시총 1조 이상 기업을 crawl_keywords.json에 추가"""
import json
from pathlib import Path

BACKEND = Path(__file__).resolve().parent.parent
DATA_DIR = BACKEND / "lstm" / "data"
KEYWORDS_FILE = DATA_DIR / "news" / "crawl_keywords.json"
KOSPI_DAILY = DATA_DIR / "kospi_daily"
KOSDAQ_DAILY = DATA_DIR / "kosdaq_daily"


def main():
    with open(KEYWORDS_FILE, "r", encoding="utf-8") as f:
        data = json.load(f)

    existing = set(data["keywords"])

    added = []
    for folder in [KOSPI_DAILY, KOSDAQ_DAILY]:
        if not folder.exists():
            continue
        for p in folder.glob("*.csv"):
            name = p.stem
            if name not in existing:
                existing.add(name)
                added.append(name)

    if not added:
        print("추가할 기업 없음 (이미 전부 포함됨)")
        return

    data["keywords"].extend(sorted(added))
    with open(KEYWORDS_FILE, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

    print(f"추가 완료: {len(added)}개 기업")


if __name__ == "__main__":
    main()
