"""
뉴스 API 기반 수집 (프로그래밍 호출용)
- 끊겼던 부분부터 이어서 수집
- news_service, crawl_daum_list에서 공통 사용
"""
import csv
import html
import json
import re
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

from news_api import fetch_news_api
from config import NAVER_CLIENT_ID, NAVER_CLIENT_SECRET, KAKAO_REST_API_KEY

BACKEND_DIR = Path(__file__).resolve().parent
DATA_DIR = BACKEND_DIR / "lstm" / "data" / "news"
MERGED_FILENAME = "news_merged.csv"
KEYWORDS_FILE = "crawl_keywords.json"
PROGRESS_FILE = "crawl_list_progress.json"
FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _load_keywords() -> list:
    """crawl_keywords.json에서 키워드 로드. 종목명/기업명 포함 전부 사용 (제외 없음)."""
    p = DATA_DIR / KEYWORDS_FILE
    if p.exists():
        try:
            with open(p, "r", encoding="utf-8") as f:
                data = json.load(f)
                kw = data.get("keywords", [])
                if kw:
                    return kw
        except Exception:
            pass
    return ["주식", "코스피", "코스닥", "증시", "투자", "금융", "경제"]


def _load_existing_links() -> set:
    p = DATA_DIR / MERGED_FILENAME
    if not p.exists():
        return set()
    links = set()
    try:
        with open(p, "r", encoding="utf-8", newline="") as f:
            for row in csv.DictReader(f):
                lnk = (row.get("link") or "").strip()
                if lnk:
                    links.add(lnk)
    except Exception:
        pass
    return links


def _ensure_output_file():
    p = DATA_DIR / MERGED_FILENAME
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    if not p.exists():
        with open(p, "w", encoding="utf-8", newline="") as f:
            csv.DictWriter(f, fieldnames=FIELDNAMES).writeheader()


def _clean_text(text: str) -> str:
    """HTML 엔티티 제거 (&quot;, &amp; 등)"""
    if not text:
        return ""
    s = html.unescape(str(text))
    s = re.sub(r"&#\d+;", "", s)
    return s.strip()


def _append_rows(rows: list):
    p = DATA_DIR / MERGED_FILENAME
    file_exists = p.exists()
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    with open(p, "a", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDNAMES)
        if not file_exists:
            w.writeheader()
        for r in rows:
            row = {
                "title": _clean_text(r.get("title", "")),
                "link": (r.get("link") or "").strip(),
                "description": _clean_text(r.get("description", "")),
                "pubDate": (r.get("pubDate") or "").strip(),
                "keyword": (r.get("keyword") or keyword or "").strip(),
            }
            w.writerow(row)


def _load_progress(source: str) -> dict:
    p = DATA_DIR / PROGRESS_FILE
    if not p.exists():
        return {"completed_keywords": [], "total_saved": 0, "last_updated": None}
    try:
        with open(p, "r", encoding="utf-8") as f:
            data = json.load(f)
        sub = data.get(source, {})
        return {
            "completed_keywords": sub.get("completed_keywords", []),
            "total_saved": sub.get("total_saved", 0),
            "last_updated": sub.get("last_updated"),
        }
    except Exception:
        return {"completed_keywords": [], "total_saved": 0, "last_updated": None}


def _save_progress(source: str, completed_keywords: list, total_saved: int):
    from datetime import datetime
    p = DATA_DIR / PROGRESS_FILE
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    data = {}
    if p.exists():
        try:
            with open(p, "r", encoding="utf-8") as f:
                data = json.load(f)
        except Exception:
            pass
    data[source] = {
        "completed_keywords": completed_keywords,
        "total_saved": total_saved,
        "last_updated": datetime.now().isoformat(),
    }
    with open(p, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False)


def _fetch_keyword(keyword: str, source: str, max_results: int = 999999, max_pages: int = 0):
    result = fetch_news_api(source=source, query=keyword, max_results=max_results, max_pages=max_pages)
    rate_limited = result.get("rate_limited", False)
    items = result.get("items", [])
    if result.get("error") and not items and not rate_limited:
        raise RuntimeError(result.get("error", "Unknown error"))
    return (keyword, items, rate_limited)


def run_crawl_api(
    source: str = "naver",
    workers: int = 6,
    reset: bool = False,
    max_pages: int = 0,
    keywords_limit: int = 0,
    checkpoint_every: int = 100,
) -> dict:
    """
    API 기반 뉴스 수집 (끊겼던 부분부터 이어서)
    Returns: {total_saved, added_this_run, rate_limited, message, error}
    """
    _ensure_output_file()
    keywords = _load_keywords()
    if keywords_limit > 0:
        keywords = keywords[:keywords_limit]

    progress = _load_progress(source)
    if reset:
        progress = {"completed_keywords": [], "total_saved": 0, "last_updated": None}
        _save_progress(source, [], 0)

    if source == "naver" and (not NAVER_CLIENT_ID or not NAVER_CLIENT_SECRET):
        return {"error": "NAVER_CLIENT_ID, NAVER_CLIENT_SECRET 필요", "total_saved": 0, "added_this_run": 0}
    if source == "daum" and not KAKAO_REST_API_KEY:
        return {"error": "KAKAO_REST_API_KEY 필요", "total_saved": 0, "added_this_run": 0}

    completed = set(progress["completed_keywords"])
    keywords_to_do = [kw for kw in keywords if kw not in completed]
    if not keywords_to_do:
        return {
            "total_saved": progress["total_saved"],
            "added_this_run": 0,
            "rate_limited": False,
            "message": "남은 키워드 없음. 완료.",
        }

    existing = _load_existing_links()
    workers = min(workers, len(keywords_to_do)) or 1

    t0 = time.perf_counter()
    completed_keywords = list(completed)
    total_saved = progress["total_saved"]
    batch = []
    seen = set(existing)
    rate_limited_hit = False

    executor = ThreadPoolExecutor(max_workers=workers)
    try:
        futures = {
            executor.submit(_fetch_keyword, kw, source, 999999, max_pages): kw for kw in keywords_to_do
        }
        for future in as_completed(futures):
            kw = futures[future]
            try:
                kw2, items, rate_limited = future.result()
                if rate_limited:
                    rate_limited_hit = True
                    for it in items:
                        link = it.get("link", "")
                        if not link or link in seen:
                            continue
                        seen.add(link)
                        it["keyword"] = it.get("keyword") or kw2
                        batch.append(it)
                    if batch:
                        _append_rows(batch)
                        total_saved += len(batch)
                        batch = []
                    break
                completed_keywords.append(kw2)
                for it in items:
                    link = it.get("link", "")
                    if not link or link in seen:
                        continue
                    seen.add(link)
                    it["keyword"] = it.get("keyword") or kw2
                    batch.append(it)
                    if len(batch) >= checkpoint_every:
                        _append_rows(batch)
                        total_saved += len(batch)
                        _save_progress(source, completed_keywords, total_saved)
                        batch = []
            except Exception:
                pass
    finally:
        try:
            executor.shutdown(wait=not rate_limited_hit, cancel_futures=rate_limited_hit)
        except TypeError:
            executor.shutdown(wait=not rate_limited_hit)

    if batch:
        _append_rows(batch)
        total_saved += len(batch)
    _save_progress(source, completed_keywords, total_saved)

    added_this_run = total_saved - progress["total_saved"]
    elapsed = time.perf_counter() - t0
    msg = f"이번 실행 {added_this_run}건, 누적 {total_saved}건"
    if rate_limited_hit:
        msg += " (호출 제한 도달, 저장됨. 다음 실행 시 이어서 진행)"
    return {
        "total_saved": total_saved,
        "added_this_run": added_this_run,
        "rate_limited": rate_limited_hit,
        "elapsed_sec": round(elapsed, 1),
        "message": msg,
    }
