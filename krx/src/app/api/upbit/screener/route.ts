import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";
const MAX_RECOMMENDATIONS = 3;

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const rawLimit = searchParams.get("limit") || "3";
    const minTradeValue24H = searchParams.get("min_trade_value_24h") || "5000000000";
    const parsedLimit = Number.parseInt(rawLimit, 10);
    const limit = Number.isFinite(parsedLimit)
      ? String(Math.max(1, Math.min(parsedLimit, MAX_RECOMMENDATIONS)))
      : String(MAX_RECOMMENDATIONS);

    const url = new URL(`${BACKEND_BASE}/api/upbit/screener`);
    url.searchParams.set("limit", limit);
    url.searchParams.set("min_trade_value_24h", minTradeValue24H);

    const res = await fetch(url.toString(), {
      method: "GET",
      cache: "no-store",
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        {
          error: data.error || data.message || "업비트 데이터를 불러오지 못했습니다.",
        },
        { status: res.status },
      );
    }

    return NextResponse.json(data, {
      headers: {
        "cache-control": "public, max-age=10, stale-while-revalidate=20",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "업비트 데이터를 불러오지 못했습니다.", message },
      { status: 500 },
    );
  }
}
