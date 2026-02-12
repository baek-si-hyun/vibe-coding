"use client";

import { useEffect, useMemo, useState, useRef } from "react";
import { io, Socket } from "socket.io-client";

type ScreenerMode = "volume" | "ma7" | "ma20" | "pattern";

type ScreenerItem = {
  symbol: string;
  price: number;
  candleTime: number;
  volume?: number;
  prevVolume?: number;
  ratio?: number;
  ma?: number;
  deviationPct?: number;
  signals?: {
    spikeRatio: number;
    spikeTime: number;
    resBreakTime: number;
    ma7Time?: number;
    ma20Time?: number;
  };
};

type ScreenerResponse = {
  mode: ScreenerMode;
  asOf: number;
  items: ScreenerItem[];
};

const MODES: Array<{
  id: ScreenerMode;
  label: string;
  detail: string;
  pulse: string;
}> = [
  {
    id: "volume",
    label: "거래량 급증",
    detail: "5분봉 직전 대비 5배 이상",
    pulse: "5분 거래량 급증",
  },
  {
    id: "ma7",
    label: "7봉 지지",
    detail: "현재 5분봉이 7봉선 아래꼬리 터치 후 양봉",
    pulse: "5분 MA7 아래꼬리 반등",
  },
  {
    id: "ma20",
    label: "20봉 지지",
    detail: "현재 5분봉이 20봉선 아래꼬리 터치 후 양봉",
    pulse: "5분 MA20 아래꼬리 반등",
  },
  {
    id: "pattern",
    label: "공통 시그널",
    detail: "최근 4시간 내 거래량 3배 + 저항 돌파 + MA7/20 지지",
    pulse: "거래량 3배 + 저항 돌파 + MA 지지",
  },
];

export default function BithumbMarketScreener() {
  const [mode, setMode] = useState<ScreenerMode>("volume");
  const [data, setData] = useState<ScreenerResponse | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [refreshIndex, setRefreshIndex] = useState(0);
  const socketRef = useRef<Socket | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  const buildBithumbUrl = (symbol: string) =>
    `https://www.bithumb.com/trade/order/${symbol}_KRW`;

  const activeMode = MODES.find((item) => item.id === mode) ?? MODES[0];

  const priceFormatter = useMemo(
    () =>
      new Intl.NumberFormat("ko-KR", {
        maximumFractionDigits: 4,
      }),
    [],
  );
  const volumeFormatter = useMemo(
    () =>
      new Intl.NumberFormat("ko-KR", {
        notation: "compact",
        maximumFractionDigits: 2,
      }),
    [],
  );
  const ratioFormatter = useMemo(
    () =>
      new Intl.NumberFormat("en-US", {
        maximumFractionDigits: 2,
      }),
    [],
  );
  const percentFormatter = useMemo(
    () =>
      new Intl.NumberFormat("ko-KR", {
        maximumFractionDigits: 2,
      }),
    [],
  );
  const timeFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat("ko-KR", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "Asia/Seoul",
      }),
    [],
  );

  useEffect(() => {
    setStatus("loading");
    setErrorMessage(null);

    const socket = io("http://localhost:5001", {
      transports: ["polling"],
    });

    socketRef.current = socket;

    socket.on("connect", () => {
      setIsConnected(true);
      setStatus("idle");
      socket.emit("subscribe_bithumb", { mode });
    });

    socket.on("disconnect", () => {
      setIsConnected(false);
      setStatus("error");
      setErrorMessage("연결이 끊어졌습니다. 재연결 중...");
    });

    socket.on("connect_error", () => {
      setIsConnected(false);
      setStatus("error");
      setErrorMessage("서버에 연결할 수 없습니다.");
    });

    const eventName = `bithumb_${mode}`;
    socket.on(eventName, (payload: ScreenerResponse) => {
      if (payload.mode === mode) {
        setData(payload);
        setStatus("idle");
        setErrorMessage(null);
      }
    });

    socket.emit("subscribe_bithumb", { mode });

    return () => {
      socket.emit("unsubscribe_bithumb", {});
      socket.disconnect();
      socketRef.current = null;
    };
  }, [mode]);

  useEffect(() => {
    if (refreshIndex === 0) return;

    const load = async () => {
      try {
        const response = await fetch(
          `http://localhost:5001/api/bithumb/screener?mode=${mode}`,
        );
        if (!response.ok) {
          throw new Error(`API error: ${response.status} ${response.statusText}`);
        }
        const payload = (await response.json()) as ScreenerResponse;
        setData(payload);
        setStatus("idle");
      } catch (error) {
        setStatus("error");
        setErrorMessage(
          error instanceof Error
            ? error.message
            : "데이터를 불러오지 못했어요. 잠시 후 다시 시도해주세요."
        );
      }
    };
    void load();
  }, [refreshIndex, mode]);

  const isVolume = mode === "volume";
  const isPattern = mode === "pattern";
  const gridClass = isVolume
    ? "sm:grid-cols-[minmax(120px,1.2fr)_minmax(120px,1fr)_minmax(140px,1fr)_minmax(140px,1fr)_minmax(90px,0.7fr)]"
    : isPattern
      ? "sm:grid-cols-[minmax(120px,1.2fr)_minmax(120px,1fr)_minmax(140px,1fr)_minmax(140px,1fr)_minmax(140px,1fr)]"
      : "sm:grid-cols-[minmax(120px,1.2fr)_minmax(120px,1fr)_minmax(120px,1fr)_minmax(120px,0.7fr)]";

  const updatedLabel = data?.asOf ? timeFormatter.format(new Date(data.asOf)) : "";

  const formatAge = (timestamp?: number) => {
    if (!timestamp) {
      return "없음";
    }
    const base = data?.asOf ?? Date.now();
    const minutes = Math.max(0, Math.round((base - timestamp) / 60000));
    if (minutes < 60) {
      return `${minutes}분 전`;
    }
    const hours = Math.round(minutes / 60);
    return `${hours}시간 전`;
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <section className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <div className="flex flex-col gap-6 mb-6">
            <div className="flex flex-wrap gap-3">
              {MODES.map((item) => {
                const isActive = item.id === mode;
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setMode(item.id)}
                    className={`px-5 py-2 text-sm font-semibold rounded-lg transition-colors ${
                      isActive
                        ? "bg-blue-600 text-white"
                        : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                    }`}
                  >
                    {item.label}
                  </button>
                );
              })}
            </div>

            <div className="flex items-center justify-between">
              <div className="flex flex-col gap-1">
                <span className="text-xs text-gray-500">최근 업데이트</span>
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-900">
                    {updatedLabel || "불러오는 중..."}
                  </span>
                  {isConnected ? (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-green-700 bg-green-100 rounded-full">
                      <span className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse"></span>
                      실시간
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-red-700 bg-red-100 rounded-full">
                      <span className="w-1.5 h-1.5 bg-red-500 rounded-full"></span>
                      연결 끊김
                    </span>
                  )}
                </div>
              </div>
              <button
                type="button"
                onClick={() => setRefreshIndex((prev) => prev + 1)}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
              >
                새로고침
              </button>
            </div>
          </div>

          <div className="flex flex-col gap-4">
            <div
              className={`hidden text-xs font-medium text-gray-500 border-b border-gray-200 pb-2 sm:grid ${gridClass}`}
            >
              <span>심볼</span>
              <span>가격</span>
              {isVolume ? (
                <>
                  <span>5분 거래량</span>
                  <span>직전 5분</span>
                  <span>급증</span>
                </>
              ) : isPattern ? (
                <>
                  <span>급증</span>
                  <span>MA 지지</span>
                  <span>저항 돌파</span>
                </>
              ) : (
                <>
                  <span>이동평균</span>
                  <span>괴리율</span>
                </>
              )}
            </div>

            {status === "loading" && (
              <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
                데이터를 불러오는 중입니다...
              </div>
            )}

            {status === "error" && (
              <div className="rounded-lg px-5 py-6 text-sm text-red-600 text-center bg-red-50">
                {errorMessage}
              </div>
            )}

            {status !== "loading" && data?.items?.length === 0 && (
              <div className="rounded-lg px-5 py-6 text-sm text-gray-500 text-center bg-gray-50">
                조건에 맞는 종목이 없습니다.
              </div>
            )}

            {data?.items?.map((item, index) => {
              const href = buildBithumbUrl(item.symbol);
              const ma7Time = item.signals?.ma7Time;
              const ma20Time = item.signals?.ma20Time;
              const maTime =
                ma7Time && ma20Time ? (ma7Time >= ma20Time ? ma7Time : ma20Time) : ma7Time ?? ma20Time;
              const maLabel = maTime ? (maTime === ma7Time ? "MA7" : "MA20") : "없음";
              return (
                <div
                  key={item.symbol}
                  className={`grid grid-cols-1 gap-4 rounded-lg border border-gray-200 bg-white px-5 py-4 text-sm hover:shadow-md transition-shadow sm:gap-0 ${gridClass}`}
                >
                  <div className="flex items-center justify-between sm:block">
                    <span className="text-xs font-medium text-gray-500 sm:hidden">
                      심볼
                    </span>
                    <a
                      href={href}
                      target="_blank"
                      rel="noreferrer"
                      className="group inline-flex items-center gap-2"
                      aria-label={`${item.symbol} 빗썸 거래 페이지 열기`}
                    >
                      <span className="text-base font-semibold text-gray-900">
                        {item.symbol}
                      </span>
                      <span className="rounded px-2 py-0.5 text-xs font-medium text-gray-600 bg-gray-100 group-hover:bg-gray-200 transition-colors">
                        거래
                      </span>
                    </a>
                  </div>
                  <div className="flex items-center justify-between sm:block">
                    <span className="text-xs font-medium text-gray-500 sm:hidden">
                      가격
                    </span>
                    <span className="font-semibold text-gray-900">
                      {priceFormatter.format(item.price)}
                    </span>
                  </div>
                  {isVolume ? (
                    <>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          5분 거래량
                        </span>
                        <span className="text-gray-900">{volumeFormatter.format(item.volume ?? 0)}</span>
                      </div>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          직전 5분
                        </span>
                        <span className="text-gray-900">{volumeFormatter.format(item.prevVolume ?? 0)}</span>
                      </div>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          급증
                        </span>
                        <span className="rounded-full bg-orange-100 px-3 py-1 text-xs font-semibold text-orange-700">
                          x{ratioFormatter.format(item.ratio ?? 0)}
                        </span>
                      </div>
                    </>
                  ) : isPattern ? (
                    <>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          급증
                        </span>
                        <div className="flex flex-col items-start gap-1">
                          <span className="rounded-full bg-orange-100 px-3 py-1 text-xs font-semibold text-orange-700">
                            x{ratioFormatter.format(item.signals?.spikeRatio ?? 0)}
                          </span>
                          <span className="text-xs text-gray-500">
                            {formatAge(item.signals?.spikeTime)}
                          </span>
                        </div>
                      </div>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          MA 지지
                        </span>
                        <div className="flex flex-col items-start gap-1">
                          <span className="rounded-full bg-teal-100 px-3 py-1 text-xs font-semibold text-teal-700">
                            {maLabel}
                          </span>
                          <span className="text-xs text-gray-500">
                            {formatAge(maTime)}
                          </span>
                        </div>
                      </div>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          저항 돌파
                        </span>
                        <div className="flex flex-col items-start gap-1">
                          <span className="rounded-full bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-700">
                            돌파
                          </span>
                          <span className="text-xs text-gray-500">
                            {formatAge(item.signals?.resBreakTime)}
                          </span>
                        </div>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          이동평균
                        </span>
                        <span className="text-gray-900">{priceFormatter.format(item.ma ?? 0)}</span>
                      </div>
                      <div className="flex items-center justify-between sm:block">
                        <span className="text-xs font-medium text-gray-500 sm:hidden">
                          괴리율
                        </span>
                        <span className="rounded-full bg-teal-100 px-3 py-1 text-xs font-semibold text-teal-700">
                          {percentFormatter.format(item.deviationPct ?? 0)}%
                        </span>
                      </div>
                    </>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      </main>
    </div>
  );
}
