import { NextRequest, NextResponse } from "next/server";

const BACKEND_BASE = process.env.BACKEND_URL || "http://localhost:5002";

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const api = searchParams.get("api")?.trim() ?? "";
    const limit = searchParams.get("limit")?.trim() ?? "";
    const offset = searchParams.get("offset")?.trim() ?? "";

    const url = new URL(`${BACKEND_BASE}/api/files`);
    if (api) {
      url.searchParams.set("api", api);
    }
    if (limit) {
      url.searchParams.set("limit", limit);
    }
    if (offset) {
      url.searchParams.set("offset", offset);
    }

    const res = await fetch(url.toString(), {
      method: "GET",
      cache: "no-store",
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return NextResponse.json(
        { error: data.error || "KRX 수집 파일 목록 조회 실패" },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "KRX 수집 파일 목록 조회 중 오류가 발생했습니다.", message },
      { status: 500 }
    );
  }
}
