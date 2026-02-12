#!/usr/bin/env python3
import csv
import re
import shutil
from html import unescape
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
OUTPUT_DIR = PROJECT_ROOT / "backend" / "lstm" / "data" / "telegram_chats"
FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _sanitize_filename(name: str) -> str:
    name = re.sub(r'[<>:"/\\|?*]', "_", name.strip())
    return name[:100] if name else "unnamed"


def _parse_date(title_attr: str) -> str:
    if not title_attr:
        return ""
    m = re.match(r"(\d{1,2})\.(\d{1,2})\.(\d{4})", title_attr.strip())
    if m:
        return f"{m.group(3)}-{m.group(2).zfill(2)}-{m.group(1).zfill(2)}"
    return ""


def _split_into_items(html_text: str) -> list[str]:
    if not html_text:
        return []
    text = re.sub(r"<br\s*/?>\s*<br\s*/?>", "\n\nSPLIT\n\n", html_text, flags=re.I)
    items = [s.strip() for s in text.split("\n\nSPLIT\n\n") if s.strip()]
    return items if items else [html_text]


def _extract_links_and_text_from_item(html_fragment: str) -> tuple[str, list[str]]:
    if not html_fragment:
        return "", []
    text = unescape(html_fragment)
    links = []
    for m in re.finditer(r'<a\s+href="([^"]+)"', text, re.I):
        url = unescape(m.group(1).strip())
        if url.startswith("http"):
            links.append(url)
    plain = re.sub(r"<br\s*/?>", "\n", text, flags=re.I)
    plain = re.sub(r"<[^>]+>", "", plain)
    plain = re.sub(r"\s+", " ", plain).strip()
    return plain, links


def _parse_messages_html(html_path: Path) -> tuple[str, list[dict]]:
    chat_name = ""
    rows = []

    with open(html_path, "r", encoding="utf-8") as f:
        content = f.read()

    m = re.search(r'class="text bold"[^>]*>\s*([^<]+)\s*<', content)
    if m:
        chat_name = unescape(m.group(1).strip())

    for msg_m in re.finditer(r'<div class="message default clearfix[^"]*"[^>]*id="([^"]*)"', content):
        if "service" in msg_m.group(0):
            continue
        msg_id = msg_m.group(1)
        chunk = content[msg_m.end() : msg_m.end() + 10000]
        date_m = re.search(r'class="[^"]*date[^"]*"[^>]*title="([^"]*)"', chunk)
        pub_date = _parse_date(date_m.group(1)) if date_m else ""
        text_m = re.search(r'<div class="text">(.*?)</div>', chunk, re.DOTALL)
        if not text_m:
            continue
        fragment = text_m.group(1)
        items = _split_into_items(fragment)
        keyword = chat_name.strip() or "telegram"
        for item_html in items:
            description, links = _extract_links_and_text_from_item(item_html)
            if not description and not links:
                continue
            title = f"[{chat_name}] {description[:80].replace(chr(10), ' ')}" if description else f"[{chat_name}]"
            link_val = "|".join(links) if links else f"telegram:{_sanitize_filename(chat_name)}:{msg_id}"
            rows.append({"title": title, "link": link_val, "description": description, "pubDate": pub_date, "keyword": keyword})

    return chat_name, rows


def _collect_chat_data(export_dir: Path) -> dict[str, list[dict]]:
    chats: dict[str, list[dict]] = {}
    for html_file in sorted(export_dir.glob("messages*.html")):
        try:
            chat_name, rows = _parse_messages_html(html_file)
            if not chat_name:
                chat_name = "unnamed"
            if chat_name not in chats:
                chats[chat_name] = []
            chats[chat_name].extend(rows)
        except Exception:
            continue
    return chats


def _write_chat_csv(filepath: Path, rows: list[dict]):
    seen = set()
    with open(filepath, "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDNAMES)
        w.writeheader()
        for row in rows:
            key = (row.get("link", ""), row.get("description", "")[:200])
            if key in seen:
                continue
            seen.add(key)
            w.writerow(row)


def main():
    import argparse

    parser = argparse.ArgumentParser(description="텔레그램 ChatExport HTML → telegram_chats CSV 변환")
    parser.add_argument(
        "folders",
        nargs="*",
        help="ChatExport 폴더 경로 (예: ChatExport_2026-02-12). 생략 시 프로젝트 루트에서 ChatExport_* 검색",
    )
    parser.add_argument("--no-delete", action="store_true", help="변환 후 ChatExport 폴더 삭제 안 함")
    args = parser.parse_args()

    if args.folders:
        patterns = [Path(f).name for f in args.folders]
        export_dirs = [Path(f).resolve() for f in args.folders if Path(f).is_dir()]
    else:
        patterns = [d.name for d in PROJECT_ROOT.iterdir() if d.is_dir() and d.name.startswith("ChatExport_")]
        export_dirs = [PROJECT_ROOT / p for p in patterns]

    if not export_dirs:
        print("ChatExport 폴더가 없습니다. 폴더 경로를 인자로 지정하세요.")
        return

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    all_chats: dict[str, list[dict]] = {}

    for export_dir in export_dirs:
        if not export_dir.is_dir():
            continue
        print(f"처리 중: {export_dir.name}")
        chats = _collect_chat_data(export_dir)
        for chat_name, rows in chats.items():
            if chat_name not in all_chats:
                all_chats[chat_name] = []
            all_chats[chat_name].extend(rows)

    print(f"\n채팅방 {len(all_chats)}개 변환 완료")

    for chat_name, rows in all_chats.items():
        if not rows:
            continue
        safe_name = _sanitize_filename(chat_name)
        filepath = OUTPUT_DIR / f"{safe_name}.csv"
        _write_chat_csv(filepath, rows)
        print(f"  - {chat_name}: {len(rows)}건 -> {filepath.name}")

    if not args.no_delete:
        for export_dir in export_dirs:
            if export_dir.is_dir():
                try:
                    shutil.rmtree(export_dir)
                    print(f"삭제 완료: {export_dir.name}")
                except Exception as e:
                    print(f"삭제 실패 {export_dir.name}: {e}")


if __name__ == "__main__":
    main()
