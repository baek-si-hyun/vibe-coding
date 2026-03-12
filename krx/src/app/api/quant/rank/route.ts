import { NextRequest, NextResponse } from "next/server";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";
const MIN_MARKET_CAP = 1_000_000_000_000;
const MAX_RECOMMENDATIONS = 3;

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const url = new URL(`${BACKEND_BASE}/api/quant/rank`);

    const market = searchParams.get("market")?.trim();
    const limit = searchParams.get("limit")?.trim();
    const minMarketCap =
      searchParams.get("min_market_cap")?.trim() ??
      searchParams.get("minMarketCap")?.trim();

    if (market) url.searchParams.set("market", market);
    const parsedLimit = Number.parseInt(limit ?? "", 10);
    const normalizedLimit = Number.isFinite(parsedLimit)
      ? Math.max(1, Math.min(parsedLimit, MAX_RECOMMENDATIONS))
      : MAX_RECOMMENDATIONS;
    url.searchParams.set("limit", String(normalizedLimit));
    const parsedMinMarketCap = Number.parseInt((minMarketCap ?? "").replace(/[^0-9]/g, ""), 10);
    const normalizedMinMarketCap = Number.isFinite(parsedMinMarketCap)
      ? Math.max(parsedMinMarketCap, MIN_MARKET_CAP)
      : MIN_MARKET_CAP;
    url.searchParams.set("min_market_cap", String(normalizedMinMarketCap));

    const res = await fetch(url.toString(), { method: "GET", cache: "no-store" });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (res.status === 404) {
        return NextResponse.json(
          {
            error: `백엔드에 /api/quant/rank 라우트가 없습니다 (${BACKEND_BASE}). backend-go 서버를 최신 코드로 재시작하세요.`,
            message: data.error || data.message || "Not Found",
          },
          { status: 404 }
        );
      }
      return NextResponse.json(
        { error: data.error || "퀀트 랭킹 조회 실패" },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      {
        error: `퀀트 랭킹 조회 중 오류가 발생했습니다. backend-go(${BACKEND_BASE})가 실행 중인지 확인하세요.`,
        message,
      },
      { status: 500 }
    );
  }
}
