"""
뉴스 크롤링 서비스
"""
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
FIELDNAMES = ["title", "link", "description", "pubDate"]


def _read_csv_items(filepath: Path) -> List[Dict]:
    """CSV 파일에서 뉴스 행 목록 읽기."""
    items = []
    with open(filepath, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            items.append({
                "title": row.get("title", ""),
                "link": row.get("link", ""),
                "description": row.get("description", ""),
                "pubDate": row.get("pubDate", ""),
            })
    return items


def _dedupe_by_link(items: List[Dict]) -> List[Dict]:
    """link 기준 중복 제거 (먼저 나온 것 유지)."""
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
    """뉴스 크롤링 서비스 클래스"""
    
    @staticmethod
    def fetch_news(query: str, source: str = "naver", date: Optional[str] = None, 
                   max_results: int = 100, **kwargs) -> Dict:
        """
        뉴스 검색
        
        Args:
            query: 검색어
            source: 소스 (naver 또는 daum)
            date: 날짜 (YYYYMMDD 형식)
            max_results: 최대 결과 수
            **kwargs: 추가 파라미터 (display, start, size, page 등)
        
        Returns:
            검색 결과 딕셔너리
        
        Raises:
            ValueError: 검색어가 없거나 소스가 유효하지 않은 경우
        """
        if not query:
            raise ValueError("검색어(query)가 필요합니다.")
        
        if source not in ["naver", "daum"]:
            raise ValueError(f"지원하지 않는 소스: {source}. 지원: naver, daum")
        
        # 날짜 형식 검증
        if date:
            try:
                datetime.strptime(date, "%Y%m%d")
            except ValueError:
                raise ValueError(f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})")
        
        # 크롤러 인스턴스 생성 및 검색
        crawler = get_news_crawler(source)
        
        if max_results > 100:
            result = crawler.fetch_all_pages(query, date, max_results=max_results)
        else:
            result = crawler.fetch(query, date, max_results=max_results, **kwargs)
        
        return result
    
    @staticmethod
    def fetch_daum_section(section: str, max_results: int = 100) -> Dict:
        """
        다음 뉴스 섹션 페이지 크롤링 (economy, stock, politics 등)
        
        news.daum.net은 리스트 형식이 아니라 카드/블록으로 되어 있어
        fetch_section으로 v.daum.net/v/ 링크를 수집합니다.
        
        Args:
            section: economy, stock, politics, society, policy, industry, finance, estate, coin
            max_results: 최대 수집 개수
        
        Returns:
            섹션 크롤링 결과
        """
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
        """
        뉴스 크롤링 및 저장
        
        Args:
            sources: 크롤링할 소스 목록 (naver, daum)
            max_results: 최대 결과 수
            queries: 검색어 목록 (기본값: 주식 관련 키워드) - naver/daum 검색용
            daum_sections: 다음 섹션 목록 (economy, stock 등) - daum 전용, 리스트 형식 대응
        
        Returns:
            크롤링 결과 딕셔너리
        
        Raises:
            ValueError: sources가 유효하지 않은 경우
        """
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
                # 다음 섹션 페이지 크롤링 (리스트 아닌 카드/블록 형식)
                crawler = get_news_crawler(source)
                for section in daum_sections:
                    try:
                        result = crawler.fetch_section(section, max_results=max_results)
                        if result.get("items"):
                            source_results.extend(result["items"])
                            total_count += len(result["items"])
                    except Exception:
                        continue
            else:
                # 검색 기반 크롤링 (naver, daum 검색)
                crawler = get_news_crawler(source)
                for query in queries:
                    try:
                        result = crawler.fetch(query, max_results=max_results)
                        if result.get("items"):
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
        
        # 이번 크롤링 결과만 평탄화
        new_items: List[Dict] = []
        for r in all_results:
            for item in r["items"]:
                new_items.append({
                    "title": item.get("title", ""),
                    "link": item.get("link", ""),
                    "description": item.get("description", ""),
                    "pubDate": item.get("pubDate", ""),
                })

        # 기존 통합 파일이 있으면 읽어서 합치기
        merged_path = lstm_data_dir / MERGED_FILENAME
        existing_items: List[Dict] = []
        if merged_path.exists():
            try:
                existing_items = _read_csv_items(merged_path)
            except Exception:
                existing_items = []

        # 기존 + 새 뉴스 합친 뒤 link 기준 중복 제거
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
    def crawl_api_resume(sources: Optional[List[str]] = None) -> Dict:
        """
        API 기반 뉴스 수집 (끊겼던 부분부터 이어서)
        네이버/다음 API로 키워드별 수집, 진행 상황 저장, 호출 제한 시 중단 후 재실행 가능
        """
        if sources is None:
            sources = ["daum", "naver"]
        total_saved = 0
        added_total = 0
        results = []
        skipped = []  # {"source": str, "reason": str}
        rate_limited = False
        errors = []
        for source in sources:
            if source not in ["naver", "daum"]:
                continue
            r = run_crawl_api(source=source, workers=6, reset=False, max_pages=0)
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
        # 소스별 실행 결과 (네이버/다음 각각 호출 여부 확인용)
        source_results = [
            {"source": r["source"], "added": r.get("added_this_run", 0), "total": r.get("total_saved", 0), "rate_limited": r.get("rate_limited", False)}
            for r in results
        ]
        for s in skipped:
            source_results.append({"source": s["source"], "skipped": True, "reason": s["reason"]})
        # 호출 제한 시 다음 소스로 이어서 진행했는지 메시지에 표시
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
        """저장된 뉴스 CSV 파일 목록 (통합 파일 우선, 이후 최신순)"""
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
        """뉴스 CSV 파일 경로 반환."""
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
        """저장된 CSV에서 뉴스 목록 읽기. filename 없으면 통합 파일(news_merged.csv)."""
        filepath = NewsService._get_news_filepath(filename)
        return _read_csv_items(filepath) if filepath else []

    @staticmethod
    def read_saved_news_paginated(
        page: int = 1,
        limit: int = 50,
        q: Optional[str] = None,
        filename: Optional[str] = None,
    ) -> Dict:
        """
        페이지네이션 및 필터 지원. 스트리밍으로 해당 페이지만 로드.
        Returns: { items, total, page, limit, hasMore }
        """
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
