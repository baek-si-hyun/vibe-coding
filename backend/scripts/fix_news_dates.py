#!/usr/bin/env python3
import csv
import re
from datetime import datetime
from pathlib import Path


def normalize_date(val: str, default_today: str) -> str:
    if not val or not str(val).strip():
        return default_today
    val = str(val).strip()
    m = re.match(r"^(\d{4})-(\d{1,2})-(\d{1,2})", val)
    if m:
        try:
            datetime(int(m.group(1)), int(m.group(2)), int(m.group(3)))
            return m.group(1) + "-" + m.group(2).zfill(2) + "-" + m.group(3).zfill(2)
        except ValueError:
            pass
    m = re.match(r"^(\d{4})\.(\d{1,2})\.(\d{1,2})", val)
    if m:
        try:
            d = datetime(int(m.group(1)), int(m.group(2)), int(m.group(3)))
            return d.strftime("%Y-%m-%d")
        except ValueError:
            pass
    return default_today


def main():
    data_dir = Path(__file__).parent.parent / "lstm" / "data" / "news"
    path = data_dir / "news_merged.csv"
    if not path.exists():
        print("news_merged.csv 없음")
        return

    today = datetime.now().strftime("%Y-%m-%d")
    items = []
    fixed_count = 0
    empty_filled = 0

    with open(path, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        fieldnames = reader.fieldnames or ["title", "link", "description", "pubDate"]
        for row in reader:
            old_pd = (row.get("pubDate") or "").strip()
            new_pd = normalize_date(old_pd, default_today=today)
            if old_pd != new_pd:
                fixed_count += 1
            if not old_pd and new_pd:
                empty_filled += 1
            row["pubDate"] = new_pd
            items.append(row)

    with open(path, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames)
        w.writeheader()
        w.writerows(items)

    print(f"완료: {path}")
    print(f"  - 수정된 행: {fixed_count}건")
    print(f"  - 빈 값→오늘 채움: {empty_filled}건 (1회 한정)")


if __name__ == "__main__":
    main()
