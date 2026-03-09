"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

type CollectStatus = "idle" | "loading" | "success" | "error";

type EndpointMap = Record<string, { url?: string; name?: string }>;

type APIOption = {
  id: string;
  name: string;
};

type FileItem = {
  api_id: string;
  api_name: string;
  file_name: string;
  relative_path: string;
  size_bytes: number;
  modified_at: string;
};

type APISummary = {
  api_id: string;
  api_name: string;
  file_count: number;
};

const FILE_LIMIT = 200;
const COLLECT_DELAY_SEC = 0.5;
const MAX_DATES_PER_BATCH = 8;

type KrxCollectPageProps = {
  forcedAPIID?: string;
  hideAPITabs?: boolean;
};

function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
}

function formatDateTime(raw: string): string {
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  return date.toLocaleString("ko-KR", { hour12: false });
}

export default function KrxCollectPage({
  forcedAPIID,
  hideAPITabs = false,
}: KrxCollectPageProps) {
  const [status, setStatus] = useState<CollectStatus>("idle");
  const [message, setMessage] = useState<string>("");
  const [progress, setProgress] = useState<string>("");

  const [apiOptions, setApiOptions] = useState<APIOption[]>([]);
  const [selectedAPI, setSelectedAPI] = useState<string>("");
  const [listLoading, setListLoading] = useState<boolean>(false);
  const [listError, setListError] = useState<string>("");
  const [files, setFiles] = useState<FileItem[]>([]);
  const [totalFiles, setTotalFiles] = useState<number>(0);
  const [apiSummaries, setApiSummaries] = useState<APISummary[]>([]);

  const selectedSummary = useMemo(
    () => apiSummaries.find((item) => item.api_id === selectedAPI),
    [apiSummaries, selectedAPI]
  );
  const selectedAPIOption = useMemo(
    () => apiOptions.find((item) => item.id === selectedAPI),
    [apiOptions, selectedAPI]
  );
  const summaryCountByAPI = useMemo(() => {
    const map = new Map<string, number>();
    for (const summary of apiSummaries) {
      map.set(summary.api_id, summary.file_count);
    }
    return map;
  }, [apiSummaries]);

  const fetchFiles = useCallback(async (apiID: string) => {
    if (!apiID) return;
    setListLoading(true);
    setListError("");
    try {
      const res = await fetch(
        `/api/krx/files?api=${encodeURIComponent(apiID)}&limit=${FILE_LIMIT}`,
        { method: "GET" }
      );
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || "수집 파일 목록 조회 실패");
      }
      setFiles(Array.isArray(data.items) ? data.items : []);
      setTotalFiles(Number.isFinite(data.total) ? Number(data.total) : 0);
      const summaries: APISummary[] = Array.isArray(data.api_summaries) ? data.api_summaries : [];
      setApiSummaries((prev) => {
        const merged = new Map<string, APISummary>();
        for (const item of prev) {
          merged.set(item.api_id, item);
        }
        for (const item of summaries) {
          merged.set(item.api_id, item);
        }
        return Array.from(merged.values());
      });
    } catch (error) {
      setFiles([]);
      setTotalFiles(0);
      setListError(
        error instanceof Error
          ? error.message
          : "수집 파일 목록 조회 중 오류가 발생했습니다."
      );
    } finally {
      setListLoading(false);
    }
  }, []);

  const fetchEndpoints = useCallback(async () => {
    try {
      const res = await fetch("/api/krx/endpoints", { method: "GET" });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || "KRX API 목록 조회 실패");
      }

      const endpoints: EndpointMap =
        data.endpoints && typeof data.endpoints === "object" ? data.endpoints : {};
      const ids: string[] = Array.isArray(data.available_apis)
        ? data.available_apis.filter((id: unknown): id is string => typeof id === "string")
        : Object.keys(endpoints).sort();
      const options: APIOption[] = ids.map((id) => ({
        id,
        name:
          typeof endpoints[id]?.name === "string" && endpoints[id].name
            ? endpoints[id].name
            : id,
      }));
      setApiOptions(options);
      setSelectedAPI((prev) => {
        if (forcedAPIID) {
          const matched = options.find((option) => option.id === forcedAPIID);
          if (matched) return forcedAPIID;
        }
        return prev || options[0]?.id || "";
      });
    } catch (error) {
      setListError(
        error instanceof Error
          ? error.message
          : "KRX API 목록 조회 중 오류가 발생했습니다."
      );
    }
  }, []);

  useEffect(() => {
    void fetchEndpoints();
  }, [fetchEndpoints]);

  useEffect(() => {
    if (!selectedAPI) return;
    void fetchFiles(selectedAPI);
  }, [fetchFiles, selectedAPI]);

  useEffect(() => {
    if (!forcedAPIID || apiOptions.length === 0) return;
    if (!apiOptions.some((option) => option.id === forcedAPIID)) return;
    setSelectedAPI(forcedAPIID);
  }, [forcedAPIID, apiOptions]);

  const handleCollect = async (reset = false) => {
    if (!selectedAPI) {
      setStatus("error");
      setMessage("수집할 API를 먼저 선택해주세요.");
      return;
    }
    setStatus("loading");
    setMessage("");
    const targetName = selectedAPIOption?.name
      ? `${selectedAPIOption.name} (${selectedAPI})`
      : selectedAPI;
    setProgress(
      reset
        ? `${targetName} 데이터를 가져오는 중... (선택 API progress 초기화 후 처음부터)`
        : `${targetName} 데이터를 가져오는 중... (선택 API 끊긴 지점부터 이어서)`
    );

    try {
      let keepRunning = true;
      let nextReset = reset;
      let batch = 0;
      let lastMessage = "";

      while (keepRunning) {
        batch += 1;
        const response = await fetch("/api/krx/collect/resume", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            delay: COLLECT_DELAY_SEC,
            maxDates: MAX_DATES_PER_BATCH,
            reset: nextReset,
            apiIds: [selectedAPI],
          }),
        });
        nextReset = false;

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
          throw new Error(result.error || "KRX 수집 실패");
        }

        const datesDone = Number(result.dates_done ?? 0);
        const totalDates = Number(result.total_dates ?? 0);
        const rateLimited = Boolean(result.rate_limited ?? false);
        const baseMsg = rateLimited
          ? `배치 ${batch}회차: ${datesDone}일 수집, 누적 ${totalDates}일 (호출 제한 도달)`
          : `배치 ${batch}회차: ${datesDone}일 수집, 누적 ${totalDates}일`;
        lastMessage = String(result.message || baseMsg);
        setMessage(lastMessage);

        if (rateLimited || datesDone <= 0) {
          keepRunning = false;
        } else {
          setProgress(`${targetName} 데이터를 이어서 수집 중... (배치 ${batch} 완료)`);
        }
      }

      setStatus("success");
      setMessage(lastMessage || "수집 완료");
      setProgress("");

      if (selectedAPI) {
        void fetchFiles(selectedAPI);
      }
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
          KRX OPEN API 데이터를 <strong>2010-01-03부터 오늘까지</strong> 저장합니다.
          유가증권/코스닥 일별매매정보는 <strong>시가총액 1조원 이상</strong>만 저장하고,
          종목기본정보는 위 조건에 해당하는 종목만 날짜 파일로 저장합니다.{" "}
          <strong>불러오기</strong>는 선택한 API 탭 기준 progress 파일을 사용해 이어서 진행합니다.
        </p>

        <div className="space-y-4">
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => handleCollect(false)}
              disabled={status === "loading" || !selectedAPI}
              className={`px-6 py-3 text-base font-medium rounded-lg transition-colors ${
                status === "loading" || !selectedAPI
                  ? "bg-gray-400 text-white cursor-not-allowed"
                  : "bg-blue-600 text-white hover:bg-blue-700"
              }`}
            >
              {status === "loading" ? "수집 중..." : "선택 API 불러오기"}
            </button>
            <button
              type="button"
              onClick={() => handleCollect(true)}
              disabled={status === "loading" || !selectedAPI}
              className="px-4 py-3 text-base font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="선택 API progress 파일 초기화 후 처음부터 수집"
            >
              선택 API 처음부터
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

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 space-y-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div className="space-y-1">
            <h2 className="text-lg font-semibold text-gray-900">API별 저장 파일 조회</h2>
            <p className="text-sm text-gray-600">
              {hideAPITabs
                ? `현재 선택된 API의 파일을 최신 기준 최대 ${FILE_LIMIT}개까지 보여줍니다.`
                : `API 탭을 선택하면 최신 파일 기준으로 최대 ${FILE_LIMIT}개까지 보여줍니다.`}
            </p>
          </div>
          <button
            type="button"
            onClick={() => {
              if (selectedAPI) void fetchFiles(selectedAPI);
            }}
            disabled={listLoading || !selectedAPI}
            className="px-3 py-2 text-sm font-medium text-gray-700 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 w-fit"
          >
            새로고침
          </button>
        </div>

        {!hideAPITabs && (
          <div className="overflow-x-auto pb-1">
            <div className="flex w-max gap-2">
              {apiOptions.map((api) => {
                const isActive = api.id === selectedAPI;
                const count = summaryCountByAPI.get(api.id);
                return (
                  <button
                    key={api.id}
                    type="button"
                    onClick={() => setSelectedAPI(api.id)}
                    className={`px-3 py-2 rounded-lg border text-left transition-colors min-w-[180px] ${
                      isActive
                        ? "bg-blue-600 text-white border-blue-600"
                        : "bg-white text-gray-700 border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    <div className="text-sm font-medium truncate">{api.name}</div>
                    <div className={`text-xs ${isActive ? "text-blue-100" : "text-gray-500"}`}>
                      {api.id}
                      {typeof count === "number" ? ` · ${count.toLocaleString()} files` : ""}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {selectedSummary && (
          <div className="text-sm text-gray-700">
            선택 API 총 파일 수: <strong>{selectedSummary.file_count.toLocaleString()}</strong>
            {" · "}현재 표시: <strong>{files.length.toLocaleString()}</strong>
            {" · "}조회 결과 전체: <strong>{totalFiles.toLocaleString()}</strong>
          </div>
        )}

        {listError && (
          <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-800">
            {listError}
          </div>
        )}

        <div className="overflow-x-auto border border-gray-200 rounded-lg">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left font-medium text-gray-700">파일명</th>
                <th className="px-4 py-2 text-left font-medium text-gray-700">상대경로</th>
                <th className="px-4 py-2 text-right font-medium text-gray-700">크기</th>
                <th className="px-4 py-2 text-left font-medium text-gray-700">수정시각</th>
              </tr>
            </thead>
            <tbody>
              {listLoading && (
                <tr>
                  <td className="px-4 py-3 text-gray-600" colSpan={4}>
                    목록을 불러오는 중입니다...
                  </td>
                </tr>
              )}
              {!listLoading && files.length === 0 && (
                <tr>
                  <td className="px-4 py-3 text-gray-500" colSpan={4}>
                    표시할 파일이 없습니다.
                  </td>
                </tr>
              )}
              {!listLoading &&
                files.map((item) => (
                  <tr key={`${item.api_id}:${item.relative_path}`} className="border-t border-gray-100">
                    <td className="px-4 py-2 text-gray-900">{item.file_name}</td>
                    <td className="px-4 py-2 text-gray-600">{item.relative_path}</td>
                    <td className="px-4 py-2 text-right text-gray-600">
                      {formatBytes(item.size_bytes)}
                    </td>
                    <td className="px-4 py-2 text-gray-600">{formatDateTime(item.modified_at)}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
