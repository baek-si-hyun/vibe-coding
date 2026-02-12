"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useInfiniteQuery, useQueryClient } from "@tanstack/react-query";

type TelegramSavedItem = {
  title: string;
  link: string;
  description: string;
  pubDate: string;
  keyword?: string;
};

type ItemsPageResponse = {
  items: TelegramSavedItem[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
};

type VerifyResponse = {
  success: boolean;
  sessionString?: string;
  message?: string;
  error?: string;
};

type SendCodeResponse = {
  success: boolean;
  phoneCodeHash?: string;
  message?: string;
  error?: string;
};

const API_BASE = "http://localhost:5001/api/telegram";
const PAGE_SIZE = 50;

export default function TelegramPage() {
  const queryClient = useQueryClient();
  const [showAuth, setShowAuth] = useState(false);
  const [authCode, setAuthCode] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [authStatus, setAuthStatus] = useState<"idle" | "loading" | "success" | "error">("idle");
  const [authMessage, setAuthMessage] = useState<string | null>(null);
  const [codeRequested, setCodeRequested] = useState(false);
  const [requestingCode, setRequestingCode] = useState(false);
  const [phoneCodeHash, setPhoneCodeHash] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshMessage, setRefreshMessage] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [filterInput, setFilterInput] = useState("");
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const { data: chatsData } = useQuery({
    queryKey: ["telegram-auth-check"],
    queryFn: async () => {
      const res = await fetch("/api/telegram/chats?limit=1");
      return res.json();
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  const needsAuth =
    chatsData?.needsAuth === true ||
    (chatsData?.error && String(chatsData.error).includes("인증"));

  useEffect(() => {
    if (needsAuth) setShowAuth(true);
  }, [needsAuth]);

  const {
    data: itemsData,
    isLoading: isLoadingItems,
    error: itemsError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery<ItemsPageResponse>({
    queryKey: ["telegram-items", filter],
    queryFn: async ({ pageParam = 1 }) => {
      const params = new URLSearchParams({
        page: String(pageParam),
        limit: String(PAGE_SIZE),
      });
      if (filter) params.set("q", filter);
      const res = await fetch(`${API_BASE}/items?${params}`);
      if (!res.ok) throw new Error("저장된 데이터를 불러오지 못했습니다.");
      return res.json();
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) =>
      lastPage.hasMore ? lastPage.page + 1 : undefined,
  });

  const items = itemsData?.pages.flatMap((p) => p.items) ?? [];
  const total = itemsData?.pages[0]?.total ?? 0;

  useEffect(() => {
    const el = loadMoreRef.current;
    if (!el || !hasNextPage || isFetchingNextPage) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          fetchNextPage();
        }
      },
      { rootMargin: "1000px", threshold: 0 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const scrollToBottom = () => {
    scrollContainerRef.current?.scrollTo({
      top: scrollContainerRef.current.scrollHeight,
      behavior: "smooth",
    });
  };

  const handleApplyFilter = () => {
    setFilter(filterInput.trim());
  };

  const handleRequestCode = async () => {
    setRequestingCode(true);
    setAuthMessage(null);
    try {
      const response = await fetch("/api/telegram/send-code", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      const data = (await response.json()) as SendCodeResponse;
      if (data.success && data.phoneCodeHash) {
        setCodeRequested(true);
        setPhoneCodeHash(data.phoneCodeHash);
        setAuthMessage(
          "✅ 인증 코드가 텔레그램 앱으로 전송되었습니다.\n텔레그램 앱에서 받은 코드를 입력하세요.",
        );
        setAuthStatus("idle");
      } else {
        setAuthStatus("error");
        setAuthMessage(data.error || "코드 요청에 실패했습니다.");
      }
    } catch (error) {
      setAuthStatus("error");
      setAuthMessage(
        error instanceof Error ? error.message : "코드 요청 중 오류가 발생했습니다.",
      );
    } finally {
      setRequestingCode(false);
    }
  };

  const handleAuth = async () => {
    if (!authCode.trim()) {
      setAuthMessage("인증 코드를 입력하세요.");
      setAuthStatus("error");
      return;
    }
    setAuthStatus("loading");
    setAuthMessage(null);
    try {
      const response = await fetch("/api/telegram/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          code: authCode.trim(),
          phoneCodeHash: phoneCodeHash || undefined,
          password: authPassword.trim() || undefined,
        }),
      });
      const data = (await response.json()) as VerifyResponse;
      if (data.success && data.sessionString) {
        setAuthStatus("success");
        setAuthMessage(
          `✅ 인증 성공!\n\n.env.local에 TELEGRAM_SESSION_STRING=${data.sessionString} 추가 후 서버를 재시작하세요.`,
        );
        setShowAuth(false);
        setAuthCode("");
        setAuthPassword("");
        setCodeRequested(false);
        queryClient.invalidateQueries({ queryKey: ["telegram-auth-check"] });
      } else {
        setAuthStatus("error");
        setAuthMessage(data.error || "인증에 실패했습니다.");
      }
    } catch (error) {
      setAuthStatus("error");
      setAuthMessage(
        error instanceof Error ? error.message : "인증 중 오류가 발생했습니다.",
      );
    }
  };

  const handleRefresh = async (freshStart = false) => {
    setRefreshing(true);
    setRefreshMessage(null);
    try {
      const saveRes = await fetch("/api/telegram/save-to-csv", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ freshStart }),
      });
      const saveData = await saveRes.json();
      if (!saveData.success) {
        if (saveData.error?.includes("인증")) {
          setShowAuth(true);
        }
        setRefreshMessage(`❌ ${saveData.error || "저장 실패"}`);
        return;
      }
      setRefreshMessage(
        `✅ ${saveData.message}\n(총 ${saveData.total}건, 신규 ${saveData.added}건)`,
      );
      queryClient.invalidateQueries({ queryKey: ["telegram-items", filter] });
    } catch (error) {
      setRefreshMessage(
        error instanceof Error ? error.message : "불러오기 중 오류가 발생했습니다.",
      );
    } finally {
      setRefreshing(false);
    }
  };

  const extractLinks = (text: string) => {
    const pattern = /https?:\/\/[^\s]+/g;
    const matches = text.match(pattern) || [];
    return matches.map((raw) => raw.replace(/[),.;]+$/g, ""));
  };

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <h2 className="text-xl font-semibold text-gray-900">텔레그램 저장 데이터</h2>
          <div className="flex gap-2 items-center">
            <button
              type="button"
              onClick={() => handleRefresh(false)}
              disabled={refreshing || needsAuth}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {refreshing ? "불러오는 중..." : "불러오기"}
            </button>
            <button
              type="button"
              onClick={() => handleRefresh(true)}
              disabled={refreshing || needsAuth}
              className="px-3 py-2 text-sm font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="중단된 진행은 무시하고 처음부터 수집"
            >
              처음부터
            </button>
          </div>
        </div>

        {refreshMessage && (
          <div className="rounded-lg px-4 py-3 text-sm bg-blue-50 text-blue-800 border border-blue-200">
            <pre className="whitespace-pre-wrap font-sans">{refreshMessage}</pre>
          </div>
        )}

        {needsAuth && (
          <div className="rounded-lg px-5 py-6 bg-yellow-50 border border-yellow-200">
            <div className="flex items-start justify-between mb-4">
              <div>
                <h3 className="text-sm font-semibold text-yellow-900 mb-1">
                  텔레그램 인증이 필요합니다
                </h3>
                <p className="text-sm text-yellow-700">
                  불러오기로 새 데이터를 가져오려면 먼저 인증해야 합니다.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowAuth(!showAuth)}
                className="px-4 py-2 text-sm font-medium text-yellow-700 bg-yellow-100 border border-yellow-300 rounded-lg hover:bg-yellow-200 transition-colors"
              >
                {showAuth ? "숨기기" : "인증하기"}
              </button>
            </div>

            {showAuth && (
              <div className="space-y-4 mt-4">
                {!codeRequested ? (
                  <>
                    <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                      <h4 className="text-sm font-semibold text-blue-900 mb-2">인증 절차</h4>
                      <ol className="text-sm text-blue-800 space-y-1 list-decimal list-inside">
                        <li>아래 &quot;인증 코드 요청&quot; 버튼을 클릭하세요</li>
                        <li>텔레그램 앱에서 인증 코드를 확인하세요</li>
                        <li>받은 코드와 비밀번호를 입력하고 인증하세요</li>
                        <li>인증 후 불러오기를 누르면 새 데이터가 저장됩니다</li>
                      </ol>
                    </div>
                    <button
                      type="button"
                      onClick={handleRequestCode}
                      disabled={requestingCode}
                      className="w-full px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:bg-gray-400 transition-colors"
                    >
                      {requestingCode ? "코드 요청 중..." : "인증 코드 요청"}
                    </button>
                  </>
                ) : (
                  <>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        인증 코드
                      </label>
                      <input
                        type="text"
                        value={authCode}
                        onChange={(e) => setAuthCode(e.target.value)}
                        placeholder="텔레그램 앱에서 받은 코드 입력"
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        autoFocus
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        2단계 인증 비밀번호
                      </label>
                      <input
                        type="password"
                        value={authPassword}
                        onChange={(e) => setAuthPassword(e.target.value)}
                        placeholder="2단계 인증 비밀번호 (설정한 경우)"
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      />
                    </div>
                    <button
                      type="button"
                      onClick={handleAuth}
                      disabled={authStatus === "loading"}
                      className="w-full px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:bg-gray-400 transition-colors"
                    >
                      {authStatus === "loading" ? "인증 중..." : "인증하기"}
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setCodeRequested(false);
                        setAuthCode("");
                        setAuthMessage(null);
                        setPhoneCodeHash(null);
                      }}
                      className="w-full px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
                    >
                      코드 다시 요청
                    </button>
                  </>
                )}
                {authMessage && (
                  <div
                    className={`rounded-lg px-4 py-3 text-sm ${
                      authStatus === "success"
                        ? "bg-green-50 text-green-700 border border-green-200"
                        : authStatus === "error"
                          ? "bg-red-50 text-red-700 border border-red-200"
                          : "bg-blue-50 text-blue-700 border border-blue-200"
                    }`}
                  >
                    <pre className="whitespace-pre-wrap font-sans">{authMessage}</pre>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        <div className="flex gap-2 items-center">
          <input
            type="text"
            value={filterInput}
            onChange={(e) => setFilterInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleApplyFilter()}
            placeholder="제목·내용 검색"
            className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
          <button
            type="button"
            onClick={handleApplyFilter}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
          >
            검색
          </button>
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-semibold text-gray-900">저장된 메시지</h3>
          {isLoadingItems ? (
            <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
              불러오는 중...
            </div>
          ) : itemsError ? (
            <div className="rounded-lg px-5 py-6 text-sm text-red-600 text-center bg-red-50">
              {itemsError instanceof Error
                ? itemsError.message
                : "데이터를 불러오지 못했습니다."}
            </div>
          ) : items.length === 0 ? (
            <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
              {total === 0
                ? "저장된 데이터가 없습니다. 인증 후 불러오기를 눌러 데이터를 가져오세요."
                : "검색 결과가 없습니다."}
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              <div className="flex items-center justify-between gap-2 mb-1">
                <span className="text-xs text-gray-500">
                  총 {total}건 중 {items.length}건 표시
                  {hasNextPage && " · 스크롤 시 자동 로드"}
                </span>
                <button
                  type="button"
                  onClick={scrollToBottom}
                  className="shrink-0 px-3 py-1.5 text-xs font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
                >
                  맨 아래로
                </button>
              </div>
              <div
                ref={scrollContainerRef}
                className="flex flex-col gap-3 max-h-[600px] overflow-y-auto"
              >
                {items.map((item, idx) => {
                  const links = extractLinks(item.description);
                  return (
                    <div
                      key={item.link || `${item.pubDate}-${idx}`}
                      className="bg-gray-50 rounded-lg border border-gray-200 p-4"
                    >
                      <div className="flex items-start justify-between mb-3">
                        <div className="text-sm font-medium text-gray-900 line-clamp-2">
                          {item.title}
                        </div>
                        <div className="text-xs text-gray-500 shrink-0 ml-2">
                          {item.pubDate}
                        </div>
                      </div>
                      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
                        <div className="text-sm text-gray-700 whitespace-pre-wrap">
                          {item.description}
                        </div>
                        <div className="rounded-md border border-gray-200 bg-white px-3 py-2">
                          <div className="text-xs font-semibold text-gray-600 mb-2">링크</div>
                          {links.length === 0 ? (
                            item.link ? (
                              <a
                                href={item.link}
                                target="_blank"
                                rel="noreferrer"
                                className="text-xs text-blue-600 hover:text-blue-700 underline break-all"
                              >
                                {item.link}
                              </a>
                            ) : (
                              <div className="text-xs text-gray-400">링크 없음</div>
                            )
                          ) : (
                            <div className="flex flex-col gap-2">
                              {links.map((link) => (
                                <a
                                  key={link}
                                  href={link}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="text-xs text-blue-600 hover:text-blue-700 underline break-all"
                                >
                                  {link}
                                </a>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}
                <div
                  ref={loadMoreRef}
                  className="flex h-12 shrink-0 items-center justify-center text-sm text-gray-500"
                >
                  {isFetchingNextPage && "불러오는 중..."}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
