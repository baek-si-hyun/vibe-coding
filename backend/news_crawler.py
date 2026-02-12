import re
import httpx
import time
from typing import Dict, List, Optional
from datetime import datetime, timedelta
from bs4 import BeautifulSoup
from urllib.parse import quote, urlencode


def _relative_to_date(text: str) -> str:
    if not text or not isinstance(text, str):
        return ""
    text = text.strip()
    now = datetime.now()
    m = re.search(r"(\d+)\s*분\s*전", text)
    if m:
        delta = timedelta(minutes=int(m.group(1)))
        return (now - delta).strftime("%Y-%m-%d")
    m = re.search(r"(\d+)\s*시간\s*전", text)
    if m:
        delta = timedelta(hours=int(m.group(1)))
        return (now - delta).strftime("%Y-%m-%d")
    m = re.search(r"(\d+)\s*일\s*전", text)
    if m:
        delta = timedelta(days=int(m.group(1)))
        return (now - delta).strftime("%Y-%m-%d")
    if "어제" in text:
        return (now - timedelta(days=1)).strftime("%Y-%m-%d")
    if "오늘" in text or "방금" in text:
        return now.strftime("%Y-%m-%d")
    m = re.search(r"(\d+)\s*주\s*전", text)
    if m:
        delta = timedelta(weeks=int(m.group(1)))
        return (now - delta).strftime("%Y-%m-%d")
    m = re.search(r"(\d+)\s*개?월\s*전", text)
    if m:
        delta = timedelta(days=30 * int(m.group(1)))
        return (now - delta).strftime("%Y-%m-%d")
    m = re.search(r"(\d{4})[.\-](\d{1,2})[.\-](\d{1,2})", text)
    if m:
        y, mo, d = int(m.group(1)), int(m.group(2)), int(m.group(3))
        try:
            return datetime(y, mo, d).strftime("%Y-%m-%d")
        except ValueError:
            pass
    return ""


def _date_from_url(link: str) -> str:
    if not link:
        return ""
    m = re.search(r"v\.daum\.net/v/(\d{4})(\d{2})(\d{2})", link)
    if m:
        try:
            return datetime(int(m.group(1)), int(m.group(2)), int(m.group(3))).strftime("%Y-%m-%d")
        except ValueError:
            pass
    m = re.search(r"/(\d{4})[/\-]?(\d{1,2})[/\-]?(\d{1,2})", link)
    if m:
        try:
            return datetime(int(m.group(1)), int(m.group(2)), int(m.group(3))).strftime("%Y-%m-%d")
        except ValueError:
            pass
    return ""


class NewsCrawler:
    def __init__(self):
        self.rate_limit_delay = 0.3
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
            "Accept-Language": "ko-KR, ko;q=0.9, en-US;q=0.8, en;q=0.7",
            "Accept-Encoding": "gzip, deflate, br",
            "Referer": "https://www.naver.com/",
            "Sec-Ch-Ua": '"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"',
            "Sec-Ch-Ua-Mobile": "?0",
            "Sec-Ch-Ua-Platform": '"Windows"',
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "same-origin",
            "Sec-Fetch-User": "?1",
            "Upgrade-Insecure-Requests": "1",
        }
    
    def fetch(self, query: str, date: Optional[str] = None, **kwargs) -> Dict:
        raise NotImplementedError


class NaverNewsCrawler(NewsCrawler):
    def __init__(self):
        super().__init__()
        self.base_url = "https://search.naver.com/search.naver"
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "ko-KR,ko;q=0.9,en;q=0.8",
            "Referer": "https://www.naver.com/",
        }

    def fetch(self, query: str, date: Optional[str] = None,
              date_start: Optional[str] = None,
              date_end: Optional[str] = None,
              max_results: Optional[int] = None, display: Optional[int] = None,
              start: Optional[int] = None, max_pages: Optional[int] = None,
              **kwargs) -> Dict:
        max_results = max_results or 999999
        if display is not None:
            max_results = display
        current_start = start if start is not None else 1

        ds_val, de_val = None, None
        if date_start and date_end:
            try:
                ds_val = datetime.strptime(date_start, "%Y%m%d").strftime("%Y.%m.%d")
                de_val = datetime.strptime(date_end, "%Y%m%d").strftime("%Y.%m.%d")
            except ValueError:
                pass
        elif date:
            try:
                d = datetime.strptime(date, "%Y%m%d").strftime("%Y.%m.%d")
                ds_val, de_val = d, d
            except ValueError:
                pass

        all_items = []
        page_count = 0
        stop_crawl = False

        while len(all_items) < max_results and not stop_crawl:
            if max_pages is not None and page_count >= max_pages:
                break
            params = {
                "where": "news",
                "query": query,
                "start": current_start,
                "sort": 1,
            }
            if ds_val and de_val:
                params["pd"] = 3
                params["ds"] = ds_val
                params["de"] = de_val

            time.sleep(self.rate_limit_delay)

            for retry in range(4):
                try:
                    with httpx.Client(
                        timeout=30.0,
                        follow_redirects=True,
                        headers=self.headers,
                    ) as client:
                        response = client.get(self.base_url, params=params)
                        response.raise_for_status()
                    soup = BeautifulSoup(response.text, "lxml")
                    grp = soup.select_one("div.group_news")
                    if not grp:
                        break

                    profiles = grp.select('div.sds-comps-profile[data-sds-comp="Profile"]')
                    for pr in profiles:
                        if len(all_items) >= max_results:
                            break
                        press_el = pr.select_one("span.sds-comps-profile-info-title-text")
                        date_el = pr.select_one("span.sds-comps-profile-info-subtext")
                        press = (press_el.get_text(strip=True) or "").strip() if press_el else ""
                        pub_str = (date_el.get_text(strip=True) or "").strip() if date_el else ""
                        pub_date = _relative_to_date(pub_str) or pub_str

                        parent = pr.parent
                        title, link, description = "", "", ""
                        for _ in range(8):
                            if not parent:
                                break
                            tit_a = parent.select_one('a[data-heatmap-target=".tit"]')
                            if tit_a and tit_a.get("href") and "naver.com" not in (tit_a.get("href") or ""):
                                link = (tit_a.get("href") or "").strip()
                                title = (tit_a.get_text(strip=True) or "").strip()
                                body_a = parent.select_one('a[data-heatmap-target=".body"]')
                                description = (body_a.get_text(strip=True) or "").strip() if body_a else ""
                                break
                            parent = parent.parent

                        if not title or not link or link in {x.get("link") for x in all_items}:
                            continue
                        if not pub_date and link:
                            pub_date = _date_from_url(link)
                        all_items.append({
                            "title": title,
                            "link": link,
                            "description": description,
                            "press": press,
                            "pubDate": pub_date,
                        })

                    if not profiles:
                        stop_crawl = True
                    else:
                        current_start += 10
                        page_count += 1
                        if current_start > 1000:
                            stop_crawl = True
                    break
                except httpx.HTTPStatusError as e:
                    if e.response.status_code in (429, 403):
                        wait = (2 ** retry) * 5
                        print(f"[HTTP {e.response.status_code} → {wait}초 대기 후 재시도 ({retry+1}/4)]", flush=True)
                        time.sleep(wait)
                        continue
                    raise Exception(f"네이버 뉴스 크롤링 실패: HTTP {e.response.status_code}")
                except Exception as e:
                    if retry < 3:
                        raise Exception(f"네이버 뉴스 크롤링 실패: {str(e)}")
            else:
                break
            if stop_crawl:
                break

        return {
            "query": query,
            "date": date,
            "fetchedAt": datetime.now().isoformat(),
            "source": "naver",
            "total": len(all_items),
            "count": len(all_items),
            "items": all_items[:max_results],
        }
    
    def fetch_all_pages(self, query: str, date: Optional[str] = None, 
                        max_results: int = 1000) -> Dict:
        return self.fetch(query, date, max_results=max_results)


_DAUM_PRESS_SUFFIXES = (
    "이데일리", "연합뉴스", "연합인포맥스", "매일경제", "한국경제", "서울경제",
    "조선비즈", "파이낸셜뉴스", "아주경제", "헤럴드경제", "더팩트", "브릿지경제",
    "메트로경제", "이투데이", "한국경제TV", "시사IN", "노컷뉴스", "SBS", "KBS",
    "JTBC", "채널A", "MBC", "뉴스1", "뉴시스", "한겨레", "경향신문", "동아일보",
    "중앙일보", "조선일보", "국민일보", "문화일보", "한스경제", "데일리안",
    "디지털타임스", "전자신문", "지디넷코리아", "시사저널", "경상일보",
)

DAUM_SECTIONS = {
    "economy": "https://news.daum.net/economy",
    "stock": "https://news.daum.net/stock",
    "politics": "https://news.daum.net/politics",
    "society": "https://news.daum.net/society",
    "policy": "https://news.daum.net/policy",
    "industry": "https://news.daum.net/industry",
    "finance": "https://news.daum.net/finance",
    "estate": "https://news.daum.net/estate",
    "coin": "https://news.daum.net/coin",
}


class DaumNewsCrawler(NewsCrawler):
    def __init__(self):
        super().__init__()
        self.base_url = "https://search.daum.net/search"
        self.section_base = "https://news.daum.net"
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "ko-KR,ko;q=0.9,en;q=0.8",
            "Referer": "https://www.daum.net/",
        }
    
    def fetch(self, query: str, date: Optional[str] = None,
              date_start: Optional[str] = None,
              date_end: Optional[str] = None,
              max_results: Optional[int] = None, size: Optional[int] = None,
              page: Optional[int] = None, max_pages: Optional[int] = None,
              **kwargs) -> Dict:
        if size is not None:
            max_results = size
        if page is not None:
            initial_page = page
        else:
            initial_page = 1
        
        all_items = []
        current_page = initial_page
        max_results = max_results or 999999
        
        sd_val, ed_val = None, None
        if date_start and date_end:
            try:
                sd_val = datetime.strptime(date_start, "%Y%m%d").strftime("%Y%m%d")
                ed_val = datetime.strptime(date_end, "%Y%m%d").strftime("%Y%m%d")
            except ValueError:
                pass
        elif date:
            try:
                d = datetime.strptime(date, "%Y%m%d").strftime("%Y%m%d")
                sd_val, ed_val = d, d
            except ValueError:
                pass
        
        while len(all_items) < max_results:
            if max_pages is not None and current_page > max_pages:
                break
            params = {
                "w": "news",
                "q": query,
                "p": current_page,
                "DA": "PGD",
                "cluster": "y",
            }
            if sd_val and ed_val:
                params["period"] = "u"
                params["sd"] = sd_val
                params["ed"] = ed_val
            
            time.sleep(self.rate_limit_delay)
            
            stop_crawl = False
            for retry in range(4):
                try:
                    with httpx.Client(timeout=30.0, follow_redirects=True, headers=self.headers) as client:
                        response = client.get(self.base_url, params=params)
                        response.raise_for_status()
                        soup = BeautifulSoup(response.text, "lxml")
                
                    list_items = soup.select("li[data-docid]")
                    for li in list_items:
                        if len(all_items) >= max_results:
                            break
                        tit_a = li.select_one("strong.tit-g a[href*='v.daum.net/v/']")
                        if not tit_a:
                            continue
                        title = (tit_a.get_text(strip=True) or "").strip()
                        link = (tit_a.get("href") or "").strip()
                        if not link or not title or link in {x.get("link") for x in all_items}:
                            continue
                        desc_el = li.select_one("p.conts-desc")
                        description = (desc_el.get_text(strip=True) or "").strip() if desc_el else ""
                        date_el = li.select_one("span.gem-subinfo span.txt_info")
                        pub_date = (date_el.get_text(strip=True) or "").strip() if date_el else ""
                        pub_date = _relative_to_date(pub_date) or pub_date or _date_from_url(link)
                        press_el = li.select_one("strong.tit_item span.txt_info")
                        press = (press_el.get_text(strip=True) or "").strip() if press_el else ""
                        if press == "뉴스검색 설정 안내":
                            press = ""
                        all_items.append({
                            "title": title,
                            "link": link,
                            "description": description,
                            "press": press,
                            "pubDate": pub_date,
                        })
        
                    if not list_items:
                        stop_crawl = True
                    else:
                        current_page += 1
                    break  # retry 성공
                except httpx.HTTPStatusError as e:
                    if e.response.status_code in (429, 403):
                        wait = (2 ** retry) * 5
                        print(f"[HTTP {e.response.status_code} → {wait}초 대기 후 재시도 ({retry+1}/4)]", flush=True)
                        time.sleep(wait)
                        continue
                    raise Exception(f"다음 뉴스 크롤링 실패: HTTP {e.response.status_code}")
                except Exception as e:
                    if retry < 3:
                        raise Exception(f"다음 뉴스 크롤링 실패: {str(e)}")
            else:
                break  # 모든 retry 실패
            if stop_crawl:
                break
        
        return {
            "query": query,
            "date": date,
            "fetchedAt": datetime.now().isoformat(),
            "source": "daum",
            "total": len(all_items),
            "count": len(all_items),
            "items": all_items[:max_results]
        }
    
    def fetch_section(self, section: str, max_results: int = 100) -> Dict:
        if section not in DAUM_SECTIONS:
            raise ValueError(
                f"지원하지 않는 섹션: {section}. "
                f"지원: {', '.join(DAUM_SECTIONS.keys())}"
            )
        
        url = DAUM_SECTIONS[section]
        seen_links: set = set()
        all_items: List[Dict] = []
        
        press_time_re = re.compile(
            r"([가-힣a-zA-Z0-9\[\]\-]{2,25}?)"
            r"(\d+분\s*전|\d+시간\s*전|\d+시간전|\d+일\s*전|방금\s*전|오늘|\d{4}\.\d{1,2}\.\d{1,2})\s*$",
        )
        time_only_re = re.compile(
            r"(\d+분\s*전|\d+시간\s*전|\d+시간전|\d+일\s*전|방금\s*전|오늘|\d{4}\.\d{1,2}\.\d{1,2})"
        )
        
        time.sleep(self.rate_limit_delay)
        
        try:
            with httpx.Client(timeout=30.0, follow_redirects=True, headers=self.headers) as client:
                response = client.get(url)
                response.raise_for_status()
                soup = BeautifulSoup(response.text, "lxml")
                
                for a in soup.select('a[href*="v.daum.net/v/"]'):
                    if len(all_items) >= max_results:
                        break
                    
                    href = (a.get("href") or "").strip()
                    if not href or href in seen_links:
                        continue
                    if any(x in href for x in ["channel.", "correct", "cplist", "cs.daum", "issue.daum"]):
                        continue
                    
                    full_text = (a.get_text(strip=True) or "").strip()
                    if len(full_text) < 5:
                        continue
                    
                    title = ""
                    strong_el = a.select_one("strong, b, [class*='tit']")
                    if strong_el:
                        title = (strong_el.get_text(strip=True) or "").strip()
                    if not title:
                        title = full_text[:80] if len(full_text) > 80 else full_text
                    if len(title) < 5:
                        continue
                    
                    press = ""
                    pub_date = ""
                    description = ""
                    m = press_time_re.search(full_text)
                    if m:
                        press = (m.group(1) or "").strip()
                        for suffix in _DAUM_PRESS_SUFFIXES:
                            if suffix in press and press.endswith(suffix):
                                press = suffix
                                break
                        pub_date = _relative_to_date(m.group(2))
                        content_before = full_text[: m.start()].strip()
                        description = content_before.replace(title, "", 1).strip()
                        if description and len(description) < 20 and "…" in description:
                            description = ""
                    else:
                        m2 = time_only_re.search(full_text)
                        if m2:
                            pub_date = _relative_to_date(m2.group(1))
                    if not pub_date and href:
                        pub_date = _date_from_url(href)
                    
                    if len(description) > 500:
                        description = description[:497] + "..."
                    
                    seen_links.add(href)
                    all_items.append({
                        "title": title[:200],
                        "link": href,
                        "description": description[:500],
                        "press": press[:50],
                        "pubDate": pub_date,
                    })
        
        except httpx.HTTPStatusError as e:
            if e.response.status_code == 429:
                raise Exception("다음 뉴스 요청 제한(429). 잠시 후 다시 시도하세요.")
            raise Exception(f"다음 뉴스 섹션 크롤링 실패: HTTP {e.response.status_code}")
        except Exception as e:
            raise Exception(f"다음 뉴스 섹션 크롤링 실패: {str(e)}")
        
        return {
            "section": section,
            "url": url,
            "fetchedAt": datetime.now().isoformat(),
            "source": "daum",
            "total": len(all_items),
            "count": len(all_items),
            "items": all_items,
        }
    
    def fetch_all_pages(self, query: str, date: Optional[str] = None, 
                        max_results: int = 1000) -> Dict:
        return self.fetch(query, date, max_results=max_results)


def get_news_crawler(source: str) -> NewsCrawler:
    if source == "naver":
        return NaverNewsCrawler()
    elif source == "daum":
        return DaumNewsCrawler()
    else:
        raise ValueError(f"알 수 없는 소스: {source}. 지원: naver, daum")
