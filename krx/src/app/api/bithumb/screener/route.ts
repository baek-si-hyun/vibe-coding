import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE = process.env.BACKEND_URL || "http://localhost:5002";

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const mode = searchParams.get("mode") || "volume";

    const url = new URL(`${BACKEND_BASE}/api/bithumb/screener`);
    url.searchParams.set("mode", mode);

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
