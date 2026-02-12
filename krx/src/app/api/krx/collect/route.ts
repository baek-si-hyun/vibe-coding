import { NextRequest, NextResponse } from "next/server";

const BACKEND_BASE = process.env.BACKEND_URL || "http://localhost:5001";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json().catch(() => ({}));
    const date = body.date;
    const apiIds = body.apiIds;

    const url = new URL(`${BACKEND_BASE}/api/collect`);
    if (date) url.searchParams.set("date", date);
    if (apiIds?.length) url.searchParams.set("api", apiIds.join(","));

    const res = await fetch(url.toString(), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        { error: data.error || "KRX 수집 실패" },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "KRX 수집 중 오류가 발생했습니다.", message },
      { status: 500 }
    );
  }
}
