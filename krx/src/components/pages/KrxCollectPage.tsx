"use client";

import { useState } from "react";

type CollectStatus = "idle" | "loading" | "success" | "error";

export default function KrxCollectPage() {
  const [status, setStatus] = useState<CollectStatus>("idle");
  const [message, setMessage] = useState<string>("");
  const [progress, setProgress] = useState<string>("");

  const handleCollect = async (reset = false) => {
    setStatus("loading");
    setMessage("");
    setProgress(
      reset
        ? "KRX API로 데이터를 가져오는 중... (progress 초기화 후 처음부터)"
        : "KRX API로 데이터를 가져오는 중... (끊겼던 부분부터 이어서)"
    );

    try {
      const response = await fetch("/api/krx/collect/resume", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ delay: 2, maxDates: 0, reset }),
      });

      const result = await response.json().catch(() => ({}));

      if (!response.ok) {
        throw new Error(result.error || "KRX 수집 실패");
      }

      setStatus("success");
      const datesDone = result.dates_done ?? 0;
      const totalDates = result.total_dates ?? 0;
      const rateLimited = result.rate_limited ?? false;
      const baseMsg = rateLimited
        ? `이번에 ${datesDone}일 수집, 누적 ${totalDates}일. (호출 제한 도달 - 다음에 이어서 진행 가능)`
        : `수집 완료! 이번에 ${datesDone}일 추가되어, 총 ${totalDates}일이 저장되었습니다.`;
      setMessage(result.message || baseMsg);
      setProgress("");
    } catch (error) {
      setStatus("error");
      setMessage(
        error instanceof Error ? error.message : "KRX 수집 중 오류가 발생했습니다."
      );
      setProgress("");
    }
  };

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h1 className="text-2xl font-bold text-gray-900 mb-4">KRX 데이터 수집</h1>
        <p className="text-gray-600 mb-6">
          유가증권·코스닥 일별매매정보와 종목기본정보를 KRX API로 수집합니다.
          <strong> 시가총액 1조원 이상</strong> 기업만 저장합니다. 일별매매는 종목명.csv
          한 파일에 날짜순으로 모두 저장됩니다. 호출 제한을 피하기 위해 요청 간 2초 대기 후
          저장합니다. <strong>불러오기</strong>는 lstm/data/krx_collect_progress.json을 참고해
          끊겼던 부분부터 이어서 진행합니다. <strong>처음부터</strong>는 progress를 초기화한 뒤 처음부터 수집합니다.
        </p>

        <div className="space-y-4">
          <div className="flex gap-2">
          <button
            type="button"
            onClick={() => handleCollect(false)}
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
            onClick={() => handleCollect(true)}
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
        <h2 className="text-lg font-semibold text-gray-900 mb-2">수집 대상 API</h2>
        <ul className="list-disc list-inside text-gray-600 space-y-1 text-sm">
          <li>kospi_daily → lstm/data/kospi_daily/종목명.csv (시총 1조 이상)</li>
          <li>kosdaq_daily → lstm/data/kosdaq_daily/종목명.csv (시총 1조 이상)</li>
          <li>kospi_basic → lstm/data/kospi_basic/YYYYMMDD.csv (시총 1조 이상)</li>
          <li>kosdaq_basic → lstm/data/kosdaq_basic/YYYYMMDD.csv (시총 1조 이상)</li>
        </ul>
      </div>
    </div>
  );
}
