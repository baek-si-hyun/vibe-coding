import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";
const MAX_RECOMMENDATIONS = 3;

function isLegacyBithumbPayload(data: unknown): boolean {
  if (!data || typeof data !== "object") return false;
  const items = (data as { items?: unknown }).items;
  if (!Array.isArray(items) || items.length === 0) return false;
  const first = items[0];
  if (!first || typeof first !== "object") return false;
  return "ratio" in first && !("total_score" in first);
}

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const rawLimit = searchParams.get("limit") || "3";
    const minTradeValue24H = searchParams.get("min_trade_value_24h") || "5000000000";
    const parsedLimit = Number.parseInt(rawLimit, 10);
    const limit = Number.isFinite(parsedLimit)
      ? String(Math.max(1, Math.min(parsedLimit, MAX_RECOMMENDATIONS)))
      : String(MAX_RECOMMENDATIONS);

    const url = new URL(`${BACKEND_BASE}/api/bithumb/screener`);
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
          error: data.error || data.message || "빗썸 데이터를 불러오지 못했습니다.",
        },
        { status: res.status },
      );
    }

    if (isLegacyBithumbPayload(data)) {
      return NextResponse.json(
        {
          error:
            "backend-go의 /api/bithumb/screener가 구형 응답을 반환하고 있습니다. backend-go 서버를 새 코드로 재시작하세요.",
        },
        { status: 503 },
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
      { error: "빗썸 데이터를 불러오지 못했습니다.", message },
      { status: 500 },
    );
  }
}
