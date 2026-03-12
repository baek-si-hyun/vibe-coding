import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json().catch(() => ({}));
    const res = await fetch(`${BACKEND_BASE}/api/news/crawl/resume`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
      cache: "no-store",
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        {
          error: data.error || "뉴스 수집을 실행하지 못했습니다.",
          message: data.message,
        },
        { status: res.status },
      );
    }

    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "뉴스 수집을 실행하지 못했습니다.", message },
      { status: 500 },
    );
  }
}
