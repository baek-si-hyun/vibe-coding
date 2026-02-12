import re
import time
from typing import Dict

import httpx

from config import NAVER_CLIENT_ID, NAVER_CLIENT_SECRET, KAKAO_REST_API_KEY


def _strip_html(text: str) -> str:
    if not text:
        return ""
    return re.sub(r"<[^>]+>", "", text).strip()


def _parse_pubdate_naver(pub_date: str) -> str:
    if not pub_date:
        return ""
    try:
        from email.utils import parsedate_to_datetime
        dt = parsedate_to_datetime(pub_date)
        return dt.strftime("%Y-%m-%d")
    except Exception:
        return pub_date


def _parse_pubdate_kakao(dt_str: str) -> str:
    if not dt_str:
        return ""
    m = re.search(r"(\d{4})-(\d{2})-(\d{2})", dt_str)
    if m:
        return f"{m.group(1)}-{m.group(2)}-{m.group(3)}"
    return dt_str


def _extract_press_from_url(url: str) -> str:
    if not url:
        return ""
    m = re.search(r"([a-z0-9-]+)\.(daum|naver|yonhap|hani|donga|chosun|joongang|mt|mk|hankyung|etnews|yna|khan|hani|sedaily)\.(net|co\.kr|com)", url, re.I)
    if m:
        return m.group(1)
    return ""


def fetch_naver_news(
    query: str,
    display: int = 10,
    start: int = 1,
    sort: str = "date",
) -> Dict:
    if not NAVER_CLIENT_ID or not NAVER_CLIENT_SECRET:
        return {"items": [], "total": 0, "error": "NAVER_CLIENT_ID/SECRET 미설정"}

    url = "https://openapi.naver.com/v1/search/news.json"
    params = {
        "query": query,
        "display": min(100, max(1, display)),
        "start": min(1000, max(1, start)),
        "sort": "date" if sort == "date" else "sim",
    }
    headers = {
        "X-Naver-Client-Id": NAVER_CLIENT_ID,
        "X-Naver-Client-Secret": NAVER_CLIENT_SECRET,
    }

    try:
        with httpx.Client(timeout=15) as client:
            resp = client.get(url, params=params, headers=headers)
        resp.raise_for_status()
        data = resp.json()
    except httpx.HTTPStatusError as e:
        rate_limited = e.response.status_code in (429, 403)
        err_msg = str(e)
        if rate_limited or "limit" in err_msg.lower() or "한도" in err_msg or "quota" in err_msg.lower():
            return {"items": [], "total": 0, "error": err_msg, "rate_limited": True}
        return {"items": [], "total": 0, "error": err_msg}
    except Exception as e:
        return {"items": [], "total": 0, "error": str(e)}

    total = data.get("total", 0)
    raw_items = data.get("items", [])
    items = []
    for it in raw_items:
        link = it.get("link") or it.get("originallink", "")
        press = _extract_press_from_url(link)
        if not press and it.get("originallink"):
            press = _extract_press_from_url(it.get("originallink", ""))
        items.append({
            "title": _strip_html(it.get("title", "")),
            "link": link,
            "description": _strip_html(it.get("description", "")),
            "press": press or "",
            "pubDate": _parse_pubdate_naver(it.get("pubDate", "")),
        })
    return {"items": items, "total": total}


def fetch_kakao_web(
    query: str,
    size: int = 10,
    page: int = 1,
    sort: str = "recency",
) -> Dict:
    if not KAKAO_REST_API_KEY:
        return {"items": [], "total": 0, "error": "KAKAO_REST_API_KEY 미설정"}

    url = "https://dapi.kakao.com/v2/search/web"
    params = {
        "query": query,
        "size": min(50, max(1, size)),
        "page": min(50, max(1, page)),
        "sort": "recency" if sort == "recency" else "accuracy",
    }
    headers = {"Authorization": f"KakaoAK {KAKAO_REST_API_KEY}"}

    try:
        with httpx.Client(timeout=15) as client:
            resp = client.get(url, params=params, headers=headers)
        resp.raise_for_status()
        data = resp.json()
    except httpx.HTTPStatusError as e:
        rate_limited = e.response.status_code in (429, 403)
        err_msg = str(e)
        if rate_limited or "limit" in err_msg.lower() or "한도" in err_msg or "quota" in err_msg.lower():
            return {"items": [], "total": 0, "error": err_msg, "rate_limited": True}
        return {"items": [], "total": 0, "error": err_msg}
    except Exception as e:
        return {"items": [], "total": 0, "error": str(e)}

    meta = data.get("meta", {})
    total = meta.get("total_count", 0)
    raw_items = data.get("documents", [])
    items = []
    for it in raw_items:
        link = it.get("url", "")
        items.append({
            "title": _strip_html(it.get("title", "")),
            "link": link,
            "description": _strip_html(it.get("contents", "")),
            "press": _extract_press_from_url(link),
            "pubDate": _parse_pubdate_kakao(it.get("datetime", "")),
        })
    return {"items": items, "total": total}


MIN_DATE_DEFAULT = "2010-01-01"


def _is_valid_date(pub_date: str, min_date: str) -> bool:
    if not pub_date or len(pub_date) < 10:
        return False
    return pub_date >= min_date


def fetch_news_api(
    source: str,
    query: str,
    max_results: int = 999999,
    min_date: str = MIN_DATE_DEFAULT,
    max_pages: int = 0,
) -> Dict:
    all_items = []
    display = 100 if source == "naver" else 10
    per_page = 50 if source == "daum" else 100
    page = 0

    while True:
        if max_pages > 0 and page >= max_pages:
            break
        if source == "naver":
            start = page * display + 1
            if start > 1000:
                break
            result = fetch_naver_news(query, display=min(display, 1000 - start + 1), start=start, sort="date")
        elif source == "daum":
            if page >= 50:
                break
            result = fetch_kakao_web(query, size=per_page, page=page + 1, sort="recency")
        else:
            return {"items": [], "error": f"지원하지 않는 소스: {source}"}

        if result.get("error"):
            if result.get("rate_limited"):
                return {"items": all_items, "total": len(all_items), "rate_limited": True, "error": result.get("error")}
            if page == 0:
                return result
            break

        items = result.get("items", [])
        for it in items:
            if _is_valid_date(it.get("pubDate", ""), min_date):
                all_items.append(it)
        if len(all_items) >= max_results:
            break
        if not items:
            break
        page += 1
        time.sleep(0.1)

    return {"items": all_items[:max_results], "total": len(all_items)}
