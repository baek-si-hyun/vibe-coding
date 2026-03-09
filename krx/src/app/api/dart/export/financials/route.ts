import { NextRequest, NextResponse } from "next/server";

// DART API는 Go 백엔드(5002)에서만 제공됩니다. BACKEND_GO_URL 미설정 시 5002 사용.
const BACKEND_GO_BASE = process.env.BACKEND_GO_URL || "http://localhost:5002";
const MIN_MARKET_CAP = 1_000_000_000_000;

export async function POST(request: NextRequest) {
  try {
    const body = await request.json().catch(() => ({}));
    const parsedMinMarketCap = Number.parseInt(
      String(body?.minMarketCap ?? body?.min_market_cap ?? "").replace(/[^0-9]/g, ""),
      10
    );
    const normalizedBody = {
      ...(body ?? {}),
      minMarketCap: Number.isFinite(parsedMinMarketCap)
        ? Math.max(parsedMinMarketCap, MIN_MARKET_CAP)
        : MIN_MARKET_CAP,
    };
    let res: Response;
    try {
      res = await fetch(`${BACKEND_GO_BASE}/api/dart/export/financials`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(normalizedBody),
        cache: "no-store",
      });
    } catch (fetchError) {
      const msg =
        fetchError instanceof Error ? fetchError.message : String(fetchError);
      const isConnectionError =
        /fetch failed|ECONNREFUSED|ENOTFOUND|network/i.test(msg) || msg === "The operation was aborted.";
      return NextResponse.json(
        {
          error: isConnectionError
            ? `Go 백엔드에 연결할 수 없습니다 (${BACKEND_GO_BASE}). backend-go 서버를 실행했는지, 포트가 5002인지 확인하세요.`
            : "DART 재무제표 CSV 생성 중 오류가 발생했습니다.",
          message: msg,
        },
        { status: 500 }
      );
    }

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      const backendError = data.error || data.message || "DART 재무제표 CSV 생성 실패";
      if (res.status === 404) {
        return NextResponse.json(
          {
            error:
              "DART API를 찾을 수 없습니다(404). backend-go를 포트 5002에서 실행한 뒤 다시 시도하세요. (프로젝트 루트: backend-go, 실행: go run ./cmd/backend)",
            message: backendError,
          },
          { status: 404 }
        );
      }
      return NextResponse.json(
        { error: backendError, message: data.message ?? backendError },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "DART 재무제표 CSV 생성 중 오류가 발생했습니다.", message },
      { status: 500 }
    );
  }
}
