import { NextResponse } from "next/server";

const BACKEND_BASE = process.env.BACKEND_URL || "http://localhost:5002";

export async function GET() {
  try {
    const res = await fetch(`${BACKEND_BASE}/api/endpoints`, {
      method: "GET",
      cache: "no-store",
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        { error: data.error || "KRX API 목록 조회 실패" },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "KRX API 목록 조회 중 오류가 발생했습니다.", message },
      { status: 500 }
    );
  }
}
