"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useInfiniteQuery, useQueryClient } from "@tanstack/react-query";

type TelegramChat = {
  id: number;
  title: string;
  type: string;
  memberCount?: number;
};

type TelegramMessage = {
  id: number;
  message: string;
  date: number;
  senderId?: number;
  senderName?: string;
  chatId: number;
  chatTitle?: string;
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

export default function TelegramPage() {
  const queryClient = useQueryClient();
  const [selectedChatId, setSelectedChatId] = useState<number | null>(null);
  const [showAuth, setShowAuth] = useState(false);
  const [authCode, setAuthCode] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [authStatus, setAuthStatus] = useState<"idle" | "loading" | "success" | "error">("idle");
  const [authMessage, setAuthMessage] = useState<string | null>(null);
  const [codeRequested, setCodeRequested] = useState(false);
  const [requestingCode, setRequestingCode] = useState(false);
  const [phoneCodeHash, setPhoneCodeHash] = useState<string | null>(null);
  const [savingToCsv, setSavingToCsv] = useState(false);
  const [csvSaveMessage, setCsvSaveMessage] = useState<string | null>(null);

  const pinnedChatIds = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_TELEGRAM_PINNED_CHAT_IDS;
    if (!raw) return [];
    return raw
      .split(",")
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isFinite(value));
  }, []);

  const hasPinnedChats = pinnedChatIds.length > 0;
  
  // 채팅 목록을 React Query로 관리
  const {
    data: chatsData,
    isLoading: isLoadingChats,
    error: chatsError,
    refetch: refetchChats,
  } = useQuery<{ chats: TelegramChat[]; needsAuth?: boolean; error?: string }>({
    queryKey: ["telegram-chats", pinnedChatIds.join(",")],
    queryFn: async () => {
      const idsQuery = hasPinnedChats
        ? `&ids=${pinnedChatIds.join(",")}`
        : "";
      const response = await fetch(`/api/telegram/chats?limit=100${idsQuery}`);
      const data = await response.json();
      
      if (!response.ok) {
        throw new Error(data.error || "채팅 목록을 불러오지 못했습니다.");
      }
      
      return data;
    },
    staleTime: 2 * 60 * 1000, // 2분간 캐시 유지 (채팅 목록은 자주 변하지 않음)
    gcTime: 10 * 60 * 1000, // 10분간 메모리 유지
    refetchOnWindowFocus: false,
    retry: 1,
  });

  const chats = chatsData?.chats || [];
  const needsAuth = chatsData?.needsAuth || chatsError?.message?.includes("인증") || chatsError?.message?.includes("설정");
  
  // 인증이 필요하면 자동으로 인증 UI 표시
  useEffect(() => {
    if (needsAuth) {
      setShowAuth(true);
    }
  }, [needsAuth]);
  
  const visibleChats = useMemo(() => {
    if (!hasPinnedChats) return chats;
    const pinnedSet = new Set(pinnedChatIds);
    return chats.filter((chat) => pinnedSet.has(chat.id));
  }, [chats, hasPinnedChats, pinnedChatIds]);

  useEffect(() => {
    if (isLoadingChats) return;
    if (visibleChats.length === 0) return;
    if (selectedChatId && visibleChats.some((chat) => chat.id === selectedChatId)) {
      return;
    }
    setSelectedChatId(visibleChats[0].id);
  }, [isLoadingChats, visibleChats, selectedChatId]);

  const MESSAGES_PAGE_SIZE = 100;

  // 메시지 목록을 useInfiniteQuery로 관리 (페이지네이션 지원)
  const {
    data: messagesData,
    isLoading: isLoadingMessages,
    error: messagesError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: ["telegram-messages", selectedChatId],
    queryFn: async ({ pageParam }) => {
      if (!selectedChatId) return { messages: [] as TelegramMessage[] };
      const params = new URLSearchParams({
        chatId: String(selectedChatId),
        limit: String(MESSAGES_PAGE_SIZE),
      });
      if (pageParam != null) params.set("offsetId", String(pageParam));
      const response = await fetch(
        `/api/telegram/messages?${params.toString()}`,
      );
      if (!response.ok) {
        throw new Error("메시지를 불러오지 못했습니다.");
      }
      return response.json() as Promise<{ messages: TelegramMessage[] }>;
    },
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (lastPage) => {
      const msgs = lastPage?.messages ?? [];
      if (msgs.length < MESSAGES_PAGE_SIZE) return undefined;
      const oldestId = msgs[msgs.length - 1]?.id;
      return oldestId != null ? oldestId : undefined;
    },
    enabled: !!selectedChatId,
    staleTime: 30 * 1000,
    gcTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    refetchOnMount: false,
  });

  const messages = useMemo(
    () =>
      messagesData?.pages?.flatMap((p) => p.messages ?? []) ?? [],
    [messagesData],
  );

  const loadMoreRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

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

  const handleRequestCode = async () => {
    setRequestingCode(true);
    setAuthMessage(null);

    try {
      const response = await fetch("/api/telegram/send-code", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      });

      const data = (await response.json()) as SendCodeResponse;

      if (data.success && data.phoneCodeHash) {
        setCodeRequested(true);
        setPhoneCodeHash(data.phoneCodeHash);
        setAuthMessage(
          "✅ 인증 코드가 텔레그램 앱으로 전송되었습니다.\n텔레그램 앱을 확인하여 받은 코드를 아래에 입력하세요.",
        );
        setAuthStatus("idle");
      } else {
        setAuthStatus("error");
        setAuthMessage(data.error || "코드 요청에 실패했습니다.");
      }
    } catch (error) {
      setAuthStatus("error");
      setAuthMessage(
        error instanceof Error
          ? error.message
          : "코드 요청 중 오류가 발생했습니다.",
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
      const requestBody = {
        code: authCode.trim(),
        phoneCodeHash: phoneCodeHash || undefined,
        password: authPassword.trim() || undefined,
      };
      
      const response = await fetch("/api/telegram/verify", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(requestBody),
      });

      const data = (await response.json()) as VerifyResponse;

      if (data.success && data.sessionString) {
        setAuthStatus("success");
        setAuthMessage(
          `✅ 인증 성공!\n\n.env.local 파일에 다음을 추가하세요:\n\nTELEGRAM_SESSION_STRING=${data.sessionString}\n\n추가 후 개발 서버를 재시작하세요.`,
        );
        setShowAuth(false);
        setAuthCode("");
        setAuthPassword("");
        setCodeRequested(false);
        // 인증 후 채팅 목록 새로고침
        setTimeout(() => {
          queryClient.invalidateQueries({ queryKey: ["telegram-chats"] });
        }, 1000);
      } else {
        setAuthStatus("error");
        setAuthMessage(data.error || "인증에 실패했습니다.");
      }
    } catch (error) {
      setAuthStatus("error");
      setAuthMessage(
        error instanceof Error
          ? error.message
          : "인증 중 오류가 발생했습니다.",
      );
    }
  };

  const handleSaveToCsv = async () => {
    setSavingToCsv(true);
    setCsvSaveMessage(null);
    try {
      const chatIds = visibleChats.length > 0
        ? visibleChats.map((c) => c.id)
        : [];
      const response = await fetch("/api/telegram/save-to-csv", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          chatIds: chatIds.length > 0 ? chatIds : undefined,
          maxMessagesPerChat: 1000,
        }),
      });
      const data = await response.json();
      if (data.success) {
        setCsvSaveMessage(
          `✅ ${data.message} (총 ${data.total}건, 신규 ${data.added}건)\n저장 경로: ${data.savedPath || data.filename}`,
        );
      } else {
        setCsvSaveMessage(`❌ ${data.error || "저장 실패"}`);
      }
    } catch (error) {
      setCsvSaveMessage(
        error instanceof Error ? error.message : "CSV 저장 중 오류가 발생했습니다.",
      );
    } finally {
      setSavingToCsv(false);
    }
  };

  const timeFormatter = new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Seoul",
  });
  
  const extractLinks = (text: string) => {
    const pattern = /https?:\/\/[^\s]+/g;
    const matches = text.match(pattern) || [];
    return matches.map((raw) => raw.replace(/[),.;]+$/g, ""));
  };

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold text-gray-900">텔레그램 채팅</h2>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleSaveToCsv}
              disabled={savingToCsv || needsAuth}
              className="px-4 py-2 text-sm font-medium text-white bg-green-600 rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {savingToCsv ? "저장 중..." : "CSV로 저장"}
            </button>
            <button
              type="button"
              onClick={() => {
                queryClient.invalidateQueries({ queryKey: ["telegram-chats"] });
                if (selectedChatId) {
                  queryClient.invalidateQueries({ queryKey: ["telegram-messages", selectedChatId] });
                }
              }}
              disabled={isLoadingChats}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoadingChats ? "새로고침 중..." : "새로고침"}
            </button>
          </div>
        </div>

        {csvSaveMessage && (
          <div className="rounded-lg px-4 py-3 text-sm bg-blue-50 text-blue-800 border border-blue-200">
            <pre className="whitespace-pre-wrap font-sans">{csvSaveMessage}</pre>
          </div>
        )}

        {hasPinnedChats && visibleChats.length === 0 && !isLoadingChats && (
          <div className="rounded-lg px-5 py-4 text-sm text-yellow-700 bg-yellow-50 border border-yellow-200">
            설정한 채팅 ID가 현재 목록에 없습니다.{" "}
            <span className="font-medium">
              NEXT_PUBLIC_TELEGRAM_PINNED_CHAT_IDS
            </span>{" "}
            값을 확인하세요.
          </div>
        )}

        {visibleChats.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {visibleChats.map((chat) => (
              <button
                key={`tab-${chat.id}`}
                type="button"
                onClick={() => setSelectedChatId(chat.id)}
                className={`px-3 py-2 text-sm font-medium rounded-full border transition-colors ${
                  selectedChatId === chat.id
                    ? "bg-blue-600 text-white border-blue-600"
                    : "bg-white text-gray-700 border-gray-300 hover:bg-gray-50"
                }`}
                title={`채팅 ID: ${chat.id}`}
              >
                {chat.title}
              </button>
            ))}
          </div>
        )}

        {/* 인증 필요 메시지 */}
        {needsAuth && (
          <div className="rounded-lg px-5 py-6 bg-yellow-50 border border-yellow-200">
            <div className="flex items-start justify-between mb-4">
              <div>
                <h3 className="text-sm font-semibold text-yellow-900 mb-1">
                  텔레그램 인증이 필요합니다
                </h3>
                <p className="text-sm text-yellow-700">
                  텔레그램 채팅을 보려면 먼저 인증해야 합니다.
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
                      <h4 className="text-sm font-semibold text-blue-900 mb-2">
                        인증 절차
                      </h4>
                      <ol className="text-sm text-blue-800 space-y-1 list-decimal list-inside">
                        <li>아래 &quot;인증 코드 요청&quot; 버튼을 클릭하세요</li>
                        <li>텔레그램 앱에서 인증 코드를 확인하세요</li>
                        <li>받은 코드를 입력하고 인증하세요</li>
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
                      <p className="mt-1 text-xs text-gray-500">
                        텔레그램 앱을 확인하여 받은 코드를 입력하세요
                      </p>
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        2단계 인증 비밀번호
                      </label>
                      <input
                        type="password"
                        value={authPassword}
                        onChange={(e) => setAuthPassword(e.target.value)}
                        placeholder="텔레그램 계정의 2단계 인증 비밀번호 입력"
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      />
                      <p className="mt-1 text-xs text-gray-500">
                        텔레그램 계정에 2단계 인증을 설정한 경우 비밀번호를 입력하세요
                      </p>
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

        {chatsError && !needsAuth && (
          <div className="rounded-lg px-5 py-6 text-sm text-red-600 text-center bg-red-50">
            {chatsError instanceof Error
              ? chatsError.message
              : "채팅 목록을 불러오지 못했습니다."}
          </div>
        )}
        
        {isLoadingChats && (
          <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
            채팅 목록을 불러오는 중...
          </div>
        )}

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-semibold text-gray-900">메시지</h3>
          {!selectedChatId ? (
            <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
              채팅을 선택하세요.
            </div>
          ) : isLoadingMessages ? (
            <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
              불러오는 중...
            </div>
          ) : messagesError ? (
            <div className="rounded-lg px-5 py-6 text-sm text-red-600 text-center bg-red-50">
              {messagesError instanceof Error
                ? messagesError.message
                : "메시지를 불러오지 못했습니다."}
            </div>
          ) : messages.length === 0 ? (
            <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
              메시지가 없습니다.
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              <div className="flex items-center justify-between gap-2 mb-1">
                <span className="text-xs text-gray-500">
                  {messages.length}건 표시
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
              {messages.map((message) => {
                const links = extractLinks(message.message);
                return (
                  <div
                    key={message.id}
                    className="bg-gray-50 rounded-lg border border-gray-200 p-4"
                  >
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex flex-col gap-1">
                        {message.chatTitle && (
                          <div className="text-xs font-medium text-blue-600">
                            {message.chatTitle}
                          </div>
                        )}
                        {message.senderName && (
                          <div className="font-medium text-gray-900">
                            {message.senderName}
                          </div>
                        )}
                      </div>
                      <div className="text-xs text-gray-500">
                        {timeFormatter.format(new Date(message.date))}
                      </div>
                    </div>
                    <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
                      <div className="text-sm text-gray-700 whitespace-pre-wrap">
                        {message.message}
                      </div>
                      <div className="rounded-md border border-gray-200 bg-white px-3 py-2">
                        <div className="text-xs font-semibold text-gray-600 mb-2">
                          링크
                        </div>
                        {links.length === 0 ? (
                          <div className="text-xs text-gray-400">링크 없음</div>
                        ) : (
                          <div className="flex flex-col gap-2">
                            {links.map((link) => (
                              <a
                                key={`${message.id}-${link}`}
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
