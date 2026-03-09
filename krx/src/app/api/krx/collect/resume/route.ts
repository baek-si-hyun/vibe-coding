import { NextRequest, NextResponse } from "next/server";

const BACKEND_BASE = process.env.BACKEND_URL || "http://localhost:5002";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json().catch(() => ({}));
    const normalizedBody: Record<string, unknown> =
      body && typeof body === "object" ? { ...(body as Record<string, unknown>) } : {};

    const rawAPIIds = normalizedBody.apiIds;
    const apiIDs = Array.isArray(rawAPIIds)
      ? rawAPIIds.filter((id): id is string => typeof id === "string" && id.trim().length > 0)
      : [];

    // Keep each batch small so Next API proxy request does not run for minutes.
    const requestedMaxDates = Number(normalizedBody.maxDates ?? 0);
    const safeMaxDates = apiIDs.length <= 1 ? 8 : 2;
    if (!Number.isFinite(requestedMaxDates) || requestedMaxDates <= 0 || requestedMaxDates > safeMaxDates) {
      normalizedBody.maxDates = safeMaxDates;
    }

    const requestedDelay = Number(normalizedBody.delay ?? 0.5);
    if (!Number.isFinite(requestedDelay) || requestedDelay < 0) {
      normalizedBody.delay = 0.5;
    } else if (requestedDelay > 2) {
      normalizedBody.delay = 2;
    }

    const url = `${BACKEND_BASE}/api/collect/resume`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 180_000);
    let res: Response;
    try {
      res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(normalizedBody),
        signal: controller.signal,
      });
    } catch (fetchError) {
      clearTimeout(timeoutId);
      const msg =
        fetchError instanceof Error ? fetchError.message : String(fetchError);
      const isTimeoutError =
        /aborted|aborterror|timed out|timeout|UND_ERR_/i.test(msg);
      const isConnectionError =
        /fetch failed|ECONNREFUSED|ENOTFOUND|network/i.test(msg);
      return NextResponse.json(
        {
          error: isTimeoutError
            ? `백엔드 응답 시간이 초과되었습니다 (${url}). 잠시 후 다시 시도하세요.`
            : isConnectionError
            ? `백엔드에 연결할 수 없습니다 (${url}). Go 서버가 실행 중인지, PORT가 5002인지 확인하세요.`
            : "KRX 수집 중 오류가 발생했습니다.",
          message: msg,
        },
        { status: isTimeoutError ? 504 : 500 }
      );
    }
    clearTimeout(timeoutId);

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      const backendError = data.error || data.message || "KRX 수집 실패";
      return NextResponse.json(
        { error: backendError, message: data.message ?? backendError },
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
