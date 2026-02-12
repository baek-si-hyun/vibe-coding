#!/usr/bin/env python3
import csv
import re
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
CHATS_DIR = PROJECT_ROOT / "backend" / "lstm" / "data" / "telegram_chats"
FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _extract_urls(text: str) -> list[str]:
    return re.findall(r"https?://[^\s]+", text)


def _strip_urls_from_text(text: str) -> str:
    return re.sub(r"https?://[^\s]+", "", text).strip()


def _clean_description(desc: str) -> str:
    return re.sub(r"\s+", " ", _strip_urls_from_text(desc)).strip()


def _split_description_by_links(desc: str) -> list[tuple[str, str]]:
    tokens = re.split(r"(https?://[^\s]+)", desc)
    parts = []
    for i in range(1, len(tokens), 2):
        text_before = tokens[i - 1].strip() if i > 0 else ""
        url = tokens[i] if i < len(tokens) else ""
        if url:
            text_before = re.sub(r"\s+", " ", text_before).strip()
            text_before = re.sub(r"https?://[^\s]+", "", text_before).strip()
            parts.append((text_before or url[:80], url))
    return parts


def _process_csv(filepath: Path) -> list[dict]:
    rows = []
    with open(filepath, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append(row)

    by_desc: dict[str, list[dict]] = {}
    for row in rows:
        desc = row.get("description", "")
        key = (desc[:500], row.get("pubDate", ""), row.get("keyword", ""))
        if key not in by_desc:
            by_desc[key] = []
        by_desc[key].append(row)

    out = []
    for (desc_prefix, pub_date, keyword), group in by_desc.items():
        all_links = []
        for row in group:
            link = row.get("link", "")
            if link.startswith("http"):
                all_links.append(link)
            elif "|" in link:
                all_links.extend(link.split("|"))

        full_desc = group[0].get("description", "")
        link_str = "|".join(dict.fromkeys(all_links)) if all_links else (group[0].get("link", ""))

        url_matches = _extract_urls(full_desc)
        if len(url_matches) >= 2 and len(url_matches) == len(all_links):
            parts = _split_description_by_links(full_desc)
            for text, url in parts:
                if not text or not url:
                    continue
                desc_clean = _clean_description(text)
                title_part = desc_clean[:80].replace(chr(10), ' ')
                title = f"[{keyword}] {title_part}" if keyword else title_part
                out.append({
                    "title": title,
                    "link": url,
                    "description": desc_clean,
                    "pubDate": pub_date,
                    "keyword": keyword,
                })
        else:
            title = _clean_description(group[0].get("title", ""))
            if keyword and not title.startswith("["):
                title = f"[{keyword}] {title[:80]}"
            desc_clean = _clean_description(full_desc)
            out.append({
                "title": title,
                "link": link_str,
                "description": desc_clean,
                "pubDate": pub_date,
                "keyword": keyword,
            })

    return out


def main():
    for csv_path in CHATS_DIR.glob("*.csv"):
        print(f"처리 중: {csv_path.name}")
        fixed = _process_csv(csv_path)
        seen = set()
        unique = []
        for row in fixed:
            k = (row.get("link", ""), row.get("description", "")[:300])
            if k in seen:
                continue
            seen.add(k)
            unique.append(row)

        with open(csv_path, "w", encoding="utf-8", newline="") as f:
            w = csv.DictWriter(f, fieldnames=FIELDNAMES)
            w.writeheader()
            w.writerows(unique)
        print(f"  -> {len(unique)}건 (기존 {len(fixed)}건)")


if __name__ == "__main__":
    main()
