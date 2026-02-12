"use client";

import { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";

const API_BASE = "http://localhost:5001/api/news";
const PAGE_SIZE = 50;

type CrawlStatus = "idle" | "loading" | "success" | "error";

type NewsItem = {
  title: string;
  link: string;
  description: string;
  pubDate: string;
  keyword?: string;
};

type NewsItemsPageResponse = {
  items: NewsItem[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
};

export default function NewsPage() {
  const [status, setStatus] = useState<CrawlStatus>("idle");
  const [message, setMessage] = useState<string>("");
  const [progress, setProgress] = useState<string>("");
  const [filter, setFilter] = useState("");
  const [filterInput, setFilterInput] = useState("");
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();

  const {
    data,
    isLoading: isLoadingList,
    error: listError,
    refetch: refetchItems,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery<NewsItemsPageResponse>({
    queryKey: ["news-items", filter],
    queryFn: async ({ pageParam = 1 }) => {
      const params = new URLSearchParams({
        page: String(pageParam),
        limit: String(PAGE_SIZE),
      });
      if (filter) params.set("q", filter);
      const res = await fetch(`${API_BASE}/items?${params}`);
      if (!res.ok) throw new Error("뉴스 목록을 불러오지 못했습니다.");
      return res.json();
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) =>
      lastPage.hasMore ? lastPage.page + 1 : undefined,
  });

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const total = data?.pages[0]?.total ?? 0;

  const handleApplyFilter = () => {
    setFilter(filterInput.trim());
  };

  const handleCrawl = async (reset = false) => {
    setStatus("loading");
    setMessage("");
    setProgress(
      reset
        ? "API로 뉴스를 가져오는 중... (progress 초기화 후 처음부터)"
        : "API로 뉴스를 가져오는 중... (끊겼던 부분부터 이어서)"
    );

    try {
      const response = await fetch(`${API_BASE}/crawl/resume`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sources: ["daum", "naver"],
          reset,
        }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || "뉴스 수집 실패");
      }

      const result = await response.json();
      setStatus("success");
      const added = result.added ?? 0;
      const totalRes = result.total ?? 0;
      const rateLimited = result.rate_limited ?? false;
      const sourceResults = result.source_results ?? [];
      const skipped = result.skipped ?? [];
      const ranSources = sourceResults
        .filter((s: { skipped?: boolean }) => !s.skipped)
        .map((s: { source: string }) => s.source);
      const skippedInfo =
        skipped.length > 0
          ? skipped
              .map((s: { source: string; reason: string }) => `${s.source}: ${s.reason}`)
              .join("; ")
          : "";
      const baseMsg = rateLimited
        ? `이번에 ${added}건 추가, 총 ${totalRes}건 저장. (호출 제한 도달 - 다음에 이어서 진행 가능)`
        : `수집 완료! 이번에 ${added}건 추가되어, 총 ${totalRes}건이 저장되었습니다.`;
      setMessage(
        ranSources.length > 0 && skipped.length > 0
          ? `${baseMsg} [실행: ${ranSources.join(", ")} | 건너뜀: ${skippedInfo}]`
          : ranSources.length > 0
            ? `${baseMsg} [실행: ${ranSources.join(", ")}]`
            : baseMsg
      );
      setProgress("");
      queryClient.invalidateQueries({ queryKey: ["news-items"] });
      refetchItems();
    } catch (error) {
      setStatus("error");
      setMessage(
        error instanceof Error ? error.message : "뉴스 수집 중 오류가 발생했습니다."
      );
      setProgress("");
    }
  };

  useEffect(() => {
    const el = loadMoreRef.current;
    if (!el || !hasNextPage || isFetchingNextPage) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) fetchNextPage();
      },
      { threshold: 0.1 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, items.length]);

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h1 className="text-2xl font-bold text-gray-900 mb-4">뉴스 수집</h1>
        <p className="text-gray-600 mb-6">
          네이버·다음 API로 뉴스를 수집합니다. <strong>불러오기</strong>는
          lstm/data/news/crawl_list_progress.json을 참고해 끊겼던 키워드부터 이어서 가져옵니다.
          <strong>처음부터</strong>는 progress를 초기화한 뒤 처음부터 수집합니다.
        </p>

        <div className="space-y-4">
          <div className="flex gap-2">
          <button
            type="button"
            onClick={() => handleCrawl(false)}
            disabled={status === "loading"}
            className={`px-6 py-3 text-base font-medium rounded-lg transition-colors ${
              status === "loading"
                ? "bg-gray-400 text-white cursor-not-allowed"
                : "bg-blue-600 text-white hover:bg-blue-700"
            }`}
          >
            {status === "loading" ? "수집 중..." : "불러오기"}
          </button>
          <button
            type="button"
            onClick={() => handleCrawl(true)}
            disabled={status === "loading"}
            className="px-4 py-3 text-base font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            title="progress 파일 초기화 후 처음부터 수집"
          >
            처음부터
          </button>
          </div>

          {progress && <div className="text-sm text-gray-600">{progress}</div>}

          {status === "success" && message && (
            <div className="p-4 bg-green-50 border border-green-200 rounded-lg">
              <p className="text-green-800">{message}</p>
            </div>
          )}

          {status === "error" && message && (
            <div className="p-4 bg-red-50 border border-red-200 rounded-lg">
              <p className="text-red-800">{message}</p>
            </div>
          )}
        </div>
      </div>

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-4">
          <h2 className="text-lg font-semibold text-gray-900">
            저장된 뉴스 목록 {total > 0 && `(${total}건)`}
          </h2>
          <div className="flex gap-2">
            <input
              type="text"
              placeholder="제목·내용 검색..."
              value={filterInput}
              onChange={(e) => setFilterInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleApplyFilter()}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 w-48 sm:w-56"
            />
            <button
              type="button"
              onClick={handleApplyFilter}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-200"
            >
              검색
            </button>
            {filter && (
              <button
                type="button"
                onClick={() => {
                  setFilter("");
                  setFilterInput("");
                }}
                className="px-4 py-2 text-gray-500 text-sm hover:text-gray-700"
              >
                초기화
              </button>
            )}
          </div>
        </div>

        {isLoadingList && (
          <p className="text-gray-500 py-4">목록을 불러오는 중...</p>
        )}
        {listError && (
          <p className="text-red-600 py-4">
            목록을 불러오지 못했습니다. 크롤링 후 불러오기를 실행해 주세요.
          </p>
        )}
        {!isLoadingList && !listError && items.length === 0 && (
          <p className="text-gray-500 py-4">
            {filter
              ? "검색 결과가 없습니다."
              : "저장된 뉴스가 없습니다. 위에서 &quot;불러오기&quot;를 눌러 크롤링해 주세요."}
          </p>
        )}
        {!isLoadingList && items.length > 0 && (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-600 uppercase">
                    제목
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-gray-600 uppercase w-36">
                    날짜
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {items.map((row, idx) => (
                  <tr key={`${row.link}-${idx}`} className="hover:bg-gray-50">
                    <td className="px-4 py-3 text-sm">
                      <a
                        href={row.link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 hover:underline line-clamp-2"
                      >
                        {row.title || "-"}
                      </a>
                      {row.description && (
                        <p className="text-gray-500 text-xs mt-1 line-clamp-1">
                          {row.description}
                        </p>
                      )}
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-500">
                      {row.pubDate || "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div ref={loadMoreRef} className="h-10 flex items-center justify-center py-4">
              {isFetchingNextPage && (
                <div className="text-gray-500 text-sm">더 불러오는 중...</div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
