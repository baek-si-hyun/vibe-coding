import { NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";

export async function GET() {
  try {
    const res = await fetch(`${BACKEND_BASE}/api/news/files`, {
      method: "GET",
      cache: "no-store",
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        {
          error: data.error || "뉴스 파일 목록을 불러오지 못했습니다.",
          message: data.message,
        },
        { status: res.status },
      );
    }
    return NextResponse.json(data, {
      headers: {
        "cache-control": "public, max-age=30, stale-while-revalidate=60",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "뉴스 파일 목록을 불러오지 못했습니다.", message },
      { status: 500 },
    );
  }
}
