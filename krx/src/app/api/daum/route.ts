import { NextRequest, NextResponse } from "next/server";
import { getNewsConfig, fetchDaumNews } from "@/utils/news";
import { ERROR_MESSAGES } from "@/constants/messages";

export async function GET(request: NextRequest) {
  const startTime = Date.now();
  const { searchParams } = new URL(request.url);
  const query = searchParams.get("query");
  const limit = Number.parseInt(searchParams.get("limit") || "10", 10);

  if (!query) {
    return NextResponse.json(
      { error: "query 파라미터가 필요합니다." },
      { status: 400 },
    );
  }

  const newsConfig = getNewsConfig();
  if (!newsConfig.daum) {
    return NextResponse.json(
      { error: "다음(카카오) API 설정이 없습니다." },
      { status: 400 },
    );
  }

  try {
    const signal = await fetchDaumNews(query, newsConfig.daum);
    
    if (!signal) {
      return NextResponse.json(
        { error: "뉴스를 가져올 수 없습니다." },
        { status: 404 },
      );
    }

    const response = {
      query,
      source: signal.source,
      count: signal.count,
      sentiment: signal.sentiment,
    };

    return NextResponse.json(response);
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
