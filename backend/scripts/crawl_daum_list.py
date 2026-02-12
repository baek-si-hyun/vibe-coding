#!/usr/bin/env python3
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from crawl_api import run_crawl_api

DEFAULT_WORKERS = 6
CHECKPOINT_EVERY = 100


def main():
    import argparse

    parser = argparse.ArgumentParser(description="뉴스 검색 리스트 크롤링")
    parser.add_argument("--source", choices=["naver", "daum"], default="daum", help="크롤링 소스")
    parser.add_argument("--test", action="store_true", help="테스트: 3개 키워드, 2페이지만")
    parser.add_argument("--keywords", type=int, default=0, help="사용할 키워드 개수 (0=전부)")
    parser.add_argument("--max-pages", type=int, default=0, help="키워드당 최대 페이지 (0=끝까지)")
    parser.add_argument("--workers", type=int, default=DEFAULT_WORKERS, help="병렬 워커 수")
    parser.add_argument("--reset", action="store_true", help="진행 상황 초기화 후 처음부터")
    parser.add_argument("--checkpoint", type=int, default=CHECKPOINT_EVERY, help="체크포인트 간격 (건수)")
    args = parser.parse_args()

    keywords_limit = 3 if args.test else (args.keywords if args.keywords > 0 else 0)
    max_pages = 2 if args.test else args.max_pages

    result = run_crawl_api(
        source=args.source,
        workers=args.workers,
        reset=args.reset,
        max_pages=max_pages,
        keywords_limit=keywords_limit if keywords_limit > 0 else 0,
        checkpoint_every=args.checkpoint,
    )

    if result.get("error"):
        print(f"오류: {result['error']}")
        return
    print(result.get("message", ""))
    if result.get("rate_limited"):
        print("다음 실행 시 이어서 진행됩니다.")


if __name__ == "__main__":
    main()
