#!/usr/bin/env python3
import csv
import json
import re
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
DATA_DIR = BACKEND_DIR / "lstm" / "data" / "news"
TELEGRAM_CHATS_DIR = BACKEND_DIR / "lstm" / "data" / "telegram_chats"
KEYWORDS_FILE = "crawl_keywords.json"
FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _load_keywords() -> list:
    p = DATA_DIR / KEYWORDS_FILE
    if not p.exists():
        return ["주식", "코스피", "코스닥", "증시", "투자", "금융", "경제"]
    try:
        with open(p, "r", encoding="utf-8") as f:
            data = json.load(f)
            kw = data.get("keywords", [])
            return kw if kw else ["주식", "코스피", "코스닥", "증시", "투자", "금융", "경제"]
    except Exception:
        return ["주식", "코스피", "코스닥", "증시", "투자", "금융", "경제"]


def _match_keyword(text: str, keywords: list) -> str:
    if not text:
        return ""
    text_lower = text.lower()
    sorted_kw = sorted(keywords, key=len, reverse=True)
    for kw in sorted_kw:
        if not kw or len(kw) < 2:
            continue
        if kw in text or kw.lower() in text_lower:
            return kw
    return ""


def _add_keywords_news(filepath: Path, keywords: list) -> tuple[int, int]:
    rows = []
    with open(filepath, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        fieldnames = reader.fieldnames or list(FIELDNAMES)
        if "keyword" not in fieldnames:
            fieldnames = list(fieldnames) + ["keyword"]
        for row in reader:
            rows.append(row)

    updated = 0
    for row in rows:
        existing = (row.get("keyword") or "").strip()
        if existing:
            continue
        text = f"{row.get('title', '')} {row.get('description', '')}"
        kw = _match_keyword(text, keywords)
        if kw:
            row["keyword"] = kw
            updated += 1
        else:
            row["keyword"] = row.get("keyword", "")

    with open(filepath, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDNAMES, extrasaction="ignore")
        w.writeheader()
        for row in rows:
            out = {k: row.get(k, "") for k in FIELDNAMES}
            w.writerow(out)

    return updated, len(rows)


def _add_keywords_telegram(filepath: Path) -> tuple[int, int]:
    rows = []
    with open(filepath, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        fieldnames = reader.fieldnames or list(FIELDNAMES)
        if "keyword" not in fieldnames:
            fieldnames = list(fieldnames) + ["keyword"]
        for row in reader:
            rows.append(row)

    updated = 0
    for row in rows:
        existing = (row.get("keyword") or "").strip()
        if existing:
            continue
        title = row.get("title", "")
        m = re.match(r"\[([^\]]+)\]", title)
        kw = m.group(1).strip() if m else "telegram"
        row["keyword"] = kw
        updated += 1

    with open(filepath, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDNAMES, extrasaction="ignore")
        w.writeheader()
        for row in rows:
            out = {k: row.get(k, "") for k in FIELDNAMES}
            w.writerow(out)

    return updated, len(rows)


def main():
    keywords = _load_keywords()
    print(f"키워드 {len(keywords)}개 로드")

    news_path = DATA_DIR / "news_merged.csv"
    if news_path.exists():
        u, t = _add_keywords_news(news_path, keywords)
        print(f"news_merged.csv: {u}건 키워드 추가 (총 {t}건)")
    else:
        print("news_merged.csv 없음")

    if TELEGRAM_CHATS_DIR.is_dir():
        tg_files = sorted(TELEGRAM_CHATS_DIR.glob("*.csv"))
        for tg_path in tg_files:
            u, t = _add_keywords_telegram(tg_path)
            print(f"telegram_chats/{tg_path.name}: {u}건 키워드 추가 (총 {t}건)")
        if not tg_files:
            print("telegram_chats/*.csv 없음")
    else:
        print("telegram_chats 폴더 없음")


if __name__ == "__main__":
    main()
