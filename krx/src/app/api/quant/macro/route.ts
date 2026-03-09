import { NextResponse } from "next/server";

const BACKEND_BASE =
  process.env.BACKEND_GO_URL ||
  process.env.BACKEND_URL ||
  "http://localhost:5002";

export async function GET() {
  try {
    const res = await fetch(`${BACKEND_BASE}/api/quant/macro`, {
      method: "GET",
      cache: "no-store",
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (res.status === 404) {
        return NextResponse.json(
          {
            error: `백엔드에 /api/quant/macro 라우트가 없습니다 (${BACKEND_BASE}). backend-go 서버를 최신 코드로 재시작하세요.`,
            message: data.error || data.message || "Not Found",
          },
          { status: 404 }
        );
      }
      return NextResponse.json(
        { error: data.error || "퀀트 매크로 조회 실패" },
        { status: res.status }
      );
    }
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      {
        error: `퀀트 매크로 조회 중 오류가 발생했습니다. backend-go(${BACKEND_BASE})가 실행 중인지 확인하세요.`,
        message,
      },
      { status: 500 }
    );
  }
}
