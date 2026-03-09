"use client";

import { useState } from "react";

type ExportStatus = "idle" | "loading" | "success" | "error";
type FSDiv = "CFS" | "OFS";

type ExportResponse = {
  success?: boolean;
  output_path?: string;
  market_date?: string;
  requested_min_market_cap?: number;
  fs_div?: string;
  all_target_companies?: number;
  selected_target_companies?: number;
  processed_companies?: number;
  missing_corp_code?: number;
  annual_matched_companies?: number;
  quarterly_matched_companies?: number;
  no_data_companies?: number;
  written_rows?: number;
  rate_limited?: boolean;
  warnings_count?: number;
  warnings?: Array<Record<string, string>>;
  error?: string;
};

const MIN_MARKET_CAP = 1_000_000_000_000;

function toNumberString(value: string, fallback: number): string {
  const n = Number.parseInt(value.replace(/[^0-9]/g, ""), 10);
  if (!Number.isFinite(n) || n <= 0) {
    return String(fallback);
  }
  return String(n);
}

export default function DartCollectPage() {
  const [status, setStatus] = useState<ExportStatus>("idle");
  const [message, setMessage] = useState<string>("");
  const [result, setResult] = useState<ExportResponse | null>(null);

  const [fsDiv, setFSDiv] = useState<FSDiv>("CFS");
  const [asOfDate, setAsOfDate] = useState<string>("");
  const [maxCompaniesInput, setMaxCompaniesInput] = useState<string>("");
  const [delayInput, setDelayInput] = useState<string>("0.15");
  const [outputPath, setOutputPath] = useState<string>("");

  const handleExport = async () => {
    setStatus("loading");
    setMessage("DART 재무제표 CSV 생성 중...");
    setResult(null);

    const maxCompanies = maxCompaniesInput.trim() === "" ? 0 : Number.parseInt(toNumberString(maxCompaniesInput, 0), 10);
    const delay = Number.parseFloat(delayInput);

    try {
      const res = await fetch("/api/dart/export/financials", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          minMarketCap: MIN_MARKET_CAP,
          fsDiv,
          asOfDate: asOfDate.trim(),
          maxCompanies,
          delay: Number.isFinite(delay) && delay >= 0 ? delay : 0.15,
          outputPath: outputPath.trim(),
        }),
      });

      const data = (await res.json().catch(() => ({}))) as ExportResponse;
      if (!res.ok) {
        throw new Error(data.error || "DART 재무제표 CSV 생성 실패");
      }

      setStatus("success");
      setResult(data);
      setMessage("CSV 생성이 완료되었습니다.");
    } catch (error) {
      setStatus("error");
      setMessage(error instanceof Error ? error.message : "CSV 생성 중 오류가 발생했습니다.");
    }
  };

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 space-y-5">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 mb-2">DART 재무제표 CSV Export</h1>
          <p className="text-gray-600">
            국내 코스피/코스닥 상장사 중 시가총액 1조원 이상 기업만 대상으로 최근 연간+분기 재무제표를 하나의 CSV로 저장합니다.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div>
            <label className="block text-sm text-gray-600 mb-1">최소 시가총액(원, 1조 고정)</label>
            <input
              type="text"
              value={String(MIN_MARKET_CAP)}
              readOnly
              className="w-full px-3 py-2 border border-gray-300 rounded-lg bg-gray-50 text-gray-600"
            />
          </div>

          <div>
            <label className="block text-sm text-gray-600 mb-1">재무제표 구분</label>
            <select
              value={fsDiv}
              onChange={(e) => setFSDiv(e.target.value as FSDiv)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
            >
              <option value="CFS">CFS (연결)</option>
              <option value="OFS">OFS (별도)</option>
            </select>
          </div>

          <div>
            <label className="block text-sm text-gray-600 mb-1">기준일(YYYYMMDD, 선택)</label>
            <input
              type="text"
              value={asOfDate}
              onChange={(e) => setAsOfDate(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
              placeholder="예: 20260305"
            />
          </div>

          <div>
            <label className="block text-sm text-gray-600 mb-1">기업 수 제한(선택)</label>
            <input
              type="text"
              value={maxCompaniesInput}
              onChange={(e) => setMaxCompaniesInput(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
              placeholder="빈 값이면 전체"
            />
          </div>

          <div>
            <label className="block text-sm text-gray-600 mb-1">호출 간 지연(초)</label>
            <input
              type="text"
              value={delayInput}
              onChange={(e) => setDelayInput(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
            />
          </div>

          <div>
            <label className="block text-sm text-gray-600 mb-1">출력 파일 경로(선택)</label>
            <input
              type="text"
              value={outputPath}
              onChange={(e) => setOutputPath(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
              placeholder="미입력 시 기본 경로 사용"
            />
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={handleExport}
            disabled={status === "loading"}
            className="px-4 py-2 rounded-lg bg-blue-600 text-white font-medium hover:bg-blue-700 disabled:opacity-60"
          >
            {status === "loading" ? "생성 중..." : "CSV 생성"}
          </button>
          <span className="text-sm text-gray-500">기존 DART 수집 API는 제거되었습니다.</span>
        </div>
      </div>

      {message && (
        <div
          className={`rounded-lg border px-4 py-3 text-sm ${
            status === "error"
              ? "border-red-200 bg-red-50 text-red-700"
              : status === "success"
                ? "border-green-200 bg-green-50 text-green-700"
                : "border-blue-200 bg-blue-50 text-blue-700"
          }`}
        >
          {message}
        </div>
      )}

      {result && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 space-y-3 text-sm">
          <h2 className="text-lg font-semibold text-gray-900">실행 결과</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-gray-700">
            <div>출력 파일: {result.output_path || "-"}</div>
            <div>시장 데이터 기준일: {result.market_date || "-"}</div>
            <div>대상 기업(전체): {result.all_target_companies ?? 0}</div>
            <div>실행 기업(선택): {result.selected_target_companies ?? 0}</div>
            <div>처리 기업: {result.processed_companies ?? 0}</div>
            <div>corp_code 누락: {result.missing_corp_code ?? 0}</div>
            <div>연간 매칭 기업: {result.annual_matched_companies ?? 0}</div>
            <div>분기 매칭 기업: {result.quarterly_matched_companies ?? 0}</div>
            <div>무데이터 기업: {result.no_data_companies ?? 0}</div>
            <div>저장 행 수: {result.written_rows ?? 0}</div>
            <div>호출 제한 도달: {result.rate_limited ? "예" : "아니오"}</div>
            <div>경고 건수: {result.warnings_count ?? 0}</div>
          </div>
        </div>
      )}
    </div>
  );
}
