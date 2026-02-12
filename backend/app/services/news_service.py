import csv
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Optional

from news_crawler import get_news_crawler
from crawl_api import run_crawl_api


def _get_news_data_dir() -> Path:
    project_root = Path(__file__).parent.parent.parent
    return project_root / "lstm" / "data" / "news"


MERGED_FILENAME = "news_merged.csv"
FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _read_csv_items(filepath: Path) -> List[Dict]:
    items = []
    with open(filepath, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            items.append({
                "title": row.get("title", ""),
                "link": row.get("link", ""),
                "description": row.get("description", ""),
                "pubDate": row.get("pubDate", ""),
                "keyword": row.get("keyword", ""),
            })
    return items


def _dedupe_by_link(items: List[Dict]) -> List[Dict]:
    seen: set = set()
    out: List[Dict] = []
    for item in items:
        link = (item.get("link") or "").strip()
        if not link or link in seen:
            continue
        seen.add(link)
        out.append(item)
    return out


class NewsService:
    @staticmethod
    def fetch_news(query: str, source: str = "naver", date: Optional[str] = None, 
                   max_results: int = 100, **kwargs) -> Dict:
        if not query:
            raise ValueError("검색어(query)가 필요합니다.")
        
        if source not in ["naver", "daum"]:
            raise ValueError(f"지원하지 않는 소스: {source}. 지원: naver, daum")
        
        if date:
            try:
                datetime.strptime(date, "%Y%m%d")
            except ValueError:
                raise ValueError(f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})")
        
        crawler = get_news_crawler(source)
        
        if max_results > 100:
            result = crawler.fetch_all_pages(query, date, max_results=max_results)
        else:
            result = crawler.fetch(query, date, max_results=max_results, **kwargs)
        
        return result
    
    @staticmethod
    def fetch_daum_section(section: str, max_results: int = 100) -> Dict:
        from news_crawler import DAUM_SECTIONS
        
        if section not in DAUM_SECTIONS:
            raise ValueError(
                f"지원하지 않는 섹션: {section}. "
                f"지원: {', '.join(DAUM_SECTIONS.keys())}"
            )
        
        crawler = get_news_crawler("daum")
        return crawler.fetch_section(section, max_results=max_results)
    
    @staticmethod
    def crawl_and_save(sources: List[str], max_results: int = 100,
                       queries: Optional[List[str]] = None,
                       daum_sections: Optional[List[str]] = None) -> Dict:
        if not isinstance(sources, list) or len(sources) == 0:
            raise ValueError("sources는 비어있지 않은 배열이어야 합니다.")
        
        if queries is None:
            queries = ["주식", "코스피", "코스닥", "증시", "주가", "증권", "투자"]
        
        lstm_data_dir = _get_news_data_dir()
        lstm_data_dir.mkdir(parents=True, exist_ok=True)
        project_root = Path(__file__).parent.parent.parent
        
        all_results = []
        total_count = 0
        
        for source in sources:
            if source not in ["naver", "daum"]:
                continue
            
            source_results = []
            
            if source == "daum" and daum_sections:
                crawler = get_news_crawler(source)
                for section in daum_sections:
                    try:
                        result = crawler.fetch_section(section, max_results=max_results)
                        if result.get("items"):
                            for it in result["items"]:
                                it["keyword"] = it.get("keyword") or section
                            source_results.extend(result["items"])
                            total_count += len(result["items"])
                    except Exception:
                        continue
            else:
                crawler = get_news_crawler(source)
                for query in queries:
                    try:
                        result = crawler.fetch(query, max_results=max_results)
                        if result.get("items"):
                            for it in result["items"]:
                                it["keyword"] = it.get("keyword") or query
                            source_results.extend(result["items"])
                            total_count += len(result["items"])
                    except Exception:
                        continue
            
            if source_results:
                all_results.append({
                    "source": source,
                    "count": len(source_results),
                    "items": source_results
                })
        
        new_items: List[Dict] = []
        for r in all_results:
            for item in r["items"]:
                new_items.append({
                    "title": item.get("title", ""),
                    "link": item.get("link", ""),
                    "description": item.get("description", ""),
                    "pubDate": item.get("pubDate", ""),
                    "keyword": item.get("keyword", ""),
                })

        merged_path = lstm_data_dir / MERGED_FILENAME
        existing_items: List[Dict] = []
        if merged_path.exists():
            try:
                existing_items = _read_csv_items(merged_path)
            except Exception:
                existing_items = []

        merged_items = _dedupe_by_link(existing_items + new_items)
        added_count = len(merged_items) - len(existing_items)

        with open(merged_path, "w", encoding="utf-8", newline="") as f:
            writer = csv.DictWriter(f, fieldnames=FIELDNAMES)
            writer.writeheader()
            writer.writerows(merged_items)

        return {
            "success": True,
            "message": "크롤링이 완료되었습니다. 새로 추가된 뉴스는 중복 제거 후 저장됩니다.",
            "total": len(merged_items),
            "added": added_count,
            "sources": [r["source"] for r in all_results],
            "sourceCounts": {r["source"]: r["count"] for r in all_results},
            "savedPath": str(merged_path.relative_to(project_root)),
            "filename": MERGED_FILENAME,
        }

    @staticmethod
    def crawl_api_resume(sources: Optional[List[str]] = None, reset: bool = False) -> Dict:
        if sources is None:
            sources = ["daum", "naver"]
        total_saved = 0
        added_total = 0
        results = []
        skipped = []
        rate_limited = False
        errors = []
        for source in sources:
            if source not in ["naver", "daum"]:
                continue
            r = run_crawl_api(source=source, workers=6, reset=reset, max_pages=0)
            if r.get("error"):
                err_msg = r["error"]
                errors.append(f"{source}: {err_msg}")
                skipped.append({"source": source, "reason": err_msg})
                continue
            total_saved = r.get("total_saved", 0)
            added_total += r.get("added_this_run", 0)
            rate_limited = rate_limited or r.get("rate_limited", False)
            results.append({"source": source, **r})
        if not results and errors:
            return {"error": "; ".join(errors), "total_saved": 0, "added_this_run": 0}
        items = NewsService.read_saved_news()
        source_results = [
            {"source": r["source"], "added": r.get("added_this_run", 0), "total": r.get("total_saved", 0), "rate_limited": r.get("rate_limited", False)}
            for r in results
        ]
        for s in skipped:
            source_results.append({"source": s["source"], "skipped": True, "reason": s["reason"]})
        rate_limited_sources = [r["source"] for r in results if r.get("rate_limited")]
        continued_sources = [r["source"] for r in results if not r.get("rate_limited")]
        msg = results[-1].get("message", "") if results else "완료"
        if rate_limited_sources and continued_sources:
            msg += f" [호출 제한: {', '.join(rate_limited_sources)} → {', '.join(continued_sources)} 계속 실행]"
        return {
            "success": True,
            "total": len(items),
            "added": added_total,
            "rate_limited": rate_limited,
            "message": msg,
            "sources": [r["source"] for r in results],
            "source_results": source_results,
            "skipped": skipped,
        }

    @staticmethod
    def list_saved_files() -> List[Dict]:
        data_dir = _get_news_data_dir()
        if not data_dir.exists():
            return []
        files = []
        merged_path = data_dir / MERGED_FILENAME
        if merged_path.exists():
            try:
                count = len(_read_csv_items(merged_path))
            except Exception:
                count = 0
            files.append({
                "filename": MERGED_FILENAME,
                "savedAt": datetime.fromtimestamp(merged_path.stat().st_mtime).isoformat(),
                "count": count,
            })
        for p in sorted(data_dir.glob("news_*.csv"), key=lambda x: x.stat().st_mtime, reverse=True):
            if p.name == MERGED_FILENAME:
                continue
            try:
                with open(p, "r", encoding="utf-8") as f:
                    count = sum(1 for _ in csv.DictReader(f))
            except Exception:
                count = 0
            files.append({
                "filename": p.name,
                "savedAt": datetime.fromtimestamp(p.stat().st_mtime).isoformat(),
                "count": count,
            })
        return files

    @staticmethod
    def _get_news_filepath(filename: Optional[str] = None) -> Optional[Path]:
        data_dir = _get_news_data_dir()
        if not data_dir.exists():
            return None
        if filename:
            filepath = data_dir / filename
            return filepath if filepath.is_file() else None
        merged_path = data_dir / MERGED_FILENAME
        if merged_path.exists():
            return merged_path
        csv_files = sorted(
            data_dir.glob("news_*.csv"),
            key=lambda x: x.stat().st_mtime,
            reverse=True,
        )
        return csv_files[0] if csv_files else None

    @staticmethod
    def read_saved_news(filename: Optional[str] = None) -> List[Dict]:
        filepath = NewsService._get_news_filepath(filename)
        return _read_csv_items(filepath) if filepath else []

    @staticmethod
    def read_saved_news_paginated(
        page: int = 1,
        limit: int = 50,
        q: Optional[str] = None,
        filename: Optional[str] = None,
    ) -> Dict:
        filepath = NewsService._get_news_filepath(filename)
        if not filepath:
            return {"items": [], "total": 0, "page": page, "limit": limit, "hasMore": False}

        q_lower = (q or "").strip().lower()
        start_idx = (page - 1) * limit
        end_idx = start_idx + limit
        items: List[Dict] = []
        total = 0

        with open(filepath, "r", encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                it = {
                    "title": row.get("title", ""),
                    "link": row.get("link", ""),
                    "description": row.get("description", ""),
                    "pubDate": row.get("pubDate", ""),
                    "keyword": row.get("keyword", ""),
                }
                if q_lower and (
                    q_lower not in (it.get("title") or "").lower()
                    and q_lower not in (it.get("description") or "").lower()
                ):
                    continue
                total += 1
                if start_idx <= total - 1 < end_idx:
                    items.append(it)

        return {
            "items": items,
            "total": total,
            "page": page,
            "limit": limit,
            "hasMore": total > end_idx,
        }
