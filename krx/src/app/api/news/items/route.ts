import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";

export async function GET(request: NextRequest) {
  try {
    const url = new URL(`${BACKEND_BASE}/api/news/items`);
    const { searchParams } = new URL(request.url);
    searchParams.forEach((value, key) => {
      url.searchParams.set(key, value);
    });

    const res = await fetch(url.toString(), {
      method: "GET",
      cache: "no-store",
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        {
          error: data.error || "뉴스 목록을 불러오지 못했습니다.",
          message: data.message,
        },
        { status: res.status },
      );
    }

    return NextResponse.json(data, {
      headers: {
        "cache-control": "public, max-age=15, stale-while-revalidate=30",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "뉴스 목록을 불러오지 못했습니다.", message },
      { status: 500 },
    );
  }
}
