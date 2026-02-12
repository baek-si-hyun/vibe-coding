import { NextRequest, NextResponse } from "next/server";
import {
  getNewsConfig,
  fetchNewsSignal,
} from "@/utils/news";
import { ERROR_MESSAGES } from "@/constants/messages";

type NewsRequest = {
  query: string;
  groupId?: string;
};

type NewsResponse = {
  query: string;
  totalCount: number;
  volumeScore: number;
  sentimentScore: number;
  score: number;
  sources: string[];
  issueTypes?: string[];
} | null;

export async function POST(request: NextRequest) {
  const startTime = Date.now();
  try {
    const body = (await request.json()) as NewsRequest;
    const { query } = body;

    if (!query || typeof query !== "string") {
      return NextResponse.json(
        { error: "Query is required" },
        { status: 400 },
      );
    }

    const newsConfig = getNewsConfig();
    if (newsConfig.enabledSources.length === 0) {
      return NextResponse.json(null, {
        headers: {
          "cache-control": "public, max-age=300",
        },
      });
    }

    const signal = await fetchNewsSignal(query, newsConfig);
    const duration = Date.now() - startTime;

    return NextResponse.json(signal as NewsResponse, {
      headers: {
        "cache-control": "public, max-age=300, stale-while-revalidate=600",
      },
    });
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}

export async function GET(request: NextRequest) {
  const startTime = Date.now();
  const { searchParams } = new URL(request.url);
  const query = searchParams.get("query");

  if (!query) {
    return NextResponse.json(
      { error: "Query parameter is required" },
      { status: 400 },
    );
  }

  try {
    const newsConfig = getNewsConfig();
    if (newsConfig.enabledSources.length === 0) {
      return NextResponse.json(null, {
        headers: {
          "cache-control": "public, max-age=300",
        },
      });
    }

    const signal = await fetchNewsSignal(query, newsConfig);
    const duration = Date.now() - startTime;

    return NextResponse.json(signal as NewsResponse, {
      headers: {
        "cache-control": "public, max-age=300, stale-while-revalidate=600",
      },
    });
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
